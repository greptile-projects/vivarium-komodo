// Package accessibilitybarriers retains consent-bounded lived accessibility evidence and reproductions.
package accessibilitybarriers

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

var ErrNotFound = errors.New("accessibility barrier not found")
var ErrInvalid = errors.New("invalid accessibility barrier")

type Context struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Path       string `json:"path,omitempty"`
	Revision   string `json:"revision"`
}
type Evidence struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	MediaType  string `json:"media_type"`
	Content    string `json:"content,omitempty"`
	Visibility string `json:"visibility"`
	Redacted   bool   `json:"redacted"`
}
type Environment struct {
	Browser             string `json:"browser"`
	DeviceClass         string `json:"device_class"`
	AssistiveTechnology string `json:"assistive_technology"`
	Locale              string `json:"locale,omitempty"`
	SensitiveDeviceData string `json:"sensitive_device_data,omitempty"`
}
type Input struct {
	Context              Context     `json:"context"`
	AccessNeeds          string      `json:"access_needs"`
	ExpectedOutcome      string      `json:"expected_outcome"`
	Steps                []string    `json:"interaction_steps"`
	Environment          Environment `json:"environment"`
	IdentityVisibility   string      `json:"identity_visibility"`
	DeviceDataVisibility string      `json:"device_data_visibility"`
	Evidence             []Evidence  `json:"evidence"`
}
type AttemptInput struct {
	ExecutionKind string      `json:"execution_kind"`
	ExecutionID   string      `json:"execution_id"`
	Revision      string      `json:"revision"`
	Environment   Environment `json:"environment"`
	Result        string      `json:"result"`
	Notes         string      `json:"notes"`
	Evidence      []Evidence  `json:"evidence"`
}
type Attempt struct {
	ID string `json:"id"`
	AttemptInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Barrier struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	ReporterID   string `json:"reporter_id,omitempty"`
	Input
	Attempts  []Attempt `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	root, _ = filepath.Abs(root)
	if e := os.MkdirAll(root, 0750); e != nil {
		return nil, e
	}
	return &Store{root: root, now: time.Now}, nil
}
func id() string                { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func text(v string, n int) bool { return strings.TrimSpace(v) != "" && len(v) <= n }
func validEvidence(es []Evidence) bool {
	if len(es) > 10 {
		return false
	}
	total := 0
	for _, e := range es {
		total += len(e.Content)
		if !e.Redacted || !map[string]bool{"screenshot": true, "recording": true, "accessibility_tree": true, "speech_output": true, "input_trace": true}[e.Kind] || !map[string]bool{"audience": true, "maintainers": true}[e.Visibility] || !text(e.Name, 200) || !text(e.MediaType, 100) || e.Content == "" || len(e.Content) > 1<<20 {
			return false
		}
	}
	return total <= 5<<20
}
func validEnvironment(e Environment) bool {
	return text(e.Browser, 200) && text(e.DeviceClass, 200) && text(e.AssistiveTechnology, 300) && len(e.Locale) <= 100 && len(e.SensitiveDeviceData) <= 2000
}
func validInput(in Input) bool {
	if !map[string]bool{"release": true, "page": true, "journey": true, "preview": true}[in.Context.Kind] || in.Context.ResourceID == "" || in.Context.Revision == "" || !text(in.AccessNeeds, 65536) || !text(in.ExpectedOutcome, 65536) || len(in.Steps) == 0 || len(in.Steps) > 100 || !validEnvironment(in.Environment) || !map[string]bool{"audience": true, "maintainers": true}[in.IdentityVisibility] || !map[string]bool{"audience": true, "maintainers": true}[in.DeviceDataVisibility] || !validEvidence(in.Evidence) {
		return false
	}
	for _, x := range in.Steps {
		if !text(x, 4000) {
			return false
		}
	}
	return true
}
func (s *Store) Create(repo, actor string, in Input) (Barrier, error) {
	if repo == "" || actor == "" || !validInput(in) {
		return Barrier{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	for i := range in.Evidence {
		in.Evidence[i].ID = id()
	}
	v := Barrier{ID: id(), RepositoryID: repo, ReporterID: actor, Input: in, Attempts: []Attempt{}, CreatedAt: now, UpdatedAt: now}
	return v, s.write(v)
}
func (s *Store) AddAttempt(repo, bid, actor string, in AttemptInput) (Barrier, error) {
	if actor == "" || !map[string]bool{"workspace": true, "preview": true}[in.ExecutionKind] || in.ExecutionID == "" || in.Revision == "" || !validEnvironment(in.Environment) || !map[string]bool{"reproducible": true, "intermittent": true, "environment_specific": true, "unconfirmed": true}[in.Result] || !text(in.Notes, 65536) || !validEvidence(in.Evidence) {
		return Barrier{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, bid)
	if e != nil {
		return v, e
	}
	for i := range in.Evidence {
		in.Evidence[i].ID = id()
	}
	now := s.now().UTC()
	v.Attempts = append(v.Attempts, Attempt{ID: id(), AttemptInput: in, ActorID: actor, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) Get(repo, bid string) (Barrier, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, bid)
}
func (s *Store) List(repo string) ([]Barrier, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Barrier{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Barrier{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			v, er := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
			if er != nil {
				return nil, er
			}
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) read(repo, bid string) (Barrier, error) {
	var v Barrier
	b, e := os.ReadFile(filepath.Join(s.root, repo, bid+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e != nil || json.Unmarshal(b, &v) != nil || v.ID != bid || v.RepositoryID != repo {
		return Barrier{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) write(v Barrier) error {
	d := filepath.Join(s.root, v.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(d, v.ID+".json"), b, 0600)
}
