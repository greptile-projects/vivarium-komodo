// Package accessibilityassessments retains revision-exact automated and human accessibility evidence.
package accessibilityassessments

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

var ErrNotFound = errors.New("accessibility assessment not found")
var ErrInvalid = errors.New("invalid accessibility assessment")

type Location struct {
	Path      string `json:"path"`
	BlobID    string `json:"blob_id"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}
type Scenario struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Journey     string     `json:"journey"`
	Audiences   []string   `json:"affected_audiences"`
	Evaluations []string   `json:"required_evaluations"`
	Locations   []Location `json:"source_locations"`
	Digest      string     `json:"digest"`
}
type Input struct {
	Revision          string     `json:"revision"`
	CommitmentID      string     `json:"commitment_id,omitempty"`
	CommitmentVersion int64      `json:"commitment_version,omitempty"`
	Scenarios         []Scenario `json:"scenarios"`
}
type Citation struct {
	Kind        string   `json:"kind"`
	ResourceID  string   `json:"resource_id"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}
type FindingInput struct {
	ScenarioID              string     `json:"scenario_id"`
	Evaluation              string     `json:"evaluation"`
	Result                  string     `json:"result"`
	Severity                string     `json:"severity"`
	Audiences               []string   `json:"affected_audiences"`
	Locations               []Location `json:"source_locations"`
	Summary                 string     `json:"summary"`
	Uncertainty             string     `json:"uncertainty,omitempty"`
	RequiresHumanEvaluation bool       `json:"requires_human_evaluation"`
	Citation                Citation   `json:"citation"`
}
type DecisionInput struct {
	Outcome     string `json:"outcome"`
	Rationale   string `json:"rationale"`
	DuplicateOf string `json:"duplicate_of,omitempty"`
}
type Decision struct {
	DecisionInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type RepairProgress struct {
	Status    string    `json:"status"`
	Summary   string    `json:"summary"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type RepairDelivery struct {
	PullRequestID        string    `json:"pull_request_id"`
	Revision             string    `json:"revision"`
	PreviewID            string    `json:"preview_id"`
	DesignChanges        []string  `json:"design_changes"`
	CodeChanges          []string  `json:"code_changes"`
	InteractionTradeoffs []string  `json:"interaction_tradeoffs"`
	ContentTradeoffs     []string  `json:"content_tradeoffs"`
	LinkedByID           string    `json:"linked_by_id"`
	LinkedAt             time.Time `json:"linked_at"`
}
type Repair struct {
	ID                 string           `json:"id"`
	Revision           string           `json:"revision"`
	AcceptanceCriteria []string         `json:"acceptance_criteria"`
	EvidenceIDs        []string         `json:"evidence_ids"`
	CommitmentID       string           `json:"commitment_id"`
	CommitmentVersion  int64            `json:"commitment_version"`
	ComponentGuidance  []string         `json:"component_guidance"`
	OwnerKind          string           `json:"owner_kind"`
	OwnerID            string           `json:"owner_id"`
	ProposalID         string           `json:"proposal_id"`
	TaskID             string           `json:"task_id"`
	ChangeSessionID    string           `json:"change_session_id,omitempty"`
	WorkspaceID        string           `json:"workspace_id,omitempty"`
	CreatedByID        string           `json:"created_by_id"`
	CreatedAt          time.Time        `json:"created_at"`
	Progress           []RepairProgress `json:"progress"`
	Delivery           *RepairDelivery  `json:"delivery,omitempty"`
}
type Finding struct {
	ID string `json:"id"`
	FindingInput
	ActorID      string     `json:"actor_id"`
	CreatedAt    time.Time  `json:"created_at"`
	Decisions    []Decision `json:"decisions"`
	Repair       *Repair    `json:"repair,omitempty"`
	Stale        bool       `json:"stale"`
	StaleReasons []string   `json:"stale_reasons"`
}
type Automation struct {
	RunID                   string     `json:"run_id"`
	Name                    string     `json:"name"`
	ScenarioIDs             []string   `json:"scenario_ids"`
	Evaluations             []string   `json:"evaluations"`
	Status                  string     `json:"status"`
	RequiresHumanEvaluation []string   `json:"requires_human_evaluation"`
	Inputs                  []Location `json:"inputs"`
	ActorID                 string     `json:"actor_id"`
	CreatedAt               time.Time  `json:"created_at"`
	Stale                   bool       `json:"stale"`
}
type Gap struct {
	ScenarioID string `json:"scenario_id"`
	Evaluation string `json:"evaluation"`
	Kind       string `json:"kind"`
}
type Assessment struct {
	ID            string `json:"id"`
	RepositoryID  string `json:"repository_id"`
	PullRequestID string `json:"pull_request_id"`
	Input
	CreatedByID string       `json:"created_by_id"`
	CreatedAt   time.Time    `json:"created_at"`
	Automation  []Automation `json:"automation"`
	Findings    []Finding    `json:"findings"`
	Gaps        []Gap        `json:"gaps"`
	Stale       bool         `json:"stale"`
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
func id() string                { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func text(v string, n int) bool { return strings.TrimSpace(v) != "" && len(v) <= n }
func list(v []string) bool {
	if len(v) == 0 || len(v) > 50 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range v {
		if !text(x, 200) || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}

var evaluations = map[string]bool{"semantics": true, "keyboard": true, "focus": true, "contrast": true, "motion": true, "captions": true, "journey": true}

func validLocation(x Location) bool {
	return text(x.Path, 500) && x.BlobID != "" && x.StartLine >= 0 && x.EndLine >= x.StartLine
}
func validInput(in Input) bool {
	if in.Revision == "" || len(in.Scenarios) == 0 || len(in.Scenarios) > 100 || (in.CommitmentID == "") != (in.CommitmentVersion == 0) {
		return false
	}
	seen := map[string]bool{}
	for _, s := range in.Scenarios {
		if !text(s.ID, 100) || seen[s.ID] || !text(s.Name, 500) || !text(s.Journey, 4000) || !list(s.Audiences) || !list(s.Evaluations) || len(s.Locations) == 0 || s.Digest == "" {
			return false
		}
		seen[s.ID] = true
		for _, e := range s.Evaluations {
			if !evaluations[e] {
				return false
			}
		}
		for _, x := range s.Locations {
			if !validLocation(x) {
				return false
			}
		}
	}
	return true
}
func validFinding(a Assessment, in FindingInput) bool {
	found := false
	for _, s := range a.Scenarios {
		if s.ID == in.ScenarioID {
			found = true
		}
	}
	if !found || !evaluations[in.Evaluation] || !map[string]bool{"passed": true, "barrier": true, "not_evaluated": true}[in.Result] || !map[string]bool{"critical": true, "high": true, "medium": true, "low": true, "none": true}[in.Severity] || !list(in.Audiences) || !text(in.Summary, 65536) || len(in.Uncertainty) > 65536 || len(in.Locations) == 0 || !map[string]bool{"preview": true, "reproduction": true}[in.Citation.Kind] || in.Citation.ResourceID == "" {
		return false
	}
	for _, x := range in.Locations {
		if !validLocation(x) {
			return false
		}
	}
	return true
}
func (s *Store) path(repo, pull, aid string) string {
	return filepath.Join(s.root, repo, pull, aid+".json")
}
func (s *Store) write(a Assessment) error {
	d := filepath.Dir(s.path(a.RepositoryID, a.PullRequestID, a.ID))
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(a, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(s.path(a.RepositoryID, a.PullRequestID, a.ID), b, 0640)
}
func (s *Store) read(repo, pull, aid string) (Assessment, error) {
	var a Assessment
	b, e := os.ReadFile(s.path(repo, pull, aid))
	if errors.Is(e, fs.ErrNotExist) {
		return a, ErrNotFound
	}
	if e != nil || json.Unmarshal(b, &a) != nil || a.ID != aid || a.RepositoryID != repo || a.PullRequestID != pull {
		return Assessment{}, ErrNotFound
	}
	return a, nil
}
func (s *Store) Create(repo, pull, actor string, in Input) (Assessment, error) {
	if repo == "" || pull == "" || actor == "" || !validInput(in) {
		return Assessment{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a := Assessment{ID: id(), RepositoryID: repo, PullRequestID: pull, Input: in, CreatedByID: actor, CreatedAt: s.now().UTC(), Automation: []Automation{}, Findings: []Finding{}}
	return a, s.write(a)
}
func (s *Store) AddFinding(repo, pull, aid, actor string, in FindingInput) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pull, aid)
	if e != nil {
		return a, e
	}
	if actor == "" || !validFinding(a, in) {
		return a, ErrInvalid
	}
	a.Findings = append(a.Findings, Finding{ID: id(), FindingInput: in, ActorID: actor, CreatedAt: s.now().UTC(), Decisions: []Decision{}})
	return a, s.write(a)
}
func (s *Store) Decide(repo, pull, aid, fid, actor string, in DecisionInput) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pull, aid)
	if e != nil {
		return a, e
	}
	if actor == "" || !map[string]bool{"confirmed": true, "false_positive": true, "duplicate": true}[in.Outcome] || !text(in.Rationale, 65536) || (in.Outcome == "duplicate") != (in.DuplicateOf != "") {
		return a, ErrInvalid
	}
	found := false
	for i := range a.Findings {
		if a.Findings[i].ID == fid {
			if in.DuplicateOf == fid {
				return a, ErrInvalid
			}
			a.Findings[i].Decisions = append(a.Findings[i].Decisions, Decision{DecisionInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
			found = true
		}
	}
	if !found {
		return a, ErrNotFound
	}
	return a, s.write(a)
}
func (s *Store) AddAutomation(repo, pull, aid, actor string, in Automation) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pull, aid)
	if e != nil {
		return a, e
	}
	if actor == "" || in.RunID == "" || in.Name == "" || !list(in.ScenarioIDs) || !list(in.Evaluations) || !map[string]bool{"queued": true, "running": true, "succeeded": true, "failed": true, "canceled": true}[in.Status] {
		return a, ErrInvalid
	}
	in.ActorID = actor
	in.CreatedAt = s.now().UTC()
	a.Automation = append(a.Automation, in)
	return a, s.write(a)
}
func (s *Store) CreateRepair(repo, pull, aid, fid, actor string, repair Repair) (Assessment, Repair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pull, aid)
	if e != nil {
		return a, repair, e
	}
	if actor == "" || repair.Revision != a.Revision || repair.ProposalID == "" || repair.TaskID == "" || repair.CommitmentID == "" || repair.CommitmentVersion < 1 || repair.OwnerID == "" || !map[string]bool{"human": true, "agent": true}[repair.OwnerKind] || !list(repair.AcceptanceCriteria) || len(repair.ComponentGuidance) == 0 || len(repair.ComponentGuidance) > 50 {
		return a, repair, ErrInvalid
	}
	for _, x := range repair.ComponentGuidance {
		if !text(x, 4000) {
			return a, repair, ErrInvalid
		}
	}
	for i := range a.Findings {
		f := &a.Findings[i]
		if f.ID != fid {
			continue
		}
		confirmed := len(f.Decisions) > 0 && f.Decisions[len(f.Decisions)-1].Outcome == "confirmed"
		if !confirmed || f.Stale || f.Repair != nil {
			return a, repair, ErrInvalid
		}
		now := s.now().UTC()
		repair.ID, repair.CreatedByID, repair.CreatedAt = id(), actor, now
		repair.Progress = []RepairProgress{{Status: "planned", Summary: "Governed repair work created from the confirmed finding.", ActorID: actor, CreatedAt: now}}
		f.Repair = &repair
		return a, repair, s.write(a)
	}
	return a, repair, ErrNotFound
}
func (s *Store) AddRepairProgress(repo, pull, aid, fid, rid, actor, status, summary string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pull, aid)
	if e != nil {
		return a, e
	}
	if actor == "" || !map[string]bool{"planned": true, "in_progress": true, "blocked": true, "ready_for_review": true, "completed": true, "canceled": true}[status] || !text(summary, 65536) {
		return a, ErrInvalid
	}
	for i := range a.Findings {
		f := &a.Findings[i]
		if f.ID == fid && f.Repair != nil && f.Repair.ID == rid {
			f.Repair.Progress = append(f.Repair.Progress, RepairProgress{Status: status, Summary: summary, ActorID: actor, CreatedAt: s.now().UTC()})
			return a, s.write(a)
		}
	}
	return a, ErrNotFound
}
func (s *Store) LinkRepairDelivery(repo, pull, aid, fid, rid, actor string, delivery RepairDelivery) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pull, aid)
	if e != nil {
		return a, e
	}
	if actor == "" || delivery.PullRequestID == "" || delivery.Revision == "" || delivery.PreviewID == "" || !list(delivery.DesignChanges) || !list(delivery.CodeChanges) || !list(delivery.InteractionTradeoffs) || !list(delivery.ContentTradeoffs) {
		return a, ErrInvalid
	}
	for i := range a.Findings {
		f := &a.Findings[i]
		if f.ID == fid && f.Repair != nil && f.Repair.ID == rid {
			if f.Repair.Delivery != nil {
				return a, ErrInvalid
			}
			delivery.LinkedByID, delivery.LinkedAt = actor, s.now().UTC()
			f.Repair.Delivery = &delivery
			f.Repair.Progress = append(f.Repair.Progress, RepairProgress{Status: "ready_for_review", Summary: "Revision-exact pull request and preview linked for inspection.", ActorID: actor, CreatedAt: delivery.LinkedAt})
			return a, s.write(a)
		}
	}
	return a, ErrNotFound
}
func derive(a *Assessment) {
	covered := map[string]bool{}
	for _, x := range a.Automation {
		if !x.Stale && x.Status == "succeeded" {
			for _, sid := range x.ScenarioIDs {
				for _, e := range x.Evaluations {
					covered[sid+"\x00"+e] = true
				}
			}
		}
	}
	for _, x := range a.Findings {
		if !x.Stale && x.Result != "not_evaluated" {
			covered[x.ScenarioID+"\x00"+x.Evaluation] = true
		}
	}
	a.Gaps = nil
	for _, sc := range a.Scenarios {
		for _, e := range sc.Evaluations {
			if !covered[sc.ID+"\x00"+e] {
				a.Gaps = append(a.Gaps, Gap{ScenarioID: sc.ID, Evaluation: e, Kind: "unevaluated"})
			}
		}
	}
}
func (s *Store) Get(repo, pull, aid string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pull, aid)
	derive(&a)
	return a, e
}
func (s *Store) List(repo, pull string) ([]Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo, pull))
	if errors.Is(e, fs.ErrNotExist) {
		return []Assessment{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Assessment{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			a, er := s.read(repo, pull, strings.TrimSuffix(x.Name(), ".json"))
			if er != nil {
				return nil, er
			}
			derive(&a)
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) ListRepository(repo string) ([]Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pulls, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Assessment{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Assessment{}
	for _, p := range pulls {
		if !p.IsDir() {
			continue
		}
		es, er := os.ReadDir(filepath.Join(s.root, repo, p.Name()))
		if er != nil {
			return nil, er
		}
		for _, x := range es {
			if filepath.Ext(x.Name()) == ".json" {
				a, re := s.read(repo, p.Name(), strings.TrimSuffix(x.Name(), ".json"))
				if re != nil {
					return nil, re
				}
				derive(&a)
				out = append(out, a)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func Derive(a *Assessment, currentRevision string, currentBlobs map[string]string) {
	a.Stale = a.Revision != currentRevision
	for i := range a.Findings {
		f := &a.Findings[i]
		f.StaleReasons = nil
		for _, x := range f.Locations {
			if currentBlobs[x.Path] != x.BlobID {
				f.StaleReasons = append(f.StaleReasons, "source_changed:"+x.Path)
			}
		}
		f.Stale = len(f.StaleReasons) > 0
	}
	for i := range a.Automation {
		a.Automation[i].Stale = false
		for _, x := range a.Automation[i].Inputs {
			if currentBlobs[x.Path] != x.BlobID {
				a.Automation[i].Stale = true
			}
		}
	}
	derive(a)
}
