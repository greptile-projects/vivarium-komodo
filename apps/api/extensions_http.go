package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/extensions"
)

func registerExtensionsHTTP(mux *http.ServeMux, s *extensions.Store, repos ownedRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /extensions", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.ProfileWrite)
		if !ok {
			return
		}
		var in extensions.Input
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, err := s.Create(a.UserID, in)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_extension"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET /extensions", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.ProfileRead)
		if !ok {
			return
		}
		xs, err := s.List(a.UserID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": xs, "total_count": len(xs)})
	})
	mux.HandleFunc("GET /extensions/{extension}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.ProfileRead)
		if !ok {
			return
		}
		x, err := s.Get(r.PathValue("extension"))
		if errors.Is(err, extensions.ErrNotFound) || err == nil && x.OwnerID != a.UserID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST /extensions/{extension}/endpoint-verifications", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.ProfileWrite)
		if !ok {
			return
		}
		var in struct {
			Endpoint string `json:"endpoint"`
			Token    string `json:"token"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		x, err := s.Verify(r.PathValue("extension"), a.UserID, in.Endpoint, in.Token)
		if err != nil {
			writeExtensionError(w, err)
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST /repositories/{repository}/extension-authority-previews", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if repo.OwnerID != a.UserID {
			writeJSON(w, 403, map[string]string{"error": "owner_required"})
			return
		}
		var in struct {
			ExtensionID string   `json:"extension_id"`
			Permissions []string `json:"permissions"`
			EventTypes  []string `json:"event_types"`
		}
		if !readJSON(w, r, &in, 16384) {
			return
		}
		v, err := s.Preview(in.ExtensionID, string(repo.ID), in.Permissions, in.EventTypes)
		if err != nil {
			writeExtensionError(w, err)
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST /repositories/{repository}/extension-installations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if repo.OwnerID != a.UserID {
			writeJSON(w, 403, map[string]string{"error": "owner_required"})
			return
		}
		var in struct {
			ExtensionID string   `json:"extension_id"`
			Permissions []string `json:"permissions"`
			EventTypes  []string `json:"event_types"`
		}
		if !readJSON(w, r, &in, 16384) {
			return
		}
		v, err := s.Install(in.ExtensionID, string(repo.ID), a.UserID, in.Permissions, in.EventTypes)
		if err != nil {
			writeExtensionError(w, err)
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("GET /repositories/{repository}/extension-installations", func(w http.ResponseWriter, r *http.Request) {
		_, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		xs, err := s.ListInstallations(r.PathValue("repository"))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": xs, "total_count": len(xs)})
	})
	mux.HandleFunc("DELETE /repositories/{repository}/extension-installations/{installation}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if repo.OwnerID != a.UserID {
			writeJSON(w, 403, map[string]string{"error": "owner_required"})
			return
		}
		x, err := s.Revoke(string(repo.ID), r.PathValue("installation"), a.UserID)
		if err != nil {
			writeExtensionError(w, err)
			return
		}
		writeJSON(w, 200, x)
	})
}

func writeExtensionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, extensions.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(err, extensions.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
	default:
		writeJSON(w, 422, map[string]string{"error": "invalid_extension_authority"})
	}
}
