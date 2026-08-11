// Package contributorpathways owns immutable repository contributor guidance.
package contributorpathways

import (
	"crypto/rand"
	"encoding/hex"
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
	ErrNotFound = errors.New("contributor pathway not found")
	ErrInvalid  = errors.New("invalid contributor pathway")
	ErrConflict = errors.New("contributor pathway version conflict")
)

type Reference struct {
	Kind       string `json:"kind"`
	Label      string `json:"label"`
	URL        string `json:"url,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
	Path       string `json:"path,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Status     string `json:"status,omitempty"`
	Detail     string `json:"detail,omitempty"`
}
type WorkCategory struct {
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	SuitableFor        string   `json:"suitable_for"`
	Prerequisites      []string `json:"prerequisites"`
	ReviewExpectations string   `json:"review_expectations"`
}
type Version struct {
	Number                    int64          `json:"number"`
	Goals                     []string       `json:"goals"`
	Prerequisites             []string       `json:"prerequisites"`
	ConductGuidance           string         `json:"conduct_guidance"`
	SecurityGuidance          string         `json:"security_guidance"`
	SupportedSetup            []string       `json:"supported_setup"`
	CommunicationExpectations []string       `json:"communication_expectations"`
	ReviewPolicy              []string       `json:"review_policy"`
	WorkCategories            []WorkCategory `json:"work_categories"`
	References                []Reference    `json:"references"`
	AuthorID                  string         `json:"author_id"`
	ChangeReason              string         `json:"change_reason"`
	CreatedAt                 time.Time      `json:"created_at"`
}
type Acknowledgement struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`
	ActorID   string    `json:"actor_id"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Pathway struct {
	RepositoryID     string            `json:"repository_id"`
	CurrentVersion   int64             `json:"current_version"`
	Versions         []Version         `json:"versions"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
}
type VersionInput struct {
	Goals                     []string       `json:"goals"`
	Prerequisites             []string       `json:"prerequisites"`
	ConductGuidance           string         `json:"conduct_guidance"`
	SecurityGuidance          string         `json:"security_guidance"`
	SupportedSetup            []string       `json:"supported_setup"`
	CommunicationExpectations []string       `json:"communication_expectations"`
	ReviewPolicy              []string       `json:"review_policy"`
	WorkCategories            []WorkCategory `json:"work_categories"`
	References                []Reference    `json:"references"`
	ChangeReason              string         `json:"change_reason"`
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
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(abs, 0750); err != nil {
		return nil, err
	}
	return &Store{root: abs, now: time.Now}, nil
}
func cleanList(values []string) bool {
	if len(values) == 0 || len(values) > 30 {
		return false
	}
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
		if values[i] == "" || len(values[i]) > 2000 {
			return false
		}
	}
	return true
}
func valid(in VersionInput) bool {
	if !cleanList(in.Goals) || !cleanList(in.Prerequisites) || !cleanList(in.SupportedSetup) || !cleanList(in.CommunicationExpectations) || !cleanList(in.ReviewPolicy) || strings.TrimSpace(in.ConductGuidance) == "" || strings.TrimSpace(in.SecurityGuidance) == "" || strings.TrimSpace(in.ChangeReason) == "" || len(in.WorkCategories) == 0 || len(in.WorkCategories) > 30 || len(in.References) == 0 || len(in.References) > 100 {
		return false
	}
	for _, c := range in.WorkCategories {
		if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Description) == "" || strings.TrimSpace(c.ReviewExpectations) == "" || (c.SuitableFor != "human" && c.SuitableFor != "agent" && c.SuitableFor != "human_or_agent") {
			return false
		}
	}
	allowed := map[string]bool{"documentation": true, "ownership": true, "release": true, "issue": true, "proposal": true, "workspace_definition": true}
	for _, r := range in.References {
		if !allowed[r.Kind] || strings.TrimSpace(r.Label) == "" {
			return false
		}
		if (r.Kind == "documentation" || r.Kind == "workspace_definition") && (r.Path == "" || r.Revision == "") {
			return false
		}
		if r.Kind != "documentation" && r.Kind != "workspace_definition" && r.ResourceID == "" {
			return false
		}
	}
	return true
}
func (s *Store) Publish(repo, actor string, expected int64, in VersionInput) (Pathway, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Pathway{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repo)
	if errors.Is(err, ErrNotFound) {
		p = Pathway{RepositoryID: repo, Versions: []Version{}, Acknowledgements: []Acknowledgement{}}
	} else if err != nil {
		return Pathway{}, err
	}
	if p.CurrentVersion != expected {
		return Pathway{}, ErrConflict
	}
	now := s.now().UTC()
	v := Version{Number: p.CurrentVersion + 1, Goals: in.Goals, Prerequisites: in.Prerequisites, ConductGuidance: strings.TrimSpace(in.ConductGuidance), SecurityGuidance: strings.TrimSpace(in.SecurityGuidance), SupportedSetup: in.SupportedSetup, CommunicationExpectations: in.CommunicationExpectations, ReviewPolicy: in.ReviewPolicy, WorkCategories: in.WorkCategories, References: in.References, AuthorID: actor, ChangeReason: strings.TrimSpace(in.ChangeReason), CreatedAt: now}
	p.CurrentVersion = v.Number
	p.Versions = append(p.Versions, v)
	return p, s.write(p)
}
func (s *Store) Get(repo string) (Pathway, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo)
}
func (s *Store) Acknowledge(repo, actor string, version int64, note string) (Pathway, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repo)
	if err != nil {
		return p, err
	}
	if version < 1 || version > p.CurrentVersion {
		return p, ErrInvalid
	}
	for _, a := range p.Acknowledgements {
		if a.Version == version && a.ActorID == actor {
			return p, ErrConflict
		}
	}
	idb := make([]byte, 16)
	if _, err = rand.Read(idb); err != nil {
		return p, err
	}
	p.Acknowledgements = append(p.Acknowledgements, Acknowledgement{ID: hex.EncodeToString(idb), Version: version, ActorID: actor, Note: strings.TrimSpace(note), CreatedAt: s.now().UTC()})
	return p, s.write(p)
}
func (s *Store) read(repo string) (Pathway, error) {
	b, err := os.ReadFile(filepath.Join(s.root, repo+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Pathway{}, ErrNotFound
	}
	var p Pathway
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(b, &p)
	return p, err
}
func (s *Store) write(p Pathway) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.root, "pathway-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(s.root, p.RepositoryID+".json"))
}
