// Package productroadmaps owns versioned, accountable product direction.
package productroadmaps

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

var ErrNotFound = errors.New("product roadmap not found")
var ErrInvalid = errors.New("invalid product roadmap")
var ErrConflict = errors.New("product roadmap version conflict")

type Outcome struct {
	ID                    string    `json:"id"`
	OpportunityID         string    `json:"opportunity_id"`
	OpportunityVersion    int64     `json:"opportunity_version"`
	Title                 string    `json:"title"`
	Decision              string    `json:"decision"`
	Status                string    `json:"status"`
	OwnerID               string    `json:"owner_id,omitempty"`
	OwnerAvailable        bool      `json:"owner_available"`
	TargetHorizon         time.Time `json:"target_horizon,omitempty"`
	SuccessMeasures       []string  `json:"success_measures"`
	DependsOn             []string  `json:"depends_on"`
	Goals                 []string  `json:"goals"`
	Risks                 []string  `json:"risks"`
	GovernanceDecisionIDs []string  `json:"governance_decision_ids"`
	CommitmentIDs         []string  `json:"commitment_ids"`
	CapacityUnits         int       `json:"capacity_units"`
	Sequence              int       `json:"sequence"`
	Rationale             string    `json:"rationale"`
}
type Input struct {
	Name          string    `json:"name"`
	Goals         []string  `json:"goals"`
	CapacityUnits int       `json:"capacity_units"`
	Outcomes      []Outcome `json:"outcomes"`
	ChangeReason  string    `json:"change_reason"`
}
type Version struct {
	Number        int64     `json:"number"`
	Name          string    `json:"name"`
	Goals         []string  `json:"goals"`
	CapacityUnits int       `json:"capacity_units"`
	Outcomes      []Outcome `json:"outcomes"`
	ChangeReason  string    `json:"change_reason"`
	AuthorID      string    `json:"author_id"`
	CreatedAt     time.Time `json:"created_at"`
	Blockers      []string  `json:"blockers"`
}
type Scenario struct {
	ID                string    `json:"id"`
	BaseVersion       int64     `json:"base_version"`
	Summary           string    `json:"summary"`
	Outcomes          []Outcome `json:"outcomes"`
	AuthorID          string    `json:"author_id"`
	AuthorKind        string    `json:"author_kind"`
	CreatedAt         time.Time `json:"created_at"`
	ResourceAuthority bool      `json:"resource_authority"`
}
type Comment struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`
	Body      string    `json:"body"`
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Roadmap struct {
	ID                   string     `json:"id"`
	RepositoryID         string     `json:"repository_id"`
	CurrentVersion       int64      `json:"current_version"`
	Versions             []Version  `json:"versions"`
	Scenarios            []Scenario `json:"scenarios"`
	Comments             []Comment  `json:"comments"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	OperationalAuthority bool       `json:"operational_authority"`
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
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 || strings.TrimSpace(in.ChangeReason) == "" || in.CapacityUnits < 0 || len(in.Goals) == 0 || len(in.Outcomes) == 0 || len(in.Outcomes) > 200 {
		return false
	}
	ids := map[string]bool{}
	seq := map[int]bool{}
	for _, o := range in.Outcomes {
		if o.ID == "" || ids[o.ID] || seq[o.Sequence] || o.Sequence < 1 || o.OpportunityID == "" || o.OpportunityVersion < 1 || strings.TrimSpace(o.Title) == "" || !map[string]bool{"accepted": true, "rejected": true, "deferred": true}[o.Decision] || strings.TrimSpace(o.Rationale) == "" {
			return false
		}
		ids[o.ID] = true
		seq[o.Sequence] = true
		if o.Decision == "accepted" && (o.OwnerID == "" || o.TargetHorizon.IsZero() || len(o.SuccessMeasures) == 0 || o.CapacityUnits < 1) {
			return false
		}
	}
	return true
}
func makeVersion(n int64, actor string, in Input, now time.Time) Version {
	in.Name = strings.TrimSpace(in.Name)
	in.ChangeReason = strings.TrimSpace(in.ChangeReason)
	return Version{Number: n, Name: in.Name, Goals: in.Goals, CapacityUnits: in.CapacityUnits, Outcomes: in.Outcomes, ChangeReason: in.ChangeReason, AuthorID: actor, CreatedAt: now, Blockers: blockers(in, now)}
}
func blockers(in Input, now time.Time) []string {
	out := []string{}
	accepted := map[string]bool{}
	used := 0
	commitments := map[string]string{}
	for _, o := range in.Outcomes {
		if o.Decision == "accepted" {
			accepted[o.ID] = true
			used += o.CapacityUnits
			if !o.OwnerAvailable {
				out = append(out, "unavailable_owner:"+o.ID)
			}
			if !o.TargetHorizon.After(now) {
				out = append(out, "slipped_target:"+o.ID)
			}
		}
		for _, c := range o.CommitmentIDs {
			if prior := commitments[c]; prior != "" && prior != o.ID {
				out = append(out, "conflicting_commitment:"+c)
			} else {
				commitments[c] = o.ID
			}
		}
	}
	if used > in.CapacityUnits {
		out = append(out, "capacity_exceeded")
	}
	for _, o := range in.Outcomes {
		if o.Decision == "accepted" {
			for _, d := range o.DependsOn {
				if !accepted[d] {
					out = append(out, "unavailable_dependency:"+o.ID+":"+d)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}
func (s *Store) Create(repo, actor string, in Input) (Roadmap, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Roadmap{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	v := Roadmap{ID: id("roadmap_"), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{makeVersion(1, actor, in, now)}, Scenarios: []Scenario{}, Comments: []Comment{}, CreatedAt: now, UpdatedAt: now, OperationalAuthority: false}
	return v, s.write(v)
}
func (s *Store) Replan(repo, rid, actor string, expected int64, in Input) (Roadmap, error) {
	if actor == "" || !valid(in) {
		return Roadmap{}, ErrInvalid
	}
	return s.mutate(repo, rid, func(v *Roadmap) error {
		if v.CurrentVersion != expected {
			return ErrConflict
		}
		now := s.now().UTC()
		v.CurrentVersion++
		v.Versions = append(v.Versions, makeVersion(v.CurrentVersion, actor, in, now))
		v.UpdatedAt = now
		return nil
	})
}
func (s *Store) Scenario(repo, rid, actor, kind string, base int64, summary string, outcomes []Outcome) (Roadmap, error) {
	summary = strings.TrimSpace(summary)
	if actor == "" || summary == "" || base < 1 || !map[string]bool{"human": true, "agent": true}[kind] {
		return Roadmap{}, ErrInvalid
	}
	return s.mutate(repo, rid, func(v *Roadmap) error {
		if base != v.CurrentVersion {
			return ErrConflict
		}
		now := s.now().UTC()
		v.Scenarios = append(v.Scenarios, Scenario{ID: id("scenario_"), BaseVersion: base, Summary: summary, Outcomes: outcomes, AuthorID: actor, AuthorKind: kind, CreatedAt: now, ResourceAuthority: false})
		v.UpdatedAt = now
		return nil
	})
}
func (s *Store) Comment(repo, rid, actor, body string, version int64) (Roadmap, error) {
	body = strings.TrimSpace(body)
	if actor == "" || body == "" || version < 1 {
		return Roadmap{}, ErrInvalid
	}
	return s.mutate(repo, rid, func(v *Roadmap) error {
		if version > v.CurrentVersion {
			return ErrInvalid
		}
		now := s.now().UTC()
		v.Comments = append(v.Comments, Comment{ID: id("comment_"), Version: version, Body: body, AuthorID: actor, CreatedAt: now})
		v.UpdatedAt = now
		return nil
	})
}
func (s *Store) Get(repo, id string) (Roadmap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Roadmap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Roadmap{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Roadmap{}
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
func (s *Store) mutate(repo, id string, fn func(*Roadmap) error) (Roadmap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return v, e
	}
	if e = fn(&v); e != nil {
		return Roadmap{}, e
	}
	return v, s.write(v)
}
func (s *Store) read(repo, id string) (Roadmap, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if e != nil {
		return Roadmap{}, ErrNotFound
	}
	var v Roadmap
	if json.Unmarshal(b, &v) != nil || v.ID != id || v.RepositoryID != repo {
		return Roadmap{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) write(v Roadmap) error {
	d := filepath.Join(s.root, v.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, ".roadmap-*")
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
func id(prefix string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}
