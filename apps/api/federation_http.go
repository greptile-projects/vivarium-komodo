package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/federation"
)

func registerFederationHTTP(mux *http.ServeMux, store *federation.Store, credentials authStore) {
	mux.HandleFunc("GET /.well-known/komodo-federation", func(w http.ResponseWriter, r *http.Request) {
		doc, err := store.Document()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		writeJSON(w, 200, doc)
	})
	mux.HandleFunc("GET /federation/actors/{kind}/{id}", func(w http.ResponseWriter, r *http.Request) {
		doc, err := store.Document()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		for _, actor := range doc.Actors {
			if actor.Kind == r.PathValue("kind") && actor.LocalID == r.PathValue("id") {
				writeJSON(w, 200, map[string]any{"actor": actor, "instance": doc.Instance, "document_version": doc.Version, "key_id": doc.KeyID, "signature": doc.Signature})
				return
			}
		}
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	})
	mux.HandleFunc("GET /federation/peers", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticateRequest(w, r, credentials, auth.ProfileRead); !ok {
			return
		}
		items, err := store.Peers()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	})
	mux.HandleFunc("POST /federation/actors", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticateRequest(w, r, credentials, auth.ProfileWrite); !ok {
			return
		}
		var in struct {
			Kind        string `json:"kind"`
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			ProfileURL  string `json:"profile_url"`
		}
		if !readJSON(w, r, &in, 8192) {
			return
		}
		actor, err := store.PublishActor(in.Kind, in.ID, in.DisplayName, in.ProfileURL)
		if err != nil {
			writeFederationError(w, err)
			return
		}
		writeJSON(w, 201, actor)
	})
	mux.HandleFunc("POST /federation/keys/rotations", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticateRequest(w, r, credentials, auth.ProfileWrite); !ok {
			return
		}
		doc, err := store.Rotate()
		if err != nil {
			writeFederationError(w, err)
			return
		}
		writeJSON(w, 201, doc)
	})
	mux.HandleFunc("POST /federation/peers/discoveries", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticateRequest(w, r, credentials, auth.ProfileWrite); !ok {
			return
		}
		var in struct {
			URL string `json:"url"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		u, parseErr := url.Parse(in.URL)
		if parseErr != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			writeJSON(w, 422, map[string]string{"error": "invalid_discovery_url"})
			return
		}
		client := &http.Client{Timeout: 5 * time.Second}
		response, err := client.Get(in.URL)
		if err != nil {
			peer, _ := store.Observe(in.URL, federation.Document{}, err)
			writeJSON(w, 202, peer)
			return
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			peer, _ := store.Observe(in.URL, federation.Document{}, errors.New("peer returned "+response.Status))
			writeJSON(w, 202, peer)
			return
		}
		var doc federation.Document
		if err = json.NewDecoder(http.MaxBytesReader(w, response.Body, 1<<20)).Decode(&doc); err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_federation_document"})
			return
		}
		peer, err := store.Observe(in.URL, doc, nil)
		if err != nil {
			writeFederationError(w, err)
			return
		}
		writeJSON(w, 200, peer)
	})
	mux.HandleFunc("POST /federation/peer-trust", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticateRequest(w, r, credentials, auth.ProfileWrite); !ok {
			return
		}
		var in struct {
			Instance string `json:"instance"`
			Action   string `json:"action"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		peer, err := store.Trust(in.Instance, in.Action)
		if err != nil {
			writeFederationError(w, err)
			return
		}
		writeJSON(w, 200, peer)
	})
}

func writeFederationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, federation.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(err, federation.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "federation_conflict"})
	default:
		writeJSON(w, 422, map[string]string{"error": "invalid_federation_resource"})
	}
}
