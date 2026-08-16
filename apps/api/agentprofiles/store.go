// Package agentprofiles owns public, versioned agent identity disclosures.
package agentprofiles

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("agent profile not found")
var ErrInvalid = errors.New("invalid agent profile")
var ErrConflict = errors.New("agent profile version conflict")
var ErrIdentityTaken = errors.New("agent identity taken")
var handleRE = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?$`)

type Model struct {
	Provider       string `json:"provider"`
	Name           string `json:"name"`
	Version        string `json:"version"`
	TrainingCutoff string `json:"training_cutoff,omitempty"`
}
type Execution struct {
	Runtime   string   `json:"runtime"`
	Regions   []string `json:"regions"`
	Remote    bool     `json:"remote"`
	Boundary  string   `json:"boundary"`
	Isolation string   `json:"isolation"`
}
type DataTerms struct {
	ContextUsed     []string `json:"context_used"`
	Purposes        []string `json:"purposes"`
	Retention       string   `json:"retention"`
	TrainingUse     string   `json:"training_use"`
	DeletionProcess string   `json:"deletion_process"`
}
type Subprocessor struct {
	Name     string   `json:"name"`
	Purpose  string   `json:"purpose"`
	Location string   `json:"location"`
	Data     []string `json:"data"`
}
type Pricing struct {
	Model        string   `json:"model"`
	Currency     string   `json:"currency,omitempty"`
	Amount       float64  `json:"amount,omitempty"`
	Unit         string   `json:"unit"`
	Requirements []string `json:"resource_requirements"`
}
type Support struct {
	Contact        string `json:"contact"`
	Hours          string `json:"hours"`
	ResponseTarget string `json:"response_target"`
	StatusURL      string `json:"status_url,omitempty"`
}
type Input struct {
	DisplayName           string         `json:"display_name"`
	Summary               string         `json:"summary"`
	Ownership             string         `json:"ownership"`
	SupportedTasks        []string       `json:"supported_tasks"`
	Tools                 []string       `json:"tools"`
	Models                []Model        `json:"models"`
	Execution             Execution      `json:"execution"`
	DataUse               DataTerms      `json:"data_use"`
	Subprocessors         []Subprocessor `json:"subprocessors"`
	Pricing               Pricing        `json:"pricing"`
	RequestedCapabilities []string       `json:"requested_capabilities"`
	Availability          string         `json:"availability"`
	Support               Support        `json:"support"`
	ChangeReason          string         `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	PublishedBy string    `json:"published_by"`
	PublishedAt time.Time `json:"published_at"`
}
type Evidence struct {
	Kind       string    `json:"kind"`
	Status     string    `json:"status"`
	Subject    string    `json:"subject"`
	Detail     string    `json:"detail"`
	VerifiedAt time.Time `json:"verified_at"`
}
type Profile struct {
	ID                       string     `json:"id"`
	Handle                   string     `json:"handle"`
	OperatorID               string     `json:"operator_id"`
	CurrentVersion           int64      `json:"current_version"`
	Versions                 []Version  `json:"versions"`
	PlatformVerifiedEvidence []Evidence `json:"platform_verified_evidence"`
	OperatorClaimsVerified   bool       `json:"operator_claims_verified"`
	AuthorityGranted         bool       `json:"authority_granted"`
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
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return "agt_" + hex.EncodeToString(b[:]) }
func listOK(v []string, required bool) bool {
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
func valid(in Input) bool {
	if in.DisplayName == "" || in.Summary == "" || in.Ownership == "" || in.ChangeReason == "" || !listOK(in.SupportedTasks, true) || !listOK(in.Tools, true) || !listOK(in.RequestedCapabilities, false) || len(in.Models) == 0 {
		return false
	}
	for _, m := range in.Models {
		if m.Provider == "" || m.Name == "" || m.Version == "" {
			return false
		}
	}
	if in.Execution.Runtime == "" || in.Execution.Boundary == "" || in.Execution.Isolation == "" || !listOK(in.Execution.Regions, true) || !listOK(in.DataUse.ContextUsed, true) || !listOK(in.DataUse.Purposes, true) || in.DataUse.Retention == "" || in.DataUse.TrainingUse == "" || in.DataUse.DeletionProcess == "" || in.Pricing.Model == "" || in.Pricing.Unit == "" || in.Pricing.Amount < 0 || in.Availability == "" || in.Support.Contact == "" || in.Support.Hours == "" || in.Support.ResponseTarget == "" {
		return false
	}
	for _, s := range in.Subprocessors {
		if s.Name == "" || s.Purpose == "" || s.Location == "" || !listOK(s.Data, true) {
			return false
		}
	}
	return true
}
func (s *Store) Create(operator, handle string, in Input, identityAvailable func(string) bool) (Profile, error) {
	handle = strings.ToLower(strings.TrimSpace(handle))
	if !handleRE.MatchString(handle) || strings.HasPrefix(handle, "user-") || strings.HasPrefix(handle, "installation-") || strings.HasPrefix(handle, "agent-") || !valid(in) {
		return Profile{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !identityAvailable(handle) {
		return Profile{}, ErrIdentityTaken
	}
	all, _ := s.list()
	for _, p := range all {
		if p.Handle == handle {
			return Profile{}, ErrIdentityTaken
		}
	}
	now := s.now().UTC()
	p := Profile{ID: id(), Handle: handle, OperatorID: operator, CurrentVersion: 1, Versions: []Version{{Number: 1, Input: in, PublishedBy: operator, PublishedAt: now}}, PlatformVerifiedEvidence: []Evidence{{Kind: "authenticated_operator_control", Status: "verified", Subject: operator, Detail: "Published using an authenticated platform identity; ownership and capability statements remain operator claims.", VerifiedAt: now}, {Kind: "profile_schema_validation", Status: "verified", Subject: handle, Detail: "Required provenance, execution, data-use, pricing, availability, and support disclosures were present at publication.", VerifiedAt: now}}, OperatorClaimsVerified: false, AuthorityGranted: false}
	return p, s.write(p)
}
func (s *Store) Revise(pid, operator string, expected int64, in Input) (Profile, error) {
	if !valid(in) {
		return Profile{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(pid)
	if e != nil {
		return Profile{}, e
	}
	if p.OperatorID != operator {
		return Profile{}, ErrNotFound
	}
	if p.CurrentVersion != expected {
		return Profile{}, ErrConflict
	}
	p.CurrentVersion++
	p.Versions = append(p.Versions, Version{Number: p.CurrentVersion, Input: in, PublishedBy: operator, PublishedAt: s.now().UTC()})
	return p, s.write(p)
}
func (s *Store) Get(pid string) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(pid)
}
func (s *Store) List() ([]Profile, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.list() }
func (s *Store) list() ([]Profile, error) {
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Profile{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		p, e := s.read(strings.TrimSuffix(x.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Handle < out[j].Handle })
	return out, nil
}
func (s *Store) read(pid string) (Profile, error) {
	var p Profile
	b, e := os.ReadFile(filepath.Join(s.root, pid+".json"))
	if os.IsNotExist(e) {
		return p, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &p)
	}
	return p, e
}
func (s *Store) write(p Profile) error {
	b, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	tmp := filepath.Join(s.root, p.ID+".tmp")
	if e = os.WriteFile(tmp, b, 0640); e == nil {
		e = os.Rename(tmp, filepath.Join(s.root, p.ID+".json"))
	}
	return e
}
