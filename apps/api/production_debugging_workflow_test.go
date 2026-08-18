package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	dw "github.com/greptile-projects/vivarium-komodo/apps/api/debuggingworkspaces"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/runtimeinvestigations"
	rp "github.com/greptile-projects/vivarium-komodo/apps/api/runtimeprobes"
	rr "github.com/greptile-projects/vivarium-komodo/apps/api/runtimerepairs"
	rplay "github.com/greptile-projects/vivarium-komodo/apps/api/runtimereplays"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

// TestProductionDebuggingWorkflow is the black-box boundary from an
// intermittent released-user failure to a reviewed, deployed, production-
// validated repair. Runtime evidence and agent analysis remain bounded records;
// stock Git, ordinary checks, review, release and deployment retain authority.
func TestProductionDebuggingWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releasesStore, _ := releases.New(t.TempDir())
	deploymentStore, _ := deployments.New(t.TempDir())
	debugging, _ := dw.New(t.TempDir())
	probes, _ := rp.New(t.TempDir())
	investigations, _ := ri.New(t.TempDir())
	replays, _ := rplay.New(t.TempDir())
	repairs, _ := rr.New(t.TempDir())
	workspaceStore, _ := workspaces.New(t.TempDir())
	workspaceRunner := workspaces.NewRunner(workspaceStore, catalog)
	previewStore, _ := previews.New(t.TempDir())
	checkRunner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, nil, checkRunner, checks)
	registerCheckRunsHTTP(mux, checks, checkRunner, pulls, catalog, credentials, nil, nil)
	registerReleasesHTTP(mux, releasesStore, checks, checkRunner, pulls, catalog, credentials)
	registerDeploymentsHTTP(mux, deploymentStore, releasesStore, checks, catalog, credentials, nil, nil, pulls)
	registerWorkspacesHTTP(mux, workspaceStore, workspaceRunner, catalog, credentials, plans, pulls, nil)
	registerDebuggingWorkspacesHTTP(mux, debugging, catalog, credentials, releasesStore)
	registerRuntimeProbesHTTP(mux, probes, debugging, catalog, credentials)
	registerRuntimeInvestigationsHTTP(mux, investigations, debugging, probes, catalog, credentials)
	registerRuntimeReplaysHTTP(mux, replays, debugging, probes, investigations, workspaceStore, previewStore, catalog, credentials)
	registerRuntimeRepairsHTTP(mux, repairs, debugging, replays, investigations, plans, pulls, checks, releasesStore, deploymentStore, catalog, credentials)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "service-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	responder := issueAccess(t, credentials, "responder", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "debug-agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "service-owner", auth.Git, auth.GitRead, auth.GitWrite)
	agentGit := issueAccess(t, credentials, "debug-agent", auth.Git, auth.GitRead, auth.GitWrite)
	var repo repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"intermittent-checkout","visibility":"private"}`, http.StatusCreated, &repo)
	for _, actor := range []string{"responder", "debug-agent"} {
		if _, err := catalog.AddCollaborator("service-owner", repo.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + string(repo.ID))
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	work := gitClone(t, remote(ownerGit))
	gitOutput(t, work, "config", "user.name", "Service Owner")
	gitOutput(t, work, "config", "user.email", "owner@example.com")
	writeWorkflowFile(t, work, "checkout/retry.go", "package checkout\nfunc Retry(sequence int) bool { return sequence < 2 }\n")
	writeWorkflowFile(t, work, ".komodo/checks.json", `{"version":1,"checks":[{"name":"checkout-contract","command":"grep -q 'sequence <= 2' checkout/retry.go"}]}`)
	writeWorkflowFile(t, work, ".komodo/workspaces.json", `{"version":1,"tools":[{"name":"sh","version":"system"}],"dependencies":["repository snapshot"],"setup":["true"],"resources":{"cpu_seconds":10,"memory_mb":128,"disk_mb":128,"setup_timeout_seconds":10}}`)
	writeWorkflowFile(t, work, ".komodo/releases.json", `{"version":1,"builds":[{"name":"checkout","command":"mkdir -p dist; cp checkout/retry.go dist/retry.go","artifacts":["dist/retry.go"]}]}`)
	writeWorkflowFile(t, work, ".komodo/deployments.json", `{"version":1,"environments":[{"name":"production","stages":[{"name":"canary","health":[{"name":"artifact","command":"test -s \"$KOMODO_ARTIFACT_PATH\""}]}]}]}`)
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Release intermittent retry failure")
	affectedRevision := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "main")
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+string(repo.ID)+"/required-checks", owner, `{"branch":"main","checks":["checkout-contract"]}`, http.StatusOK, nil)
	var affected releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/releases", owner, `{"version":"v1.0.0","commit_id":"`+affectedRevision+`","notes":"Intermittent user failure"}`, http.StatusCreated, &affected)
	waitForReleaseArtifact(t, server.URL, string(repo.ID), affected.ID, owner)

	debugBase := "/repositories/" + string(repo.ID) + "/debugging-workspaces"
	var workspace dw.Workspace
	body := `{"title":"Second checkout retry loses the cart","origin":{"kind":"support_thread","resource_id":"support-418","summary":"Released user consented to investigation of intermittent failure"},"release_id":"` + affected.ID + `","release_revision":"` + affectedRevision + `","environment":"production","time_window":{"start":"2026-08-18T12:00:00Z","end":"2026-08-18T12:10:00Z"},"user_journey":"checkout retry after payment timeout","owner_ids":["service-owner"],"severity":"high","source_revision":"` + affectedRevision + `","bindings":[{"kind":"package","resource_id":"checkout","revision":"pkg-v1","status":"available"},{"kind":"configuration","resource_id":"checkout-prod","revision":"cfg-9","status":"available"},{"kind":"infrastructure","resource_id":"deploy-prod","revision":"infra-12","status":"available"}],"permitted_evidence":[{"kind":"traces","audience":"participants","access":"permitted","retention":"24h"},{"kind":"profile","audience":"participants","access":"permitted","retention":"1h"}],"audience":"participants","participant_ids":["service-owner","responder","debug-agent"]}`
	workflowJSON(t, server.URL, http.MethodPost, debugBase, responder, body, http.StatusCreated, &workspace)
	probeBase := debugBase + "/" + workspace.ID + "/probes"
	expiry := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	requestProbe := func(kind string) rp.Probe {
		var p rp.Probe
		workflowJSON(t, server.URL, http.MethodPost, probeBase, responder, `{"environment":"production","kind":"`+kind+`","scope":["checkout/retry"],"preview":{"data_categories":["timing","request shape"],"estimated_cost":0.12,"estimated_load":"low","audience":"participants","sampling_rate":0.05,"retention_hours":24,"privacy_policy":"user consent and field redaction","security_policy":"no secrets"},"purpose":"correlate intermittent retry","consent_actor_ids":["responder"],"expires_at":"`+expiry+`"}`, http.StatusCreated, &p)
		return p
	}
	denied := requestProbe("profile")
	workflowJSON(t, server.URL, http.MethodPost, probeBase+"/"+denied.ID+"/decision", owner, `{"decision":"denied","reason":"profile load is unnecessary for this journey"}`, http.StatusOK, &denied)
	trace := requestProbe("traces")
	workflowJSON(t, server.URL, http.MethodPost, probeBase+"/"+trace.ID+"/decision", owner, `{"decision":"approved","reason":"bounded five-percent trace is within consent and load policy"}`, http.StatusOK, &trace)
	capture := func(expected int, records, gaps string) rp.Probe {
		var p rp.Probe
		workflowJSON(t, server.URL, http.MethodPost, probeBase+"/"+trace.ID+"/captures", responder, `{"started_at":"2026-08-18T12:01:00Z","ended_at":"2026-08-18T12:01:05Z","records_expected":`+formatFloat(float64(expected))+`,"records":`+records+`,"gaps":`+gaps+`,"provenance":"collector trace-7 sampled under approved probe"}`, http.StatusCreated, &p)
		return p
	}
	trace = capture(3, `["retry=2 user_id=usr-9 token=live correlation=abc"]`, `["sampling noise omitted two spans"]`)
	noisy := trace.Captures[0]
	trace = capture(1, `["retry=2 user_id=usr-9 token=live path=checkout.Retry correlation=abc"]`, `[]`)
	clean := trace.Captures[1]
	if denied.Status != "denied" || noisy.Completeness != "incomplete" || !strings.Contains(clean.SanitizedData[0], "[REDACTED_USER_DATA]") || !strings.Contains(clean.SanitizedData[0], "[REDACTED]") {
		t.Fatalf("probe containment missing: denied=%s noisy=%#v clean=%#v", denied.Status, noisy, clean)
	}

	invBase := debugBase + "/" + workspace.ID + "/investigations"
	var inv ri.Investigation
	workflowJSON(t, server.URL, http.MethodPost, invBase, responder, `{"title":"Correlate retry trace","question":"Why does only the second released retry fail?","audience":"participants","participants":["service-owner","debug-agent"],"evidence":[{"probe_id":"`+trace.ID+`","capture_id":"`+clean.ID+`","summary":"sanitized second-retry span","audience":"participants"}],"correlations":[{"kind":"symbol","resource_id":"checkout.Retry","revision":"`+affectedRevision+`","path":"checkout/retry.go","symbol":"Retry","relationship":"trace names exact released symbol","status":"resolved"},{"kind":"deployment","resource_id":"production-v1","revision":"`+affectedRevision+`","relationship":"affected release deployment","status":"resolved"}]}`, http.StatusCreated, &inv)
	evidenceID, codeID := inv.Evidence[0].ID, inv.Correlations[0].ID
	workflowJSON(t, server.URL, http.MethodPost, invBase+"/"+inv.ID+"/claims", responder, `{"kind":"hypothesis","body":"Payment provider latency causes the retry loss.","citations":[{"evidence_id":"`+evidenceID+`"}]}`, http.StatusCreated, &inv)
	wrongID := inv.Claims[0].ID
	workflowJSON(t, server.URL, http.MethodPost, invBase+"/"+inv.ID+"/claims", owner, `{"kind":"challenge","body":"The trace ends before provider access; this explanation is unsupported.","citations":[{"claim_id":"`+wrongID+`"}]}`, http.StatusCreated, &inv)
	var delegated struct {
		Investigation ri.Investigation `json:"investigation"`
		Credential    string           `json:"credential"`
	}
	workflowJSON(t, server.URL, http.MethodPost, invBase+"/"+inv.ID+"/agents", owner, `{"agent_id":"debug-agent","mandate":"Read the selected sanitized trace and exact symbol only","evidence_ids":["`+evidenceID+`"],"correlation_ids":["`+codeID+`"],"expires_at":"`+expiry+`"}`, http.StatusCreated, &delegated)
	workflowJSON(t, server.URL, http.MethodPost, "/runtime-investigation-agent/claims", delegated.Credential, `{"kind":"finding","body":"The strict sequence guard rejects the second retry before provider access.","verdict":"supported","citations":[{"evidence_id":"`+evidenceID+`"},{"correlation_id":"`+codeID+`"}]}`, http.StatusCreated, &inv)
	cause := inv.Claims[len(inv.Claims)-1]
	session := delegated.Investigation.AgentSessions[0]
	workflowJSON(t, server.URL, http.MethodPost, invBase+"/"+inv.ID+"/agents/"+session.ID+"/controls", owner, `{"action":"revoke","guidance":"analysis complete; remove runtime evidence access"}`, http.StatusOK, &inv)
	workflowJSON(t, server.URL, http.MethodGet, "/runtime-investigation-agent/context", delegated.Credential, "", http.StatusForbidden, nil)
	if inv.Claims[0].Status != "disputed" || cause.Status != "supported" || len(inv.Authority) != 0 {
		t.Fatalf("challenge or authority trail missing: %#v", inv)
	}

	var isolated workspaces.Workspace
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/workspaces", responder, `{"revision":"`+affectedRevision+`","source_context":{"type":"repository"}}`, http.StatusCreated, &isolated)
	replayBase := debugBase + "/" + workspace.ID + "/replays"
	var replay rplay.Scenario
	workflowJSON(t, server.URL, http.MethodPost, replayBase, responder, `{"investigation_id":"`+inv.ID+`","name":"synthetic second retry","behavior":"second retry is rejected before provider access","audience":"participants","participant_ids":["service-owner","debug-agent"],"evidence_ids":["`+clean.ID+`"],"state_kind":"synthetic","inputs":[{"name":"retry.json","kind":"synthetic","value":"{\"sequence\":2}","source_evidence_id":"`+clean.ID+`","transformation":"identifier removed and timing bucketed"}],"commands":["go test ./checkout"],"invariants":[{"name":"second-retry-rejected","expectation":"Retry(2) returns false"}]}`, http.StatusCreated, &replay)
	attempt := func(actor, revision, mode string, observed bool, target string) rplay.Scenario {
		var v rplay.Scenario
		workflowJSON(t, server.URL, http.MethodPost, replayBase+"/"+replay.ID+"/attempts", actor, `{"mode":"`+mode+`","target_kind":"workspace","target_id":"`+target+`","revision":"`+revision+`","environment":{"network":"disabled","state":"synthetic"},"commands":["go test ./checkout"],"traces":["Retry sequence bucket 2"],"outputs":["bounded synthetic result"],"invariant_results":{"second-retry-rejected":`+map[bool]string{true: "true", false: "false"}[observed]+`},"cost":0.03,"production_differences":["synthetic user and payment state"]}`, http.StatusCreated, &v)
		return v
	}
	replay = attempt(responder, affectedRevision, "reproduction", false, isolated.ID)
	replay = attempt(agent, affectedRevision, "reproduction", true, isolated.ID)
	replay = attempt(owner, affectedRevision, "reproduction", true, isolated.ID)
	if !replay.Reproduced || replay.RepeatedPassingAttempts != 2 || replay.Attempts[0].Status != "not_reproduced" {
		t.Fatalf("reproduction correction missing: %#v", replay)
	}

	var repairResponse struct {
		Repair   rr.Repair          `json:"repair"`
		Proposal proposals.Proposal `json:"proposal"`
		Task     proposals.Task     `json:"task"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/runtime-repairs", owner, `{"workspace_id":"`+workspace.ID+`","replay_id":"`+replay.ID+`","investigation_id":"`+inv.ID+`","cause_claim_id":"`+cause.ID+`","title":"Allow the second checkout retry","owner_kind":"human","owner_id":"responder","affected_revision":"`+affectedRevision+`","acceptance_criteria":["second retry reaches provider"],"regression_criteria":["bounded synthetic replay remains fixed"]}`, http.StatusCreated, &repairResponse)
	assignmentPath := "/repositories/" + string(repo.ID) + "/proposals/" + repairResponse.Proposal.ID + "/plan/tasks/" + repairResponse.Task.ID + "/assignment"
	workflowJSON(t, server.URL, http.MethodPut, assignmentPath, owner, `{"kind":"human","assignee_id":"responder","mandate":"Publish the agent-authored change only through ordinary review.","base_revision":"`+affectedRevision+`"}`, http.StatusOK, &repairResponse.Task)
	agentWork := gitClone(t, remote(agentGit))
	gitOutput(t, agentWork, "config", "user.name", "Debug Agent")
	gitOutput(t, agentWork, "config", "user.email", "debug-agent@example.com")
	gitOutput(t, agentWork, "switch", "-c", "repair/retry")
	writeWorkflowFile(t, agentWork, "checkout/retry.go", "package checkout\n// Permit the idempotent second attempt; later retries remain bounded.\nfunc Retry(sequence int) bool { return sequence <= 2 }\n")
	gitOutput(t, agentWork, "add", "checkout/retry.go")
	gitOutput(t, agentWork, "commit", "-m", "Allow idempotent second checkout retry")
	candidate := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push", "-u", "origin", "repair/retry")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/pull-requests", responder, `{"proposal_id":"`+repairResponse.Proposal.ID+`","task_id":"`+repairResponse.Task.ID+`","title":"Repair intermittent second retry","body":"Agent-authored commit published by the accountable responder from bounded debugging evidence; no production access.","source_branch":"repair/retry","target_branch":"main"}`, http.StatusCreated, &pull)
	pullBase := "/repositories/" + string(repo.ID) + "/pull-requests/" + pull.ID
	run := waitForWorkflowCheck(t, server.URL, pullBase, owner, candidate, checkruns.Succeeded)
	repairBase := "/repositories/" + string(repo.ID) + "/runtime-repairs/" + repairResponse.Repair.ID
	var repair rr.Repair
	workflowJSON(t, server.URL, http.MethodPost, repairBase+"/verifications", owner, `{"pull_request_id":"`+pull.ID+`","revision":"`+candidate+`","replay_attempt_id":"`+replay.Attempts[2].ID+`","required_check_run_ids":["`+run.ID+`"]}`, http.StatusCreated, &repair)
	var candidateWorkspace workspaces.Workspace
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/workspaces", owner, `{"revision":"`+candidate+`","source_context":{"type":"repository"}}`, http.StatusCreated, &candidateWorkspace)
	replay = attempt(owner, candidate, "repair_verification", false, candidateWorkspace.ID)
	fixedAttempt := replay.Attempts[len(replay.Attempts)-1]
	workflowJSON(t, server.URL, http.MethodPost, repairBase+"/verifications", owner, `{"pull_request_id":"`+pull.ID+`","revision":"`+candidate+`","replay_attempt_id":"`+fixedAttempt.ID+`","required_check_run_ids":["`+run.ID+`"]}`, http.StatusCreated, &repair)
	if repair.Verifications[0].Passed || !repair.Verifications[1].Passed {
		t.Fatalf("failed-first repair containment missing: %#v", repair.Verifications)
	}
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", owner, `{}`, http.StatusOK, &pull)
	var fixed releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/releases", owner, `{"version":"v1.0.1","commit_id":"`+pull.MergeCommitID+`","prior_release_id":"`+affected.ID+`","notes":"Reviewed replay-verified retry repair"}`, http.StatusCreated, &fixed)
	build, artifact := waitForReleaseArtifact(t, server.URL, string(repo.ID), fixed.ID, owner)
	var environment deployments.Environment
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/environments", owner, `{"name":"production","position":1,"command":"printf deployed","required_approvals":1,"concurrency":1}`, http.StatusCreated, &environment)
	deployed := promoteAndApprove(t, server.URL, string(repo.ID), responder, owner, environment.ID, fixed.ID, build.ID, artifact.ID)
	deployed = waitForDeployment(t, server.URL, string(repo.ID), deployed.ID, owner, "succeeded")
	validate := func(stage string, passed bool, action string) {
		workflowJSON(t, server.URL, http.MethodPost, repairBase+"/validations", owner, `{"deployment_id":"`+deployed.ID+`","release_id":"`+fixed.ID+`","revision":"`+fixed.CommitID+`","stage":"`+stage+`","signals":[{"name":"second retry success","evidence_id":"sanitized-signal-`+stage+`","original_behavior":"second retry rejected","observed_value":"`+map[bool]string{true: "retry succeeds", false: "retry still rejected"}[passed]+`","passed":`+map[bool]string{true: "true", false: "false"}[passed]+`}],"failure_action":"`+action+`","rationale":"compare the consented journey using aggregate production signal"}`, http.StatusCreated, &repair)
	}
	validate("one-percent", false, "pause")
	validate("five-percent", true, "")
	validate("production", true, "")
	if repair.State != "production_validated" || repair.Validations[0].RequiredAction != "pause" || len(repair.Authority) != 0 || pull.MergeCommitID == "" {
		t.Fatalf("confirmed outcome trail incomplete: %#v", repair)
	}
}
