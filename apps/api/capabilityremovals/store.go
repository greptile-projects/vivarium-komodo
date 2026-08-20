// Package capabilityremovals retains governed staged removal and verified-cleanup evidence.
package capabilityremovals

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

	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityproofs"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityretirements"
)

var ErrNotFound = errors.New("capability removal not found")
var ErrInvalid = errors.New("invalid capability removal")
var ErrForbidden = errors.New("capability removal owner required")
var ErrConflict = errors.New("capability removal conflict")

type Proofs interface {
	Get(repository, candidate string) (capabilityproofs.Candidate, error)
}
type Plans interface {
	Get(repository, plan string) (capabilityretirements.Plan, error)
}

type Stage struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	PlanStageID      string   `json:"plan_stage_id"`
	RequiredEvidence []string `json:"required_evidence"`
	MaxRemainingUse  int64    `json:"max_remaining_use"`
	RollbackBoundary string   `json:"rollback_boundary"`
}
type Input struct {
	PlanID            string   `json:"plan_id"`
	ProofID           string   `json:"proof_id"`
	CandidateRevision string   `json:"candidate_revision"`
	OwnerIDs          []string `json:"owner_ids"`
	Stages            []Stage  `json:"stages"`
}
type DeliveryEvidence struct {
	ID         string    `json:"id"`
	StageID    string    `json:"stage_id"`
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	Revision   string    `json:"revision"`
	Status     string    `json:"status"`
	Reference  string    `json:"reference"`
	AuthorID   string    `json:"author_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type SignalInput struct {
	StageID           string `json:"stage_id"`
	RemainingUse      int64  `json:"remaining_use"`
	Health            string `json:"health"`
	Control           string `json:"control"`
	EvidenceReference string `json:"evidence_reference"`
	Environment       string `json:"environment"`
	Release           string `json:"release"`
	NextAction        string `json:"next_action"`
}
type Signal struct {
	ID string `json:"id"`
	SignalInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Consumer struct {
	ID                string    `json:"id"`
	ConsumerID        string    `json:"consumer_id"`
	EvidenceReference string    `json:"evidence_reference"`
	Summary           string    `json:"summary"`
	AuthorID          string    `json:"author_id"`
	CreatedAt         time.Time `json:"created_at"`
}
type Control struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	StageID   string    `json:"stage_id"`
	Rationale string    `json:"rationale"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}
type CleanupItem struct {
	Category          string `json:"category"`
	Subject           string `json:"subject"`
	Status            string `json:"status"`
	EvidenceReference string `json:"evidence_reference"`
	Revision          string `json:"revision"`
}
type CompletionInput struct {
	Cleanup            []CleanupItem `json:"cleanup"`
	OutcomeMeasures    []string      `json:"outcome_measures"`
	HistoricalEvidence []string      `json:"historical_evidence"`
	CompletedRevision  string        `json:"completed_revision"`
}
type Completion struct {
	CompletionInput
	OwnerID     string    `json:"owner_id"`
	CompletedAt time.Time `json:"completed_at"`
}
type Blocker struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Detail  string `json:"detail"`
}
type Removal struct {
	ID            string             `json:"id"`
	RepositoryID  string             `json:"repository_id"`
	Input         Input              `json:"input"`
	CreatedByID   string             `json:"created_by_id"`
	CreatedAt     time.Time          `json:"created_at"`
	Revision      int64              `json:"revision"`
	State         string             `json:"state"`
	ActiveStageID string             `json:"active_stage_id"`
	Compatibility string             `json:"compatibility"`
	Evidence      []DeliveryEvidence `json:"delivery_evidence"`
	Signals       []Signal           `json:"signals"`
	Consumers     []Consumer         `json:"unexpected_consumers"`
	Controls      []Control          `json:"controls"`
	Blockers      []Blocker          `json:"blockers"`
	NextAction    string             `json:"next_action"`
	Completion    *Completion        `json:"completion,omitempty"`
	NonAuthority  []string           `json:"non_authority"`
}
type Store struct {
	root   string
	proofs Proofs
	plans  Plans
	mu     sync.Mutex
	now    func() time.Time
}

