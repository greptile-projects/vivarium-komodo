package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancegraphs"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestProvenanceGraphIsRevisionExactAndPermissionAware(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "project", Visibility: repositories.Public})
	opened, _ := repos.Open(repo.ID)
	sourceBody := []byte("package main\n")
	source, _ := opened.WriteObject(storage.BlobObject, sourceBody)
	sum := sha256.Sum256(sourceBody)
	nodes := []provenancegraphs.Node{
		{ID: "file:main", Kind: "file", Label: "main.go", Audience: "public", License: "Apache-2.0", Confidence: 1, Citations: []provenancegraphs.Citation{{Path: "main.go", BlobSHA256: hex.EncodeToString(sum[:])}}, Obligations: []string{"retain notice"}, Claims: []string{"original implementation"}},
		{ID: "upstream:private", Kind: "upstream_project", Label: "private generator", Audience: "restricted", Confidence: .8, Citations: []provenancegraphs.Citation{{Reference: "private:generator", Revision: "v2"}}, Claims: []string{"generated implementation"}},
	}
	edges := []provenancegraphs.Edge{{From: "upstream:private", To: "file:main", Kind: "generated"}}
	decl, _ := json.Marshal(map[string]any{"nodes": nodes, "edges": edges})
	declaration, _ := opened.WriteObject(storage.BlobObject, decl)
	treeBody := append([]byte("100644 main.go\x00"), objectIDBytes(t, source)...)
	treeBody = append(treeBody, []byte("100644 provenance.json\x00")...)
	treeBody = append(treeBody, objectIDBytes(t, declaration)...)
	tree, _ := opened.WriteObject(storage.TreeObject, treeBody)
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor Claimed <claim@example.test> 1 +0000\ncommitter Builder <build@example.test> 1 +0000\n\nship\n", tree)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	if retained, err := relationshipBlob(repos, string(repo.ID), string(commit), "provenance.json"); err != nil || string(retained) != string(decl) {
		t.Fatalf("declaration blob mismatch: %q, %v", retained, err)
	}
	store, _ := provenancegraphs.New(t.TempDir())
	mux := http.NewServeMux()
	registerProvenanceGraphsHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	input, _ := json.Marshal(map[string]any{"revision": commit, "declaration_path": "provenance.json"})
	var made provenancegraphs.Graph
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/provenance-graphs", token, string(input), http.StatusCreated, &made)
	if made.Revision != string(commit) || made.Status != "complete" || len(made.Edges) != 1 {
		t.Fatalf("graph lost exact lineage: %#v", made)
	}
	var public struct {
		Items []provenancegraphs.Graph `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+string(repo.ID)+"/provenance-graphs", "", "", http.StatusOK, &public)
	if len(public.Items) != 1 || public.Items[0].Nodes[1].Label != "inaccessible provenance node" || len(public.Items[0].Nodes[1].Citations) != 0 {
		t.Fatalf("restricted provenance leaked: %#v", public)
	}
}
