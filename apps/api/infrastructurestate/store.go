// Package infrastructurestate owns immutable, reviewable infrastructure inventories.
package infrastructurestate

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

var (
	ErrNotFound = errors.New("infrastructure definition not found")
	ErrInvalid  = errors.New("invalid infrastructure state")
	ErrConflict = errors.New("infrastructure definition version conflict")
)

type Boundary struct {
	Name           string `json:"name"`
	Source         string `json:"source"`
	SecretBacked   bool   `json:"secret_backed"`
	Classification string `json:"classification"`
}
type Constraint struct {
	Kind       string `json:"kind"`
	Commitment string `json:"commitment"`
	Reference  string `json:"reference,omitempty"`
}
type Resource struct {
	ID               string       `json:"id"`
	Kind             string       `json:"kind"`
	Name             string       `json:"name"`
	Provider         string       `json:"provider"`
	ProviderResource string       `json:"provider_resource,omitempty"`
	OwnerIDs         []string     `json:"owner_ids"`
	DependsOn        []string     `json:"depends_on"`
	Environments     []string     `json:"environments"`
	Configuration    []Boundary   `json:"configuration"`
	Constraints      []Constraint `json:"constraints"`
}
type Environment struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Tier      string   `json:"tier"`
	Regions   []string `json:"regions"`
	OwnerIDs  []string `json:"owner_ids"`
	ReleaseID string   `json:"release_id,omitempty"`
}
type VersionInput struct {
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	SourceRevision string        `json:"source_revision"`
	DefinitionPath string        `json:"definition_path"`
	Format         string        `json:"format"`
	OwnerIDs       []string      `json:"owner_ids"`
	Environments   []Environment `json:"environments"`
	Resources      []Resource    `json:"resources"`
	ChangeReason   string        `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	VersionInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type ObservedResource struct {
	ResourceID         string `json:"resource_id,omitempty"`
	ProviderResource   string `json:"provider_resource"`
	Kind               string `json:"kind"`
	Status             string `json:"status"`
	Region             string `json:"region,omitempty"`
	Capacity           string `json:"capacity,omitempty"`
	ConfigurationState string `json:"configuration_state"`
}
type ObservationInput struct {
	DefinitionVersion  int64              `json:"definition_version"`
	SourceRevision     string             `json:"source_revision"`
	EnvironmentID      string             `json:"environment_id"`
	ReleaseID          string             `json:"release_id,omitempty"`
	Provider           string             `json:"provider"`
	ProviderAccessible bool               `json:"provider_accessible"`
	EvidenceReference  string             `json:"evidence_reference"`
	ObservedAt         time.Time          `json:"observed_at"`
	ValidUntil         time.Time          `json:"valid_until"`
	Resources          []ObservedResource `json:"resources"`
	Summary            string             `json:"summary"`
}
type Observation struct {
	ID string `json:"id"`
	ObservationInput
	ObserverID string    `json:"observer_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Gap struct {
	Kind          string `json:"kind"`
	ResourceID    string `json:"resource_id,omitempty"`
	EnvironmentID string `json:"environment_id,omitempty"`
	ObservationID string `json:"observation_id,omitempty"`
	Detail        string `json:"detail"`
}
type Definition struct {
	ID             string        `json:"id"`
	RepositoryID   string        `json:"repository_id"`
	CurrentVersion int64         `json:"current_version"`
	Versions       []Version     `json:"versions"`
	Observations   []Observation `json:"observations"`
	Gaps           []Gap         `json:"gaps"`
	NonAuthority   []string      `json:"non_authority"`
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
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func newid() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func stringsOK(xs []string, required bool) bool {
	if required && len(xs) == 0 || len(xs) > 100 {
		return false
	}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" {
			return false
		}
	}
	return true
}
func valid(in VersionInput) bool {
	if in.Name == "" || in.Description == "" || in.SourceRevision == "" || in.DefinitionPath == "" || in.Format == "" || in.ChangeReason == "" || !stringsOK(in.OwnerIDs, false) || len(in.Environments) == 0 || len(in.Resources) == 0 {
		return false
	}
	envs, res := map[string]bool{}, map[string]bool{}
	for _, e := range in.Environments {
		if e.ID == "" || envs[e.ID] || e.Name == "" || !map[string]bool{"development": true, "test": true, "staging": true, "production": true, "other": true}[e.Tier] || !stringsOK(e.Regions, true) || !stringsOK(e.OwnerIDs, false) {
			return false
		}
		envs[e.ID] = true
	}
	for _, r := range in.Resources {
		if r.ID == "" || res[r.ID] || r.Name == "" || r.Provider == "" || !map[string]bool{"service": true, "network": true, "identity": true, "data_store": true, "compute": true, "external_dependency": true}[r.Kind] || !stringsOK(r.OwnerIDs, false) || !stringsOK(r.DependsOn, false) || !stringsOK(r.Environments, true) {
			return false
		}
		for _, e := range r.Environments {
			if !envs[e] {
				return false
			}
		}
		for _, b := range r.Configuration {
			if b.Name == "" || b.Source == "" || credentialShaped(b.Source) || !map[string]bool{"public": true, "internal": true, "sensitive": true, "restricted": true}[b.Classification] {
				return false
			}
		}
		for _, c := range r.Constraints {
			if !map[string]bool{"cost": true, "capacity": true, "security": true, "privacy": true, "reliability": true, "continuity": true, "regional": true}[c.Kind] || c.Commitment == "" {
				return false
			}
		}
		res[r.ID] = true
	}
	for _, r := range in.Resources {
		for _, d := range r.DependsOn {
			if !res[d] {
				return false
			}
		}
	}
	return true
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(x Definition) error {
	p := s.path(x.RepositoryID, x.ID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e != nil {
		return e
	}
	if e = os.WriteFile(p+".tmp", b, 0640); e != nil {
		return e
	}
	return os.Rename(p+".tmp", p)
}
func (s *Store) load(repo, id string) (Definition, error) {
	var x Definition
	b, e := os.ReadFile(s.path(repo, id))
	if os.IsNotExist(e) {
		return x, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func derive(x Definition, now time.Time) Definition {
	x.Gaps = nil
	x.NonAuthority = []string{"publication grants no provider, credential, deployment, environment, or operational authority", "observations are bounded attestations, not provider authority"}
	if len(x.Versions) == 0 {
		return x
	}
	v := x.Versions[len(x.Versions)-1]
	if len(v.OwnerIDs) == 0 {
		x.Gaps = append(x.Gaps, Gap{Kind: "missing_ownership", Detail: "the infrastructure definition has no accountable owner"})
	}
	owners := map[string][]string{}
	for _, r := range v.Resources {
		if len(r.OwnerIDs) == 0 {
			x.Gaps = append(x.Gaps, Gap{Kind: "missing_ownership", ResourceID: r.ID, Detail: r.Name + " has no accountable owner"})
		}
		if p := r.ProviderResource; p != "" {
			owners[r.Provider+"/"+p] = append(owners[r.Provider+"/"+p], r.ID)
		}
		for _, b := range r.Configuration {
			if b.SecretBacked {
				x.Gaps = append(x.Gaps, Gap{Kind: "secret_backed_value", ResourceID: r.ID, Detail: b.Name + " is secret-backed; only its boundary is published"})
			}
		}
	}
	for k, ids := range owners {
		if len(ids) > 1 {
			x.Gaps = append(x.Gaps, Gap{Kind: "conflicting_ownership", ResourceID: strings.Join(ids, ","), Detail: k + " is claimed by multiple declared resources"})
		}
	}
	latest := map[string]Observation{}
	for _, o := range x.Observations {
		if p, ok := latest[o.EnvironmentID]; !ok || p.ObservedAt.Before(o.ObservedAt) {
			latest[o.EnvironmentID] = o
		}
	}
	for _, e := range v.Environments {
		o, ok := latest[e.ID]
		if !ok {
			x.Gaps = append(x.Gaps, Gap{Kind: "missing_observation", EnvironmentID: e.ID, Detail: e.Name + " has no permitted observation"})
			continue
		}
		if !o.ProviderAccessible {
			x.Gaps = append(x.Gaps, Gap{Kind: "inaccessible_provider", EnvironmentID: e.ID, ObservationID: o.ID, Detail: o.Provider + " was inaccessible"})
		}
		if now.After(o.ValidUntil) || o.DefinitionVersion != x.CurrentVersion || o.SourceRevision != v.SourceRevision {
			x.Gaps = append(x.Gaps, Gap{Kind: "stale_observation", EnvironmentID: e.ID, ObservationID: o.ID, Detail: e.Name + " observation is stale against the current definition"})
		}
		for _, r := range o.Resources {
			if r.ResourceID == "" {
				x.Gaps = append(x.Gaps, Gap{Kind: "unmanaged_resource", EnvironmentID: e.ID, ObservationID: o.ID, Detail: r.ProviderResource + " is observed but not declared"})
			}
		}
	}
	sort.Slice(x.Gaps, func(i, j int) bool { return x.Gaps[i].Kind+x.Gaps[i].Detail < x.Gaps[j].Kind+x.Gaps[j].Detail })
	return x
}
func (s *Store) Create(repo, author string, in VersionInput) (Definition, error) {
	if repo == "" || author == "" || !valid(in) {
		return Definition{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.now().UTC()
	x := Definition{ID: newid(), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{{Number: 1, VersionInput: in, AuthorID: author, CreatedAt: n}}}
	if e := s.save(x); e != nil {
		return Definition{}, e
	}
	return derive(x, n), nil
}
func (s *Store) Revise(repo, id, author string, expected int64, in VersionInput) (Definition, error) {
	if author == "" || !valid(in) {
		return Definition{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, id)
	if e != nil {
		return x, e
	}
	if x.CurrentVersion != expected {
		return Definition{}, ErrConflict
	}
	n := s.now().UTC()
	x.CurrentVersion++
	x.Versions = append(x.Versions, Version{Number: x.CurrentVersion, VersionInput: in, AuthorID: author, CreatedAt: n})
	if e = s.save(x); e != nil {
		return Definition{}, e
	}
	return derive(x, n), nil
}
func validObservation(x Definition, in ObservationInput) bool {
	if in.DefinitionVersion < 1 || in.DefinitionVersion > x.CurrentVersion || in.SourceRevision == "" || in.EnvironmentID == "" || in.Provider == "" || in.EvidenceReference == "" || in.ObservedAt.IsZero() || !in.ValidUntil.After(in.ObservedAt) || in.Summary == "" {
		return false
	}
	for _, value := range []string{in.EvidenceReference, in.Summary} {
		if credentialShaped(value) {
			return false
		}
	}
	v := x.Versions[in.DefinitionVersion-1]
	found := false
	for _, e := range v.Environments {
		if e.ID == in.EnvironmentID {
			found = true
		}
	}
	if !found {
		return false
	}
	declared := map[string]bool{}
	for _, r := range v.Resources {
		declared[r.ID] = true
	}
	for _, r := range in.Resources {
		if r.ProviderResource == "" || r.Kind == "" || r.Status == "" || !map[string]bool{"matching": true, "drifted": true, "unknown": true, "redacted": true}[r.ConfigurationState] || (r.ResourceID != "" && !declared[r.ResourceID]) {
			return false
		}
		if credentialShaped(r.ProviderResource) || credentialShaped(r.Status) || credentialShaped(r.Capacity) {
			return false
		}
	}
	return true
}

func credentialShaped(value string) bool {
	v := strings.ToLower(value)
	for _, marker := range []string{"-----begin private key", "password=", "password:", "token=", "token:", "secret=", "secret:", "vka_"} {
		if strings.Contains(v, marker) {
			return true
		}
	}
	return false
}
func (s *Store) Observe(repo, id, actor string, in ObservationInput) (Definition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, id)
	if e != nil {
		return x, e
	}
	n := s.now().UTC()
	if actor == "" || !validObservation(x, in) || in.ObservedAt.After(n.Add(5*time.Minute)) || in.ValidUntil.Sub(in.ObservedAt) > 90*24*time.Hour {
		return Definition{}, ErrInvalid
	}
	x.Observations = append(x.Observations, Observation{ID: newid(), ObservationInput: in, ObserverID: actor, CreatedAt: n})
	if e = s.save(x); e != nil {
		return Definition{}, e
	}
	return derive(x, n), nil
}
func (s *Store) Get(repo, id string) (Definition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, id)
	return derive(x, s.now().UTC()), e
}
func (s *Store) List(repo string) ([]Definition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if os.IsNotExist(e) {
		return []Definition{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Definition{}
	for _, f := range es {
		if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, er := s.load(repo, strings.TrimSuffix(f.Name(), ".json"))
		if er != nil {
			return nil, er
		}
		out = append(out, derive(x, s.now().UTC()))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Versions[len(out[i].Versions)-1].Name < out[j].Versions[len(out[j].Versions)-1].Name
	})
	return out, nil
}
