// Package recoveryresponses retains the shared control record for live restoration.
package recoveryresponses

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/protectionplans"
)

var (
	ErrNotFound = errors.New("recovery response not found")
	ErrInvalid  = errors.New("invalid recovery response")
	ErrConflict = errors.New("recovery response conflict")
)

type Step struct {
	ID             string        `json:"id"`
	Title          string        `json:"title"`
	ResourceID     string        `json:"resource_id"`
	DependsOn      []string      `json:"depends_on"`
	ExecutorKind   string        `json:"executor_kind"`
	ExecutorID     string        `json:"executor_id"`
	Command        string        `json:"command"`
	Expected       string        `json:"expected"`
	Destructive    bool          `json:"destructive"`
	Status         string        `json:"status"`
	StartedAt      *time.Time    `json:"started_at,omitempty"`
	FinishedAt     *time.Time    `json:"finished_at,omitempty"`
	EvidenceDigest string        `json:"evidence_digest,omitempty"`
	Summary        string        `json:"summary,omitempty"`
	ActorID        string        `json:"actor_id,omitempty"`
	Blockers       []string      `json:"blockers"`
	Attempts       []StepAttempt `json:"attempts"`
}
type StepAttempt struct {
	Status         string    `json:"status"`
	Summary        string    `json:"summary"`
	EvidenceDigest string    `json:"evidence_digest"`
	ActorID        string    `json:"actor_id"`
	CreatedAt      time.Time `json:"created_at"`
	Blockers       []string  `json:"blockers"`
}
type ActivationInput struct {
	IdempotencyKey        string   `json:"idempotency_key"`
	TriggerKind           string   `json:"trigger_kind"`
	TriggerID             string   `json:"trigger_id"`
	LossConfirmed         bool     `json:"loss_confirmed"`
	Summary               string   `json:"summary"`
	PlanID                string   `json:"plan_id"`
	PlanVersion           int64    `json:"plan_version"`
	CaptureID             string   `json:"capture_id"`
	WorkspaceID           string   `json:"workspace_id"`
	EnvironmentID         string   `json:"environment_id"`
	EstimatedLoss         string   `json:"estimated_loss"`
	ApproverIDs           []string `json:"approver_ids"`
	CommunicationChannels []string `json:"communication_channels"`
	RollbackOptions       []string `json:"rollback_options"`
	Steps                 []Step   `json:"steps"`
}
type Approval struct {
	ActorID   string    `json:"actor_id"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	CreatedAt time.Time `json:"created_at"`
}
type Decision struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	ActorID   string    `json:"actor_id"`
	Rationale string    `json:"rationale"`
	CreatedAt time.Time `json:"created_at"`
}
type Communication struct {
	ActorID   string    `json:"actor_id"`
	Audience  string    `json:"audience"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
type Validation struct {
	ActorID        string    `json:"actor_id"`
	Passed         bool      `json:"passed"`
	EvidenceDigest string    `json:"evidence_digest"`
	Summary        string    `json:"summary"`
	CreatedAt      time.Time `json:"created_at"`
}
type Response struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Revision     int64  `json:"revision"`
	ActivationInput
	ActivatedBy       string          `json:"activated_by"`
	ActivatedAt       time.Time       `json:"activated_at"`
	Approvals         []Approval      `json:"approvals"`
	Decisions         []Decision      `json:"decisions"`
	Communications    []Communication `json:"communications"`
	Validations       []Validation    `json:"validations"`
	State             string          `json:"state"`
	Blockers          []string        `json:"blockers"`
	NextStepIDs       []string        `json:"next_step_ids"`
	ProgressCompleted int             `json:"progress_completed"`
	ProgressTotal     int             `json:"progress_total"`
}
type StepUpdate struct {
	ExpectedRevision   int64  `json:"expected_revision"`
	Status             string `json:"status"`
	Summary            string `json:"summary"`
	EvidenceDigest     string `json:"evidence_digest"`
	ConflictingWrites  bool   `json:"conflicting_writes"`
	KeyAvailable       bool   `json:"key_available"`
	ReplicaCurrent     bool   `json:"replica_current"`
	PartialRestoration bool   `json:"partial_restoration"`
}
type DecisionInput struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Kind             string `json:"kind"`
	Rationale        string `json:"rationale"`
}
type ValidationInput struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Passed           bool   `json:"passed"`
	EvidenceDigest   string `json:"evidence_digest"`
	Summary          string `json:"summary"`
}
type CommunicationInput struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Audience         string `json:"audience"`
	Message          string `json:"message"`
}
type Store struct {
	root  string
	plans *protectionplans.Store
	mu    sync.Mutex
	now   func() time.Time
}

