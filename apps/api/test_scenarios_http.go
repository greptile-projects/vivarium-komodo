package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/testscenarios"
)

func registerTestScenariosHTTP(mux *http.ServeMux, s *testscenarios.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/test-scenarios"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Catalog(string(repo.ID))
		if !testScenarioError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in testscenarios.Input
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !testScenarioError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{scenario}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("scenario"))
		if !testScenarioError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{scenario}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			testscenarios.Input
		}
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("scenario"), a.UserID, in.ExpectedVersion, in.Input)
		if !testScenarioError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func testScenarioError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, testscenarios.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "test_scenario_not_found"})
	case errors.Is(e, testscenarios.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "test_scenario_changed_or_conflicting"})
	case errors.Is(e, testscenarios.ErrUnsafeFixture):
		writeJSON(w, 422, map[string]string{"error": "unsafe_reusable_fixture"})
	case errors.Is(e, testscenarios.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_test_scenario"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
