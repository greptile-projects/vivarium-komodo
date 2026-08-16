// Package recoveryobjectives owns versioned repository continuity commitments.
package recoveryobjectives

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
	ErrNotFound = errors.New("recovery objective not found")
	ErrInvalid  = errors.New("invalid recovery objective")
	ErrConflict = errors.New("recovery objective version conflict")
)

type Resource struct {
	ID                   string   `json:"id"`
	Kind                 string   `json:"kind"`
	ResourceID           string   `json:"resource_id,omitempty"`
	Name                 string   `json:"name"`
	UserCapability       string   `json:"user_capability"`
	OwnerIDs             []string `json:"owner_ids"`
	DependencyIDs        []string `json:"dependency_ids"`
	AcceptableLoss       string   `json:"acceptable_loss"`
	RestorationTime      string   `json:"restoration_time"`
	Retention            string   `json:"retention"`
	Jurisdictions        []string `json:"jurisdictions"`
	ValidationCriteria   []string `json:"validation_criteria"`
	Feasibility          string   `json:"feasibility"`
	FeasibilityRationale string   `json:"feasibility_rationale,omitempty"`
}

type Dependency struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	OwnerIDs   []string `json:"owner_ids"`
	Protected  bool     `json:"protected"`
	Protection string   `json:"protection_reference,omitempty"`
}

type Link struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Label      string `json:"label"`
}

type Exclusion struct {
	ResourceID string `json:"resource_id,omitempty"`
	Scope      string `json:"scope"`
	Reason     string `json:"reason"`
	ApprovedBy string `json:"approved_by"`
}

