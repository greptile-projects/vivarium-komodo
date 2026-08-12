package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributionopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributorpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/federation"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type federatedSnapshot struct {
	SchemaVersion             int                                     `json:"schema_version"`
	Instance                  string                                  `json:"instance"`
	Repository                map[string]any                          `json:"repository"`
	Revision                  string                                  `json:"revision"`
	PublishedAt               time.Time                               `json:"published_at"`
	Capabilities              []string                                `json:"capabilities"`
	UnsupportedCapabilities   []string                                `json:"unsupported_capabilities"`
	Branches                  []branchResponse                        `json:"branches"`
	Releases                  []releases.Release                      `json:"releases"`
	ContributorPathway        *contributorpathways.Pathway            `json:"contributor_pathway,omitempty"`
	Issues                    []issues.Issue                          `json:"issues"`
	ContributionOpportunities []contributionopportunities.Opportunity `json:"contribution_opportunities"`
	Activity                  any                                     `json:"activity"`
}

type signedFederatedSnapshot struct {
	Snapshot  federatedSnapshot `json:"snapshot"`
	KeyID     string            `json:"key_id"`
	Signature string            `json:"signature"`
}

type federatedObjectBundle struct {
	SchemaVersion int              `json:"schema_version"`
	Instance      string           `json:"instance"`
	RepositoryID  string           `json:"repository_id"`
	Branch        string           `json:"branch"`
	Tip           string           `json:"tip"`
	Objects       []storage.Object `json:"objects"`
	KeyID         string           `json:"key_id"`
	Signature     string           `json:"signature"`
}
type federatedContribution struct {
	SchemaVersion      int               `json:"schema_version"`
	SourceInstance     string            `json:"source_instance"`
	SourceRepositoryID string            `json:"source_repository_id"`
	SourceBranch       string            `json:"source_branch"`
	SourceCommitID     string            `json:"source_commit_id"`
	TargetReference    string            `json:"target_reference"`
	TargetBranch       string            `json:"target_branch"`
	TargetCommitID     string            `json:"target_commit_id"`
	AuthorSubject      string            `json:"author_subject"`
	Title              string            `json:"title"`
	Body               string            `json:"body"`
	Objects            []storage.Object  `json:"objects"`
	Context            map[string]string `json:"context,omitempty"`
	KeyID              string            `json:"key_id"`
	Signature          string            `json:"signature"`
}

