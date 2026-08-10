package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/investigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/questions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

func TestSharedInvestigationRetainsChallengesInvitesAndStaleEvidence(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	canvas, _ := investigations.New(t.TempDir())
	workspaceStore, _ := workspaces.New(t.TempDir())
	questionStore, _ := questions.New(t.TempDir())
	item, _ := catalog.Create("owner", repositories.Metadata{Name: "inquiry", Visibility: repositories.Public})
	_, _ = catalog.AddCollaborator("owner", item.ID, "peer")
	repo, _ := catalog.Open(item.ID)
	blob, _ := repo.WriteObject(storage.BlobObject, []byte("package inquiry\n\nfunc Route() string { return \"stable\" }\n"))
	treeBytes := append([]byte("100644 route.go\x00"), objectIDBytes(t, blob)...)
	tree, _ := repo.WriteObject(storage.TreeObject, treeBytes)
	commit, _ := repo.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor Ada <ada@example.test> 1 +0000\ncommitter Ada <ada@example.test> 1 +0000\n\nroute\n", tree)))
	_ = repo.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	peer := issueAccess(t, credentials, "peer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerInvestigationsHTTP(mux, canvas, catalog, credentials, workspaceStore, questionStore)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := server.URL + "/repositories/" + string(item.ID) + "/investigations"
	var created investigations.Investigation
	conversation, _ := questionStore.Create(questions.Conversation{RepositoryID: string(item.ID), CommitID: string(commit), ActorID: "owner", Question: "How does routing work?"})
	investigationJSON(t, http.MethodPost, base, owner, map[string]any{"title": "Trace routing", "question": "Why is the route stable?", "revision": "main", "conversation_id": conversation.ID}, http.StatusCreated, &created)
	if created.CommitID != string(commit) || len(created.Runs) != 1 {
		t.Fatalf("created = %#v", created)
	}
	if created.ConversationID != conversation.ID {
		t.Fatalf("grounded conversation was not retained: %#v", created)
	}
	investigationJSON(t, http.MethodGet, base+"/"+created.ID, peer, nil, http.StatusNotFound, nil)
	investigationJSON(t, http.MethodPost, base+"/"+created.ID+"/participants", owner, map[string]string{"user_id": "peer"}, http.StatusOK, &created)
	investigationJSON(t, http.MethodPost, base+"/"+created.ID+"/entries", owner, map[string]any{"type": "hypothesis", "body": "The handler returns a constant.", "citations": []map[string]any{{"kind": "source", "path": "route.go", "line_start": 3}}}, http.StatusCreated, &created)
	claim := created.Entries[0]
	if claim.ActorID != "owner" || claim.Citations[0].ObjectID != string(blob) {
		t.Fatalf("claim = %#v", claim)
	}
	workspace, _ := workspaceStore.Create(string(item.ID), string(commit), "owner", workspaces.SourceContext{Type: "repository"}, workspaces.Access{RepositoryID: string(item.ID), ActorID: "owner", Permission: "write"}, workspaces.Definition{}, "definition")
	workspace, _ = workspaceStore.Finish(workspace.ID, true, "ready")
	workspace, _ = workspaceStore.RecordActivity(string(item.ID), workspace.ID, workspaces.Event{Type: "command", Kind: "observation", ActorID: "owner", Message: "private output is not copied"})
	investigationJSON(t, http.MethodPost, base+"/"+created.ID+"/entries", owner, map[string]any{"type": "runtime_observation", "body": "The bounded reproduction returned the stable route.", "citations": []map[string]any{{"kind": "workspace_observation", "workspace_id": workspace.ID, "workspace_sequence": workspace.Activity[0].Sequence}}}, http.StatusCreated, &created)
	if created.Entries[1].Citations[0].CommitID != string(commit) || created.Entries[1].Body == workspace.Activity[0].Message {
		t.Fatalf("workspace observation = %#v", created.Entries[1])
	}
	investigationJSON(t, http.MethodPost, base+"/"+created.ID+"/entries", peer, map[string]any{"type": "challenge", "body": "That explains output, not why callers rely on it.", "challenges": claim.ID}, http.StatusCreated, &created)
	if created.Entries[2].ActorID != "peer" || created.Entries[2].Challenges != claim.ID {
		t.Fatalf("challenge = %#v", created.Entries[2])
	}
	commit2, _ := repo.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor Ada <ada@example.test> 2 +0000\ncommitter Ada <ada@example.test> 2 +0000\n\nrecheck\n", tree, commit)))
	_ = repo.UpdateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit2})
	investigationJSON(t, http.MethodPost, base+"/"+created.ID+"/runs", peer, map[string]string{"revision": "main", "reason": "Branch advanced"}, http.StatusCreated, &created)
	if created.CommitID != string(commit2) || len(created.Runs) != 2 || !created.Entries[0].Stale || !created.Entries[1].Stale || !created.Entries[2].Stale {
		t.Fatalf("rerun = %#v", created)
	}
}

func investigationJSON(t *testing.T, method, url, token string, body any, status int, out any) {
	t.Helper()
	var data []byte
	if body != nil {
		data, _ = json.Marshal(body)
	}
	req, _ := http.NewRequest(method, url, bytes.NewReader(data))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != status {
		t.Fatalf("%s %s = %d", method, url, response.StatusCode)
	}
	if out != nil && json.NewDecoder(response.Body).Decode(out) != nil {
		t.Fatal("decode response")
	}
}
