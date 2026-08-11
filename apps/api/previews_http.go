package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
)

type previewPullStore interface {
	Get(string, string) (pullrequests.PullRequest, error)
}
type previewRunner interface {
	Definition(string, string) (previews.Definition, string, error)
	Start(previews.Preview)
}

func registerPreviewsHTTP(mux *http.ServeMux, store *previews.Store, runner previewRunner, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/previews", createPreview(store, runner, pulls, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/previews", listPreviews(store, pulls, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}", getPreview(store, pulls, repositories, credentials))
	mux.HandleFunc("/repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/proxy/{path...}", proxyPreview(store, pulls, repositories, credentials))
}

func previewContext(w http.ResponseWriter, r *http.Request, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore, scope auth.Scope, requireActor bool) (pullrequests.PullRequest, string, bool) {
	repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, scope, requireActor)
	if !ok {
		return pullrequests.PullRequest{}, "", false
	}
	pull, err := pulls.Get(string(repo.ID), r.PathValue("pull_request"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return pull, "", false
	}
	return pull, actor.UserID, true
}

func createPreview(store *previews.Store, runner previewRunner, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actor, ok := previewContext(w, r, pulls, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if pull.Status != pullrequests.Open {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "pull_request_not_open"})
			return
		}
		var input struct {
			Configuration map[string]string `json:"configuration"`
		}
		if !readJSON(w, r, &input, 64<<10) {
			return
		}
		definition, definitionDigest, err := runner.Definition(pull.SourceRepositoryID, pull.SourceCommitID)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid_preview_definition", "message": err.Error()})
			return
		}
		if input.Configuration == nil {
			input.Configuration = map[string]string{}
		}
		allowed := map[string]bool{}
		for _, key := range definition.Configuration {
			allowed[key] = true
		}
		for key, value := range input.Configuration {
			if !allowed[key] || len(value) > 2000 {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid_preview_configuration"})
				return
			}
		}
		for _, key := range definition.Configuration {
			if _, found := input.Configuration[key]; !found {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "missing_preview_configuration", "name": key})
				return
			}
		}
		keys := make([]string, 0, len(input.Configuration))
		for key := range input.Configuration {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		hash := sha256.New()
		for _, key := range keys {
			_, _ = fmt.Fprintf(hash, "%s\x00%s\x00", key, input.Configuration[key])
		}
		item, err := store.Create(previews.Preview{RepositoryID: pull.RepositoryID, SourceRepositoryID: pull.SourceRepositoryID, PullRequestID: pull.ID, Revision: pull.SourceCommitID, CreatorID: actor, Definition: definition, Configuration: input.Configuration, Attestation: previews.Attestation{CommitID: pull.SourceCommitID, DefinitionDigest: definitionDigest, ConfigurationDigest: fmt.Sprintf("%x", hash.Sum(nil))}})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		runner.Start(item)
		writeJSON(w, http.StatusCreated, item)
	}
}

func listPreviews(store *previews.Store, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, _, ok := previewContext(w, r, pulls, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.List(pull.RepositoryID, pull.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		for i := range items {
			items[i].Stale = items[i].Revision != pull.SourceCommitID
		}
		writeJSON(w, 200, map[string]any{"items": items})
	}
}
func getPreview(store *previews.Store, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, _, ok := previewContext(w, r, pulls, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, err := store.Get(pull.RepositoryID, pull.ID, r.PathValue("preview"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		item.Stale = item.Revision != pull.SourceCommitID
		writeJSON(w, 200, item)
	}
}
func proxyPreview(store *previews.Store, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, _, ok := previewContext(w, r, pulls, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, err := store.Get(pull.RepositoryID, pull.ID, r.PathValue("preview"))
		if err != nil || item.State != "ready" || item.LocalPort == 0 {
			writeJSON(w, 409, map[string]string{"error": "preview_not_ready"})
			return
		}
		target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", item.LocalPort))
		prefix := "/repositories/" + pull.RepositoryID + "/pull-requests/" + pull.ID + "/previews/" + item.ID + "/proxy"
		r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		httputil.NewSingleHostReverseProxy(target).ServeHTTP(w, r)
	}
}
