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
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestPublicRepositoryBrowserPreservesRevisionAndPathContext(t *testing.T) {
	gitStorage, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := repositories.New(t.TempDir(), gitStorage)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := auth.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	item, err := catalog.Create("owner", repositories.Metadata{Name: "project", Visibility: repositories.Public})
	if err != nil {
		t.Fatal(err)
	}
	opened, err := catalog.Open(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	readme, _ := opened.WriteObject(storage.BlobObject, []byte("# Project\n\nA useful project.\n"))
	source, _ := opened.WriteObject(storage.BlobObject, []byte("package project\n"))
	sourceTree, _ := opened.WriteObject(storage.TreeObject, append([]byte("100644 main.go\x00"), objectIDBytes(t, source)...))
	rootContent := append([]byte("100644 README.md\x00"), objectIDBytes(t, readme)...)
	rootContent = append(rootContent, []byte("40000 src\x00")...)
	rootContent = append(rootContent, objectIDBytes(t, sourceTree)...)
	root, _ := opened.WriteObject(storage.TreeObject, rootContent)
	commitContent := fmt.Sprintf("tree %s\nauthor Ada Lovelace <ada@example.test> 1722470400 +0000\ncommitter Ada Lovelace <ada@example.test> 1722470400 +0000\n\nExplain the project\n", root)
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(commitContent))
	if err := opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit}); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerRepositoryBrowserHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	for path, check := range map[string]func(map[string]any){
		"/branches": func(body map[string]any) {
			if body["default_branch"] != "main" || len(body["items"].([]any)) != 1 {
				t.Fatalf("branches = %#v", body)
			}
		},
		"/commits?ref=main": func(body map[string]any) {
			if body["commit_id"] != string(commit) || body["revision"] != "main" {
				t.Fatalf("commits = %#v", body)
			}
		},
		"/tree?ref=main&path=src": func(body map[string]any) {
			if body["path"] != "src" || body["commit_id"] != string(commit) {
				t.Fatalf("tree = %#v", body)
			}
		},
		"/blob?ref=main&path=README.md": func(body map[string]any) {
			if body["content"] != "# Project\n\nA useful project.\n" || body["binary"] != false {
				t.Fatalf("blob = %#v", body)
			}
		},
	} {
		response, err := http.Get(server.URL + "/repositories/" + string(item.ID) + path)
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&body) != nil {
			t.Fatalf("GET %s status = %d", path, response.StatusCode)
		}
		response.Body.Close()
		check(body)
	}
}

func objectIDBytes(t *testing.T, id storage.ObjectID) []byte {
	t.Helper()
	raw, err := hex.DecodeString(string(id))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
