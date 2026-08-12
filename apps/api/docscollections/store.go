// Package documentation owns repository-backed documentation collection contracts.
package docscollections

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("documentation collection not found")
	ErrInvalid  = errors.New("invalid documentation collection")
	ErrConflict = errors.New("documentation collection changed")
)

type TaskOrigin struct {
	Kind           string `json:"kind"`
	ResourceID     string `json:"resource_id"`
	ParentID       string `json:"parent_id,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
}
type CodeReference struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Revision  string `json:"revision"`
	BlobID    string `json:"blob_id"`
	Excerpt   string `json:"excerpt"`
}
type TaskEvent struct {
	Sequence    int64           `json:"sequence"`
	Type        string          `json:"type"`
	ActorID     string          `json:"actor_id"`
	Body        string          `json:"body,omitempty"`
	Draft       string          `json:"draft,omitempty"`
	Rendered    string          `json:"rendered,omitempty"`
	References  []CodeReference `json:"references,omitempty"`
	Citations   []string        `json:"citations,omitempty"`
	Uncertainty string          `json:"uncertainty,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}
type Task struct {
	ID                string      `json:"id"`
	RepositoryID      string      `json:"repository_id"`
	CollectionID      string      `json:"collection_id"`
	CollectionVersion int64       `json:"collection_version"`
	Title             string      `json:"title"`
	Path              string      `json:"path"`
	Origin            TaskOrigin  `json:"origin"`
	Revision          string      `json:"revision"`
	Evidence          []string    `json:"evidence"`
	Mode              string      `json:"mode"`
	Branch            string      `json:"branch,omitempty"`
	WorkspaceID       string      `json:"workspace_id,omitempty"`
	CreatorID         string      `json:"creator_id"`
	Events            []TaskEvent `json:"events"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type VersionMapping struct {
	Label          string `json:"label"`
	SourceRevision string `json:"source_revision"`
	ReleaseID      string `json:"release_id,omitempty"`
}
type Link struct {
	Kind       string `json:"kind"`
	Label      string `json:"label"`
	ResourceID string `json:"resource_id,omitempty"`
	Path       string `json:"path,omitempty"`
	Symbol     string `json:"symbol,omitempty"`
}
type Policy struct {
	Navigation  string            `json:"navigation"`
	Renderer    string            `json:"renderer"`
	Publication string            `json:"publication"`
	Visibility  string            `json:"visibility,omitempty"`
	Redirects   map[string]string `json:"redirects,omitempty"`
}

// Publication is an immutable reader-facing edition. Aliases are resolved at
// read time; publishing a replacement archives this record instead of editing
// its pages or provenance.
type Publication struct {
	ID                string            `json:"id"`
	RepositoryID      string            `json:"repository_id"`
	CollectionID      string            `json:"collection_id"`
	CollectionVersion int64             `json:"collection_version"`
	PullRequestID     string            `json:"pull_request_id"`
	PreviewID         string            `json:"preview_id"`
	SourceRevision    string            `json:"source_revision"`
	MergeRevision     string            `json:"merge_revision"`
	Pages             []ReviewPage      `json:"pages"`
	Versions          []VersionMapping  `json:"versions"`
	Audiences         []string          `json:"audiences"`
	Redirects         map[string]string `json:"redirects,omitempty"`
	PublishedByID     string            `json:"published_by_id"`
	PublishedAt       time.Time         `json:"published_at"`
}
type FeedbackEvidence struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Content string `json:"content"`
}
type FeedbackTriage struct {
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	ActorID    string    `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Feedback struct {
	ID              string             `json:"id"`
	RepositoryID    string             `json:"repository_id"`
	PublicationID   string             `json:"publication_id"`
	CollectionID    string             `json:"collection_id"`
	PagePath        string             `json:"page_path,omitempty"`
	Kind            string             `json:"kind"`
	Body            string             `json:"body"`
	Query           string             `json:"query,omitempty"`
	ExpectedVersion string             `json:"expected_version,omitempty"`
	Evidence        []FeedbackEvidence `json:"evidence,omitempty"`
	ReporterID      string             `json:"reporter_id"`
	CreatedAt       time.Time          `json:"created_at"`
	Triage          *FeedbackTriage    `json:"triage,omitempty"`
}
type Version struct {
	Number       int64            `json:"number"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	RootPath     string           `json:"root_path"`
	EntryPaths   []string         `json:"entry_paths"`
	Versions     []VersionMapping `json:"versions"`
	OwnerIDs     []string         `json:"owner_ids"`
	Audiences    []string         `json:"audiences"`
	Policy       Policy           `json:"policy"`
	Links        []Link           `json:"links"`
	AuthorID     string           `json:"author_id"`
	ChangeReason string           `json:"change_reason"`
	CreatedAt    time.Time        `json:"created_at"`
}
type Collection struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	History        []Version `json:"history"`
}
type Input struct {
	ExpectedVersion int64            `json:"expected_version"`
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	RootPath        string           `json:"root_path"`
	EntryPaths      []string         `json:"entry_paths"`
	Versions        []VersionMapping `json:"versions"`
	OwnerIDs        []string         `json:"owner_ids"`
	Audiences       []string         `json:"audiences"`
	Policy          Policy           `json:"policy"`
	Links           []Link           `json:"links"`
	ChangeReason    string           `json:"change_reason"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

// ReviewPreview is the durable, revision-exact documentation review contract
// attached to an ordinary pull request. Rendered pages are snapshots rather
// than mutable caches so comments and decisions always retain their subject.
type ReviewPage struct {
	Path     string `json:"path"`
	BlobID   string `json:"blob_id"`
	Rendered string `json:"rendered"`
}
type NavigationChange struct {
	Path   string `json:"path"`
	Change string `json:"change"`
}
type ReviewGap struct {
	Area   string `json:"area"`
	Detail string `json:"detail"`
}
type VerifiedExample struct {
	Name       string `json:"name"`
	CheckRunID string `json:"check_run_id"`
	Status     string `json:"status"`
}
type ReviewInvitation struct {
	UserID      string    `json:"user_id"`
	Role        string    `json:"role"`
	InvitedByID string    `json:"invited_by_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type ReviewComment struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actor_id"`
	Path      string    `json:"path"`
	BlobID    string    `json:"blob_id"`
	Start     int       `json:"start"`
	End       int       `json:"end"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
type AreaDecision struct {
	ActorID   string    `json:"actor_id"`
	Area      string    `json:"area"`
	Decision  string    `json:"decision"`
	Body      string    `json:"body,omitempty"`
	BlobIDs   []string  `json:"blob_ids"`
	CreatedAt time.Time `json:"created_at"`
}
type ReviewPreview struct {
	ID                string             `json:"id"`
	RepositoryID      string             `json:"repository_id"`
	PullRequestID     string             `json:"pull_request_id"`
	CollectionID      string             `json:"collection_id"`
	CollectionVersion int64              `json:"collection_version"`
	Revision          string             `json:"revision"`
	Pages             []ReviewPage       `json:"pages"`
	Navigation        []NavigationChange `json:"navigation_changes"`
	Examples          []VerifiedExample  `json:"verified_examples"`
	AffectedVersions  []string           `json:"affected_versions"`
	Gaps              []ReviewGap        `json:"gaps"`
	Invitations       []ReviewInvitation `json:"invitations"`
	Comments          []ReviewComment    `json:"comments"`
	Decisions         []AreaDecision     `json:"decisions"`
	CreatedByID       string             `json:"created_by_id"`
	CreatedAt         time.Time          `json:"created_at"`
}

func (s *Store) CreateReviewPreview(p ReviewPreview) (ReviewPreview, error) {
	if p.RepositoryID == "" || p.PullRequestID == "" || p.CollectionID == "" || len(p.Revision) != 40 || len(p.Pages) == 0 {
		return p, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p.ID = id()
	p.CreatedAt = s.now().UTC()
	return p, s.writeReviewPreview(p)
}
func (s *Store) GetReviewPreview(repo, pull, preview string) (ReviewPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readReviewPreview(repo, pull, preview)
}
func (s *Store) ListReviewPreviews(repo, pull string) ([]ReviewPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, repo, "reviews", pull)
	es, e := os.ReadDir(dir)
	if errors.Is(e, fs.ErrNotExist) {
		return []ReviewPreview{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []ReviewPreview{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		p, e := s.readReviewPreview(repo, pull, strings.TrimSuffix(x.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, nil
}
func (s *Store) InviteReview(repo, pull, preview, actor, user, role string) (ReviewPreview, error) {
	if user == "" || (role != "technical" && role != "audience") {
		return ReviewPreview{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.readReviewPreview(repo, pull, preview)
	if e != nil {
		return p, e
	}
	for _, v := range p.Invitations {
		if v.UserID == user {
			return p, ErrConflict
		}
	}
	p.Invitations = append(p.Invitations, ReviewInvitation{UserID: user, Role: role, InvitedByID: actor, CreatedAt: s.now().UTC()})
	return p, s.writeReviewPreview(p)
}
func (s *Store) AddReviewComment(repo, pull, preview, actor string, c ReviewComment) (ReviewPreview, error) {
	if actor == "" || strings.TrimSpace(c.Body) == "" || c.Start < 0 || c.End < c.Start || len(c.Body) > 10000 {
		return ReviewPreview{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.readReviewPreview(repo, pull, preview)
	if e != nil {
		return p, e
	}
	found := false
	for _, pg := range p.Pages {
		if pg.Path == c.Path && pg.BlobID == c.BlobID {
			found = true
		}
	}
	if !found {
		return p, ErrInvalid
	}
	c.ID = id()
	c.ActorID = actor
	c.CreatedAt = s.now().UTC()
	p.Comments = append(p.Comments, c)
	return p, s.writeReviewPreview(p)
}
func (s *Store) PutAreaDecision(repo, pull, preview, actor string, d AreaDecision) (ReviewPreview, error) {
	if actor == "" || (d.Area != "technical" && d.Area != "audience") || (d.Decision != "approve" && d.Decision != "request_changes") || (d.Decision == "request_changes" && strings.TrimSpace(d.Body) == "") {
		return ReviewPreview{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.readReviewPreview(repo, pull, preview)
	if e != nil {
		return p, e
	}
	d.ActorID = actor
	d.CreatedAt = s.now().UTC()
	d.BlobIDs = nil
	for _, pg := range p.Pages {
		d.BlobIDs = append(d.BlobIDs, pg.BlobID)
	}
	for i := range p.Decisions {
		if p.Decisions[i].ActorID == actor && p.Decisions[i].Area == d.Area {
			p.Decisions[i] = d
			return p, s.writeReviewPreview(p)
		}
	}
	p.Decisions = append(p.Decisions, d)
	return p, s.writeReviewPreview(p)
}
func (s *Store) readReviewPreview(repo, pull, pid string) (ReviewPreview, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, "reviews", pull, pid+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return ReviewPreview{}, ErrNotFound
	}
	var p ReviewPreview
	if e == nil {
		e = json.Unmarshal(b, &p)
	}
	return p, e
}
func (s *Store) writeReviewPreview(p ReviewPreview) error {
	d := filepath.Join(s.root, p.RepositoryID, "reviews", p.PullRequestID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, ".review-")
	if e != nil {
		return e
	}
	n := tmp.Name()
	defer os.Remove(n)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(d, p.ID+".json"))
	}
	return e
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func validPath(p string) bool {
	p = strings.Trim(p, "/")
	return p != "" && p != "." && !strings.Contains(p, "..") && !strings.Contains(p, "\\")
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Name) == "" || !validPath(in.RootPath) || len(in.EntryPaths) == 0 || len(in.EntryPaths) > 100 || len(in.Versions) == 0 || len(in.Versions) > 30 || len(in.Audiences) == 0 || strings.TrimSpace(in.ChangeReason) == "" {
		return false
	}
	if in.Policy.Navigation != "manual" && in.Policy.Navigation != "path" {
		return false
	}
	if in.Policy.Renderer != "markdown" && in.Policy.Renderer != "plain_text" {
		return false
	}
	if in.Policy.Publication != "maintainer_reviewed" && in.Policy.Publication != "owner_reviewed" {
		return false
	}
	if in.Policy.Visibility != "" && in.Policy.Visibility != "public" && in.Policy.Visibility != "repository" {
		return false
	}
	for from, to := range in.Policy.Redirects {
		if !validPath(from) || !validPath(to) {
			return false
		}
	}
	for _, p := range in.EntryPaths {
		if !validPath(p) {
			return false
		}
	}
	for _, v := range in.Versions {
		if strings.TrimSpace(v.Label) == "" || len(v.SourceRevision) != 40 {
			return false
		}
	}
	return true
}

func (s *Store) Publish(p Publication) (Publication, error) {
	if p.RepositoryID == "" || p.CollectionID == "" || p.PullRequestID == "" || p.PreviewID == "" || len(p.SourceRevision) != 40 || len(p.MergeRevision) != 40 || len(p.Pages) == 0 || p.PublishedByID == "" {
		return p, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.listPublications(p.RepositoryID, p.CollectionID)
	if err != nil {
		return p, err
	}
	for _, x := range items {
		if x.PullRequestID == p.PullRequestID && x.PreviewID == p.PreviewID {
			return x, nil
		}
	}
	p.ID = id()
	p.PublishedAt = s.now().UTC()
	d := filepath.Join(s.root, p.RepositoryID, "publications")
	if err = os.MkdirAll(d, 0750); err != nil {
		return p, err
	}
	err = writeJSONFile(filepath.Join(d, p.ID+".json"), p)
	return p, err
}
func (s *Store) ListPublications(repo, collection string) ([]Publication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listPublications(repo, collection)
}
func (s *Store) listPublications(repo, collection string) ([]Publication, error) {
	d := filepath.Join(s.root, repo, "publications")
	es, e := os.ReadDir(d)
	if errors.Is(e, fs.ErrNotExist) {
		return []Publication{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Publication{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		var p Publication
		b, e := os.ReadFile(filepath.Join(d, x.Name()))
		if e == nil {
			e = json.Unmarshal(b, &p)
		}
		if e != nil {
			return nil, e
		}
		if collection == "" || p.CollectionID == collection {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PublishedAt.Before(out[j].PublishedAt) })
	return out, nil
}
func (s *Store) GetPublication(repo, id string) (Publication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var p Publication
	b, e := os.ReadFile(filepath.Join(s.root, repo, "publications", id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return p, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &p)
	}
	return p, e
}
func (s *Store) CreateFeedback(f Feedback) (Feedback, error) {
	if f.RepositoryID == "" || f.PublicationID == "" || f.ReporterID == "" || strings.TrimSpace(f.Body) == "" || len(f.Body) > 10000 {
		return f, ErrInvalid
	}
	allowed := map[string]bool{"page_feedback": true, "failed_example": true, "search_miss": true, "version_mismatch": true}
	if !allowed[f.Kind] || f.Kind != "search_miss" && !validPath(f.PagePath) || f.Kind == "search_miss" && strings.TrimSpace(f.Query) == "" || len(f.Evidence) > 3 {
		return f, ErrInvalid
	}
	total := 0
	for i := range f.Evidence {
		e := &f.Evidence[i]
		if e.Kind != "log" && e.Kind != "screenshot" && e.Kind != "sample_input" || e.Name == "" {
			return f, ErrInvalid
		}
		total += len(e.Content)
		if len(e.Content) > 256<<10 {
			return f, ErrInvalid
		}
		e.Content = redactEvidence(e.Content)
	}
	if total > 512<<10 {
		return f, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f.ID = id()
	f.CreatedAt = s.now().UTC()
	d := filepath.Join(s.root, f.RepositoryID, "feedback")
	if e := os.MkdirAll(d, 0750); e != nil {
		return f, e
	}
	return f, writeJSONFile(filepath.Join(d, f.ID+".json"), f)
}
func (s *Store) ListFeedback(repo, collection string) ([]Feedback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := filepath.Join(s.root, repo, "feedback")
	es, e := os.ReadDir(d)
	if errors.Is(e, fs.ErrNotExist) {
		return []Feedback{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Feedback{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		var f Feedback
		b, e := os.ReadFile(filepath.Join(d, x.Name()))
		if e == nil {
			e = json.Unmarshal(b, &f)
		}
		if e != nil {
			return nil, e
		}
		if collection == "" || f.CollectionID == collection {
			out = append(out, f)
		}
	}
	return out, nil
}
func (s *Store) TriageFeedback(repo, fid, actor, kind, resource string) (Feedback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := filepath.Join(s.root, repo, "feedback", fid+".json")
	var f Feedback
	b, e := os.ReadFile(p)
	if errors.Is(e, fs.ErrNotExist) {
		return f, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &f)
	}
	if e != nil {
		return f, e
	}
	if f.Triage != nil || actor == "" || resource == "" || (kind != "issue" && kind != "proposal" && kind != "documentation_task") {
		return f, ErrInvalid
	}
	f.Triage = &FeedbackTriage{Kind: kind, ResourceID: resource, ActorID: actor, CreatedAt: s.now().UTC()}
	return f, writeJSONFile(p, f)
}
func writeJSONFile(name string, v any) error {
	d := filepath.Dir(name)
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, ".atomic-")
	if e != nil {
		return e
	}
	n := tmp.Name()
	defer os.Remove(n)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(n, name)
	}
	return e
}
func redactEvidence(v string) string {
	for _, marker := range []string{"password=", "token=", "authorization:"} {
		lower := strings.ToLower(v)
		i := strings.Index(lower, marker)
		if i < 0 {
			continue
		}
		start := i + len(marker)
		end := strings.IndexAny(v[start:], " \r\n&")
		if end < 0 {
			end = len(v) - start
		}
		v = v[:start] + "[REDACTED]" + v[start+end:]
	}
	return v
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) Create(repo, actor string, in Input) (Collection, error) {
	if repo == "" || actor == "" || in.ExpectedVersion != 0 || !valid(in) {
		return Collection{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := Collection{ID: id(), RepositoryID: repo}
	return s.add(c, actor, in)
}
func (s *Store) Update(repo, cid, actor string, in Input) (Collection, error) {
	if !valid(in) {
		return Collection{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.read(repo, cid)
	if e != nil {
		return c, e
	}
	if c.CurrentVersion != in.ExpectedVersion {
		return c, ErrConflict
	}
	return s.add(c, actor, in)
}
func (s *Store) add(c Collection, actor string, in Input) (Collection, error) {
	v := Version{Number: c.CurrentVersion + 1, Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description), RootPath: strings.Trim(in.RootPath, "/"), EntryPaths: in.EntryPaths, Versions: in.Versions, OwnerIDs: in.OwnerIDs, Audiences: in.Audiences, Policy: in.Policy, Links: in.Links, AuthorID: actor, ChangeReason: strings.TrimSpace(in.ChangeReason), CreatedAt: s.now().UTC()}
	c.CurrentVersion = v.Number
	c.History = append(c.History, v)
	return c, s.write(c)
}
func (s *Store) Get(repo, id string) (Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Collection{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Collection{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		c, e := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, nil
}
func (s *Store) CreateTask(repo, collection, actor, title, page, revision string, origin TaskOrigin, evidence []string, mode, branch string) (Task, error) {
	if repo == "" || actor == "" || strings.TrimSpace(title) == "" || len(revision) != 40 || (mode != "branch" && mode != "workspace") || !validPath(page) || origin.Kind == "" || origin.ResourceID == "" || mode == "branch" && strings.TrimSpace(branch) == "" {
		return Task{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.read(repo, collection)
	if err != nil {
		return Task{}, err
	}
	cur := c.History[len(c.History)-1]
	full := strings.Trim(strings.Trim(cur.RootPath, "/")+"/"+strings.Trim(page, "/"), "/")
	if !strings.HasPrefix(full+"/", strings.Trim(cur.RootPath, "/")+"/") {
		return Task{}, ErrInvalid
	}
	now := s.now().UTC()
	t := Task{ID: id(), RepositoryID: repo, CollectionID: collection, CollectionVersion: c.CurrentVersion, Title: strings.TrimSpace(title), Path: full, Origin: origin, Revision: revision, Evidence: evidence, Mode: mode, Branch: strings.TrimSpace(branch), CreatorID: actor, CreatedAt: now, UpdatedAt: now, Events: []TaskEvent{{Sequence: 1, Type: "opened", ActorID: actor, Citations: evidence, CreatedAt: now}}}
	return t, s.writeTask(t)
}
func (s *Store) SetTaskWorkspace(repo, task, workspace string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, e := s.readTask(repo, task)
	if e != nil {
		return t, e
	}
	t.WorkspaceID = workspace
	t.UpdatedAt = s.now().UTC()
	return t, s.writeTask(t)
}
func (s *Store) GetTask(repo, task string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readTask(repo, task)
}
func (s *Store) ListTasks(repo, collection string) ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, repo, "tasks")
	es, e := os.ReadDir(dir)
	if errors.Is(e, fs.ErrNotExist) {
		return []Task{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Task{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		t, e := s.readTask(repo, strings.TrimSuffix(x.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		if collection == "" || t.CollectionID == collection {
			out = append(out, t)
		}
	}
	return out, nil
}
func (s *Store) AddTaskEvent(repo, task, actor string, event TaskEvent) (Task, error) {
	if actor == "" || (event.Type != "discussion" && event.Type != "suggestion" && event.Type != "draft") || strings.TrimSpace(event.Body) == "" && strings.TrimSpace(event.Draft) == "" {
		return Task{}, ErrInvalid
	}
	if event.Type == "suggestion" && len(event.Citations) == 0 {
		return Task{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, e := s.readTask(repo, task)
	if e != nil {
		return t, e
	}
	event.Sequence = int64(len(t.Events) + 1)
	event.ActorID = actor
	event.CreatedAt = s.now().UTC()
	t.Events = append(t.Events, event)
	t.UpdatedAt = event.CreatedAt
	return t, s.writeTask(t)
}
func (s *Store) readTask(repo, task string) (Task, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, "tasks", task+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Task{}, ErrNotFound
	}
	var t Task
	if e == nil {
		e = json.Unmarshal(b, &t)
	}
	return t, e
}
func (s *Store) writeTask(t Task) error {
	d := filepath.Join(s.root, t.RepositoryID, "tasks")
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(t, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, ".task-")
	if e != nil {
		return e
	}
	n := tmp.Name()
	defer os.Remove(n)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(d, t.ID+".json"))
	}
	return e
}
func (s *Store) read(repo, id string) (Collection, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Collection{}, ErrNotFound
	}
	var c Collection
	if e == nil {
		e = json.Unmarshal(b, &c)
	}
	return c, e
}
func (s *Store) write(c Collection) error {
	d := filepath.Join(s.root, c.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, ".docs-")
	if e != nil {
		return e
	}
	n := tmp.Name()
	defer os.Remove(n)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(d, c.ID+".json"))
	}
	return e
}
