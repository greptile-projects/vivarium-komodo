package main

import (
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

func snapshotBytes(snapshot federatedSnapshot) []byte { data, _ := json.Marshal(snapshot); return data }

func registerFederatedRepositoriesHTTP(mux *http.ServeMux, fed *federation.Store, repos ownedRepositoryStore, releasesStore releaseStore, pathways contributorPathwayStore, issueStore *issues.Store, opportunities *contributionopportunities.Store, activity activityStore, credentials authStore) {
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

var _ = errors.Is
