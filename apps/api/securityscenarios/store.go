// Package securityscenarios retains reviewed, revision-exact abuse and defense evidence.
package securityscenarios

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

var ErrInvalid = errors.New("invalid security scenario")
var ErrNotFound = errors.New("security scenario not found")

type Fixture struct {
	ID                     string `json:"id"`
	Description            string `json:"description"`
	Generator              string `json:"generator"`
	Synthetic              bool   `json:"synthetic"`
	ContainsProductionData bool   `json:"contains_production_data"`
	ContainsSecrets        bool   `json:"contains_secrets"`
}
type Action struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Command     string `json:"command"`
}
type Criterion struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Observable  string `json:"observable"`
}
type Capability struct {
	Name     string `json:"name"`
	Boundary string `json:"boundary"`
}
type Input struct {
	Name                  string       `json:"name"`
	ThreatModelID         string       `json:"threat_model_id"`
	ThreatModelRevision   string       `json:"threat_model_revision"`
	AbusePathID           string       `json:"abuse_path_id"`
	SourceRevision        string       `json:"source_revision"`
	DefinitionPath        string       `json:"definition_path"`
	AttackerPreconditions []string     `json:"attacker_preconditions"`
	Capabilities          []Capability `json:"bounded_capabilities"`
	Fixtures              []Fixture    `json:"fixtures"`
	Actions               []Action     `json:"actions"`
	Containment           []Criterion  `json:"expected_containment"`
	Detection             []Criterion  `json:"detection_criteria"`
	Recovery              []Criterion  `json:"recovery_criteria"`
	OwnerIDs              []string     `json:"owner_ids"`
	ChangeReason          string       `json:"change_reason"`
}
type Review struct {
	ID              string    `json:"id"`
	OwnerID         string    `json:"owner_id"`
	Decision        string    `json:"decision"`
	Rationale       string    `json:"rationale"`
	ScenarioVersion int64     `json:"scenario_version"`
	CreatedAt       time.Time `json:"created_at"`
}
type Artifact struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
	Sanitized bool   `json:"sanitized"`
}
type Coverage struct {
	ContainmentIDs []string `json:"containment_ids"`
	DetectionIDs   []string `json:"detection_ids"`
	RecoveryIDs    []string `json:"recovery_ids"`
}
type AttemptInput struct {
	ScenarioVersion          int64      `json:"scenario_version"`
	TargetKind               string     `json:"target_kind"`
	PullRequestID            string     `json:"pull_request_id"`
	PreviewID                string     `json:"preview_id,omitempty"`
	Revision                 string     `json:"revision"`
	Isolation                string     `json:"isolation"`
	Network                  string     `json:"network"`
	Status                   string     `json:"status"`
	Commands                 []string   `json:"sanitized_commands"`
	Logs                     []string   `json:"sanitized_logs"`
	Traces                   []string   `json:"sanitized_traces"`
	Artifacts                []Artifact `json:"artifacts"`
	Coverage                 Coverage   `json:"coverage"`
	Cost                     float64    `json:"cost"`
	Currency                 string     `json:"currency"`
	Provenance               []string   `json:"provenance"`
	Blockers                 []string   `json:"blockers"`
	DestructiveEffects       bool       `json:"destructive_effects"`
	ContainsSecrets          bool       `json:"contains_secrets"`
	ContainsProductionData   bool       `json:"contains_production_data"`
	ContainsHiddenMaterial   bool       `json:"contains_hidden_test_material"`
	InaccessibleDependencies []string   `json:"inaccessible_dependencies"`
}
type Attempt struct {
	ID string `json:"id"`
	AttemptInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
	Current   bool      `json:"current"`
}
type Version struct {
	Version int64 `json:"version"`
	Input
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Scenario struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	Versions       []Version `json:"versions"`
	Reviews        []Review  `json:"reviews"`
	Attempts       []Attempt `json:"attempts"`
	CurrentVersion int64     `json:"current_version"`
	Approved       bool      `json:"approved"`
	Gaps           []string  `json:"gaps"`
	NonAuthority   []string  `json:"non_authority"`
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
func texts(xs []string, required bool) bool {
	if required && len(xs) == 0 || len(xs) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x == "" || len(x) > 4000 || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Name) == "" || in.ThreatModelID == "" || in.ThreatModelRevision == "" || in.AbusePathID == "" || in.SourceRevision == "" || in.DefinitionPath == "" || !strings.HasPrefix(in.DefinitionPath, ".komodo/") || !texts(in.AttackerPreconditions, true) || !texts(in.OwnerIDs, true) || strings.TrimSpace(in.ChangeReason) == "" || len(in.Capabilities) == 0 || len(in.Fixtures) == 0 || len(in.Actions) == 0 || len(in.Containment) == 0 || len(in.Detection) == 0 || len(in.Recovery) == 0 {
		return false
	}
	for _, x := range in.Capabilities {
		if x.Name == "" || x.Boundary == "" {
			return false
		}
	}
	for _, x := range in.Fixtures {
		if x.ID == "" || x.Description == "" || x.Generator == "" || !x.Synthetic || x.ContainsProductionData || x.ContainsSecrets {
			return false
		}
	}
	for _, x := range in.Actions {
		if x.ID == "" || x.Description == "" || x.Command == "" {
			return false
		}
	}
	for _, xs := range [][]Criterion{in.Containment, in.Detection, in.Recovery} {
		seen := map[string]bool{}
		for _, x := range xs {
			if x.ID == "" || x.Description == "" || x.Observable == "" || seen[x.ID] {
				return false
			}
			seen[x.ID] = true
		}
	}
	return true
}
func (s *Store) path(repo string) string { return filepath.Join(s.root, repo+".json") }
func (s *Store) read(repo string) ([]Scenario, error) {
	b, e := os.ReadFile(s.path(repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Scenario{}, nil
	}
	if e != nil {
		return nil, e
	}
	var x []Scenario
	if json.Unmarshal(b, &x) != nil {
		return nil, ErrInvalid
	}
	return x, nil
}
func (s *Store) write(repo string, x []Scenario) error {
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(repo), b, 0640)
	}
	return e
}
func derive(x *Scenario) {
	x.CurrentVersion = int64(len(x.Versions))
	x.Approved = false
	x.Gaps = []string{}
	for _, r := range x.Reviews {
		if r.ScenarioVersion == x.CurrentVersion && r.Decision == "approve" {
			x.Approved = true
		}
		if r.ScenarioVersion == x.CurrentVersion && r.Decision == "request_changes" {
			x.Approved = false
		}
	}
	if !x.Approved {
		x.Gaps = append(x.Gaps, "current_version_not_approved")
	}
	current := false
	for i := range x.Attempts {
		x.Attempts[i].Current = x.Attempts[i].ScenarioVersion == x.CurrentVersion && x.Attempts[i].Revision == x.Versions[len(x.Versions)-1].SourceRevision
		current = current || x.Attempts[i].Current
	}
	if !current {
		x.Gaps = append(x.Gaps, "no_current_execution_evidence")
	}
	sort.Strings(x.Gaps)
}
func (s *Store) Create(repo, actor string, in Input) (Scenario, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Scenario{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.read(repo)
	if e != nil {
		return Scenario{}, e
	}
	now := s.now().UTC()
	x := Scenario{ID: id(), RepositoryID: repo, Versions: []Version{{Version: 1, Input: in, AuthorID: actor, CreatedAt: now}}, Reviews: []Review{}, Attempts: []Attempt{}, NonAuthority: []string{"repository write", "secret or credential access", "security approval", "deployment or environment authority"}}
	derive(&x)
	all = append(all, x)
	return x, s.write(repo, all)
}
func find(all []Scenario, sid string) (int, bool) {
	for i := range all {
		if all[i].ID == sid {
			return i, true
		}
	}
	return 0, false
}
func (s *Store) List(repo string) ([]Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo)
	for i := range x {
		derive(&x[i])
	}
	return x, e
}
func (s *Store) Get(repo, sid string) (Scenario, error) {
	x, e := s.List(repo)
	if e != nil {
		return Scenario{}, e
	}
	i, ok := find(x, sid)
	if !ok {
		return Scenario{}, ErrNotFound
	}
	return x[i], nil
}
func (s *Store) Revise(repo, sid, actor string, in Input) (Scenario, error) {
	if actor == "" || !valid(in) {
		return Scenario{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.read(repo)
	if e != nil {
		return Scenario{}, e
	}
	i, ok := find(all, sid)
	if !ok {
		return Scenario{}, ErrNotFound
	}
	all[i].Versions = append(all[i].Versions, Version{Version: int64(len(all[i].Versions) + 1), Input: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	derive(&all[i])
	return all[i], s.write(repo, all)
}
func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func (s *Store) Review(repo, sid, actor, decision, rationale string, version int64) (Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.read(repo)
	if e != nil {
		return Scenario{}, e
	}
	i, ok := find(all, sid)
	if !ok {
		return Scenario{}, ErrNotFound
	}
	v := all[i].Versions[len(all[i].Versions)-1]
	if !contains(v.OwnerIDs, actor) || version != v.Version || !map[string]bool{"approve": true, "request_changes": true}[decision] || strings.TrimSpace(rationale) == "" {
		return Scenario{}, ErrInvalid
	}
	all[i].Reviews = append(all[i].Reviews, Review{ID: id(), OwnerID: actor, Decision: decision, Rationale: rationale, ScenarioVersion: version, CreatedAt: s.now().UTC()})
	derive(&all[i])
	return all[i], s.write(repo, all)
}
func criteriaOK(ids []string, xs []Criterion) bool {
	if len(ids) == 0 {
		return false
	}
	for _, v := range ids {
		ok := false
		for _, x := range xs {
			ok = ok || x.ID == v
		}
		if !ok {
			return false
		}
	}
	return true
}
func (s *Store) AddAttempt(repo, sid, actor string, in AttemptInput) (Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.read(repo)
	if e != nil {
		return Scenario{}, e
	}
	i, ok := find(all, sid)
	if !ok {
		return Scenario{}, ErrNotFound
	}
	v := all[i].Versions[len(all[i].Versions)-1]
	safe := !in.DestructiveEffects && !in.ContainsSecrets && !in.ContainsProductionData && !in.ContainsHiddenMaterial
	statusOK := map[string]bool{"passed": true, "failed": true, "unsafe": true, "non_reproducible": true, "blocked": true}[in.Status]
	evidenceOK := texts(in.Commands, false) && texts(in.Logs, false) && texts(in.Traces, false) && texts(in.Provenance, true) && in.Cost >= 0 && in.Currency != ""
	coverageOK := criteriaOK(in.Coverage.ContainmentIDs, v.Containment) && criteriaOK(in.Coverage.DetectionIDs, v.Detection) && criteriaOK(in.Coverage.RecoveryIDs, v.Recovery)
	if actor == "" || in.ScenarioVersion != v.Version || in.Revision != v.SourceRevision || !map[string]bool{"workspace": true, "preview": true}[in.TargetKind] || in.PullRequestID == "" || in.Isolation != "ephemeral" || in.Network != "none" || !statusOK || !evidenceOK {
		return Scenario{}, ErrInvalid
	}
	if in.Status == "passed" && (!safe || len(in.InaccessibleDependencies) > 0 || !coverageOK) {
		return Scenario{}, ErrInvalid
	}
	if (!safe || len(in.InaccessibleDependencies) > 0 || !coverageOK) && len(in.Blockers) == 0 {
		return Scenario{}, ErrInvalid
	}
	for _, a := range in.Artifacts {
		if a.Name == "" || a.Digest == "" || a.MediaType == "" || a.Size < 0 || !a.Sanitized {
			return Scenario{}, ErrInvalid
		}
	}
	all[i].Attempts = append(all[i].Attempts, Attempt{ID: id(), AttemptInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
	derive(&all[i])
	return all[i], s.write(repo, all)
}
