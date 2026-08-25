// Package stackedchanges stores collaborative, revision-exact change stacks.
package stackedchanges

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

var ErrNotFound = errors.New("change stack not found")
var ErrInvalid = errors.New("invalid change stack")

type Blocker struct {
	Kind     string `json:"kind"`
	MemberID string `json:"member_id,omitempty"`
	Detail   string `json:"detail"`
}
type Permission struct {
	Read         bool   `json:"read"`
	Publish      bool   `json:"publish"`
	UpdateBranch bool   `json:"update_branch"`
	Reason       string `json:"reason"`
}
type Scope struct {
	CommitCount  int      `json:"commit_count"`
	CommitIDs    []string `json:"commit_ids"`
	ChangedPaths []string `json:"changed_paths"`
	Changes      []Change `json:"changes"`
	FromRevision string   `json:"from_revision"`
	ToRevision   string   `json:"to_revision"`
}
type Change struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Binary    bool   `json:"binary"`
	Patch     string `json:"patch,omitempty"`
}
type MemberInput struct {
	ID                 string   `json:"id"`
	Branch             string   `json:"branch"`
	BranchState        string   `json:"branch_state"`
	PullRequestID      string   `json:"pull_request_id,omitempty"`
	Revision           string   `json:"revision"`
	ParentID           string   `json:"parent_id,omitempty"`
	Authors            []string `json:"authors"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	RepositoryID       string   `json:"repository_id,omitempty"`
	BranchOwnerIDs     []string `json:"branch_owner_ids,omitempty"`
	BranchAccess       string   `json:"branch_access,omitempty"`
}
type Input struct {
	Title          string        `json:"title"`
	Outcome        string        `json:"outcome"`
	TargetBranch   string        `json:"target_branch"`
	TargetRevision string        `json:"target_revision"`
	Members        []MemberInput `json:"members"`
}
type Publication struct {
	ID          string    `json:"id"`
	MemberID    string    `json:"member_id"`
	Revision    string    `json:"revision"`
	PublishedBy string    `json:"published_by"`
	PublishedAt time.Time `json:"published_at"`
	ReviewState string    `json:"review_state"`
}

// Evidence binds an existing collaboration artifact to the layer it actually
// evaluated. Reference is an opaque ID; this resource never grants authority
// over the referenced discussion, decision, check, preview, or agent record.
type Evidence struct {
	ID                   string            `json:"id"`
	Kind                 string            `json:"kind"`
	Reference            string            `json:"reference"`
	Scope                string            `json:"scope"`
	MemberID             string            `json:"member_id"`
	Revision             string            `json:"revision"`
	UpstreamRevisions    map[string]string `json:"upstream_revisions"`
	ActorID              string            `json:"actor_id"`
	CreatedAt            time.Time         `json:"created_at"`
	State                string            `json:"state"`
	StaleIfMembersChange []string          `json:"stale_if_members_change"`
}
type Assignment struct {
	ID                 string    `json:"id"`
	ParticipantID      string    `json:"participant_id"`
	ParticipantKind    string    `json:"participant_kind"`
	AgentApprovalID    string    `json:"agent_approval_id,omitempty"`
	AuthorizedBranches []string  `json:"authorized_branches"`
	AssignedBy         string    `json:"assigned_by"`
	AssignedAt         time.Time `json:"assigned_at"`
}
type Workspace struct {
	ID                 string            `json:"id"`
	Kind               string            `json:"kind"`
	MemberID           string            `json:"member_id"`
	ParticipantID      string            `json:"participant_id"`
	Outcome            string            `json:"outcome"`
	ParentRevision     string            `json:"parent_revision"`
	MemberRevision     string            `json:"member_revision"`
	AcceptanceCriteria []string          `json:"acceptance_criteria"`
	Evidence           []Evidence        `json:"evidence"`
	UpstreamRevisions  map[string]string `json:"upstream_revisions"`
	EditableBranches   []string          `json:"editable_branches"`
	Audience           string            `json:"audience"`
	AuthorityGranted   []string          `json:"authority_granted"`
	OpenedBy           string            `json:"opened_by"`
	OpenedAt           time.Time         `json:"opened_at"`
}
type TimelineEvent struct {
	ID                string            `json:"id"`
	Kind              string            `json:"kind"`
	MemberID          string            `json:"member_id"`
	WorkspaceID       string            `json:"workspace_id,omitempty"`
	ActorID           string            `json:"actor_id"`
	Summary           string            `json:"summary"`
	ProposedMembers   []MemberInput     `json:"proposed_members,omitempty"`
	UpstreamRevisions map[string]string `json:"upstream_revisions"`
	State             string            `json:"state"`
	Audience          string            `json:"audience"`
	CreatedAt         time.Time         `json:"created_at"`
}
type Member struct {
	MemberInput
	Position                 int               `json:"position"`
	BaseRevision             string            `json:"base_revision"`
	IndividualScope          Scope             `json:"individual_scope"`
	CumulativeScope          Scope             `json:"cumulative_scope"`
	EffectivePermissions     Permission        `json:"effective_permissions"`
	Blockers                 []Blocker         `json:"blockers"`
	Publications             []Publication     `json:"publications"`
	Evidence                 []Evidence        `json:"evidence"`
	UpstreamRevisions        map[string]string `json:"upstream_revisions"`
	ReviewState              string            `json:"review_state"`
	DownstreamEvidenceAtRisk []string          `json:"downstream_evidence_at_risk"`
	Reviewable               bool              `json:"reviewable"`
	Assignments              []Assignment      `json:"assignments"`
	Workspaces               []Workspace       `json:"workspaces"`
}
type CommitRewrite struct {
	MemberID            string `json:"member_id"`
	OldCommit           string `json:"old_commit,omitempty"`
	NewCommit           string `json:"new_commit"`
	OldBase             string `json:"old_base,omitempty"`
	NewBase             string `json:"new_base"`
	Kind                string `json:"kind"`
	AuthorshipPreserved bool   `json:"authorship_preserved"`
}
type BranchUpdate struct {
	MemberID          string `json:"member_id"`
	Branch            string `json:"branch"`
	ExpectedRevision  string `json:"expected_revision,omitempty"`
	PublishedRevision string `json:"published_revision"`
	State             string `json:"state"`
}
type Revision struct {
	Number              int             `json:"number"`
	PreviousNumber      int             `json:"previous_number"`
	Reason              string          `json:"reason"`
	PreviousMembers     []Member        `json:"previous_members"`
	Members             []Member        `json:"members"`
	CommitRewrites      []CommitRewrite `json:"commit_rewrites"`
	BranchUpdates       []BranchUpdate  `json:"branch_updates"`
	ReviewInvalidations []string        `json:"review_invalidations"`
	CheckImpacts        []string        `json:"check_impacts"`
	Blockers            []Blocker       `json:"blockers"`
	Status              string          `json:"status"`
	CreatedBy           string          `json:"created_by"`
	CreatedAt           time.Time       `json:"created_at"`
	AppliedBy           string          `json:"applied_by,omitempty"`
	AppliedAt           *time.Time      `json:"applied_at,omitempty"`
}
type LandingEvidence struct {
	ID                string    `json:"id"`
	Kind              string    `json:"kind"`
	Reference         string    `json:"reference"`
	Status            string    `json:"status"`
	CandidateRevision string    `json:"candidate_revision"`
	BaseRevision      string    `json:"base_revision"`
	SourceRevision    string    `json:"source_revision"`
	ActorID           string    `json:"actor_id"`
	CreatedAt         time.Time `json:"created_at"`
}
type LandingCandidate struct {
	ID                string            `json:"id"`
	Generation        int               `json:"generation"`
	Position          int               `json:"position"`
	MemberID          string            `json:"member_id"`
	BaseRevision      string            `json:"base_revision"`
	SourceRevision    string            `json:"source_revision"`
	CandidateRevision string            `json:"candidate_revision"`
	CandidateTree     string            `json:"candidate_tree"`
	Status            string            `json:"status"`
	RequiredEvidence  []string          `json:"required_evidence"`
	Evidence          []LandingEvidence `json:"evidence"`
	Blockers          []Blocker         `json:"blockers"`
	CreatedAt         time.Time         `json:"created_at"`
}
type LandingEvent struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	MemberID  string    `json:"member_id,omitempty"`
	ActorID   string    `json:"actor_id"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Landing struct {
	ID                     string             `json:"id"`
	StackRevision          int                `json:"stack_revision"`
	TargetBranch           string             `json:"target_branch"`
	OriginalTargetRevision string             `json:"original_target_revision"`
	CurrentTargetRevision  string             `json:"current_target_revision"`
	Mode                   string             `json:"mode"`
	AtomicPermitted        bool               `json:"atomic_permitted"`
	Status                 string             `json:"status"`
	MergedMembers          []string           `json:"merged_members"`
	PausedFromMember       string             `json:"paused_from_member,omitempty"`
	Candidates             []LandingCandidate `json:"candidates"`
	Events                 []LandingEvent     `json:"events"`
	CreatedBy              string             `json:"created_by"`
	CreatedAt              time.Time          `json:"created_at"`
	AuthorityGranted       []string           `json:"authority_granted"`
}
type Stack struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Input
	Members          []Member        `json:"members"`
	Status           string          `json:"status"`
	Blockers         []Blocker       `json:"blockers"`
	CreatedBy        string          `json:"created_by"`
	CreatedAt        time.Time       `json:"created_at"`
	AuthorityGranted []string        `json:"authority_granted"`
	CurrentRevision  int             `json:"current_revision"`
	Revisions        []Revision      `json:"revisions"`
	Timeline         []TimelineEvent `json:"timeline"`
	Landings         []Landing       `json:"landings"`
}

