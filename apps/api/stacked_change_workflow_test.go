package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/stackedchanges"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestLargeChangeToIntegratedStack is the black-box boundary for delivering a
// large human-agent change as reviewable layers. It uses stock Git and the same
// public HTTP surface as view=stacks; stack records coordinate existing branch,
// review, agent, check, and merge authority without replacing any of it.
func TestLargeChangeToIntegratedStack(t *testing.T) {
	requireGit(t)
	work := t.TempDir()
	gitOutput(t, work, "init", "-b", "main")
	gitOutput(t, work, "config", "user.name", "Developer")
	gitOutput(t, work, "config", "user.email", "developer@example.test")
	write := func(name, contents string) {
		t.Helper()
		path := filepath.Join(work, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(message string, paths ...string) string {
		t.Helper()
		args := append([]string{"add"}, paths...)
		gitOutput(t, work, args...)
		gitOutput(t, work, "commit", "-m", message)
		return gitOutput(t, work, "rev-parse", "HEAD")
	}
	write("feature.conf", "mode=off\n")
	base := commit("Establish feature boundary", "feature.conf")
	gitOutput(t, work, "switch", "-c", "feature-foundation")
	write("feature.conf", "mode=legacy\n")
	foundation := commit("Add feature configuration", "feature.conf")
	gitOutput(t, work, "switch", "-c", "feature-agent")
	gitOutput(t, work, "config", "user.name", "Approved Agent")
	gitOutput(t, work, "config", "user.email", "agent@example.test")
	write("feature.conf", "mode=legacy\nendpoint=/v2/stacked\n")
	write("handler.go", "package feature\n\nconst Endpoint = \"/v2/stacked\"\n")
	agentLayer := commit("Implement the agent-owned endpoint", "feature.conf", "handler.go")
	gitOutput(t, work, "switch", "-c", "feature-docs")
	gitOutput(t, work, "config", "user.name", "Developer")
	gitOutput(t, work, "config", "user.email", "developer@example.test")
	write("docs/feature.md", "# Stacked feature\n\nCall `/v2/stacked`.\n")
	docsLayer := commit("Document the complete feature", "docs/feature.md")

	objects, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), objects)
	credentials, _ := auth.New(t.TempDir())
	stacks, _ := stackedchanges.New(t.TempDir())
	repository, _ := repos.Create("developer", repositories.Metadata{Name: "stacked-delivery", Visibility: repositories.Public})
	for _, actor := range []string{"agent", "reviewer", "maintainer"} {
		if _, err := repos.AddCollaborator("developer", repository.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	opened, _ := repos.Open(repository.ID)
	gitOutput(t, work, "remote", "add", "platform", opened.GitDir())
	gitOutput(t, work, "push", "platform", base+":refs/heads/main", foundation+":refs/heads/feature-foundation", agentLayer+":refs/heads/feature-agent", docsLayer+":refs/heads/feature-docs")
	tokens := map[string]string{}
	for _, actor := range []string{"developer", "agent", "reviewer", "maintainer"} {
		tokens[actor] = issueAccess(t, credentials, actor, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	}
	mux := http.NewServeMux()
	registerStackedChangesHTTP(mux, stacks, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	root := "/repositories/" + string(repository.ID) + "/change-stacks"
	request := func(method, path, actor string, value any, want int, out any) {
		t.Helper()
		body := ""
		if value != nil {
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			body = string(encoded)
		}
		workflowJSON(t, server.URL, method, path, tokens[actor], body, want, out)
	}
	members := []stackedchanges.MemberInput{
		{ID: "foundation", Branch: "feature-foundation", BranchState: "existing", PullRequestID: "pull:101", Revision: foundation, Authors: []string{"developer"}, BranchOwnerIDs: []string{"developer"}, AcceptanceCriteria: []string{"configuration defaults remain compatible"}},
		{ID: "agent-endpoint", Branch: "feature-agent", BranchState: "existing", PullRequestID: "pull:102", Revision: agentLayer, ParentID: "foundation", Authors: []string{"agent"}, BranchOwnerIDs: []string{"developer"}, AcceptanceCriteria: []string{"endpoint uses the current configuration contract"}},
		{ID: "docs", Branch: "feature-docs", BranchState: "existing", PullRequestID: "pull:103", Revision: docsLayer, ParentID: "agent-endpoint", Authors: []string{"developer"}, BranchOwnerIDs: []string{"developer"}, AcceptanceCriteria: []string{"developer journey is documented"}},
	}
	var stack stackedchanges.Stack
	request(http.MethodPost, root, "developer", stackedchanges.Input{Title: "Ship the stacked feature", Outcome: "Developers can use one reviewed configuration, endpoint, and guide", TargetBranch: "main", TargetRevision: base, Members: members}, http.StatusCreated, &stack)
	if stack.Status != "reviewable" || stack.Members[1].IndividualScope.CommitCount != 1 || stack.Members[2].CumulativeScope.CommitCount != 3 || len(stack.Members[2].CumulativeScope.Changes) != 3 {
		t.Fatalf("focused and cumulative review context is incomplete: %#v", stack)
	}
	for _, member := range stack.Members {
		request(http.MethodPost, fmt.Sprintf("%s/%s/members/%s/publications", root, stack.ID, member.ID), "developer", map[string]string{"revision": member.Revision}, http.StatusCreated, &stack)
	}
	request(http.MethodPost, fmt.Sprintf("%s/%s/members/foundation/evidence", root, stack.ID), "reviewer", map[string]string{"revision": foundation, "kind": "review_decision", "reference": "review:foundation-changes-requested", "scope": "layer"}, http.StatusCreated, &stack)
	request(http.MethodPost, fmt.Sprintf("%s/%s/members/agent-endpoint/evidence", root, stack.ID), "reviewer", map[string]string{"revision": agentLayer, "kind": "review_decision", "reference": "review:agent-provisional", "scope": "cumulative"}, http.StatusCreated, &stack)
	request(http.MethodPost, fmt.Sprintf("%s/%s/members/agent-endpoint/assignments", root, stack.ID), "developer", map[string]any{"participant_id": "agent", "participant_kind": "agent", "agent_approval_id": "agent-approval:current", "authorized_branches": []string{"feature-agent"}}, http.StatusCreated, &stack)
	assignment := stack.Members[1].Assignments[0]
	request(http.MethodPost, fmt.Sprintf("%s/%s/members/agent-endpoint/workspaces", root, stack.ID), "agent", map[string]string{"assignment_id": assignment.ID, "kind": "conflict_resolution", "audience": "repository"}, http.StatusCreated, &stack)
	workspace := stack.Members[1].Workspaces[0]
	request(http.MethodPost, fmt.Sprintf("%s/%s/timeline", root, stack.ID), "agent", map[string]string{"member_id": "agent-endpoint", "workspace_id": workspace.ID, "kind": "question", "summary": "Should legacy mode become explicit enabled mode?", "audience": "repository"}, http.StatusCreated, &stack)

	// Address the early review and use stock Git to reorder docs before the agent
	// layer. Cherry-picking the agent change now conflicts semantically with the
	// renamed configuration value, so the assigned agent resolves it explicitly.
	gitOutput(t, work, "switch", "feature-foundation")
	write("feature.conf", "mode=enabled\n")
	revisedFoundation := commit("Address configuration review", "feature.conf")
	gitOutput(t, work, "switch", "--detach", revisedFoundation)
	gitOutput(t, work, "cherry-pick", docsLayer)
	reorderedDocs := gitOutput(t, work, "rev-parse", "HEAD")
	conflict := exec.Command("git", "cherry-pick", agentLayer)
	conflict.Dir = work
	if output, err := conflict.CombinedOutput(); err == nil {
		t.Fatalf("semantic conflict was not reproduced: %s", output)
	}
	write("feature.conf", "mode=enabled\nendpoint=/v2/stacked\n")
	gitOutput(t, work, "add", "feature.conf", "handler.go")
	gitOutput(t, work, "-c", "user.name=Approved Agent", "-c", "user.email=agent@example.test", "cherry-pick", "--continue")
	restackedAgent := gitOutput(t, work, "rev-parse", "HEAD")
	// Upload the proposed objects without moving any source branch. The public
	// revision preview can inspect them before its later optimistic ref update.
	gitOutput(t, work, "push", "platform", revisedFoundation+":refs/stacked/proposals/foundation", reorderedDocs+":refs/stacked/proposals/docs", restackedAgent+":refs/stacked/proposals/agent")
	request(http.MethodPost, fmt.Sprintf("%s/%s/timeline", root, stack.ID), "agent", map[string]string{"member_id": "agent-endpoint", "workspace_id": workspace.ID, "kind": "checkpoint", "summary": "Resolved mode semantics while preserving the reviewed endpoint behavior", "audience": "repository"}, http.StatusCreated, &stack)

	finalMembers := []stackedchanges.MemberInput{
		{ID: "foundation", Branch: "feature-foundation", BranchState: "existing", PullRequestID: "pull:101", Revision: revisedFoundation, Authors: []string{"developer"}, BranchOwnerIDs: []string{"developer"}, AcceptanceCriteria: members[0].AcceptanceCriteria},
		{ID: "docs", Branch: "feature-docs", BranchState: "existing", PullRequestID: "pull:103", Revision: reorderedDocs, ParentID: "foundation", Authors: []string{"developer"}, BranchOwnerIDs: []string{"developer"}, AcceptanceCriteria: members[2].AcceptanceCriteria},
		{ID: "agent-endpoint", Branch: "feature-agent", BranchState: "existing", PullRequestID: "pull:102", Revision: restackedAgent, ParentID: "docs", Authors: []string{"agent"}, BranchOwnerIDs: []string{"developer"}, AcceptanceCriteria: members[1].AcceptanceCriteria},
	}
	revoked := append([]stackedchanges.MemberInput{}, finalMembers...)
	revoked[2].BranchAccess = "revoked"
	request(http.MethodPost, root+"/"+stack.ID+"/revisions", "developer", map[string]any{"expected_revision": 1, "reason": "Agent approval was revoked before publication", "members": revoked}, http.StatusCreated, &stack)
	if stack.Revisions[0].Status != "blocked" || !workflowHasStackBlocker(stack.Revisions[0].Blockers, "revoked_access") {
		t.Fatalf("revoked agent access was not contained: %#v", stack.Revisions[0])
	}
	request(http.MethodPost, root+"/"+stack.ID+"/revisions", "developer", map[string]any{"expected_revision": 1, "reason": "Use replacement approval and resolved restack", "members": finalMembers}, http.StatusCreated, &stack)
	if stack.Revisions[1].Status != "ready" || len(stack.Revisions[1].ReviewInvalidations) != 2 || !workflowHasRewrite(stack.Revisions[1], "docs", "rebase") {
		t.Fatalf("reorder, stale review, and restack impact were not retained: %#v", stack.Revisions[1])
	}
	// A concurrent stock-Git push makes the all-ref transaction fail. No sibling
	// ref moves; after incorporating and reconciling that push, the next retained
	// preview can publish the complete corrected stack.
	gitOutput(t, work, "push", "platform", revisedFoundation+":refs/heads/feature-foundation")
	request(http.MethodPost, fmt.Sprintf("%s/%s/revisions/3/apply", root, stack.ID), "developer", map[string]any{}, http.StatusConflict, &stack)
	docsRef, _ := opened.ReadReference("refs/heads/feature-docs")
	if stack.Revisions[1].Status != "failed" || docsRef.ObjectID != storage.ObjectID(docsLayer) {
		t.Fatalf("concurrent push partially published the stack: %#v", stack.Revisions[1])
	}
	gitOutput(t, work, "push", "--force-with-lease=refs/heads/feature-foundation:"+revisedFoundation, "platform", foundation+":refs/heads/feature-foundation")
	request(http.MethodPost, root+"/"+stack.ID+"/revisions", "developer", map[string]any{"expected_revision": 1, "reason": "Retry after reconciling the concurrent push", "members": finalMembers}, http.StatusCreated, &stack)
	request(http.MethodPost, fmt.Sprintf("%s/%s/revisions/4/apply", root, stack.ID), "developer", map[string]any{}, http.StatusOK, &stack)
	if stack.CurrentRevision != 4 || stack.Timeline[0].State != "upstream_changed" || stack.Members[2].Authors[0] != "agent" {
		t.Fatalf("restack lineage or authorship was lost: %#v", stack)
	}

	landingPath := fmt.Sprintf("%s/%s/landings", root, stack.ID)
	request(http.MethodPost, landingPath, "maintainer", map[string]any{"expected_stack_revision": 4, "expected_target_revision": base, "mode": "ordered", "atomic_permitted": false}, http.StatusCreated, &stack)
	landing := stack.Landings[0]
	addEvidence := func(candidate stackedchanges.LandingCandidate, failedKind string) {
		for _, kind := range candidate.RequiredEvidence {
			status := "passed"
			if kind == failedKind {
				status = "failed"
			}
			request(http.MethodPost, fmt.Sprintf("%s/%s/candidates/%s/evidence", landingPath, landing.ID, candidate.ID), "maintainer", map[string]string{"kind": kind, "reference": kind + ":" + candidate.MemberID, "status": status}, http.StatusCreated, &stack)
		}
	}
	addEvidence(landing.Candidates[0], "")
	addEvidence(landing.Candidates[1], "required_check")
	addEvidence(landing.Candidates[2], "")
	request(http.MethodPost, landingPath+"/"+landing.ID+"/merge", "maintainer", map[string]any{"member_id": "foundation", "atomic": false}, http.StatusOK, &stack)
	landing = stack.Landings[0]
	if len(landing.MergedMembers) != 1 || landing.PausedFromMember != "docs" {
		t.Fatalf("failed middle check did not preserve the integrated prefix: %#v", landing)
	}
	// Correct the middle check, then let the target advance independently before
	// merge. The stale candidate fails closed and only the unmerged suffix rebuilds.
	middle := landing.Candidates[1]
	request(http.MethodPost, fmt.Sprintf("%s/%s/candidates/%s/evidence", landingPath, landing.ID, middle.ID), "maintainer", map[string]string{"kind": "required_check", "reference": "required_check:docs-retry", "status": "passed"}, http.StatusCreated, &stack)
	gitOutput(t, work, "switch", "--detach", revisedFoundation)
	write("TARGET.md", "Concurrent target maintenance.\n")
	targetAdvance := commit("Advance target independently", "TARGET.md")
	gitOutput(t, work, "push", "platform", targetAdvance+":refs/heads/main")
	request(http.MethodPost, landingPath+"/"+landing.ID+"/merge", "maintainer", map[string]any{"member_id": "docs", "atomic": false}, http.StatusConflict, &stack)
	request(http.MethodPost, landingPath+"/"+landing.ID+"/rebuild", "maintainer", map[string]string{"expected_target_revision": targetAdvance}, http.StatusCreated, &stack)
	landing = stack.Landings[0]
	if landing.Candidates[1].Status != "superseded" || len(landing.Candidates) != 5 || landing.Candidates[3].Generation != 2 {
		t.Fatalf("target advance did not rebuild only the unsafe suffix: %#v", landing)
	}
	for _, candidate := range landing.Candidates[3:] {
		addEvidence(candidate, "")
		request(http.MethodPost, landingPath+"/"+landing.ID+"/merge", "maintainer", map[string]any{"member_id": candidate.MemberID, "atomic": false}, http.StatusOK, &stack)
	}
	mainRef, _ := opened.ReadReference("refs/heads/main")
	landing = stack.Landings[0]
	if landing.Status != "merged" || len(landing.MergedMembers) != 3 || string(mainRef.ObjectID) != landing.Candidates[4].CandidateRevision || len(landing.Events) < 5 || len(landing.AuthorityGranted) != 0 {
		t.Fatalf("final integrated history or retained recovery trail is incomplete: %#v main=%s", landing, mainRef.ObjectID)
	}
}

func workflowHasStackBlocker(blockers []stackedchanges.Blocker, kind string) bool {
	for _, blocker := range blockers {
		if blocker.Kind == kind {
			return true
		}
	}
	return false
}

func workflowHasRewrite(revision stackedchanges.Revision, member, kind string) bool {
	for _, rewrite := range revision.CommitRewrites {
		if rewrite.MemberID == member && rewrite.Kind == kind {
			return true
		}
	}
	return false
}
