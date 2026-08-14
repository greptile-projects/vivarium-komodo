package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/datacommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dataflows"
	"github.com/greptile-projects/vivarium-komodo/apps/api/extensions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/privacyassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/privacydrift"
	"github.com/greptile-projects/vivarium-komodo/apps/api/privacyverification"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

// TestPrivacyEngineeringWorkflow is the black-box boundary for the complete
// commitment-to-corrected-data-use loop. It proves that declarations, agent
// reasoning, synthetic evidence, approvals, production signals, and repairs
// remain connected without turning any of them into data or merge authority.
func TestPrivacyEngineeringWorkflow(t *testing.T) {
	requireGit(t)
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	previewStore, _ := previews.New(t.TempDir())
	commitments, _ := datacommitments.New(t.TempDir())
	flows, _ := dataflows.New(t.TempDir())
	assessments, _ := privacyassessments.New(t.TempDir())
	verification, _ := privacyverification.New(t.TempDir())
	drift, _ := privacydrift.New(t.TempDir())
	people, _ := users.New(t.TempDir())
	activity, _ := activities.New(t.TempDir(), people)
	extensionStore, _ := extensions.New(t.TempDir())
	orgs, _ := organizations.New(t.TempDir())
	runner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, activity, runner, checks, previewStore, verification)
	registerCheckRunsHTTP(mux, checks, runner, pulls, catalog, credentials, nil, activity)
	registerPreviewsHTTP(mux, previewStore, previews.NewRunner(previewStore, catalog), pulls, catalog, credentials, previewSources{})
	registerReleasesHTTP(mux, releaseStore, checks, runner, pulls, catalog, credentials)
	registerDataCommitmentsHTTP(mux, commitments, catalog, credentials)
	registerDataFlowsHTTP(mux, flows, commitments, catalog, credentials)
	registerPrivacyAssessmentsHTTP(mux, assessments, catalog, credentials, privacyAssessmentSources{pulls: pulls, flows: flows, commitments: commitments, repositories: catalog})
	registerPrivacyVerificationHTTP(mux, verification, catalog, credentials, commitments, previewStore, checks, pulls)
	registerPrivacyDriftHTTP(mux, drift, catalog, credentials, commitments, plans)
	registerExtensionsHTTP(mux, extensionStore, catalog, orgs, credentials, activity, pulls)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "privacy-owner", auth.API, auth.ProfileRead, auth.ProfileWrite, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "privacy-owner", auth.Git, auth.GitRead, auth.GitWrite)
	agent := issueAccess(t, credentials, "codex", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agentGit := issueAccess(t, credentials, "codex", auth.Git, auth.GitRead, auth.GitWrite)
	developer := issueAccess(t, credentials, "extension-developer", auth.API, auth.ProfileRead, auth.ProfileWrite, auth.RepositoryRead, auth.RepositoryWrite)
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"trusted-export","visibility":"private"}`, 201, &repository)
	if _, err := catalog.AddCollaborator("privacy-owner", repository.ID, "codex"); err != nil {
		t.Fatal(err)
	}
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		u.User = url.UserPassword("git", token)
		return u.String()
	}

	work := gitClone(t, remote(ownerGit))
	gitOutput(t, work, "config", "user.name", "Privacy Owner")
	gitOutput(t, work, "config", "user.email", "privacy@example.test")
	writeWorkflowFile(t, work, "app/export.sh", "#!/bin/sh\nprintf 'existing account export deleted after 7 days\\n'\n")
	writeWorkflowFile(t, work, ".komodo/data-flows.json", `{"version":1,"journey":"account export"}`)
	writeWorkflowFile(t, work, ".komodo/releases.json", `{"version":1,"builds":[{"name":"export-service","command":"mkdir -p dist; cp app/export.sh dist/export.sh","artifacts":["dist/export.sh"]}]}`)
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Document existing account export")
	baseline := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "main")

	base := "/repositories/" + string(repository.ID)
	commitmentBody := `{"title":"Account export limits","scopes":[{"kind":"repository","name":"Account export"},{"kind":"extension","resource_id":"export-helper","name":"Export helper"}],"data_uses":[{"id":"account-export","name":"Requested account archive","categories":["profile"],"purposes":["user requested portability"],"subjects":["requesting user"],"collection":"only after explicit export request","processing":["assemble archive"],"sharing":["requesting user only"],"retention":"seven days","residency":["selected project region"],"deletion":"automatic after seven days and immediate on cancellation","consent":"explicit per-export confirmation","owner_ids":["privacy-owner"]}],"guarantees":[{"id":"bounded-export","description":"Consent, minimization, seven-day retention, and deletion are enforced","status":"supported"}],"owner_ids":["privacy-owner"],"links":[{"kind":"policy","url":"https://example.test/privacy"},{"kind":"notice","url":"https://example.test/export-notice"}],"change_reason":"Make the existing user expectation testable"}`
	var commitment datacommitments.Commitment
	workflowJSON(t, server.URL, http.MethodPost, base+"/data-commitments", owner, commitmentBody, 201, &commitment)
	flowBody := func(revision, title, recipient string) string {
		return `{"revision":"` + revision + `","title":"` + title + `","manifest":{"path":".komodo/data-flows.json"},"commitments":[{"id":"` + commitment.ID + `","version":1,"data_use_ids":["account-export"]}],"nodes":[{"id":"request","kind":"interaction","name":"User requests export","location":{"path":"app/export.sh"},"evidence_accessible":true},{"id":"archive","kind":"store","name":"Temporary archive","location":{"path":"app/export.sh"},"evidence_accessible":true},{"id":"helper","kind":"extension","name":"Export helper","resource_id":"` + recipient + `","evidence_accessible":false,"restricted_evidence_ref":"extension installation logs require owner access"}],"edges":[{"id":"collect","from":"request","to":"archive","action":"enters","categories":["profile"],"purpose":"user requested portability"},{"id":"deliver","from":"archive","to":"helper","action":"leaves","categories":["profile"],"purpose":"assemble the requested archive"}]}`
	}
	var baselineFlow dataflows.Flow
	workflowJSON(t, server.URL, http.MethodPost, base+"/data-flows", owner, flowBody(baseline, "Account export journey", "export-helper"), 201, &baselineFlow)
	workflowJSON(t, server.URL, http.MethodGet, base+"/data-flows/"+baselineFlow.ID, owner, "", 200, &baselineFlow)
	if len(baselineFlow.Blockers) == 0 || baselineFlow.Blockers[0].Kind != "inaccessible_dependency" {
		t.Fatalf("restricted dependency was not explicit: %#v", baselineFlow.Blockers)
	}

	// The extension is real and least-privilege; its later removal must empty
	// authority without erasing its bounded place in the historical flow map.
	var extension extensions.Extension
	workflowJSON(t, server.URL, http.MethodPost, "/extensions", developer, `{"name":"Export helper","description":"Builds requested archives","operator_contact":"extensions@example.test","capabilities":["assemble export"],"callback_url":"https://example.test/events","action_url":"https://example.test/actions","requested_permissions":["metadata:read"],"event_types":["pull_request.created"],"rotation_policy":{"interval_days":30,"overlap_hours":24,"contact_on_failure":true}}`, 201, &extension)
	workflowJSON(t, server.URL, http.MethodPost, "/extensions/"+extension.ID+"/endpoint-verifications", developer, `{"endpoint":"callback","token":"`+extension.Callback.VerificationToken+`"}`, 200, &extension)
	workflowJSON(t, server.URL, http.MethodPost, "/extensions/"+extension.ID+"/endpoint-verifications", developer, `{"endpoint":"actions","token":"`+extension.Actions.VerificationToken+`"}`, 200, &extension)
	var installation extensions.Installation
	workflowJSON(t, server.URL, http.MethodPost, base+"/extension-installations", owner, `{"extension_id":"`+extension.ID+`","permissions":["metadata:read"],"event_types":["pull_request.created"],"resource_types":["pull_requests"],"capability_decisions":[{"capability":"assemble export","decision":"approved"}],"settings":{"mode":"bounded"}}`, 201, &installation)

	agentWork := gitClone(t, remote(agentGit))
	gitOutput(t, agentWork, "config", "user.name", "Codex Agent")
	gitOutput(t, agentWork, "config", "user.email", "codex@agents.local")
	gitOutput(t, agentWork, "switch", "-c", "feature/shared-export")
	writeWorkflowFile(t, agentWork, "app/export.sh", "#!/bin/sh\nprintf 'consent=yes fields=profile retention=7d deletion=verified recipient=requester\\n'\n")
	writeWorkflowFile(t, agentWork, ".komodo/privacy-checks.json", `{"version":1,"checks":[{"name":"privacy/account-export","command":"sh app/export.sh","privacy":{"journey_ids":["account-export"],"dimensions":["consent","minimization","retention","deletion"],"inputs":["app/export.sh",".komodo/data-flows.json"],"commitment_ids":["`+commitment.ID+`"],"synthetic_data":true,"requires_preview":true}}]}`)
	gitOutput(t, agentWork, "add", ".")
	gitOutput(t, agentWork, "commit", "-m", "Add agent-assisted shared export flow")
	firstCandidate := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push", "-u", "origin", "feature/shared-export")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, base+"/pull-requests", agent, `{"title":"Add bounded shared export","body":"Agent-assisted flow constrained by the data-use commitment.","source_branch":"feature/shared-export","target_branch":"main"}`, 201, &pull)
	var candidateFlow dataflows.Flow
	workflowJSON(t, server.URL, http.MethodPost, base+"/data-flows", agent, flowBody(firstCandidate, "Account export journey", extension.ID), 201, &candidateFlow)

	assessmentInput := func(revision, flowID, summary string) string {
		return `{"revision":"` + revision + `","target_revision":"` + baseline + `","summary":"` + summary + `","comparisons":[{"kind":"recipient","summary":"A helper participates in archive assembly","baseline_flow_id":"` + baselineFlow.ID + `","candidate_flow_id":"` + flowID + `","before":"requester only","after":"bounded helper then requester","evidence":[{"path":"app/export.sh"}]},{"kind":"retention","summary":"Archive remains seven-day bounded","baseline_flow_id":"` + baselineFlow.ID + `","candidate_flow_id":"` + flowID + `","evidence":[{"path":"app/export.sh"}]}],"commitments":[{"id":"` + commitment.ID + `","baseline_version":1,"candidate_version":1,"data_use_ids":["account-export"]}],"requirements":[{"id":"owner","kind":"owner_acknowledgement","owner_ids":["privacy-owner"],"rationale":"Confirm the helper does not become a recipient"},{"id":"tests","kind":"test","owner_ids":["privacy-owner"],"rationale":"Prove consent, minimization, retention, and deletion"}],"residual_risk":"Extension evidence remains permission scoped"}`
	}
	assessmentPath := base + "/pull-requests/" + pull.ID + "/privacy-assessments"
	var assessment privacyassessments.Assessment
	workflowJSON(t, server.URL, http.MethodPost, assessmentPath, agent, assessmentInput(firstCandidate, candidateFlow.ID, "Initial design review"), 201, &assessment)
	workflowJSON(t, server.URL, http.MethodPost, assessmentPath+"/"+assessment.ID+"/entries", owner, `{"kind":"challenge","body":"Do not send the archive to the helper; run it as a minimization-only transformer.","requirement_ids":["owner"],"evidence":[{"path":"app/export.sh"}]}`, 201, &assessment)

	// Human challenge changes the design. The old exact-candidate assessment is
	// stale and cannot carry its acceptance onto the revised commit.
	writeWorkflowFile(t, agentWork, "app/export.sh", "#!/bin/sh\nprintf 'consent=yes fields=profile retention=7d deletion=verified recipient=requester helper_mode=transform-only\\n'\n")
	gitOutput(t, agentWork, "add", "app/export.sh")
	gitOutput(t, agentWork, "commit", "-m", "Keep export recipient user-bound")
	revised := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push")
	workflowJSON(t, server.URL, http.MethodPost, base+"/pull-requests/"+pull.ID+"/synchronize", agent, `{}`, 200, &pull)
	workflowJSON(t, server.URL, http.MethodGet, assessmentPath+"/"+assessment.ID, owner, "", 200, &assessment)
	if !assessment.Stale {
		t.Fatal("source change did not invalidate the old privacy analysis")
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/data-flows", agent, flowBody(revised, "Account export journey", extension.ID), 201, &candidateFlow)
	workflowJSON(t, server.URL, http.MethodPost, assessmentPath, agent, assessmentInput(revised, candidateFlow.ID, "Revised minimization design"), 201, &assessment)
	for _, requirement := range []string{"owner", "tests"} {
		workflowJSON(t, server.URL, http.MethodPost, assessmentPath+"/"+assessment.ID+"/acknowledgements", owner, `{"requirement_id":"`+requirement+`","decision":"accept","rationale":"Current revision matches the bounded commitment","revision":"`+revised+`"}`, 201, &assessment)
	}

	var policy privacyverification.Policy
	workflowJSON(t, server.URL, http.MethodPost, base+"/privacy-verification-policies", owner, `{"name":"Account export privacy gate","commitment_id":"`+commitment.ID+`","commitment_version":1,"target_branches":["main"],"paths":["app"],"required_checks":["privacy/account-export"],"required_dimensions":["consent","minimization","retention","deletion"],"privacy_owner_ids":["privacy-owner"]}`, 201, &policy)
	preview, _ := previewStore.Create(previews.Preview{RepositoryID: string(repository.ID), PullRequestID: pull.ID, Revision: revised, State: "ready", Definition: previews.Definition{Resources: previews.Resources{LifetimeMinutes: 60}}})
	waitForWorkflowCheck(t, server.URL, base+"/pull-requests/"+pull.ID, owner, revised, checkruns.Succeeded)
	// A waiver without valid bounded follow-up is rejected; delivery continues
	// through current proof rather than treating the failed request as approval.
	workflowJSON(t, server.URL, http.MethodPost, base+"/pull-requests/"+pull.ID+"/privacy-verification-exceptions", owner, `{"policy_id":"`+policy.ID+`","dimensions":["deletion"],"reason":"ship without current proof","follow_up":{"kind":"issue","resource_id":""},"expires_at":"`+time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339Nano)+`"}`, 422, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/pull-requests/"+pull.ID+"/privacy-verification-acknowledgements", owner, `{"policy_id":"`+policy.ID+`","preview_id":"`+preview.ID+`","decision":"accept","rationale":"Synthetic preview proves consent, minimization, retention, and deletion on the current candidate"}`, 201, nil)
	var privacyReady privacyverification.Assessment
	workflowJSON(t, server.URL, http.MethodPost, base+"/releases/privacy-readiness", owner, `{"pull_request_id":"`+pull.ID+`","revision":"`+revised+`","target_branch":"main","paths":["app/export.sh"]}`, 200, &privacyReady)
	if !privacyReady.Ready || len(privacyReady.Coverage) != 4 {
		t.Fatalf("current privacy proof not ready: %#v", privacyReady)
	}
	workflowJSON(t, server.URL, http.MethodPut, base+"/pull-requests/"+pull.ID+"/reviews/me", owner, `{"decision":"approve"}`, 200, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/pull-requests/"+pull.ID+"/merge", owner, `{}`, 200, &pull)
	var released releases.Release
	workflowJSON(t, server.URL, http.MethodPost, base+"/releases", owner, `{"version":"v1.0.0","commit_id":"`+pull.MergeCommitID+`","notes":"Privacy-reviewed account export"}`, 201, &released)

	var monitor privacydrift.Monitor
	workflowJSON(t, server.URL, http.MethodPost, base+"/privacy-drift/monitors", owner, `{"name":"Export deletion SLO","commitment_id":"`+commitment.ID+`","commitment_version":1,"data_use_ids":["account-export"],"signal_kinds":["failed_deletion"],"release_id":"`+released.ID+`","release_revision":"`+pull.MergeCommitID+`","environment_id":"production","extension_id":"`+installation.ID+`","owner_ids":["privacy-owner"],"participant_ids":["codex"],"retention_days":30}`, 201, &monitor)
	var signal privacydrift.Signal
	windowStart := time.Now().UTC().Add(-time.Hour)
	workflowJSON(t, server.URL, http.MethodPost, base+"/privacy-drift/signals", owner, `{"monitor_id":"`+monitor.ID+`","kind":"failed_deletion","data_use_id":"account-export","observed":"one aggregate deletion queue exceeded seven days","expected":"all archives deleted within seven days","evidence":{"signal_reference":"metric:export-deletion-age","metric":"overdue_archive_count","aggregate_count":1,"window_start":"`+windowStart.Format(time.RFC3339Nano)+`","window_end":"`+time.Now().UTC().Format(time.RFC3339Nano)+`","digest":"sha256:sanitized","summary":"aggregate count only; no subject or archive identifiers","sanitized":true}}`, 201, &signal)
	for _, event := range []string{`{"kind":"contain","summary":"Disable new helper processing while preserving user-local export"}`, `{"kind":"private_incident","summary":"Investigate the deletion queue with permission-scoped evidence","resource_kind":"investigation","resource_id":"privacy-investigation-1"}`, `{"kind":"exception_rejected","summary":"Privacy owner rejected extending retention","resource_kind":"proposal","resource_id":"rejected-retention-exception"}`} {
		workflowJSON(t, server.URL, http.MethodPost, base+"/privacy-drift/signals/"+signal.ID+"/events", owner, event, 201, nil)
	}
	var repair struct {
		Signal privacydrift.Signal `json:"signal"`
		Task   proposals.Task      `json:"task"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/privacy-drift/signals/"+signal.ID+"/repairs", owner, `{"title":"Restore seven-day deletion","owner_kind":"agent","owner_id":"codex","acceptance_criteria":["Deletion queue drains within seven days","Privacy synthetic journey remains current"]}`, 201, &repair)
	if repair.Task.ReasoningContext == nil || repair.Signal.AuthorityGranted {
		t.Fatalf("repair lost bounded reasoning or granted authority: %#v", repair)
	}

	workflowJSON(t, server.URL, http.MethodDelete, base+"/extension-installations/"+installation.ID, owner, "", 200, &installation)
	if installation.Status != "revoked" || len(installation.Authority.Permissions) != 0 {
		t.Fatalf("revoked extension retained authority: %#v", installation)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/privacy-drift/signals/"+signal.ID+"/events", owner, `{"kind":"resolved","summary":"Connected repair restored seven-day deletion; extension authority remains revoked"}`, 201, nil)
	var retained struct {
		Signals []privacydrift.Signal `json:"signals"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base+"/privacy-drift", owner, "", 200, &retained)
	if len(retained.Signals) != 1 || retained.Signals[0].State != "resolved" || retained.Signals[0].Repair == nil || !strings.Contains(retained.Signals[0].Events[len(retained.Signals[0].Events)-1].Summary, "revoked") {
		t.Fatalf("correction trail incomplete: %#v", retained.Signals)
	}
}
