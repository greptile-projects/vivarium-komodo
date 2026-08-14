package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitybarriers"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/docscollections"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

type accessibilityBarrierSources struct {
	releases interface {
		Get(string, string) (releases.Release, error)
	}
	docs interface {
		Get(string, string) (docscollections.Collection, error)
	}
	previews interface {
		GetByID(string) (previews.Preview, error)
	}
	workspaces interface {
		Get(string, string) (workspaces.Workspace, error)
	}
	repositories interface {
		Open(storage.ID) (*storage.Repository, error)
	}
}

func registerAccessibilityBarriersHTTP(mux *http.ServeMux, s *accessibilitybarriers.Store, repos proposalRepositoryStore, c authStore, src accessibilityBarrierSources) {
	base := "/repositories/{repository}/accessibility-barriers"
	access := func(w http.ResponseWriter, r *http.Request, scope auth.Scope) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, scope, true)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	maintainer := func(repo, actor string) bool {
		v, e := repos.Inspect(storage.ID(repo))
		if e != nil {
			return false
		}
		if v.OwnerID == actor {
			return true
		}
		for _, x := range v.CollaboratorIDs {
			if x == actor {
				return true
			}
		}
		return false
	}
	project := func(v accessibilitybarriers.Barrier, actor string) accessibilitybarriers.Barrier {
		m := maintainer(v.RepositoryID, actor) || actor == v.ReporterID
		reporter := v.ReporterID
		if v.IdentityVisibility == "maintainers" && !m {
			v.ReporterID = ""
		}
		if v.DeviceDataVisibility == "maintainers" && !m {
			v.Environment.SensitiveDeviceData = ""
		}
		for i := range v.Evidence {
			if v.Evidence[i].Visibility == "maintainers" && !m {
				v.Evidence[i].Content = ""
			}
		}
		for i := range v.Attempts {
			v.Attempts[i].Environment.SensitiveDeviceData = ""
			for j := range v.Attempts[i].Evidence {
				if v.Attempts[i].Evidence[j].Visibility == "maintainers" && !m {
					v.Attempts[i].Evidence[j].Content = ""
				}
			}
			if v.IdentityVisibility == "maintainers" && !m && v.Attempts[i].ActorID == reporter {
				v.Attempts[i].ActorID = ""
			}
		}
		return v
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, auth.RepositoryRead)
		if !ok {
			return
		}
		items, e := s.List(repo)
		if barrierError(w, e) {
			return
		}
		for i := range items {
			items[i] = project(items[i], a)
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, auth.RepositoryRead)
		if !ok {
			return
		}
		var in accessibilitybarriers.Input
		if !readJSON(w, r, &in, 6<<20) {
			return
		}
		if !validBarrierContext(repo, in.Context, src) {
			writeJSON(w, 422, map[string]string{"error": "invalid_accessibility_barrier_context"})
			return
		}
		v, e := s.Create(repo, a, in)
		if barrierError(w, e) {
			return
		}
		writeJSON(w, 201, project(v, a))
	})
	mux.HandleFunc("GET "+base+"/{barrier}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, auth.RepositoryRead)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("barrier"))
		if barrierError(w, e) {
			return
		}
		writeJSON(w, 200, project(v, a))
	})
	mux.HandleFunc("POST "+base+"/{barrier}/attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in accessibilitybarriers.AttemptInput
		if !readJSON(w, r, &in, 6<<20) {
			return
		}
		valid := false
		if in.ExecutionKind == "workspace" {
			x, e := src.workspaces.Get(repo, in.ExecutionID)
			valid = e == nil && x.Revision == in.Revision
		}
		if in.ExecutionKind == "preview" {
			x, e := src.previews.GetByID(in.ExecutionID)
			valid = e == nil && x.RepositoryID == repo && x.Revision == in.Revision
		}
		if !valid {
			writeJSON(w, 422, map[string]string{"error": "invalid_revision_exact_reproduction"})
			return
		}
		v, e := s.AddAttempt(repo, r.PathValue("barrier"), a, in)
		if barrierError(w, e) {
			return
		}
		writeJSON(w, 201, project(v, a))
	})
}
func validBarrierContext(repo string, c accessibilitybarriers.Context, s accessibilityBarrierSources) bool {
	switch c.Kind {
	case "release":
		v, e := s.releases.Get(repo, c.ResourceID)
		return e == nil && v.CommitID == c.Revision
	case "preview":
		v, e := s.previews.GetByID(c.ResourceID)
		return e == nil && v.RepositoryID == repo && v.Revision == c.Revision
	case "journey":
		v, e := s.docs.Get(repo, c.ResourceID)
		if e != nil {
			return false
		}
		for _, h := range v.History {
			for _, m := range h.Versions {
				if m.SourceRevision == c.Revision {
					return true
				}
			}
		}
		return false
	case "page":
		if c.Path == "" || c.ResourceID != repo {
			return false
		}
		r, e := s.repositories.Open(storage.ID(repo))
		if e != nil {
			return false
		}
		_, e = r.ReadCommit(storage.ObjectID(c.Revision))
		return e == nil
	}
	return false
}
func barrierError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, accessibilitybarriers.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "accessibility_barrier_not_found"})
	} else if errors.Is(e, accessibilitybarriers.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_accessibility_barrier"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
