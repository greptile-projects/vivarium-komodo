// Package assuranceassessments retains revision-exact compliance impact decisions.
package assuranceassessments

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

	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceprograms"
)

var ErrNotFound = errors.New("assurance assessment not found")
var ErrInvalid = errors.New("invalid assurance assessment")
var ErrConflict = errors.New("assurance assessment conflict")

var candidateKinds = map[string]bool{"pull_request": true, "infrastructure_plan": true, "schema_migration": true, "extension_installation": true, "package_update": true, "release_candidate": true}
var actionKinds = map[string]bool{"test": true, "notice": true, "retention": true, "exception": true, "evidence": true, "mitigation": true}
var annotationKinds = map[string]bool{"challenge": true, "analysis": true, "mitigation": true, "residual_risk": true, "alternative": true}

type BoundInput struct {
	Key      string `json:"key"`
	Revision string `json:"revision"`
}
type Action struct {
	ID                 string   `json:"id"`
	Kind               string   `json:"kind"`
	Description        string   `json:"description"`
	OwnerIDs           []string `json:"owner_ids"`
	EvidencePackageIDs []string `json:"evidence_package_ids,omitempty"`
	Required           bool     `json:"required"`
}
type Impact struct {
	ControlID            string   `json:"control_id"`
	RequirementIDs       []string `json:"requirement_ids"`
	Rationale            string   `json:"rationale"`
	ChangedEvidence      []string `json:"changed_evidence"`
	RequiredOwnerIDs     []string `json:"required_owner_ids"`
	Actions              []Action `json:"actions"`
	InputKeys            []string `json:"input_keys"`
	RequiredForReadiness bool     `json:"required_for_readiness"`
}
type Input struct {
	CandidateKind     string       `json:"candidate_kind"`
	CandidateID       string       `json:"candidate_id"`
	CandidateRevision string       `json:"candidate_revision"`
	ProgramID         string       `json:"program_id"`
	ProgramVersion    int64        `json:"program_version"`
	Summary           string       `json:"summary"`
	Inputs            []BoundInput `json:"inputs"`
	Impacts           []Impact     `json:"impacts"`
}
type Snapshot struct {
	Revision  string       `json:"revision"`
	Inputs    []BoundInput `json:"inputs"`
	ActorID   string       `json:"actor_id"`
	CreatedAt time.Time    `json:"created_at"`
}
type Citation struct {
	Reference string `json:"reference"`
	Revision  string `json:"revision"`
	Audience  string `json:"audience"`
}
type AnnotationInput struct {
	Kind       string     `json:"kind"`
	Body       string     `json:"body"`
	ControlIDs []string   `json:"control_ids"`
	Citations  []Citation `json:"citations"`
}
type Annotation struct {
	ID string `json:"id"`
	AnnotationInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Decision struct {
	ControlID      string            `json:"control_id"`
	OwnerID        string            `json:"owner_id"`
	Decision       string            `json:"decision"`
	Rationale      string            `json:"rationale"`
	InputRevisions map[string]string `json:"input_revisions"`
	CreatedAt      time.Time         `json:"created_at"`
	Stale          bool              `json:"stale"`
}
type Blocker struct {
	Kind      string `json:"kind"`
	ControlID string `json:"control_id,omitempty"`
	ActionID  string `json:"action_id,omitempty"`
	Detail    string `json:"detail"`
}
type Assessment struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Input
	CreatedByID      string       `json:"created_by_id"`
	CreatedAt        time.Time    `json:"created_at"`
	Snapshots        []Snapshot   `json:"snapshots"`
	Annotations      []Annotation `json:"annotations"`
	Decisions        []Decision   `json:"decisions"`
	Blockers         []Blocker    `json:"blockers"`
	Ready            bool         `json:"ready"`
	AuthorityGranted bool         `json:"authority_granted"`
}
type Store struct {
	root     string
	programs interface {
		Get(string, string) (assuranceprograms.Program, error)
	}
	mu  sync.Mutex
	now func() time.Time
}

