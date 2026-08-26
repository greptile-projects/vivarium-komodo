// Package signalevaluations retains reproducible use and governed retirement of delivered signals.
package signalevaluations

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

var ErrNotFound = errors.New("signal evaluation not found")
var ErrInvalid = errors.New("invalid signal evaluation")
var ErrConflict = errors.New("signal evaluation changed")
var ErrForbidden = errors.New("signal evaluation action forbidden")

type Signal struct {
	ID              string `json:"id"`
	ContractID      string `json:"contract_id"`
	ContractVersion int64  `json:"contract_version"`
	RolloutID       string `json:"rollout_id"`
	Revision        string `json:"revision"`
	Kind            string `json:"kind"`
}
type Query struct {
	ID                  string            `json:"id"`
	Expression          string            `json:"expression"`
	WindowStart         time.Time         `json:"window_start"`
	WindowEnd           time.Time         `json:"window_end"`
	SignalIDs           []string          `json:"signal_ids"`
	ReleaseIDs          []string          `json:"release_ids"`
	DeploymentIDs       []string          `json:"deployment_ids"`
	CodeRevisions       []string          `json:"code_revisions"`
	DependencyRevisions []string          `json:"dependency_revisions"`
	JourneyIDs          []string          `json:"journey_ids"`
	Parameters          map[string]string `json:"parameters"`
	ResultDigest        string            `json:"result_digest"`
}
type Citation struct {
	ID         string `json:"id"`
	QueryID    string `json:"query_id"`
	Source     string `json:"source"`
	Revision   string `json:"revision"`
	Digest     string `json:"digest"`
	Accessible bool   `json:"accessible"`
}
type Input struct {
	GapVersion int64      `json:"gap_version"`
	Title      string     `json:"title"`
	SignalIDs  []string   `json:"signal_ids"`
	Signals    []Signal   `json:"signals"`
	Queries    []Query    `json:"queries"`
	Citations  []Citation `json:"citations"`
}
type FindingInput struct {
	ExpectedRevision int64             `json:"expected_revision"`
	Kind             string            `json:"kind"`
	Statement        string            `json:"statement"`
	CitationIDs      []string          `json:"citation_ids"`
	Uncertainty      string            `json:"uncertainty"`
	Reproduction     string            `json:"reproduction"`
	Criteria         map[string]string `json:"criteria"`
}
type Finding struct {
	ID string `json:"id"`
	FindingInput
	ActorID   string    `json:"actor_id"`
	ActorKind string    `json:"actor_kind"`
	CreatedAt time.Time `json:"created_at"`
}
type ResolutionInput struct {
	ExpectedRevision int64  `json:"expected_revision"`
	FindingID        string `json:"finding_id"`
	Disposition      string `json:"disposition"`
	TargetKind       string `json:"target_kind"`
	TargetID         string `json:"target_id"`
	TargetRevision   string `json:"target_revision"`
	RepairKind       string `json:"repair_kind,omitempty"`
	RepairID         string `json:"repair_id,omitempty"`
	Rationale        string `json:"rationale"`
}
type Resolution struct {
	ID string `json:"id"`
	ResolutionInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Consumer struct {
	Kind         string `json:"kind"`
	ID           string `json:"id"`
	Revision     string `json:"revision"`
	OwnerID      string `json:"owner_id"`
	Impact       string `json:"impact"`
	Acknowledged bool   `json:"acknowledged"`
}
type LifecycleInput struct {
	ExpectedRevision     int64      `json:"expected_revision"`
	Action               string     `json:"action"`
	SignalIDs            []string   `json:"signal_ids"`
	Rationale            string     `json:"rationale"`
	PolicyID             string     `json:"policy_id"`
	PolicyRevision       string     `json:"policy_revision"`
	ApprovedByID         string     `json:"approved_by_id"`
	Consumers            []Consumer `json:"consumers"`
	HistoricalMeaning    string     `json:"historical_meaning"`
	ProvenanceRefs       []string   `json:"provenance_refs"`
	ReplacementSignalIDs []string   `json:"replacement_signal_ids"`
	StopEvidenceIDs      []string   `json:"stop_evidence_ids"`
	CollectionStoppedAt  *time.Time `json:"collection_stopped_at,omitempty"`
}
type Lifecycle struct {
	ID string `json:"id"`
	LifecycleInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
	Applied   bool      `json:"applied"`
	Blockers  []string  `json:"blockers"`
}
type Evaluation struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	GapID        string `json:"gap_id"`
	Revision     int64  `json:"revision"`
	Input
	CreatedByID        string            `json:"created_by_id"`
	CreatedAt          time.Time         `json:"created_at"`
	Findings           []Finding         `json:"findings"`
	Resolutions        []Resolution      `json:"resolutions"`
	Lifecycles         []Lifecycle       `json:"lifecycles"`
	CriteriaStatus     map[string]string `json:"criteria_status"`
	CurrentSignalState map[string]string `json:"current_signal_state"`
	Blockers           []string          `json:"blockers"`
	NonAuthority       []string          `json:"non_authority"`
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
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, now: time.Now}, e
}
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func validInput(in Input) bool {
	if in.GapVersion < 1 || in.Title == "" || len(in.SignalIDs) == 0 || len(in.Signals) == 0 || len(in.Queries) == 0 || len(in.Citations) == 0 {
		return false
	}
	ss := map[string]bool{}
	for _, s := range in.Signals {
		if s.ID == "" || s.ContractID == "" || s.ContractVersion < 1 || s.RolloutID == "" || s.Revision == "" {
			return false
		}
		ss[s.ID] = true
	}
	qs := map[string]bool{}
	for _, q := range in.Queries {
		if q.ID == "" || q.Expression == "" || !q.WindowEnd.After(q.WindowStart) || q.ResultDigest == "" || len(q.SignalIDs) == 0 {
			return false
		}
		for _, x := range q.SignalIDs {
			if !ss[x] {
				return false
			}
		}
		qs[q.ID] = true
	}
	for _, c := range in.Citations {
		if c.ID == "" || !qs[c.QueryID] || c.Source == "" || c.Revision == "" || c.Digest == "" {
			return false
		}
	}
	return true
}
func (s *Store) Create(repo, gap, actor string, in Input) (Evaluation, error) {
	if repo == "" || gap == "" || actor == "" || !validInput(in) {
		return Evaluation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := derive(Evaluation{ID: id(), RepositoryID: repo, GapID: gap, Revision: 1, Input: in, CreatedByID: actor, CreatedAt: s.now().UTC(), CriteriaStatus: map[string]string{}, CurrentSignalState: map[string]string{}})
	return e, s.write(e)
}
func (s *Store) mutate(repo, gap, eid string, expected int64, fn func(*Evaluation) error) (Evaluation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, x := s.read(repo, eid)
	if x != nil {
		return e, x
	}
	if e.GapID != gap {
		return Evaluation{}, ErrNotFound
	}
	if e.Revision != expected {
		return Evaluation{}, ErrConflict
	}
	if x = fn(&e); x != nil {
		return Evaluation{}, x
	}
	e.Revision++
	e = derive(e)
	return e, s.write(e)
}
func (s *Store) AddFinding(repo, gap, eid, actor, kind string, in FindingInput) (Evaluation, error) {
	if actor == "" || !map[string]bool{"human": true, "read_only_agent": true}[kind] || !map[string]bool{"supported": true, "misleading": true, "insufficient": true}[in.Kind] || in.Statement == "" || len(in.CitationIDs) == 0 || in.Uncertainty == "" || in.Reproduction == "" || len(in.Criteria) == 0 {
		return Evaluation{}, ErrInvalid
	}
	return s.mutate(repo, gap, eid, in.ExpectedRevision, func(e *Evaluation) error {
		known := map[string]bool{}
		for _, c := range e.Citations {
			known[c.ID] = c.Accessible
		}
		for _, c := range in.CitationIDs {
			if !known[c] {
				return ErrInvalid
			}
		}
		e.Findings = append(e.Findings, Finding{ID: id(), FindingInput: in, ActorID: actor, ActorKind: kind, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Resolve(repo, gap, eid, actor string, in ResolutionInput) (Evaluation, error) {
	validTarget := map[string]bool{"service_objective": true, "response_alert": true, "runbook": true, "investigation": true, "quality_check": true, "decision": true}
	if actor == "" || in.FindingID == "" || !map[string]bool{"accepted": true, "repair_required": true, "rejected": true}[in.Disposition] || in.Rationale == "" {
		return Evaluation{}, ErrInvalid
	}
	if in.Disposition == "accepted" && (!validTarget[in.TargetKind] || in.TargetID == "" || in.TargetRevision == "") {
		return Evaluation{}, ErrInvalid
	}
	if in.Disposition == "repair_required" && (in.RepairKind == "" || in.RepairID == "") {
		return Evaluation{}, ErrInvalid
	}
	return s.mutate(repo, gap, eid, in.ExpectedRevision, func(e *Evaluation) error {
		for _, f := range e.Findings {
			if f.ID == in.FindingID {
				e.Resolutions = append(e.Resolutions, Resolution{ID: id(), ResolutionInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
				return nil
			}
		}
		return ErrInvalid
	})
}
func (s *Store) Lifecycle(repo, gap, eid, actor string, in LifecycleInput) (Evaluation, error) {
	if actor == "" || !map[string]bool{"retain": true, "revise": true, "reduce": true, "archive": true, "remove": true}[in.Action] || len(in.SignalIDs) == 0 || in.Rationale == "" || in.PolicyID == "" || in.PolicyRevision == "" || in.ApprovedByID == "" || in.ApprovedByID == actor || in.HistoricalMeaning == "" || len(in.ProvenanceRefs) == 0 {
		return Evaluation{}, ErrInvalid
	}
	return s.mutate(repo, gap, eid, in.ExpectedRevision, func(e *Evaluation) error {
		owners := map[string]bool{}
		for _, x := range e.Signals {
			owners[x.ID] = true
		}
		for _, x := range in.SignalIDs {
			if !owners[x] {
				return ErrInvalid
			}
		}
		l := Lifecycle{ID: id(), LifecycleInput: in, ActorID: actor, CreatedAt: s.now().UTC()}
		for _, c := range in.Consumers {
			if c.ID == "" || c.Revision == "" || c.OwnerID == "" || c.Impact == "" || !c.Acknowledged {
				l.Blockers = append(l.Blockers, "dependent consumer "+c.ID+" has not acknowledged the impact")
			}
		}
		if (in.Action == "archive" || in.Action == "remove") && (in.CollectionStoppedAt == nil || len(in.StopEvidenceIDs) == 0) {
			l.Blockers = append(l.Blockers, "obsolete collection has not been verified as stopped")
		}
		l.Applied = len(l.Blockers) == 0
		e.Lifecycles = append(e.Lifecycles, l)
		return nil
	})
}
func derive(e Evaluation) Evaluation {
	e.Blockers = nil
	e.CriteriaStatus = map[string]string{}
	e.CurrentSignalState = map[string]string{}
	for _, x := range e.SignalIDs {
		e.CurrentSignalState[x] = "active"
	}
	for _, f := range e.Findings {
		for k, v := range f.Criteria {
			e.CriteriaStatus[k] = v
		}
	}
	for _, r := range e.Resolutions {
		if r.Disposition == "repair_required" {
			e.Blockers = append(e.Blockers, "finding "+r.FindingID+" requires connected repair "+r.RepairID)
		}
	}
	for _, l := range e.Lifecycles {
		if !l.Applied {
			e.Blockers = append(e.Blockers, l.Blockers...)
			continue
		}
		for _, x := range l.SignalIDs {
			e.CurrentSignalState[x] = l.Action
		}
	}
	e.NonAuthority = []string{"Signal evaluations and lifecycle records grant no repository, telemetry, collector, data, policy, deployment, environment, spending, or operational authority."}
	return e
}
func (s *Store) Get(repo, eid string) (Evaluation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, x := s.read(repo, eid)
	return derive(e), x
}
func (s *Store) List(repo, gap string) ([]Evaluation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fsx, x := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if x != nil {
		return nil, x
	}
	sort.Strings(fsx)
	out := []Evaluation{}
	for _, f := range fsx {
		b, x := os.ReadFile(f)
		var e Evaluation
		if x == nil {
			x = json.Unmarshal(b, &e)
		}
		if x != nil {
			return nil, x
		}
		if e.GapID == gap {
			out = append(out, derive(e))
		}
	}
	return out, nil
}
func (s *Store) read(repo, eid string) (Evaluation, error) {
	b, x := os.ReadFile(filepath.Join(s.root, repo, eid+".json"))
	if errors.Is(x, fs.ErrNotExist) {
		return Evaluation{}, ErrNotFound
	}
	var e Evaluation
	if x == nil {
		x = json.Unmarshal(b, &e)
	}
	return e, x
}
func (s *Store) write(e Evaluation) error {
	d := filepath.Join(s.root, e.RepositoryID)
	if x := os.MkdirAll(d, 0750); x != nil {
		return x
	}
	b, x := json.MarshalIndent(e, "", "  ")
	if x != nil {
		return x
	}
	f, x := os.CreateTemp(d, "evaluation-*.tmp")
	if x != nil {
		return x
	}
	n := f.Name()
	defer os.Remove(n)
	if _, x = f.Write(b); x == nil {
		x = f.Sync()
	}
	if z := f.Close(); x == nil {
		x = z
	}
	if x == nil {
		x = os.Rename(n, filepath.Join(d, e.ID+".json"))
	}
	return x
}
