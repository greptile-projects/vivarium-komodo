// Package projectdeliveries retains the first revision-exact, governed product slice of an incubated project.
package projectdeliveries

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

var ErrNotFound = errors.New("project delivery not found")
var ErrInvalid = errors.New("invalid project delivery")
var ErrForbidden = errors.New("project delivery forbidden")
var ErrConflict = errors.New("project delivery conflict")

type Step struct {
	ID                 string   `json:"id"`
	Order              int      `json:"order"`
	Kind               string   `json:"kind"`
	Title              string   `json:"title"`
	OwnerID            string   `json:"owner_id"`
	DependsOnIDs       []string `json:"depends_on_ids"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	State              string   `json:"state"`
}
type Member struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	SubjectID     string    `json:"subject_id"`
	Role          string    `json:"role"`
	ParticipantID string    `json:"participant_id,omitempty"`
	Scope         string    `json:"scope"`
	ExpiresAt     time.Time `json:"expires_at"`
	State         string    `json:"state"`
}
type Workspace struct {
	ID               string    `json:"id"`
	StepID           string    `json:"step_id"`
	RepositoryHandle string    `json:"repository_handle"`
	BaseRevision     string    `json:"base_revision"`
	DefinitionDigest string    `json:"definition_digest"`
	Commands         []string  `json:"commands"`
	OwnerID          string    `json:"owner_id"`
	State            string    `json:"state"`
	CreatedAt        time.Time `json:"created_at"`
}
type PullRequest struct {
	ID               string   `json:"id"`
	StepID           string   `json:"step_id"`
	WorkspaceID      string   `json:"workspace_id"`
	RepositoryHandle string   `json:"repository_handle"`
	Revision         string   `json:"revision"`
	Kind             string   `json:"kind"`
	URL              string   `json:"url"`
	AuthorID         string   `json:"author_id"`
	ConnectedPullIDs []string `json:"connected_pull_ids"`
	Checks           []Check  `json:"checks"`
	Reviews          []Review `json:"reviews"`
	State            string   `json:"state"`
}
type Check struct {
	Name      string    `json:"name"`
	Revision  string    `json:"revision"`
	Outcome   string    `json:"outcome"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Review struct {
	ReviewerID string    `json:"reviewer_id"`
	Revision   string    `json:"revision"`
	Decision   string    `json:"decision"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}
type Preview struct {
	ID             string            `json:"id"`
	Revision       string            `json:"revision"`
	PullIDs        []string          `json:"pull_ids"`
	URL            string            `json:"url"`
	Journey        string            `json:"journey"`
	InvitedUserIDs []string          `json:"invited_user_ids"`
	Evidence       []PreviewEvidence `json:"evidence"`
	State          string            `json:"state"`
	CreatedAt      time.Time         `json:"created_at"`
}
type PreviewEvidence struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Revision    string    `json:"revision"`
	Outcome     string    `json:"outcome"`
	Observation string    `json:"observation"`
	Artifact    string    `json:"artifact,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
type Activity struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	ActorID   string    `json:"actor_id"`
	FromID    string    `json:"from_id,omitempty"`
	ToID      string    `json:"to_id,omitempty"`
	StepID    string    `json:"step_id,omitempty"`
	Detail    string    `json:"detail"`
	Cost      float64   `json:"cost"`
	Revision  string    `json:"revision,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Input struct {
	IncubatorID      string   `json:"incubator_id"`
	BoundaryID       string   `json:"boundary_id"`
	BoundaryRevision int64    `json:"boundary_revision"`
	AlternativeID    string   `json:"alternative_id"`
	Journey          string   `json:"representative_journey"`
	SuccessCriteria  []string `json:"success_criteria"`
	CostLimit        float64  `json:"cost_limit"`
	Steps            []Step   `json:"steps"`
	Team             []Member `json:"team"`
}
type Delivery struct {
	ID string `json:"id"`
	Input
	Revision         int64         `json:"revision"`
	CreatedByID      string        `json:"created_by_id"`
	Workspaces       []Workspace   `json:"workspaces"`
	PullRequests     []PullRequest `json:"pull_requests"`
	Previews         []Preview     `json:"previews"`
	Activity         []Activity    `json:"activity"`
	TotalCost        float64       `json:"total_cost"`
	Blockers         []string      `json:"blockers"`
	AuthorityGranted bool          `json:"authority_granted"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func id(p string) string { var b [12]byte; _, _ = rand.Read(b[:]); return p + hex.EncodeToString(b[:]) }
func validList(x []string, required bool) bool {
	if required && len(x) == 0 {
		return false
	}
	for _, v := range x {
		if strings.TrimSpace(v) == "" {
			return false
		}
	}
	return len(x) <= 100
}
func valid(in Input) bool {
	if in.IncubatorID == "" || in.BoundaryID == "" || in.BoundaryRevision < 1 || in.AlternativeID == "" || strings.TrimSpace(in.Journey) == "" || !validList(in.SuccessCriteria, true) || in.CostLimit < 0 || len(in.Steps) == 0 || len(in.Team) == 0 {
		return false
	}
	seen := map[string]bool{}
	for i, s := range in.Steps {
		if s.ID == "" || s.Order != i+1 || s.Title == "" || s.OwnerID == "" || !map[string]bool{"code": true, "tests": true, "documentation": true, "infrastructure": true, "interface": true}[s.Kind] || !validList(s.AcceptanceCriteria, true) || seen[s.ID] {
			return false
		}
		for _, d := range s.DependsOnIDs {
			if !seen[d] {
				return false
			}
		}
		seen[s.ID] = true
	}
	now := time.Now()
	for _, m := range in.Team {
		if m.ID == "" || m.SubjectID == "" || m.Role == "" || m.Scope == "" || !map[string]bool{"human": true, "agent": true}[m.Kind] || !m.ExpiresAt.After(now) {
			return false
		}
	}
	return true
}
func derive(v *Delivery) {
	v.TotalCost = 0
	v.Blockers = nil
	for _, a := range v.Activity {
		v.TotalCost += a.Cost
	}
	if v.TotalCost > v.CostLimit {
		v.Blockers = append(v.Blockers, "cost limit exceeded")
	}
	kinds := map[string]bool{}
	for _, p := range v.PullRequests {
		kinds[p.Kind] = true
		if p.State != "reviewed" {
			v.Blockers = append(v.Blockers, "pull request "+p.ID+" lacks current passing checks and approval")
		}
	}
	for _, k := range []string{"code", "tests", "documentation", "infrastructure", "interface"} {
		if !kinds[k] {
			v.Blockers = append(v.Blockers, "missing "+k+" pull request")
		}
	}
	current := false
	for _, p := range v.Previews {
		if p.State == "current" && len(p.Evidence) > 0 {
			current = true
		}
	}
	if !current {
		v.Blockers = append(v.Blockers, "current target-user preview evidence required")
	}
}
func (s *Store) Create(actor string, in Input) (Delivery, error) {
	if actor == "" || !valid(in) {
		return Delivery{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.now().UTC()
	for i := range in.Steps {
		in.Steps[i].State = "planned"
	}
	for i := range in.Team {
		in.Team[i].State = "active"
	}
	v := Delivery{ID: id("slice_"), Input: in, Revision: 1, CreatedByID: actor, Workspaces: []Workspace{}, PullRequests: []PullRequest{}, Previews: []Preview{}, Activity: []Activity{}, AuthorityGranted: false, CreatedAt: n, UpdatedAt: n}
	derive(&v)
	return v, s.write(v)
}
func (s *Store) Get(x string) (Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	derive(&v)
	return v, e
}
func (s *Store) Workspace(x, actor string, w Workspace) (Delivery, error) {
	return s.mutate(x, func(v *Delivery) error {
		if !member(v, actor) {
			return ErrForbidden
		}
		if w.StepID == "" || w.RepositoryHandle == "" || w.BaseRevision == "" || w.DefinitionDigest == "" || !validList(w.Commands, true) {
			return ErrInvalid
		}
		w.ID = id("ws_")
		w.OwnerID = actor
		w.State = "ready"
		w.CreatedAt = s.now().UTC()
		v.Workspaces = append(v.Workspaces, w)
		return nil
	})
}
func (s *Store) Pull(x, actor string, p PullRequest) (Delivery, error) {
	return s.mutate(x, func(v *Delivery) error {
		if !member(v, actor) {
			return ErrForbidden
		}
		if p.StepID == "" || p.WorkspaceID == "" || p.RepositoryHandle == "" || p.Revision == "" || p.URL == "" || !map[string]bool{"code": true, "tests": true, "documentation": true, "infrastructure": true, "interface": true}[p.Kind] {
			return ErrInvalid
		}
		p.ID = id("pull_")
		p.AuthorID = actor
		p.State = "open"
		p.Checks = []Check{}
		p.Reviews = []Review{}
		v.PullRequests = append(v.PullRequests, p)
		return nil
	})
}
func (s *Store) CheckReview(x, pid, actor, kind, revision, outcome, body string) (Delivery, error) {
	return s.mutate(x, func(v *Delivery) error {
		if !member(v, actor) {
			return ErrForbidden
		}
		for i := range v.PullRequests {
			p := &v.PullRequests[i]
			if p.ID == pid {
				if revision != p.Revision {
					return ErrConflict
				}
				if kind == "check" {
					if outcome != "passed" && outcome != "failed" {
						return ErrInvalid
					}
					p.Checks = append(p.Checks, Check{body, revision, outcome, actor, s.now().UTC()})
				} else if kind == "review" {
					if outcome != "approved" && outcome != "changes_requested" {
						return ErrInvalid
					}
					p.Reviews = append(p.Reviews, Review{actor, revision, outcome, body, s.now().UTC()})
				} else {
					return ErrInvalid
				}
				passed, approved := false, false
				for _, c := range p.Checks {
					passed = passed || (c.Revision == p.Revision && c.Outcome == "passed")
				}
				for _, r := range p.Reviews {
					approved = approved || (r.Revision == p.Revision && r.Decision == "approved" && r.ReviewerID != p.AuthorID)
				}
				if passed && approved {
					p.State = "reviewed"
				}
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) Preview(x, actor string, p Preview) (Delivery, error) {
	return s.mutate(x, func(v *Delivery) error {
		if !member(v, actor) || p.Revision == "" || p.URL == "" || p.Journey != v.Journey || len(p.PullIDs) == 0 || len(p.InvitedUserIDs) == 0 {
			return ErrInvalid
		}
		for _, old := range v.Previews {
			_ = old
		}
		for i := range v.Previews {
			v.Previews[i].State = "superseded"
		}
		p.ID = id("preview_")
		p.State = "current"
		p.Evidence = []PreviewEvidence{}
		p.CreatedAt = s.now().UTC()
		v.Previews = append(v.Previews, p)
		return nil
	})
}
func (s *Store) Evidence(x, pid, actor, outcome, observation, artifact, revision string) (Delivery, error) {
	return s.mutate(x, func(v *Delivery) error {
		for i := range v.Previews {
			p := &v.Previews[i]
			if p.ID == pid {
				inv := false
				for _, u := range p.InvitedUserIDs {
					inv = inv || u == actor
				}
				if !inv {
					return ErrForbidden
				}
				if revision != p.Revision {
					return ErrConflict
				}
				if outcome == "" || observation == "" {
					return ErrInvalid
				}
				p.Evidence = append(p.Evidence, PreviewEvidence{id("evi_"), actor, revision, outcome, observation, artifact, s.now().UTC()})
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) ActivityEvent(x, actor string, a Activity) (Delivery, error) {
	return s.mutate(x, func(v *Delivery) error {
		if !member(v, actor) || !map[string]bool{"agent_action": true, "handoff": true, "deviation": true}[a.Kind] || a.Detail == "" || a.Cost < 0 {
			return ErrInvalid
		}
		a.ID = id("act_")
		a.ActorID = actor
		a.CreatedAt = s.now().UTC()
		v.Activity = append(v.Activity, a)
		return nil
	})
}
func member(v *Delivery, a string) bool {
	if v.CreatedByID == a {
		return true
	}
	n := time.Now()
	for _, m := range v.Team {
		if m.SubjectID == a && m.State == "active" && m.ExpiresAt.After(n) {
			return true
		}
	}
	return false
}
func (s *Store) mutate(x string, fn func(*Delivery) error) (Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil {
		return v, e
	}
	if e = fn(&v); e != nil {
		return v, e
	}
	v.UpdatedAt = s.now().UTC()
	derive(&v)
	return v, s.write(v)
}
func (s *Store) path(x string) string { return filepath.Join(s.root, x+".json") }
func (s *Store) write(v Delivery) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	p := s.path(v.ID) + ".tmp"
	if e = os.WriteFile(p, b, 0640); e == nil {
		e = os.Rename(p, s.path(v.ID))
	}
	return e
}
func (s *Store) read(x string) (Delivery, error) {
	var v Delivery
	b, e := os.ReadFile(s.path(x))
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) List() ([]Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Delivery{}
	for _, f := range es {
		if filepath.Ext(f.Name()) == ".json" {
			v, x := s.read(strings.TrimSuffix(f.Name(), ".json"))
			if x != nil {
				return nil, x
			}
			derive(&v)
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
