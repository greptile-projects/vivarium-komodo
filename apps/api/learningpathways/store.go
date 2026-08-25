// Package learningpathways owns immutable project-native learning curricula.
package learningpathways

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("learning pathway not found")
	ErrInvalid  = errors.New("invalid learning pathway")
	ErrConflict = errors.New("learning pathway version conflict")
)

type Resource struct {
	Kind       string `json:"kind"`
	Label      string `json:"label"`
	URL        string `json:"url,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
	Path       string `json:"path,omitempty"`
	Symbol     string `json:"symbol,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Status     string `json:"status,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

type Exercise struct {
	Title              string   `json:"title"`
	Kinds              []string `json:"kinds"`
	Instructions       string   `json:"instructions"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	Tools              []Tool   `json:"tools"`
	Data               []Data   `json:"data"`
	SetupCommands      []string `json:"setup_commands"`
	MaximumCost        float64  `json:"maximum_cost"`
}

type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Data struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Digest string `json:"digest"`
}

type Module struct {
	ID                    string     `json:"id"`
	Title                 string     `json:"title"`
	WhyItMatters          string     `json:"why_it_matters"`
	Objectives            []string   `json:"objectives"`
	ExpectedEffortMinutes int        `json:"expected_effort_minutes"`
	Exercises             []Exercise `json:"exercises"`
	Resources             []Resource `json:"resources"`
}

type Environment struct {
	Name        string `json:"name"`
	Requirement string `json:"requirement"`
	Supported   bool   `json:"supported"`
}

type Version struct {
	Number                int64         `json:"number"`
	Role                  string        `json:"role"`
	Outcome               string        `json:"outcome"`
	Prerequisites         []string      `json:"prerequisites"`
	Objectives            []string      `json:"objectives"`
	SupportedRevisions    []string      `json:"supported_revisions"`
	Modules               []Module      `json:"modules"`
	MentorIDs             []string      `json:"mentor_ids"`
	ExpectedEffortMinutes int           `json:"expected_effort_minutes"`
	AccessibilityNeeds    []string      `json:"accessibility_needs"`
	LocalizationNeeds     []string      `json:"localization_needs"`
	LearnerEnvironments   []Environment `json:"learner_environments"`
	CompletionEvidence    []string      `json:"completion_evidence"`
	AuthorID              string        `json:"author_id"`
	ChangeReason          string        `json:"change_reason"`
	CreatedAt             time.Time     `json:"created_at"`
	Findings              []Finding     `json:"findings,omitempty"`
}

type Finding struct {
	Kind          string `json:"kind"`
	ModuleID      string `json:"module_id,omitempty"`
	ResourceLabel string `json:"resource_label,omitempty"`
	OwnerID       string `json:"owner_id,omitempty"`
	Detail        string `json:"detail"`
}

type Pathway struct {
	RepositoryID   string    `json:"repository_id"`
	ID             string    `json:"id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
}
type VersionInput struct {
	Role                  string        `json:"role"`
	Outcome               string        `json:"outcome"`
	Prerequisites         []string      `json:"prerequisites"`
	Objectives            []string      `json:"objectives"`
	SupportedRevisions    []string      `json:"supported_revisions"`
	Modules               []Module      `json:"modules"`
	MentorIDs             []string      `json:"mentor_ids"`
	ExpectedEffortMinutes int           `json:"expected_effort_minutes"`
	AccessibilityNeeds    []string      `json:"accessibility_needs"`
	LocalizationNeeds     []string      `json:"localization_needs"`
	LearnerEnvironments   []Environment `json:"learner_environments"`
	CompletionEvidence    []string      `json:"completion_evidence"`
	ChangeReason          string        `json:"change_reason"`
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
	abs, e := filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(abs, 0750); e != nil {
		return nil, e
	}
	return &Store{root: abs, now: time.Now}, nil
}
func clean(values []string, required bool) bool {
	if (required && len(values) == 0) || len(values) > 50 {
		return false
	}
	for _, v := range values {
		if strings.TrimSpace(v) == "" || len(v) > 2000 {
			return false
		}
	}
	return true
}
func safeID(value string) bool {
	if value == "" || len(value) > 100 {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}
func valid(in VersionInput) bool {
	if strings.TrimSpace(in.Role) == "" || strings.TrimSpace(in.Outcome) == "" || strings.TrimSpace(in.ChangeReason) == "" || !clean(in.Prerequisites, true) || !clean(in.Objectives, true) || !clean(in.SupportedRevisions, true) || !clean(in.CompletionEvidence, true) || !clean(in.AccessibilityNeeds, false) || !clean(in.LocalizationNeeds, false) || in.ExpectedEffortMinutes < 1 || in.ExpectedEffortMinutes > 100000 || len(in.Modules) == 0 || len(in.Modules) > 50 || len(in.LearnerEnvironments) == 0 {
		return false
	}
	ids := map[string]bool{}
	allowed := map[string]bool{"documentation": true, "symbol": true, "decision": true, "issue": true, "api": true, "package": true, "contributor_guidance": true}
	for _, m := range in.Modules {
		if !safeID(m.ID) || ids[m.ID] || m.Title == "" || m.WhyItMatters == "" || !clean(m.Objectives, true) || m.ExpectedEffortMinutes < 1 || len(m.Resources) == 0 || len(m.Exercises) == 0 {
			return false
		}
		ids[m.ID] = true
		for _, e := range m.Exercises {
			if e.Title == "" || e.Instructions == "" || !clean(e.AcceptanceCriteria, true) || !clean(e.Kinds, false) || !clean(e.SetupCommands, false) || e.MaximumCost < 0 || e.MaximumCost > 10000 || len(e.Tools) > 30 || len(e.Data) > 30 {
				return false
			}
			for _, tool := range e.Tools {
				if strings.TrimSpace(tool.Name) == "" || strings.TrimSpace(tool.Version) == "" {
					return false
				}
			}
			for _, data := range e.Data {
				if strings.TrimSpace(data.Name) == "" || (data.Kind != "synthetic" && data.Kind != "permitted") || strings.TrimSpace(data.Digest) == "" {
					return false
				}
			}
		}
		for _, r := range m.Resources {
			if !allowed[r.Kind] || r.Label == "" || r.Revision == "" {
				return false
			}
			if (r.Kind == "documentation" || r.Kind == "symbol" || r.Kind == "contributor_guidance") && r.Path == "" {
				return false
			}
			if r.Kind != "documentation" && r.Kind != "symbol" && r.Kind != "contributor_guidance" && r.ResourceID == "" {
				return false
			}
		}
	}
	for _, e := range in.LearnerEnvironments {
		if e.Name == "" || e.Requirement == "" {
			return false
		}
	}
	return true
}
func (s *Store) Publish(repo, id, actor string, expected int64, in VersionInput) (Pathway, error) {
	if repo == "" || !safeID(id) || actor == "" || !valid(in) {
		return Pathway{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, id)
	if errors.Is(e, ErrNotFound) {
		p = Pathway{RepositoryID: repo, ID: id, Versions: []Version{}}
	} else if e != nil {
		return p, e
	}
	if p.CurrentVersion != expected {
		return p, ErrConflict
	}
	v := Version{Number: p.CurrentVersion + 1, Role: strings.TrimSpace(in.Role), Outcome: strings.TrimSpace(in.Outcome), Prerequisites: in.Prerequisites, Objectives: in.Objectives, SupportedRevisions: in.SupportedRevisions, Modules: in.Modules, MentorIDs: in.MentorIDs, ExpectedEffortMinutes: in.ExpectedEffortMinutes, AccessibilityNeeds: in.AccessibilityNeeds, LocalizationNeeds: in.LocalizationNeeds, LearnerEnvironments: in.LearnerEnvironments, CompletionEvidence: in.CompletionEvidence, AuthorID: actor, ChangeReason: strings.TrimSpace(in.ChangeReason), CreatedAt: s.now().UTC()}
	p.CurrentVersion = v.Number
	p.Versions = append(p.Versions, v)
	return p, s.write(p)
}
func (s *Store) Get(repo, id string) (Pathway, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Pathway, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Pathway{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Pathway{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		p, e := s.read(repo, strings.TrimSuffix(entry.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, nil
}
func (s *Store) read(repo, id string) (Pathway, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Pathway{}, ErrNotFound
	}
	var p Pathway
	if e == nil {
		e = json.Unmarshal(b, &p)
	}
	return p, e
}
func (s *Store) write(p Pathway) error {
	dir := filepath.Join(s.root, p.RepositoryID)
	if e := os.MkdirAll(dir, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(dir, "pathway-*.tmp")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, filepath.Join(dir, p.ID+".json"))
}
