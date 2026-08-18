// Package designproposals owns revision-bound product behavior proposals and their review artifacts.
package designproposals

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

var ErrNotFound = errors.New("design proposal not found")
var ErrInvalid = errors.New("invalid design proposal")
var ErrConflict = errors.New("design proposal changed")

type Origin struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Revision string `json:"revision,omitempty"`
}
type Journey struct {
	Name    string   `json:"name"`
	Steps   []string `json:"steps"`
	Outcome string   `json:"outcome"`
}
type State struct {
	Name     string `json:"name"`
	Trigger  string `json:"trigger"`
	Behavior string `json:"behavior"`
	Content  string `json:"content"`
}
type Constraint struct {
	Kind        string `json:"kind"`
	Requirement string `json:"requirement"`
}
type Alternative struct {
	Name     string `json:"name"`
	Tradeoff string `json:"tradeoff"`
	Reason   string `json:"reason"`
}
type Measure struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}
type Evidence struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision,omitempty"`
	Summary   string `json:"summary"`
	Audience  string `json:"audience"`
}
type Input struct {
	Title              string        `json:"title"`
	Origin             Origin        `json:"origin"`
	UserGoal           string        `json:"user_goal"`
	Journeys           []Journey     `json:"journeys"`
	States             []State       `json:"states"`
	Content            []string      `json:"content"`
	Constraints        []Constraint  `json:"constraints"`
	Alternatives       []Alternative `json:"alternatives"`
	SuccessMeasures    []Measure     `json:"success_measures"`
	AffectedComponents []string      `json:"affected_components"`
	Evidence           []Evidence    `json:"evidence"`
	Uncertainty        []string      `json:"uncertainty"`
	ChangeReason       string        `json:"change_reason"`
}
type Revision struct {
	Number int64 `json:"number"`
	Input
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Participant struct {
	ID                  string    `json:"id"`
	SubjectID           string    `json:"subject_id"`
	Kind                string    `json:"kind"`
	Role                string    `json:"role"`
	InvitedBy           string    `json:"invited_by"`
	GroundedEvidenceIDs []string  `json:"grounded_evidence_ids,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}
type Frame struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Format      string `json:"format"`
	Body        string `json:"body"`
}
type Interaction struct {
	Trigger string `json:"trigger"`
	Action  string `json:"action"`
	Result  string `json:"result"`
}
type ArtifactInput struct {
	Kind             string        `json:"kind"`
	Title            string        `json:"title"`
	ProposalRevision int64         `json:"proposal_revision"`
	Frames           []Frame       `json:"frames"`
	Interactions     []Interaction `json:"interactions"`
	EvidenceIDs      []string      `json:"evidence_ids"`
	Uncertainty      []string      `json:"uncertainty"`
	ChangeReason     string        `json:"change_reason"`
}
type ArtifactRevision struct {
	Number int64 `json:"number"`
	ArtifactInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Artifact struct {
	ID             string             `json:"id"`
	CurrentVersion int64              `json:"current_version"`
	Revisions      []ArtifactRevision `json:"revisions"`
}
type Comment struct {
	ID              string    `json:"id"`
	AuthorID        string    `json:"author_id"`
	SubjectKind     string    `json:"subject_kind"`
	SubjectID       string    `json:"subject_id"`
	Body            string    `json:"body"`
	Stance          string    `json:"stance"`
	SubjectRevision int64     `json:"subject_revision"`
	EvidenceIDs     []string  `json:"evidence_ids"`
	Uncertainty     string    `json:"uncertainty,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
type Acknowledgement struct {
	ID               string     `json:"id"`
	OwnerID          string     `json:"owner_id"`
	RequestedBy      string     `json:"requested_by"`
	Status           string     `json:"status"`
	Rationale        string     `json:"rationale,omitempty"`
	ProposalRevision int64      `json:"proposal_revision"`
	RequestedAt      time.Time  `json:"requested_at"`
	RespondedAt      *time.Time `json:"responded_at,omitempty"`
	Current          bool       `json:"current"`
}
type Proposal struct {
	ID                   string            `json:"id"`
	RepositoryID         string            `json:"repository_id"`
	CurrentRevision      int64             `json:"current_revision"`
	Revisions            []Revision        `json:"revisions"`
	Participants         []Participant     `json:"participants"`
	Artifacts            []Artifact        `json:"artifacts"`
	Comments             []Comment         `json:"comments"`
	Acknowledgements     []Acknowledgement `json:"acknowledgements"`
	PrivateEvidenceCount int               `json:"private_evidence_count"`
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
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, now: time.Now}, e
}
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func validInput(in Input) bool {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.UserGoal) == "" || in.ChangeReason == "" || in.Origin.ID == "" || !map[string]bool{"feedback": true, "issue": true, "roadmap_outcome": true, "accessibility_finding": true, "pull_request": true}[in.Origin.Kind] || len(in.Journeys) == 0 || len(in.States) == 0 || len(in.Content) == 0 || len(in.Constraints) == 0 || len(in.Alternatives) == 0 || len(in.SuccessMeasures) == 0 || len(in.AffectedComponents) == 0 {
		return false
	}
	for _, x := range in.Journeys {
		if x.Name == "" || len(x.Steps) == 0 || x.Outcome == "" {
			return false
		}
	}
	for _, x := range in.States {
		if x.Name == "" || x.Behavior == "" || x.Content == "" {
			return false
		}
	}
	for _, x := range in.Constraints {
		if !map[string]bool{"accessibility": true, "technical": true, "privacy": true, "localization": true, "policy": true, "business": true}[x.Kind] || x.Requirement == "" {
			return false
		}
	}
	for _, x := range in.Evidence {
		if x.ID == "" || x.Reference == "" || x.Summary == "" || !map[string]bool{"public": true, "repository": true, "private_research": true, "inaccessible_asset": true}[x.Audience] {
			return false
		}
	}
	return true
}
func (s *Store) path(repo, pid string) string { return filepath.Join(s.root, repo, pid+".json") }
func (s *Store) save(p Proposal) error {
	if e := os.MkdirAll(filepath.Dir(s.path(p.RepositoryID, p.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(p, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(p.RepositoryID, p.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) persist(p Proposal) (Proposal, error) {
	e := s.save(p)
	return project(p), e
}
func (s *Store) read(repo, pid string) (Proposal, error) {
	b, e := os.ReadFile(s.path(repo, pid))
	if errors.Is(e, fs.ErrNotExist) {
		return Proposal{}, ErrNotFound
	}
	var p Proposal
	if e != nil || json.Unmarshal(b, &p) != nil || p.RepositoryID != repo || p.ID != pid {
		return Proposal{}, ErrNotFound
	}
	return p, nil
}
func project(p Proposal) Proposal {
	n := 0
	for ri := range p.Revisions {
		ev := p.Revisions[ri].Evidence[:0]
		for _, e := range p.Revisions[ri].Evidence {
			if e.Audience == "public" || e.Audience == "repository" {
				ev = append(ev, e)
			} else {
				n++
			}
		}
		p.Revisions[ri].Evidence = ev
	}
	p.PrivateEvidenceCount = n
	for i := range p.Acknowledgements {
		p.Acknowledgements[i].Current = p.Acknowledgements[i].ProposalRevision == p.CurrentRevision
	}
	return p
}
func (s *Store) Create(repo, actor string, in Input) (Proposal, error) {
	if actor == "" || !validInput(in) {
		return Proposal{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := Revision{Number: 1, Input: in, AuthorID: actor, CreatedAt: s.now().UTC()}
	p := Proposal{ID: id(), RepositoryID: repo, CurrentRevision: 1, Revisions: []Revision{r}}
	return s.persist(p)
}
func (s *Store) Get(repo, pid string) (Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, pid)
	return project(p), e
}
func (s *Store) List(repo string) ([]Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Proposal{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Proposal{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		p, e := s.read(repo, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, project(p))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Revisions[len(out[i].Revisions)-1].CreatedAt.After(out[j].Revisions[len(out[j].Revisions)-1].CreatedAt)
	})
	return out, nil
}
func (s *Store) Revise(repo, pid, actor string, expected int64, in Input) (Proposal, error) {
	if actor == "" || !validInput(in) {
		return Proposal{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, pid)
	if e != nil {
		return Proposal{}, e
	}
	if p.CurrentRevision != expected {
		return Proposal{}, ErrConflict
	}
	r := Revision{Number: expected + 1, Input: in, AuthorID: actor, CreatedAt: s.now().UTC()}
	p.CurrentRevision = r.Number
	p.Revisions = append(p.Revisions, r)
	return s.persist(p)
}
func evidenceAllowed(p Proposal, revision int64, ids []string) bool {
	if revision < 1 || revision > int64(len(p.Revisions)) {
		return false
	}
	allowed := map[string]bool{}
	for _, e := range p.Revisions[revision-1].Evidence {
		allowed[e.ID] = e.Audience == "public" || e.Audience == "repository"
	}
	for _, x := range ids {
		if !allowed[x] {
			return false
		}
	}
	return true
}
func (s *Store) Invite(repo, pid, actor, subject, kind, role string, evidence []string, expected int64) (Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, pid)
	if e != nil {
		return Proposal{}, e
	}
	if p.CurrentRevision != expected {
		return Proposal{}, ErrConflict
	}
	if subject == "" || !map[string]bool{"designer": true, "developer": true, "user": true, "agent": true}[kind] || !map[string]bool{"author": true, "reviewer": true, "research_participant": true}[role] || (kind == "agent" && (len(evidence) == 0 || !evidenceAllowed(p, expected, evidence))) {
		return Proposal{}, ErrInvalid
	}
	p.Participants = append(p.Participants, Participant{ID: id(), SubjectID: subject, Kind: kind, Role: role, InvitedBy: actor, GroundedEvidenceIDs: evidence, CreatedAt: s.now().UTC()})
	return s.persist(p)
}
func validArtifact(x ArtifactInput) bool {
	return map[string]bool{"wireframe": true, "prototype": true}[x.Kind] && x.Title != "" && x.ChangeReason != "" && len(x.Frames) > 0
}
func (s *Store) AddArtifact(repo, pid, actor string, in ArtifactInput) (Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, pid)
	if e != nil {
		return Proposal{}, e
	}
	if !validArtifact(in) || in.ProposalRevision != p.CurrentRevision || !evidenceAllowed(p, in.ProposalRevision, in.EvidenceIDs) {
		return Proposal{}, ErrInvalid
	}
	r := ArtifactRevision{Number: 1, ArtifactInput: in, AuthorID: actor, CreatedAt: s.now().UTC()}
	p.Artifacts = append(p.Artifacts, Artifact{ID: id(), CurrentVersion: 1, Revisions: []ArtifactRevision{r}})
	return s.persist(p)
}
func (s *Store) ReviseArtifact(repo, pid, aid, actor string, expected int64, in ArtifactInput) (Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, pid)
	if e != nil {
		return Proposal{}, e
	}
	if !validArtifact(in) || in.ProposalRevision != p.CurrentRevision || !evidenceAllowed(p, in.ProposalRevision, in.EvidenceIDs) {
		return Proposal{}, ErrInvalid
	}
	for i := range p.Artifacts {
		if p.Artifacts[i].ID == aid {
			if p.Artifacts[i].CurrentVersion != expected {
				return Proposal{}, ErrConflict
			}
			r := ArtifactRevision{Number: expected + 1, ArtifactInput: in, AuthorID: actor, CreatedAt: s.now().UTC()}
			p.Artifacts[i].CurrentVersion = r.Number
			p.Artifacts[i].Revisions = append(p.Artifacts[i].Revisions, r)
			return s.persist(p)
		}
	}
	return Proposal{}, ErrNotFound
}
func (s *Store) Comment(repo, pid, actor, subjectKind, subjectID, body, stance, uncertainty string, revision int64, evidence []string) (Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, pid)
	if e != nil {
		return Proposal{}, e
	}
	if body == "" || !map[string]bool{"proposal": true, "artifact": true}[subjectKind] || !map[string]bool{"comment": true, "support": true, "dissent": true, "question": true}[stance] || !evidenceAllowed(p, p.CurrentRevision, evidence) {
		return Proposal{}, ErrInvalid
	}
	if subjectKind == "proposal" {
		if subjectID != pid || revision < 1 || revision > p.CurrentRevision {
			return Proposal{}, ErrInvalid
		}
	} else {
		found := false
		for _, a := range p.Artifacts {
			if a.ID == subjectID && revision >= 1 && revision <= a.CurrentVersion {
				found = true
			}
		}
		if !found {
			return Proposal{}, ErrInvalid
		}
	}
	p.Comments = append(p.Comments, Comment{ID: id(), AuthorID: actor, SubjectKind: subjectKind, SubjectID: subjectID, SubjectRevision: revision, Body: body, Stance: stance, EvidenceIDs: evidence, Uncertainty: uncertainty, CreatedAt: s.now().UTC()})
	return s.persist(p)
}
func (s *Store) RequestAcknowledgement(repo, pid, actor, owner string, expected int64) (Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, pid)
	if e != nil {
		return Proposal{}, e
	}
	if p.CurrentRevision != expected {
		return Proposal{}, ErrConflict
	}
	if owner == "" {
		return Proposal{}, ErrInvalid
	}
	p.Acknowledgements = append(p.Acknowledgements, Acknowledgement{ID: id(), OwnerID: owner, RequestedBy: actor, Status: "requested", ProposalRevision: expected, RequestedAt: s.now().UTC()})
	return s.persist(p)
}
func (s *Store) Respond(repo, pid, aid, actor, status, rationale string) (Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, pid)
	if e != nil {
		return Proposal{}, e
	}
	if rationale == "" || !map[string]bool{"acknowledged": true, "changes_requested": true}[status] {
		return Proposal{}, ErrInvalid
	}
	for i := range p.Acknowledgements {
		a := &p.Acknowledgements[i]
		if a.ID == aid {
			if a.OwnerID != actor || a.Status != "requested" || a.ProposalRevision != p.CurrentRevision {
				return Proposal{}, ErrConflict
			}
			now := s.now().UTC()
			a.Status = status
			a.Rationale = rationale
			a.RespondedAt = &now
			return s.persist(p)
		}
	}
	return Proposal{}, ErrNotFound
}
