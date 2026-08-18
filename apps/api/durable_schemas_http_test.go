package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/durableschemas"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestDurableSchemaAndMigrationPublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "state", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "consumer")
	consumerRepo, _ := catalog.Create("consumer", repositories.Metadata{Name: "worker", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("consumer", consumerRepo.ID, "owner")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	consumer := issueAccess(t, credentials, "consumer", auth.API, auth.RepositoryRead)
	store, _ := durableschemas.New(t.TempDir())
	deploymentStore, _ := deployments.New(t.TempDir())
	environment, _ := deploymentStore.PutEnvironment(string(repo.ID), "", "owner", deployments.EnvironmentInput{Name: "Production", Position: 1, Command: "deploy", Concurrency: 2})
	mux := http.NewServeMux()
	registerDurableSchemasHTTP(mux, store, catalog, credentials, deploymentStore)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/durable-schemas"
	body := `{"name":"jobs","store_kind":"queue","description":"background work envelope","source_revision":"0123456789012345678901234567890123456789","definition_path":"schemas/jobs.json","format":"json_schema","fields":[{"name":"user_id","type":"string","required":true,"classification":"personal","description":"account receiving the result"}],"owner_ids":["owner"],"compatibility":"consumers tolerate additive optional fields","retention":"delete after 7 days","privacy_commitments":["user ids remain in the EU"],"links":[{"kind":"service","resource_id":"worker","label":"job worker"},{"kind":"environment","resource_id":"production","label":"production queue"}],"change_reason":"publish reviewed schema"}`
	var schema durableschemas.Schema
	workflowJSON(t, server.URL, http.MethodPost, base, owner, body, 201, &schema)
	if schema.CurrentVersion != 1 || schema.Versions[0].AuthorID != "owner" || len(schema.Gaps) != 0 {
		t.Fatalf("unexpected schema: %+v", schema)
	}
	revised := body[:1] + `"expected_version":1,` + body[1:]
	revised = strings.Replace(revised, "additive optional fields", "additive fields; removed fields require migration", 1)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+schema.ID+"/versions", owner, revised, 201, &schema)
	migration := `{"title":"Re-key queued jobs","source_kind":"pull_request","source_id":"pull-42","schema_id":"` + schema.ID + `","from_version":1,"to_version":2,"operations":[{"kind":"read","object":"jobs.user_id","description":"read old identifier","reversible":true},{"kind":"write","object":"jobs.subject_id","description":"write replacement identifier","reversible":true},{"kind":"backfill","object":"pending jobs","description":"copy identifiers","reversible":true},{"kind":"destructive","object":"jobs.user_id","description":"remove old field after drain","destructive":true,"reversible":false}],"affected_consumers":["worker","admin replay"],"rollback_limits":["cannot restore user_id after old messages expire"],"steps":[{"id":"dual-write","description":"write both fields","operation_kinds":["write"],"owner_id":"owner"},{"id":"backfill","description":"copy pending messages","depends_on":["dual-write"],"operation_kinds":["read","backfill"],"owner_id":"owner"},{"id":"remove","description":"remove old field","depends_on":["backfill"],"operation_kinds":["destructive"],"owner_id":"owner"}],"success_measures":["zero old-field reads for 24h"],"required_approver_ids":["consumer"],"summary":"make the irreversible queue change visible before deployment"}`
	var plan durableschemas.Migration
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations", owner, migration, 201, &plan)
	if len(plan.Blockers) != 1 || plan.Blockers[0] != "approval_required:consumer" {
		t.Fatalf("approval not derived: %+v", plan)
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/approvals", consumer, `{"owner_id":"consumer","decision":"approved","rationale":"dual-read window protects our consumer"}`, 201, &plan)
	if len(plan.Blockers) != 0 || len(plan.Events) != 2 {
		t.Fatalf("approval history missing: %+v", plan)
	}
	work := `{"kind":"task","phase":"compatibility","repository_id":"` + string(consumerRepo.ID) + `","resource_id":"task-worker-dual-read","owner_kind":"human","owner_id":"consumer","base_revision":"1111111111111111111111111111111111111111","allowed_paths":["worker/decoder.go"],"context":["schema:` + schema.ID + `@1..2","reads user_id until flag retirement"],"acceptance_criteria":["old and new messages decode"]}`
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/work-items", owner, work, 201, &plan)
	if len(plan.WorkItems) != 1 || plan.WorkItems[0].Position != 1 || len(plan.WorkItems[0].Context) != 2 {
		t.Fatalf("bounded cross-repository work missing: %+v", plan.WorkItems)
	}
	dependent := `{"kind":"workspace","phase":"verification","repository_id":"` + string(repo.ID) + `","resource_id":"workspace-coexistence","owner_kind":"agent","owner_id":"agent:compat","depends_on":["` + plan.WorkItems[0].ID + `"],"base_revision":"2222222222222222222222222222222222222222","allowed_paths":["tests/compat"],"acceptance_criteria":["mixed-version fixture passes"]}`
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/work-items", owner, dependent, 201, &plan)
	contract := `{"repository_id":"` + string(consumerRepo.ID) + `","pull_request_id":"pull-worker-7","revision":"3333333333333333333333333333333333333333","work_item_ids":["` + plan.WorkItems[0].ID + `"],"old_readers":["worker@v1 reads user_id"],"new_readers":["worker@v2 reads subject_id then user_id"],"old_writers":["api@v1 writes user_id"],"new_writers":["api@v2 writes both fields"],"rollout_flags":["jobs_subject_id_dual_write"],"idempotency":"backfill keyed by message id; repeated writes are no-ops","data_transformations":["subject_id = stable account subject for user_id"],"owner_ids":["owner","consumer"],"rollback_assumptions":["keep user_id until old-reader traffic is zero"]}`
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/pull-contracts", owner, contract, 201, &plan)
	if len(plan.PullContracts) != 1 || plan.PullContracts[0].Revision == "" || len(plan.Events) != 5 {
		t.Fatalf("review contract missing: %+v", plan)
	}
	rehearsal := `{"title":"Queue identity migration rehearsal","application_revisions":{"api":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","worker":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},"migration_revision":"cccccccccccccccccccccccccccccccccccccccc","definition_path":".komodo/migration-checks.json","definition_digest":"sha256:checks-v1","dataset":{"kind":"privacy_preserving_representative","generator":"fixtures/jobs-v2.go","shape_digest":"sha256:shape-v1","privacy_method":"irreversible tokenization and rare-value suppression","row_count":1000,"object_count":1000,"byte_count":64000},"dependencies":{"postgres":"16.4"},"checks":[{"id":"upgrade","kind":"upgrade","command":"go test ./migrations -run Upgrade","input_keys":["migration","definition","data_shape","dependency:postgres"],"expected":["1000 rows upgraded"],"rollback_possible":true},{"id":"dual-read","kind":"dual_read","command":"go test ./compat -run DualRead","input_keys":["application:worker","definition","data_shape"],"expected":["old and new rows preserve subject meaning"]},{"id":"dual-write","kind":"dual_write","command":"go test ./compat -run DualWrite","input_keys":["application:api","definition","data_shape"],"expected":["both identifiers agree"]},{"id":"backfill","kind":"backfill","command":"go test ./migrations -run Backfill","input_keys":["migration","data_shape"],"expected":["backfill is idempotent"]},{"id":"validate","kind":"validation","command":"go test ./migrations -run Validate","input_keys":["application:worker","migration","data_shape"],"expected":["all invariants pass"]},{"id":"rollback","kind":"rollback","command":"go test ./migrations -run Rollback","input_keys":["migration","data_shape"],"expected":["pre-cutover state restored"],"rollback_possible":true},{"id":"failure","kind":"failure_injection","command":"go test ./migrations -run InterruptedBackfill","input_keys":["migration","dependency:postgres","data_shape"],"expected":["retry resumes without duplicate writes"]}],"maximum_duration_seconds":120,"maximum_cost":5,"currency":"USD"}`
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/rehearsals", owner, rehearsal, 201, &plan)
	if len(plan.Rehearsals) != 1 || plan.Rehearsals[0].Authority == nil || plan.Rehearsals[0].Dataset.Kind != "privacy_preserving_representative" {
		t.Fatalf("bounded rehearsal missing: %+v", plan.Rehearsals)
	}
	rh := plan.Rehearsals[0]
	results := `{"expected_version":1,"input_digests":{"application:api":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","application:worker":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","migration":"cccccccccccccccccccccccccccccccccccccccc","definition":"sha256:checks-v1","data_shape":"sha256:shape-v1","dependency:postgres":"16.4"},"results":[` +
		`{"check_id":"upgrade","status":"passed","sanitized_log":"authorization: Bearer should-not-survive\n1000 rows upgraded","counts":[{"name":"jobs","before":1000,"after":1000}],"invariants":[{"name":"row identity","passed":true,"detail":"all representative identities preserved"}],"performance":[{"name":"upgrade","value":810,"unit":"ms","limit":2000}],"artifacts":[{"name":"upgrade-report.json","digest":"sha256:upgrade","media_type":"application/json","size":240}],"duration_ms":810,"cost":0.2},` +
		`{"check_id":"dual-read","status":"passed","sanitized_log":"both versions read 1000 rows","duration_ms":120,"cost":0.1},{"check_id":"dual-write","status":"passed","sanitized_log":"both versions wrote compatible rows","duration_ms":130,"cost":0.1},{"check_id":"backfill","status":"passed","sanitized_log":"second pass changed zero rows","duration_ms":400,"cost":0.1},{"check_id":"validate","status":"passed","sanitized_log":"meaning preserved","duration_ms":90,"cost":0.1},{"check_id":"rollback","status":"passed","sanitized_log":"rollback restored old reader state","duration_ms":310,"cost":0.1},{"check_id":"failure","status":"passed","sanitized_log":"interruption resumed at cursor 500","duration_ms":510,"cost":0.2}],"attestation":"isolated networkless runner; no production credentials or authority"}`
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/rehearsals/"+rh.ID+"/attempts", owner, results, 201, &plan)
	rh = plan.Rehearsals[0]
	if len(rh.Blockers) != 0 || !rh.Attempts[0].Results[0].Redacted || strings.Contains(rh.Attempts[0].Results[0].SanitizedLog, "Bearer") {
		t.Fatalf("sanitized current rehearsal evidence missing: %+v", rh)
	}
	attemptID := rh.Attempts[0].ID
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/rehearsals/"+rh.ID+"/investigation", consumer, `{"actor_kind":"human","attempt_id":"`+attemptID+`","check_id":"failure","body":"Resume cursor remained stable after dependency interruption","evidence":["artifact:sha256:upgrade","check:failure"],"uncertainty":"does not model multi-region latency"}`, 201, &plan)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/rehearsals/"+rh.ID+"/attestations", consumer, `{"attempt_id":"`+attemptID+`","decision":"accepted","rationale":"counts, invariants, rollback, and injected failure are reviewable"}`, 201, &plan)
	rh = plan.Rehearsals[0]
	deployment, _ := deploymentStore.Create(deployments.CreateDeployment{RepositoryID: string(repo.ID), EnvironmentID: environment.ID, ReleaseID: "release-v2", SourceCommitID: "dddddddddddddddddddddddddddddddddddddddd", ActorID: "owner"})
	deployment, _ = deploymentStore.Start(string(repo.ID), deployment.ID)
	deployment, _ = deploymentStore.Complete(string(repo.ID), deployment.ID, true, "healthy rollout")
	executionBase := "/repositories/" + string(repo.ID) + "/schema-migrations/" + plan.ID + "/executions"
	workflowJSON(t, server.URL, http.MethodPost, executionBase, owner, `{"environment_id":"`+environment.ID+`","active_revision":"dddddddddddddddddddddddddddddddddddddddd","controller_kind":"human","controller_id":"owner","compatibility_ends_at":"2099-01-01T00:00:00Z","maximum_cost":100,"currency":"USD","privacy_constraints":["EU residency","no raw identifiers in observations"]}`, 201, &plan)
	if len(plan.Executions) != 1 || plan.Executions[0].Phase != "expand" || plan.Executions[0].EnvironmentName != "Production" || plan.Executions[0].ControllerID != "owner" {
		t.Fatalf("governed live execution missing: %+v", plan.Executions)
	}
	execution := plan.Executions[0]
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/approvals", consumer, `{"owner_id":"consumer","decision":"rejected","rationale":"revoke while a conflicting writer is investigated"}`, 201, &plan)
	execution = plan.Executions[0]
	if execution.State != "paused" || execution.SafetyPoint == nil || !slices.Contains(execution.SafetyPoint.Reasons, "approval_revoked:consumer") {
		t.Fatalf("revoked approval did not force an attributable safety pause: %+v", execution)
	}
	workflowJSON(t, server.URL, http.MethodPost, executionBase+"/"+execution.ID+"/controls", owner, `{"expected_revision":`+fmt.Sprint(execution.Revision)+`,"kind":"resume","reason":"must not bypass revoked approval"}`, 422, nil)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/approvals", consumer, `{"owner_id":"consumer","decision":"approved","rationale":"conflicting writer stopped; approval restored"}`, 201, &plan)
	execution = plan.Executions[0]
	workflowJSON(t, server.URL, http.MethodPost, executionBase+"/"+execution.ID+"/controls", owner, `{"expected_revision":`+fmt.Sprint(execution.Revision)+`,"kind":"resume","reason":"owner approval restored"}`, 201, &plan)
	execution = plan.Executions[0]
	observe := func(progress float64, passed bool, deploymentID string) {
		body := `{"expected_revision":` + fmt.Sprint(execution.Revision) + `,"progress":` + fmt.Sprint(progress) + `,"lag":2,"lag_unit":"seconds","invariants":[{"name":"identity preserved","passed":` + fmt.Sprint(passed) + `,"detail":"aggregate old/new identities agree"}],"service_health":[{"service":"worker","status":"healthy","detail":"error rate within objective"}],"privacy_status":"compliant","privacy_detail":"aggregate metrics only","incremental_cost":1.5,"summary":"phase observation"`
		if deploymentID != "" {
			body += `,"deployment_id":"` + deploymentID + `"`
		}
		body += `}`
		workflowJSON(t, server.URL, http.MethodPost, executionBase+"/"+execution.ID+"/observations", owner, body, 201, &plan)
		execution = plan.Executions[0]
	}
	observe(40, false, "")
	if execution.State != "paused" || execution.SafetyPoint == nil || len(execution.Blockers) == 0 || slices.Contains(execution.NextActions, "advance_to:deploy") {
		t.Fatalf("failed invariant did not block: %+v", execution)
	}
	workflowJSON(t, server.URL, http.MethodPost, executionBase+"/"+execution.ID+"/recoveries", owner, `{"expected_revision":`+fmt.Sprint(execution.Revision)+`,"kind":"open_repair","evidence":["observation:`+execution.SafetyPoint.ObservationID+`"],"reason":"trace conflicting legacy writer","repair_kind":"task","repair_id":"task-reconcile-writer","owner_id":"owner"}`, 201, &plan)
	execution = plan.Executions[0]
	if len(execution.RecoveryActions) != 1 || execution.RecoveryActions[0].RepairID != "task-reconcile-writer" || execution.State != "paused" {
		t.Fatalf("connected repair did not retain the safety evidence: %+v", execution)
	}
	workflowJSON(t, server.URL, http.MethodPost, executionBase+"/"+execution.ID+"/controls", owner, `{"expected_revision":`+fmt.Sprint(execution.Revision)+`,"kind":"pause","reason":"inspect identity mismatch"}`, 201, &plan)
	execution = plan.Executions[0]
	workflowJSON(t, server.URL, http.MethodPost, executionBase+"/"+execution.ID+"/controls", owner, `{"expected_revision":`+fmt.Sprint(execution.Revision)+`,"kind":"throttle","throttle":25,"reason":"bound database load"}`, 201, &plan)
	execution = plan.Executions[0]
	workflowJSON(t, server.URL, http.MethodPost, executionBase+"/"+execution.ID+"/controls", owner, `{"expected_revision":`+fmt.Sprint(execution.Revision)+`,"kind":"resume","reason":"aggregate mismatch resolved"}`, 201, &plan)
	execution = plan.Executions[0]
	workflowJSON(t, server.URL, http.MethodPost, executionBase+"/"+execution.ID+"/recovery-points", owner, `{"expected_revision":`+fmt.Sprint(execution.Revision)+`,"schema_version":2,"artifact_digest":"sha256:recovery-expand-40","attestation":"aggregate snapshot verified by restore rehearsal","counts":{"jobs":1000}}`, 201, &plan)
	execution = plan.Executions[0]
	for phase := 0; phase < 5; phase++ {
		if execution.Phase == "cutover" {
			workflowJSON(t, server.URL, http.MethodPost, executionBase+"/"+execution.ID+"/controls", owner, `{"expected_revision":`+fmt.Sprint(execution.Revision)+`,"kind":"abort","reason":"must not pretend irreversible cutover can be undone"}`, 422, nil)
		}
		deploymentID := ""
		if execution.Phase == "deploy" {
			deploymentID = deployment.ID
		}
		observe(100, true, deploymentID)
		workflowJSON(t, server.URL, http.MethodPost, executionBase+"/"+execution.ID+"/advance", owner, `{"expected_revision":`+fmt.Sprint(execution.Revision)+`}`, 201, &plan)
		execution = plan.Executions[0]
	}
	if execution.State != "completed" || execution.Phase != "contract" || execution.Cost != 9 || len(execution.Controls) != 4 || execution.Throttle != 25 {
		t.Fatalf("live migration did not retain ordered evidence and controls: %+v", execution)
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/retirement", owner, `{"execution_id":"`+execution.ID+`","compatibility_code":["worker dual-read flag"],"obsolete_fields":["jobs.user_id"],"success_evidence":["observation:zero-old-reads-24h"],"observation_started_at":"2020-01-01T00:00:00Z","observation_ends_at":"2020-01-02T00:00:00Z","required_owner_ids":["consumer"]}`, 201, &plan)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/retirement/approvals", consumer, `{"expected_revision":1,"owner_id":"consumer","decision":"approved","rationale":"observation window proves old reads drained"}`, 201, &plan)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/retirement/complete", owner, `{"expected_revision":2,"retained_data":["1000 job payloads and subject identifiers"],"deleted_data":["1000 legacy user_id values"],"deletion_evidence":["sha256:deletion-report"],"irreversible_decisions":["jobs.user_id physically removed"],"exceptions":["archived sanitized aggregate report retained"],"environments":[{"environment_id":"`+environment.ID+`","environment_name":"Production","schema_version":2,"definition_digest":"sha256:jobs-v2"}],"cost":3,"currency":"USD"}`, 201, &plan)
	if plan.Retirement == nil || plan.Retirement.Completion == nil || len(plan.Retirement.Completion.Environments) != 1 || len(execution.Observations) != 6 {
		t.Fatalf("verified retirement did not preserve execution evidence: %+v", plan.Retirement)
	}
	_, err := store.StartExecution(string(repo.ID), plan.ID, "agent:unassigned", durableschemas.StartExecutionInput{EnvironmentID: environment.ID, EnvironmentName: environment.Name, ActiveRevision: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", ControllerKind: "agent", ControllerID: "agent:unassigned", CompatibilityEndsAt: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), MaximumCost: 10, Currency: "USD", PrivacyConstraints: []string{"aggregate evidence only"}})
	if err != durableschemas.ErrInvalid {
		t.Fatalf("unassigned agent acquired live authority: %v", err)
	}
	change := `{"expected_version":4,"application_revisions":{"api":"dddddddddddddddddddddddddddddddddddddddd","worker":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}`
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/rehearsals/"+rh.ID+"/inputs", owner, change, 201, &plan)
	rh = plan.Rehearsals[0]
	stale := map[string]bool{}
	for _, result := range rh.Attempts[0].Results {
		stale[result.CheckID] = result.Stale
	}
	if !stale["dual-write"] || stale["upgrade"] || stale["dual-read"] || len(rh.Investigation) != 1 || len(rh.Attestations) != 1 || !rh.Attestations[0].Stale {
		t.Fatalf("affected-only invalidation or public collaboration missing: stale=%v rehearsal=%+v", stale, rh)
	}
	_, err = store.StartExecution(string(repo.ID), plan.ID, "owner", durableschemas.StartExecutionInput{EnvironmentID: environment.ID, EnvironmentName: environment.Name, ActiveRevision: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", ControllerKind: "human", ControllerID: "owner", CompatibilityEndsAt: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), MaximumCost: 10, Currency: "USD", PrivacyConstraints: []string{"aggregate evidence only"}})
	if err != durableschemas.ErrInvalid {
		t.Fatalf("stale rehearsal authorized production execution: %v", err)
	}
}