type Store struct {
	mu   sync.Mutex
	root string
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrInvalid
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}
func id() string                                { var b [8]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, stack string) string { return filepath.Join(s.root, repo, stack+".json") }
func validate(in Input) error {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Outcome) == "" || strings.TrimSpace(in.TargetBranch) == "" || strings.TrimSpace(in.TargetRevision) == "" || len(in.Members) == 0 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, m := range in.Members {
		if m.ID == "" || m.Branch == "" || m.Revision == "" || len(m.Authors) == 0 || len(m.AcceptanceCriteria) == 0 || seen[m.ID] || (m.BranchState != "existing" && m.BranchState != "new") {
			return ErrInvalid
		}
		if m.BranchAccess != "" && m.BranchAccess != "current" && m.BranchAccess != "revoked" {
			return ErrInvalid
		}
		seen[m.ID] = true
	}
	return nil
}
func (s *Store) Create(repo, actor string, in Input, members []Member, blockers []Blocker) (Stack, error) {
	if err := validate(in); err != nil {
		return Stack{}, err
	}
	now := time.Now().UTC()
	x := Stack{ID: id(), RepositoryID: repo, Input: in, Members: members, Blockers: blockers, CreatedBy: actor, CreatedAt: now, AuthorityGranted: []string{}, CurrentRevision: 1, Revisions: []Revision{}, Timeline: []TimelineEvent{}, Landings: []Landing{}}
	x.Status = "reviewable"
	if len(blockers) > 0 {
		x.Status = "blocked"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Join(s.root, repo), 0755); err != nil {
		return Stack{}, err
	}
	return x, s.save(x)
}

