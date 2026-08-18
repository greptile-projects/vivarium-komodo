// Package runtimerepairs retains the governed path from a reproduced runtime
// diagnosis to revision-exact delivery and production validation.
package runtimerepairs

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
)

var ErrNotFound = errors.New("runtime repair not found")
var ErrInvalid = errors.New("invalid runtime repair")

type CreateInput struct {
	WorkspaceID        string   `json:"workspace_id"`
	ReplayID           string   `json:"replay_id"`
	InvestigationID    string   `json:"investigation_id"`
	CauseClaimID       string   `json:"cause_claim_id"`
	Title              string   `json:"title"`
	OwnerKind          string   `json:"owner_kind"`
	OwnerID            string   `json:"owner_id"`
	AffectedRevision   string   `json:"affected_revision"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	RegressionCriteria []string `json:"regression_criteria"`
}
type VerificationInput struct {
	PullRequestID       string   `json:"pull_request_id"`
	Revision            string   `json:"revision"`
	ReplayAttemptID     string   `json:"replay_attempt_id"`
	RequiredCheckRunIDs []string `json:"required_check_run_ids"`
}
type Verification struct {
	ID                  string    `json:"id"`
	PullRequestID       string    `json:"pull_request_id"`
	Revision            string    `json:"revision"`
	ReplayAttemptID     string    `json:"replay_attempt_id"`
	RequiredCheckRunIDs []string  `json:"required_check_run_ids"`
	Passed              bool      `json:"passed"`
	ActorID             string    `json:"actor_id"`
	CreatedAt           time.Time `json:"created_at"`
}
type Signal struct {
	Name             string `json:"name"`
	EvidenceID       string `json:"evidence_id"`
	OriginalBehavior string `json:"original_behavior"`
	ObservedValue    string `json:"observed_value"`
	Passed           bool   `json:"passed"`
}
type ValidationInput struct {
	DeploymentID  string   `json:"deployment_id"`
	ReleaseID     string   `json:"release_id"`
	Revision      string   `json:"revision"`
	Stage         string   `json:"stage"`
	Signals       []Signal `json:"signals"`
	FailureAction string   `json:"failure_action,omitempty"`
	Rationale     string   `json:"rationale"`
}
type Validation struct {
	ID             string    `json:"id"`
	DeploymentID   string    `json:"deployment_id"`
	ReleaseID      string    `json:"release_id"`
	Revision       string    `json:"revision"`
	Stage          string    `json:"stage"`
	Signals        []Signal  `json:"signals"`
	State          string    `json:"state"`
	RequiredAction string    `json:"required_action"`
	Rationale      string    `json:"rationale"`
	ActorID        string    `json:"actor_id"`
	CreatedAt      time.Time `json:"created_at"`
}
type Repair struct {
	ID                 string         `json:"id"`
	RepositoryID       string         `json:"repository_id"`
	ProposalID         string         `json:"proposal_id"`
	TaskID             string         `json:"task_id"`
	WorkspaceID        string         `json:"workspace_id"`
	ReplayID           string         `json:"replay_id"`
	InvestigationID    string         `json:"investigation_id"`
	CauseClaimID       string         `json:"cause_claim_id"`
	Title              string         `json:"title"`
	OwnerKind          string         `json:"owner_kind"`
	OwnerID            string         `json:"owner_id"`
	AffectedRevision   string         `json:"affected_revision"`
	AcceptanceCriteria []string       `json:"acceptance_criteria"`
	RegressionCriteria []string       `json:"regression_criteria"`
	Verifications      []Verification `json:"verifications"`
	Validations        []Validation   `json:"validations"`
	State              string         `json:"state"`
	Authority          []string       `json:"authority"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
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
	if e := os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	return &Store{root: root, now: time.Now}, nil
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func validStrings(v []string) bool {
	if len(v) == 0 || len(v) > 30 {
		return false
	}
	for _, x := range v {
		if strings.TrimSpace(x) == "" || len(x) > 2000 {
			return false
		}
	}
	return true
}
func (s *Store) Create(repo, actor, proposal, task string, in CreateInput) (Repair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || actor == "" || proposal == "" || task == "" || in.WorkspaceID == "" || in.ReplayID == "" || in.InvestigationID == "" || in.CauseClaimID == "" || in.Title == "" || in.OwnerID == "" || !map[string]bool{"human": true, "agent": true}[in.OwnerKind] || in.AffectedRevision == "" || !validStrings(in.AcceptanceCriteria) || !validStrings(in.RegressionCriteria) {
		return Repair{}, ErrInvalid
	}
	now := s.now().UTC()
	v := Repair{ID: id(), RepositoryID: repo, ProposalID: proposal, TaskID: task, WorkspaceID: in.WorkspaceID, ReplayID: in.ReplayID, InvestigationID: in.InvestigationID, CauseClaimID: in.CauseClaimID, Title: strings.TrimSpace(in.Title), OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, AffectedRevision: in.AffectedRevision, AcceptanceCriteria: in.AcceptanceCriteria, RegressionCriteria: in.RegressionCriteria, State: "planned", Authority: []string{}, CreatedAt: now, UpdatedAt: now}
	return v, s.write(v)
}
func (s *Store) Get(repo, x string) (Repair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Repair{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) List(repo string) ([]Repair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Repair{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		v, x := s.read(strings.TrimSuffix(f.Name(), ".json"))
		if x == nil && v.RepositoryID == repo {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Verify(repo, x, actor string, in VerificationInput, passed bool) (Repair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Repair{}, ErrNotFound
	}
	if actor == "" || in.PullRequestID == "" || in.Revision == "" || in.ReplayAttemptID == "" || len(in.RequiredCheckRunIDs) == 0 {
		return Repair{}, ErrInvalid
	}
	now := s.now().UTC()
	v.Verifications = append(v.Verifications, Verification{ID: id(), PullRequestID: in.PullRequestID, Revision: in.Revision, ReplayAttemptID: in.ReplayAttemptID, RequiredCheckRunIDs: in.RequiredCheckRunIDs, Passed: passed, ActorID: actor, CreatedAt: now})
	if passed {
		v.State = "verified_for_review"
	} else {
		v.State = "verification_failed"
	}
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) Validate(repo, x, actor string, in ValidationInput) (Repair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Repair{}, ErrNotFound
	}
	if actor == "" || in.DeploymentID == "" || in.ReleaseID == "" || in.Revision == "" || in.Stage == "" || len(in.Signals) == 0 {
		return Repair{}, ErrInvalid
	}
	all := true
	for _, q := range in.Signals {
		if q.Name == "" || q.EvidenceID == "" || q.OriginalBehavior == "" || q.ObservedValue == "" {
			return Repair{}, ErrInvalid
		}
		all = all && q.Passed
	}
	state, action := "passed", "continue"
	if !all {
		state = "failed"
		action = in.FailureAction
		if !map[string]bool{"pause": true, "restore_known_good": true, "reopen_diagnosis": true}[action] {
			return Repair{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	v.Validations = append(v.Validations, Validation{ID: id(), DeploymentID: in.DeploymentID, ReleaseID: in.ReleaseID, Revision: in.Revision, Stage: in.Stage, Signals: in.Signals, State: state, RequiredAction: action, Rationale: in.Rationale, ActorID: actor, CreatedAt: now})
	if all {
		v.State = "production_validated"
	} else {
		v.State = action
	}
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) read(x string) (Repair, error) {
	b, e := os.ReadFile(filepath.Join(s.root, x+".json"))
	if e != nil {
		return Repair{}, e
	}
	var v Repair
	if json.Unmarshal(b, &v) != nil || v.ID != x {
		return Repair{}, ErrInvalid
	}
	return v, nil
}
func (s *Store) write(v Repair) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(s.root, ".repair-")
	if e != nil {
		return e
	}
	n := f.Name()
	defer os.Remove(n)
	_ = f.Chmod(0600)
	if _, e = f.Write(append(b, '\n')); e == nil {
		e = f.Close()
	} else {
		_ = f.Close()
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(s.root, v.ID+".json"))
	}
	return e
}