func New(root string, programs interface {
	Get(string, string) (assuranceprograms.Program, error)
}) (*Store, error) {
	if strings.TrimSpace(root) == "" || programs == nil {
		return nil, ErrInvalid
	}
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, programs: programs, now: time.Now}, e
}
func id() string                { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func text(s string, n int) bool { return strings.TrimSpace(s) != "" && len(s) <= n }
func unique(xs []string, required bool) bool {
	if required && len(xs) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range xs {
		if !text(x, 500) || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func inputMap(xs []BoundInput) (map[string]string, bool) {
	m := map[string]string{}
	for _, x := range xs {
		if !text(x.Key, 500) || !text(x.Revision, 500) || m[x.Key] != "" {
			return nil, false
		}
		m[x.Key] = x.Revision
	}
	return m, len(m) > 0
}
func version(p assuranceprograms.Program, n int64) (assuranceprograms.Version, bool) {
	for _, v := range p.Versions {
		if v.Number == n {
			return v, true
		}
	}
	return assuranceprograms.Version{}, false
}
func valid(in Input, p assuranceprograms.Program) bool {
	if !candidateKinds[in.CandidateKind] || !text(in.CandidateID, 500) || !text(in.CandidateRevision, 500) || in.ProgramID != p.ID || !text(in.Summary, 65536) || len(in.Impacts) == 0 {
		return false
	}
	im, ok := inputMap(in.Inputs)
	if !ok || im["candidate"] != in.CandidateRevision || im["program"] != hexInt(in.ProgramVersion) {
		return false
	}
	v, ok := version(p, in.ProgramVersion)
	if !ok {
		return false
	}
	controls := map[string]assuranceprograms.Control{}
	for _, c := range v.Controls {
		controls[c.ID] = c
	}
	seen := map[string]bool{}
	for _, x := range in.Impacts {
		c, ok := controls[x.ControlID]
		if !ok || seen[x.ControlID] || !text(x.Rationale, 65536) || !unique(x.RequirementIDs, true) || !unique(x.RequiredOwnerIDs, x.RequiredForReadiness) || !unique(x.InputKeys, true) {
			return false
		}
		seen[x.ControlID] = true
		controlOwners := map[string]bool{}
		for _, owner := range c.OwnerIDs {
			controlOwners[owner] = true
		}
		for _, owner := range x.RequiredOwnerIDs {
			if !controlOwners[owner] {
				return false
			}
		}
		for _, k := range x.InputKeys {
			if im[k] == "" {
				return false
			}
		}
		req := map[string]bool{}
		for _, r := range c.RequirementIDs {
			req[r] = true
		}
		for _, r := range x.RequirementIDs {
			if !req[r] {
				return false
			}
		}
		acts := map[string]bool{}
		for _, a := range x.Actions {
			if !text(a.ID, 100) || acts[a.ID] || !actionKinds[a.Kind] || !text(a.Description, 4000) || !unique(a.OwnerIDs, false) || !unique(a.EvidencePackageIDs, false) {
				return false
			}
			acts[a.ID] = true
		}
	}
	return true
}
func hexInt(n int64) string {
	const d = "0123456789"
	if n <= 0 {
		return ""
	}
	out := ""
	for n > 0 {
		out = string(d[n%10]) + out
		n /= 10
	}
	return out
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(a Assessment) error {
	if e := os.MkdirAll(filepath.Dir(s.path(a.RepositoryID, a.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(a, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(a.RepositoryID, a.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) read(repo, aid string) (Assessment, error) {
	var a Assessment
	b, e := os.ReadFile(s.path(repo, aid))
	if errors.Is(e, fs.ErrNotExist) {
		return a, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &a)
	}
	if e != nil || a.ID != aid || a.RepositoryID != repo {
		return Assessment{}, ErrNotFound
	}
	return a, nil
}
func (s *Store) Create(repo, actor string, in Input) (Assessment, error) {
	p, e := s.programs.Get(repo, in.ProgramID)
	if e != nil {
		return Assessment{}, ErrNotFound
	}
	if !text(repo, 500) || !text(actor, 500) || !valid(in, p) {
		return Assessment{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	a := Assessment{ID: id(), RepositoryID: repo, Input: in, CreatedByID: actor, CreatedAt: now, Snapshots: []Snapshot{{Revision: in.CandidateRevision, Inputs: in.Inputs, ActorID: actor, CreatedAt: now}}, Annotations: []Annotation{}, Decisions: []Decision{}, AuthorityGranted: false}
	derive(&a, p)
	return a, s.save(a)
}
func (s *Store) mutate(repo, aid string, fn func(*Assessment) error) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, aid)
	if e == nil {
		e = fn(&a)
	}
	if e == nil {
		p, x := s.programs.Get(repo, a.ProgramID)
		if x != nil {
			return Assessment{}, ErrNotFound
		}
		derive(&a, p)
		e = s.save(a)
	}
	return a, e
}
func (s *Store) Rebind(repo, aid, actor, expectedRevision, newRevision string, inputs []BoundInput) (Assessment, error) {
	return s.mutate(repo, aid, func(a *Assessment) error {
		cur := a.Snapshots[len(a.Snapshots)-1]
		m, ok := inputMap(inputs)
		if !text(actor, 500) || expectedRevision != cur.Revision || !ok || m["candidate"] != newRevision {
			return ErrConflict
		}
		a.Snapshots = append(a.Snapshots, Snapshot{Revision: newRevision, Inputs: inputs, ActorID: actor, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Annotate(repo, aid, actor string, in AnnotationInput) (Assessment, error) {
	return s.mutate(repo, aid, func(a *Assessment) error {
		if !text(actor, 500) || !annotationKinds[in.Kind] || !text(in.Body, 65536) || !unique(in.ControlIDs, true) || len(in.Citations) == 0 {
			return ErrInvalid
		}
		known := map[string]bool{}
		for _, x := range a.Impacts {
			known[x.ControlID] = true
		}
		for _, x := range in.ControlIDs {
			if !known[x] {
				return ErrInvalid
			}
		}
		for _, c := range in.Citations {
			if !text(c.Reference, 1000) || !text(c.Revision, 500) || (c.Audience != "public" && c.Audience != "repository") {
				return ErrInvalid
			}
		}
		a.Annotations = append(a.Annotations, Annotation{ID: id(), AnnotationInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Decide(repo, aid, actor, control, decision, rationale string) (Assessment, error) {
	return s.mutate(repo, aid, func(a *Assessment) error {
		var impact *Impact
		for i := range a.Impacts {
			if a.Impacts[i].ControlID == control {
				impact = &a.Impacts[i]
			}
		}
		if impact == nil || !text(rationale, 65536) || (decision != "acknowledge" && decision != "request_changes") {
			return ErrInvalid
		}
		owner := false
		for _, o := range impact.RequiredOwnerIDs {
			owner = owner || o == actor
		}
		if !owner {
			return ErrInvalid
		}
		cur, _ := inputMap(a.Snapshots[len(a.Snapshots)-1].Inputs)
		bound := map[string]string{}
		for _, k := range impact.InputKeys {
			bound[k] = cur[k]
		}
		a.Decisions = append(a.Decisions, Decision{ControlID: control, OwnerID: actor, Decision: decision, Rationale: rationale, InputRevisions: bound, CreatedAt: s.now().UTC()})
		return nil
	})
}
func derive(a *Assessment, p assuranceprograms.Program) {
	a.Blockers = nil
	a.Ready = true
	cur, _ := inputMap(a.Snapshots[len(a.Snapshots)-1].Inputs)
	if p.CurrentVersion != a.ProgramVersion {
		cur["program"] = hexInt(p.CurrentVersion)
	}
	for i := range a.Decisions {
		d := &a.Decisions[i]
		d.Stale = false
		for k, v := range d.InputRevisions {
			if cur[k] != v {
				d.Stale = true
			}
		}
	}
	for _, x := range a.Impacts {
		if !x.RequiredForReadiness {
			continue
		}
		accepted := map[string]bool{}
		rejected := false
		for _, d := range a.Decisions {
			if d.ControlID == x.ControlID && !d.Stale {
				if d.Decision == "acknowledge" {
					accepted[d.OwnerID] = true
				}
				rejected = rejected || d.Decision == "request_changes"
			}
		}
		allAccepted := true
		for _, owner := range x.RequiredOwnerIDs {
			allAccepted = allAccepted && accepted[owner]
		}
		if rejected {
			a.Blockers = append(a.Blockers, Blocker{Kind: "changes_requested", ControlID: x.ControlID, Detail: "a required control owner requested changes"})
		} else if !allAccepted {
			a.Blockers = append(a.Blockers, Blocker{Kind: "missing_current_acknowledgement", ControlID: x.ControlID, Detail: "the affected control lacks a current owner acknowledgement"})
		}
		for _, act := range x.Actions {
			if act.Required && act.Kind == "evidence" && len(act.EvidencePackageIDs) == 0 {
				a.Blockers = append(a.Blockers, Blocker{Kind: "missing_evidence", ControlID: x.ControlID, ActionID: act.ID, Detail: "required current evidence is missing"})
			}
		}
	}
	a.Ready = len(a.Blockers) == 0
}
func (s *Store) Get(repo, aid string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, aid)
	if e == nil {
		p, x := s.programs.Get(repo, a.ProgramID)
		if x != nil {
			return Assessment{}, ErrNotFound
		}
		derive(&a, p)
	}
	return a, e
}
func (s *Store) List(repo string) ([]Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Assessment{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Assessment{}
	for _, f := range es {
		if filepath.Ext(f.Name()) == ".json" {
			a, x := s.read(repo, strings.TrimSuffix(f.Name(), ".json"))
			if x != nil {
				return nil, x
			}
			p, x := s.programs.Get(repo, a.ProgramID)
			if x == nil {
				derive(&a, p)
			}
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
