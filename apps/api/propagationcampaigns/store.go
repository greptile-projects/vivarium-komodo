// Package propagationcampaigns retains the agreed scope of cross-line changes.
package propagationcampaigns

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

var ErrNotFound = errors.New("propagation campaign not found")
var ErrInvalid = errors.New("invalid propagation campaign")

type Source struct {
	Kind               string   `json:"kind"`
	RepositoryID       string   `json:"repository_id"`
	ResourceID         string   `json:"resource_id"`
	CommitIDs          []string `json:"commit_ids"`
	Revision           string   `json:"revision"`
	EvidenceReferences []string `json:"evidence_references,omitempty"`
}

type Authority struct {
	OwnerIDs   []string  `json:"owner_ids"`
	Access     string    `json:"access"`
	Basis      string    `json:"basis"`
	ObservedAt time.Time `json:"observed_at"`
}

type Target struct {
	ID                  string    `json:"id"`
	RepositoryID        string    `json:"repository_id,omitempty"`
	RepositoryReference string    `json:"repository_reference,omitempty"`
	ReleaseLine         string    `json:"release_line"`
	Revision            string    `json:"revision,omitempty"`
	PackageIDs          []string  `json:"package_ids,omitempty"`
	OwnerIDs            []string  `json:"owner_ids,omitempty"`
	Deadline            time.Time `json:"deadline"`
	DependsOn           []string  `json:"depends_on,omitempty"`
	Disposition         string    `json:"disposition"`
	DispositionReason   string    `json:"disposition_reason,omitempty"`
	Authority           Authority `json:"authority"`
}

type CompletionPolicy struct {
	Mode                   string   `json:"mode"`
	RequiredTargetIDs      []string `json:"required_target_ids,omitempty"`
	AllowEquivalent        bool     `json:"allow_already_equivalent"`
	ExceptionRequiresOwner bool     `json:"exception_requires_owner"`
}

type Input struct {
	Title              string           `json:"title"`
	Intent             string           `json:"intent"`
	AcceptanceCriteria []string         `json:"acceptance_criteria"`
	Source             Source           `json:"source"`
	Targets            []Target         `json:"targets"`
	CompletionPolicy   CompletionPolicy `json:"completion_policy"`
}

type Blocker struct {
	TargetID string `json:"target_id"`
	Kind     string `json:"kind"`
	Detail   string `json:"detail"`
}
type Campaign struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	CreatorID    string `json:"creator_id"`
	Input
	Blockers  []Blocker `json:"blockers"`
	CreatedAt time.Time `json:"created_at"`
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
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func textList(v []string, required bool) bool {
	if required && len(v) == 0 || len(v) > 100 {
		return false
	}
	for _, x := range v {
		if strings.TrimSpace(x) == "" {
			return false
		}
	}
	return true
}

func validate(in Input) bool {
	kinds := map[string]bool{"merged_pull_request": true, "security_repair": true, "regression_correction": true, "policy_change": true, "package_release": true, "interface_evolution": true}
	dispositions := map[string]bool{"pending": true, "unknown": true, "unsupported": true, "inaccessible": true, "already_equivalent": true}
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Intent) == "" || !textList(in.AcceptanceCriteria, true) || !kinds[in.Source.Kind] || in.Source.RepositoryID == "" || in.Source.ResourceID == "" || in.Source.Revision == "" || !textList(in.Source.CommitIDs, true) || len(in.Targets) == 0 || !map[string]bool{"all_supported": true, "required_targets": true}[in.CompletionPolicy.Mode] {
		return false
	}
	seen := map[string]bool{}
	for _, t := range in.Targets {
		if t.ID == "" || seen[t.ID] || (t.RepositoryID == "" && t.RepositoryReference == "") || t.ReleaseLine == "" || t.Deadline.IsZero() || !dispositions[t.Disposition] || t.Authority.Access == "" || t.Authority.Basis == "" || t.Authority.ObservedAt.IsZero() || !textList(t.OwnerIDs, false) || !textList(t.Authority.OwnerIDs, false) || (t.Disposition != "pending" && t.DispositionReason == "") {
			return false
		}
		seen[t.ID] = true
	}
	for _, t := range in.Targets {
		for _, d := range t.DependsOn {
			if !seen[d] || d == t.ID {
				return false
			}
		}
	}
	if cyclic(in.Targets) {
		return false
	}
	if in.CompletionPolicy.Mode == "required_targets" {
		if !textList(in.CompletionPolicy.RequiredTargetIDs, true) {
			return false
		}
		for _, x := range in.CompletionPolicy.RequiredTargetIDs {
			if !seen[x] {
				return false
			}
		}
	}
	return true
}
func cyclic(ts []Target) bool {
	deps := map[string][]string{}
	for _, t := range ts {
		deps[t.ID] = t.DependsOn
	}
	state := map[string]byte{}
	var visit func(string) bool
	visit = func(x string) bool {
		if state[x] == 1 {
			return true
		}
		if state[x] == 2 {
			return false
		}
		state[x] = 1
		for _, d := range deps[x] {
			if visit(d) {
				return true
			}
		}
		state[x] = 2
		return false
	}
	for x := range deps {
		if visit(x) {
			return true
		}
	}
	return false
}
func blockers(in Input) []Blocker {
	var out []Blocker
	for _, t := range in.Targets {
		if t.Disposition != "pending" && !(t.Disposition == "already_equivalent" && in.CompletionPolicy.AllowEquivalent) {
			out = append(out, Blocker{t.ID, t.Disposition, t.DispositionReason})
		}
	}
	return out
}
func (s *Store) path(repo, campaign string) string {
	return filepath.Join(s.root, repo, campaign+".json")
}
func (s *Store) Create(repo, actor string, in Input) (Campaign, error) {
	if repo == "" || actor == "" || !validate(in) {
		return Campaign{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x := Campaign{ID: id(), RepositoryID: repo, CreatorID: actor, Input: in, Blockers: blockers(in), CreatedAt: s.now().UTC()}
	dir := filepath.Dir(s.path(repo, x.ID))
	if e := os.MkdirAll(dir, 0750); e != nil {
		return Campaign{}, e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(repo, x.ID), b, 0640)
	}
	return x, e
}
func (s *Store) Get(repo, campaign string) (Campaign, error) {
	var x Campaign
	b, e := os.ReadFile(s.path(repo, campaign))
	if os.IsNotExist(e) {
		return x, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) List(repo string) ([]Campaign, error) {
	entries, e := os.ReadDir(filepath.Join(s.root, repo))
	if os.IsNotExist(e) {
		return []Campaign{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Campaign{}
	for _, f := range entries {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, e := s.Get(repo, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
