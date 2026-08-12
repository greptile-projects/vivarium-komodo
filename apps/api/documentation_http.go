package main

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"path"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/docscollections"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

type docscollectionsStore interface {
	Create(string, string, docscollections.Input) (docscollections.Collection, error)
	Update(string, string, string, docscollections.Input) (docscollections.Collection, error)
	Get(string, string) (docscollections.Collection, error)
	List(string) ([]docscollections.Collection, error)
	CreateTask(string, string, string, string, string, string, docscollections.TaskOrigin, []string, string, string) (docscollections.Task, error)
	SetTaskWorkspace(string, string, string) (docscollections.Task, error)
	GetTask(string, string) (docscollections.Task, error)
	ListTasks(string, string) ([]docscollections.Task, error)
	AddTaskEvent(string, string, string, docscollections.TaskEvent) (docscollections.Task, error)
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

type documentationWorkspaceStore interface {
	Create(string, string, string, workspaces.SourceContext, workspaces.Access, workspaces.Definition, string) (workspaces.Workspace, error)
}
type documentationWorkspaceRunner interface {
	Definition(string, string) (workspaces.Definition, string, error)
	Start(workspaces.Workspace)
}

func registerDocumentationHTTP(mux *http.ServeMux, s docscollectionsStore, repos contributorPathwayRepositories, credentials authStore, rs docscollectionsReleaseStore, extras ...any) {
	var ws documentationWorkspaceStore
	var runner documentationWorkspaceRunner
	for _, extra := range extras {
		if x, ok := extra.(documentationWorkspaceStore); ok {
			ws = x
		}
		if x, ok := extra.(documentationWorkspaceRunner); ok {
			runner = x
		}
	}
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
	mux.HandleFunc("GET /repositories/{repository}/documentation-tasks", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := s.ListTasks(string(repo.ID), r.URL.Query().Get("collection"))
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /repositories/{repository}/documentation-collections/{collection}/tasks", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Title, Path, Revision, Mode, Branch string
			Origin                              docscollections.TaskOrigin
			Evidence                            []string
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		opened, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_documentation_revision"})
			return
		}
		if _, e = opened.ReadCommit(storage.ObjectID(in.Revision)); e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_documentation_revision"})
			return
		}
		if !allowedDocumentationOrigin(in.Origin.Kind) {
			writeJSON(w, 422, map[string]string{"error": "invalid_documentation_origin"})
			return
		}
		evidence := append([]string{fmt.Sprintf("%s:%s@%s", in.Origin.Kind, in.Origin.ResourceID, in.Revision)}, in.Evidence...)
		t, e := s.CreateTask(string(repo.ID), r.PathValue("collection"), a.UserID, in.Title, in.Path, in.Revision, in.Origin, evidence, in.Mode, in.Branch)
		if documentationError(w, e) {
			return
		}
		if in.Mode == "workspace" {
			if ws == nil || runner == nil {
				writeJSON(w, 500, map[string]string{"error": "workspace_unavailable"})
				return
			}
			def, digest, e := runner.Definition(string(repo.ID), in.Revision)
			if e != nil {
				writeJSON(w, 422, map[string]string{"error": "workspace_definition_unavailable"})
				return
			}
			made, e := ws.Create(string(repo.ID), in.Revision, a.UserID, workspaces.SourceContext{Type: "documentation_task", ID: t.ID, ParentID: t.CollectionID, Evidence: evidence, Guidance: []string{"Draft only within " + t.Path, "Cite revision-grounded sources and declare uncertainty."}}, workspaces.Access{RepositoryID: string(repo.ID), ActorID: a.UserID, Permission: "repository:write"}, def, digest)
			if e != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			runner.Start(made)
			t, _ = s.SetTaskWorkspace(string(repo.ID), t.ID, made.ID)
		}
		writeJSON(w, 201, t)
	})
	mux.HandleFunc("GET /repositories/{repository}/documentation-tasks/{task}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		t, e := s.GetTask(string(repo.ID), r.PathValue("task"))
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 200, t)
	})
	mux.HandleFunc("POST /repositories/{repository}/documentation-tasks/{task}/events", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in docscollections.TaskEvent
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		task, e := s.GetTask(string(repo.ID), r.PathValue("task"))
		if documentationError(w, e) {
			return
		}
		for i := range in.References {
			ref := &in.References[i]
			if ref.Revision == "" {
				ref.Revision = task.Revision
			}
			if ref.Revision != task.Revision || ref.StartLine < 1 || ref.EndLine < ref.StartLine || ref.EndLine-ref.StartLine > 40 {
				writeJSON(w, 422, map[string]string{"error": "invalid_code_reference"})
				return
			}
			blob, e := documentationBlobAtPathMust(repos, repo.ID, ref.Revision, ref.Path)
			if e != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_code_reference"})
				return
			}
			ref.BlobID = blob.id
			ref.Excerpt = excerptLines(blob.content, ref.StartLine, ref.EndLine)
			if ref.Excerpt == "" {
				writeJSON(w, 422, map[string]string{"error": "invalid_code_reference"})
				return
			}
		}
		if in.Type == "draft" {
			in.Rendered = renderDocumentationDraft(in.Draft)
		}
		t, e := s.AddTaskEvent(string(repo.ID), task.ID, a.UserID, in)
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 201, t)
	})
}
func allowedDocumentationOrigin(k string) bool {
	return map[string]bool{"proposal": true, "issue": true, "pull_request": true, "release": true, "code_investigation": true, "stewardship_opportunity": true}[k]
}

type documentationBlob struct{ id, content string }

func documentationBlobAtPathMust(repos contributorPathwayRepositories, repo storage.ID, revision, p string) (documentationBlob, error) {
	opened, e := repos.Open(repo)
	if e != nil {
		return documentationBlob{}, e
	}
	id, e := documentationBlobAtPath(opened, storage.ObjectID(revision), p)
	if e != nil {
		return documentationBlob{}, e
	}
	obj, e := opened.ReadObject(id)
	if e != nil || obj.Type != storage.BlobObject {
		return documentationBlob{}, storage.ErrNotTree
	}
	return documentationBlob{string(id), string(obj.Content)}, nil
}
func excerptLines(content string, start, end int) string {
	xs := strings.Split(content, "\n")
	if start > len(xs) {
		return ""
	}
	if end > len(xs) {
		end = len(xs)
	}
	return strings.Join(xs[start-1:end], "\n")
}
func renderDocumentationDraft(v string) string {
	parts := strings.Split(strings.ReplaceAll(v, "\r\n", "\n"), "\n\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, "<p>"+html.EscapeString(p)+"</p>")
		}
	}
	return strings.Join(out, "\n")
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
