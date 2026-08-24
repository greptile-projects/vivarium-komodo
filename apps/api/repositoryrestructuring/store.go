// Package repositoryrestructuring retains revision-exact, reviewable plans for
// changing project repository boundaries before any repository identity moves.
package repositoryrestructuring

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

var ErrNotFound = errors.New("repository restructuring plan not found")
var ErrInvalid = errors.New("invalid repository restructuring plan")

var resourceKinds = map[string]bool{"ref": true, "pull_request": true, "issue": true, "task": true, "release": true, "package": true, "documentation": true, "policy": true, "workspace": true, "automation": true, "consumer": true, "federated_relationship": true}
var dispositions = map[string]bool{"move": true, "remain": true, "copy": true, "split": true, "redirect": true, "retire": true, "unresolved": true}
var accessStates = map[string]bool{"accessible": true, "inaccessible": true, "ambiguous": true, "shared": true}
var historyModes = map[string]bool{"full": true, "path_history": true, "selected_commits": true, "squash": true, "none": true}

type Source struct {
	RepositoryID string   `json:"repository_id"`
	Revision     string   `json:"revision"`
	OwnerIDs     []string `json:"owner_ids"`
	Role         string   `json:"role"`
}

type Destination struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	OwnerIDs           []string `json:"owner_ids"`
	Visibility         string   `json:"visibility"`
	DefaultBranch      string   `json:"default_branch"`
	RetainedIdentities []string `json:"retained_identities,omitempty"`
}

type Mapping struct {
	ID                 string   `json:"id"`
	SourceRepositoryID string   `json:"source_repository_id"`
	SourceRevision     string   `json:"source_revision"`
	SourcePaths        []string `json:"source_paths"`
	DestinationID      string   `json:"destination_id,omitempty"`
	DestinationPaths   []string `json:"destination_paths,omitempty"`
	HistoryMode        string   `json:"history_mode"`
	IncludeRefs        []string `json:"include_refs,omitempty"`
	ExcludeRefs        []string `json:"exclude_refs,omitempty"`
	Disposition        string   `json:"disposition"`
	Rationale          string   `json:"rationale"`
}

