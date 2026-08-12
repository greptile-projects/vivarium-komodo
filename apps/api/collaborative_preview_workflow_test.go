package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestCollaborativePreviewToReleaseWorkflow proves that an affected person can
// evaluate exact candidate revisions without repository access, retain safe
// feedback through an agent repair, and provide acceptance that gates delivery.
// All workflow transitions use public HTTP, credential-bound worker APIs, and
// stock Git; the test also retains failed setup, expired access, and stale
// acceptance as bounded recovery evidence.
func TestCollaborativePreviewToReleaseWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the collaborative preview workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	previewStore, _ := previews.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	checkRunner := checkruns.NewRunner(checks, catalog)
	previewRunner := previews.NewRunner(previewStore, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, nil, checkRunner, checks, previewStore)
	registerChangeSessionsHTTP(mux, sessions, pulls, catalog, credentials, nil, checkRunner)
	registerCheckRunsHTTP(mux, checks, checkRunner, pulls, catalog, credentials, sessions, nil)
	registerPreviewsHTTP(mux, previewStore, previewRunner, pulls, catalog, credentials, previewSources{}, previewRepairStores{plans: plans, sessions: sessions})
	registerReleasesHTTP(mux, releaseStore, checks, checkRunner, pulls, catalog, credentials)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.ProfileRead, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	contributor := issueAccess(t, credentials, "contributor", auth.API, auth.ProfileRead, auth.RepositoryRead, auth.RepositoryWrite)
	contributorGit := issueAccess(t, credentials, "contributor", auth.Git, auth.GitRead, auth.GitWrite)
	stakeholder := issueAccess(t, credentials, "stakeholder", auth.API, auth.ProfileRead)
	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"collaborative-preview","visibility":"private"}`, http.StatusCreated, &repository)
	if _, err := catalog.AddCollaborator("maintainer", storage.ID(repository.ID), "contributor"); err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/required-checks", maintainer, `{"branch":"main","checks":["experience"]}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/preview-acceptance-requirements", maintainer, `{"requirements":[{"id":"stakeholder-experience","target_branches":["main"],"paths":["site/**"],"risk_classes":["experience"],"scenarios":[{"id":"submit-copy","description":"The primary action is clear to affected users.","required_roles":["feedback"]}]}]}`, http.StatusOK, nil)

	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		value.User = url.UserPassword("git", token)
		return value.String()
	}
	ownerClone := gitClone(t, remote(maintainerGit))
	gitOutput(t, ownerClone, "config", "user.name", "Maintainer")
	gitOutput(t, ownerClone, "config", "user.email", "maintainer@example.com")
	writeWorkflowFile(t, ownerClone, "README.md", "# Collaborative previews\n")
	writeWorkflowFile(t, ownerClone, ".komodo/releases.json", `{"version":1,"builds":[{"name":"site","command":"mkdir -p dist; cp site/index.html dist/index.html","artifacts":["dist/index.html"]}]}`)
	gitOutput(t, ownerClone, "add", ".")
	gitOutput(t, ownerClone, "commit", "-m", "Initialize preview delivery")
	gitOutput(t, ownerClone, "push", "-u", "origin", "main")

	var proposal proposals.Proposal
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/proposals", contributor, `{"title":"Let stakeholders prove the submit experience","body":"Preview the exact change with affected users before delivery."}`, http.StatusCreated, &proposal)
	work := gitClone(t, remote(contributorGit))
	gitOutput(t, work, "config", "user.name", "Contributor")
	gitOutput(t, work, "config", "user.email", "contributor@example.com")
	gitOutput(t, work, "switch", "-c", "experience/submit")
	writeWorkflowFile(t, work, "site/index.html", "<button>Do it</button>\n")
	writeWorkflowFile(t, work, ".komodo/previews.json", previewManifest("test -f site/build-ready"))
	writeWorkflowFile(t, work, ".komodo/checks.json", `{"version":1,"checks":[{"name":"experience","command":"grep -q 'Submit changes' site/index.html","timeout_seconds":30}]}`)
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Propose submit experience")
	gitOutput(t, work, "push", "-u", "origin", "experience/submit")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests", contributor, `{"title":"Preview the submit experience","body":"Stakeholder validation required.","source_branch":"experience/submit","target_branch":"main","proposal_id":"`+proposal.ID+`"}`, http.StatusCreated, &pull)
	base := "/repositories/" + repository.ID + "/pull-requests/" + pull.ID

	var failedPreview previews.Preview
	workflowJSON(t, server.URL, http.MethodPost, base+"/previews", contributor, `{}`, http.StatusCreated, &failedPreview)
	failedPreview = waitForPreviewState(t, server.URL, base, failedPreview.ID, contributor, "failed")
	if failedPreview.Failure != "build command failed" {
		t.Fatalf("failed build evidence was not retained: %#v", failedPreview)
	}
	writeWorkflowFile(t, work, "site/build-ready", "ready\n")
	gitOutput(t, work, "add", "site/build-ready")
	gitOutput(t, work, "commit", "-m", "Repair preview build")
	gitOutput(t, work, "push", "origin", "experience/submit")
	workflowJSON(t, server.URL, http.MethodPost, base+"/synchronize", contributor, `{}`, http.StatusOK, &pull)
	var candidatePreview previews.Preview
	workflowJSON(t, server.URL, http.MethodPost, base+"/previews", contributor, `{}`, http.StatusCreated, &candidatePreview)
	candidatePreview = waitForPreviewState(t, server.URL, base, candidatePreview.ID, contributor, "ready")

	expiring := time.Now().UTC().Add(150 * time.Millisecond).Format(time.RFC3339Nano)
	var invited previews.Preview
	workflowJSON(t, server.URL, http.MethodPost, base+"/previews/"+candidatePreview.ID+"/invitations", contributor, `{"user_id":"stakeholder","role":"feedback","source_kind":"user","expires_at":"`+expiring+`"}`, http.StatusCreated, &invited)
	time.Sleep(250 * time.Millisecond)
	workflowJSON(t, server.URL, http.MethodGet, base+"/previews/"+candidatePreview.ID+"/audience", stakeholder, "", http.StatusNotFound, nil)
	expires := time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano)
	workflowJSON(t, server.URL, http.MethodPost, base+"/previews/"+candidatePreview.ID+"/invitations", contributor, `{"user_id":"stakeholder","role":"feedback","source_kind":"user","expires_at":"`+expires+`"}`, http.StatusCreated, &invited)
	var audience struct {
		Revision        string          `json:"revision"`
		EffectiveAccess map[string]bool `json:"effective_access"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base+"/previews/"+candidatePreview.ID+"/audience", stakeholder, "", http.StatusOK, &audience)
	if audience.Revision != candidatePreview.Revision || audience.EffectiveAccess["repository"] || !audience.EffectiveAccess["preview"] {
		t.Fatalf("outsider preview boundary is incomplete: %#v", audience)
	}
	evidence := base64.StdEncoding.EncodeToString([]byte("token=secret-value button says Do it"))
	var finding previews.Finding
	workflowJSON(t, server.URL, http.MethodPost, base+"/previews/"+candidatePreview.ID+"/findings", stakeholder, `{"route":"/checkout?token=secret-value","title":"Primary action is unclear","description":"Authorization: Bearer private-value","reproduction_steps":["Open checkout","Read the primary action"],"blocking":true,"evidence":[{"kind":"console","name":"checkout.txt","media_type":"text/plain","content":"`+evidence+`"}]}`, http.StatusCreated, &finding)
	if finding.Revision != candidatePreview.Revision || len(finding.Evidence) != 1 || !finding.Evidence[0].Redacted || finding.Evidence[0].Content != "" {
		t.Fatalf("finding did not retain safe exact-revision evidence: %#v", finding)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/previews/"+candidatePreview.ID+"/acceptance", stakeholder, `{"requirement_id":"stakeholder-experience","scenario_id":"submit-copy","decision":"accepted","note":"The issue is accurately captured."}`, http.StatusCreated, nil)

	var linked struct {
		Finding  previews.Finding       `json:"finding"`
		Resource changesessions.Session `json:"resource"`
	}
	workPath := base + "/previews/" + candidatePreview.ID + "/findings/" + finding.ID
	workflowJSON(t, server.URL, http.MethodPost, workPath+"/work", contributor, `{"kind":"change_session","proposal_id":"`+proposal.ID+`","title":"Clarify the submit action","acceptance_criteria":["The primary action reads Submit changes"],"evidence_ids":["`+finding.Evidence[0].ID+`"],"owner_kind":"agent","owner_id":"codex"}`, http.StatusCreated, &linked)
	var delegated struct {
		Run        changesessions.Run `json:"run"`
		Credential struct {
			Token string `json:"token"`
		} `json:"credential"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/change-sessions/"+linked.Resource.ID+"/runs", contributor, `{"instructions":"Clarify the primary action using the preview finding.","revision_id":"`+candidatePreview.Revision+`","context_paths":["site/index.html"],"working_branch":"experience/submit","agent":"codex"}`, http.StatusCreated, &delegated)
	run, workerToken := delegated.Run, delegated.Credential.Token
	runBase := base + "/change-sessions/" + linked.Resource.ID + "/runs/" + run.ID
	agentClone := gitClone(t, remote(workerToken))
	gitOutput(t, agentClone, "config", "user.name", "Codex Agent")
	gitOutput(t, agentClone, "config", "user.email", "codex@agents.local")
	gitOutput(t, agentClone, "switch", "experience/submit")
	writeWorkflowFile(t, agentClone, "site/index.html", "<button>Submit changes</button>\n")
	gitOutput(t, agentClone, "commit", "-am", "Clarify submit action")
	repairedRevision := gitOutput(t, agentClone, "rev-parse", "HEAD")
	gitOutput(t, agentClone, "push", "origin", "experience/submit")
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/events", workerToken, `{"type":"run.started","metadata":{"status":"Repairing the preview finding"}}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/publication", workerToken, `{"summary":"Clarified the stakeholder-reported action.","checks":["experience"],"concerns":[]}`, http.StatusCreated, nil)
	passed := waitForWorkflowCheck(t, server.URL, base, maintainer, repairedRevision, checkruns.Succeeded)
	var stale readinessResponse
	workflowJSON(t, server.URL, http.MethodGet, base+"/readiness", maintainer, "", http.StatusOK, &stale)
	if stale.Ready || stale.Acceptance == nil || stale.Acceptance.Satisfied || len(stale.Acceptance.Scenarios) != 1 || len(stale.Acceptance.Scenarios[0].StaleAcknowledgements) != 1 {
		t.Fatalf("old stakeholder acceptance did not become a visible blocker: %#v", stale)
	}

	var repaired struct {
		Finding previews.Finding `json:"finding"`
		Preview previews.Preview `json:"preview"`
	}
	workflowJSON(t, server.URL, http.MethodPost, workPath+"/repairs", contributor, `{"revision":"`+repairedRevision+`","commit_ids":["`+repairedRevision+`"],"commands":["updated site/index.html"],"checks":["experience"],"author_ids":["codex"],"change_session_id":"`+linked.Resource.ID+`"}`, http.StatusCreated, &repaired)
	repaired.Preview = waitForPreviewState(t, server.URL, base, repaired.Preview.ID, contributor, "ready")
	expires = time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano)
	workflowJSON(t, server.URL, http.MethodPost, base+"/previews/"+repaired.Preview.ID+"/invitations", contributor, `{"user_id":"stakeholder","role":"feedback","source_kind":"user","expires_at":"`+expires+`"}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/previews/"+repaired.Preview.ID+"/acceptance", stakeholder, `{"requirement_id":"stakeholder-experience","scenario_id":"submit-copy","decision":"accepted","note":"Submit changes is clear."}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPut, base+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
	var ready readinessResponse
	workflowJSON(t, server.URL, http.MethodGet, base+"/readiness", maintainer, "", http.StatusOK, &ready)
	if !ready.Ready || !ready.CanMerge || ready.Acceptance == nil || !ready.Acceptance.Satisfied || ready.Checks.Requirements[0].RunID != passed.ID {
		t.Fatalf("current preview, check, and review did not make the repair mergeable: %#v", ready)
	}
	var merged pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, base+"/merge", maintainer, `{}`, http.StatusOK, &merged)
	var release releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", maintainer, `{"version":"v1.0.0","commit_id":"`+merged.MergeCommitID+`","notes":"Stakeholder-accepted preview delivery."}`, http.StatusCreated, &release)
	waitForReleaseArtifact(t, server.URL, repository.ID, release.ID, maintainer)
	if len(release.PullRequests) != 1 || release.PullRequests[0].ID != pull.ID || release.PullRequests[0].MergeCommitID != merged.MergeCommitID {
		t.Fatalf("release lost the collaborative preview delivery trail: %#v", release)
	}
	assertFile(t, filepath.Join(gitClone(t, remote(maintainerGit)), "site/index.html"), "<button>Submit changes</button>\n", 0)
}

func previewManifest(build string) string {
	return `{"version":1,"build":["` + build + `"],"start":"python3 -m http.server \"$PORT\" --directory site","port":8080,"resources":{"cpu_seconds":30,"memory_mb":128,"disk_mb":128,"build_timeout_seconds":30,"lifetime_minutes":1},"audience":{"network":"none","data":"synthetic","identity":"anonymous","actions":["navigate","comment"]}}`
}

func waitForPreviewState(t *testing.T, origin, base, id, actor, state string) previews.Preview {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var item previews.Preview
		workflowJSON(t, origin, http.MethodGet, base+"/previews/"+id, actor, "", http.StatusOK, &item)
		if item.State == state {
			return item
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("preview %s did not reach %s", id, state)
	return previews.Preview{}
}
