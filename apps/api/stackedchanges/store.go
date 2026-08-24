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
type Stack struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Input
	Members          []Member  `json:"members"`
	Status           string    `json:"status"`
	Blockers         []Blocker `json:"blockers"`
	CreatedBy        string    `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
	AuthorityGranted []string  `json:"authority_granted"`
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
		seen[m.ID] = true
	}
	return nil
}
func (s *Store) Create(repo, actor string, in Input, members []Member, blockers []Blocker) (Stack, error) {
	if err := validate(in); err != nil {
		return Stack{}, err
	}
	now := time.Now().UTC()
	x := Stack{ID: id(), RepositoryID: repo, Input: in, Members: members, Blockers: blockers, CreatedBy: actor, CreatedAt: now, AuthorityGranted: []string{}}
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
