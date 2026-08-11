package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type previewPullStore interface {
	Get(string, string) (pullrequests.PullRequest, error)
}
type previewRunner interface {
	Definition(string, string) (previews.Definition, string, error)
	Start(previews.Preview)
}
type previewAudienceSources interface {
	Issue(string, string) (issues.Issue, error)
	Decision(string, string) (decisions.Decision, error)
	Proposal(string, string) (proposals.Proposal, error)
	ProposalComments(string, string) ([]proposals.Comment, error)
}

type previewSources struct {
	issues    issueStore
	decisions decisionStore
	proposals proposalStore
}

func (s previewSources) Issue(r, id string) (issues.Issue, error) { return s.issues.Get(r, id) }
func (s previewSources) Decision(r, id string) (decisions.Decision, error) {
	return s.decisions.Get(r, id)
}
func (s previewSources) Proposal(r, id string) (proposals.Proposal, error) {
	return s.proposals.Get(r, id)
}
func (s previewSources) ProposalComments(r, id string) ([]proposals.Comment, error) {
	return s.proposals.ListComments(r, id)
}

func registerPreviewsHTTP(mux *http.ServeMux, store *previews.Store, runner previewRunner, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore, sources ...previewAudienceSources) {
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/previews", createPreview(store, runner, pulls, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/previews", listPreviews(store, pulls, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}", getPreview(store, pulls, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/audience", previewAudience(store, pulls, credentials))
	mux.HandleFunc("/repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/proxy/{path...}", proxyPreview(store, pulls, repositories, credentials))
	if len(sources) > 0 {
		mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/invitations", invitePreview(store, pulls, repositories, credentials, sources[0]))
		mux.HandleFunc("DELETE /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/invitations/{invitation}", revokePreviewInvitation(store, pulls, repositories, credentials))
	}
}

func previewAudience(store *previews.Store, pulls previewPullStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		grant, ok := authenticateRequest(w, r, c, auth.ProfileRead)
		if !ok {
			return
		}
		pull, e := pulls.Get(r.PathValue("repository"), r.PathValue("pull_request"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		item, in, e := store.Authorize(pull.RepositoryID, pull.ID, r.PathValue("preview"), grant.UserID)
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, map[string]any{"preview_id": item.ID, "revision": item.Revision, "state": item.State, "stale": item.Revision != pull.SourceCommitID, "role": in.Role, "expires_at": in.ExpiresAt, "audience": item.Definition.Audience, "effective_access": map[string]bool{"preview": true, "repository": false, "git": false, "workspace": false, "deployment": false, "production": false}, "configuration_values_exposed": false})
	}
}

func invitePreview(store *previews.Store, pulls previewPullStore, repos pullRequestRepositoryStore, c authStore, s previewAudienceSources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actor, ok := previewContext(w, r, pulls, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			UserID     string    `json:"user_id"`
			Role       string    `json:"role"`
			SourceKind string    `json:"source_kind"`
			SourceID   string    `json:"source_id"`
			ExpiresAt  time.Time `json:"expires_at"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		valid := in.SourceKind == "user" && in.SourceID == ""
		switch in.SourceKind {
		case "issue":
			if v, e := s.Issue(pull.RepositoryID, in.SourceID); e == nil {
				valid = v.ReporterID == in.UserID
				for _, x := range v.Triage.AssigneeIDs {
					valid = valid || x == in.UserID
				}
				for _, x := range v.Comments {
					valid = valid || x.AuthorID == in.UserID
				}
			}
		case "decision":
			if v, e := s.Decision(pull.RepositoryID, in.SourceID); e == nil {
				for _, x := range v.Scope.ParticipantIDs {
					valid = valid || x == in.UserID
				}
			}
		case "proposal":
			if v, e := s.Proposal(pull.RepositoryID, in.SourceID); e == nil {
				valid = v.AuthorID == in.UserID
				if xs, e := s.ProposalComments(pull.RepositoryID, in.SourceID); e == nil {
					for _, x := range xs {
						valid = valid || x.AuthorID == in.UserID
					}
				}
			}
		}
		if !valid {
			writeJSON(w, 422, map[string]string{"error": "user_not_source_participant"})
			return
		}
		v, e := store.Invite(pull.RepositoryID, pull.ID, r.PathValue("preview"), actor, previews.Invitation{UserID: in.UserID, Role: in.Role, SourceKind: in.SourceKind, SourceID: in.SourceID, ExpiresAt: in.ExpiresAt})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_invitation"})
			return
		}
		writeJSON(w, 201, v)
	}
}
func revokePreviewInvitation(store *previews.Store, pulls previewPullStore, repos pullRequestRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actor, ok := previewContext(w, r, pulls, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		v, e := store.Revoke(pull.RepositoryID, pull.ID, r.PathValue("preview"), r.PathValue("invitation"), actor)
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, v)
	}
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
		repo, err := repositories.Inspect(storage.ID(r.PathValue("repository")))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		pull, err := pulls.Get(string(repo.ID), r.PathValue("pull_request"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		grant, ok := authenticateRequest(w, r, credentials, auth.ProfileRead)
		if !ok {
			return
		}
		participant := grant.UserID == repo.OwnerID || string(repo.Visibility) == "public"
		if !participant {
			participant, _ = repositories.IsCollaborator(repo.ID, grant.UserID)
		}
		var invitation previews.Invitation
		if !participant {
			if _, invitation, err = store.Authorize(pull.RepositoryID, pull.ID, r.PathValue("preview"), grant.UserID); err != nil {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return
			}
		}
		item, err := store.Get(pull.RepositoryID, pull.ID, r.PathValue("preview"))
		if err != nil || item.State != "ready" || item.LocalPort == 0 {
			writeJSON(w, 409, map[string]string{"error": "preview_not_ready"})
			return
		}
		allowedAction := r.Method == http.MethodGet || r.Method == http.MethodHead
		for _, action := range item.Definition.Audience.Actions {
			if action == "submit_test_data" && invitation.Role == "test" {
				allowedAction = true
			}
		}
		if !allowedAction {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "preview_action_restricted"})
			return
		}
		w.Header().Set("X-Komodo-Preview-Network", item.Definition.Audience.Network)
		w.Header().Set("X-Komodo-Preview-Data", item.Definition.Audience.Data)
		w.Header().Set("X-Komodo-Preview-Identity", item.Definition.Audience.Identity)
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; form-action 'self'; frame-ancestors 'self'")
		target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", item.LocalPort))
		prefix := "/repositories/" + pull.RepositoryID + "/pull-requests/" + pull.ID + "/previews/" + item.ID + "/proxy"
		r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		original := proxy.Director
		proxy.Director = func(req *http.Request) {
			original(req)
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")
			req.Header.Del("X-Forwarded-User")
		}
		proxy.ServeHTTP(w, r)
	}
}
