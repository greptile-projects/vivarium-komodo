package main

import (
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
	tree1, _ := opened.WriteObject(storage.TreeObject, []byte("100644 api.go\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"))
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
	if len(stack.Members[1].Evidence) != 1 || stack.Members[1].Evidence[0].UpstreamRevisions["api"] != string(one) || stack.Members[1].Evidence[0].State != "current" || len(stack.Members[0].DownstreamEvidenceAtRisk) != 1 || len(stack.Members[1].IndividualScope.Changes) == 0 || len(stack.Members[1].CumulativeScope.CommitIDs) != 2 {
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
