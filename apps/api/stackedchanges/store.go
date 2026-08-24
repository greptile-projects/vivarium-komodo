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
type Stack struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Input
	Members          []Member   `json:"members"`
	Status           string     `json:"status"`
	Blockers         []Blocker  `json:"blockers"`
	CreatedBy        string     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	AuthorityGranted []string   `json:"authority_granted"`
	CurrentRevision  int        `json:"current_revision"`
	Revisions        []Revision `json:"revisions"`
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
	x := Stack{ID: id(), RepositoryID: repo, Input: in, Members: members, Blockers: blockers, CreatedBy: actor, CreatedAt: now, AuthorityGranted: []string{}, CurrentRevision: 1, Revisions: []Revision{}}
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
