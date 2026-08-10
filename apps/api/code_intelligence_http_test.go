package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/relationships"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestCodeIntelligenceIsRevisionExactAndLinksEvidence(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	credentials, _ := auth.New(t.TempDir())
	relations, _ := relationships.New(t.TempDir())
	item, _ := catalog.Create("owner", repositories.Metadata{Name: "project", Visibility: repositories.Public})
	provider, _ := catalog.Create("provider", repositories.Metadata{Name: "service", Visibility: repositories.Public})
	opened, _ := catalog.Open(item.ID)
	source, _ := opened.WriteObject(storage.BlobObject, []byte("package project\n\nfunc Serve() {}\nfunc Main() { Serve() }\n"))
	testSource, _ := opened.WriteObject(storage.BlobObject, []byte("package project\n\nfunc TestServe() { Serve() }\n"))
	rootContent := append([]byte("100644 main.go\x00"), objectIDBytes(t, source)...)
	rootContent = append(rootContent, []byte("100644 main_test.go\x00")...)
	rootContent = append(rootContent, objectIDBytes(t, testSource)...)
	root, _ := opened.WriteObject(storage.TreeObject, rootContent)
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor Ada <ada@example.test> 1722470400 +0000\ncommitter Ada <ada@example.test> 1722470400 +0000\n\nAdd server\n", root)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	_, _ = relations.Declare(relationships.Dependency{RepositoryID: string(item.ID), CommitID: string(commit), ProviderRepositoryID: string(provider.ID), InterfaceName: "http", Constraint: "^1.0.0", DeclaredByID: "owner"})

	mux := http.NewServeMux()
	registerCodeIntelligenceHTTP(mux, catalog, credentials, relations)
	server := httptest.NewServer(mux)
	defer server.Close()
	response, err := http.Get(server.URL + "/repositories/" + string(item.ID) + "/code-intelligence?ref=" + string(commit) + "&symbol=Serve")
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		CommitID     string           `json:"commit_id"`
		Symbols      []codeSymbol     `json:"symbols"`
		Dependencies []codeDependency `json:"dependencies"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&body) != nil {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if body.CommitID != string(commit) || len(body.Symbols) != 1 {
		t.Fatalf("body = %#v", body)
	}
	if len(body.Symbols[0].Callers) != 2 || len(body.Symbols[0].Tests) != 1 || body.Symbols[0].Owner == nil || body.Symbols[0].Owner.Author != "Ada" {
		t.Fatalf("symbol = %#v", body.Symbols[0])
	}
	if len(body.Dependencies) != 1 || body.Dependencies[0].State != "unresolved" {
		t.Fatalf("dependencies = %#v", body.Dependencies)
	}
}
