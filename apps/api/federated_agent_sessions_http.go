package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/federatedagents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/federation"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type federatedAgentRepositoryStore interface {
	ownedRepositoryStore
	Open(storage.ID) (*storage.Repository, error)
}

func registerFederatedAgentSessionsHTTP(mux *http.ServeMux, store *federatedagents.Store, fed *federation.Store, repos federatedAgentRepositoryStore, credentials changeSessionCredentialStore) {
	base := "/federation/repositories/{repository}/agent-sessions"
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if repo.RemoteUpstreamRef == "" {
			writeJSON(w, 422, map[string]string{"error": "federated_source_repository_required"})
			return
		}
		var in struct {
			Agent, Purpose, Instructions, TargetPullReference, Revision, Branch string
			Paths, Evidence                                                     []string
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		if in.Agent == "" {
			in.Agent = "codex"
		}
		if !availableTaskAgents[in.Agent] || in.Revision == "" || in.Branch == "" || len(in.Paths) > 50 || len(in.Evidence) > 50 {
			writeJSON(w, 422, map[string]string{"error": "invalid_delegation"})
			return
		}
		for _, p := range in.Paths {
			if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
				writeJSON(w, 422, map[string]string{"error": "invalid_context_path"})
				return
			}
		}
		opened, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		tip, branch, found := branchTip(opened, in.Branch)
		if !found || string(tip) != in.Revision {
			writeJSON(w, 409, map[string]string{"error": "source_revision_changed"})
			return
		}
		remote, e := fed.Remote(repo.RemoteUpstreamRef)
		if e != nil || remote.Status != "current" || remote.Stale {
			writeJSON(w, 409, map[string]string{"error": "current_remote_context_required"})
			return
		}
		peer, e := fed.Peer(remote.Instance)
		if e != nil || peer.Trust != "trusted" || peer.LastDocument == nil || !hasCapability(peer.LastDocument.Capabilities, "pull_request.exchange") {
			writeJSON(w, 409, map[string]string{"error": "peer_exchange_unavailable"})
			return
		}
		_, _, _, valid := parseFederatedPullReference(in.TargetPullReference)
		if !valid {
			writeJSON(w, 422, map[string]string{"error": "invalid_target_pull_reference"})
			return
		}
		issued, e := credentials.IssueRepositoryGit(actor.UserID, "Federated agent collaboration", string(repo.ID), "refs/heads/"+branch, 24*time.Hour)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		ctx := federatedagents.Context{TargetPullReference: in.TargetPullReference, SourcePullReference: federatedPullReference(mustFederationInstance(fed), string(repo.ID), "contribution"), RemoteRepositoryReference: repo.RemoteUpstreamRef, Revision: in.Revision, Branch: branch, Paths: in.Paths, Evidence: in.Evidence}
		item, e := store.Create(federatedagents.CreateParams{RepositoryID: string(repo.ID), InitiatorID: actor.UserID, Agent: in.Agent, Purpose: in.Purpose, Instructions: in.Instructions, CredentialGrantID: issued.ID, CredentialExpiresAt: issued.ExpiresAt, Context: ctx})
		if e != nil {
			_, _ = credentials.Revoke(actor.UserID, issued.ID)
			writeJSON(w, 422, map[string]string{"error": "invalid_delegation"})
			return
		}
		writeJSON(w, 201, map[string]any{"session": item, "credential": map[string]any{"token": issued.Token, "username": "agent", "repository_id": repo.ID, "branch": "refs/heads/" + branch, "expires_at": issued.ExpiresAt}, "authority": map[string]any{"remote_repository": false, "secrets": false, "checks": false, "merge": false}})
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := store.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	})
	mux.HandleFunc("POST "+base+"/{session}/events", func(w http.ResponseWriter, r *http.Request) {
		grant, ok := authenticateRequest(w, r, credentials, auth.GitWrite)
		if !ok {
			return
		}
		item, e := store.Get(r.PathValue("repository"), r.PathValue("session"))
		if e != nil || grant.ID != item.CredentialGrantID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			Type     string
			Metadata map[string]string
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		item, e = store.Event(item.RepositoryID, item.ID, in.Type, in.Metadata)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_event"})
			return
		}
		writeJSON(w, 201, item)
	})
	mux.HandleFunc("DELETE "+base+"/{session}/credential", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		item, e := store.Get(string(repo.ID), r.PathValue("session"))
		if e != nil || item.InitiatorID != actor.UserID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		_, _ = credentials.Revoke(actor.UserID, item.CredentialGrantID)
		_, e = store.Revoke(item.RepositoryID, item.ID, time.Now())
		if e != nil {
			writeJSON(w, 409, map[string]string{"error": "invalid_session_state"})
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST "+base+"/{session}/publication", func(w http.ResponseWriter, r *http.Request) {
		grant, ok := authenticateRequest(w, r, credentials, auth.GitWrite)
		if !ok {
			return
		}
		item, e := store.Get(r.PathValue("repository"), r.PathValue("session"))
		if e != nil || grant.ID != item.CredentialGrantID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			Summary                              string
			Commands, Evidence, ResidualConcerns []string
			Costs                                map[string]string
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		opened, e := repos.Open(storage.ID(item.RepositoryID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		tip, _, found := branchTip(opened, item.Context.Branch)
		if !found || string(tip) == item.Context.Revision {
			writeJSON(w, 409, map[string]string{"error": "source_branch_not_updated"})
			return
		}
		commits, e := commitsBetween(opened, tip, storage.ObjectID(item.Context.Revision))
		if e != nil {
			writeJSON(w, 409, map[string]string{"error": "source_history_diverged"})
			return
		}
		files, e := filesBetween(opened, tip, storage.ObjectID(item.Context.Revision))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		cids := make([]string, len(commits))
		for i := range commits {
			cids[i] = commits[i].ID
		}
		paths := make([]string, len(files))
		for i := range files {
			paths[i] = files[i].Path
		}
		pub := federatedagents.Publication{Summary: in.Summary, Commands: in.Commands, Evidence: in.Evidence, Costs: in.Costs, ResidualConcerns: in.ResidualConcerns, CommitIDs: cids, ChangedFiles: paths, SourceCommitID: string(tip)}
		item, e = store.Publish(item.RepositoryID, item.ID, pub)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_publication"})
			return
		}
		payload, _ := json.Marshal(pub)
		event := federation.PullRequestEvent{SchemaVersion: 1, IdempotencyKey: "agent-session:" + item.ID + ":" + string(tip), PullReference: item.Context.SourcePullReference, TargetReference: item.Context.TargetPullReference, SourceInstance: mustFederationInstance(fed), ActorSubject: "agent:" + item.Agent + "@" + mustFederationInstance(fed), Kind: "agent_session", Revision: string(tip), Body: string(payload), State: "published", Audience: "participants", Evidence: map[string]string{"session_id": item.ID, "authorized_by": "user:" + item.InitiatorID + "@" + mustFederationInstance(fed)}, OccurredAt: pub.PublishedAt}
		event.KeyID, event.Signature, e = fed.Sign(federation.PullRequestEventBytes(event))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		targetInstance, _, _, _ := parseFederatedPullReference(item.Context.TargetPullReference)
		peer, _ := fed.Peer(targetInstance)
		body, _ := json.Marshal(event)
		response, e := http.Post(peer.LastDocument.Endpoints.PullRequestEvents, "application/vnd.komodo.federated-pull-event+json; version=1", bytes.NewReader(body))
		if e != nil {
			writeJSON(w, 202, map[string]any{"session": item, "exchange": "peer_unavailable", "recovery": "retry publication event with the same idempotency key"})
			return
		}
		defer response.Body.Close()
		remoteBody, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		_, _ = credentials.Revoke(item.InitiatorID, item.CredentialGrantID)
		_, _ = store.Revoke(item.RepositoryID, item.ID, time.Now())
		writeJSON(w, 201, map[string]any{"session": item, "exchange_status": response.StatusCode, "remote_response": json.RawMessage(remoteBody)})
	})
}
