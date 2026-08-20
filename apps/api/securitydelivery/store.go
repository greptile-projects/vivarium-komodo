// Package securitydelivery governs revision-exact security assurance and post-delivery drift.
package securitydelivery

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

var ErrInvalid = errors.New("invalid security delivery record")

type PolicyInput struct {
	Name                    string   `json:"name"`
	ScopeKind               string   `json:"scope_kind"`
	ScopeID                 string   `json:"scope_id"`
	Branches                []string `json:"branches"`
	Components              []string `json:"components,omitempty"`
	Assets                  []string `json:"assets,omitempty"`
	RiskClasses             []string `json:"risk_classes,omitempty"`
	RequiredThreatModels    []string `json:"required_threat_models"`
	RequiredScenarios       []string `json:"required_scenarios"`
	RequiredControlOwnerIDs []string `json:"required_control_owner_ids"`
	RequireResolvedFindings bool     `json:"require_resolved_findings"`
}
type Policy struct {
	ID string `json:"id"`
	PolicyInput
	CreatedByID string    `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type Evidence struct {
	ThreatModelID        string   `json:"threat_model_id"`
	ThreatModelRevision  string   `json:"threat_model_revision"`
	InputKeys            []string `json:"input_keys"`
	Current              bool     `json:"current"`
	ResidualRisk         string   `json:"residual_risk"`
	ScenarioID           string   `json:"scenario_id,omitempty"`
	ScenarioVersion      int64    `json:"scenario_version,omitempty"`
	AttemptID            string   `json:"attempt_id,omitempty"`
	AttemptRevision      string   `json:"attempt_revision,omitempty"`
	AttemptStatus        string   `json:"attempt_status,omitempty"`
	Coverage             []string `json:"coverage,omitempty"`
	UnresolvedFindingIDs []string `json:"unresolved_finding_ids,omitempty"`
}
type Acknowledgement struct {
	ID          string    `json:"id"`
	PolicyID    string    `json:"policy_id"`
	SubjectKind string    `json:"subject_kind"`
	SubjectID   string    `json:"subject_id"`
	Revision    string    `json:"revision"`
	OwnerID     string    `json:"owner_id"`
	Decision    string    `json:"decision"`
	Rationale   string    `json:"rationale"`
	CreatedAt   time.Time `json:"created_at"`
}
type Exception struct {
	ID               string    `json:"id"`
	PolicyID         string    `json:"policy_id"`
	SubjectKind      string    `json:"subject_kind"`
	SubjectID        string    `json:"subject_id"`
	Revision         string    `json:"revision"`
	Reason           string    `json:"reason"`
	ApprovedByID     string    `json:"approved_by_id"`
	RequirementKinds []string  `json:"requirement_kinds"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
}
type Requirement struct {
	PolicyID     string   `json:"policy_id"`
	Kind         string   `json:"kind"`
	Reference    string   `json:"reference"`
	Detail       string   `json:"detail"`
	InputKeys    []string `json:"input_keys,omitempty"`
	Blocking     bool     `json:"blocking"`
	ExceptedByID string   `json:"excepted_by_id,omitempty"`
}
type Assessment struct {
	SubjectKind      string            `json:"subject_kind"`
	SubjectID        string            `json:"subject_id"`
	Revision         string            `json:"revision"`
	AppliedPolicyIDs []string          `json:"applied_policy_ids"`
	Evidence         []Evidence        `json:"evidence"`
	Requirements     []Requirement     `json:"requirements"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	ActiveExceptions []Exception       `json:"active_exceptions"`
	ResidualRisk     []string          `json:"residual_risk"`
	Ready            bool              `json:"ready"`
}
type SignalInput struct {
	DeploymentID string    `json:"deployment_id"`
	ReleaseID    string    `json:"release_id"`
	Revision     string    `json:"revision"`
	Environment  string    `json:"environment"`
	Assumption   string    `json:"assumption"`
	ControlID    string    `json:"control_id"`
	Outcome      string    `json:"outcome"`
	Summary      string    `json:"summary"`
	InputKeys    []string  `json:"input_keys"`
	ObservedAt   time.Time `json:"observed_at"`
}
type Signal struct {
	ID string `json:"id"`
	SignalInput
	Sanitized bool      `json:"sanitized"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
	Violated  bool      `json:"violated"`
	Response  *Response `json:"response,omitempty"`
}
type Response struct {
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	OpenedByID string    `json:"opened_by_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type ledger struct {
	Policies         []Policy          `json:"policies"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	Exceptions       []Exception       `json:"exceptions"`
	Signals          []Signal          `json:"signals"`
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
func newID() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func listOK(xs []string, required bool) bool {
	if required && len(xs) == 0 || len(xs) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || len(x) > 500 || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func validPolicy(x PolicyInput) bool {
	return strings.TrimSpace(x.Name) != "" && map[string]bool{"repository": true, "organization": true}[x.ScopeKind] && x.ScopeID != "" && listOK(x.Branches, true) && listOK(x.Components, false) && listOK(x.Assets, false) && listOK(x.RiskClasses, false) && listOK(x.RequiredThreatModels, false) && listOK(x.RequiredScenarios, false) && listOK(x.RequiredControlOwnerIDs, false) && (len(x.RequiredThreatModels)+len(x.RequiredScenarios)+len(x.RequiredControlOwnerIDs) > 0 || x.RequireResolvedFindings)
}
func (s *Store) path(scope string) string { return filepath.Join(s.root, scope+".json") }
func (s *Store) read(scope string) (ledger, error) {
	l := ledger{[]Policy{}, []Acknowledgement{}, []Exception{}, []Signal{}}
	b, e := os.ReadFile(s.path(scope))
	if errors.Is(e, fs.ErrNotExist) {
		return l, nil
	}
	if e != nil {
		return l, e
	}
	if json.Unmarshal(b, &l) != nil {
		return l, ErrInvalid
	}
	return l, nil
}
func (s *Store) write(scope string, l ledger) error {
	b, e := json.MarshalIndent(l, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(scope), b, 0640)
	}
	return e
}
func (s *Store) CreatePolicy(scope, actor string, in PolicyInput) (Policy, error) {
	if scope == "" || actor == "" || scope != in.ScopeKind+":"+in.ScopeID || !validPolicy(in) {
		return Policy{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, e := s.read(scope)
	if e != nil {
		return Policy{}, e
	}
	p := Policy{newID(), in, actor, s.now().UTC()}
	l.Policies = append(l.Policies, p)
	return p, s.write(scope, l)
}
func (s *Store) ListPolicies(scope string) ([]Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, e := s.read(scope)
	return l.Policies, e
}
func has(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func applies(p Policy, branch string, components, assets, risks []string) bool {
	if !has(p.Branches, branch) {
		return false
	}
	match := func(rule, actual []string) bool {
		if len(rule) == 0 {
			return true
		}
		for _, r := range rule {
			for _, a := range actual {
				if a == r || strings.HasPrefix(a, strings.TrimSuffix(r, "/")+"/") {
					return true
				}
			}
		}
		return false
	}
	return match(p.Components, components) && match(p.Assets, assets) && match(p.RiskClasses, risks)
}
func (s *Store) Acknowledge(scope, policy, kind, subject, revision, actor, decision, rationale string) (Acknowledgement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, e := s.read(scope)
	if e != nil {
		return Acknowledgement{}, e
	}
	owner := false
	for _, p := range l.Policies {
		if p.ID == policy {
			owner = has(p.RequiredControlOwnerIDs, actor)
		}
	}
	if !owner || subject == "" || revision == "" || !map[string]bool{"accept": true, "request_changes": true}[decision] || strings.TrimSpace(rationale) == "" {
		return Acknowledgement{}, ErrInvalid
	}
	a := Acknowledgement{newID(), policy, kind, subject, revision, actor, decision, rationale, s.now().UTC()}
	l.Acknowledgements = append(l.Acknowledgements, a)
	return a, s.write(scope, l)
}
func (s *Store) Except(scope, policy, kind, subject, revision, actor, reason string, kinds []string, expires time.Time) (Exception, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, e := s.read(scope)
	if e != nil {
		return Exception{}, e
	}
	found := false
	for _, p := range l.Policies {
		found = found || p.ID == policy
	}
	now := s.now().UTC()
	if !found || subject == "" || revision == "" || actor == "" || strings.TrimSpace(reason) == "" || !listOK(kinds, true) || !expires.After(now) || expires.After(now.Add(30*24*time.Hour)) {
		return Exception{}, ErrInvalid
	}
	x := Exception{newID(), policy, kind, subject, revision, reason, actor, kinds, expires, now}
	l.Exceptions = append(l.Exceptions, x)
	return x, s.write(scope, l)
}
func (s *Store) Assess(scopes []string, kind, subject, revision, branch string, components, assets, risks []string, evidence []Evidence) (Assessment, error) {
	a := Assessment{kind, subject, revision, []string{}, evidence, []Requirement{}, []Acknowledgement{}, []Exception{}, []string{}, true}
	now := s.now().UTC()
	for _, scope := range scopes {
		s.mu.Lock()
		l, e := s.read(scope)
		s.mu.Unlock()
		if e != nil {
			return a, e
		}
		for _, p := range l.Policies {
			if !applies(p, branch, components, assets, risks) {
				continue
			}
			a.AppliedPolicyIDs = append(a.AppliedPolicyIDs, p.ID)
			active := []Exception{}
			for _, x := range l.Exceptions {
				if x.PolicyID == p.ID && x.SubjectKind == kind && x.SubjectID == subject && x.Revision == revision && x.ExpiresAt.After(now) {
					active = append(active, x)
					a.ActiveExceptions = append(a.ActiveExceptions, x)
				}
			}
			add := func(k, ref, detail string, keys []string) {
				r := Requirement{PolicyID: p.ID, Kind: k, Reference: ref, Detail: detail, InputKeys: keys, Blocking: true}
				for _, x := range active {
					if has(x.RequirementKinds, k) {
						r.Blocking = false
						r.ExceptedByID = x.ID
					}
				}
				a.Requirements = append(a.Requirements, r)
				a.Ready = a.Ready && !r.Blocking
			}
			for _, id := range p.RequiredThreatModels {
				found := false
				for _, v := range evidence {
					if v.ThreatModelID == id && v.Current {
						found = true
						if v.ResidualRisk != "" && !has(a.ResidualRisk, v.ResidualRisk) {
							a.ResidualRisk = append(a.ResidualRisk, v.ResidualRisk)
						}
					}
				}
				if !found {
					add("threat_coverage", id, "a current threat model does not cover this exact revision", nil)
				}
			}
			for _, id := range p.RequiredScenarios {
				found := false
				var keys []string
				for _, v := range evidence {
					if v.ScenarioID == id {
						keys = v.InputKeys
						found = v.Current && v.AttemptRevision == revision && v.AttemptStatus == "passed" && has(v.Coverage, "containment") && has(v.Coverage, "detection") && has(v.Coverage, "recovery")
					}
				}
				if !found {
					add("scenario_result", id, "a current passing three-domain scenario attempt is required", keys)
				}
			}
			if p.RequireResolvedFindings {
				for _, v := range evidence {
					for _, id := range v.UnresolvedFindingIDs {
						add("unresolved_finding", id, "a current confirmed finding remains unresolved", v.InputKeys)
					}
				}
			}
			for _, owner := range p.RequiredControlOwnerIDs {
				var ack *Acknowledgement
				for i := range l.Acknowledgements {
					x := &l.Acknowledgements[i]
					if x.PolicyID == p.ID && x.SubjectKind == kind && x.SubjectID == subject && x.Revision == revision && x.OwnerID == owner {
						ack = x
					}
				}
				if ack != nil {
					a.Acknowledgements = append(a.Acknowledgements, *ack)
				}
				if ack == nil || ack.Decision != "accept" {
					add("control_owner_acknowledgement", owner, "the named control owner has not accepted the exact candidate evidence", nil)
				}
			}
		}
	}
	sort.Strings(a.AppliedPolicyIDs)
	sort.Strings(a.ResidualRisk)
	return a, nil
}
func (s *Store) RecordSignal(scope, actor string, in SignalInput, sanitized bool) (Signal, error) {
	if actor == "" || in.DeploymentID == "" || in.ReleaseID == "" || in.Revision == "" || in.Environment == "" || in.Assumption == "" || in.ControlID == "" || !map[string]bool{"satisfied": true, "violated": true, "control_failed": true, "inconclusive": true}[in.Outcome] || in.Summary == "" || !sanitized || !listOK(in.InputKeys, true) {
		return Signal{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, e := s.read(scope)
	if e != nil {
		return Signal{}, e
	}
	x := Signal{ID: newID(), SignalInput: in, Sanitized: true, ActorID: actor, CreatedAt: s.now().UTC(), Violated: in.Outcome == "violated" || in.Outcome == "control_failed"}
	l.Signals = append(l.Signals, x)
	return x, s.write(scope, l)
}
func (s *Store) OpenResponse(scope, signal, actor, kind, resource string) (Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, e := s.read(scope)
	if e != nil {
		return Signal{}, e
	}
	for i := range l.Signals {
		x := &l.Signals[i]
		if x.ID == signal && x.Violated && x.Response == nil && actor != "" && resource != "" && map[string]bool{"private_incident": true, "security_advisory": true, "repair": true}[kind] {
			x.Response = &Response{kind, resource, actor, s.now().UTC()}
			return *x, s.write(scope, l)
		}
	}
	return Signal{}, ErrInvalid
}
func (s *Store) ListSignals(scope string) ([]Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, e := s.read(scope)
	return l.Signals, e
}
