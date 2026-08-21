// Package adoptionworkspaces owns evidence-backed software fit evaluations.
package adoptionworkspaces

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

var ErrNotFound = errors.New("adoption workspace not found")
var ErrInvalid = errors.New("invalid adoption workspace")
var ErrForbidden = errors.New("adoption workspace action forbidden")

var originKinds = map[string]bool{"roadmap_outcome": true, "support_gap": true, "incubator": true, "decision": true, "package": true, "api": true, "federated_repository": true}
var dimensions = map[string]bool{"capability": true, "provenance": true, "support": true, "security": true, "data_use": true, "compatibility": true, "gap": true}

type Origin struct {
	Kind         string `json:"kind"`
	ResourceID   string `json:"resource_id"`
	Revision     string `json:"revision,omitempty"`
	RepositoryID string `json:"repository_id,omitempty"`
	Label        string `json:"label,omitempty"`
}
type Environment struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Version  string `json:"version,omitempty"`
}
type Criterion struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}
type Input struct {
	Title              string        `json:"title"`
	Outcome            string        `json:"outcome"`
	Origin             Origin        `json:"origin"`
	RequiredJourneys   []string      `json:"required_journeys"`
	Environments       []Environment `json:"environments"`
	Constraints        []string      `json:"constraints"`
	Budget             string        `json:"budget"`
	OwnerIDs           []string      `json:"owner_ids"`
	EvaluationCriteria []Criterion   `json:"evaluation_criteria"`
	Visibility         string        `json:"visibility"`
}
type Participant struct {
	ID             string     `json:"id"`
	Kind           string     `json:"kind"`
	SubjectID      string     `json:"subject_id"`
	Role           string     `json:"role"`
	EvidenceAccess string     `json:"evidence_access"`
	Consent        string     `json:"consent"`
	InvitedByID    string     `json:"invited_by_id"`
	InvitedAt      time.Time  `json:"invited_at"`
	RespondedAt    *time.Time `json:"responded_at,omitempty"`
}
type Evidence struct {
	ID           string     `json:"id"`
	Dimension    string     `json:"dimension"`
	Claim        string     `json:"claim"`
	Reference    string     `json:"reference,omitempty"`
	Revision     string     `json:"revision"`
	Visibility   string     `json:"visibility"`
	Availability string     `json:"availability"`
	ValidUntil   *time.Time `json:"valid_until,omitempty"`
	AddedByID    string     `json:"added_by_id"`
	CreatedAt    time.Time  `json:"created_at"`
	Status       string     `json:"status"`
	ProofOfFit   bool       `json:"proof_of_fit"`
	Gap          string     `json:"gap,omitempty"`
}
type Candidate struct {
	ID                 string            `json:"id"`
	Project            string            `json:"project"`
	ProviderRepository string            `json:"provider_repository"`
	Version            string            `json:"version"`
	Revision           string            `json:"revision"`
	Evidence           []Evidence        `json:"evidence"`
	Coverage           map[string]string `json:"coverage"`
	Blockers           []string          `json:"blockers"`
	AddedByID          string            `json:"added_by_id"`
	CreatedAt          time.Time         `json:"created_at"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	SubjectID string    `json:"subject_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Workspace struct {
	ID string `json:"id"`
	Input
	CreatedByID      string        `json:"created_by_id"`
	Participants     []Participant `json:"participants"`
	Candidates       []Candidate   `json:"candidates"`
	History          []Event       `json:"history"`
	AuthorityGranted bool          `json:"authority_granted"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
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
func id(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}
func listOK(xs []string, required bool) bool {
	if (required && len(xs) == 0) || len(xs) > 50 {
		return false
	}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || len(x) > 2000 {
			return false
		}
	}
	return true
}
func validInput(v Input) bool {
	if v.Title == "" || v.Outcome == "" || !originKinds[v.Origin.Kind] || v.Origin.ResourceID == "" || v.Budget == "" || !listOK(v.RequiredJourneys, true) || !listOK(v.Constraints, false) || !listOK(v.OwnerIDs, true) || (v.Visibility != "public" && v.Visibility != "participants") || len(v.Environments) == 0 || len(v.EvaluationCriteria) == 0 {
		return false
	}
	for _, e := range v.Environments {
		if e.Name == "" || e.Platform == "" {
			return false
		}
	}
	for _, c := range v.EvaluationCriteria {
		if c.ID == "" || c.Description == "" {
			return false
		}
	}
	return true
}
func (s *Store) Create(actor string, in Input) (Workspace, error) {
	if actor == "" || !validInput(in) {
		return Workspace{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	v := Workspace{ID: id("adp_"), Input: in, CreatedByID: actor, Participants: []Participant{{ID: id("par_"), Kind: "human", SubjectID: actor, Role: "adopter_owner", EvidenceAccess: "all", Consent: "accepted", InvitedByID: actor, InvitedAt: now, RespondedAt: &now}}, Candidates: []Candidate{}, History: []Event{}, AuthorityGranted: false, CreatedAt: now, UpdatedAt: now}
	v.event("workspace.opened", actor, "")
	return v, s.write(v)
}
func canRead(v Workspace, actor string) bool {
	if v.Visibility == "public" {
		return true
	}
	for _, p := range v.Participants {
		if p.SubjectID == actor && p.Consent == "accepted" {
			return true
		}
	}
	return false
}
func canContribute(v Workspace, actor string) bool {
	for _, p := range v.Participants {
		if p.SubjectID == actor && p.Consent == "accepted" && p.Kind == "human" {
			return true
		}
	}
	return false
}
func isOwner(v Workspace, actor string) bool {
	if v.CreatedByID == actor {
		return true
	}
	for _, x := range v.OwnerIDs {
		if x == actor {
			return true
		}
	}
	return false
}
func (s *Store) Invite(wid, actor string, p Participant) (Workspace, error) {
	return s.mutate(wid, actor, false, func(v *Workspace) error {
		if !isOwner(*v, actor) {
			return ErrForbidden
		}
		if p.SubjectID == "" || !map[string]bool{"provider_maintainer": true, "affected_user": true, "read_only_agent": true}[p.Role] {
			return ErrInvalid
		}
		if p.Role == "read_only_agent" && p.Kind != "agent" || p.Role != "read_only_agent" && p.Kind != "human" {
			return ErrInvalid
		}
		for _, x := range v.Participants {
			if x.SubjectID == p.SubjectID {
				return ErrInvalid
			}
		}
		now := s.now().UTC()
		p.ID = id("par_")
		p.InvitedByID = actor
		p.InvitedAt = now
		p.Consent = "pending"
		if p.EvidenceAccess == "" {
			p.EvidenceAccess = "shared"
		}
		if !map[string]bool{"shared": true, "provider": true}[p.EvidenceAccess] {
			return ErrInvalid
		}
		if p.Kind == "agent" {
			p.Consent = "accepted"
			p.RespondedAt = &now
		}
		v.Participants = append(v.Participants, p)
		v.event("participant.invited", actor, p.ID)
		return nil
	})
}
func (s *Store) Consent(wid, pid, actor, decision string) (Workspace, error) {
	return s.mutate(wid, actor, false, func(v *Workspace) error {
		if decision != "accepted" && decision != "declined" {
			return ErrInvalid
		}
		for i := range v.Participants {
			p := &v.Participants[i]
			if p.ID == pid && p.SubjectID == actor && p.Kind == "human" && p.Consent == "pending" {
				now := s.now().UTC()
				p.Consent = decision
				p.RespondedAt = &now
				v.event("participant."+decision, actor, p.ID)
				return nil
			}
		}
		return ErrForbidden
	})
}
func (s *Store) AddCandidate(wid, actor string, c Candidate) (Workspace, error) {
	return s.mutate(wid, actor, true, func(v *Workspace) error {
		if c.Project == "" || c.Version == "" || c.Revision == "" || c.ProviderRepository == "" {
			return ErrInvalid
		}
		c.ID = id("can_")
		c.AddedByID = actor
		c.CreatedAt = s.now().UTC()
		c.Evidence = []Evidence{}
		v.Candidates = append(v.Candidates, c)
		v.event("candidate.added", actor, c.ID)
		return nil
	})
}
func (s *Store) AddEvidence(wid, cid, actor string, e Evidence) (Workspace, error) {
	return s.mutate(wid, actor, true, func(v *Workspace) error {
		if !dimensions[e.Dimension] || e.Claim == "" || e.Revision == "" || !map[string]bool{"public": true, "shared": true, "provider": true}[e.Visibility] || !map[string]bool{"available": true, "unavailable": true}[e.Availability] || (e.Availability == "available" && e.Reference == "") {
			return ErrInvalid
		}
		for i := range v.Candidates {
			if v.Candidates[i].ID == cid {
				e.ID = id("evd_")
				e.AddedByID = actor
				e.CreatedAt = s.now().UTC()
				e.Status = "current"
				e.ProofOfFit = true
				v.Candidates[i].Evidence = append(v.Candidates[i].Evidence, e)
				v.event("evidence.added", actor, e.ID)
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) List(actor string) ([]Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.list()
	if e != nil {
		return nil, e
	}
	out := []Workspace{}
	for _, v := range all {
		if canRead(v, actor) {
			out = append(out, s.project(v, actor))
		}
	}
	return out, nil
}
func (s *Store) Get(wid, actor string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(wid)
	if e == nil && !canRead(v, actor) {
		return Workspace{}, ErrNotFound
	}
	if e == nil {
		v = s.project(v, actor)
	}
	return v, e
}
func (s *Store) project(v Workspace, actor string) Workspace {
	provider := false
	participant := false
	for _, p := range v.Participants {
		if p.SubjectID == actor && p.Consent == "accepted" {
			participant = true
			if p.EvidenceAccess == "provider" {
				provider = true
			}
		}
	}
	now := s.now().UTC()
	for ci := range v.Candidates {
		c := &v.Candidates[ci]
		c.Coverage = map[string]string{}
		c.Blockers = []string{}
		for d := range dimensions {
			c.Coverage[d] = "missing"
		}
		for ei := range c.Evidence {
			e := &c.Evidence[ei]
			e.Status = "current"
			e.ProofOfFit = true
			e.Gap = ""
			if e.Dimension == "gap" {
				e.Status = "known_gap"
				e.ProofOfFit = false
				e.Gap = e.Claim
			} else if e.Availability == "unavailable" {
				e.Status = "unavailable"
				e.ProofOfFit = false
				e.Reference = ""
				e.Gap = "evidence unavailable"
			} else if e.Revision != c.Revision {
				e.Status = "stale"
				e.ProofOfFit = false
				e.Gap = "evidence is for a different candidate revision"
			} else if e.ValidUntil != nil && !e.ValidUntil.After(now) {
				e.Status = "expired"
				e.ProofOfFit = false
				e.Gap = "evidence validity expired"
			} else if e.Visibility == "shared" && !participant {
				e.Status = "inaccessible"
				e.ProofOfFit = false
				e.Reference = ""
				e.Gap = "workspace evidence is not accessible to this viewer"
			} else if e.Visibility == "provider" && !provider {
				e.Status = "inaccessible"
				e.ProofOfFit = false
				e.Reference = ""
				e.Gap = "provider evidence is not accessible to this viewer"
			}
			if e.ProofOfFit {
				c.Coverage[e.Dimension] = "supported"
			} else if c.Coverage[e.Dimension] != "supported" {
				c.Coverage[e.Dimension] = e.Status
			}
			if !e.ProofOfFit {
				c.Blockers = append(c.Blockers, e.Dimension+": "+e.Gap)
			}
		}
		for d, status := range c.Coverage {
			if status == "missing" {
				c.Blockers = append(c.Blockers, d+": no evidence")
			}
		}
		sort.Strings(c.Blockers)
	}
	return v
}
func (s *Store) mutate(wid, actor string, contribute bool, fn func(*Workspace) error) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(wid)
	if e != nil {
		return v, e
	}
	if !canRead(v, actor) || (contribute && !canContribute(v, actor)) {
		return Workspace{}, ErrForbidden
	}
	if e = fn(&v); e != nil {
		return v, e
	}
	v.UpdatedAt = s.now().UTC()
	return v, s.write(v)
}
func (v *Workspace) event(t, a, subject string) {
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: t, ActorID: a, SubjectID: subject, CreatedAt: time.Now().UTC()})
}
func (s *Store) path(wid string) string { return filepath.Join(s.root, wid+".json") }
func (s *Store) write(v Workspace) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := s.path(v.ID) + ".tmp"
	if e = os.WriteFile(tmp, b, 0640); e == nil {
		e = os.Rename(tmp, s.path(v.ID))
	}
	return e
}
func (s *Store) read(wid string) (Workspace, error) {
	var v Workspace
	b, e := os.ReadFile(s.path(wid))
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) list() ([]Workspace, error) {
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Workspace{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			v, er := s.read(strings.TrimSuffix(x.Name(), ".json"))
			if er != nil {
				return nil, er
			}
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
