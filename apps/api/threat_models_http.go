package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/threatmodels"
)

type threatModelSources struct {
	pulls interface {
		Get(string, string) (pullrequests.PullRequest, error)
	}
}

func registerThreatModelsHTTP(mux *http.ServeMux, s *threatmodels.Store, repos proposalRepositoryStore, c authStore, src threatModelSources) {
	base := "/repositories/{repository}/threat-models"
	access := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	project := func(repo string, m *threatmodels.Model) {
		current := map[string]string{}
		if m.Origin.Kind == "pull_request" {
			if p, e := src.pulls.Get(repo, m.Origin.Reference); e == nil {
				current["origin:"+m.Origin.Reference] = p.SourceCommitID
				for _, x := range m.Inputs {
					if x.Kind == "code" {
						current[x.Kind+":"+x.Reference] = p.SourceCommitID
					}
				}
			}
		}
		threatmodels.Derive(m, current)
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r)
		if !ok {
			return
		}
		items, e := s.List(repo)
		if threatModelError(w, e) {
			return
		}
		for i := range items {
			project(repo, &items[i])
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in threatmodels.Input
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		if in.Origin.Kind == "pull_request" {
			p, e := src.pulls.Get(repo, in.Origin.Reference)
			if e != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_threat_model_origin"})
				return
			}
			if p.SourceCommitID != in.Origin.Revision {
				writeJSON(w, 409, map[string]string{"error": "exact_origin_revision_required"})
				return
			}
		}
		m, e := s.Create(repo, actor, in)
		if threatModelError(w, e) {
			return
		}
		project(repo, &m)
		writeJSON(w, 201, m)
	})
	mux.HandleFunc("GET "+base+"/{model}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r)
		if !ok {
			return
		}
		m, e := s.Get(repo, r.PathValue("model"))
		if threatModelError(w, e) {
			return
		}
		project(repo, &m)
		writeJSON(w, 200, m)
	})
	mux.HandleFunc("POST "+base+"/{model}/findings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in threatmodels.FindingInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		m, e := s.AddFinding(repo, r.PathValue("model"), actor, in)
		if threatModelError(w, e) {
			return
		}
		project(repo, &m)
		writeJSON(w, 201, m)
	})
	mux.HandleFunc("POST "+base+"/{model}/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			Decision       string `json:"decision"`
			Rationale      string `json:"rationale"`
			OriginRevision string `json:"origin_revision"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		m, e := s.Acknowledge(repo, r.PathValue("model"), actor, in.Decision, in.Rationale, in.OriginRevision)
		if threatModelError(w, e) {
			return
		}
		project(repo, &m)
		writeJSON(w, 201, m)
	})
}
func threatModelError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, threatmodels.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "threat_model_not_found"})
	} else if errors.Is(e, threatmodels.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_threat_model"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
