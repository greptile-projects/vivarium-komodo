package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/extensions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type extensionOrganizations interface{ IsOwner(string, string) bool }

func registerExtensionsHTTP(mux *http.ServeMux, s *extensions.Store, repos ownedRepositoryStore, orgs extensionOrganizations, credentials authStore, activity activityStore) {
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
		if errors.Is(err, extensions.ErrNotFound) || err == nil && x.OwnerID != a.UserID && x.Status != "verified" {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST /extensions/{extension}/transfer", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.ProfileWrite)
		if !ok {
			return
		}
		var in struct {
			OwnerID string `json:"owner_id"`
			Reason  string `json:"reason"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		x, err := s.Transfer(r.PathValue("extension"), a.UserID, in.OwnerID, in.Reason)
		if err != nil {
			writeExtensionError(w, err)
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
	mux.HandleFunc("POST /organizations/{organization}/extension-installations", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		organization := r.PathValue("organization")
		if !orgs.IsOwner(organization, a.UserID) {
			writeJSON(w, 403, map[string]string{"error": "owner_required"})
			return
		}
		var in struct {
			ExtensionID   string   `json:"extension_id"`
			RepositoryIDs []string `json:"repository_ids"`
			extensions.GrantInput
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		if len(in.RepositoryIDs) == 0 || len(in.RepositoryIDs) > 100 {
			writeJSON(w, 422, map[string]string{"error": "invalid_extension_authority"})
			return
		}
		out := make([]extensions.Installation, 0, len(in.RepositoryIDs))
		seen := map[string]bool{}
		for _, id := range in.RepositoryIDs {
			if seen[id] {
				writeJSON(w, 422, map[string]string{"error": "invalid_extension_authority"})
				return
			}
			seen[id] = true
			repo, err := repos.Inspect(storage.ID(id))
			if err != nil || repo.OrganizationID != organization {
				writeJSON(w, 422, map[string]string{"error": "repository_outside_organization"})
				return
			}
		}
		for _, id := range in.RepositoryIDs {
			installed, err := s.InstallGrant(in.ExtensionID, id, a.UserID, in.GrantInput)
			if err != nil {
				writeExtensionError(w, err)
				return
			}
			out = append(out, installed)
		}
		writeJSON(w, 201, map[string]any{"items": out, "total_count": len(out)})
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
			ExtensionID string `json:"extension_id"`
			extensions.GrantInput
		}
		if !readJSON(w, r, &in, 16384) {
			return
		}
		v, err := s.InstallGrant(in.ExtensionID, string(repo.ID), a.UserID, in.GrantInput)
		if err != nil {
			writeExtensionError(w, err)
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("PATCH /repositories/{repository}/extension-installations/{installation}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if repo.OwnerID != a.UserID {
			writeJSON(w, 403, map[string]string{"error": "owner_required"})
			return
		}
		var in struct {
			Action          string                 `json:"action"`
			Reason          string                 `json:"reason"`
			ExpectedVersion int64                  `json:"expected_version"`
			Grant           *extensions.GrantInput `json:"grant,omitempty"`
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		x, err := s.Update(string(repo.ID), r.PathValue("installation"), a.UserID, in.Action, in.Reason, in.ExpectedVersion, in.Grant)
		if err != nil {
			writeExtensionError(w, err)
			return
		}
		writeJSON(w, 200, x)
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
	mux.HandleFunc("GET /repositories/{repository}/extension-installations/{installation}/deliveries", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		items, err := activity.List(string(repo.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		deliveries, err := s.Reconcile(string(repo.ID), r.PathValue("installation"), items)
		if err != nil {
			writeExtensionError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": deliveries, "total_count": len(deliveries), "schema_version": extensions.DeliverySchemaVersion})
	})
	mux.HandleFunc("POST /repositories/{repository}/extension-installations/{installation}/deliveries/{delivery}/attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if repo.OwnerID != a.UserID {
			writeJSON(w, 403, map[string]string{"error": "owner_required"})
			return
		}
		var in struct {
			Replay bool `json:"replay"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		events, err := activity.List(string(repo.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		deliveries, err := s.Reconcile(string(repo.ID), r.PathValue("installation"), events)
		if err != nil {
			writeExtensionError(w, err)
			return
		}
		var selected *extensions.Delivery
		for n := range deliveries {
			if deliveries[n].ID == r.PathValue("delivery") {
				selected = &deliveries[n]
			}
		}
		if selected == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		installation, extension, key, err := s.DeliveryContext(string(repo.ID), r.PathValue("installation"))
		if err != nil {
			writeExtensionError(w, err)
			return
		}
		if installation.Status != "active" {
			writeJSON(w, 403, map[string]string{"error": "installation_inactive"})
			return
		}
		now := time.Now().UTC()
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, extension.Callback.URL, bytes.NewReader(selected.Payload))
		if err == nil {
			req.Header.Set("Content-Type", "application/vnd.komodo.extension-event+json; version=1")
			req.Header.Set("X-Komodo-Delivery", selected.ID)
			req.Header.Set("X-Komodo-Event", selected.EventType)
			req.Header.Set("X-Komodo-Ordering-ID", strconv.FormatInt(selected.OrderingID, 10))
			req.Header.Set("X-Komodo-Timestamp", strconv.FormatInt(now.Unix(), 10))
			req.Header.Set("X-Komodo-Signature-256", extensions.Sign(key, now, selected.Payload))
		}
		outcome, message, code := "failed", "", 0
		if err == nil {
			client := &http.Client{Timeout: 10 * time.Second}
			resp, e := client.Do(req)
			if e != nil {
				message = e.Error()
			} else {
				code = resp.StatusCode
				_ = resp.Body.Close()
				if code >= 200 && code < 300 {
					outcome = "delivered"
				} else {
					message = fmt.Sprintf("callback returned HTTP %d", code)
				}
			}
		} else {
			message = err.Error()
		}
		updated, err := s.RecordAttempt(string(repo.ID), installation.ID, selected.ID, outcome, code, message, in.Replay)
		if err != nil {
			writeExtensionError(w, err)
			return
		}
		writeJSON(w, 200, updated)
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
