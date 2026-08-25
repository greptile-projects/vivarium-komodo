// Package reviewwork owns the shared, revision-exact workspace used by parallel reviewers.
package reviewwork

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewrouting"
)

var ErrInvalid = errors.New("invalid shared review work")
var ErrNotFound = errors.New("shared review work not found")
var ErrConflict = errors.New("shared review work changed")

type QueueItem struct {
	ID         string   `json:"id"`
	AreaID     string   `json:"area_id"`
	Kind       string   `json:"kind"`
	Reference  string   `json:"reference"`
	Revision   string   `json:"revision"`
	Question   string   `json:"question,omitempty"`
	Paths      []string `json:"paths,omitempty"`
	DependsOn  []string `json:"depends_on,omitempty"`
	Accessible bool     `json:"accessible"`
}
type Citation struct {
	Kind       string `json:"kind"`
	Reference  string `json:"reference"`
	Revision   string `json:"revision"`
	Summary    string `json:"summary"`
	Accessible bool   `json:"accessible"`
	Audience   string `json:"audience"`
}
type Progress struct {
	AssignmentID string    `json:"assignment_id"`
	ActorID      string    `json:"actor_id"`
	State        string    `json:"state"`
	QueueItemIDs []string  `json:"queue_item_ids"`
	Coverage     []string  `json:"coverage"`
	Uncertainty  []string  `json:"uncertainty"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type Finding struct {
	ID           string     `json:"id"`
	AreaID       string     `json:"area_id"`
	AssignmentID string     `json:"assignment_id"`
	ActorID      string     `json:"actor_id"`
	ActorKind    string     `json:"actor_kind"`
	Summary      string     `json:"summary"`
	Severity     string     `json:"severity"`
	Conclusion   string     `json:"conclusion"`
	Citations    []Citation `json:"citations"`
	Uncertainty  []string   `json:"uncertainty"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
}
type Message struct {
	ID         string     `json:"id"`
	AreaID     string     `json:"area_id"`
	ActorID    string     `json:"actor_id"`
	Kind       string     `json:"kind"`
	Body       string     `json:"body"`
	FindingIDs []string   `json:"finding_ids,omitempty"`
	Citations  []Citation `json:"citations,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
type Handoff struct {
	ID                  string     `json:"id"`
	AreaID              string     `json:"area_id"`
	FromAssignmentID    string     `json:"from_assignment_id"`
	ToAssignmentID      string     `json:"to_assignment_id"`
	ActorID             string     `json:"actor_id"`
	QueueItemIDs        []string   `json:"queue_item_ids"`
	FindingIDs          []string   `json:"finding_ids"`
	Reason              string     `json:"reason"`
	ResidualUncertainty []string   `json:"residual_uncertainty"`
	State               string     `json:"state"`
	CreatedAt           time.Time  `json:"created_at"`
	AcceptedAt          *time.Time `json:"accepted_at,omitempty"`
}
type Workspace struct {
	RepositoryID    string              `json:"repository_id"`
	PullRequestID   string              `json:"pull_request_id"`
	PlanVersion     int64               `json:"plan_version"`
	Revision        string              `json:"revision"`
	Version         int64               `json:"version"`
	Queue           []QueueItem         `json:"queue"`
	Progress        []Progress          `json:"progress"`
	Findings        []Finding           `json:"findings"`
	Discussion      []Message           `json:"discussion"`
	Handoffs        []Handoff           `json:"handoffs"`
	Coverage        map[string][]string `json:"coverage"`
	Conflicts       []string            `json:"conflicts"`
	Blockers        []string            `json:"blockers"`
	AuthorityNotice string              `json:"authority_notice"`
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
func (s *Store) path(repo, pull string) string { return filepath.Join(s.root, repo, pull+".json") }
func (s *Store) read(repo, pull string) (Workspace, error) {
	b, e := os.ReadFile(s.path(repo, pull))
	if errors.Is(e, os.ErrNotExist) {
		return Workspace{}, ErrNotFound
	}
	var x Workspace
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) write(x Workspace) error {
	b, _ := json.MarshalIndent(x, "", "  ")
	p := s.path(x.RepositoryID, x.PullRequestID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	return os.WriteFile(p, b, 0640)
}
func itemID(area, kind, ref string) string { return area + ":" + kind + ":" + ref }
func (s *Store) Open(repo, pull string, plan reviewplans.Version, routing reviewrouting.Routing) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, pull)
	if e == nil && (x.PlanVersion != plan.Number || x.Revision != plan.Revision) {
		return Workspace{}, ErrConflict
	}
	if errors.Is(e, ErrNotFound) {
		x = Workspace{RepositoryID: repo, PullRequestID: pull, PlanVersion: plan.Number, Revision: plan.Revision, Coverage: map[string][]string{}, AuthorityNotice: "Shared review work grants no repository, approval, merge, secret, policy, governance, or operational authority. Agent findings are proposals and cannot satisfy a required human role."}
		for _, a := range plan.Areas {
			for _, p := range a.Paths {
				x.Queue = append(x.Queue, QueueItem{ID: itemID(a.ID, "file", p), AreaID: a.ID, Kind: "file", Reference: p, Revision: plan.Revision, Paths: []string{p}, DependsOn: a.DependsOn, Accessible: true})
				x.Queue = append(x.Queue, QueueItem{ID: itemID(a.ID, "diff", p), AreaID: a.ID, Kind: "diff", Reference: p, Revision: plan.Revision, Paths: []string{p}, DependsOn: a.DependsOn, Accessible: true})
			}
			for i, q := range a.Questions {
				x.Queue = append(x.Queue, QueueItem{ID: itemID(a.ID, "requirement", fmt.Sprint(i)), AreaID: a.ID, Kind: "requirement", Reference: fmt.Sprint(i), Revision: plan.Revision, Question: q, DependsOn: a.DependsOn, Accessible: true})
			}
			for i, v := range a.Evidence {
				x.Queue = append(x.Queue, QueueItem{ID: itemID(a.ID, v.Kind, fmt.Sprint(i)), AreaID: a.ID, Kind: v.Kind, Reference: v.Reference, Revision: plan.Revision, Question: v.Description, DependsOn: a.DependsOn, Accessible: v.Reference != "inaccessible"})
			}
		}
		for _, c := range append(append([]reviewplans.Context{}, plan.Context...), plan.Commitments...) {
			for _, a := range plan.Areas {
				kind := c.Kind
				if kind == "decision" {
					kind = "prior_decision"
				}
				revision := c.Revision
				if revision == "" {
					revision = plan.Revision
				}
				x.Queue = append(x.Queue, QueueItem{ID: itemID(a.ID, kind, c.Reference), AreaID: a.ID, Kind: kind, Reference: c.Reference, Revision: revision, DependsOn: a.DependsOn, Accessible: c.Accessible})
			}
		}
		sort.Slice(x.Queue, func(i, j int) bool { return x.Queue[i].ID < x.Queue[j].ID })
		e = s.write(x)
	}
	if e != nil {
		return Workspace{}, e
	}
	derive(&x, routing)
	return x, nil
}
func assignment(r reviewrouting.Routing, id, actor string) (reviewrouting.Assignment, bool) {
	for _, a := range r.Assignments {
		if a.ID == id && a.ParticipantID == actor && a.State == "accepted" && a.PlanVersion == r.PlanVersion && a.Revision == r.Revision {
			return a, true
		}
	}
	return reviewrouting.Assignment{}, false
}
func validCitation(c Citation, revision string) bool {
	return c.Kind != "" && c.Reference != "" && c.Revision != "" && c.Summary != "" && c.Accessible && (c.Audience == "public" || c.Audience == "repository") && (c.Revision == revision || c.Kind == "prior_decision")
}
func idsExist(items []QueueItem, ids []string, area string) bool {
	for _, id := range ids {
		ok := false
		for _, q := range items {
			if q.ID == id && q.AreaID == area && q.Accessible {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return len(ids) > 0
}
func (s *Store) mutate(repo, pull string, expected int64, fn func(*Workspace) error) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, pull)
	if e != nil {
		return x, e
	}
	if x.Version != expected {
		return x, ErrConflict
	}
	if e = fn(&x); e != nil {
		return x, e
	}
	x.Version++
	derive(&x, reviewrouting.Routing{})
	return x, s.write(x)
}
func (s *Store) RecordProgress(repo, pull, actor string, r reviewrouting.Routing, assignmentID, state string, itemIDs, coverage, uncertainty []string, expected int64) (Workspace, error) {
	a, ok := assignment(r, assignmentID, actor)
	if !ok || !map[string]bool{"not_started": true, "investigating": true, "blocked": true, "ready_for_handoff": true, "complete": true}[state] {
		return Workspace{}, ErrInvalid
	}
	return s.mutate(repo, pull, expected, func(x *Workspace) error {
		if !idsExist(x.Queue, itemIDs, a.AreaID) {
			return ErrInvalid
		}
		p := Progress{assignmentID, actor, state, itemIDs, clean(coverage), clean(uncertainty), s.now().UTC()}
		for i := range x.Progress {
			if x.Progress[i].AssignmentID == assignmentID {
				x.Progress[i] = p
				return nil
			}
		}
		x.Progress = append(x.Progress, p)
		return nil
	})
}
func (s *Store) AddFinding(repo, pull, actor string, r reviewrouting.Routing, assignmentID, summary, severity, conclusion string, citations []Citation, uncertainty []string, expected int64) (Workspace, error) {
	a, ok := assignment(r, assignmentID, actor)
	if !ok || summary == "" || !map[string]bool{"info": true, "low": true, "medium": true, "high": true, "critical": true}[severity] || !map[string]bool{"concern": true, "no_issue": true, "uncertain": true}[conclusion] || len(citations) == 0 {
		return Workspace{}, ErrInvalid
	}
	return s.mutate(repo, pull, expected, func(x *Workspace) error {
		for _, c := range citations {
			if !validCitation(c, x.Revision) {
				return ErrInvalid
			}
		}
		now := s.now().UTC()
		status := "published"
		if a.Kind == "agent" {
			status = "proposed_by_agent"
		}
		x.Findings = append(x.Findings, Finding{ID: fmt.Sprintf("finding-%d", now.UnixNano()), AreaID: a.AreaID, AssignmentID: a.ID, ActorID: actor, ActorKind: a.Kind, Summary: summary, Severity: severity, Conclusion: conclusion, Citations: citations, Uncertainty: clean(uncertainty), Status: status, CreatedAt: now})
		return nil
	})
}
func (s *Store) AddMessage(repo, pull, actor, area, kind, body string, r reviewrouting.Routing, assignmentID string, findings []string, citations []Citation, expected int64) (Workspace, error) {
	a, ok := assignment(r, assignmentID, actor)
	if !ok || a.AreaID != area || body == "" || !map[string]bool{"discussion": true, "input_request": true, "answer": true, "challenge": true}[kind] {
		return Workspace{}, ErrInvalid
	}
	return s.mutate(repo, pull, expected, func(x *Workspace) error {
		if !findingIDsExist(x.Findings, findings, area) {
			return ErrInvalid
		}
		for _, c := range citations {
			if !validCitation(c, x.Revision) {
				return ErrInvalid
			}
		}
		now := s.now().UTC()
		x.Discussion = append(x.Discussion, Message{fmt.Sprintf("message-%d", now.UnixNano()), area, actor, kind, body, clean(findings), citations, now})
		return nil
	})
}
func (s *Store) Handoff(repo, pull, actor string, r reviewrouting.Routing, from, to, reason string, items, findings, uncertainty []string, expected int64) (Workspace, error) {
	a, ok := assignment(r, from, actor)
	b, ok2 := assignmentByID(r, to)
	if !ok || !ok2 || a.AreaID != b.AreaID || a.ID == b.ID || reason == "" {
		return Workspace{}, ErrInvalid
	}
	return s.mutate(repo, pull, expected, func(x *Workspace) error {
		if !idsExist(x.Queue, items, a.AreaID) {
			return ErrInvalid
		}
		if !findingIDsExist(x.Findings, findings, a.AreaID) {
			return ErrInvalid
		}
		now := s.now().UTC()
		x.Handoffs = append(x.Handoffs, Handoff{ID: fmt.Sprintf("handoff-%d", now.UnixNano()), AreaID: a.AreaID, FromAssignmentID: from, ToAssignmentID: to, ActorID: actor, QueueItemIDs: clean(items), FindingIDs: clean(findings), Reason: reason, ResidualUncertainty: clean(uncertainty), State: "requested", CreatedAt: now})
		return nil
	})
}
func findingIDsExist(all []Finding, ids []string, area string) bool {
	for _, id := range ids {
		found := false
		for _, f := range all {
			if f.ID == id && f.AreaID == area {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func assignmentByID(r reviewrouting.Routing, id string) (reviewrouting.Assignment, bool) {
	for _, a := range r.Assignments {
		if a.ID == id && a.State == "accepted" && a.PlanVersion == r.PlanVersion && a.Revision == r.Revision {
			return a, true
		}
	}
	return reviewrouting.Assignment{}, false
}
func (s *Store) AcceptHandoff(repo, pull, actor, id string, r reviewrouting.Routing, expected int64) (Workspace, error) {
	return s.mutate(repo, pull, expected, func(x *Workspace) error {
		for i := range x.Handoffs {
			h := &x.Handoffs[i]
			if h.ID == id && h.State == "requested" {
				a, ok := assignmentByID(r, h.ToAssignmentID)
				if !ok || a.ParticipantID != actor {
					return ErrInvalid
				}
				now := s.now().UTC()
				h.State = "accepted"
				h.AcceptedAt = &now
				return nil
			}
		}
		return ErrNotFound
	})
}
func clean(xs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
func derive(x *Workspace, r reviewrouting.Routing) {
	x.Coverage = map[string][]string{}
	x.Conflicts = nil
	x.Blockers = nil
	for _, p := range x.Progress {
		for _, q := range p.QueueItemIDs {
			x.Coverage[q] = append(x.Coverage[q], p.AssignmentID)
		}
		if p.State == "blocked" {
			x.Blockers = append(x.Blockers, p.AssignmentID+": reviewer blocked")
		}
	}
	for q, as := range x.Coverage {
		if len(as) > 1 {
			x.Conflicts = append(x.Conflicts, q+": overlapping work by "+strings.Join(as, ", "))
		}
	}
	for i, a := range x.Findings {
		for j := i + 1; j < len(x.Findings); j++ {
			b := x.Findings[j]
			if a.AreaID == b.AreaID && a.Conclusion != b.Conclusion {
				x.Conflicts = append(x.Conflicts, a.ID+" conflicts with "+b.ID)
			}
		}
	}
	sort.Strings(x.Conflicts)
}
