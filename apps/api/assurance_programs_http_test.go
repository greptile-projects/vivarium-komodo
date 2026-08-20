package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceprograms"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestAssuranceProgramPublicAPIIsVersionedAndPermissionChecked(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "service", Visibility: repositories.Public})
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := assuranceprograms.New(t.TempDir())
	mux := http.NewServeMux()
	registerAssuranceProgramsHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/assurance-programs"
	in := assuranceprograms.Input{Name: "Service obligations", Description: "Applicable rules", Scope: "production", ChangeReason: "initial", Requirements: []assuranceprograms.Requirement{{ID: "contract", SourceKind: "contractual", SourceReference: "msa:security", SourceVersion: "v2", Title: "Encryption", Text: "encrypt transfers", Applicability: "customer data", Interpretation: "TLS at external crossings", AuthorID: "legal"}}}
	b, _ := json.Marshal(in)
	workflowJSON(t, server.URL, http.MethodPost, base, reader, string(b), http.StatusUnauthorized, nil)
	var created assuranceprograms.Program
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(b), http.StatusCreated, &created)
	unmapped := false
	for _, gap := range created.Gaps {
		unmapped = unmapped || gap.Kind == "unmapped_requirement"
	}
	if created.ClaimStatus != "gaps_explicit" || !unmapped {
		t.Fatalf("unmapped obligation hidden: %#v", created)
	}
	var catalog assuranceprograms.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &catalog)
	if len(catalog.Items) != 1 {
		t.Fatalf("public catalog unavailable: %#v", catalog)
	}
	in.ChangeReason = "reviewed"
	revision := struct {
		ExpectedVersion int64 `json:"expected_version"`
		assuranceprograms.Input
	}{1, in}
	b, _ = json.Marshal(revision)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, string(b), http.StatusCreated, &created)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, string(b), http.StatusConflict, nil)
}
