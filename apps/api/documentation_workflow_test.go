package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/docscollections"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestCodeToTrustedGuidanceWorkflow composes living documentation with the
// ordinary proposal, Git, check, review, merge, release, and reader-feedback
// contracts. It deliberately repairs an archived release's instructions so
// version selection never silently replaces what an older reader was taught.
func TestCodeToTrustedGuidanceWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the documentation workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	documentation, _ := docscollections.New(t.TempDir())
	runner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, nil, runner, checks)
	registerCheckRunsHTTP(mux, checks, runner, pulls, catalog, credentials, nil, nil)
	registerReleasesHTTP(mux, releaseStore, checks, runner, pulls, catalog, credentials)
	registerDocumentationHTTP(mux, documentation, catalog, credentials, releaseStore, pulls)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "owner", auth.Git, auth.GitRead, auth.GitWrite)
	contributor := issueAccess(t, credentials, "contributor", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	contributorGit := issueAccess(t, credentials, "contributor", auth.Git, auth.GitRead, auth.GitWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	agent := issueAccess(t, credentials, "codex", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agentGit := issueAccess(t, credentials, "codex", auth.Git, auth.GitRead, auth.GitWrite)
	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"living-guide","visibility":"public"}`, http.StatusCreated, &repository)
	for _, participant := range []string{"contributor", "codex"} {
		if _, err := catalog.AddCollaborator("owner", storage.ID(repository.ID), participant); err != nil {
			t.Fatal(err)
		}
	}
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/required-checks", owner, `{"branch":"main","checks":["docs/install"]}`, http.StatusOK, nil)
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	work := gitClone(t, remote(ownerGit))
	gitOutput(t, work, "config", "user.name", "Owner")
	gitOutput(t, work, "config", "user.email", "owner@example.com")
	writeWorkflowFile(t, work, "cmd.txt", "legacy\n")
	writeWorkflowFile(t, work, "docs/install.md", "# Install\n\nRun `grep -qx legacy cmd.txt`.\n")
	writeWorkflowFile(t, work, ".komodo/documentation-checks.json", documentationCheckManifest("grep -qx legacy cmd.txt"))
	writeWorkflowFile(t, work, ".komodo/releases.json", `{"version":1,"builds":[{"name":"guide","command":"mkdir -p dist; cp docs/install.md dist/install.md","artifacts":["dist/install.md"]}]}`)
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Publish legacy behavior and guidance")
	legacyRevision := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "main")
	var legacyRelease releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", owner, `{"version":"v1.0.0","commit_id":"`+legacyRevision+`","notes":"Legacy supported behavior."}`, http.StatusCreated, &legacyRelease)

	collectionBody := func(expected int64, revision, releaseID, reason string) string {
		return `{"expected_version":` + workflowInt(expected) + `,"name":"Install guide","description":"Version-exact command behavior.","root_path":"docs","entry_paths":["install.md"],"versions":[{"label":"v1.0","source_revision":"` + revision + `","release_id":"` + releaseID + `"}],"owner_ids":["owner"],"audiences":["developers"],"policy":{"navigation":"path","renderer":"markdown","publication":"owner_reviewed","visibility":"public"},"links":[{"kind":"release","label":"Supported release","resource_id":"` + releaseID + `"}],"change_reason":"` + reason + `"}`
	}
	var collection docscollections.Collection
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/documentation-collections", owner, collectionBody(0, legacyRevision, legacyRelease.ID, "Establish versioned guidance"), http.StatusCreated, &collection)

	var proposal proposals.Proposal
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/proposals", contributor, `{"title":"Rename the supported command marker","body":"Change the behavior and its installation instruction together."}`, http.StatusCreated, &proposal)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/proposals/"+proposal.ID+"/comments", owner, `{"body":"Keep the old release discoverable and prove the new example."}`, http.StatusCreated, nil)
	candidate := gitClone(t, remote(contributorGit))
	gitOutput(t, candidate, "config", "user.name", "Contributor")
	gitOutput(t, candidate, "config", "user.email", "contributor@example.com")
	gitOutput(t, candidate, "switch", "-c", "docs/current-command")
	writeWorkflowFile(t, candidate, "cmd.txt", "current\n")
	writeWorkflowFile(t, candidate, "docs/install.md", "# Install\n\nRun `grep -qx current cmd.txt`.\n")
	writeWorkflowFile(t, candidate, ".komodo/documentation-checks.json", documentationCheckManifest("grep -qx current cmd.txt"))
	gitOutput(t, candidate, "add", ".")
	gitOutput(t, candidate, "commit", "-m", "Align behavior and installation guide")
	candidateRevision := gitOutput(t, candidate, "rev-parse", "HEAD")
	gitOutput(t, candidate, "push", "-u", "origin", "docs/current-command")

	var task docscollections.Task
	taskRoot := "/repositories/" + repository.ID + "/documentation-collections/" + collection.ID + "/tasks"
	workflowJSON(t, server.URL, http.MethodPost, taskRoot, contributor, `{"title":"Explain the current command","path":"install.md","revision":"`+candidateRevision+`","mode":"branch","branch":"docs/current-command","origin":{"kind":"proposal","resource_id":"`+proposal.ID+`"},"evidence":["cmd.txt"]}`, http.StatusCreated, &task)
	taskBase := "/repositories/" + repository.ID + "/documentation-tasks/" + task.ID
	workflowJSON(t, server.URL, http.MethodPost, taskBase+"/events", contributor, `{"type":"draft","draft":"# Install\n\nRun the current marker command.","references":[{"path":"cmd.txt","start_line":1,"end_line":1}]}`, http.StatusCreated, &task)
	workflowJSON(t, server.URL, http.MethodPost, taskBase+"/events", agent, `{"type":"suggestion","body":"The example matches the candidate marker.","citations":["cmd.txt:1@`+candidateRevision+`"],"uncertainty":"Only the declared v1.0 matrix has been executed."}`, http.StatusCreated, &task)
	if len(task.Events) != 3 || task.Events[1].References[0].BlobID == "" || task.Events[2].ActorID != "codex" {
		t.Fatalf("grounded draft trail is incomplete: %#v", task.Events)
	}

	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests", contributor, `{"title":"Align command behavior and guidance","body":"One reviewed code-and-docs change.","source_branch":"docs/current-command","target_branch":"main","proposal_id":"`+proposal.ID+`"}`, http.StatusCreated, &pull)
	pullBase := "/repositories/" + repository.ID + "/pull-requests/" + pull.ID
	docCheck := waitForWorkflowCheck(t, server.URL, pullBase, owner, candidateRevision, checkruns.Succeeded)
	if docCheck.Definition.Documentation == nil || docCheck.Definition.Documentation.Versions[0].Label != "v1.0" {
		t.Fatalf("documentation matrix evidence is missing: %#v", docCheck)
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/documentation-collections/"+collection.ID+"/versions", owner, collectionBody(1, candidateRevision, "", "Review the behavior change"), http.StatusCreated, &collection)
	var preview docscollections.ReviewPreview
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/documentation-previews", contributor, `{"collection_id":"`+collection.ID+`","affected_versions":["v1.0"],"verified_examples":[{"name":"install marker","check_run_id":"`+docCheck.ID+`","status":"succeeded"}]}`, http.StatusCreated, &preview)
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/documentation-previews/"+preview.ID+"/decisions/technical", owner, `{"decision":"approve","body":"Behavior and example agree."}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	var merged pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", owner, `{}`, http.StatusOK, &merged)
	var currentRelease releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", owner, `{"version":"v2.0.0","commit_id":"`+merged.MergeCommitID+`","prior_release_id":"`+legacyRelease.ID+`","notes":"Current command behavior."}`, http.StatusCreated, &currentRelease)
	var firstEdition docscollections.Publication
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/documentation-publications", owner, `{"preview_id":"`+preview.ID+`"}`, http.StatusCreated, &firstEdition)

	var feedback docscollections.Feedback
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/documentation/"+firstEdition.ID+"/feedback", reader, `{"page_path":"docs/install.md","kind":"failed_example","body":"The older v1.0 release still uses the legacy marker.","expected_version":"v1.0","evidence":[{"kind":"log","name":"output.txt","content":"token=reader-secret no match"}]}`, http.StatusCreated, &feedback)
	if feedback.ReporterID != "reader" || strings.Contains(feedback.Evidence[0].Content, "secret") {
		t.Fatalf("reader evidence boundary is incomplete: %#v", feedback)
	}
	var repairTask docscollections.Task
	workflowJSON(t, server.URL, http.MethodPost, taskRoot, owner, `{"title":"Repair v1.0 instructions","path":"install.md","revision":"`+merged.MergeCommitID+`","mode":"branch","branch":"docs/v1-repair","origin":{"kind":"release","resource_id":"`+legacyRelease.ID+`"},"evidence":["feedback:`+feedback.ID+`"]}`, http.StatusCreated, &repairTask)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/documentation-feedback/"+feedback.ID+"/triage", owner, `{"kind":"documentation_task","resource_id":"`+repairTask.ID+`"}`, http.StatusOK, &feedback)
	repairTaskBase := "/repositories/" + repository.ID + "/documentation-tasks/" + repairTask.ID
	workflowJSON(t, server.URL, http.MethodPost, repairTaskBase+"/events", agent, `{"type":"draft","draft":"For v1.0 run the legacy marker; for v2.0 run current.","references":[{"path":"cmd.txt","start_line":1,"end_line":1}],"citations":["feedback:`+feedback.ID+`","release:`+legacyRelease.ID+`"],"uncertainty":"The legacy release source must remain explicitly selected."}`, http.StatusCreated, &repairTask)

	agentWork := gitClone(t, remote(agentGit))
	gitOutput(t, agentWork, "config", "user.name", "Codex Agent")
	gitOutput(t, agentWork, "config", "user.email", "codex@agents.local")
	gitOutput(t, agentWork, "switch", "-c", "docs/v1-repair")
	writeWorkflowFile(t, agentWork, "docs/install.md", "# Install\n\nFor v1.0 run `grep -qx legacy cmd.txt`. For v2.0 run `grep -qx current cmd.txt`.\n")
	writeWorkflowFile(t, agentWork, ".komodo/documentation-checks.json", documentationCheckManifest("grep -qx current cmd.txt"))
	gitOutput(t, agentWork, "add", ".")
	gitOutput(t, agentWork, "commit", "-m", "Repair version-specific installation guidance")
	repairRevision := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push", "-u", "origin", "docs/v1-repair")
	var repairPull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests", agent, `{"title":"Repair v1.0 installation guidance","body":"Derived from reader feedback `+feedback.ID+` and documentation task `+repairTask.ID+`.","source_branch":"docs/v1-repair","target_branch":"main"}`, http.StatusCreated, &repairPull)
	repairBase := "/repositories/" + repository.ID + "/pull-requests/" + repairPull.ID
	repairCheck := waitForWorkflowCheck(t, server.URL, repairBase, owner, repairRevision, checkruns.Succeeded)
	versions := `[{"label":"v1.0","source_revision":"` + legacyRevision + `","release_id":"` + legacyRelease.ID + `"},{"label":"v2.0","source_revision":"` + repairRevision + `"}]`
	update := `{"expected_version":2,"name":"Install guide","description":"Version-exact command behavior.","root_path":"docs","entry_paths":["install.md"],"versions":` + versions + `,"owner_ids":["owner"],"audiences":["developers"],"policy":{"navigation":"path","renderer":"markdown","publication":"owner_reviewed","visibility":"public"},"links":[{"kind":"release","label":"Reported release","resource_id":"` + legacyRelease.ID + `"}],"change_reason":"Repair reader-reported version mismatch"}`
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/documentation-collections/"+collection.ID+"/versions", owner, update, http.StatusCreated, &collection)
	var repairPreview docscollections.ReviewPreview
	workflowJSON(t, server.URL, http.MethodPost, repairBase+"/documentation-previews", agent, `{"collection_id":"`+collection.ID+`","affected_versions":["v1.0","v2.0"],"verified_examples":[{"name":"current marker","check_run_id":"`+repairCheck.ID+`","status":"succeeded"}]}`, http.StatusCreated, &repairPreview)
	workflowJSON(t, server.URL, http.MethodPut, repairBase+"/documentation-previews/"+repairPreview.ID+"/decisions/audience", owner, `{"decision":"approve","body":"The older and current instructions are explicit."}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, repairBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, repairBase+"/merge", owner, `{}`, http.StatusOK, &repairPull)
	var repairRelease releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", owner, `{"version":"v2.0.1","commit_id":"`+repairPull.MergeCommitID+`","prior_release_id":"`+currentRelease.ID+`","notes":"Corrected version-specific guidance."}`, http.StatusCreated, &repairRelease)
	var repairedEdition docscollections.Publication
	workflowJSON(t, server.URL, http.MethodPost, repairBase+"/documentation-publications", owner, `{"preview_id":"`+repairPreview.ID+`"}`, http.StatusCreated, &repairedEdition)
	var readerView struct {
		Items []publishedDocumentationResponse `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+repository.ID+"/documentation?version=v1.0", reader, "", http.StatusOK, &readerView)
	var selected *publishedDocumentationResponse
	for i := range readerView.Items {
		if !readerView.Items[i].Archived {
			selected = &readerView.Items[i]
		}
	}
	if len(readerView.Items) != 2 || selected == nil || !strings.Contains(selected.Pages[0].Rendered, "legacy") || repairedEdition.PublishedByID != "owner" || repairPull.AuthorID != "codex" || repairRelease.PullRequests[0].ID != repairPull.ID {
		t.Fatalf("published repair lost version, authorship, or delivery provenance: view=%#v edition=%#v pull=%#v release=%#v", readerView, repairedEdition, repairPull, repairRelease)
	}
	assertFile(t, filepath.Join(gitClone(t, remote(ownerGit)), "docs/install.md"), "# Install\n\nFor v1.0 run `grep -qx legacy cmd.txt`. For v2.0 run `grep -qx current cmd.txt`.\n", 0)
}

func documentationCheckManifest(command string) string {
	return `{"version":1,"checks":[{"name":"docs/install","command":"` + command + `","timeout_seconds":30,"documentation":{"kind":"tutorial","collection_id":"install-guide","inputs":["docs/install.md","cmd.txt"],"pages":["docs/install.md"],"versions":[{"label":"v1.0","source_commit":"supported"}],"expected_output":"exit 0","coverage":{"samples":1}}}]}`
}