func objectBundleBytes(v federatedObjectBundle) []byte {
	v.Signature = ""
	v.KeyID = ""
	b, _ := json.Marshal(v)
	return b
}
func contributionBytes(v federatedContribution) []byte {
	v.Signature = ""
	v.KeyID = ""
	b, _ := json.Marshal(v)
	return b
}
func fetchObjectBundle(doc *federation.Document, repositoryID, branch string) (federatedObjectBundle, error) {
	var bundle federatedObjectBundle
	if doc == nil || doc.Endpoints.RepositoryObjects == "" {
		return bundle, errors.New("object endpoint unavailable")
	}
	endpoint := strings.ReplaceAll(doc.Endpoints.RepositoryObjects, "{id}", url.PathEscape(repositoryID)) + "?branch=" + url.QueryEscape(branch)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Get(endpoint)
	if err != nil {
		return bundle, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return bundle, fmt.Errorf("object endpoint returned %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil || json.Unmarshal(body, &bundle) != nil {
		return bundle, errors.New("invalid object bundle")
	}
	if bundle.SchemaVersion != 1 || bundle.Instance != doc.Instance || bundle.RepositoryID != repositoryID || bundle.Branch != branch || len(bundle.Objects) > 10000 || federation.VerifySigned(*doc, bundle.KeyID, bundle.Signature, objectBundleBytes(bundle)) != nil {
		return bundle, errors.New("invalid object signature")
	}
	return bundle, nil
}

func snapshotBytes(snapshot federatedSnapshot) []byte { data, _ := json.Marshal(snapshot); return data }

func registerFederatedRepositoriesHTTP(mux *http.ServeMux, fed *federation.Store, repos ownedRepositoryStore, pulls pullRequestStore, releasesStore releaseStore, pathways contributorPathwayStore, issueStore *issues.Store, opportunities *contributionopportunities.Store, activity activityStore, credentials authStore) {
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/federated-merge-receipt/retry", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		pull, ok := readPullRequest(w, pulls, string(repo.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		if pull.Status != pullrequests.Merged || pull.FederatedContext == nil {
			writeJSON(w, 409, map[string]string{"error": "federated_merge_receipt_unavailable"})
			return
		}
		receipt, err := fed.MergeReceipt("merge:" + pull.RepositoryID + ":" + pull.ID)
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": "federated_merge_receipt_unavailable"})
			return
		}
		peer, err := fed.Peer(receipt.ContributorInstance)
		if err != nil || peer.LastDocument == nil || peer.LastDocument.Endpoints.ContributionReceipts == "" {
			writeJSON(w, 202, map[string]string{"status": "peer_unavailable", "recovery": "retry_same_receipt"})
			return
		}
		body, _ := json.Marshal(receipt)
		response, err := (&http.Client{Timeout: 10 * time.Second}).Post(peer.LastDocument.Endpoints.ContributionReceipts, "application/vnd.komodo.federated-merge-receipt+json; version=1", bytes.NewReader(body))
		if err != nil {
			writeJSON(w, 202, map[string]string{"status": "peer_unavailable", "recovery": "retry_same_receipt"})
			return
		}
		defer response.Body.Close()
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(payload)
	})
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/federated-events", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		pull, ok := readPullRequest(w, pulls, string(repo.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		ref := federatedPullReference(mustFederationInstance(fed), string(repo.ID), pull.ID)
		items, err := fed.PullRequestEvents(ref)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		for i := range items {
			items[i].Current = items[i].Revision == pull.SourceCommitID
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items), "authoritative_local_revision": pull.SourceCommitID})
	})
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/federated-events", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		pull, ok := readPullRequest(w, pulls, string(repo.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		var in struct {
			IdempotencyKey, TargetInstance, TargetReference, Kind, Revision, Body, State, Audience string
			Evidence                                                                               map[string]string
			OccurredAt                                                                             time.Time `json:"occurred_at"`
		}
		if !readJSON(w, r, &in, 96<<10) {
			return
		}
		if pull.Status != pullrequests.Open || in.Revision != pull.SourceCommitID || strings.HasPrefix(pull.SourceBranch, "embargo/") {
			writeJSON(w, 409, map[string]string{"error": "federated_event_not_shareable"})
			return
		}
		peer, err := fed.Peer(in.TargetInstance)
		if err != nil || peer.Trust != "trusted" || peer.LastDocument == nil || !hasCapability(peer.LastDocument.Capabilities, "pull_request.exchange") || peer.LastDocument.Endpoints.PullRequestEvents == "" {
			writeJSON(w, 409, map[string]string{"error": "peer_exchange_unavailable"})
			return
		}
		if in.OccurredAt.IsZero() {
			in.OccurredAt = time.Now().UTC()
		}
		e := federation.PullRequestEvent{SchemaVersion: 1, IdempotencyKey: in.IdempotencyKey, PullReference: federatedPullReference(mustFederationInstance(fed), string(repo.ID), pull.ID), TargetReference: in.TargetReference, SourceInstance: mustFederationInstance(fed), ActorSubject: "user:" + actor.UserID + "@" + mustFederationInstance(fed), Kind: in.Kind, Revision: in.Revision, Body: in.Body, State: in.State, Audience: in.Audience, Evidence: in.Evidence, OccurredAt: in.OccurredAt}
		e.KeyID, e.Signature, err = fed.Sign(federation.PullRequestEventBytes(e))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		body, _ := json.Marshal(e)
		response, err := (&http.Client{Timeout: 10 * time.Second}).Post(peer.LastDocument.Endpoints.PullRequestEvents, "application/vnd.komodo.federated-pull-event+json; version=1", bytes.NewReader(body))
		if err != nil {
			writeJSON(w, 202, map[string]string{"status": "peer_unavailable", "recovery": "retry_with_same_idempotency_key"})
			return
		}
		defer response.Body.Close()
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(payload)
	})
	mux.HandleFunc("POST /federation/pull-request-events", func(w http.ResponseWriter, r *http.Request) {
		var event federation.PullRequestEvent
		if !readJSON(w, r, &event, 96<<10) {
			return
		}
		peer, err := fed.Peer(event.SourceInstance)
		if err != nil || peer.Trust != "trusted" || peer.LastDocument == nil || !hasCapability(peer.LastDocument.Capabilities, "pull_request.exchange") || federation.VerifySigned(*peer.LastDocument, event.KeyID, event.Signature, federation.PullRequestEventBytes(event)) != nil {
			writeJSON(w, 422, map[string]string{"error": "untrusted_or_invalid_event"})
			return
		}
		instance, repositoryID, pullID, valid := parseFederatedPullReference(event.TargetReference)
		if !valid || instance != mustFederationInstance(fed) {
			writeJSON(w, 422, map[string]string{"error": "invalid_target"})
			return
		}
		repo, err := repos.Inspect(storage.ID(repositoryID))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		pull, err := pulls.Get(repositoryID, pullID)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if strings.HasPrefix(pull.SourceBranch, "embargo/") || (event.Audience == "public" && repo.Visibility != repositories.Public) {
			writeJSON(w, 403, map[string]string{"error": "data_sharing_policy_denied"})
			return
		}
		event.Verification = "verified_peer_signature"
		event.Current = event.Revision == pull.SourceCommitID
		retained, err := fed.PutPullRequestEvent(event)
		if errors.Is(err, federation.ErrConflict) {
			writeJSON(w, 409, map[string]string{"error": "idempotency_conflict", "recovery": "use_original_payload_or_new_key"})
			return
		}
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_event"})
			return
		}
		status := 201
		if retained.ImportedAt.Before(time.Now().UTC().Add(-time.Second)) {
			status = 200
		}
		writeJSON(w, status, retained)
	})
	mux.HandleFunc("GET /federation/repositories/{repository}/objects", func(w http.ResponseWriter, r *http.Request) {
		repo, err := repos.Inspect(storage.ID(r.PathValue("repository")))
		if err != nil || repo.Visibility != repositories.Public {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		opened, err := repos.(interface {
			Open(storage.ID) (*storage.Repository, error)
		}).Open(repo.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		branch := strings.TrimSpace(r.URL.Query().Get("branch"))
		tip, _, ok := branchTip(opened, branch)
		if !ok {
			writeJSON(w, 422, map[string]string{"error": "invalid_branch"})
			return
		}
		objects, err := opened.ListObjects()
		if err != nil || len(objects) > 10000 {
			writeJSON(w, 422, map[string]string{"error": "object_transfer_unavailable"})
			return
		}
		bundle := federatedObjectBundle{SchemaVersion: 1, Instance: mustFederationInstance(fed), RepositoryID: string(repo.ID), Branch: branch, Tip: string(tip), Objects: objects}
		bundle.KeyID, bundle.Signature, err = fed.Sign(objectBundleBytes(bundle))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, bundle)
	})
	mux.HandleFunc("POST /federation/repositories/forks", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			Reference, Branch, Name, Description string
			Visibility                           repositories.Visibility
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		remote, err := fed.Remote(in.Reference)
		if err != nil || remote.Status != "current" || remote.Stale {
			writeJSON(w, 409, map[string]string{"error": "current_remote_repository_required"})
			return
		}
		peer, err := fed.Peer(remote.Instance)
		if err != nil || peer.Trust != "trusted" || peer.LastDocument == nil || !hasCapability(peer.LastDocument.Capabilities, "repository.contributions") {
			writeJSON(w, 409, map[string]string{"error": "remote_contribution_unsupported"})
			return
		}
		bundle, err := fetchObjectBundle(peer.LastDocument, remote.RepositoryID, in.Branch)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "remote_object_transfer_failed"})
			return
		}
		if in.Name == "" {
			in.Name = remote.RepositoryID + "-fork"
		}
		if in.Visibility == "" {
			in.Visibility = repositories.Private
		}
		item, err := repos.CreateRemoteFork(actor.UserID, repositories.Metadata{Name: in.Name, Description: in.Description, Visibility: in.Visibility}, in.Reference, bundle.Branch, bundle.Objects, storage.ObjectID(bundle.Tip))
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "remote_fork_failed"})
			return
		}
		writeJSON(w, 201, repositoryResponse(item))
	})
	mux.HandleFunc("POST /federation/repositories/{repository}/sync", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		repo, err := repos.Inspect(storage.ID(r.PathValue("repository")))
		if err != nil || repo.OwnerID != actor.UserID || repo.RemoteUpstreamRef == "" {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			Branch string `json:"branch"`
		}
		if !readJSON(w, r, &in, 1024) {
			return
		}
		remote, err := fed.Remote(repo.RemoteUpstreamRef)
		if err != nil || remote.Status != "current" {
			writeJSON(w, 409, map[string]string{"error": "remote_repository_unavailable"})
			return
		}
		peer, _ := fed.Peer(remote.Instance)
		bundle, err := fetchObjectBundle(peer.LastDocument, remote.RepositoryID, in.Branch)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "remote_object_transfer_failed"})
			return
		}
		result, err := repos.ImportRemoteBranch(actor.UserID, repo.ID, in.Branch, bundle.Objects, storage.ObjectID(bundle.Tip))
		if errors.Is(err, repositories.ErrForkConflict) {
			writeJSON(w, 409, map[string]string{"error": "fork_branch_diverged"})
			return
		}
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "remote_sync_failed"})
			return
		}
		writeJSON(w, 200, map[string]any{"branch": result.Branch, "before_commit_id": result.Before, "after_commit_id": result.After, "updated": result.Updated})
	})
	mux.HandleFunc("POST /federation/repositories/{repository}/proposals", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		repo, err := repos.Inspect(storage.ID(r.PathValue("repository")))
		if err != nil || repo.OwnerID != actor.UserID || repo.RemoteUpstreamRef == "" {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			SourceBranch, SourceCommitID, TargetBranch, TargetCommitID, Title, Body string
			Context                                                                 map[string]string
		}
		if !readJSON(w, r, &in, 80<<10) {
			return
		}
		opened, _ := repos.(interface {
			Open(storage.ID) (*storage.Repository, error)
		}).Open(repo.ID)
		tip, branch, valid := branchTip(opened, in.SourceBranch)
		if !valid || string(tip) != in.SourceCommitID {
			writeJSON(w, 409, map[string]string{"error": "source_revision_changed"})
			return
		}
		remote, err := fed.Remote(repo.RemoteUpstreamRef)
		if err != nil || remote.Status != "current" {
			writeJSON(w, 409, map[string]string{"error": "remote_repository_unavailable"})
			return
		}
		var snap federatedSnapshot
		if json.Unmarshal(remote.Snapshot, &snap) != nil {
			writeJSON(w, 409, map[string]string{"error": "invalid_remote_snapshot"})
			return
		}
		targetOK := false
		for _, b := range snap.Branches {
			targetOK = targetOK || b.Name == in.TargetBranch && b.CommitID == in.TargetCommitID
		}
		if !targetOK {
			writeJSON(w, 409, map[string]string{"error": "target_revision_changed"})
			return
		}
		objects, _ := opened.ListObjects()
		subject := "user:" + actor.UserID + "@" + mustFederationInstance(fed)
		c := federatedContribution{SchemaVersion: 1, SourceInstance: mustFederationInstance(fed), SourceRepositoryID: string(repo.ID), SourceBranch: branch, SourceCommitID: string(tip), TargetReference: repo.RemoteUpstreamRef, TargetBranch: in.TargetBranch, TargetCommitID: in.TargetCommitID, AuthorSubject: subject, Title: in.Title, Body: in.Body, Objects: objects, Context: in.Context}
		c.KeyID, c.Signature, err = fed.Sign(contributionBytes(c))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		peer, _ := fed.Peer(remote.Instance)
		endpoint := peer.LastDocument.Endpoints.Contributions
		body, _ := json.Marshal(c)
		response, err := http.Post(endpoint, "application/vnd.komodo.federated-contribution+json; version=1", bytes.NewReader(body))
		if err != nil {
			writeJSON(w, 202, map[string]string{"status": "transfer_failed", "error": "remote_unreachable"})
			return
		}
		defer response.Body.Close()
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(payload)
	})
	mux.HandleFunc("POST /federation/contributions", func(w http.ResponseWriter, r *http.Request) {
		var c federatedContribution
		if !readJSON(w, r, &c, 16<<20) {
			return
		}
		peer, err := fed.Peer(c.SourceInstance)
		if err != nil || peer.Trust != "trusted" || peer.LastDocument == nil || federation.VerifySigned(*peer.LastDocument, c.KeyID, c.Signature, contributionBytes(c)) != nil {
			writeJSON(w, 422, map[string]string{"error": "untrusted_or_invalid_contribution"})
			return
		}
		instance, targetID, valid := parseFederatedRepositoryReference(c.TargetReference)
		if !valid || instance != mustFederationInstance(fed) {
			writeJSON(w, 422, map[string]string{"error": "invalid_target"})
			return
		}
		target, err := repos.Inspect(storage.ID(targetID))
		if err != nil || target.Visibility != repositories.Public {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		targetOpened, _ := repos.(interface {
			Open(storage.ID) (*storage.Repository, error)
		}).Open(target.ID)
		targetTip, targetBranch, ok := branchTip(targetOpened, c.TargetBranch)
		if !ok || string(targetTip) != c.TargetCommitID {
			writeJSON(w, 409, map[string]string{"error": "target_revision_changed"})
			return
		}
		source, err := repos.CreateRemoteFork(target.OwnerID, repositories.Metadata{Name: "federated-" + strings.ReplaceAll(c.SourceRepositoryID, "_", "-"), Description: "Federated contribution snapshot", Visibility: repositories.Private}, "repository:"+c.SourceRepositoryID+"@"+c.SourceInstance, c.SourceBranch, c.Objects, storage.ObjectID(c.SourceCommitID))
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "object_transfer_rejected"})
			return
		}
		item, err := pulls.Create(pullrequests.CreateParams{RepositoryID: string(target.ID), SourceRepositoryID: string(source.ID), AuthorID: c.AuthorSubject, Title: c.Title, Body: c.Body, SourceBranch: c.SourceBranch, TargetBranch: targetBranch, SourceCommitID: c.SourceCommitID, TargetCommitID: c.TargetCommitID, FederatedContext: &pullrequests.FederatedContext{SourceInstance: c.SourceInstance, SourceRepositoryID: c.SourceRepositoryID, SourceBranch: c.SourceBranch, SourceCommitID: c.SourceCommitID, SourcePullReference: c.Context["source_pull_reference"], TargetReference: c.TargetReference, AuthorSubject: c.AuthorSubject, ProposalKeyID: c.KeyID, ProposalSignature: c.Signature}})
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "proposal_rejected"})
			return
		}
		writeJSON(w, 201, item)
	})
	mux.HandleFunc("POST /federation/contribution-receipts", func(w http.ResponseWriter, r *http.Request) {
		var receipt federation.MergeReceipt
		if !readJSON(w, r, &receipt, 128<<10) {
			return
		}
		peer, err := fed.Peer(receipt.UpstreamInstance)
		if err != nil || peer.Trust != "trusted" || peer.LastDocument == nil || federation.VerifySigned(*peer.LastDocument, receipt.KeyID, receipt.Signature, federation.MergeReceiptBytes(receipt)) != nil {
			writeJSON(w, 422, map[string]string{"error": "untrusted_or_invalid_receipt"})
			return
		}
		now := time.Now().UTC()
		receipt.Verification, receipt.VerifiedAt, receipt.CurrentTrust = "verified_peer_signature", &now, peer.Trust
		retained, err := fed.PutMergeReceipt(receipt)
		if errors.Is(err, federation.ErrConflict) {
			writeJSON(w, 409, map[string]string{"error": "idempotency_conflict"})
			return
		}
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_receipt"})
			return
		}
		writeJSON(w, 201, retained)
	})
	mux.HandleFunc("GET /federation/contribution-receipts", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticateRequest(w, r, credentials, auth.ProfileRead); !ok {
			return
		}
		items, err := fed.MergeReceipts()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		for i := range items {
			if peer, e := fed.Peer(items[i].UpstreamInstance); e == nil {
				items[i].CurrentTrust = peer.Trust
			} else {
				items[i].CurrentTrust = "unavailable"
			}
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	})
	mux.HandleFunc("GET /federation/repositories/{repository}", func(w http.ResponseWriter, r *http.Request) {
		repo, err := repos.Inspect(storage.ID(r.PathValue("repository")))
		if err != nil || repo.Visibility != repositories.Public {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		opened, err := repos.(interface {
			Open(storage.ID) (*storage.Repository, error)
		}).Open(repo.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		branches := []branchResponse{}
		revision := ""
		if refs, e := opened.ListReferences(); e == nil {
			def, _ := opened.DefaultBranch()
			for _, ref := range refs {
				if name, ok := strings.CutPrefix(string(ref.Name), "refs/heads/"); ok && !strings.HasPrefix(name, "embargo/") && ref.ObjectID != "" {
					branches = append(branches, branchResponse{Name: name, CommitID: string(ref.ObjectID), IsDefault: ref.Name == def})
					if ref.Name == def {
						revision = string(ref.ObjectID)
					}
				}
			}
		}
		sort.Slice(branches, func(i, j int) bool { return branches[i].Name < branches[j].Name })
		rels, _ := releasesStore.List(string(repo.ID))
		pathway, pathwayErr := pathways.Get(string(repo.ID))
		issueData, _ := issueStore.List(string(repo.ID))
		oppData, _ := opportunities.List(string(repo.ID))
		events, _ := activity.List(string(repo.ID))
		publicIssues := []issues.Issue{}
		for _, item := range issueData {
			if item.Visibility == "public" {
				item.Attachments = nil
				publicIssues = append(publicIssues, item)
			}
		}
		snapshot := federatedSnapshot{SchemaVersion: 1, Instance: mustFederationInstance(fed), Repository: repositoryResponse(repo), Revision: revision, PublishedAt: time.Now().UTC(), Capabilities: []string{"repository.metadata", "repository.branches", "repository.releases", "repository.contributor_pathway", "repository.issues", "repository.contribution_opportunities", "repository.activity"}, UnsupportedCapabilities: []string{"repository.contents", "repository.mutations"}, Branches: branches, Releases: rels, Issues: publicIssues, ContributionOpportunities: oppData.Opportunities, Activity: events}
		if pathwayErr == nil {
			snapshot.ContributorPathway = &pathway
		}
		key, signature, err := fed.Sign(snapshotBytes(snapshot))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=60")
		writeJSON(w, 200, signedFederatedSnapshot{snapshot, key, signature})
	})
	mux.HandleFunc("GET /federation/repositories", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticateRequest(w, r, credentials, auth.ProfileRead); !ok {
			return
		}
		items, _ := fed.Remotes()
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	})
	mux.HandleFunc("GET /federation/repositories/resolutions", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticateRequest(w, r, credentials, auth.ProfileRead); !ok {
			return
		}
		item, err := fed.Remote(r.URL.Query().Get("reference"))
		if err != nil {
			writeFederationError(w, err)
			return
		}
		writeJSON(w, 200, item)
	})
	mux.HandleFunc("POST /federation/repositories/resolutions", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticateRequest(w, r, credentials, auth.ProfileWrite); !ok {
			return
		}
		var in struct {
			Reference string `json:"reference"`
			Follow    bool   `json:"follow"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		instance, repositoryID, ok := parseFederatedRepositoryReference(in.Reference)
		if !ok {
			writeJSON(w, 422, map[string]string{"error": "invalid_federated_repository_reference"})
			return
		}
		peer, err := fed.Peer(instance)
		if err != nil || peer.Trust != "trusted" || peer.LastDocument == nil {
			writeJSON(w, 409, map[string]string{"error": "trusted_peer_required"})
			return
		}
		item := federation.RemoteRepository{Reference: in.Reference, Instance: instance, RepositoryID: repositoryID, Followed: in.Follow, LastCheckedAt: time.Now().UTC()}
		if old, e := fed.Remote(in.Reference); e == nil {
			item = old
			item.Followed = in.Follow
			item.LastCheckedAt = time.Now().UTC()
		}
		if !hasCapability(peer.LastDocument.Capabilities, "repository.discovery") || peer.LastDocument.Endpoints.Repositories == "" {
			item.Status = "unsupported"
			item.LastError = "peer does not advertise repository.discovery"
			fed.SaveRemote(item)
			writeJSON(w, 200, item)
			return
		}
		item.SourceURL = strings.ReplaceAll(peer.LastDocument.Endpoints.Repositories, "{id}", url.PathEscape(repositoryID))
		response, e := (&http.Client{Timeout: 5 * time.Second}).Get(item.SourceURL)
		if e != nil {
			item.Status = "unreachable"
			item.Stale = len(item.Snapshot) > 0
			item.LastError = e.Error()
			fed.SaveRemote(item)
			writeJSON(w, 202, item)
			return
		}
		defer response.Body.Close()
		if response.StatusCode == http.StatusNotFound {
			item.Status = "withdrawn"
			item.VisibilityChanged = len(item.Snapshot) > 0
			item.Stale = len(item.Snapshot) > 0
			item.LastError = "remote repository is no longer public"
			fed.SaveRemote(item)
			writeJSON(w, 200, item)
			return
		}
		if response.StatusCode != 200 {
			item.Status = "unreachable"
			item.Stale = len(item.Snapshot) > 0
			item.LastError = "peer returned " + response.Status
			fed.SaveRemote(item)
			writeJSON(w, 202, item)
			return
		}
		body, e := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_remote_response"})
			return
		}
		var signed signedFederatedSnapshot
		if json.Unmarshal(body, &signed) != nil || signed.Snapshot.Instance != instance || fmt.Sprint(signed.Snapshot.Repository["id"]) != repositoryID || federation.VerifySigned(*peer.LastDocument, signed.KeyID, signed.Signature, snapshotBytes(signed.Snapshot)) != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_remote_signature"})
			return
		}
		raw, _ := json.Marshal(signed.Snapshot)
		digest := sha256.Sum256(raw)
		item.Snapshot = raw
		item.Revision = "sha256:" + hex.EncodeToString(digest[:])
		item.Signature = signed.Signature
		item.KeyID = signed.KeyID
		item.Status = "current"
		item.Stale = false
		item.VisibilityChanged = false
		item.LastError = ""
		now := time.Now().UTC()
		item.FetchedAt = &now
		fed.SaveRemote(item)
		writeJSON(w, 200, item)
	})
}

func mustFederationInstance(store *federation.Store) string {
	doc, _ := store.Document()
	return doc.Instance
}
func hasCapability(items []string, want string) bool {
	for _, v := range items {
		if v == want {
			return true
		}
	}
	return false
}
func parseFederatedRepositoryReference(ref string) (string, string, bool) {
	ref = strings.TrimSpace(ref)
	at := strings.LastIndex(ref, "@")
	if !strings.HasPrefix(ref, "repository:") || at < 12 {
		return "", "", false
	}
	id := ref[len("repository:"):at]
	instance := ref[at+1:]
	parsed, err := url.Parse(instance)
	return instance, id, err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.Path == "" && id != ""
}

func federatedPullReference(instance, repositoryID, pullID string) string {
	return "pull-request:" + pullID + "@" + instance + "#repository=" + repositoryID
}

func parseFederatedPullReference(ref string) (string, string, string, bool) {
	const prefix = "pull-request:"
	if !strings.HasPrefix(ref, prefix) {
		return "", "", "", false
	}
	value := strings.TrimPrefix(ref, prefix)
	before, repositoryID, ok := strings.Cut(value, "#repository=")
	if !ok || repositoryID == "" {
		return "", "", "", false
	}
	pullID, instance, ok := strings.Cut(before, "@")
	if !ok || pullID == "" {
		return "", "", "", false
	}
	u, err := url.Parse(instance)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", "", "", false
	}
	return strings.TrimSuffix(instance, "/"), repositoryID, pullID, true
}

var _ = errors.Is
