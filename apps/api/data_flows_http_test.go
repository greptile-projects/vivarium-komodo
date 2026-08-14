package main

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/datacommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dataflows"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestDataFlowPublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "privacy-map", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reader")
	r, _ := catalog.Open(repo.ID)
	source, _ := r.WriteObject(storage.BlobObject, []byte("func collect() {}\n"))
	manifest, _ := r.WriteObject(storage.BlobObject, []byte(`{"schema_version":1}`))
	komodo, _ := r.WriteObject(storage.TreeObject, dataFlowTreeEntry("100644", "data-flows.json", manifest))
	root, _ := r.WriteObject(storage.TreeObject, append(dataFlowTreeEntry("40000", ".komodo", komodo), dataFlowTreeEntry("100644", "analytics.go", source)...))
	revision, _ := r.WriteObject(storage.CommitObject, []byte("tree "+string(root)+"\nauthor Owner <o@example.test> 1 +0000\ncommitter Owner <o@example.test> 1 +0000\n\nflow\n"))
	cs, _ := datacommitments.New(t.TempDir())
	commitment, _ := cs.Create(string(repo.ID), "owner", datacommitments.VersionInput{Title: "Analytics", Scopes: []datacommitments.Scope{{Kind: "repository", Name: "all"}}, DataUses: []datacommitments.DataUse{{ID: "usage", Name: "Usage", Categories: []string{"event"}, Purposes: []string{"improve"}, Subjects: []string{"user"}, Collection: "UI", Processing: []string{"count"}, Sharing: []string{"processor"}, Retention: "30 days", Residency: []string{"EU"}, Deletion: "delete", Consent: "opt in", OwnerIDs: []string{"owner"}}}, Guarantees: []datacommitments.Guarantee{{ID: "eu", Description: "EU", Status: "supported"}}, OwnerIDs: []string{"owner"}, Links: []datacommitments.Link{{Kind: "policy", URL: "https://example.test/p"}, {Kind: "notice", URL: "https://example.test/n"}}, ChangeReason: "publish"})
	flows, _ := dataflows.New(t.TempDir())
	mux := http.NewServeMux()
	registerDataFlowsHTTP(mux, flows, cs, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	ownerToken := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	readerToken := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	in := dataflows.DeclarationInput{Revision: string(revision), Title: "Feature usage", Manifest: dataflows.Location{Path: ".komodo/data-flows.json"}, Commitments: []dataflows.CommitmentRef{{ID: commitment.ID, Version: 1, DataUseIDs: []string{"usage"}}}, Nodes: []dataflows.Node{{ID: "ui", Kind: "interaction", Name: "click", Location: &dataflows.Location{Path: "analytics.go"}, EvidenceAccessible: true}, {ID: "api", Kind: "interface", Name: "events API", Location: &dataflows.Location{Path: "analytics.go"}, EvidenceAccessible: true}, {ID: "processor", Kind: "external_recipient", Name: "metrics processor", EvidenceAccessible: false, RestrictedEvidenceRef: "vendor-contract:metrics"}}, Edges: []dataflows.Edge{{ID: "collect", From: "ui", To: "api", Action: "enters", Categories: []string{"event"}, Purpose: "improve"}, {ID: "share", From: "api", To: "processor", Action: "leaves", Categories: []string{"event"}, Purpose: "improve"}}}
	body, _ := json.Marshal(in)
	var flow dataflows.Flow
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/data-flows", ownerToken, string(body), 201, &flow)
	if flow.Manifest.BlobID != string(manifest) || flow.Nodes[0].Location.BlobID != string(source) {
		t.Fatalf("source identities not captured: %+v", flow)
	}
	finding := dataflows.FindingInput{Summary: "runtime trace found a store", Uncertainty: "processor internals unavailable", ObservedEdges: []dataflows.Edge{{ID: "persist", From: "api", To: "processor", Action: "persists", Categories: []string{"event"}, Purpose: "support"}}, Citations: []dataflows.Citation{{Kind: "code", Location: &dataflows.Location{Path: "analytics.go"}, EvidenceAccessible: true}, {Kind: "dependency", ResourceID: "metrics", EvidenceAccessible: false, RestrictedEvidenceRef: "vendor-assessment:2026"}}}
	body, _ = json.Marshal(finding)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/data-flows/"+flow.ID+"/findings", readerToken, string(body), 201, &flow)
	kinds := map[string]bool{}
	for _, b := range flow.Blockers {
		kinds[b.Kind] = true
	}
	if !kinds["undeclared_flow"] || !kinds["declared_observed_difference"] || !kinds["inaccessible_dependency"] || flow.Findings[0].ActorID != "reader" {
		t.Fatalf("missing projected evidence: %+v", flow)
	}
}

func dataFlowTreeEntry(mode, name string, id storage.ObjectID) []byte {
	raw, _ := hex.DecodeString(string(id))
	return append([]byte(mode+" "+name+"\x00"), raw...)
}

func TestDataFlowNewRevisionMakesPriorAnalysisStale(t *testing.T) {
	s, _ := dataflows.New(t.TempDir())
	base := dataflows.DeclarationInput{Revision: "012345", Title: "flow", Manifest: dataflows.Location{Path: ".komodo/data-flows.json", BlobID: "blob"}, Commitments: []dataflows.CommitmentRef{{ID: "c", Version: 1, DataUseIDs: []string{"u"}}}, Nodes: []dataflows.Node{{ID: "a", Kind: "interaction", Name: "a", Location: &dataflows.Location{Path: "a", BlobID: "a"}, EvidenceAccessible: true}, {ID: "b", Kind: "store", Name: "b", Location: &dataflows.Location{Path: "b", BlobID: "b"}, EvidenceAccessible: true}}, Edges: []dataflows.Edge{{ID: "e", From: "a", To: "b", Action: "persists", Categories: []string{"event"}, Purpose: "support"}}}
	first, _ := s.Create("repo", "owner", base)
	time.Sleep(time.Millisecond)
	base.Revision = "67890"
	_, _ = s.Create("repo", "owner", base)
	got, _ := s.Get("repo", first.ID)
	if !got.Stale || got.Blockers[0].Kind != "stale_analysis" {
		t.Fatalf("prior analysis should be stale: %+v", got)
	}
}
