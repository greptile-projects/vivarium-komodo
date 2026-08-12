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
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
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
	CreateReviewPreview(docscollections.ReviewPreview) (docscollections.ReviewPreview, error)
	GetReviewPreview(string, string, string) (docscollections.ReviewPreview, error)
	ListReviewPreviews(string, string) ([]docscollections.ReviewPreview, error)
	InviteReview(string, string, string, string, string, string) (docscollections.ReviewPreview, error)
	AddReviewComment(string, string, string, string, docscollections.ReviewComment) (docscollections.ReviewPreview, error)
	PutAreaDecision(string, string, string, string, docscollections.AreaDecision) (docscollections.ReviewPreview, error)
	Publish(docscollections.Publication) (docscollections.Publication, error)
	ListPublications(string, string) ([]docscollections.Publication, error)
	GetPublication(string, string) (docscollections.Publication, error)
	CreateFeedback(docscollections.Feedback) (docscollections.Feedback, error)
	ListFeedback(string, string) ([]docscollections.Feedback, error)
	TriageFeedback(string, string, string, string, string) (docscollections.Feedback, error)
}
type documentationPullStore interface {
	Get(string, string) (pullrequests.PullRequest, error)
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
	var pulls documentationPullStore
	for _, extra := range extras {
		if x, ok := extra.(documentationWorkspaceStore); ok {
			ws = x
		}
		if x, ok := extra.(documentationWorkspaceRunner); ok {
			runner = x
		}
		if x, ok := extra.(documentationPullStore); ok {
			pulls = x
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
	registerDocumentationReviewHTTP(mux, s, repos, credentials, pulls)
	registerPublishedDocumentationHTTP(mux, s, repos, credentials, pulls)
}

type publishedDocumentationResponse struct {
	docscollections.Publication
	Archived  bool   `json:"archived"`
	StableURL string `json:"stable_url"`
}

func registerPublishedDocumentationHTTP(mux *http.ServeMux, s docscollectionsStore, repos contributorPathwayRepositories, credentials authStore, pulls documentationPullStore) {
	visible := func(p docscollections.Publication, c docscollections.Collection, authenticated bool) bool {
		v := c.History[p.CollectionVersion-1]
		return v.Policy.Visibility != "repository" || authenticated
	}
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull}/documentation-publications", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if a.UserID != repo.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		var in struct {
			PreviewID string `json:"preview_id"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		pr, e := pulls.Get(string(repo.ID), r.PathValue("pull"))
		if e != nil || pr.Status != pullrequests.Merged {
			writeJSON(w, 409, map[string]string{"error": "merged_pull_request_required"})
			return
		}
		preview, e := s.GetReviewPreview(string(repo.ID), pr.ID, in.PreviewID)
		if e != nil || preview.Revision != pr.SourceCommitID {
			writeJSON(w, 409, map[string]string{"error": "current_reviewed_preview_required"})
			return
		}
		c, e := s.Get(string(repo.ID), preview.CollectionID)
		if documentationError(w, e) {
			return
		}
		if preview.CollectionVersion < 1 || preview.CollectionVersion > int64(len(c.History)) {
			writeJSON(w, 422, map[string]string{"error": "invalid_documentation_collection"})
			return
		}
		v := c.History[preview.CollectionVersion-1]
		required := v.Policy.Publication == "owner_reviewed"
		approved := false
		for _, d := range preview.Decisions {
			if d.Decision == "request_changes" {
				writeJSON(w, 409, map[string]string{"error": "documentation_review_required"})
				return
			}
			if d.Decision == "approve" && (d.ActorID == repo.OwnerID || !required) {
				approved = true
			}
		}
		if !approved {
			writeJSON(w, 409, map[string]string{"error": "documentation_review_required"})
			return
		}
		p, e := s.Publish(docscollections.Publication{RepositoryID: string(repo.ID), CollectionID: c.ID, CollectionVersion: v.Number, PullRequestID: pr.ID, PreviewID: preview.ID, SourceRevision: preview.Revision, MergeRevision: pr.MergeCommitID, Pages: preview.Pages, Versions: v.Versions, Audiences: v.Audiences, Redirects: v.Policy.Redirects, PublishedByID: a.UserID})
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("GET /repositories/{repository}/documentation", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := s.ListPublications(string(repo.ID), r.URL.Query().Get("collection"))
		if documentationError(w, e) {
			return
		}
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		version := r.URL.Query().Get("version")
		release := r.URL.Query().Get("release")
		pathQuery := strings.Trim(r.URL.Query().Get("path"), "/")
		out := []publishedDocumentationResponse{}
		latest := map[string]string{}
		for _, p := range items {
			latest[p.CollectionID] = p.ID
		}
		for _, p := range items {
			c, e := s.Get(string(repo.ID), p.CollectionID)
			if e != nil || !visible(p, c, a.UserID != "") {
				continue
			}
			matches := version == "" && release == ""
			for _, v := range p.Versions {
				matches = matches || version == v.Label || release != "" && release == v.ReleaseID
			}
			if !matches {
				continue
			}
			pages := []docscollections.ReviewPage{}
			for _, pg := range p.Pages {
				requested := pathQuery
				if to, ok := p.Redirects[requested]; ok {
					requested = to
					w.Header().Set("X-Documentation-Redirect", to)
				}
				if requested != "" && strings.Trim(pg.Path, "/") != requested {
					continue
				}
				if q != "" && !strings.Contains(strings.ToLower(pg.Path+" "+pg.Rendered), q) {
					continue
				}
				pages = append(pages, pg)
			}
			if q != "" && len(pages) == 0 {
				continue
			}
			p.Pages = pages
			out = append(out, publishedDocumentationResponse{Publication: p, Archived: latest[p.CollectionID] != p.ID, StableURL: "/repositories/" + string(repo.ID) + "?view=documentation&edition=" + p.ID})
		}
		writeJSON(w, 200, map[string]any{"items": out, "query": q})
	})
	mux.HandleFunc("POST /repositories/{repository}/documentation/{publication}/feedback", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		p, e := s.GetPublication(string(repo.ID), r.PathValue("publication"))
		if documentationError(w, e) {
			return
		}
		var in docscollections.Feedback
		if !readJSON(w, r, &in, 768<<10) {
			return
		}
		in.RepositoryID = string(repo.ID)
		in.PublicationID = p.ID
		in.CollectionID = p.CollectionID
		in.ReporterID = a.UserID
		found := in.Kind == "search_miss"
		for _, pg := range p.Pages {
			found = found || pg.Path == in.PagePath
		}
		if !found {
			writeJSON(w, 422, map[string]string{"error": "invalid_documentation_page"})
			return
		}
		f, e := s.CreateFeedback(in)
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 201, f)
	})
	mux.HandleFunc("GET /repositories/{repository}/documentation-feedback", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		items, e := s.ListFeedback(string(repo.ID), r.URL.Query().Get("collection"))
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /repositories/{repository}/documentation-feedback/{feedback}/triage", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct{ Kind, ResourceID string }
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		f, e := s.TriageFeedback(string(repo.ID), r.PathValue("feedback"), a.UserID, in.Kind, in.ResourceID)
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 200, f)
	})
}

type documentationReviewResponse struct {
	docscollections.ReviewPreview
	Stale      bool     `json:"stale"`
	StalePaths []string `json:"stale_paths,omitempty"`
}

func registerDocumentationReviewHTTP(mux *http.ServeMux, s docscollectionsStore, repos contributorPathwayRepositories, credentials authStore, pulls documentationPullStore) {
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull}/documentation-previews", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		pr, e := pulls.Get(string(repo.ID), r.PathValue("pull"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "pull_request_not_found"})
			return
		}
		var in struct {
			CollectionID     string                             `json:"collection_id"`
			Navigation       []docscollections.NavigationChange `json:"navigation_changes"`
			Examples         []docscollections.VerifiedExample  `json:"verified_examples"`
			AffectedVersions []string                           `json:"affected_versions"`
			Gaps             []docscollections.ReviewGap        `json:"gaps"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		c, e := s.Get(string(repo.ID), in.CollectionID)
		if e != nil || len(c.History) == 0 {
			documentationError(w, e)
			return
		}
		cur := c.History[len(c.History)-1]
		opened, e := repos.Open(storage.ID(pr.SourceRepositoryID))
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "documentation_source_unavailable"})
			return
		}
		pages := []docscollections.ReviewPage{}
		for _, entry := range cur.EntryPaths {
			p := path.Join(cur.RootPath, entry)
			bid, e := documentationBlobAtPath(opened, storage.ObjectID(pr.SourceCommitID), p)
			if e != nil {
				continue
			}
			obj, e := opened.ReadObject(bid)
			if e != nil {
				continue
			}
			rendered := string(obj.Content)
			if cur.Policy.Renderer == "markdown" {
				rendered = renderDocumentationDraft(rendered)
			}
			pages = append(pages, docscollections.ReviewPage{Path: p, BlobID: string(bid), Rendered: rendered})
		}
		made, e := s.CreateReviewPreview(docscollections.ReviewPreview{RepositoryID: string(repo.ID), PullRequestID: pr.ID, CollectionID: c.ID, CollectionVersion: c.CurrentVersion, Revision: pr.SourceCommitID, Pages: pages, Navigation: in.Navigation, Examples: in.Examples, AffectedVersions: in.AffectedVersions, Gaps: in.Gaps, CreatedByID: a.UserID})
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 201, documentationReviewResponse{ReviewPreview: made})
	})
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull}/documentation-previews", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		pr, e := pulls.Get(string(repo.ID), r.PathValue("pull"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "pull_request_not_found"})
			return
		}
		items, e := s.ListReviewPreviews(string(repo.ID), pr.ID)
		if documentationError(w, e) {
			return
		}
		out := []documentationReviewResponse{}
		for _, p := range items {
			out = append(out, documentationReviewState(p, pr, repos))
		}
		writeJSON(w, 200, map[string]any{"items": out})
	})
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull}/documentation-previews/{preview}/invitations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			UserID string `json:"user_id"`
			Role   string `json:"role"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		p, e := s.InviteReview(string(repo.ID), r.PathValue("pull"), r.PathValue("preview"), a.UserID, in.UserID, in.Role)
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull}/documentation-previews/{preview}/comments", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in docscollections.ReviewComment
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		p, e := s.AddReviewComment(string(repo.ID), r.PathValue("pull"), r.PathValue("preview"), a.UserID, in)
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("PUT /repositories/{repository}/pull-requests/{pull}/documentation-previews/{preview}/decisions/{area}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in docscollections.AreaDecision
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		in.Area = r.PathValue("area")
		p, e := s.PutAreaDecision(string(repo.ID), r.PathValue("pull"), r.PathValue("preview"), a.UserID, in)
		if documentationError(w, e) {
			return
		}
		writeJSON(w, 200, p)
	})
}
func documentationReviewState(p docscollections.ReviewPreview, pr pullrequests.PullRequest, repos contributorPathwayRepositories) documentationReviewResponse {
	out := documentationReviewResponse{ReviewPreview: p}
	if p.Revision == pr.SourceCommitID {
		return out
	}
	opened, e := repos.Open(storage.ID(pr.SourceRepositoryID))
	if e != nil {
		out.Stale = true
		return out
	}
	for _, pg := range p.Pages {
		bid, e := documentationBlobAtPath(opened, storage.ObjectID(pr.SourceCommitID), pg.Path)
		if e != nil || string(bid) != pg.BlobID {
			out.StalePaths = append(out.StalePaths, pg.Path)
		}
	}
	out.Stale = len(out.StalePaths) > 0
	return out
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
