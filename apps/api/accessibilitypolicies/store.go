// Package accessibilitypolicies binds accessibility evidence to merge and release candidates.
package accessibilitypolicies

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

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilityassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
)

var ErrInvalid = errors.New("invalid accessibility delivery policy")
var ErrNotFound = errors.New("accessibility delivery policy not found")

type ScenarioRequirement struct {
	ScenarioID          string   `json:"scenario_id"`
	RequiredEvaluations []string `json:"required_evaluations"`
	RequiredRoles       []string `json:"required_roles"`
}
type PolicyInput struct {
	Name              string                `json:"name"`
	CommitmentID      string                `json:"commitment_id"`
	CommitmentVersion int64                 `json:"commitment_version"`
	TargetBranches    []string              `json:"target_branches"`
	Paths             []string              `json:"paths,omitempty"`
	Journeys          []string              `json:"journeys,omitempty"`
	RiskClasses       []string              `json:"risk_classes,omitempty"`
	RequiredChecks    []string              `json:"required_checks"`
	Scenarios         []ScenarioRequirement `json:"scenarios"`
}
type Policy struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	PolicyInput
	CreatedByID string    `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type Acknowledgement struct {
	ID            string    `json:"id"`
	PolicyID      string    `json:"policy_id"`
	PullRequestID string    `json:"pull_request_id"`
	PreviewID     string    `json:"preview_id"`
	Revision      string    `json:"revision"`
	ScenarioID    string    `json:"scenario_id"`
	Role          string    `json:"role"`
	Decision      string    `json:"decision"`
	Rationale     string    `json:"rationale"`
	ActorID       string    `json:"actor_id"`
	CreatedAt     time.Time `json:"created_at"`
}
type FollowUp struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Summary    string `json:"summary"`
}
type Override struct {
	ID                string    `json:"id"`
	AcknowledgementID string    `json:"acknowledgement_id"`
	Rationale         string    `json:"rationale"`
	FollowUp          FollowUp  `json:"follow_up"`
	ActorID           string    `json:"actor_id"`
	CreatedAt         time.Time `json:"created_at"`
}
type Ledger struct {
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	Overrides        []Override        `json:"overrides"`
}
type Requirement struct {
	PolicyID   string `json:"policy_id"`
	ScenarioID string `json:"scenario_id,omitempty"`
	Kind       string `json:"kind"`
	Detail     string `json:"detail"`
	Blocking   bool   `json:"blocking"`
}
type Assessment struct {
	Revision         string            `json:"revision"`
	AppliedPolicyIDs []string          `json:"applied_policy_ids"`
	Requirements     []Requirement     `json:"requirements"`
	ActiveExceptions []string          `json:"active_exceptions"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	Overrides        []Override        `json:"overrides"`
	Ready            bool              `json:"ready"`
}
type Evidence struct {
	Assessments      []accessibilityassessments.Assessment
	Runs             []checkruns.Run
	ActiveExceptions []string
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
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func ident() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func validList(v []string, required bool) bool {
	if required && len(v) == 0 || len(v) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range v {
		x = strings.TrimSpace(x)
		if x == "" || len(x) > 500 || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func valid(in PolicyInput) bool {
	if strings.TrimSpace(in.Name) == "" || in.CommitmentID == "" || in.CommitmentVersion < 1 || !validList(in.TargetBranches, true) || !validList(in.RequiredChecks, true) || len(in.Scenarios) == 0 || len(in.Scenarios) > 100 || !validList(in.Paths, false) || !validList(in.Journeys, false) || !validList(in.RiskClasses, false) {
		return false
	}
	seen := map[string]bool{}
	for _, s := range in.Scenarios {
		if s.ScenarioID == "" || seen[s.ScenarioID] || !validList(s.RequiredEvaluations, true) || !validList(s.RequiredRoles, true) {
			return false
		}
		seen[s.ScenarioID] = true
	}
	return true
}
func (s *Store) policyPath(repo, id string) string {
	return filepath.Join(s.root, repo, "policies", id+".json")
}
func (s *Store) ledgerPath(repo, pull string) string {
	return filepath.Join(s.root, repo, "ledgers", pull+".json")
}
func write(path string, v any) error {
	if e := os.MkdirAll(filepath.Dir(path), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, b, 0640)
}
func read[T any](path string) (T, error) {
	var v T
	b, e := os.ReadFile(path)
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) Create(repo, actor string, in PolicyInput) (Policy, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Policy{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := Policy{ID: ident(), RepositoryID: repo, PolicyInput: in, CreatedByID: actor, CreatedAt: s.now().UTC()}
	return p, write(s.policyPath(repo, p.ID), p)
}
func (s *Store) List(repo string) ([]Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo, "policies"))
	if errors.Is(e, fs.ErrNotExist) {
		return []Policy{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Policy{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			p, er := read[Policy](filepath.Join(s.root, repo, "policies", x.Name()))
			if er != nil {
				return nil, er
			}
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) ledger(repo, pull string) Ledger {
	v, _ := read[Ledger](s.ledgerPath(repo, pull))
	if v.Acknowledgements == nil {
		v.Acknowledgements = []Acknowledgement{}
	}
	if v.Overrides == nil {
		v.Overrides = []Override{}
	}
	return v
}
func (s *Store) Acknowledge(repo, pull, policy, preview, revision, scenario, role, decision, rationale, actor string) (Acknowledgement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := read[Policy](s.policyPath(repo, policy))
	if e != nil {
		return Acknowledgement{}, e
	}
	ok := false
	for _, x := range p.Scenarios {
		if x.ScenarioID == scenario {
			for _, r := range x.RequiredRoles {
				ok = ok || r == role
			}
		}
	}
	if !ok || pull == "" || preview == "" || revision == "" || actor == "" || (decision != "confirmed" && decision != "rejected") || strings.TrimSpace(rationale) == "" || len(rationale) > 4000 {
		return Acknowledgement{}, ErrInvalid
	}
	a := Acknowledgement{ID: ident(), PolicyID: policy, PullRequestID: pull, PreviewID: preview, Revision: revision, ScenarioID: scenario, Role: role, Decision: decision, Rationale: rationale, ActorID: actor, CreatedAt: s.now().UTC()}
	v := s.ledger(repo, pull)
	v.Acknowledgements = append(v.Acknowledgements, a)
	return a, write(s.ledgerPath(repo, pull), v)
}
func (s *Store) Override(repo, pull, ack, actor, rationale string, follow FollowUp) (Override, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.ledger(repo, pull)
	found := false
	for _, a := range v.Acknowledgements {
		found = found || (a.ID == ack && a.Decision == "rejected")
	}
	if !found || actor == "" || strings.TrimSpace(rationale) == "" || follow.ResourceID == "" || follow.Summary == "" || !map[string]bool{"issue": true, "proposal": true, "task": true}[follow.Kind] {
		return Override{}, ErrInvalid
	}
	o := Override{ID: ident(), AcknowledgementID: ack, Rationale: rationale, FollowUp: follow, ActorID: actor, CreatedAt: s.now().UTC()}
	v.Overrides = append(v.Overrides, o)
	return o, write(s.ledgerPath(repo, pull), v)
}

func match(p Policy, branch string, paths, journeys, risks []string) bool {
	hit := false
	for _, x := range p.TargetBranches {
		hit = hit || x == branch
	}
	if !hit {
		return false
	}
	return intersects(p.Paths, paths) || intersects(p.Journeys, journeys) || intersects(p.RiskClasses, risks) || len(p.Paths)+len(p.Journeys)+len(p.RiskClasses) == 0
}
func intersects(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y || strings.HasSuffix(x, "/**") && strings.HasPrefix(y, strings.TrimSuffix(x, "**")) {
				return true
			}
		}
	}
	return false
}
func (s *Store) Assess(repo, pull, revision, branch string, paths, journeys, risks []string, e Evidence) (Assessment, error) {
	policies, err := s.List(repo)
	if err != nil {
		return Assessment{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.ledger(repo, pull)
	out := Assessment{Revision: revision, AppliedPolicyIDs: []string{}, Requirements: []Requirement{}, ActiveExceptions: e.ActiveExceptions, Acknowledgements: v.Acknowledgements, Overrides: v.Overrides, Ready: true}
	over := map[string]bool{}
	for _, o := range v.Overrides {
		over[o.AcknowledgementID] = true
	}
	for _, p := range policies {
		if !match(p, branch, paths, journeys, risks) {
			continue
		}
		out.AppliedPolicyIDs = append(out.AppliedPolicyIDs, p.ID)
		for _, name := range p.RequiredChecks {
			status := "missing"
			for _, r := range e.Runs {
				if r.Definition.Name == name {
					if r.CommitID != revision {
						status = "stale"
					} else {
						status = string(r.State)
					}
					if status == "succeeded" {
						break
					}
				}
			}
			if status != "succeeded" {
				out.Requirements = append(out.Requirements, Requirement{PolicyID: p.ID, Kind: "automated_check_" + status, Detail: "Required accessibility check “" + name + "” is " + status + " for the exact candidate.", Blocking: true})
			}
		}
		var current *accessibilityassessments.Assessment
		for i := range e.Assessments {
			a := &e.Assessments[i]
			if a.Revision == revision && a.CommitmentID == p.CommitmentID && a.CommitmentVersion == p.CommitmentVersion {
				current = a
				break
			}
		}
		for _, sc := range p.Scenarios {
			if current == nil {
				out.Requirements = append(out.Requirements, Requirement{PolicyID: p.ID, ScenarioID: sc.ScenarioID, Kind: "missing_evaluation", Detail: "No current assessment proves the declared scenario.", Blocking: true})
				continue
			}
			for _, ev := range sc.RequiredEvaluations {
				covered := false
				for _, g := range current.Gaps {
					if g.ScenarioID == sc.ScenarioID && g.Evaluation == ev {
						covered = false
						goto evaluated
					}
				}
				for _, a := range current.Automation {
					if !a.Stale && a.Status == "succeeded" {
						for _, sid := range a.ScenarioIDs {
							for _, x := range a.Evaluations {
								covered = covered || (sid == sc.ScenarioID && x == ev)
							}
						}
					}
				}
				for _, f := range current.Findings {
					covered = covered || (!f.Stale && f.ScenarioID == sc.ScenarioID && f.Evaluation == ev && f.Result == "passed")
				}
			evaluated:
				if !covered {
					out.Requirements = append(out.Requirements, Requirement{PolicyID: p.ID, ScenarioID: sc.ScenarioID, Kind: "missing_evaluation", Detail: "Required evaluation “" + ev + "” is missing or stale.", Blocking: true})
				}
			}
			for _, f := range current.Findings {
				if !f.Stale && f.ScenarioID == sc.ScenarioID && f.Result == "barrier" && (len(f.Decisions) == 0 || f.Decisions[len(f.Decisions)-1].Outcome == "confirmed") {
					out.Requirements = append(out.Requirements, Requirement{PolicyID: p.ID, ScenarioID: sc.ScenarioID, Kind: "unresolved_barrier", Detail: f.Summary, Blocking: true})
				}
			}
			for _, role := range sc.RequiredRoles {
				accepted := false
				rejected := false
				for _, a := range v.Acknowledgements {
					if a.PolicyID == p.ID && a.ScenarioID == sc.ScenarioID && a.Role == role && a.Revision == revision {
						accepted = accepted || a.Decision == "confirmed"
						rejected = rejected || (a.Decision == "rejected" && !over[a.ID])
						accepted = accepted || (a.Decision == "rejected" && over[a.ID])
					}
				}
				if !accepted || rejected {
					out.Requirements = append(out.Requirements, Requirement{PolicyID: p.ID, ScenarioID: sc.ScenarioID, Kind: "acknowledgement_required", Detail: "Current acknowledgement is required from role “" + role + "”; one participant does not represent every access need.", Blocking: true})
				}
			}
		}
	}
	for _, x := range out.ActiveExceptions {
		out.Requirements = append(out.Requirements, Requirement{Kind: "active_exception", Detail: x, Blocking: false})
	}
	for _, r := range out.Requirements {
		if r.Blocking {
			out.Ready = false
		}
	}
	return out, nil
}
