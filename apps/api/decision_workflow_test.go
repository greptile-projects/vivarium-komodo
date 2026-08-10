package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

// TestUncertaintyToMeasuredDecisionWorkflow is the complete regression boundary
// for evidence-driven decisions. Every collaboration action crosses public HTTP
// or stock Git, preserving the alternatives and evidence after delivery causes
// the team to revisit its choice.
func TestUncertaintyToMeasuredDecisionWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is required for the decision workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	decisionStore, _ := decisions.New(t.TempDir())
	workspaceStore, _ := workspaces.New(t.TempDir())
	incidentStore, _ := incidents.New(t.TempDir())
	organizationStore, _ := organizations.New(t.TempDir())
	checkRunner := checkruns.NewRunner(checks, catalog)
	workspaceRunner := workspaces.NewRunner(workspaceStore, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerProposalTaskSessionsHTTP(mux, plans, sessions, catalog, credentials, nil, pulls, checkRunner)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, nil, checkRunner, checks)
	registerCheckRunsHTTP(mux, checks, checkRunner, pulls, catalog, credentials, sessions, nil)
	registerReleasesHTTP(mux, releaseStore, checks, checkRunner, pulls, catalog, credentials)
	registerWorkspacesHTTP(mux, workspaceStore, workspaceRunner, catalog, credentials, plans, pulls, incidentStore, organizationStore, checkRunner)
	registerDecisionsHTTP(mux, decisionStore, catalog, credentials, workspaceStore, workspaceRunner, plans)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	developer := issueAccess(t, credentials, "developer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	developerGit := issueAccess(t, credentials, "developer", auth.Git, auth.GitRead, auth.GitWrite)
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"decision-service","visibility":"private"}`, http.StatusCreated, &repository)
	if _, err := catalog.AddCollaborator("maintainer", repository.ID, "developer"); err != nil {
		t.Fatal(err)
	}
	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		value.User = url.UserPassword("git", token)
		return value.String()
	}
	clone := gitClone(t, remote(maintainerGit))
	gitOutput(t, clone, "config", "user.name", "Maintainer")
	gitOutput(t, clone, "config", "user.email", "maintainer@example.com")
	writeWorkflowFile(t, clone, "README.md", "# Decision service\n\nThe cache strategy is undecided.\n")
	writeWorkflowFile(t, clone, ".komodo/checks.json", `{"version":1,"checks":[{"name":"decision","command":"test -f README.md","timeout_seconds":30}]}`)
	writeWorkflowFile(t, clone, ".komodo/releases.json", `{"version":1,"builds":[{"name":"decision-record","command":"mkdir -p dist; cp README.md dist/README.md","artifacts":["dist/README.md"]}]}`)
	writeWorkflowFile(t, clone, ".komodo/workspaces.json", `{"version":1,"tools":[{"name":"sh","version":"system"}],"dependencies":["repository snapshot"],"setup":["true"],"commands":[{"name":"prototype-memory","command":"printf 'memory p95=18ms\\n' > prototype.txt"},{"name":"prototype-redis","command":"printf 'redis p95=24ms\\n' > prototype.txt"}],"resources":{"cpu_seconds":30,"memory_mb":128,"disk_mb":128,"setup_timeout_seconds":30}}`)
	gitOutput(t, clone, "add", ".")
	gitOutput(t, clone, "commit", "-m", "Capture cache uncertainty")
	baseRevision := gitOutput(t, clone, "rev-parse", "HEAD")
	gitOutput(t, clone, "push", "-u", "origin", "main")

	observed := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	decisionBase := "/repositories/" + string(repository.ID) + "/decisions"
	var decision decisions.Decision
	create := `{"title":"Choose the request cache","context":{"kind":"repository","id":"` + string(repository.ID) + `"},"question":"Which cache meets latency goals without hiding failure modes?","constraints":["Preserve explicit fallback behavior"],"success_measures":["Production p95 remains below 25ms"],"affected_resources":[{"kind":"code","repository_id":"` + string(repository.ID) + `","ref":"` + baseRevision + `","path":"README.md","label":"request path"}],"participant_ids":["maintainer","developer"],"owner_id":"maintainer","change_summary":"Open the consequential cache choice"}`
	workflowJSON(t, server.URL, http.MethodPost, decisionBase, maintainer, create, http.StatusCreated, &decision)
	decisionBase += "/" + decision.ID
	evidence := `{"kind":"code","repository_id":"` + string(repository.ID) + `","revision":"` + baseRevision + `","path":"README.md","url":"/repositories/` + string(repository.ID) + `?ref=` + baseRevision + `","summary":"Exact undecided request path","observed_at":"` + observed + `"}`
	claims := func(assumption, tradeoff, risk, compatibility, cost, outcome string) string {
		return `[{"kind":"assumption","body":"` + assumption + `"},{"kind":"tradeoff","body":"` + tradeoff + `"},{"kind":"risk","body":"` + risk + `"},{"kind":"compatibility","body":"` + compatibility + `"},{"kind":"cost","body":"` + cost + `"},{"kind":"outcome","body":"` + outcome + `"}]`
	}
	var memory decisions.Decision
	workflowJSON(t, server.URL, http.MethodPost, decisionBase+"/alternatives", developer, `{"title":"In-process cache","claims":`+claims("working sets fit in memory", "fast but node-local", "cold restarts", "no protocol change", "low operating cost", "p95 below 20ms")+`,"evidence":[`+evidence+`]}`, http.StatusCreated, &memory)
	memoryAlternative := memory.Alternatives[0]
	var redis decisions.Decision
	workflowJSON(t, server.URL, http.MethodPost, decisionBase+"/alternatives", maintainer, `{"title":"Shared Redis cache","claims":`+claims("network remains reliable", "shared but remote", "network partitions", "existing Redis protocol", "managed service cost", "p95 below 25ms")+`,"evidence":[`+evidence+`]}`, http.StatusCreated, &redis)
	redisAlternative := redis.Alternatives[1]
	workflowJSON(t, server.URL, http.MethodPost, decisionBase+"/alternatives/"+redisAlternative.ID+"/claims", developer, `{"claims":[{"kind":"dissent","body":"A remote dependency expands the failure surface."}],"evidence":[`+evidence+`]}`, http.StatusCreated, &redis)
	var research struct {
		Decision         decisions.Decision `json:"decision"`
		WorkerCredential string             `json:"worker_credential"`
	}
	workflowJSON(t, server.URL, http.MethodPost, decisionBase+"/alternatives/"+memoryAlternative.ID+"/agent-runs", maintainer, `{}`, http.StatusCreated, &research)
	workflowJSON(t, server.URL, http.MethodGet, "/decision-research-agent/context", research.WorkerCredential, "", http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, "/decision-research-agent/findings", research.WorkerCredential, `{"body":"The exact snapshot supports a bounded in-process prototype.","uncertainty":"Production working-set size is not yet observed.","evidence":[`+evidence+`]}`, http.StatusCreated, &decision)

	startExperiment := func(alternative, command, reproduces string) (decisions.Experiment, workspaces.Workspace) {
		body := `{"revision":"` + baseRevision + `","command_name":"` + command + `","dependency_digest":"deps-v1"`
		if reproduces != "" {
			body += `,"reproduces_experiment_id":"` + reproduces + `"`
		}
		body += `}`
		var started struct {
			Experiment decisions.Experiment `json:"experiment"`
			Workspace  workspaces.Workspace `json:"workspace"`
		}
		workflowJSON(t, server.URL, http.MethodPost, decisionBase+"/alternatives/"+alternative+"/experiments", developer, body, http.StatusCreated, &started)
		workspaceBase := "/repositories/" + string(repository.ID) + "/workspaces/" + started.Workspace.ID
		started.Workspace = waitForDecisionExperiment(t, server.URL, workspaceBase, developer)
		var checkpoint workspaces.Checkpoint
		workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/checkpoints", developer, `{"summary":"Retain prototype output","paths":["prototype.txt"],"reproducibility":{"dependencies":["repository snapshot"],"commands":["`+command+`"]}}`, http.StatusCreated, &checkpoint)
		workflowJSON(t, server.URL, http.MethodPost, decisionBase+"/alternatives/"+alternative+"/experiments/"+started.Experiment.ID+"/checkpoints", developer, `{"workspace_checkpoint_id":"`+checkpoint.ID+`","summary":"Measured `+command+`","measurements":[{"name":"p95","value":20,"unit":"ms"}],"artifact_paths":["prototype.txt"]}`, http.StatusCreated, &decision)
		return started.Experiment, started.Workspace
	}
	memoryExperiment, _ := startExperiment(memoryAlternative.ID, "prototype-memory", "")
	startExperiment(memoryAlternative.ID, "prototype-memory", memoryExperiment.ID)
	startExperiment(redisAlternative.ID, "prototype-redis", "")

	var approved decisions.Decision
	workflowJSON(t, server.URL, http.MethodPost, decisionBase+"/approval-requirements", maintainer, `{"kind":"acknowledgement","actor_id":"developer"}`, http.StatusCreated, &approved)
	requirement := approved.ApprovalRequirements[0]
	workflowJSON(t, server.URL, http.MethodPost, decisionBase+"/approval-requirements/"+requirement.ID+"/responses", developer, `{"response":"acknowledged","note":"The prototype is reproducible; retain my remote-cache dissent."}`, http.StatusOK, &approved)
	commitment := `{"selected_alternative_id":"` + memoryAlternative.ID + `","rejected_alternative_ids":["` + redisAlternative.ID + `"],"rationale":"The reproduced local prototype is faster and has an explicit fallback.","accepted_tradeoffs":["Node-local state is discarded on restart"],"dissent":["Remote-cache advocates expect better cross-node hit rates"],"conditions":["Record the fallback in operator documentation"],"evidence_considered":[` + evidence + `]}`
	workflowJSON(t, server.URL, http.MethodPost, decisionBase+"/commitments", maintainer, commitment, http.StatusCreated, &decision)

	var delivery struct {
		Decision decisions.Decision `json:"decision"`
		Proposal proposals.Proposal `json:"proposal"`
		Tasks    []proposals.Task   `json:"tasks"`
	}
	deliveryBody := `{"title":"Deliver the cache decision","body":"Implement the accepted choice with human and agent ownership.","base_revision":"` + baseRevision + `","tasks":[{"title":"Document fallback","outcome":"Human-authored fallback is explicit","owner_kind":"human","owner_id":"developer","completion_criteria":["Preserve explicit fallback behavior"],"verification_plan":["Review FALLBACK.md"]},{"title":"Implement measured cache","outcome":"Agent-authored cache note records the latency target","owner_kind":"codex","owner_id":"codex","completion_criteria":["Production p95 remains below 25ms","Record the fallback in operator documentation"],"verification_plan":["Run decision check"],"depends_on":[1]}]}`
	workflowJSON(t, server.URL, http.MethodPost, decisionBase+"/delivery", maintainer, deliveryBody, http.StatusCreated, &delivery)
	planBase := "/repositories/" + string(repository.ID) + "/proposals/" + delivery.Proposal.ID + "/plan"
	human := delivery.Tasks[0]
	workflowJSON(t, server.URL, http.MethodPut, planBase+"/tasks/"+human.ID+"/assignment", maintainer, `{"kind":"human","assignee_id":"developer","mandate":"Document only the accepted fallback.","base_revision":"`+baseRevision+`"}`, http.StatusOK, &human)
	humanClone := gitClone(t, remote(developerGit))
	gitOutput(t, humanClone, "config", "user.name", "Human Developer")
	gitOutput(t, humanClone, "config", "user.email", "developer@example.com")
	gitOutput(t, humanClone, "switch", "-c", "decision/fallback")
	writeWorkflowFile(t, humanClone, "FALLBACK.md", "# Fallback\n\nBypass the cache when local state is unavailable.\n")
	gitOutput(t, humanClone, "add", "FALLBACK.md")
	gitOutput(t, humanClone, "commit", "-m", "Document cache fallback")
	gitOutput(t, humanClone, "push", "-u", "origin", "decision/fallback")
	var humanPublication struct {
		Pull pullrequests.PullRequest `json:"pull_request"`
	}
	humanAccount := `{"expected_assignment_id":"` + human.Assignment.ID + `","title":"Document cache fallback","source_branch":"decision/fallback","target_branch":"main","delivery_evidence":{"reasoning":"The accepted constraint requires an explicit bypass.","commands":["test -f FALLBACK.md"],"completion_criteria":[{"criterion":"Preserve explicit fallback behavior","status":"met","evidence":"FALLBACK.md documents the bypass."}]}}`
	workflowJSON(t, server.URL, http.MethodPost, planBase+"/tasks/"+human.ID+"/contributions", developer, humanAccount, http.StatusCreated, &humanPublication)
	humanMerge := landDecisionPull(t, server.URL, string(repository.ID), maintainer, humanPublication.Pull)

	var plan proposals.Plan
	workflowJSON(t, server.URL, http.MethodGet, planBase, maintainer, "", http.StatusOK, &plan)
	agent := orchestrationTask(t, plan, delivery.Tasks[1].ID)
	workflowJSON(t, server.URL, http.MethodPut, planBase+"/tasks/"+agent.ID+"/assignment", maintainer, `{"kind":"agent","assignee_id":"codex","mandate":"Implement the measured accepted cache note.","base_revision":"`+humanMerge.MergeCommitID+`"}`, http.StatusOK, &agent)
	var started struct {
		Session    changesessions.Session         `json:"session"`
		Run        changesessions.Run             `json:"run"`
		Credential struct{ Token, Branch string } `json:"credential"`
	}
	agentBase := planBase + "/tasks/" + agent.ID
	workflowJSON(t, server.URL, http.MethodPost, agentBase+"/change-sessions", maintainer, `{"expected_assignment_id":"`+agent.Assignment.ID+`"}`, http.StatusCreated, &started)
	workflowJSON(t, server.URL, http.MethodPost, agentBase+"/change-sessions/"+started.Session.ID+"/runs/"+started.Run.ID+"/events", started.Credential.Token, `{"type":"run.started","metadata":{"status":"Implementing the accepted decision"}}`, http.StatusCreated, nil)
	agentClone := gitClone(t, remote(started.Credential.Token))
	gitOutput(t, agentClone, "config", "user.name", "Codex Agent")
	gitOutput(t, agentClone, "config", "user.email", "codex@agents.local")
	agentBranch := strings.TrimPrefix(started.Credential.Branch, "refs/heads/")
	gitOutput(t, agentClone, "switch", agentBranch)
	assertFile(t, filepath.Join(agentClone, "FALLBACK.md"), "# Fallback\n\nBypass the cache when local state is unavailable.\n", 0)
	writeWorkflowFile(t, agentClone, "CACHE.md", "# In-process cache\n\nTarget production p95: below 25ms. See FALLBACK.md for bypass behavior.\n")
	gitOutput(t, agentClone, "add", "CACHE.md")
	gitOutput(t, agentClone, "commit", "-m", "Implement accepted cache decision")
	agentRevision := gitOutput(t, agentClone, "rev-parse", "HEAD")
	gitOutput(t, agentClone, "push", "origin", agentBranch)
	var agentPublication struct {
		Pull pullrequests.PullRequest `json:"pull_request"`
	}
	agentAccount := `{"expected_assignment_id":"` + agent.Assignment.ID + `","session_id":"` + started.Session.ID + `","title":"Implement measured cache","target_branch":"main","delivery_evidence":{"reasoning":"The reproduced local alternative meets the committed latency target.","commands":["test -f CACHE.md"],"completion_criteria":[{"criterion":"Production p95 remains below 25ms","status":"met","evidence":"The prototype retained a sub-25ms p95 measurement."},{"criterion":"Record the fallback in operator documentation","status":"met","evidence":"CACHE.md links the human-authored FALLBACK.md."}]}}`
	workflowJSON(t, server.URL, http.MethodPost, agentBase+"/contributions", started.Credential.Token, agentAccount, http.StatusCreated, &agentPublication)
	if agentPublication.Pull.SourceCommitID != agentRevision || agentPublication.Pull.ReasoningContext == nil || agentPublication.Pull.ReasoningContext.DecisionID != decision.ID {
		t.Fatalf("agent review lost decision provenance: %#v", agentPublication.Pull)
	}
	agentMerge := landDecisionPull(t, server.URL, string(repository.ID), maintainer, agentPublication.Pull)
	var release releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/releases", maintainer, `{"version":"v1.0.0","commit_id":"`+agentMerge.MergeCommitID+`","notes":"Deliver the measured cache decision."}`, http.StatusCreated, &release)
	build, artifact := waitForReleaseArtifact(t, server.URL, string(repository.ID), release.ID, maintainer)
	revisit := `{"kind":"changed_assumption","summary":"Production working-set growth disproved the retained in-memory sizing assumption after release ` + release.ID + `.","evidence_url":"/repositories/` + string(repository.ID) + `/releases/` + release.ID + `/builds/` + build.ID + `/artifacts/` + artifact.ID + `"}`
	workflowJSON(t, server.URL, http.MethodPost, decisionBase+"/revisit-requests", developer, revisit, http.StatusCreated, &decision)
	if decision.State != "reopened" || len(decision.Commitments) != 1 || len(decision.Deliveries) != 1 || len(decision.RevisitRequests) != 1 || len(decision.Alternatives) != 2 || len(decision.Alternatives[0].Experiments) != 2 || len(decision.Alternatives[1].Experiments) != 1 {
		t.Fatalf("revisit lost original evidence or delivered outcomes: %#v", decision)
	}
	decisionReleaseLink := release.PullRequests[len(release.PullRequests)-1]
	if decision.Commitments[0].Approvals[0].ActorID != "developer" || len(decision.Commitments[0].Dissent) != 1 || decisionReleaseLink.DecisionID != decision.ID || decisionReleaseLink.DecisionVersion != 1 {
		t.Fatalf("approval, dissent, or release attribution was lost: decision=%#v release=%#v", decision, release)
	}
}

func waitForDecisionExperiment(t *testing.T, origin, path, token string) workspaces.Workspace {
	t.Helper()
	var item workspaces.Workspace
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		workflowJSON(t, origin, http.MethodGet, path, token, "", http.StatusOK, &item)
		if item.State == workspaces.Ready {
			for _, event := range item.Activity {
				if event.Kind == "execution" && event.ExitCode != nil {
					return item
				}
			}
		}
		if item.State == workspaces.Failed {
			t.Fatalf("decision experiment failed: %#v", item.Activity)
		}
	}
	t.Fatalf("decision experiment did not finish: %#v", item.Activity)
	return item
}

func landDecisionPull(t *testing.T, origin, repositoryID, maintainer string, pull pullrequests.PullRequest) pullrequests.PullRequest {
	t.Helper()
	base := "/repositories/" + repositoryID + "/pull-requests/" + pull.ID
	waitForWorkflowCheck(t, origin, base, maintainer, pull.SourceCommitID, checkruns.Succeeded)
	workflowJSON(t, origin, http.MethodPut, base+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
	var merged pullrequests.PullRequest
	workflowJSON(t, origin, http.MethodPost, base+"/merge", maintainer, `{}`, http.StatusOK, &merged)
	return merged
}