func New(root string, plans *protectionplans.Store) (*Store, error) {
	if strings.TrimSpace(root) == "" || plans == nil {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, plans: plans, now: time.Now}, e
}
func newid() string                          { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) load(repo, id string) (Response, error) {
	var x Response
	b, e := os.ReadFile(s.path(repo, id))
	if os.IsNotExist(e) {
		return x, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) save(x Response) error {
	p := s.path(x.RepositoryID, x.ID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e != nil {
		return e
	}
	t := p + ".tmp"
	if e = os.WriteFile(t, b, 0640); e != nil {
		return e
	}
	return os.Rename(t, p)
}
func selectedCapture(p protectionplans.Plan, id string) (protectionplans.Capture, bool) {
	for _, c := range p.Captures {
		if c.ID == id {
			return c, true
		}
	}
	return protectionplans.Capture{}, false
}
func validActivation(in ActivationInput) bool {
	if in.IdempotencyKey == "" || (in.TriggerKind != "incident" && in.TriggerKind != "loss_event") || in.TriggerID == "" || !in.LossConfirmed || in.Summary == "" || in.PlanID == "" || in.PlanVersion < 1 || in.CaptureID == "" || in.WorkspaceID == "" || in.EnvironmentID == "" || in.EstimatedLoss == "" || len(in.ApproverIDs) == 0 || len(in.CommunicationChannels) == 0 || len(in.RollbackOptions) == 0 || len(in.Steps) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, a := range in.ApproverIDs {
		if a == "" || seen[a] {
			return false
		}
		seen[a] = true
	}
	done := map[string]bool{}
	for _, x := range in.Steps {
		if x.ID == "" || x.Title == "" || x.ResourceID == "" || (x.ExecutorKind != "human" && x.ExecutorKind != "agent") || x.ExecutorID == "" || x.Command == "" || x.Expected == "" || done[x.ID] {
			return false
		}
		for _, d := range x.DependsOn {
			if !done[d] {
				return false
			}
		}
		done[x.ID] = true
	}
	return true
}
func (s *Store) derive(x Response) Response {
	x.Blockers = nil
	x.NextStepIDs = nil
	x.ProgressTotal = len(x.Steps)
	x.ProgressCompleted = 0
	approved := map[string]bool{}
	rejected := false
	for _, a := range x.Approvals {
		approved[a.ActorID] = a.Decision == "approve"
		if a.Decision == "reject" {
			rejected = true
		}
	}
	for _, id := range x.ApproverIDs {
		if !approved[id] {
			x.Blockers = append(x.Blockers, "approval_required:"+id)
		}
	}
	if rejected {
		x.Blockers = append(x.Blockers, "approval_rejected")
	}
	p, e := s.plans.Get(x.RepositoryID, x.PlanID)
	if e != nil {
		x.Blockers = append(x.Blockers, "protection_plan_unavailable")
	} else {
		c, ok := selectedCapture(p, x.CaptureID)
		if !ok || c.PlanVersion != x.PlanVersion || !c.Recoverable {
			x.Blockers = append(x.Blockers, "recovery_point_unavailable")
		}
	}
	failed := false
	complete := map[string]bool{}
	for _, st := range x.Steps {
		x.Blockers = append(x.Blockers, st.Blockers...)
		if st.Status == "succeeded" {
			complete[st.ID] = true
			x.ProgressCompleted++
		}
		if st.Status == "failed" {
			failed = true
		}
	}
	if failed {
		x.Blockers = append(x.Blockers, "restoration_step_failed")
	}
	for _, v := range x.Validations {
		if !v.Passed {
			x.Blockers = append(x.Blockers, "validation_failed")
		}
	}
	x.State = "awaiting_approval"
	if len(x.Blockers) == 0 {
		x.State = "active"
	}
	if failed || containsPrefix(x.Blockers, "validation_failed") {
		x.State = "paused"
	}
	if x.ProgressCompleted == x.ProgressTotal {
		if len(x.Validations) == 0 {
			x.Blockers = append(x.Blockers, "validation_required")
			x.State = "validating"
		} else if x.Validations[len(x.Validations)-1].Passed {
			x.State = "restored"
		}
	}
	if x.State == "active" {
		for _, st := range x.Steps {
			if st.Status != "" && st.Status != "pending" {
				continue
			}
			ready := true
			for _, d := range st.DependsOn {
				if !complete[d] {
					ready = false
				}
			}
			if ready {
				x.NextStepIDs = append(x.NextStepIDs, st.ID)
			}
		}
	}
	if len(x.Decisions) > 0 {
		switch x.Decisions[len(x.Decisions)-1].Kind {
		case "pause":
			x.State = "paused"
			x.Blockers = append(x.Blockers, "responder_paused")
			x.NextStepIDs = nil
		case "rollback":
			x.State = "rolling_back"
			x.NextStepIDs = nil
		case "cancel":
			x.State = "cancelled"
			x.NextStepIDs = nil
		}
	}
	x.Blockers = unique(x.Blockers)
	return x
}
func containsPrefix(v []string, p string) bool {
	for _, x := range v {
		if strings.HasPrefix(x, p) {
			return true
		}
	}
	return false
}
func unique(v []string) []string {
	m := map[string]bool{}
	o := []string{}
	for _, x := range v {
		if !m[x] {
			m[x] = true
			o = append(o, x)
		}
	}
	sort.Strings(o)
	return o
}
func (s *Store) Activate(repo, actor string, in ActivationInput) (Response, error) {
	if repo == "" || actor == "" || !validActivation(in) {
		return Response{}, ErrInvalid
	}
	p, e := s.plans.Get(repo, in.PlanID)
	if e != nil || p.CurrentVersion != in.PlanVersion {
		return Response{}, ErrInvalid
	}
	c, ok := selectedCapture(p, in.CaptureID)
	if !ok || !c.Recoverable || c.PlanVersion != in.PlanVersion {
		return Response{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, _ := os.ReadDir(filepath.Join(s.root, repo))
	for _, f := range items {
		var old Response
		b, _ := os.ReadFile(filepath.Join(s.root, repo, f.Name()))
		if json.Unmarshal(b, &old) == nil && old.IdempotencyKey == in.IdempotencyKey {
			return s.derive(old), nil
		}
	}
	for i := range in.Steps {
		in.Steps[i].Status = "pending"
	}
	x := Response{ID: newid(), RepositoryID: repo, Revision: 1, ActivationInput: in, ActivatedBy: actor, ActivatedAt: s.now().UTC(), Approvals: []Approval{}, Decisions: []Decision{}, Communications: []Communication{}, Validations: []Validation{}}
	e = s.save(x)
	return s.derive(x), e
}
func (s *Store) mutate(repo, id string, expected int64, fn func(*Response) error) (Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, id)
	if e != nil {
		return x, e
	}
	if x.Revision != expected {
		return Response{}, ErrConflict
	}
	if e = fn(&x); e != nil {
		return Response{}, e
	}
	x.Revision++
	if e = s.save(x); e != nil {
		return x, e
	}
	return s.derive(x), nil
}
func (s *Store) Approve(repo, id, actor string, in DecisionInput) (Response, error) {
	return s.mutate(repo, id, in.ExpectedRevision, func(x *Response) error {
		named := false
		for _, a := range x.ApproverIDs {
			if a == actor {
				named = true
			}
		}
		if !named || (in.Kind != "approve" && in.Kind != "reject") || in.Rationale == "" {
			return ErrInvalid
		}
		for _, a := range x.Approvals {
			if a.ActorID == actor {
				return ErrConflict
			}
		}
		x.Approvals = append(x.Approvals, Approval{ActorID: actor, Decision: in.Kind, Rationale: in.Rationale, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Decide(repo, id, actor string, in DecisionInput) (Response, error) {
	return s.mutate(repo, id, in.ExpectedRevision, func(x *Response) error {
		if actor == "" || in.Rationale == "" || (in.Kind != "pause" && in.Kind != "resume" && in.Kind != "approve_cutover" && in.Kind != "rollback" && in.Kind != "cancel") {
			return ErrInvalid
		}
		if in.Kind == "resume" {
			if s.derive(*x).State != "paused" {
				return ErrConflict
			}
			for i := range x.Steps {
				if x.Steps[i].Status == "failed" {
					x.Steps[i].Status = "pending"
					x.Steps[i].Blockers = nil
				}
			}
		}
		x.Decisions = append(x.Decisions, Decision{ID: newid(), Kind: in.Kind, ActorID: actor, Rationale: in.Rationale, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) UpdateStep(repo, id, step, actor string, in StepUpdate) (Response, error) {
	return s.mutate(repo, id, in.ExpectedRevision, func(x *Response) error {
		d := s.derive(*x)
		if d.State != "active" {
			return ErrConflict
		}
		for i := range x.Steps {
			st := &x.Steps[i]
			if st.ID != step {
				continue
			}
			if st.ExecutorID != actor || in.Summary == "" || in.EvidenceDigest == "" || (in.Status != "succeeded" && in.Status != "failed") {
				return ErrInvalid
			}
			ready := false
			for _, n := range d.NextStepIDs {
				ready = ready || n == step
			}
			if !ready {
				return ErrConflict
			}
			if st.Destructive {
				cut := false
				for _, q := range x.Decisions {
					cut = cut || q.Kind == "approve_cutover"
				}
				if !cut {
					return ErrConflict
				}
			}
			now := s.now().UTC()
			st.StartedAt = &now
			st.FinishedAt = &now
			st.Status = in.Status
			st.Summary = in.Summary
			st.EvidenceDigest = in.EvidenceDigest
			st.ActorID = actor
			st.Blockers = nil
			if in.ConflictingWrites || !in.KeyAvailable || !in.ReplicaCurrent || in.PartialRestoration {
				st.Status = "failed"
				if in.ConflictingWrites {
					st.Blockers = append(st.Blockers, "conflicting_writes")
				}
				if !in.KeyAvailable {
					st.Blockers = append(st.Blockers, "key_unavailable")
				}
				if !in.ReplicaCurrent {
					st.Blockers = append(st.Blockers, "stale_replica")
				}
				if in.PartialRestoration {
					st.Blockers = append(st.Blockers, "partial_restoration")
				}
			}
			st.Attempts = append(st.Attempts, StepAttempt{Status: st.Status, Summary: st.Summary, EvidenceDigest: st.EvidenceDigest, ActorID: actor, CreatedAt: now, Blockers: append([]string{}, st.Blockers...)})
			return nil
		}
		return ErrNotFound
	})
}
func (s *Store) Communicate(repo, id, actor string, in CommunicationInput) (Response, error) {
	return s.mutate(repo, id, in.ExpectedRevision, func(x *Response) error {
		if actor == "" || in.Audience == "" || in.Message == "" {
			return ErrInvalid
		}
		x.Communications = append(x.Communications, Communication{ActorID: actor, Audience: in.Audience, Message: in.Message, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Validate(repo, id, actor string, in ValidationInput) (Response, error) {
	return s.mutate(repo, id, in.ExpectedRevision, func(x *Response) error {
		if actor == "" || in.EvidenceDigest == "" || in.Summary == "" {
			return ErrInvalid
		}
		if s.derive(*x).ProgressCompleted != len(x.Steps) {
			return ErrConflict
		}
		x.Validations = append(x.Validations, Validation{ActorID: actor, Passed: in.Passed, EvidenceDigest: in.EvidenceDigest, Summary: in.Summary, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Get(repo, id string) (Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, id)
	return s.derive(x), e
}
func (s *Store) List(repo string) ([]Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fs, e := os.ReadDir(filepath.Join(s.root, repo))
	if os.IsNotExist(e) {
		return []Response{}, nil
	}
	if e != nil {
		return nil, e
	}
	o := []Response{}
	for _, f := range fs {
		if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, e := s.load(repo, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		o = append(o, s.derive(x))
	}
	sort.Slice(o, func(i, j int) bool { return o[i].ActivatedAt.After(o[j].ActivatedAt) })
	return o, nil
}