type Exception struct {
	ID         string    `json:"id"`
	Scope      string    `json:"scope"`
	Reason     string    `json:"reason"`
	OwnerID    string    `json:"owner_id"`
	ApprovedBy string    `json:"approved_by"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type VersionInput struct {
	Title           string       `json:"title"`
	Description     string       `json:"description"`
	OwnerIDs        []string     `json:"owner_ids"`
	Resources       []Resource   `json:"resources"`
	Dependencies    []Dependency `json:"dependencies"`
	Links           []Link       `json:"links"`
	Exclusions      []Exclusion  `json:"declared_exclusions"`
	Exceptions      []Exception  `json:"exceptions"`
	ExceptionPolicy string       `json:"exception_policy"`
	ChangeReason    string       `json:"change_reason"`
}

type Version struct {
	Number int64 `json:"number"`
	VersionInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Blocker struct {
	Kind         string `json:"kind"`
	ResourceID   string `json:"resource_id,omitempty"`
	DependencyID string `json:"dependency_id,omitempty"`
	ExceptionID  string `json:"exception_id,omitempty"`
	Detail       string `json:"detail"`
}

type Objective struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Blockers       []Blocker `json:"blockers"`
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
	a, err := filepath.Abs(root)
	if err == nil {
		err = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, err
}

func newid() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }

func stringsOK(xs []string, required bool) bool {
	if required && len(xs) == 0 {
		return false
	}
	if len(xs) > 100 {
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
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Description) == "" || strings.TrimSpace(in.ChangeReason) == "" || strings.TrimSpace(in.ExceptionPolicy) == "" || len(in.Resources) == 0 || !stringsOK(in.OwnerIDs, false) {
		return false
	}
	resources, dependencies, exceptions := map[string]bool{}, map[string]bool{}, map[string]bool{}
	kinds := map[string]bool{"repository": true, "package": true, "artifact": true, "configuration": true, "collaboration_records": true, "deployed_service_data": true}
	for _, r := range in.Resources {
		if r.ID == "" || resources[r.ID] || !kinds[r.Kind] || r.Name == "" || r.UserCapability == "" || r.AcceptableLoss == "" || r.RestorationTime == "" || r.Retention == "" || !stringsOK(r.OwnerIDs, false) || !stringsOK(r.DependencyIDs, false) || !stringsOK(r.Jurisdictions, true) || !stringsOK(r.ValidationCriteria, true) || !map[string]bool{"achievable": true, "unverified": true, "impossible": true}[r.Feasibility] {
			return false
		}
		if r.Kind != "repository" && r.ResourceID == "" {
			return false
		}
		if r.Feasibility == "impossible" && strings.TrimSpace(r.FeasibilityRationale) == "" {
			return false
		}
		resources[r.ID] = true
	}
	for _, d := range in.Dependencies {
		if d.ID == "" || dependencies[d.ID] || d.Name == "" || d.Kind == "" || !stringsOK(d.OwnerIDs, false) || (d.Protected && d.Protection == "") {
			return false
		}
		dependencies[d.ID] = true
	}
	for _, r := range in.Resources {
		for _, id := range r.DependencyIDs {
			if !dependencies[id] {
				return false
			}
		}
	}
	for _, l := range in.Links {
		if !map[string]bool{"service_objective": true, "environment": true, "incident": true, "privacy_rule": true, "governance": true}[l.Kind] || l.ResourceID == "" || l.Label == "" {
			return false
		}
	}
	for _, x := range in.Exclusions {
		if x.Scope == "" || x.Reason == "" || x.ApprovedBy == "" {
			return false
		}
	}
	for _, x := range in.Exceptions {
		if x.ID == "" || exceptions[x.ID] || x.Scope == "" || x.Reason == "" || x.OwnerID == "" || x.ApprovedBy == "" || x.ExpiresAt.IsZero() {
			return false
		}
		exceptions[x.ID] = true
	}
	return true
}

func (s *Store) path(repository, id string) string {
	return filepath.Join(s.root, repository, id+".json")
}
func (s *Store) save(x Objective) error {
	dir := filepath.Dir(s.path(x.RepositoryID, x.ID))
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path(x.RepositoryID, x.ID) + ".tmp"
	if err = os.WriteFile(tmp, b, 0640); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(x.RepositoryID, x.ID))
}
func (s *Store) load(repository, id string) (Objective, error) {
	var x Objective
	b, err := os.ReadFile(s.path(repository, id))
	if os.IsNotExist(err) {
		return x, ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal(b, &x)
	}
	return x, err
}

func derive(x Objective, now time.Time) Objective {
	x.Blockers = nil
	if len(x.Versions) == 0 {
		return x
	}
	v := x.Versions[len(x.Versions)-1]
	if len(v.OwnerIDs) == 0 {
		x.Blockers = append(x.Blockers, Blocker{Kind: "missing_ownership", Detail: "the continuity commitment has no accountable owner"})
	}
	deps := map[string]Dependency{}
	for _, d := range v.Dependencies {
		deps[d.ID] = d
	}
	for _, r := range v.Resources {
		if len(r.OwnerIDs) == 0 {
			x.Blockers = append(x.Blockers, Blocker{Kind: "missing_ownership", ResourceID: r.ID, Detail: r.Name + " has no restoration owner"})
		}
		if r.Feasibility == "impossible" {
			x.Blockers = append(x.Blockers, Blocker{Kind: "impossible_target", ResourceID: r.ID, Detail: r.Name + ": " + r.FeasibilityRationale})
		}
		if r.Feasibility == "unverified" {
			x.Blockers = append(x.Blockers, Blocker{Kind: "unverified_target", ResourceID: r.ID, Detail: r.Name + " targets have not been demonstrated"})
		}
		for _, id := range r.DependencyIDs {
			d := deps[id]
			if !d.Protected {
				x.Blockers = append(x.Blockers, Blocker{Kind: "unprotected_dependency", ResourceID: r.ID, DependencyID: id, Detail: d.Name + " is required by " + r.Name + " but has no declared protection"})
			}
			if len(d.OwnerIDs) == 0 {
				x.Blockers = append(x.Blockers, Blocker{Kind: "missing_dependency_owner", ResourceID: r.ID, DependencyID: id, Detail: d.Name + " has no accountable owner"})
			}
		}
	}
	for _, e := range v.Exceptions {
		remaining := e.ExpiresAt.Sub(now)
		if remaining <= 0 {
			x.Blockers = append(x.Blockers, Blocker{Kind: "expired_exception", ExceptionID: e.ID, Detail: e.Scope + " exception expired"})
		} else if remaining <= 30*24*time.Hour {
			x.Blockers = append(x.Blockers, Blocker{Kind: "expiring_exception", ExceptionID: e.ID, Detail: e.Scope + " exception expires within 30 days"})
		}
	}
	return x
}

func (s *Store) Create(repository, author string, in VersionInput) (Objective, error) {
	if repository == "" || author == "" || !valid(in) {
		return Objective{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	x := Objective{ID: newid(), RepositoryID: repository, CurrentVersion: 1, Versions: []Version{{Number: 1, VersionInput: in, AuthorID: author, CreatedAt: now}}}
	if err := s.save(x); err != nil {
		return Objective{}, err
	}
	return derive(x, now), nil
}
func (s *Store) Revise(repository, id, author string, expected int64, in VersionInput) (Objective, error) {
	if author == "" || !valid(in) {
		return Objective{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, err := s.load(repository, id)
	if err != nil {
		return x, err
	}
	if x.CurrentVersion != expected {
		return Objective{}, ErrConflict
	}
	now := s.now().UTC()
	x.CurrentVersion++
	x.Versions = append(x.Versions, Version{Number: x.CurrentVersion, VersionInput: in, AuthorID: author, CreatedAt: now})
	if err = s.save(x); err != nil {
		return Objective{}, err
	}
	return derive(x, now), nil
}
func (s *Store) Get(repository, id string) (Objective, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, err := s.load(repository, id)
	return derive(x, s.now().UTC()), err
}
func (s *Store) List(repository string) ([]Objective, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, repository))
	if os.IsNotExist(err) {
		return []Objective{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Objective{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		x, er := s.load(repository, strings.TrimSuffix(e.Name(), ".json"))
		if er != nil {
			return nil, er
		}
		out = append(out, derive(x, s.now().UTC()))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Versions[0].CreatedAt.Before(out[j].Versions[0].CreatedAt) })
	return out, nil
}