func New(root string, proofs Proofs, plans Plans) (*Store, error) {
	if strings.TrimSpace(root) == "" || proofs == nil || plans == nil {
		return nil, ErrInvalid
	}
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, proofs: proofs, plans: plans, now: time.Now}, e
}
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func allowed(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func (s *Store) path(repo, rid string) string { return filepath.Join(s.root, repo, rid+".json") }
func validInput(in Input) bool {
	if in.PlanID == "" || in.ProofID == "" || in.CandidateRevision == "" || len(in.OwnerIDs) == 0 || len(in.Stages) == 0 {
		return false
	}
	seen := map[string]bool{}
	irreversible := false
	for _, x := range in.Stages {
		if x.ID == "" || x.Name == "" || x.PlanStageID == "" || seen[x.ID] || len(x.RequiredEvidence) == 0 || x.MaxRemainingUse < 0 || !allowed(x.RollbackBoundary, "reversible", "irreversible") {
			return false
		}
		seen[x.ID] = true
		if irreversible && x.RollbackBoundary != "irreversible" {
			return false
		}
		irreversible = irreversible || x.RollbackBoundary == "irreversible"
	}
	return irreversible
}
func (s *Store) Create(repo, actor string, in Input) (Removal, error) {
	if repo == "" || actor == "" || !validInput(in) || !contains(in.OwnerIDs, actor) {
		if !contains(in.OwnerIDs, actor) && actor != "" {
			return Removal{}, ErrForbidden
		}
		return Removal{}, ErrInvalid
	}
	p, e := s.plans.Get(repo, in.PlanID)
	if e != nil || !p.Ready {
		return Removal{}, ErrConflict
	}
	proof, e := s.proofs.Get(repo, in.ProofID)
	if e != nil || proof.Input.PlanID != in.PlanID || !proof.RemovalReady || in.CandidateRevision != proof.Input.Revisions.Provider || proof.Input.StageID != in.Stages[0].PlanStageID {
		return Removal{}, ErrConflict
	}
	if len(in.OwnerIDs) != len(proof.Input.RequiredOwnerIDs) {
		return Removal{}, ErrInvalid
	}
	for _, owner := range proof.Input.RequiredOwnerIDs {
		if !contains(in.OwnerIDs, owner) {
			return Removal{}, ErrInvalid
		}
	}
	for _, x := range in.Stages {
		found := false
		for _, ps := range p.Input.Stages {
			found = found || ps.ID == x.PlanStageID
		}
		if !found {
			return Removal{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	r := Removal{ID: id(), RepositoryID: repo, Input: in, CreatedByID: actor, CreatedAt: now, Revision: 1, State: "active", ActiveStageID: in.Stages[0].ID, Compatibility: "available", Evidence: []DeliveryEvidence{}, Signals: []Signal{}, Consumers: []Consumer{}, Controls: []Control{}, NonAuthority: []string{"Git write", "merge queue", "release", "schema migration", "infrastructure migration", "documentation publication", "deployment", "environment", "credential", "operational authority"}}
	r = s.derive(r)
	return r, s.save(r)
}
func stage(in Input, id string) (Stage, bool) {
	for _, x := range in.Stages {
		if x.ID == id {
			return x, true
		}
	}
	return Stage{}, false
}
func (s *Store) derive(r Removal) Removal {
	b := []Blocker{}
	add := func(k, sub, d string) { b = append(b, Blocker{k, sub, d}) }
	st, _ := stage(r.Input, r.ActiveStageID)
	latest := map[string]DeliveryEvidence{}
	for _, e := range r.Evidence {
		if e.StageID == r.ActiveStageID {
			latest[e.Kind] = e
		}
	}
	for _, k := range st.RequiredEvidence {
		e, ok := latest[k]
		if !ok {
			add("missing_delivery_evidence", k, "ordinary delivery evidence has not been linked")
		} else if e.Status != "passed" {
			add("failed_delivery_evidence", k, e.Status)
		}
	}
	var sig *Signal
	for i := len(r.Signals) - 1; i >= 0; i-- {
		if r.Signals[i].StageID == r.ActiveStageID {
			sig = &r.Signals[i]
			break
		}
	}
	if sig == nil {
		add("missing_stage_signal", st.ID, "remaining use, health, and control have not been observed")
	} else {
		if sig.RemainingUse > st.MaxRemainingUse {
			add("remaining_use", st.ID, "observed use exceeds this stage's limit")
		}
		if sig.Health != "healthy" {
			add("health_regression", st.ID, sig.Health)
		}
		if sig.Control != "passed" {
			add("failed_control", st.ID, sig.Control)
		}
		r.NextAction = sig.NextAction
	}
	if len(r.Consumers) > 0 {
		x := r.Consumers[len(r.Consumers)-1]
		add("unexpected_consumer", x.ConsumerID, x.Summary)
	}
	if p, e := s.plans.Get(r.RepositoryID, r.Input.PlanID); e != nil || !p.Ready {
		add("retirement_plan_stale", r.Input.PlanID, "the bound plan is no longer ready")
	}
	if p, e := s.proofs.Get(r.RepositoryID, r.Input.ProofID); e != nil || !p.RemovalReady {
		add("migration_proof_stale", r.Input.ProofID, "the bound proof is no longer removal-ready")
	}
	r.Blockers = b
	if r.Completion != nil {
		r.State = "completed"
		r.NextAction = "retain history and monitor the verified product outcome"
	} else if r.State != "restored" {
		if len(b) > 0 {
			r.State = "paused"
		} else if r.State == "paused" {
			ownerPaused := len(r.Controls) > 0 && r.Controls[len(r.Controls)-1].Action == "pause"
			if !ownerPaused {
				r.State = "active"
			}
		}
	}
	if r.NextAction == "" {
		if len(b) > 0 {
			r.NextAction = "resolve blockers or restore compatibility"
		} else {
			r.NextAction = "advance when the stage owner accepts current evidence"
		}
	}
	return r
}
func (s *Store) mutate(repo, rid string, expected int64, fn func(*Removal) error) (Removal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, e := s.read(repo, rid)
	if e != nil {
		return r, e
	}
	if r.Revision != expected {
		return Removal{}, ErrConflict
	}
	if e = fn(&r); e != nil {
		return Removal{}, e
	}
	r.Revision++
	r = s.derive(r)
	return r, s.save(r)
}
func (s *Store) AddEvidence(repo, rid, actor string, expected int64, in DeliveryEvidence) (Removal, error) {
	if actor == "" || in.StageID == "" || !allowed(in.Kind, "merge_queue", "release", "schema_migration", "infrastructure_migration", "documentation", "protected_environment") || in.ResourceID == "" || in.Revision == "" || !allowed(in.Status, "passed", "failed", "pending") || in.Reference == "" {
		return Removal{}, ErrInvalid
	}
	return s.mutate(repo, rid, expected, func(r *Removal) error {
		if _, ok := stage(r.Input, in.StageID); !ok {
			return ErrInvalid
		}
		in.ID = id()
		in.AuthorID = actor
		in.CreatedAt = s.now().UTC()
		r.Evidence = append(r.Evidence, in)
		return nil
	})
}
func (s *Store) AddSignal(repo, rid, actor string, expected int64, in SignalInput) (Removal, error) {
	if actor == "" || in.RemainingUse < 0 || !allowed(in.Health, "healthy", "degraded", "regressed") || !allowed(in.Control, "passed", "failed") || in.EvidenceReference == "" || in.Environment == "" || in.Release == "" || in.NextAction == "" {
		return Removal{}, ErrInvalid
	}
	return s.mutate(repo, rid, expected, func(r *Removal) error {
		if _, ok := stage(r.Input, in.StageID); !ok {
			return ErrInvalid
		}
		r.Signals = append(r.Signals, Signal{ID: id(), SignalInput: in, AuthorID: actor, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) DiscoverConsumer(repo, rid, actor string, expected int64, consumer, ref, summary string) (Removal, error) {
	if actor == "" || consumer == "" || ref == "" || summary == "" {
		return Removal{}, ErrInvalid
	}
	return s.mutate(repo, rid, expected, func(r *Removal) error {
		r.Consumers = append(r.Consumers, Consumer{id(), consumer, ref, summary, actor, s.now().UTC()})
		return nil
	})
}
func (s *Store) Control(repo, rid, actor string, expected int64, action, rationale string) (Removal, error) {
	if actor == "" || !allowed(action, "advance", "pause", "resume", "restore") || rationale == "" {
		return Removal{}, ErrInvalid
	}
	return s.mutate(repo, rid, expected, func(r *Removal) error {
		if !contains(r.Input.OwnerIDs, actor) {
			return ErrForbidden
		}
		if r.Completion != nil {
			return ErrConflict
		}
		current, _ := stage(r.Input, r.ActiveStageID)
		if action == "advance" {
			rr := s.derive(*r)
			if len(rr.Blockers) > 0 {
				return ErrConflict
			}
			i := 0
			for ; i < len(r.Input.Stages); i++ {
				if r.Input.Stages[i].ID == r.ActiveStageID {
					break
				}
			}
			if i+1 >= len(r.Input.Stages) {
				return ErrConflict
			}
			r.ActiveStageID = r.Input.Stages[i+1].ID
			r.State = "active"
			if current.RollbackBoundary == "irreversible" {
				r.Compatibility = "removed"
			} else {
				r.Compatibility = "limited"
			}
		} else if action == "restore" {
			if current.RollbackBoundary == "irreversible" || r.Compatibility == "removed" {
				return ErrConflict
			}
			r.State = "restored"
			r.Compatibility = "restored"
		} else {
			r.State = map[string]string{"pause": "paused", "resume": "active"}[action]
		}
		r.Controls = append(r.Controls, Control{id(), action, r.ActiveStageID, rationale, actor, s.now().UTC()})
		return nil
	})
}
func (s *Store) Complete(repo, rid, actor string, expected int64, in CompletionInput) (Removal, error) {
	required := []string{"code", "flags", "data", "credentials", "telemetry", "documentation", "policy_exceptions"}
	if actor == "" || in.CompletedRevision == "" || len(in.OutcomeMeasures) == 0 || len(in.HistoricalEvidence) == 0 {
		return Removal{}, ErrInvalid
	}
	return s.mutate(repo, rid, expected, func(r *Removal) error {
		if !contains(r.Input.OwnerIDs, actor) {
			return ErrForbidden
		}
		if r.ActiveStageID != r.Input.Stages[len(r.Input.Stages)-1].ID {
			return ErrConflict
		}
		rr := s.derive(*r)
		if len(rr.Blockers) > 0 {
			return ErrConflict
		}
		seen := map[string]bool{}
		for _, x := range in.Cleanup {
			expectedStatus := map[string]string{"code": "removed", "flags": "removed", "data": "deleted", "credentials": "revoked", "telemetry": "removed", "documentation": "removed", "policy_exceptions": "removed"}[x.Category]
			if expectedStatus == "" || x.Subject == "" || x.Status != expectedStatus || x.EvidenceReference == "" || x.Revision == "" {
				return ErrInvalid
			}
			seen[x.Category] = true
		}
		for _, x := range required {
			if !seen[x] {
				return ErrInvalid
			}
		}
		r.Completion = &Completion{in, actor, s.now().UTC()}
		r.Compatibility = "removed"
		return nil
	})
}
func (s *Store) save(r Removal) error {
	if e := os.MkdirAll(filepath.Dir(s.path(r.RepositoryID, r.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(r, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(r.RepositoryID, r.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) read(repo, rid string) (Removal, error) {
	b, e := os.ReadFile(s.path(repo, rid))
	if errors.Is(e, fs.ErrNotExist) {
		return Removal{}, ErrNotFound
	}
	var r Removal
	if e != nil || json.Unmarshal(b, &r) != nil || r.RepositoryID != repo || r.ID != rid {
		return Removal{}, ErrNotFound
	}
	return s.derive(r), nil
}
func (s *Store) Get(repo, rid string) (Removal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, rid)
}
func (s *Store) List(repo string) ([]Removal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Removal{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Removal{}
	for _, f := range es {
		if filepath.Ext(f.Name()) == ".json" {
			r, e := s.read(repo, strings.TrimSuffix(f.Name(), ".json"))
			if e != nil {
				return nil, e
			}
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
