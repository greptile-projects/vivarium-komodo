package main

import (
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/docscollections"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type docscollectionsStore interface {
	Create(string, string, docscollections.Input) (docscollections.Collection, error)
	Update(string, string, string, docscollections.Input) (docscollections.Collection, error)
	Get(string, string) (docscollections.Collection, error)
	List(string) ([]docscollections.Collection, error)
}
type docscollectionsReleaseStore interface {
	Get(string, string) (releases.Release, error)
}
type documentationPage struct {
	Path           string `json:"path"`
	SourceRevision string `json:"source_revision"`
	BlobID         string `json:"blob_id"`
	Content        string `json:"content"`
	Author         string `json:"author"`
	AuthoredAt     string `json:"authored_at,omitempty"`
	CommitID       string `json:"commit_id"`
}
type documentationFinding struct {
	Code    string `json:"code"`
	Detail  string `json:"detail"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
}
type documentationView struct {
	docscollections.Collection
	Pages    []documentationPage    `json:"pages"`
	Findings []documentationFinding `json:"findings"`
	Healthy  bool                   `json:"healthy"`
}

func registerDocumentationHTTP(mux *http.ServeMux, s docscollectionsStore, repos contributorPathwayRepositories, credentials authStore, rs docscollectionsReleaseStore) {
	mux.HandleFunc("GET /repositories/{repository}/documentation-collections", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := s.List(string(repo.ID))
		if documentationError(w, e) {
			return
		}
		out := make([]documentationView, 0, len(items))
		for _, c := range items {
			out = append(out, resolveDocumentation(c, repo.OwnerID, repos, rs))
		}
		writeJSON(w, 200, map[string]any{"items": out})
	})
	mux.HandleFunc("POST /repositories/{repository}/documentation-collections", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if a.UserID != repo.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		var in docscollections.Input
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		c, e := s.Create(string(repo.ID), a.UserID, in)
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 201, resolveDocumentation(c, repo.OwnerID, repos, rs))
	})
	mux.HandleFunc("GET /repositories/{repository}/documentation-collections/{collection}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		c, e := s.Get(string(repo.ID), r.PathValue("collection"))
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 200, resolveDocumentation(c, repo.OwnerID, repos, rs))
	})
	mux.HandleFunc("POST /repositories/{repository}/documentation-collections/{collection}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if a.UserID != repo.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		var in docscollections.Input
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		c, e := s.Update(string(repo.ID), r.PathValue("collection"), a.UserID, in)
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 201, resolveDocumentation(c, repo.OwnerID, repos, rs))
	})
}
func documentationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, docscollections.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "documentation_collection_not_found"})
	case errors.Is(e, docscollections.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_documentation_collection"})
	case errors.Is(e, docscollections.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "documentation_collection_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
func resolveDocumentation(c docscollections.Collection, repositoryOwner string, repos contributorPathwayRepositories, rs docscollectionsReleaseStore) documentationView {
	v := documentationView{Collection: c, Pages: []documentationPage{}, Findings: []documentationFinding{}}
	if len(c.History) == 0 {
		return v
	}
	cur := c.History[len(c.History)-1]
	opened, _ := repos.Open(storage.ID(c.RepositoryID))
	for _, owner := range cur.OwnerIDs {
		participant := owner == repositoryOwner
		if !participant {
			participant, _ = repos.IsCollaborator(storage.ID(c.RepositoryID), owner)
		}
		if !participant {
			v.Findings = append(v.Findings, documentationFinding{Code: "missing_ownership", Detail: "The assigned owner is no longer a repository participant."})
		}
	}
	if len(cur.OwnerIDs) == 0 {
		v.Findings = append(v.Findings, documentationFinding{Code: "missing_ownership", Detail: "No maintainer owns this collection."})
	}
	for _, m := range cur.Versions {
		if m.ReleaseID != "" {
			rel, e := rs.Get(c.RepositoryID, m.ReleaseID)
			if e != nil || rel.CommitID != m.SourceRevision {
				v.Findings = append(v.Findings, documentationFinding{Code: "stale_version_mapping", Version: m.Label, Detail: "The release is missing or does not resolve to the reviewed source revision."})
			}
		}
	}
	if opened == nil {
		v.Findings = append(v.Findings, documentationFinding{Code: "broken_source", Detail: "Repository source is unavailable."})
		return v
	}
	revision := cur.Versions[0].SourceRevision
	commit, e := opened.ReadCommit(storage.ObjectID(revision))
	if e != nil {
		v.Findings = append(v.Findings, documentationFinding{Code: "broken_source", Detail: "Reviewed source revision is unavailable."})
		return v
	}
	cr := repositoryCommitResponse(commit)
	for _, entry := range cur.EntryPaths {
		full := path.Join(cur.RootPath, entry)
		blob, e := documentationBlobAtPath(opened, storage.ObjectID(revision), full)
		if e != nil {
			v.Findings = append(v.Findings, documentationFinding{Code: "broken_source", Path: full, Detail: "Configured documentation page is missing at the reviewed revision."})
			continue
		}
		obj, e := opened.ReadObject(blob)
		if e != nil || obj.Type != storage.BlobObject {
			v.Findings = append(v.Findings, documentationFinding{Code: "broken_source", Path: full, Detail: "Configured documentation page cannot be read."})
			continue
		}
		content := string(obj.Content)
		if !strings.HasSuffix(strings.ToLower(full), ".md") && cur.Policy.Renderer == "markdown" {
			v.Findings = append(v.Findings, documentationFinding{Code: "rendering_mismatch", Path: full, Detail: "Markdown rendering is configured for a non-Markdown path."})
		}
		v.Pages = append(v.Pages, documentationPage{Path: full, SourceRevision: revision, BlobID: string(blob), Content: content, Author: cr.Author, AuthoredAt: cr.AuthoredAt, CommitID: cr.ID})
	}
	v.Healthy = len(v.Findings) == 0
	return v
}
func documentationBlobAtPath(r *storage.Repository, commit storage.ObjectID, p string) (storage.ObjectID, error) {
	parts := strings.Split(strings.Trim(p, "/"), "/")
	c, e := r.ReadCommit(commit)
	if e != nil {
		return "", e
	}
	t, e := r.ReadTree(c.Tree)
	for i, part := range parts {
		if e != nil {
			return "", e
		}
		found := false
		for _, x := range t.Entries {
			if x.Name != part {
				continue
			}
			found = true
			if i == len(parts)-1 {
				return x.ObjectID, nil
			}
			t, e = r.ReadTree(x.ObjectID)
			break
		}
		if !found {
			return "", storage.ErrNotTree
		}
	}
	return "", storage.ErrNotTree
}
