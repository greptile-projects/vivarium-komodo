// Package privacyverification governs revision-exact synthetic privacy evidence.
package privacyverification

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

	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
)

var ErrInvalid = errors.New("invalid privacy verification policy")
var ErrNotFound = errors.New("privacy verification policy not found")

var dimensions = map[string]bool{"collection": true, "consent": true, "minimization": true, "access": true, "retention": true, "export": true, "deletion": true, "telemetry": true, "recipient": true}

type PolicyInput struct {
	Name               string   `json:"name"`
	CommitmentID       string   `json:"commitment_id"`
	CommitmentVersion  int64    `json:"commitment_version"`
	TargetBranches     []string `json:"target_branches"`
	Paths              []string `json:"paths,omitempty"`
	RequiredChecks     []string `json:"required_checks"`
	RequiredDimensions []string `json:"required_dimensions"`
	PrivacyOwnerIDs    []string `json:"privacy_owner_ids"`
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
	Decision      string    `json:"decision"`
	Rationale     string    `json:"rationale"`
	OwnerID       string    `json:"owner_id"`
	CreatedAt     time.Time `json:"created_at"`
}
type FollowUp struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
}
type Exception struct {
	ID            string    `json:"id"`
	PolicyID      string    `json:"policy_id"`
	PullRequestID string    `json:"pull_request_id"`
	Revision      string    `json:"revision"`
	CheckNames    []string  `json:"check_names"`
	Dimensions    []string  `json:"dimensions"`
	Reason        string    `json:"reason"`
	FollowUp      FollowUp  `json:"follow_up"`
	ExpiresAt     time.Time `json:"expires_at"`
	ApprovedByID  string    `json:"approved_by_id"`
	CreatedAt     time.Time `json:"created_at"`
}
type Ledger struct {
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	Exceptions       []Exception       `json:"exceptions"`
}
type Requirement struct {
	PolicyID string `json:"policy_id"`
	Kind     string `json:"kind"`
	Name     string `json:"name,omitempty"`
	Detail   string `json:"detail"`
	Blocking bool   `json:"blocking"`
	RunID    string `json:"run_id,omitempty"`
}
type Assessment struct {
	Revision         string            `json:"revision"`
	AppliedPolicyIDs []string          `json:"applied_policy_ids"`
	Coverage         []string          `json:"coverage"`
	Requirements     []Requirement     `json:"requirements"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	ActiveExceptions []Exception       `json:"active_exceptions"`
	Ready            bool              `json:"ready"`
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
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func listOK(v []string, required bool) bool {
	if (required && len(v) == 0) || len(v) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range v {
		if strings.TrimSpace(x) == "" || len(x) > 500 || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func valid(in PolicyInput) bool {
	if strings.TrimSpace(in.Name) == "" || in.CommitmentID == "" || in.CommitmentVersion < 1 || !listOK(in.TargetBranches, true) || !listOK(in.RequiredChecks, true) || !listOK(in.RequiredDimensions, true) || !listOK(in.PrivacyOwnerIDs, true) || !listOK(in.Paths, false) {
		return false
	}
	for _, d := range in.RequiredDimensions {
		if !dimensions[d] {
			return false
		}
	}
	return true
}
func (s *Store) path(repo string) string { return filepath.Join(s.root, repo+".json") }
func (s *Store) read(repo string) ([]Policy, Ledger, error) {
	l := Ledger{Acknowledgements: []Acknowledgement{}, Exceptions: []Exception{}}
	b, e := os.ReadFile(s.path(repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Policy{}, l, nil
	}
	if e != nil {
		return nil, l, e
	}
	var x struct {
		Policies []Policy `json:"policies"`
		Ledger   Ledger   `json:"ledger"`
	}
	if json.Unmarshal(b, &x) != nil {
		return nil, l, ErrInvalid
	}
	return x.Policies, x.Ledger, nil
}
func (s *Store) write(repo string, p []Policy, l Ledger) error {
	b, e := json.MarshalIndent(struct {
		Policies []Policy `json:"policies"`
		Ledger   Ledger   `json:"ledger"`
	}{p, l}, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(repo), b, 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in PolicyInput) (Policy, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Policy{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, l, e := s.read(repo)
	if e != nil {
		return Policy{}, e
	}
	x := Policy{ID: id(), RepositoryID: repo, PolicyInput: in, CreatedByID: actor, CreatedAt: s.now().UTC()}
	p = append(p, x)
	return x, s.write(repo, p, l)
}
func (s *Store) List(repo string) ([]Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _, e := s.read(repo)
	return p, e
}
func find(p []Policy, id string) (Policy, bool) {
	for _, x := range p {
		if x.ID == id {
			return x, true
		}
	}
	return Policy{}, false
}
func (s *Store) Acknowledge(repo, pull, policy, preview, revision, decision, rationale, actor string) (Acknowledgement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, l, e := s.read(repo)
	if e != nil {
		return Acknowledgement{}, e
	}
	x, ok := find(p, policy)
	owner := false
	for _, o := range x.PrivacyOwnerIDs {
		owner = owner || o == actor
	}
	if !ok || !owner || pull == "" || preview == "" || revision == "" || !map[string]bool{"accept": true, "request_changes": true}[decision] || strings.TrimSpace(rationale) == "" {
		return Acknowledgement{}, ErrInvalid
	}
	a := Acknowledgement{ID: id(), PolicyID: policy, PullRequestID: pull, PreviewID: preview, Revision: revision, Decision: decision, Rationale: rationale, OwnerID: actor, CreatedAt: s.now().UTC()}
	l.Acknowledgements = append(l.Acknowledgements, a)
	return a, s.write(repo, p, l)
}
func (s *Store) Except(repo, pull, policy, revision, reason, actor string, checks, dims []string, follow FollowUp, expires time.Time) (Exception, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, l, e := s.read(repo)
	if e != nil {
		return Exception{}, e
	}
	x, ok := find(p, policy)
	owner := false
	for _, o := range x.PrivacyOwnerIDs {
		owner = owner || o == actor
	}
	now := s.now().UTC()
	if !ok || !owner || pull == "" || revision == "" || strings.TrimSpace(reason) == "" || !listOK(checks, false) || !listOK(dims, false) || (len(checks) == 0 && len(dims) == 0) || follow.ResourceID == "" || !map[string]bool{"issue": true, "proposal": true, "task": true}[follow.Kind] || !expires.After(now) || expires.After(now.Add(90*24*time.Hour)) {
		return Exception{}, ErrInvalid
	}
	for _, d := range dims {
		if !dimensions[d] {
			return Exception{}, ErrInvalid
		}
	}
	xv := Exception{ID: id(), PolicyID: policy, PullRequestID: pull, Revision: revision, CheckNames: checks, Dimensions: dims, Reason: reason, FollowUp: follow, ExpiresAt: expires, ApprovedByID: actor, CreatedAt: now}
	l.Exceptions = append(l.Exceptions, xv)
	return xv, s.write(repo, p, l)
}
func matches(p Policy, branch string, paths []string) bool {
	branchOK := false
	for _, b := range p.TargetBranches {
		branchOK = branchOK || b == branch
	}
	if !branchOK {
		return false
	}
	if len(p.Paths) == 0 {
		return true
	}
	for _, changed := range paths {
		for _, prefix := range p.Paths {
			if strings.HasPrefix(changed, strings.TrimSuffix(prefix, "/")+"/") || changed == strings.TrimSuffix(prefix, "/") {
				return true
			}
		}
	}
	return false
}
func contains(v []string, x string) bool {
	for _, y := range v {
		if x == y {
			return true
		}
	}
	return false
}
func (s *Store) Assess(repo, pull, revision, branch string, paths []string, runs []checkruns.Run) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, l, e := s.read(repo)
	if e != nil {
		return Assessment{}, e
	}
	a := Assessment{Revision: revision, AppliedPolicyIDs: []string{}, Coverage: []string{}, Requirements: []Requirement{}, Acknowledgements: []Acknowledgement{}, ActiveExceptions: []Exception{}, Ready: true}
	now := s.now().UTC()
	for _, pol := range p {
		if !matches(pol, branch, paths) {
			continue
		}
		a.AppliedPolicyIDs = append(a.AppliedPolicyIDs, pol.ID)
		active := []Exception{}
		for _, x := range l.Exceptions {
			if x.PolicyID == pol.ID && x.PullRequestID == pull && x.Revision == revision && x.ExpiresAt.After(now) {
				active = append(active, x)
				a.ActiveExceptions = append(a.ActiveExceptions, x)
			}
		}
		exempt := func(name, dim string) bool {
			for _, x := range active {
				if contains(x.CheckNames, name) || contains(x.Dimensions, dim) {
					return true
				}
			}
			return false
		}
		for _, name := range pol.RequiredChecks {
			var current *checkruns.Run
			for i := range runs {
				r := &runs[i]
				if r.CommitID == revision && r.Definition.Name == name && r.Definition.Privacy != nil {
					current = r
				}
			}
			if current == nil || current.State != checkruns.Succeeded {
				blocking := !exempt(name, "")
				detail := "current synthetic privacy check is missing or failed"
				runID := ""
				if current != nil {
					runID = current.ID
				}
				a.Requirements = append(a.Requirements, Requirement{PolicyID: pol.ID, Kind: "check", Name: name, Detail: detail, Blocking: blocking, RunID: runID})
				a.Ready = a.Ready && !blocking
			} else {
				for _, d := range current.Definition.Privacy.Dimensions {
					if !contains(a.Coverage, d) {
						a.Coverage = append(a.Coverage, d)
					}
				}
			}
		}
		for _, d := range pol.RequiredDimensions {
			if !contains(a.Coverage, d) {
				blocking := !exempt("", d)
				a.Requirements = append(a.Requirements, Requirement{PolicyID: pol.ID, Kind: "coverage", Name: d, Detail: "current passing evidence does not cover this privacy behavior", Blocking: blocking})
				a.Ready = a.Ready && !blocking
			}
		}
		var ack *Acknowledgement
		for i := range l.Acknowledgements {
			x := &l.Acknowledgements[i]
			if x.PolicyID == pol.ID && x.PullRequestID == pull && x.Revision == revision {
				ack = x
			}
		}
		if ack != nil {
			a.Acknowledgements = append(a.Acknowledgements, *ack)
		}
		if ack == nil || ack.Decision != "accept" {
			a.Requirements = append(a.Requirements, Requirement{PolicyID: pol.ID, Kind: "owner_acknowledgement", Detail: "a privacy owner must acknowledge the current preview evidence", Blocking: true})
			a.Ready = false
		}
	}
	sort.Strings(a.Coverage)
	return a, nil
}
