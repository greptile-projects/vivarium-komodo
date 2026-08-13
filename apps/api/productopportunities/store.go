// Package productopportunities owns inspectable, versioned product-need syntheses.
package productopportunities

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

var ErrNotFound = errors.New("product opportunity not found")
var ErrInvalid = errors.New("invalid product opportunity")
var ErrConflict = errors.New("product opportunity version conflict")

type Source struct {
	Kind             string     `json:"kind"`
	ResourceID       string     `json:"resource_id"`
	SubresourceID    string     `json:"subresource_id,omitempty"`
	CapturedRevision string     `json:"captured_revision"`
	Relevance        string     `json:"relevance"`
	Position         string     `json:"position"`
	Detached         bool       `json:"detached"`
	DetachedByID     string     `json:"detached_by_id,omitempty"`
	DetachedAt       *time.Time `json:"detached_at,omitempty"`
}
type Input struct {
	Title             string   `json:"title"`
	Need              string   `json:"need"`
	AffectedAudiences []string `json:"affected_audiences"`
	Severity          string   `json:"severity"`
	Reach             string   `json:"reach"`
	Confidence        string   `json:"confidence"`
	ExpectedValue     string   `json:"expected_value"`
	Uncertainty       []string `json:"uncertainty"`
	Sources           []Source `json:"sources"`
	ChangeReason      string   `json:"change_reason"`
}
type Version struct {
	Number            int64     `json:"number"`
	Title             string    `json:"title"`
	Need              string    `json:"need"`
	AffectedAudiences []string  `json:"affected_audiences"`
	Severity          string    `json:"severity"`
	Reach             string    `json:"reach"`
	Confidence        string    `json:"confidence"`
	ExpectedValue     string    `json:"expected_value"`
	Uncertainty       []string  `json:"uncertainty"`
	Sources           []Source  `json:"sources"`
	ChangeReason      string    `json:"change_reason"`
	AuthorID          string    `json:"author_id"`
	CreatedAt         time.Time `json:"created_at"`
}
type Note struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	SourceKind string    `json:"source_kind,omitempty"`
	ResourceID string    `json:"resource_id,omitempty"`
	Body       string    `json:"body"`
	AuthorID   string    `json:"author_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Opportunity struct {
	ID                   string    `json:"id"`
	RepositoryID         string    `json:"repository_id"`
	CurrentVersion       int64     `json:"current_version"`
	Versions             []Version `json:"versions"`
	Notes                []Note    `json:"notes"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	OperationalAuthority bool      `json:"operational_authority"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, ErrInvalid
	}
	root, _ = filepath.Abs(root)
	if e := os.MkdirAll(root, 0750); e != nil {
		return nil, e
	}
	return &Store{root: root, now: time.Now}, nil
}
func valid(in Input) bool {
	text := func(s string, n int) bool { return strings.TrimSpace(s) != "" && len(s) <= n }
	if !text(in.Title, 200) || !text(in.Need, 65536) || !text(in.ExpectedValue, 65536) || !text(in.ChangeReason, 2000) || len(in.AffectedAudiences) == 0 || len(in.AffectedAudiences) > 30 || len(in.Sources) == 0 || len(in.Sources) > 100 || len(in.Uncertainty) > 30 {
		return false
	}
	if !map[string]bool{"low": true, "medium": true, "high": true, "critical": true}[in.Severity] || !map[string]bool{"narrow": true, "some": true, "broad": true, "unknown": true}[in.Reach] || !map[string]bool{"low": true, "medium": true, "high": true}[in.Confidence] {
		return false
	}
	for _, s := range in.Sources {
		if !map[string]bool{"feedback": true, "issue": true, "preview_finding": true, "support_signal": true, "usage_evidence": true, "experiment_outcome": true}[s.Kind] || !text(s.ResourceID, 300) || !text(s.CapturedRevision, 300) || !text(s.Relevance, 4000) || !map[string]bool{"supporting": true, "contradicting": true, "minority": true, "duplicate": true}[s.Position] {
			return false
		}
	}
	return true
}
func version(n int64, actor string, in Input, now time.Time) Version {
	return Version{Number: n, Title: strings.TrimSpace(in.Title), Need: strings.TrimSpace(in.Need), AffectedAudiences: in.AffectedAudiences, Severity: in.Severity, Reach: in.Reach, Confidence: in.Confidence, ExpectedValue: strings.TrimSpace(in.ExpectedValue), Uncertainty: in.Uncertainty, Sources: in.Sources, ChangeReason: strings.TrimSpace(in.ChangeReason), AuthorID: actor, CreatedAt: now}
}
func (s *Store) Create(repo, actor string, in Input) (Opportunity, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Opportunity{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	v := Opportunity{ID: id(), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{version(1, actor, in, now)}, Notes: []Note{}, CreatedAt: now, UpdatedAt: now, OperationalAuthority: false}
	return v, s.write(v)
}
func (s *Store) Revise(repo, oid, actor string, expected int64, in Input) (Opportunity, error) {
	if actor == "" || !valid(in) {
		return Opportunity{}, ErrInvalid
	}
	return s.mutate(repo, oid, func(v *Opportunity) error {
		if v.CurrentVersion != expected {
			return ErrConflict
		}
		now := s.now().UTC()
		v.CurrentVersion++
		v.Versions = append(v.Versions, version(v.CurrentVersion, actor, in, now))
		v.UpdatedAt = now
		return nil
	})
}
func (s *Store) Note(repo, oid, actor, kind, sk, rid, body string) (Opportunity, error) {
	body = strings.TrimSpace(body)
	if actor == "" || body == "" || len(body) > 65536 || !map[string]bool{"correction": true, "challenge": true}[kind] {
		return Opportunity{}, ErrInvalid
	}
	return s.mutate(repo, oid, func(v *Opportunity) error {
		now := s.now().UTC()
		v.Notes = append(v.Notes, Note{ID: id(), Kind: kind, SourceKind: sk, ResourceID: rid, Body: body, AuthorID: actor, CreatedAt: now})
		v.UpdatedAt = now
		return nil
	})
}
func (s *Store) DetachFeedback(repo, oid, feedback, actor string) (Opportunity, error) {
	return s.mutate(repo, oid, func(v *Opportunity) error {
		cur := v.Versions[len(v.Versions)-1]
		cur.Sources = append([]Source(nil), cur.Sources...)
		found := false
		now := s.now().UTC()
		for i := range cur.Sources {
			if cur.Sources[i].Kind == "feedback" && cur.Sources[i].ResourceID == feedback && !cur.Sources[i].Detached {
				cur.Sources[i].Detached = true
				cur.Sources[i].DetachedByID = actor
				cur.Sources[i].DetachedAt = &now
				found = true
			}
		}
		if !found {
			return ErrNotFound
		}
		v.CurrentVersion++
		cur.Number = v.CurrentVersion
		cur.AuthorID = actor
		cur.CreatedAt = now
		cur.ChangeReason = "feedback detached by its reporter"
		v.Versions = append(v.Versions, cur)
		v.UpdatedAt = now
		return nil
	})
}
func (s *Store) Get(repo, oid string) (Opportunity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, oid)
}
func (s *Store) List(repo string) ([]Opportunity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Opportunity{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Opportunity{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			v, er := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
			if er != nil {
				return nil, er
			}
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (s *Store) mutate(repo, oid string, fn func(*Opportunity) error) (Opportunity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, oid)
	if e != nil {
		return v, e
	}
	if e = fn(&v); e != nil {
		return Opportunity{}, e
	}
	return v, s.write(v)
}
func (s *Store) read(repo, oid string) (Opportunity, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, oid+".json"))
	if e != nil {
		return Opportunity{}, ErrNotFound
	}
	var v Opportunity
	if json.Unmarshal(b, &v) != nil || v.ID != oid || v.RepositoryID != repo {
		return Opportunity{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) write(v Opportunity) error {
	d := filepath.Join(s.root, v.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, ".opportunity-*")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Chmod(0600)
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(name, filepath.Join(d, v.ID+".json"))
	}
	return e
}
func id() string { b := make([]byte, 12); _, _ = rand.Read(b); return "opp_" + hex.EncodeToString(b) }