func (s *Store) Assign(repo, stack, member, actor, participant, kind, approval string, branches []string) (Stack, error) {
	if strings.TrimSpace(participant) == "" || (kind != "human" && kind != "agent") || (kind == "agent" && strings.TrimSpace(approval) == "") {
		return Stack{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, e
	}
	for i := range x.Members {
		m := &x.Members[i]
		if m.ID != member {
			continue
		}
		owners := m.BranchOwnerIDs
		if len(owners) == 0 {
			owners = m.Authors
		}
		if !contains(owners, actor) {
			return Stack{}, ErrInvalid
		}
		if len(branches) == 0 {
			branches = []string{m.Branch}
		}
		for _, b := range branches {
			if b != m.Branch {
				return Stack{}, ErrInvalid
			}
		}
		m.Assignments = append(m.Assignments, Assignment{ID: id(), ParticipantID: participant, ParticipantKind: kind, AgentApprovalID: approval, AuthorizedBranches: branches, AssignedBy: actor, AssignedAt: time.Now().UTC()})
		return x, s.save(x)
	}
	return Stack{}, ErrNotFound
}

func (s *Store) OpenWorkspace(repo, stack, member, actor, assignmentID, kind, audience string) (Stack, error) {
	allowed := map[string]bool{"session": true, "shared": true, "conflict_resolution": true}
	if !allowed[kind] || (audience != "repository" && audience != "participants" && audience != "embargoed") {
		return Stack{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, e
	}
	for i := range x.Members {
		m := &x.Members[i]
		if m.ID != member {
			continue
		}
		var a *Assignment
		for j := range m.Assignments {
			if m.Assignments[j].ID == assignmentID {
				a = &m.Assignments[j]
			}
		}
		if a == nil || a.ParticipantID != actor {
			return Stack{}, ErrInvalid
		}
		editable := []string{}
		if a.ParticipantKind == "human" || contains(a.AuthorizedBranches, m.Branch) {
			editable = append(editable, m.Branch)
		}
		evidence := append([]Evidence{}, m.Evidence...)
		w := Workspace{ID: id(), Kind: kind, MemberID: m.ID, ParticipantID: actor, Outcome: x.Outcome, ParentRevision: m.BaseRevision, MemberRevision: m.Revision, AcceptanceCriteria: append([]string{}, m.AcceptanceCriteria...), Evidence: evidence, UpstreamRevisions: upstreamFor(x, m.ID), EditableBranches: editable, Audience: audience, AuthorityGranted: []string{}, OpenedBy: actor, OpenedAt: time.Now().UTC()}
		m.Workspaces = append(m.Workspaces, w)
		return x, s.save(x)
	}
	return Stack{}, ErrNotFound
}

func (s *Store) AppendTimeline(repo, stack, member, workspace, actor, kind, summary, audience string, proposed []MemberInput) (Stack, error) {
	allowed := map[string]bool{"checkpoint": true, "question": true, "handoff": true, "proposed_restack": true}
	if !allowed[kind] || strings.TrimSpace(summary) == "" || (audience != "repository" && audience != "participants" && audience != "embargoed") {
		return Stack{}, ErrInvalid
	}
	if kind != "proposed_restack" && len(proposed) > 0 {
		return Stack{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, e
	}
	for _, m := range x.Members {
		if m.ID != member {
			continue
		}
		authorized := false
		if workspace == "" {
			owners := m.BranchOwnerIDs
			if len(owners) == 0 {
				owners = m.Authors
			}
			authorized = contains(owners, actor)
		} else {
			for _, w := range m.Workspaces {
				if w.ID == workspace && w.ParticipantID == actor {
					authorized = true
				}
			}
		}
		if !authorized {
			return Stack{}, ErrInvalid
		}
		x.Timeline = append(x.Timeline, TimelineEvent{ID: id(), Kind: kind, MemberID: member, WorkspaceID: workspace, ActorID: actor, Summary: summary, ProposedMembers: proposed, UpstreamRevisions: upstreamFor(x, member), State: "current", Audience: audience, CreatedAt: time.Now().UTC()})
		return x, s.save(x)
	}
	return Stack{}, ErrNotFound
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func (s *Store) PreviewRevision(repo, stack, actor, reason string, expected int, in Input, members []Member, blockers []Blocker) (Stack, error) {
	if err := validate(in); err != nil || strings.TrimSpace(reason) == "" {
		return Stack{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, e
	}
	if expected != x.CurrentRevision {
		return Stack{}, ErrInvalid
	}
	r := Revision{Number: len(x.Revisions) + 2, PreviousNumber: expected, Reason: reason, PreviousMembers: x.Members, Members: members, Blockers: blockers, Status: "ready", CreatedBy: actor, CreatedAt: time.Now().UTC(), CommitRewrites: []CommitRewrite{}, BranchUpdates: []BranchUpdate{}, ReviewInvalidations: []string{}, CheckImpacts: []string{}}
	old := map[string]Member{}
	for _, m := range x.Members {
		old[m.ID] = m
	}
	seenBranch := map[string]string{}
	for _, m := range members {
		o, exists := old[m.ID]
		kind := "insert"
		if exists {
			kind = "unchanged"
			if o.Revision != m.Revision {
				kind = "rewrite"
			}
			if o.BaseRevision != m.BaseRevision {
				kind = "rebase"
			}
			if o.Position != m.Position && o.Revision == m.Revision {
				kind = "reorder"
			}
		}
		if kind != "unchanged" {
			r.CommitRewrites = append(r.CommitRewrites, CommitRewrite{MemberID: m.ID, OldCommit: o.Revision, NewCommit: m.Revision, OldBase: o.BaseRevision, NewBase: m.BaseRevision, Kind: kind, AuthorshipPreserved: sameStrings(o.Authors, m.Authors)})
		}
		expectedTip := ""
		if exists && o.Branch == m.Branch && o.BranchState == "existing" {
			expectedTip = o.Revision
		}
		r.BranchUpdates = append(r.BranchUpdates, BranchUpdate{MemberID: m.ID, Branch: m.Branch, ExpectedRevision: expectedTip, PublishedRevision: m.Revision, State: "pending"})
		if prior := seenBranch[m.Branch]; prior != "" {
			r.Blockers = append(r.Blockers, Blocker{Kind: "shared_branch", MemberID: m.ID, Detail: "branch is also used by " + prior})
		}
		seenBranch[m.Branch] = m.ID
		if exists && (o.Revision != m.Revision || o.BaseRevision != m.BaseRevision) {
			for _, ev := range o.Evidence {
				r.ReviewInvalidations = append(r.ReviewInvalidations, ev.ID)
				if ev.Kind == "check" {
					r.CheckImpacts = append(r.CheckImpacts, ev.Reference)
				}
			}
		}
	}
	for id, o := range old {
		found := false
		for _, m := range members {
			if m.ID == id {
				found = true
			}
		}
		if !found {
			r.CommitRewrites = append(r.CommitRewrites, CommitRewrite{MemberID: id, OldCommit: o.Revision, OldBase: o.BaseRevision, Kind: "remove", AuthorshipPreserved: true})
			for _, ev := range o.Evidence {
				r.ReviewInvalidations = append(r.ReviewInvalidations, ev.ID)
				if ev.Kind == "check" {
					r.CheckImpacts = append(r.CheckImpacts, ev.Reference)
				}
			}
		}
	}
	r.Blockers = dedupe(r.Blockers)
	sort.Strings(r.ReviewInvalidations)
	sort.Strings(r.CheckImpacts)
	if len(r.Blockers) > 0 {
		r.Status = "blocked"
	}
	x.Revisions = append(x.Revisions, r)
	return x, s.save(x)
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string{}, a...)
	y := append([]string{}, b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
func dedupe(xs []Blocker) []Blocker {
	out := []Blocker{}
	seen := map[string]bool{}
	for _, x := range xs {
		k := x.Kind + "|" + x.MemberID + "|" + x.Detail
		if !seen[k] {
			seen[k] = true
			out = append(out, x)
		}
	}
	return out
}

func (s *Store) RevisionForApply(repo, stack string, number int) (Stack, Revision, error) {
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, Revision{}, e
	}
	for _, r := range x.Revisions {
		if r.Number == number {
			if r.Status != "ready" || r.PreviousNumber != x.CurrentRevision {
				return Stack{}, Revision{}, ErrInvalid
			}
			return x, r, nil
		}
	}
	return Stack{}, Revision{}, ErrNotFound
}
func (s *Store) FinishApply(repo, stack string, number int, actor string, applyErr error) (Stack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, e
	}
	for i := range x.Revisions {
		r := &x.Revisions[i]
		if r.Number != number {
			continue
		}
		if r.Status != "ready" || r.PreviousNumber != x.CurrentRevision {
			return Stack{}, ErrInvalid
		}
		if applyErr != nil {
			r.Status = "failed"
			r.Blockers = append(r.Blockers, Blocker{Kind: "concurrent_push_or_failed_rewrite", Detail: applyErr.Error()})
			for j := range r.BranchUpdates {
				r.BranchUpdates[j].State = "not_applied"
			}
			return x, s.save(x)
		}
		now := time.Now().UTC()
		r.Status = "applied"
		r.AppliedBy = actor
		r.AppliedAt = &now
		for j := range r.BranchUpdates {
			r.BranchUpdates[j].State = "published"
		}
		x.Input.Members = memberInputs(r.Members)
		x.Members = r.Members
		x.Blockers = r.Blockers
		x.Status = "reviewable"
		x.CurrentRevision = number
		for i := range x.Members {
			x.Members[i].BranchState = "existing"
			x.Members[i].Evidence = []Evidence{}
			x.Members[i].Publications = []Publication{}
		}
		x.Input.Members = memberInputs(x.Members)
		return x, s.save(x)
	}
	return Stack{}, ErrNotFound
}

func landingReady(c LandingCandidate) bool {
	if len(c.Blockers) > 0 {
		return false
	}
	for _, required := range c.RequiredEvidence {
		passed := false
		for _, e := range c.Evidence {
			if e.Kind == required && e.Status == "passed" && e.CandidateRevision == c.CandidateRevision && e.BaseRevision == c.BaseRevision && e.SourceRevision == c.SourceRevision {
				passed = true
			}
		}
		if !passed {
			return false
		}
	}
	return true
}

func projectLanding(l *Landing) {
	unsafe, active := false, 0
	for i := range l.Candidates {
		c := &l.Candidates[i]
		if c.Status == "superseded" {
			continue
		}
		active++
		if contains(l.MergedMembers, c.MemberID) {
			c.Status = "merged"
			continue
		}
		if unsafe {
			c.Status = "paused_suffix"
			continue
		}
		if landingReady(*c) {
			c.Status = "ready"
		} else {
			c.Status = "verifying"
		}
		if len(c.Blockers) > 0 || c.Status != "ready" {
			unsafe = true
			l.PausedFromMember = c.MemberID
		}
	}
	if len(l.MergedMembers) == active {
		l.Status, l.PausedFromMember = "merged", ""
		return
	}
	if unsafe {
		l.Status = "paused"
	} else {
		l.Status, l.PausedFromMember = "ready", ""
	}
}

func (s *Store) CreateLanding(repo, stack, actor, mode string, atomic bool, required []string, target string, candidates []LandingCandidate) (Stack, error) {
	if (mode != "ordered" && mode != "atomic") || (mode == "atomic" && !atomic) || len(candidates) == 0 {
		return Stack{}, ErrInvalid
	}
	allowed := map[string]bool{"required_check": true, "reproduction": true, "contract": true, "preview": true, "policy": true, "approval": true}
	for _, kind := range required {
		if !allowed[kind] {
			return Stack{}, ErrInvalid
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, e
	}
	if x.CurrentRevision < 1 || strings.TrimSpace(target) == "" {
		return Stack{}, ErrInvalid
	}
	now := time.Now().UTC()
	l := Landing{ID: id(), StackRevision: x.CurrentRevision, TargetBranch: x.TargetBranch, OriginalTargetRevision: target, CurrentTargetRevision: target, Mode: mode, AtomicPermitted: atomic, Status: "verifying", MergedMembers: []string{}, Candidates: candidates, Events: []LandingEvent{{ID: id(), Action: "assembled", ActorID: actor, Detail: "immutable ready-prefix candidates assembled", CreatedAt: now}}, CreatedBy: actor, CreatedAt: now, AuthorityGranted: []string{}}
	projectLanding(&l)
	x.Landings = append(x.Landings, l)
	return x, s.save(x)
}

func (s *Store) AddLandingEvidence(repo, stack, landing, candidate, actor, kind, reference, status string) (Stack, error) {
	if strings.TrimSpace(reference) == "" || (status != "passed" && status != "failed" && status != "canceled") {
		return Stack{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, e
	}
	for i := range x.Landings {
		l := &x.Landings[i]
		if l.ID != landing {
			continue
		}
		for j := range l.Candidates {
			c := &l.Candidates[j]
			if c.ID != candidate {
				continue
			}
			allowed := false
			for _, k := range c.RequiredEvidence {
				if k == kind {
					allowed = true
				}
			}
			if !allowed {
				return Stack{}, ErrInvalid
			}
			c.Evidence = append(c.Evidence, LandingEvidence{ID: id(), Kind: kind, Reference: reference, Status: status, CandidateRevision: c.CandidateRevision, BaseRevision: c.BaseRevision, SourceRevision: c.SourceRevision, ActorID: actor, CreatedAt: time.Now().UTC()})
			projectLanding(l)
			return x, s.save(x)
		}
		return Stack{}, ErrNotFound
	}
	return Stack{}, ErrNotFound
}

func (s *Store) LandingForMerge(repo, stack, landing, member string, atomic bool) (Stack, Landing, LandingCandidate, error) {
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, Landing{}, LandingCandidate{}, e
	}
	for _, l := range x.Landings {
		if l.ID != landing {
			continue
		}
		projectLanding(&l)
		if l.StackRevision != x.CurrentRevision {
			return Stack{}, Landing{}, LandingCandidate{}, ErrInvalid
		}
		if atomic {
			if l.Mode != "atomic" || !l.AtomicPermitted || l.Status != "ready" {
				return Stack{}, Landing{}, LandingCandidate{}, ErrInvalid
			}
			for i := len(l.Candidates) - 1; i >= 0; i-- {
				if l.Candidates[i].Status != "superseded" {
					return x, l, l.Candidates[i], nil
				}
			}
		}
		for _, c := range l.Candidates {
			if c.Status == "superseded" || contains(l.MergedMembers, c.MemberID) {
				continue
			}
			if c.MemberID != member || c.Status != "ready" {
				return Stack{}, Landing{}, LandingCandidate{}, ErrInvalid
			}
			return x, l, c, nil
		}
		return Stack{}, Landing{}, LandingCandidate{}, ErrInvalid
	}
	return Stack{}, Landing{}, LandingCandidate{}, ErrNotFound
}

func (s *Store) FinishLandingMerge(repo, stack, landing, member, actor string, atomic bool, mergeErr error) (Stack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, e
	}
	for i := range x.Landings {
		l := &x.Landings[i]
		if l.ID != landing {
			continue
		}
		if mergeErr != nil {
			l.Status = "paused"
			l.PausedFromMember = member
			l.Events = append(l.Events, LandingEvent{ID: id(), Action: "target_moved", MemberID: member, ActorID: actor, Detail: mergeErr.Error(), CreatedAt: time.Now().UTC()})
			return x, s.save(x)
		}
		if atomic {
			l.MergedMembers = []string{}
			for _, c := range l.Candidates {
				if c.Status != "superseded" && !contains(l.MergedMembers, c.MemberID) {
					l.MergedMembers = append(l.MergedMembers, c.MemberID)
				}
			}
		} else {
			l.MergedMembers = append(l.MergedMembers, member)
		}
		l.Events = append(l.Events, LandingEvent{ID: id(), Action: map[bool]string{true: "atomic_merged", false: "member_merged"}[atomic], MemberID: member, ActorID: actor, CreatedAt: time.Now().UTC()})
		projectLanding(l)
		return x, s.save(x)
	}
	return Stack{}, ErrNotFound
}

func (s *Store) RebuildLanding(repo, stack, landing, actor, target string, candidates []LandingCandidate) (Stack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, e
	}
	for i := range x.Landings {
		l := &x.Landings[i]
		if l.ID != landing {
			continue
		}
		if len(candidates) == 0 {
			return Stack{}, ErrInvalid
		}
		for j := range l.Candidates {
			if !contains(l.MergedMembers, l.Candidates[j].MemberID) {
				l.Candidates[j].Status = "superseded"
			}
		}
		l.CurrentTargetRevision = target
		l.Candidates = append(l.Candidates, candidates...)
		l.Events = append(l.Events, LandingEvent{ID: id(), Action: "rebuilt", ActorID: actor, Detail: "affected suffix rebuilt against " + target, CreatedAt: time.Now().UTC()})
		projectLanding(l)
		return x, s.save(x)
	}
	return Stack{}, ErrNotFound
}
func memberInputs(ms []Member) []MemberInput {
	out := make([]MemberInput, len(ms))
	for i := range ms {
		out[i] = ms[i].MemberInput
	}
	return out
}
func (s *Store) save(x Stack) error {
	b, e := json.MarshalIndent(x, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(s.path(x.RepositoryID, x.ID), b, 0644)
}
func (s *Store) Get(repo, stack string) (Stack, error) {
	b, e := os.ReadFile(s.path(repo, stack))
	if os.IsNotExist(e) {
		return Stack{}, ErrNotFound
	}
	if e != nil {
		return Stack{}, e
	}
	var x Stack
	e = json.Unmarshal(b, &x)
	return x, e
}
func (s *Store) List(repo string) ([]Stack, error) {
	entries, e := os.ReadDir(filepath.Join(s.root, repo))
	if os.IsNotExist(e) {
		return []Stack{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Stack{}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		x, er := s.Get(repo, strings.TrimSuffix(entry.Name(), ".json"))
		if er == nil {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Publish(repo, stack, member, revision, actor string) (Stack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, e
	}
	for i := range x.Members {
		m := &x.Members[i]
		if m.ID != member {
			continue
		}
		if revision != m.Revision || len(m.Blockers) > 0 || !m.EffectivePermissions.Publish {
			return Stack{}, ErrInvalid
		}
		m.Publications = append(m.Publications, Publication{ID: id(), MemberID: member, Revision: revision, PublishedBy: actor, PublishedAt: time.Now().UTC(), ReviewState: "published"})
		return x, s.save(x)
	}
	return Stack{}, ErrNotFound
}

func (s *Store) BindEvidence(repo, stack, member, revision, actor, kind, reference, scope string) (Stack, error) {
	allowed := map[string]bool{"discussion": true, "review_decision": true, "owner_acknowledgement": true, "check": true, "preview": true, "agent_finding": true}
	if !allowed[kind] || strings.TrimSpace(reference) == "" || (scope != "layer" && scope != "cumulative") {
		return Stack{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.Get(repo, stack)
	if e != nil {
		return Stack{}, e
	}
	for i := range x.Members {
		m := &x.Members[i]
		if m.ID != member {
			continue
		}
		if revision != m.Revision {
			return Stack{}, ErrInvalid
		}
		upstream := upstreamFor(x, m.ID)
		staleIf := []string{m.ID}
		for id := range upstream {
			staleIf = append(staleIf, id)
		}
		sort.Strings(staleIf)
		state := "current"
		for id := range upstream {
			if !published(x, id) {
				state = "provisional"
			}
		}
		m.Evidence = append(m.Evidence, Evidence{ID: id(), Kind: kind, Reference: reference, Scope: scope, MemberID: m.ID, Revision: revision, UpstreamRevisions: upstream, ActorID: actor, CreatedAt: time.Now().UTC(), State: state, StaleIfMembersChange: staleIf})
		return x, s.save(x)
	}
	return Stack{}, ErrNotFound
}

func upstreamFor(x Stack, member string) map[string]string {
	out := map[string]string{}
	by := map[string]Member{}
	for _, m := range x.Members {
		by[m.ID] = m
	}
	at := by[member].ParentID
	for at != "" {
		m, ok := by[at]
		if !ok {
			break
		}
		out[m.ID] = m.Revision
		at = m.ParentID
	}
	return out
}
func published(x Stack, member string) bool {
	for _, m := range x.Members {
		if m.ID == member {
			return len(m.Publications) > 0
		}
	}
	return false
}

// Project derives review sequencing and invalidation impact from immutable
// bindings each time the stack is read.
func Project(x Stack) Stack {
	for i := range x.Members {
		m := &x.Members[i]
		m.UpstreamRevisions = upstreamFor(x, m.ID)
		m.ReviewState = "reviewable_now"
		if len(m.Blockers) > 0 || len(m.Publications) == 0 {
			m.ReviewState = "not_published"
		} else {
			for id := range m.UpstreamRevisions {
				if !published(x, id) {
					m.ReviewState = "provisional"
				}
			}
		}
		for j := range m.Evidence {
			m.Evidence[j].State = "current"
			for id, revision := range m.Evidence[j].UpstreamRevisions {
				found := false
				for _, u := range x.Members {
					if u.ID == id && u.Revision == revision && published(x, id) {
						found = true
					}
				}
				if !found {
					m.Evidence[j].State = "provisional"
				}
			}
		}
		m.DownstreamEvidenceAtRisk = []string{}
	}
	for _, m := range x.Members {
		for _, e := range m.Evidence {
			for _, changed := range e.StaleIfMembersChange {
				for i := range x.Members {
					if x.Members[i].ID == changed {
						x.Members[i].DownstreamEvidenceAtRisk = append(x.Members[i].DownstreamEvidenceAtRisk, e.ID)
					}
				}
			}
		}
	}
	for i := range x.Members {
		sort.Strings(x.Members[i].DownstreamEvidenceAtRisk)
	}
	for i := range x.Timeline {
		x.Timeline[i].State = "current"
		for member, revision := range x.Timeline[i].UpstreamRevisions {
			found := false
			for _, m := range x.Members {
				if m.ID == member && m.Revision == revision {
					found = true
				}
			}
			if !found {
				x.Timeline[i].State = "upstream_changed"
			}
		}
	}
	return x
}

func (s *Store) FindByPull(repo, pull string) (Stack, Member, error) {
	xs, e := s.List(repo)
	if e != nil {
		return Stack{}, Member{}, e
	}
	for _, x := range xs {
		for _, m := range x.Members {
			if m.PullRequestID == pull {
				x = Project(x)
				for _, p := range x.Members {
					if p.ID == m.ID {
						return x, p, nil
					}
				}
			}
		}
	}
	return Stack{}, Member{}, ErrNotFound
}
