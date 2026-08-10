package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/questions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/relationships"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestGroundedQuestionIsDurableRevisionExactAndStreamed(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	relations, _ := relationships.New(t.TempDir())
	conversationStore, _ := questions.New(t.TempDir())
	checkStore, _ := checkruns.New(t.TempDir())
	item, _ := catalog.Create("owner", repositories.Metadata{Name: "grounded", Visibility: repositories.Public})
	repo, _ := catalog.Open(item.ID)
	blob, _ := repo.WriteObject(storage.BlobObject, []byte("package sample\n\n// Authenticate validates the bearer token.\nfunc Authenticate(token string) bool { return token != \"\" }\n"))
	treeBytes := append([]byte("100644 auth.go\x00"), objectIDBytes(t, blob)...)
	tree, _ := repo.WriteObject(storage.TreeObject, treeBytes)
	commit, _ := repo.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor Ada <ada@example.test> 1722470400 +0000\ncommitter Ada <ada@example.test> 1722470400 +0000\n\nAdd authentication\n", tree)))
	_ = repo.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead)

	mux := http.NewServeMux()
	registerQuestionsHTTP(mux, conversationStore, catalog, credentials, relations, checkStore)
	server := httptest.NewServer(mux)
	defer server.Close()
	body, _ := json.Marshal(map[string]any{"question": "How does Authenticate validate the token?", "revision": "main", "context": map[string]string{"type": "file", "path": "auth.go"}})
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/repositories/"+string(item.ID)+"/questions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var conversation questions.Conversation
	if response.StatusCode != http.StatusCreated || json.NewDecoder(response.Body).Decode(&conversation) != nil {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if conversation.CommitID != string(commit) || conversation.ActorID != "owner" || len(conversation.Claims) == 0 {
		t.Fatalf("conversation = %#v", conversation)
	}
	citation := conversation.Claims[0].Citations[0]
	if citation.CommitID != string(commit) || citation.Path != "auth.go" || citation.LineStart == 0 || citation.ObjectID != string(blob) {
		t.Fatalf("citation = %#v", citation)
	}

	stream, _ := http.Get(server.URL + "/repositories/" + string(item.ID) + "/questions/" + conversation.ID + "/events?after=1")
	streamBody, _ := io.ReadAll(stream.Body)
	if stream.Header.Get("Content-Type") != "text/event-stream" || !strings.Contains(string(streamBody), "event: claim") || !strings.Contains(string(streamBody), "event: done") {
		t.Fatalf("stream = %s", streamBody)
	}
	listed, _ := conversationStore.List(string(item.ID))
	if len(listed) != 1 || listed[0].ID != conversation.ID || listed[0].CommitID != string(commit) {
		t.Fatalf("listed = %#v", listed)
	}
}
