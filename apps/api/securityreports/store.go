// Package securityreports owns private vulnerability reports and their access ledger.
package securityreports

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
	ErrNotFound = errors.New("security report not found")
	ErrInvalid  = errors.New("invalid security report")
	ErrConflict = errors.New("security report conflict")
)

type AffectedRepository struct {
	RepositoryID string   `json:"repository_id"`
	Versions     []string `json:"versions"`
}
type Evidence struct {
	Title       string `json:"title"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}
type Contact struct {
	Channel string `json:"channel"`
	Value   string `json:"value"`
}
type TeamMember struct {
	UserID      string    `json:"user_id"`
	InvitedByID string    `json:"invited_by_id"`
	InvitedAt   time.Time `json:"invited_at"`
}
type Message struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
type AuditEvent struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	SubjectID string    `json:"subject_id,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Report struct {
	ID           string               `json:"id"`
	Title        string               `json:"title"`
	Summary      string               `json:"summary"`
	ReporterID   string               `json:"reporter_id"`
	Contact      Contact              `json:"contact"`
	Affected     []AffectedRepository `json:"affected_repositories"`
	Evidence     []Evidence           `json:"evidence"`
	Severity     string               `json:"severity"`
	EmbargoState string               `json:"embargo_state"`
	Team         []TeamMember         `json:"response_team"`
	Messages     []Message            `json:"messages"`
	Audit        []AuditEvent         `json:"audit_log"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
}
type CreateInput struct {
	ActorID, Title, Summary string
	Contact                 Contact
	Affected                []AffectedRepository
	Evidence                []Evidence
}
type TriageInput struct{ ActorID, Severity, EmbargoState string }

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(abs, 0750); err != nil {
		return nil, err
	}
	return &Store{root: abs, now: time.Now}, nil
}

func (s *Store) Create(in CreateInput) (Report, error) {
	in.Title, in.Summary = strings.TrimSpace(in.Title), strings.TrimSpace(in.Summary)
	in.Contact.Channel, in.Contact.Value = strings.TrimSpace(in.Contact.Channel), strings.TrimSpace(in.Contact.Value)
	if in.ActorID == "" || in.Title == "" || len(in.Title) > 200 || in.Summary == "" || len(in.Summary) > 20000 || in.Contact.Channel == "" || len(in.Contact.Channel) > 80 || in.Contact.Value == "" || len(in.Contact.Value) > 500 || len(in.Affected) == 0 || len(in.Affected) > 20 || len(in.Evidence) > 50 {
		return Report{}, ErrInvalid
	}
	seen := map[string]bool{}
	for x := range in.Affected {
		a := &in.Affected[x]
		a.RepositoryID = strings.TrimSpace(a.RepositoryID)
		if a.RepositoryID == "" || seen[a.RepositoryID] || len(a.Versions) == 0 || len(a.Versions) > 50 {
			return Report{}, ErrInvalid
		}
		seen[a.RepositoryID] = true
		for y := range a.Versions {
			a.Versions[y] = strings.TrimSpace(a.Versions[y])
			if a.Versions[y] == "" || len(a.Versions[y]) > 200 {
				return Report{}, ErrInvalid
			}
		}
	}
	for x := range in.Evidence {
		e := &in.Evidence[x]
		e.Title = strings.TrimSpace(e.Title)
		e.Kind = strings.ToLower(strings.TrimSpace(e.Kind))
		e.Description = strings.TrimSpace(e.Description)
		if e.Title == "" || len(e.Title) > 300 || !validEvidenceKind(e.Kind) || e.Description == "" || len(e.Description) > 20000 {
			return Report{}, ErrInvalid
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := newID()
	if err != nil {
		return Report{}, err
	}
	now := s.now().UTC()
	r := Report{ID: id, Title: in.Title, Summary: in.Summary, ReporterID: in.ActorID, Contact: in.Contact, Affected: in.Affected, Evidence: in.Evidence, Severity: "unknown", EmbargoState: "requested", Team: []TeamMember{}, Messages: []Message{}, Audit: []AuditEvent{}, CreatedAt: now, UpdatedAt: now}
	r.append("report.created", in.ActorID, "", "private report submitted", now)
	return r, s.write(r)
}
func (s *Store) ListVisible(actor string, maintainer func(string) bool) ([]Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	out := []Report{}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		r, er := s.read(strings.TrimSuffix(e.Name(), ".json"))
		if er != nil {
			return nil, er
		}
		if canAccess(r, actor, maintainer) {
			r.Summary = ""
			r.Contact = Contact{}
			r.Evidence = nil
			r.Messages = nil
			r.Audit = nil
			for x := range r.Affected {
				r.Affected[x].Versions = nil
			}
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (s *Store) Get(id, actor string, maintainer func(string) bool) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, actor, maintainer) {
		return Report{}, ErrNotFound
	}
	now := s.now().UTC()
	r.append("access.viewed", actor, "", "report opened", now)
	r.UpdatedAt = now
	if err = s.write(r); err != nil {
		return Report{}, err
	}
	return r, nil
}
func (s *Store) Triage(id string, in TriageInput, maintainer func(string) bool) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !isMaintainer(r, in.ActorID, maintainer) {
		return Report{}, ErrNotFound
	}
	sev := strings.ToLower(strings.TrimSpace(in.Severity))
	emb := strings.ToLower(strings.TrimSpace(in.EmbargoState))
	if sev == "" {
		sev = r.Severity
	}
	if emb == "" {
		emb = r.EmbargoState
	}
	if !oneOf(sev, "unknown", "low", "medium", "high", "critical") || !oneOf(emb, "requested", "active", "lifted") {
		return r, ErrInvalid
	}
	if sev == r.Severity && emb == r.EmbargoState {
		return r, ErrConflict
	}
	r.Severity, r.EmbargoState = sev, emb
	now := s.now().UTC()
	r.UpdatedAt = now
	r.append("triage.updated", in.ActorID, "", sev+" severity; "+emb+" embargo", now)
	return r, s.write(r)
}
func (s *Store) SetMember(id, actor, subject string, add bool, maintainer func(string) bool) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !isMaintainer(r, actor, maintainer) {
		return Report{}, ErrNotFound
	}
	if subject == "" || subject == r.ReporterID {
		return r, ErrInvalid
	}
	at := -1
	for i, m := range r.Team {
		if m.UserID == subject {
			at = i
		}
	}
	now := s.now().UTC()
	if add {
		if at >= 0 {
			return r, ErrConflict
		}
		if len(r.Team) >= 20 {
			return r, ErrInvalid
		}
		r.Team = append(r.Team, TeamMember{UserID: subject, InvitedByID: actor, InvitedAt: now})
		r.append("team.invited", actor, subject, "response access granted", now)
	} else {
		if at < 0 {
			return r, ErrConflict
		}
		r.Team = append(r.Team[:at], r.Team[at+1:]...)
		r.append("team.removed", actor, subject, "response access revoked", now)
	}
	r.UpdatedAt = now
	return r, s.write(r)
}
func (s *Store) AddMessage(id, actor, body string, maintainer func(string) bool) (Report, error) {
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 20000 {
		return Report{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, actor, maintainer) {
		return Report{}, ErrNotFound
	}
	mid, err := newID()
	if err != nil {
		return r, err
	}
	now := s.now().UTC()
	r.Messages = append(r.Messages, Message{ID: mid, AuthorID: actor, Body: body, CreatedAt: now})
	r.UpdatedAt = now
	r.append("message.created", actor, "", "private message added", now)
	return r, s.write(r)
}
func canAccess(r Report, actor string, maintainer func(string) bool) bool {
	if actor == r.ReporterID || isMaintainer(r, actor, maintainer) {
		return true
	}
	for _, m := range r.Team {
		if m.UserID == actor {
			return true
		}
	}
	return false
}
func isMaintainer(r Report, actor string, maintainer func(string) bool) bool {
	for _, a := range r.Affected {
		if maintainer(a.RepositoryID) {
			return true
		}
	}
	return false
}
func (r *Report) append(kind, actor, subject, detail string, at time.Time) {
	r.Audit = append(r.Audit, AuditEvent{Sequence: int64(len(r.Audit) + 1), Type: kind, ActorID: actor, SubjectID: subject, Detail: detail, CreatedAt: at})
}
func (s *Store) read(id string) (Report, error) {
	b, err := os.ReadFile(filepath.Join(s.root, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Report{}, ErrNotFound
	}
	if err != nil {
		return Report{}, err
	}
	var r Report
	if json.Unmarshal(b, &r) != nil {
		return Report{}, ErrInvalid
	}
	return r, nil
}
func (s *Store) write(r Report) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.root, r.ID+".json"), append(b, '\n'), 0640)
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func oneOf(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func validEvidenceKind(v string) bool {
	return oneOf(v, "description", "reproduction", "log", "artifact", "reference")
}
