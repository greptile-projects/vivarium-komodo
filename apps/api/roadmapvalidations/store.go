// Package roadmapvalidations owns pre-commitment product validation records.
package roadmapvalidations

import (
	"crypto/rand"
	"crypto/sha256"
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

var ErrNotFound = errors.New("roadmap validation not found")
var ErrInvalid = errors.New("invalid roadmap validation")
var ErrConflict = errors.New("roadmap validation conflict")
var ErrUnauthorized = errors.New("participant credential invalid")

type Measure struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	FeedbackIDs []string `json:"feedback_ids"`
	Threshold   string   `json:"threshold"`
}
type Activity struct {
	Kind     string    `json:"kind"`
	Revision string    `json:"revision"`
	Scope    string    `json:"scope"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}
type Input struct {
	OutcomeID    string    `json:"outcome_id"`
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	Hypothesis   string    `json:"hypothesis"`
	Measures     []Measure `json:"measures"`
	Activity     Activity  `json:"activity"`
	ChangeReason string    `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Invitation struct {
	ID                 string     `json:"id"`
	ParticipantID      string     `json:"participant_id"`
	FeedbackID         string     `json:"feedback_id"`
	Version            int64      `json:"version"`
	ActivityRevision   string     `json:"activity_revision"`
	AccessibilityNeeds string     `json:"accessibility_needs,omitempty"`
	InvitedByID        string     `json:"invited_by_id"`
	CreatedAt          time.Time  `json:"created_at"`
	RevokedAt          *time.Time `json:"revoked_at,omitempty"`
	TokenDigest        string     `json:"token_digest,omitempty"`
}
type Finding struct {
	ID                 string    `json:"id"`
	InvitationID       string    `json:"invitation_id"`
	ParticipantID      string    `json:"participant_id"`
	Version            int64     `json:"version"`
	ActivityRevision   string    `json:"activity_revision"`
	Finding            string    `json:"finding"`
	AccessibilityNeeds string    `json:"accessibility_needs,omitempty"`
	Dissent            string    `json:"dissent,omitempty"`
	Acceptance         string    `json:"acceptance"`
	EvidenceValidity   string    `json:"evidence_validity"`
	CreatedAt          time.Time `json:"created_at"`
}
type Assessment struct {
	ID             string    `json:"id"`
	Version        int64     `json:"version"`
	FindingIDs     []string  `json:"finding_ids"`
	EvidenceStatus string    `json:"evidence_status"`
	Decision       string    `json:"decision"`
	Rationale      string    `json:"rationale"`
	AuthorID       string    `json:"author_id"`
	CreatedAt      time.Time `json:"created_at"`
}
type Validation struct {
	ID                   string       `json:"id"`
	RepositoryID         string       `json:"repository_id"`
	RoadmapID            string       `json:"roadmap_id"`
	RoadmapVersion       int64        `json:"roadmap_version"`
	OpportunityID        string       `json:"opportunity_id"`
	OpportunityVersion   int64        `json:"opportunity_version"`
	CurrentVersion       int64        `json:"current_version"`
	Versions             []Version    `json:"versions"`
	Invitations          []Invitation `json:"invitations"`
	Findings             []Finding    `json:"findings"`
	Assessments          []Assessment `json:"assessments"`
	OperationalAuthority bool         `json:"operational_authority"`
	CreatedAt            time.Time    `json:"created_at"`
	UpdatedAt            time.Time    `json:"updated_at"`
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
	a, _ := filepath.Abs(root)
	if e := os.MkdirAll(a, 0750); e != nil {
		return nil, e
	}
	return &Store{root: a, now: time.Now}, nil
}
func valid(in Input) bool {
	if in.OutcomeID == "" || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Hypothesis) == "" || strings.TrimSpace(in.ChangeReason) == "" || !map[string]bool{"technical_decision": true, "prototype": true, "documentation_concept": true, "product_experiment": true}[in.Kind] || len(in.Measures) < 2 {
		return false
	}
	hasSuccess, hasGuardrail := false, false
	for _, m := range in.Measures {
		if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Threshold) == "" || len(m.FeedbackIDs) == 0 || !map[string]bool{"success": true, "guardrail": true}[m.Kind] {
			return false
		}
		hasSuccess = hasSuccess || m.Kind == "success"
		hasGuardrail = hasGuardrail || m.Kind == "guardrail"
	}
	a := in.Activity
	return hasSuccess && hasGuardrail && map[string]bool{"preview": true, "research": true}[a.Kind] && a.Revision != "" && strings.TrimSpace(a.Scope) != "" && a.EndsAt.After(a.StartsAt)
}
func (s *Store) Create(repo, roadmap string, rv int64, opp string, ov int64, actor string, in Input) (Validation, error) {
	if repo == "" || roadmap == "" || rv < 1 || opp == "" || ov < 1 || actor == "" || !valid(in) {
		return Validation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.now().UTC()
	v := Validation{ID: id("validation_"), RepositoryID: repo, RoadmapID: roadmap, RoadmapVersion: rv, OpportunityID: opp, OpportunityVersion: ov, CurrentVersion: 1, Versions: []Version{{Number: 1, Input: in, AuthorID: actor, CreatedAt: n}}, Invitations: []Invitation{}, Findings: []Finding{}, Assessments: []Assessment{}, CreatedAt: n, UpdatedAt: n}
	return v, s.write(v)
}
func (s *Store) Revise(repo, idv, actor string, expected int64, in Input) (Validation, error) {
	if actor == "" || !valid(in) {
		return Validation{}, ErrInvalid
	}
	return s.mutate(repo, idv, func(v *Validation) error {
		if v.CurrentVersion != expected {
			return ErrConflict
		}
		v.CurrentVersion++
		v.Versions = append(v.Versions, Version{Number: v.CurrentVersion, Input: in, AuthorID: actor, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Invite(repo, idv, actor, participant, feedback, needs string) (Validation, string, error) {
	if actor == "" || participant == "" || feedback == "" {
		return Validation{}, "", ErrInvalid
	}
	token := id("rvp_")
	digest := sha256.Sum256([]byte(token))
	var out Validation
	e := error(nil)
	out, e = s.mutate(repo, idv, func(v *Validation) error {
		cur := v.Versions[len(v.Versions)-1]
		n := s.now().UTC()
		v.Invitations = append(v.Invitations, Invitation{ID: id("invite_"), ParticipantID: participant, FeedbackID: feedback, Version: v.CurrentVersion, ActivityRevision: cur.Activity.Revision, AccessibilityNeeds: strings.TrimSpace(needs), InvitedByID: actor, CreatedAt: n, TokenDigest: hex.EncodeToString(digest[:])})
		return nil
	})
	return out, token, e
}
func (s *Store) Participant(token string) (Validation, Invitation, error) {
	d := sha256.Sum256([]byte(token))
	want := hex.EncodeToString(d[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	repos, _ := os.ReadDir(s.root)
	for _, r := range repos {
		files, _ := os.ReadDir(filepath.Join(s.root, r.Name()))
		for _, f := range files {
			v, e := s.read(r.Name(), strings.TrimSuffix(f.Name(), ".json"))
			if e != nil {
				continue
			}
			for _, i := range v.Invitations {
				if i.TokenDigest == want && i.RevokedAt == nil {
					return v, i, nil
				}
			}
		}
	}
	return Validation{}, Invitation{}, ErrUnauthorized
}
func (s *Store) Find(token string, f Finding) (Validation, error) {
	v, i, e := s.Participant(token)
	if e != nil {
		return Validation{}, e
	}
	if strings.TrimSpace(f.Finding) == "" || !map[string]bool{"accept": true, "reject": true, "dissent": true, "uncertain": true}[f.Acceptance] || !map[string]bool{"valid": true, "invalid": true, "insufficient": true}[f.EvidenceValidity] {
		return Validation{}, ErrInvalid
	}
	return s.mutate(v.RepositoryID, v.ID, func(x *Validation) error {
		f.ID = id("finding_")
		f.InvitationID = i.ID
		f.ParticipantID = i.ParticipantID
		f.Version = i.Version
		f.ActivityRevision = i.ActivityRevision
		f.CreatedAt = s.now().UTC()
		f.AccessibilityNeeds = strings.TrimSpace(f.AccessibilityNeeds)
		x.Findings = append(x.Findings, f)
		return nil
	})
}
func (s *Store) Assess(repo, idv, actor string, a Assessment) (Validation, error) {
	if actor == "" || len(a.FindingIDs) == 0 || strings.TrimSpace(a.Rationale) == "" || !map[string]bool{"valid": true, "invalid": true, "insufficient": true}[a.EvidenceStatus] || !map[string]bool{"accept": true, "revise": true, "defer": true, "reject": true}[a.Decision] {
		return Validation{}, ErrInvalid
	}
	return s.mutate(repo, idv, func(v *Validation) error {
		known := map[string]bool{}
		for _, f := range v.Findings {
			known[f.ID] = true
		}
		for _, x := range a.FindingIDs {
			if !known[x] {
				return ErrInvalid
			}
		}
		a.ID = id("assessment_")
		a.Version = v.CurrentVersion
		a.AuthorID = actor
		a.CreatedAt = s.now().UTC()
		v.Assessments = append(v.Assessments, a)
		return nil
	})
}
func (s *Store) Get(repo, idv string) (Validation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, idv)
}
func (s *Store) List(repo string) ([]Validation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Validation{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Validation{}
	for _, x := range es {
		v, er := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
		if er == nil {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (s *Store) mutate(repo, idv string, fn func(*Validation) error) (Validation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, idv)
	if e != nil {
		return v, e
	}
	if e = fn(&v); e != nil {
		return Validation{}, e
	}
	v.UpdatedAt = s.now().UTC()
	return v, s.write(v)
}
func (s *Store) read(repo, idv string) (Validation, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, idv+".json"))
	if e != nil {
		return Validation{}, ErrNotFound
	}
	var v Validation
	if json.Unmarshal(b, &v) != nil || v.ID != idv || v.RepositoryID != repo {
		return Validation{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) write(v Validation) error {
	d := filepath.Join(s.root, v.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, ".validation-*")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Chmod(0600)
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(name, filepath.Join(d, v.ID+".json"))
	}
	return e
}
func id(p string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return p + hex.EncodeToString(b)
}
