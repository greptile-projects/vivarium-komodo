// Package previews owns durable, exact-revision pull request preview attempts.
package previews

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const ManifestPath = ".komodo/previews.json"

type Resources struct {
	CPUSeconds          int `json:"cpu_seconds"`
	MemoryMB            int `json:"memory_mb"`
	DiskMB              int `json:"disk_mb"`
	BuildTimeoutSeconds int `json:"build_timeout_seconds"`
	LifetimeMinutes     int `json:"lifetime_minutes"`
}
type Definition struct {
	Version       int            `json:"version"`
	Build         []string       `json:"build"`
	Start         string         `json:"start"`
	Port          int            `json:"port"`
	Configuration []string       `json:"configuration,omitempty"`
	Resources     Resources      `json:"resources"`
	Audience      AudiencePolicy `json:"audience"`
}
type AudiencePolicy struct {
	Network  string   `json:"network"`
	Data     string   `json:"data"`
	Identity string   `json:"identity"`
	Actions  []string `json:"actions"`
}
type Invitation struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Role        string     `json:"role"`
	SourceKind  string     `json:"source_kind"`
	SourceID    string     `json:"source_id,omitempty"`
	InvitedByID string     `json:"invited_by_id"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	RevokedByID string     `json:"revoked_by_id,omitempty"`
}
type AccessEvent struct {
	Sequence     int64     `json:"sequence"`
	Type         string    `json:"type"`
	ActorID      string    `json:"actor_id"`
	InvitationID string    `json:"invitation_id"`
	CreatedAt    time.Time `json:"created_at"`
}
type Evidence struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	Redacted  bool   `json:"redacted"`
	Audience  string `json:"audience"`
	Content   string `json:"-"`
}
type FindingComment struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
type FindingEvent struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	Value     string    `json:"value,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type FindingWork struct {
	Kind               string    `json:"kind"`
	ProposalID         string    `json:"proposal_id,omitempty"`
	TaskID             string    `json:"task_id,omitempty"`
	ChangeSessionID    string    `json:"change_session_id,omitempty"`
	WorkspaceID        string    `json:"workspace_id,omitempty"`
	OwnerKind          string    `json:"owner_kind"`
	OwnerID            string    `json:"owner_id"`
	AcceptanceCriteria []string  `json:"acceptance_criteria"`
	EvidenceIDs        []string  `json:"evidence_ids"`
	CreatedByID        string    `json:"created_by_id"`
	CreatedAt          time.Time `json:"created_at"`
}
type RepairPublication struct {
	Revision        string    `json:"revision"`
	CommitIDs       []string  `json:"commit_ids"`
	Commands        []string  `json:"commands,omitempty"`
	Checks          []string  `json:"checks,omitempty"`
	AuthorIDs       []string  `json:"author_ids"`
	ChangeSessionID string    `json:"change_session_id,omitempty"`
	WorkspaceID     string    `json:"workspace_id,omitempty"`
	PreviewID       string    `json:"preview_id"`
	PublishedByID   string    `json:"published_by_id"`
	PublishedAt     time.Time `json:"published_at"`
}
type Finding struct {
	ID                string              `json:"id"`
	AuthorID          string              `json:"author_id"`
	Route             string              `json:"route"`
	Revision          string              `json:"revision"`
	Title             string              `json:"title"`
	Description       string              `json:"description"`
	ReproductionSteps []string            `json:"reproduction_steps"`
	Classification    string              `json:"classification"`
	Status            string              `json:"status"`
	DuplicateOf       string              `json:"duplicate_of,omitempty"`
	RelatedFindingIDs []string            `json:"related_finding_ids,omitempty"`
	Evidence          []Evidence          `json:"evidence"`
	Comments          []FindingComment    `json:"comments"`
	History           []FindingEvent      `json:"history"`
	Work              *FindingWork        `json:"work,omitempty"`
	Repairs           []RepairPublication `json:"repairs,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

func (s *Store) LinkFindingWork(repo, pull, preview, finding, actor string, work FindingWork) (Finding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(preview)
	if err != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Finding{}, ErrNotFound
	}
	if work.Kind != "task" && work.Kind != "change_session" && work.Kind != "workspace" {
		return Finding{}, errors.New("invalid work")
	}
	for i := range p.Findings {
		f := &p.Findings[i]
		if f.ID != finding {
			continue
		}
		if f.Work != nil || len(work.AcceptanceCriteria) == 0 || work.OwnerID == "" {
			return Finding{}, errors.New("invalid work")
		}
		known := map[string]bool{}
		for _, e := range f.Evidence {
			known[e.ID] = true
		}
		for _, id := range work.EvidenceIDs {
			if !known[id] {
				return Finding{}, errors.New("invalid evidence")
			}
		}
		now := s.now().UTC()
		work.CreatedByID, work.CreatedAt = actor, now
		f.Work, f.UpdatedAt = &work, now
		f.History = append(f.History, FindingEvent{Sequence: int64(len(f.History) + 1), Type: "finding.work_linked", ActorID: actor, Value: work.Kind, CreatedAt: now})
		return *f, s.write(p)
	}
	return Finding{}, ErrNotFound
}

func (s *Store) RecordRepair(repo, pull, preview, finding, actor string, publication RepairPublication) (Finding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(preview)
	if err != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Finding{}, ErrNotFound
	}
	for i := range p.Findings {
		f := &p.Findings[i]
		if f.ID != finding {
			continue
		}
		if f.Work == nil || publication.Revision == "" || publication.PreviewID == "" || len(publication.CommitIDs) == 0 {
			return Finding{}, errors.New("invalid repair")
		}
		now := s.now().UTC()
		publication.PublishedByID, publication.PublishedAt = actor, now
		f.Repairs = append(f.Repairs, publication)
		f.Status, f.UpdatedAt = "resolved", now
		f.History = append(f.History, FindingEvent{Sequence: int64(len(f.History) + 1), Type: "finding.repair_published", ActorID: actor, Value: publication.PreviewID, CreatedAt: now})
		return *f, s.write(p)
	}
	return Finding{}, ErrNotFound
}

type Attestation struct {
	CommitID            string `json:"commit_id"`
	DefinitionDigest    string `json:"definition_digest"`
	ConfigurationDigest string `json:"configuration_digest"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	State     string    `json:"state,omitempty"`
	Stream    string    `json:"stream,omitempty"`
	Message   string    `json:"message,omitempty"`
	Command   string    `json:"command,omitempty"`
	ExitCode  *int      `json:"exit_code,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Preview struct {
	ID                 string            `json:"id"`
	RepositoryID       string            `json:"repository_id"`
	SourceRepositoryID string            `json:"source_repository_id"`
	PullRequestID      string            `json:"pull_request_id"`
	Revision           string            `json:"revision"`
	CreatorID          string            `json:"creator_id"`
	Definition         Definition        `json:"definition"`
	Attestation        Attestation       `json:"build_attestation"`
	Configuration      map[string]string `json:"-"`
	State              string            `json:"state"`
	URL                string            `json:"url,omitempty"`
	Stale              bool              `json:"stale"`
	Failure            string            `json:"failure,omitempty"`
	Events             []Event           `json:"events"`
	CreatedAt          time.Time         `json:"created_at"`
	ReadyAt            *time.Time        `json:"ready_at,omitempty"`
	StoppedAt          *time.Time        `json:"stopped_at,omitempty"`
	ExpiresAt          time.Time         `json:"expires_at"`
	LocalPort          int               `json:"local_port,omitempty"`
	Invitations        []Invitation      `json:"invitations"`
	AccessEvents       []AccessEvent     `json:"access_events"`
	Findings           []Finding         `json:"findings"`
}

var ErrNotFound = errors.New("preview not found")

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("preview root required")
	}
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	if e != nil {
		return nil, e
	}
	return &Store{root: p, now: time.Now}, nil
}
func (s *Store) Environment(id string) string { return filepath.Join(s.root, "environments", id) }
func (s *Store) Create(p Preview) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return p, e
	}
	now := s.now().UTC()
	p.ID = hex.EncodeToString(b)
	p.State = "setting_up"
	p.CreatedAt = now
	p.ExpiresAt = now.Add(time.Duration(p.Definition.Resources.LifetimeMinutes) * time.Minute)
	p.Events = []Event{{Sequence: 1, Type: "state", State: p.State, CreatedAt: now}}
	return p, s.write(p)
}
func (s *Store) Get(repo, pull, id string) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Preview{}, ErrNotFound
	}
	return p, nil
}
func (s *Store) GetByID(id string) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(id)
}
func (s *Store) List(repo, pull string) ([]Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Preview{}
	for _, v := range entries {
		if v.IsDir() || filepath.Ext(v.Name()) != ".json" {
			continue
		}
		p, er := s.read(strings.TrimSuffix(v.Name(), ".json"))
		if er == nil && p.RepositoryID == repo && p.PullRequestID == pull {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Append(id string, e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, er := s.read(id)
	if er != nil {
		return er
	}
	e.Sequence = int64(len(p.Events) + 1)
	e.CreatedAt = s.now().UTC()
	p.Events = append(p.Events, e)
	return s.write(p)
}
func (s *Store) Transition(id, state, url, failure string, port int) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil {
		return p, e
	}
	now := s.now().UTC()
	p.State = state
	p.URL = url
	p.Failure = failure
	p.LocalPort = port
	if state == "ready" {
		p.ReadyAt = &now
	}
	if state == "failed" || state == "stopped" || state == "expired" {
		p.StoppedAt = &now
	}
	p.Events = append(p.Events, Event{Sequence: int64(len(p.Events) + 1), Type: "state", State: state, Message: failure, CreatedAt: now})
	return p, s.write(p)
}
func (s *Store) Invite(repo, pull, id, actor string, in Invitation) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Preview{}, ErrNotFound
	}
	if in.UserID == "" || (in.Role != "view" && in.Role != "test" && in.Role != "feedback") || in.ExpiresAt.Before(s.now()) || in.ExpiresAt.After(p.ExpiresAt) {
		return Preview{}, errors.New("invalid invitation")
	}
	b := make([]byte, 12)
	if _, e = rand.Read(b); e != nil {
		return Preview{}, e
	}
	now := s.now().UTC()
	in.ID, in.InvitedByID, in.CreatedAt = hex.EncodeToString(b), actor, now
	p.Invitations = append(p.Invitations, in)
	p.AccessEvents = append(p.AccessEvents, AccessEvent{Sequence: int64(len(p.AccessEvents) + 1), Type: "invited", ActorID: actor, InvitationID: in.ID, CreatedAt: now})
	return p, s.write(p)
}
func (s *Store) Revoke(repo, pull, id, invitation, actor string) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Preview{}, ErrNotFound
	}
	now := s.now().UTC()
	for i := range p.Invitations {
		if p.Invitations[i].ID == invitation && p.Invitations[i].RevokedAt == nil {
			p.Invitations[i].RevokedAt = &now
			p.Invitations[i].RevokedByID = actor
			p.AccessEvents = append(p.AccessEvents, AccessEvent{Sequence: int64(len(p.AccessEvents) + 1), Type: "revoked", ActorID: actor, InvitationID: invitation, CreatedAt: now})
			return p, s.write(p)
		}
	}
	return Preview{}, ErrNotFound
}
func (s *Store) Authorize(repo, pull, id, user string) (Preview, Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Preview{}, Invitation{}, ErrNotFound
	}
	now := s.now().UTC()
	selected := Invitation{}
	priority := map[string]int{"view": 1, "feedback": 2, "test": 3}
	for _, in := range p.Invitations {
		if in.UserID == user && in.RevokedAt == nil && now.Before(in.ExpiresAt) && priority[in.Role] > priority[selected.Role] {
			selected = in
		}
	}
	if selected.ID != "" {
		p.AccessEvents = append(p.AccessEvents, AccessEvent{Sequence: int64(len(p.AccessEvents) + 1), Type: "entered", ActorID: user, InvitationID: selected.ID, CreatedAt: now})
		return p, selected, s.write(p)
	}
	return Preview{}, Invitation{}, ErrNotFound
}
func (s *Store) HasRole(repo, pull, id, user, role string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return false
	}
	now := s.now().UTC()
	for _, in := range p.Invitations {
		if in.UserID == user && in.Role == role && in.RevokedAt == nil && now.Before(in.ExpiresAt) {
			return true
		}
	}
	return false
}
func randomID(size int) (string, error) {
	b := make([]byte, size)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}

// AddFinding retains feedback on the exact attempt. Evidence bodies are stored
// separately so ordinary pull-request reads cannot implicitly broaden them.
func (s *Store) AddFinding(repo, pull, id, actor string, f Finding) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Preview{}, ErrNotFound
	}
	f.Title = strings.TrimSpace(f.Title)
	f.Description = strings.TrimSpace(f.Description)
	f.Route = redactRoute(strings.TrimSpace(f.Route))
	if f.Title == "" || len(f.Title) > 300 || f.Description == "" || len(f.Description) > 10000 || f.Route == "" || len(f.Route) > 2000 || !strings.HasPrefix(f.Route, "/") || len(f.ReproductionSteps) == 0 || len(f.ReproductionSteps) > 50 || len(f.Evidence) > 10 {
		return Preview{}, errors.New("invalid finding")
	}
	for _, step := range f.ReproductionSteps {
		if strings.TrimSpace(step) == "" || len(step) > 2000 {
			return Preview{}, errors.New("invalid finding")
		}
	}
	f.Title, _ = redactText(f.Title)
	f.Description, _ = redactText(f.Description)
	for i := range f.ReproductionSteps {
		f.ReproductionSteps[i], _ = redactText(f.ReproductionSteps[i])
	}
	total := 0
	evidenceDir := filepath.Join(s.root, "evidence", p.ID)
	if e = os.MkdirAll(evidenceDir, 0750); e != nil {
		return Preview{}, e
	}
	for i := range f.Evidence {
		a := &f.Evidence[i]
		data, er := base64.StdEncoding.DecodeString(a.Content)
		allowed := allowedEvidence(a.Kind, a.MediaType)
		if er != nil || !allowed || strings.TrimSpace(a.Name) == "" || len(a.Name) > 200 || len(data) > 1<<20 {
			return Preview{}, errors.New("invalid evidence")
		}
		total += len(data)
		if total > 5<<20 {
			return Preview{}, errors.New("invalid evidence")
		}
		if strings.HasPrefix(a.MediaType, "text/") || a.MediaType == "application/json" {
			data, a.Redacted = redactEvidence(data)
		}
		a.ID, er = randomID(12)
		if er != nil {
			return Preview{}, er
		}
		sum := sha256.Sum256(data)
		a.Size = int64(len(data))
		a.SHA256 = hex.EncodeToString(sum[:])
		a.Audience = "exact_preview"
		a.Content = ""
		if er = os.WriteFile(filepath.Join(evidenceDir, a.ID), data, 0640); er != nil {
			return Preview{}, er
		}
	}
	f.ID, e = randomID(12)
	if e != nil {
		return Preview{}, e
	}
	now := s.now().UTC()
	f.AuthorID = actor
	f.Revision = p.Revision
	f.Status = "open"
	f.Classification = "unclassified"
	f.CreatedAt = now
	f.UpdatedAt = now
	f.Comments = []FindingComment{}
	f.History = []FindingEvent{{Sequence: 1, Type: "finding.opened", ActorID: actor, CreatedAt: now}}
	for _, existing := range p.Findings {
		if existing.Status == "open" && strings.EqualFold(existing.Title, f.Title) && existing.Route == f.Route {
			f.DuplicateOf = existing.ID
			f.History = append(f.History, FindingEvent{Sequence: 2, Type: "finding.duplicate_linked", ActorID: actor, Value: existing.ID, CreatedAt: now})
			break
		}
	}
	p.Findings = append(p.Findings, f)
	return p, s.write(p)
}
func redactRoute(route string) string {
	u, e := url.Parse(route)
	if e != nil {
		return route
	}
	q := u.Query()
	for key := range q {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "auth") || strings.Contains(lower, "cookie") {
			q.Set(key, "[redacted]")
		}
	}
	u.RawQuery = q.Encode()
	u.Fragment = ""
	return u.String()
}
func allowedEvidence(kind, media string) bool {
	return map[string]map[string]bool{
		"screenshot": {"image/png": true, "image/jpeg": true, "image/webp": true}, "recording": {"video/webm": true, "video/mp4": true}, "console": {"text/plain": true}, "trace": {"application/json": true}, "annotation": {"application/json": true, "text/plain": true},
	}[kind][media]
}
func redactEvidence(data []byte) ([]byte, bool) {
	lines := bytes.Split(data, []byte("\n"))
	redacted := false
	markers := [][]byte{[]byte("authorization"), []byte("password"), []byte("secret"), []byte("token"), []byte("cookie"), []byte("private_key"), []byte("ghp_")}
	for i, line := range lines {
		lower := bytes.ToLower(line)
		for _, m := range markers {
			if bytes.Contains(lower, m) {
				lines[i] = []byte("[redacted sensitive field]")
				redacted = true
				break
			}
		}
	}
	return bytes.Join(lines, []byte("\n")), redacted
}
func redactText(value string) (string, bool) {
	b, redacted := redactEvidence([]byte(value))
	return string(b), redacted
}
func (s *Store) ReadEvidence(repo, pull, id, finding, evidence string) (Evidence, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Evidence{}, nil, ErrNotFound
	}
	for _, f := range p.Findings {
		if f.ID == finding {
			for _, a := range f.Evidence {
				if a.ID == evidence {
					b, er := os.ReadFile(filepath.Join(s.root, "evidence", p.ID, a.ID))
					return a, b, er
				}
			}
		}
	}
	return Evidence{}, nil, ErrNotFound
}
func (s *Store) CommentFinding(repo, pull, id, finding, actor, body string) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Preview{}, ErrNotFound
	}
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 10000 {
		return Preview{}, errors.New("invalid comment")
	}
	body, _ = redactText(body)
	for i := range p.Findings {
		if p.Findings[i].ID == finding {
			now := s.now().UTC()
			cid, er := randomID(12)
			if er != nil {
				return Preview{}, er
			}
			f := &p.Findings[i]
			f.Comments = append(f.Comments, FindingComment{ID: cid, AuthorID: actor, Body: body, CreatedAt: now})
			f.History = append(f.History, FindingEvent{Sequence: int64(len(f.History) + 1), Type: "finding.commented", ActorID: actor, CreatedAt: now})
			f.UpdatedAt = now
			return p, s.write(p)
		}
	}
	return Preview{}, ErrNotFound
}
func (s *Store) UpdateFinding(repo, pull, id, finding, actor, classification, status, duplicate string, related []string) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Preview{}, ErrNotFound
	}
	classes := map[string]bool{"": true, "bug": true, "usability": true, "accessibility": true, "content": true, "performance": true, "question": true}
	statuses := map[string]bool{"": true, "open": true, "resolved": true}
	if !classes[classification] || !statuses[status] || len(related) > 20 {
		return Preview{}, errors.New("invalid update")
	}
	exists := func(target string) bool {
		if target == "" {
			return true
		}
		for _, x := range p.Findings {
			if x.ID == target {
				return true
			}
		}
		return false
	}
	if !exists(duplicate) {
		return Preview{}, errors.New("invalid duplicate")
	}
	for _, x := range related {
		if !exists(x) {
			return Preview{}, errors.New("invalid relation")
		}
	}
	for i := range p.Findings {
		if p.Findings[i].ID == finding {
			f := &p.Findings[i]
			now := s.now().UTC()
			if classification != "" && classification != f.Classification {
				f.Classification = classification
				f.History = append(f.History, FindingEvent{Sequence: int64(len(f.History) + 1), Type: "finding.classified", ActorID: actor, Value: classification, CreatedAt: now})
			}
			if status != "" && status != f.Status {
				f.Status = status
				f.History = append(f.History, FindingEvent{Sequence: int64(len(f.History) + 1), Type: "finding." + status, ActorID: actor, CreatedAt: now})
			}
			if duplicate != "" && duplicate != finding {
				f.DuplicateOf = duplicate
				f.History = append(f.History, FindingEvent{Sequence: int64(len(f.History) + 1), Type: "finding.duplicate_linked", ActorID: actor, Value: duplicate, CreatedAt: now})
			}
			if related != nil {
				f.RelatedFindingIDs = related
				f.History = append(f.History, FindingEvent{Sequence: int64(len(f.History) + 1), Type: "finding.related", ActorID: actor, CreatedAt: now})
			}
			f.UpdatedAt = now
			return p, s.write(p)
		}
	}
	return Preview{}, ErrNotFound
}
func (s *Store) read(id string) (Preview, error) {
	b, e := os.ReadFile(filepath.Join(s.root, id+".json"))
	if os.IsNotExist(e) {
		return Preview{}, ErrNotFound
	}
	var p Preview
	if e != nil || json.Unmarshal(b, &p) != nil || p.ID != id {
		return Preview{}, ErrNotFound
	}
	return p, nil
}
func (s *Store) write(p Preview) error {
	b, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	tmp := filepath.Join(s.root, "."+p.ID+".tmp")
	if e = os.WriteFile(tmp, b, 0640); e == nil {
		e = os.Rename(tmp, filepath.Join(s.root, p.ID+".json"))
	}
	return e
}
