package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/stackedchanges"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestChangeStackPublicBoundary(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	stacks, _ := stackedchanges.New(t.TempDir())
	repository, _ := repos.Create("owner", repositories.Metadata{Name: "stacked", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repository.ID, "author")
	opened, _ := repos.Open(repository.ID)
	tree0, _ := opened.WriteObject(storage.TreeObject, nil)
	base, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor A <a@x> 1 +0000\ncommitter A <a@x> 1 +0000\n\nbase\n", tree0)))
	blob, _ := opened.WriteObject(storage.BlobObject, []byte("package api\n"))
	rawBlob, _ := hex.DecodeString(string(blob))
	tree1, _ := opened.WriteObject(storage.TreeObject, append([]byte("100644 api.go\x00"), rawBlob...))
	one, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor A <a@x> 2 +0000\ncommitter A <a@x> 2 +0000\n\napi\n", tree1, base)))
	two, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor B <b@x> 3 +0000\ncommitter B <b@x> 3 +0000\n\ndocs\n", tree0, one)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: base})
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/api", ObjectID: one})
	token := issueAccess(t, credentials, "author", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerStackedChangesHTTP(mux, stacks, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	root := "/repositories/" + string(repository.ID) + "/change-stacks"
	in := stackedchanges.Input{Title: "Ship public API", Outcome: "Clients can use and understand the endpoint", TargetBranch: "main", TargetRevision: string(base), Members: []stackedchanges.MemberInput{{ID: "api", Branch: "api", BranchState: "existing", PullRequestID: "pull:1", Revision: string(one), Authors: []string{"author"}, AcceptanceCriteria: []string{"endpoint responds"}}, {ID: "docs", Branch: "docs", BranchState: "new", Revision: string(two), ParentID: "api", Authors: []string{"author", "docs"}, AcceptanceCriteria: []string{"example is usable"}}}}
	body, _ := json.Marshal(in)
	var stack stackedchanges.Stack
	workflowJSON(t, server.URL, http.MethodPost, root, token, string(body), 201, &stack)
	if stack.Status != "reviewable" || len(stack.Members) != 2 || stack.Members[1].BaseRevision != string(one) || stack.Members[0].IndividualScope.CommitCount != 1 || stack.Members[1].CumulativeScope.CommitCount != 2 || len(stack.AuthorityGranted) != 0 {
		t.Fatalf("stack context lost: %#v", stack)
	}
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+stack.ID+"/members/api/publications", token, fmt.Sprintf(`{"revision":%q}`, one), 201, &stack)
	if len(stack.Members[0].Publications) != 1 || stack.Members[0].Publications[0].Revision != string(one) {
		t.Fatalf("exact publication lost: %#v", stack)
	}
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+stack.ID+"/members/docs/publications", token, fmt.Sprintf(`{"revision":%q}`, two), 201, &stack)
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+stack.ID+"/members/docs/evidence", token, fmt.Sprintf(`{"revision":%q,"kind":"review_decision","reference":"review:7","scope":"layer"}`, two), 201, &stack)
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+stack.ID+"/members/docs/evidence", token, fmt.Sprintf(`{"revision":%q,"kind":"check","reference":"check:docs","scope":"cumulative"}`, two), 201, &stack)
	if len(stack.Members[1].Evidence) != 2 || stack.Members[1].Evidence[0].UpstreamRevisions["api"] != string(one) || stack.Members[1].Evidence[0].State != "current" || len(stack.Members[0].DownstreamEvidenceAtRisk) != 2 || len(stack.Members[1].IndividualScope.Changes) == 0 || len(stack.Members[1].CumulativeScope.CommitIDs) != 2 {
		t.Fatalf("layer review context lost: %#v", stack)
	}
	var context struct {
		Member    stackedchanges.Member `json:"member"`
		Authority []string              `json:"authority_granted"`
	}
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+string(repository.ID)+"/pull-requests/pull:1/stack-context", token, "", 200, &context)
	if context.Member.ID != "api" || context.Member.ReviewState != "reviewable_now" || len(context.Authority) != 0 {
		t.Fatalf("pull stack projection lost: %#v", context)
	}
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+stack.ID+"/members/docs/assignments", token, `{"participant_id":"author","participant_kind":"agent","agent_approval_id":"onboarding:approved","authorized_branches":["docs"]}`, 201, &stack)
	if len(stack.Members[1].Assignments) != 1 || stack.Members[1].Assignments[0].AgentApprovalID == "" {
		t.Fatalf("agent assignment lost: %#v", stack.Members[1])
	}
	assignment := stack.Members[1].Assignments[0]
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+stack.ID+"/members/docs/workspaces", token, fmt.Sprintf(`{"assignment_id":%q,"kind":"shared","audience":"repository"}`, assignment.ID), 201, &stack)
	workspace := stack.Members[1].Workspaces[0]
	if workspace.Outcome != stack.Outcome || workspace.ParentRevision != string(one) || len(workspace.AcceptanceCriteria) != 1 || len(workspace.EditableBranches) != 1 || len(workspace.AuthorityGranted) != 0 {
		t.Fatalf("scoped workspace preload lost: %#v", workspace)
	}
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+stack.ID+"/timeline", token, fmt.Sprintf(`{"member_id":"docs","workspace_id":%q,"kind":"checkpoint","summary":"docs compile against API v1","audience":"repository"}`, workspace.ID), 201, &stack)
	if len(stack.Timeline) != 1 || stack.Timeline[0].UpstreamRevisions["api"] != string(one) || stack.Timeline[0].State != "current" {
		t.Fatalf("timeline assumption lost: %#v", stack.Timeline)
	}
	newOne, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor A <a@x> 4 +0000\ncommitter A <a@x> 4 +0000\n\nrevised api\n", tree0, base)))
	newTwo, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor B <b@x> 5 +0000\ncommitter B <b@x> 5 +0000\n\nrebased docs\n", tree1, newOne)))
	revised := in.Members
	revised[0].Revision = string(newOne)
	revised[1].Revision = string(newTwo)
	revisionBody, _ := json.Marshal(map[string]any{"expected_revision": 1, "reason": "Address first-layer feedback", "members": revised})
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+stack.ID+"/revisions", token, string(revisionBody), 201, &stack)
	preview := stack.Revisions[len(stack.Revisions)-1]
	if preview.Status != "ready" || len(preview.CommitRewrites) != 2 || len(preview.ReviewInvalidations) != 2 || len(preview.CheckImpacts) != 1 || preview.BranchUpdates[0].ExpectedRevision != string(one) || preview.BranchUpdates[1].ExpectedRevision != "" {
		t.Fatalf("rewrite preview lost: %#v", preview)
	}
	workflowJSON(t, server.URL, http.MethodPost, fmt.Sprintf("%s/%s/revisions/2/apply", root, stack.ID), token, `{}`, 200, &stack)
	apiRef, _ := opened.ReadReference("refs/heads/api")
	docsRef, _ := opened.ReadReference("refs/heads/docs")
	if stack.CurrentRevision != 2 || stack.Revisions[0].Status != "applied" || apiRef.ObjectID != newOne || docsRef.ObjectID != newTwo || len(stack.Members[1].Evidence) != 0 {
		t.Fatalf("atomic rewrite not retained: %#v api=%s docs=%s", stack, apiRef.ObjectID, docsRef.ObjectID)
	}
	if len(stack.Members[1].Assignments) != 1 || len(stack.Members[1].Workspaces) != 1 || stack.Timeline[0].State != "upstream_changed" {
		t.Fatalf("collaboration lineage did not survive restack: %#v", stack)
	}
	third, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor A <a@x> 6 +0000\ncommitter A <a@x> 6 +0000\n\nsecond feedback\n", tree1, base)))
	fourth, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor B <b@x> 7 +0000\ncommitter B <b@x> 7 +0000\n\nsecond downstream rebase\n", tree0, third)))
	revised[0].Revision, revised[1].Revision = string(third), string(fourth)
	revisionBody, _ = json.Marshal(map[string]any{"expected_revision": 2, "reason": "Preview concurrent protection", "members": revised})
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+stack.ID+"/revisions", token, string(revisionBody), 201, &stack)
	_ = opened.UpdateReference(storage.Reference{Name: "refs/heads/api", ObjectID: one})
	workflowJSON(t, server.URL, http.MethodPost, fmt.Sprintf("%s/%s/revisions/3/apply", root, stack.ID), token, `{}`, 409, &stack)
	docsRef, _ = opened.ReadReference("refs/heads/docs")
	failed := stack.Revisions[len(stack.Revisions)-1]
	if failed.Status != "failed" || failed.Blockers[len(failed.Blockers)-1].Kind != "concurrent_push_or_failed_rewrite" || docsRef.ObjectID != newTwo || stack.CurrentRevision != 2 {
		t.Fatalf("concurrent push was not atomically contained: %#v docs=%s", failed, docsRef.ObjectID)
	}
	landingBody := fmt.Sprintf(`{"expected_stack_revision":2,"expected_target_revision":%q,"mode":"ordered","atomic_permitted":true,"required_evidence":["required_check","reproduction","contract","preview","policy","approval"]}`, base)
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+stack.ID+"/landings", token, landingBody, 201, &stack)
	landing := stack.Landings[0]
	if len(landing.Candidates) != 2 || landing.Candidates[0].BaseRevision != string(base) || landing.Candidates[1].BaseRevision != string(newOne) || landing.Status != "paused" {
		t.Fatalf("ready-prefix candidates lost: %#v", landing)
	}
	for _, candidate := range landing.Candidates {
		for _, kind := range candidate.RequiredEvidence {
			status := "passed"
			if candidate.MemberID == "docs" && kind == "approval" {
				status = "failed"
			}
			workflowJSON(t, server.URL, http.MethodPost, fmt.Sprintf("%s/%s/landings/%s/candidates/%s/evidence", root, stack.ID, landing.ID, candidate.ID), token, fmt.Sprintf(`{"kind":%q,"reference":%q,"status":%q}`, kind, kind+":"+candidate.MemberID, status), 201, &stack)
		}
	}
	workflowJSON(t, server.URL, http.MethodPost, fmt.Sprintf("%s/%s/landings/%s/merge", root, stack.ID, landing.ID), token, `{"member_id":"api","atomic":false}`, 200, &stack)
	landing = stack.Landings[0]
	if len(landing.MergedMembers) != 1 || landing.PausedFromMember != "docs" || landing.Candidates[1].Status != "verifying" {
		t.Fatalf("failed member did not preserve merged prefix: %#v", landing)
	}
	workflowJSON(t, server.URL, http.MethodPost, fmt.Sprintf("%s/%s/landings/%s/candidates/%s/evidence", root, stack.ID, landing.ID, landing.Candidates[1].ID), token, `{"kind":"approval","reference":"approval:restored","status":"passed"}`, 201, &stack)
	workflowJSON(t, server.URL, http.MethodPost, fmt.Sprintf("%s/%s/landings/%s/merge", root, stack.ID, landing.ID), token, `{"member_id":"docs","atomic":false}`, 200, &stack)
	mainRef, _ := opened.ReadReference("refs/heads/main")
	if stack.Landings[0].Status != "merged" || mainRef.ObjectID != newTwo || len(stack.Landings[0].Candidates[1].Evidence) != 7 {
		t.Fatalf("ordered landing did not retain exact evidence and history: %#v main=%s", stack.Landings[0], mainRef.ObjectID)
	}
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+stack.ID+"/members/docs/publications", token, fmt.Sprintf(`{"revision":%q}`, one), 422, nil)
	bad := in
	bad.Title = "Visible blockers"
	bad.Members[1].Revision = "missing"
	body, _ = json.Marshal(bad)
	workflowJSON(t, server.URL, http.MethodPost, root, token, string(body), 201, &stack)
	if stack.Status != "blocked" || stack.Members[1].Blockers[0].Kind != "missing_commit" {
		t.Fatalf("missing revision hidden: %#v", stack)
	}
}
