// Package reviewcompletion derives revision-exact, area-by-area review readiness.
package reviewcompletion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewrouting"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewwork"
)

var ErrInvalid = errors.New("invalid review completion")
var ErrConflict = errors.New("review completion changed")

type Acknowledgement struct {
	AreaID       string    `json:"area_id"`
	ActorID      string    `json:"actor_id"`
	AssignmentID string    `json:"assignment_id"`
	InputKey     string    `json:"input_key"`
	Decision     string    `json:"decision"`
	Rationale    string    `json:"rationale"`
	CreatedAt    time.Time `json:"created_at"`
}
type Override struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actor_id"`
	Reason    string    `json:"reason"`
	FollowUp  string    `json:"follow_up"`
	AreaIDs   []string  `json:"area_ids"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
type Record struct {
	RepositoryID     string            `json:"repository_id"`
	PullRequestID    string            `json:"pull_request_id"`
	Version          int64             `json:"version"`
	RequiredAreaIDs  []string          `json:"required_area_ids"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	Overrides        []Override        `json:"overrides"`
}
type Area struct {
	ID                       string            `json:"id"`
	Name                     string            `json:"name"`
	Required                 bool              `json:"required"`
	InputKey                 string            `json:"input_key"`
	Owners                   []string          `json:"owners"`
	Assignments              []string          `json:"assignments"`
	Evidence                 []string          `json:"evidence_inspected"`
	FindingIDs               []string          `json:"finding_ids"`
	Decisions                []string          `json:"decisions"`
	RequiredAcknowledgements []string          `json:"required_acknowledgements"`
	AcknowledgedBy           []string          `json:"acknowledged_by"`
	Gaps                     []string          `json:"unresolved_gaps"`
	StaleApprovals           []Acknowledgement `json:"stale_approvals"`
	Complete                 bool              `json:"complete"`
	Override                 *Override         `json:"override,omitempty"`
}
type View struct {
	Version         int64    `json:"version"`
	Revision        string   `json:"revision"`
	TargetRevision  string   `json:"target_revision"`
	Ready           bool     `json:"ready"`
	Areas           []Area   `json:"areas"`
	Blockers        []string `json:"blockers"`
	AuthorityNotice string   `json:"authority_notice"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	p, e := filepath.Abs(root)
	if root == "" {
		return nil, ErrInvalid
	}
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, now: time.Now}, e
}
func (s *Store) path(repo, pull string) string { return filepath.Join(s.root, repo, pull+".json") }
func (s *Store) read(repo, pull string) Record {
	var x Record
	b, e := os.ReadFile(s.path(repo, pull))
	if e == nil {
		_ = json.Unmarshal(b, &x)
	}
	if x.RepositoryID == "" {
		x.RepositoryID = repo
		x.PullRequestID = pull
	}
	return x
}
func (s *Store) write(x Record) error {
	b, _ := json.MarshalIndent(x, "", "  ")
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.PullRequestID)), 0750); e != nil {
		return e
	}
	return os.WriteFile(s.path(x.RepositoryID, x.PullRequestID), b, 0640)
}
func clean(xs []string) []string {
	m := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !m[x] {
			m[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
func (s *Store) SetRequired(repo, pull, actor string, ids []string, expected int64) (Record, error) {
	if actor == "" || len(clean(ids)) == 0 {
		return Record{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.read(repo, pull)
	if x.Version != expected {
		return x, ErrConflict
	}
	x.RequiredAreaIDs = clean(ids)
	x.Version++
	return x, s.write(x)
}
func inputKey(v reviewplans.Version, a reviewplans.Area) string {
	b, _ := json.Marshal(struct {
		Revision, Target, Risk             string
		Owners, Dependencies, Paths, Rules []string
	}{v.Revision, v.TargetRevision, v.Risk, clean(a.OwnerIDs), clean(a.DependsOn), clean(a.Paths), clean(a.CompletionRules)})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s *Store) Acknowledge(repo, pull, actor, area, assignment, decision, rationale string, v reviewplans.Version, r reviewrouting.Routing, expected int64) (Record, error) {
	if decision != "approve" && decision != "request_changes" || rationale == "" {
		return Record{}, ErrInvalid
	}
	var ar *reviewplans.Area
	for i := range v.Areas {
		if v.Areas[i].ID == area {
			ar = &v.Areas[i]
		}
	}
	valid := false
	for _, a := range r.Assignments {
		if a.ID == assignment && a.AreaID == area && a.ParticipantID == actor && a.Kind == "human" && a.State == "accepted" && a.Revision == v.Revision {
			valid = true
		}
	}
	if ar == nil || !valid {
		return Record{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.read(repo, pull)
	if x.Version != expected {
		return x, ErrConflict
	}
	x.Acknowledgements = append(x.Acknowledgements, Acknowledgement{area, actor, assignment, inputKey(v, *ar), decision, rationale, s.now().UTC()})
	x.Version++
	return x, s.write(x)
}
func (s *Store) Override(repo, pull, actor, reason, follow string, areas []string, expires time.Time, expected int64) (Record, error) {
	if actor == "" || reason == "" || follow == "" || len(clean(areas)) == 0 || !expires.After(s.now()) || expires.After(s.now().Add(7*24*time.Hour)) {
		return Record{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.read(repo, pull)
	if x.Version != expected {
		return x, ErrConflict
	}
	now := s.now().UTC()
	x.Overrides = append(x.Overrides, Override{hex.EncodeToString([]byte(now.Format(time.RFC3339Nano))), actor, reason, follow, clean(areas), expires.UTC(), now})
	x.Version++
	return x, s.write(x)
}
func has(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func (s *Store) View(repo, pull string, v reviewplans.Version, r reviewrouting.Routing, w reviewwork.Workspace) View {
	s.mu.Lock()
	x := s.read(repo, pull)
	s.mu.Unlock()
	out := View{Version: x.Version, Revision: v.Revision, TargetRevision: v.TargetRevision, Ready: true, AuthorityNotice: "Review completion records evidence and bounded decisions; it grants no review, approval, merge, policy, repository, or operational authority."}
	now := s.now()
	for _, pa := range v.Areas {
		a := Area{ID: pa.ID, Name: pa.Name, Required: has(x.RequiredAreaIDs, pa.ID), InputKey: inputKey(v, pa), Owners: pa.OwnerIDs}
		human := map[string]bool{}
		for _, as := range r.Assignments {
			if as.AreaID == pa.ID && as.State == "accepted" && as.Revision == v.Revision {
				a.Assignments = append(a.Assignments, as.ParticipantID)
				if as.Kind == "human" {
					human[as.ParticipantID] = true
				}
			}
		}
		for _, q := range w.Queue {
			if q.AreaID == pa.ID && len(w.Coverage[q.ID]) > 0 {
				a.Evidence = append(a.Evidence, q.ID)
			} else if q.AreaID == pa.ID && q.Accessible {
				a.Gaps = append(a.Gaps, "evidence_not_inspected:"+q.ID)
			}
		}
		for _, f := range w.Findings {
			if f.AreaID == pa.ID {
				a.FindingIDs = append(a.FindingIDs, f.ID)
				if f.Status == "open" || f.Status == "proposed_by_agent" || f.Status == "challenged" || f.Status == "deferred" {
					a.Gaps = append(a.Gaps, "unresolved_finding:"+f.ID)
				}
			}
		}
		for _, d := range w.Decisions {
			for _, f := range w.Findings {
				if d.FindingID == f.ID && f.AreaID == pa.ID {
					a.Decisions = append(a.Decisions, d.Classification+":"+d.FindingID)
				}
			}
		}
		for owner := range human {
			a.RequiredAcknowledgements = append(a.RequiredAcknowledgements, owner)
		}
		for _, ack := range x.Acknowledgements {
			if ack.AreaID != pa.ID {
				continue
			}
			if ack.InputKey != a.InputKey {
				a.StaleApprovals = append(a.StaleApprovals, ack)
				continue
			}
			if ack.Decision == "approve" {
				a.AcknowledgedBy = append(a.AcknowledgedBy, ack.ActorID)
			} else {
				a.Gaps = append(a.Gaps, "changes_requested:"+ack.ActorID)
			}
		}
		for _, owner := range a.RequiredAcknowledgements {
			if !has(a.AcknowledgedBy, owner) {
				a.Gaps = append(a.Gaps, "acknowledgement_required:"+owner)
			}
		}
		if len(human) == 0 {
			a.Gaps = append(a.Gaps, "human_assignment_required")
		}
		for i := len(x.Overrides) - 1; i >= 0; i-- {
			o := x.Overrides[i]
			if has(o.AreaIDs, pa.ID) && o.ExpiresAt.After(now) {
				a.Override = &o
				break
			}
		}
		a.Complete = len(a.Gaps) == 0 || a.Override != nil
		if a.Required && !a.Complete {
			out.Ready = false
			out.Blockers = append(out.Blockers, "review_area_incomplete:"+a.ID)
		}
		out.Areas = append(out.Areas, a)
	}
	return out
}
