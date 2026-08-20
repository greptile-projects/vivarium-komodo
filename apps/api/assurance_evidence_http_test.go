package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceevidence"
	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceprograms"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestAssuranceEvidencePublicAPIRequiresOwnerAndProjectsAudience(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "service", Visibility: repositories.Public})
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	ps, _ := assuranceprograms.New(t.TempDir())
	p, _ := ps.Create(string(repo.ID), "owner", assuranceprograms.Input{Name: "Assurance", Description: "scope", Scope: "prod", ChangeReason: "initial", Requirements: []assuranceprograms.Requirement{{ID: "r", SourceKind: "organization", SourceReference: "policy", SourceVersion: "1", Title: "checks", Text: "run checks", Applicability: "all", Interpretation: "required checks", AuthorID: "owner"}}, Controls: []assuranceprograms.Control{{ID: "checks", Objective: "verify builds", Claim: "checks run", ReviewPeriod: "monthly", RequirementIDs: []string{"r"}, OwnerIDs: []string{"owner"}}}})
	es, _ := assuranceevidence.New(t.TempDir(), ps)
	mux := http.NewServeMux()
	registerAssuranceEvidenceHTTP(mux, es, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/assurance-programs/" + p.ID + "/evidence"
	in := assuranceevidence.QueryInput{ControlVersion: 1, ControlID: "checks", Name: "required check runs", Kind: "check", Source: "/checks", Schedule: "hourly", FreshnessHours: 24, Audience: "repository"}
	b, _ := json.Marshal(in)
	workflowJSON(t, server.URL, http.MethodPost, base+"/queries", reader, string(b), http.StatusUnauthorized, nil)
	var q assuranceevidence.Query
	workflowJSON(t, server.URL, http.MethodPost, base+"/queries", owner, string(b), http.StatusCreated, &q)
	var anonymous assuranceevidence.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &anonymous)
	if len(anonymous.Queries) != 0 {
		t.Fatalf("private query leaked publicly: %#v", anonymous)
	}
	var visible assuranceevidence.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, reader, "", http.StatusOK, &visible)
	if len(visible.Queries) != 1 {
		t.Fatalf("repository reader cannot inspect evidence query: %#v", visible)
	}
}
