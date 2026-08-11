package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestReportToVerifiedResolutionWorkflow proves that retained user evidence can
// move through human retry, scoped agent diagnosis and repair, ordinary review,
// release and deployment, then pass unchanged against the delivered release.
func TestReportToVerifiedResolutionWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the issue resolution workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	issueStore, _ := issues.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	deploymentStore, _ := deployments.New(t.TempDir())
	checkRunner := checkruns.NewRunner(checks, catalog)
	reproductionRunner := issues.NewReproductionRunner(issueStore, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerIssuesHTTP(mux, issueStore, releaseStore, catalog, credentials, reproductionRunner)
	registerIssueRepairsHTTP(mux, issueStore, plans, pulls, catalog, credentials, reproductionRunner, checks)
	registerProposalTaskSessionsHTTP(mux, plans, sessions, catalog, credentials, nil, pulls, checkRunner)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, nil, checkRunner, checks)
	registerCheckRunsHTTP(mux, checks, checkRunner, pulls, catalog, credentials, sessions, nil)
	registerReleasesHTTP(mux, releaseStore, checks, checkRunner, pulls, catalog, credentials)
	registerDeploymentsHTTP(mux, deploymentStore, releaseStore, checks, catalog, credentials, nil, sessions, pulls)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	reporter := issueAccess(t, credentials, "reporter", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"reproducible-service","visibility":"public"}`, http.StatusCreated, &repository)
	if _, err := catalog.AddCollaborator("maintainer", storage.ID(repository.ID), "reporter"); err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/required-checks", maintainer, `{"branch":"main","checks":["behavior-contract"]}`, http.StatusOK, nil)
	remoteURL, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
	remoteURL.User = url.UserPassword("git", maintainerGit)
	work := gitClone(t, remoteURL.String())
	gitOutput(t, work, "config", "user.name", "Maintainer")
	gitOutput(t, work, "config", "user.email", "maintainer@example.com")
	writeWorkflowFile(t, work, "behavior.txt", "broken\n")
	writeWorkflowFile(t, work, ".komodo/checks.json", `{"version":1,"checks":[{"name":"behavior-contract","command":"test -f behavior.txt","timeout_seconds":30}]}`)
	writeWorkflowFile(t, work, ".komodo/releases.json", `{"version":1,"builds":[{"name":"package","command":"mkdir -p dist; cp behavior.txt dist/service","artifacts":["dist/service"]}]}`)
	writeWorkflowFile(t, work, ".komodo/deployments.json", `{"version":1,"environments":[{"name":"production","stages":[{"name":"rollout","health":[{"name":"artifact-readable","command":"test -s \"$KOMODO_ARTIFACT_PATH\""}]}]}]}`)
	writeWorkflowFile(t, work, ".komodo/reproductions.json", `{"version":1,"environment":"linux","resources":{"cpu_seconds":10,"memory_mb":128,"disk_mb":128},"reproductions":[{"name":"reported-case","command":"test -f .komodo-inputs/case.txt && grep -qx broken behavior.txt","timeout_seconds":10,"expected_exit_code":0}]}`)
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Release broken behavior")
	brokenRevision := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "main")

	var affected releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", maintainer, `{"version":"v1.0.0","commit_id":"`+brokenRevision+`","notes":"Affected release"}`, http.StatusCreated, &affected)
	badBuild, badArtifact := waitForReleaseArtifact(t, server.URL, repository.ID, affected.ID, maintainer)
	var environment deployments.Environment
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/environments", maintainer, `{"name":"production","position":1,"command":"printf deployed","required_approvals":1,"concurrency":1}`, http.StatusCreated, &environment)
	badDeployment := promoteAndApprove(t, server.URL, repository.ID, reporter, maintainer, environment.ID, affected.ID, badBuild.ID, badArtifact.ID)
	waitForDeployment(t, server.URL, repository.ID, badDeployment.ID, maintainer, "succeeded")

	issueBase := "/repositories/" + repository.ID + "/issues"
	var item issues.Issue
	workflowJSON(t, server.URL, http.MethodPost, issueBase, reporter, `{"title":"Released service returns broken behavior","expected_behavior":"The service returns fixed behavior","observed_behavior":"The service returns broken behavior","severity":"high","environment":"production v1.0.0","reproduction_steps":["submit the retained case","inspect behavior"],"affected_release_id":"`+affected.ID+`","visibility":"public","attachments":[{"kind":"log","name":"response.log","media_type":"text/plain","content":"YnJva2VuCg=="}]}`, http.StatusCreated, &item)
	base := issueBase + "/" + item.ID

	// Missing safe input is retained as non-reproducible; the reporter requests
	// and supplies only the missing fixture instead of allowing false closure.
	var missing issues.ReproductionAttempt
	workflowJSON(t, server.URL, http.MethodPost, base+"/reproductions", maintainer, `{"name":"reported-case","inputs":[]}`, http.StatusAccepted, &missing)
	missing = waitForIssueReproduction(t, server.URL, base, missing.ID, maintainer)
	if missing.Reproduced || missing.State != "failed" {
		t.Fatalf("missing-input attempt was not retained as non-reproducible: %#v", missing)
	}
	var firstInvestigation struct {
		Investigation issues.Investigation `json:"investigation"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/investigations", maintainer, `{"reproduction_id":"`+missing.ID+`"}`, http.StatusCreated, &firstInvestigation)
	workflowJSON(t, server.URL, http.MethodPost, base+"/investigations/"+firstInvestigation.Investigation.ID+"/entries", reporter, `{"kind":"evidence_request","body":"Please retry with the sanitized case fixture; no credential or customer data is required.","citations":[{"kind":"reproduction_event","resource_id":"`+missing.ID+`","event_sequence":1}]}`, http.StatusCreated, nil)
	fixture := base64.StdEncoding.EncodeToString([]byte("safe reported case\n"))
	var reproduced issues.ReproductionAttempt
	workflowJSON(t, server.URL, http.MethodPost, base+"/reproductions", reporter, `{"name":"reported-case","inputs":[{"name":"case.txt","media_type":"text/plain","content":"`+fixture+`"}]}`, http.StatusAccepted, &reproduced)
	reproduced = waitForIssueReproduction(t, server.URL, base, reproduced.ID, reporter)
	if !reproduced.Reproduced || reproduced.State != "completed" {
		t.Fatalf("retry did not confirm the report: %#v", reproduced)
	}
	workflowJSON(t, server.URL, http.MethodPut, base+"/triage", maintainer, `{"expected_version":`+workflowInt(item.Version+2)+`,"classification":"regression","priority":"urgent","assignee_ids":["maintainer"],"labels":["release"]}`, http.StatusOK, &item)

	var diagnosis struct {
		Investigation issues.Investigation `json:"investigation"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/investigations", reporter, `{"reproduction_id":"`+reproduced.ID+`"}`, http.StatusCreated, &diagnosis)
	var delegated struct {
		WorkerCredential string `json:"worker_credential"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/investigations/"+diagnosis.Investigation.ID+"/agent-runs", maintainer, `{"agent_id":"codex"}`, http.StatusCreated, &delegated)
	var conclusionResponse struct {
		Entry issues.InvestigationEntry `json:"entry"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/issue-investigation-agent/entries", delegated.WorkerCredential, `{"kind":"conclusion","body":"The released behavior marker is the reproduced regression.","citations":[{"kind":"reproduction_event","resource_id":"`+reproduced.ID+`","event_sequence":1}],"suspected_revisions":["`+brokenRevision+`"],"suspected_owner_ids":["maintainer"]}`, http.StatusCreated, &conclusionResponse)

	var repairResponse struct {
		Repair issues.Repair  `json:"repair"`
		Task   proposals.Task `json:"task"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/repairs", maintainer, `{"reproduction_id":"`+reproduced.ID+`","investigation_id":"`+diagnosis.Investigation.ID+`","conclusion_entry_id":"`+conclusionResponse.Entry.ID+`","acceptance_criteria":["The retained case no longer reproduces","Required checks pass"],"owner_kind":"agent","owner_id":"codex"}`, http.StatusCreated, &repairResponse)
	planBase := "/repositories/" + repository.ID + "/proposals/" + repairResponse.Repair.ProposalID + "/plan/tasks/" + repairResponse.Task.ID
	var started struct {
		Session    changesessions.Session         `json:"session"`
		Run        changesessions.Run             `json:"run"`
		Credential struct{ Token, Branch string } `json:"credential"`
	}
	workflowJSON(t, server.URL, http.MethodPost, planBase+"/change-sessions", maintainer, `{"expected_assignment_id":"`+repairResponse.Task.Assignment.ID+`"}`, http.StatusCreated, &started)
	runBase := planBase + "/change-sessions/" + started.Session.ID + "/runs/" + started.Run.ID
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/events", started.Credential.Token, `{"type":"run.started","metadata":{"status":"Repairing the retained reproduction"}}`, http.StatusCreated, nil)
	agentURL, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
	agentURL.User = url.UserPassword("git", started.Credential.Token)
	agentWork := gitClone(t, agentURL.String())
	gitOutput(t, agentWork, "config", "user.name", "Codex Agent")
	gitOutput(t, agentWork, "config", "user.email", "codex@agents.local")
	gitOutput(t, agentWork, "switch", strings.TrimPrefix(started.Credential.Branch, "refs/heads/"))
	writeWorkflowFile(t, agentWork, "behavior.txt", "fixed\n")
	gitOutput(t, agentWork, "commit", "-am", "Repair reported behavior")
	fixRevision := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push", "origin", strings.TrimPrefix(started.Credential.Branch, "refs/heads/"))
	var contribution struct {
		Pull pullrequests.PullRequest `json:"pull_request"`
	}
	workflowJSON(t, server.URL, http.MethodPost, planBase+"/contributions", started.Credential.Token, `{"expected_assignment_id":"`+repairResponse.Task.Assignment.ID+`","session_id":"`+started.Session.ID+`","title":"Repair reported behavior","target_branch":"main"}`, http.StatusCreated, &contribution)
	waitForWorkflowCheck(t, server.URL, "/repositories/"+repository.ID+"/pull-requests/"+contribution.Pull.ID, reporter, fixRevision, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPost, base+"/repairs/"+repairResponse.Repair.ID+"/pull-request", maintainer, `{"pull_request_id":"`+contribution.Pull.ID+`"}`, http.StatusOK, nil)
	var verification issues.RepairVerification
	workflowJSON(t, server.URL, http.MethodPost, base+"/repairs/"+repairResponse.Repair.ID+"/verifications", reporter, `{}`, http.StatusCreated, &verification)
	verificationEvidence := waitForIssueVerification(t, server.URL, base, repairResponse.Repair.ID, verification.ID, reporter)
	digest := verificationEvidence["evidence_digest"].(string)
	workflowJSON(t, server.URL, http.MethodPost, base+"/repairs/"+repairResponse.Repair.ID+"/verifications/"+verification.ID+"/decisions", reporter, `{"kind":"confirmed","evidence_digest":"`+digest+`"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/pull-requests/"+contribution.Pull.ID+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
	var merged pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests/"+contribution.Pull.ID+"/merge", maintainer, `{}`, http.StatusOK, &merged)

	var fixed releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", maintainer, `{"version":"v1.0.1","commit_id":"`+merged.MergeCommitID+`","prior_release_id":"`+affected.ID+`","notes":"Verified issue repair"}`, http.StatusCreated, &fixed)
	fixedBuild, fixedArtifact := waitForReleaseArtifact(t, server.URL, repository.ID, fixed.ID, maintainer)
	fixedDeployment := promoteAndApprove(t, server.URL, repository.ID, reporter, maintainer, environment.ID, fixed.ID, fixedBuild.ID, fixedArtifact.ID)
	fixedDeployment = waitForDeployment(t, server.URL, repository.ID, fixedDeployment.ID, maintainer, "succeeded")
	var deliveredAttempt issues.ReproductionAttempt
	workflowJSON(t, server.URL, http.MethodPost, base+"/reproductions", reporter, `{"name":"reported-case","release_id":"`+fixed.ID+`","inputs":[{"name":"case.txt","media_type":"text/plain","content":"`+fixture+`"}]}`, http.StatusAccepted, &deliveredAttempt)
	deliveredAttempt = waitForIssueReproduction(t, server.URL, base, deliveredAttempt.ID, reporter)
	if deliveredAttempt.Reproduced || deliveredAttempt.Revision != fixed.CommitID || deliveredAttempt.ReleaseID != fixed.ID {
		t.Fatalf("fixed release did not retain the passing original case: %#v", deliveredAttempt)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/relationships", maintainer, `{"kind":"release","resource_id":"`+fixed.ID+`","repository_id":"`+repository.ID+`","revision":"`+fixed.CommitID+`","note":"fixed release retest passed"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/relationships", maintainer, `{"kind":"deployment","resource_id":"`+fixedDeployment.ID+`","repository_id":"`+repository.ID+`","revision":"`+fixed.CommitID+`","note":"verified production delivery"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPatch, base, reporter, `{"status":"closed"}`, http.StatusOK, &item)
	if item.Status != "closed" || len(item.Repairs) != 1 || len(item.Relationships) < 3 || item.History[len(item.History)-1].Type != "status.closed" {
		t.Fatalf("resolved issue lost its permission-aware delivery trail: %#v", item)
	}
}

func waitForIssueReproduction(t *testing.T, origin, base, id, actor string) issues.ReproductionAttempt {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var attempt issues.ReproductionAttempt
		workflowJSON(t, origin, http.MethodGet, base+"/reproductions/"+id, actor, "", http.StatusOK, &attempt)
		if attempt.State != "queued" && attempt.State != "running" {
			return attempt
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("issue reproduction did not finish")
	return issues.ReproductionAttempt{}
}

func waitForIssueVerification(t *testing.T, origin, base, repair, verification, actor string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var evidence map[string]any
		workflowJSON(t, origin, http.MethodGet, base+"/repairs/"+repair+"/verifications/"+verification, actor, "", http.StatusOK, &evidence)
		if evidence["state"] == "ready_for_reporter" {
			return evidence
		}
		if evidence["state"] == "failed" || evidence["state"] == "invalid" {
			t.Fatalf("issue verification failed: %#v", evidence)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("issue verification did not finish")
	return nil
}