type InventoryItem struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	RepositoryID   string   `json:"repository_id"`
	Reference      string   `json:"reference"`
	Revision       string   `json:"revision"`
	OwnerIDs       []string `json:"owner_ids,omitempty"`
	Access         string   `json:"access"`
	Disposition    string   `json:"disposition"`
	DestinationIDs []string `json:"destination_ids,omitempty"`
	SharedWith     []string `json:"shared_with,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}

type RollbackLimits struct {
	LatestTime         time.Time `json:"latest_time"`
	IrreversibleAfter  string    `json:"irreversible_after"`
	MaximumDataLoss    string    `json:"maximum_data_loss"`
	RequiredRetentions []string  `json:"required_retentions"`
}

type Input struct {
	Title           string          `json:"title"`
	Summary         string          `json:"summary"`
	Sources         []Source        `json:"sources"`
	Destinations    []Destination   `json:"destinations"`
	Mappings        []Mapping       `json:"mappings"`
	Inventory       []InventoryItem `json:"inventory"`
	Deadline        time.Time       `json:"deadline"`
	SuccessCriteria []string        `json:"success_criteria"`
	RollbackLimits  RollbackLimits  `json:"rollback_limits"`
}

type Citation struct {
	RepositoryID string `json:"repository_id"`
	Reference    string `json:"reference"`
	Revision     string `json:"revision"`
	Path         string `json:"path,omitempty"`
}

type FindingInput struct {
	ActorKind        string     `json:"actor_kind"`
	Summary          string     `json:"summary"`
	Impact           string     `json:"impact"`
	AffectedItemIDs  []string   `json:"affected_item_ids"`
	AffectedOwnerIDs []string   `json:"affected_owner_ids,omitempty"`
	Uncertainty      string     `json:"uncertainty,omitempty"`
	Citations        []Citation `json:"citations"`
}

type Finding struct {
	ID      string `json:"id"`
	ActorID string `json:"actor_id"`
	FindingInput
	CreatedAt time.Time `json:"created_at"`
}

type Blocker struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Detail     string `json:"detail"`
}

type Plan struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	CreatorID    string `json:"creator_id"`
	Input
	Findings         []Finding `json:"findings"`
	Blockers         []Blocker `json:"blockers"`
	AuthorityGranted []string  `json:"authority_granted"`
	CreatedAt        time.Time `json:"created_at"`
}

type Store struct {
	root string
	mu   sync.Mutex
}

func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) Create(repositoryID, actorID string, in Input) (*Plan, error) {
	if !valid(in) {
		return nil, ErrInvalid
	}
	p := &Plan{ID: id("rsp"), RepositoryID: repositoryID, CreatorID: actorID, Input: in, Findings: []Finding{}, AuthorityGranted: []string{}, CreatedAt: time.Now().UTC()}
	p.Blockers = blockers(in.Inventory)
	if err := s.write(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) AddFinding(repositoryID, planID, actorID string, in FindingInput) (*Plan, error) {
	if strings.TrimSpace(in.Summary) == "" || strings.TrimSpace(in.Impact) == "" || len(in.Citations) == 0 {
		return nil, ErrInvalid
	}
	for _, c := range in.Citations {
		if c.RepositoryID == "" || c.Reference == "" || c.Revision == "" {
			return nil, ErrInvalid
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, x := range p.Inventory {
		known[x.ID] = true
	}
	for _, x := range in.AffectedItemIDs {
		if !known[x] {
			return nil, ErrInvalid
		}
	}
	p.Findings = append(p.Findings, Finding{ID: id("fnd"), ActorID: actorID, FindingInput: in, CreatedAt: time.Now().UTC()})
	if err = s.writeUnlocked(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) Get(repositoryID, planID string) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repositoryID, planID)
}
func (s *Store) List(repositoryID string) ([]Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, repositoryID))
	if os.IsNotExist(err) {
		return []Plan{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Plan{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		p, er := s.read(repositoryID, strings.TrimSuffix(e.Name(), ".json"))
		if er == nil {
			out = append(out, *p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func valid(in Input) bool {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Summary) == "" || len(in.Sources) == 0 || len(in.Destinations) == 0 || len(in.Mappings) == 0 || len(in.Inventory) == 0 || len(in.SuccessCriteria) == 0 || in.Deadline.IsZero() || in.RollbackLimits.LatestTime.IsZero() {
		return false
	}
	sources := map[string]string{}
	for _, x := range in.Sources {
		if x.RepositoryID == "" || x.Revision == "" || len(x.OwnerIDs) == 0 {
			return false
		}
		sources[x.RepositoryID] = x.Revision
	}
	dests := map[string]bool{}
	for _, x := range in.Destinations {
		if x.ID == "" || x.Name == "" || len(x.OwnerIDs) == 0 || x.DefaultBranch == "" || (x.Visibility != "public" && x.Visibility != "private" && x.Visibility != "internal") || dests[x.ID] {
			return false
		}
		dests[x.ID] = true
	}
	maps := map[string]bool{}
	for _, x := range in.Mappings {
		if x.ID == "" || maps[x.ID] || sources[x.SourceRepositoryID] != "" && sources[x.SourceRepositoryID] != x.SourceRevision || sources[x.SourceRepositoryID] == "" || !dispositions[x.Disposition] || !historyModes[x.HistoryMode] || len(x.SourcePaths) == 0 {
			return false
		}
		if x.DestinationID != "" && !dests[x.DestinationID] {
			return false
		}
		maps[x.ID] = true
	}
	items := map[string]bool{}
	for _, x := range in.Inventory {
		if x.ID == "" || items[x.ID] || !resourceKinds[x.Kind] || sources[x.RepositoryID] == "" || x.Revision == "" || !accessStates[x.Access] || !dispositions[x.Disposition] {
			return false
		}
		for _, d := range x.DestinationIDs {
			if !dests[d] {
				return false
			}
		}
		items[x.ID] = true
	}
	return true
}

func blockers(items []InventoryItem) []Blocker {
	out := []Blocker{}
	for _, x := range items {
		if x.Access != "accessible" {
			out = append(out, Blocker{Kind: x.Access, ResourceID: x.ID, Detail: x.Reason})
		}
		if x.Disposition == "unresolved" {
			out = append(out, Blocker{Kind: "unresolved_mapping", ResourceID: x.ID, Detail: x.Reason})
		}
	}
	return out
}
func id(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}
func (s *Store) path(r, p string) string { return filepath.Join(s.root, r, p+".json") }
func (s *Store) write(p *Plan) error     { s.mu.Lock(); defer s.mu.Unlock(); return s.writeUnlocked(p) }
func (s *Store) writeUnlocked(p *Plan) error {
	d := filepath.Dir(s.path(p.RepositoryID, p.ID))
	if err := os.MkdirAll(d, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path(p.RepositoryID, p.ID) + ".tmp"
	if err = os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(p.RepositoryID, p.ID))
}
func (s *Store) read(r, p string) (*Plan, error) {
	b, err := os.ReadFile(s.path(r, p))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var x Plan
	if json.Unmarshal(b, &x) != nil {
		return nil, ErrInvalid
	}
	return &x, nil
}
