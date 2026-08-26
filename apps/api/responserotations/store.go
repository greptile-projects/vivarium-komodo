// Package responserotations owns durable, context-preserving response duty schedules.
package responserotations

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

var ErrNotFound = errors.New("response rotation not found")
var ErrInvalid = errors.New("invalid response rotation")
var ErrConflict = errors.New("response rotation changed")
var ErrForbidden = errors.New("response rotation action forbidden")

type Participant struct {
	UserID         string   `json:"user_id"`
	TimeZone       string   `json:"time_zone"`
	Qualifications []string `json:"qualifications"`
	Available      bool     `json:"available"`
	Member         bool     `json:"member"`
	Access         bool     `json:"access"`
	Workload       int      `json:"workload"`
}
type Shift struct {
	ID                     string     `json:"id"`
	StartsAt               time.Time  `json:"starts_at"`
	EndsAt                 time.Time  `json:"ends_at"`
	PrimaryID              string     `json:"primary_id"`
	BackupLayers           [][]string `json:"backup_layers"`
	RequiredQualifications []string   `json:"required_qualifications"`
	ContextRevision        string     `json:"context_revision"`
	ContextReferences      []string   `json:"context_references"`
}
type AbsenceRule struct {
	Kind          string `json:"kind"`
	NoticeMinutes int    `json:"notice_minutes"`
	Action        string `json:"action"`
}
type Input struct {
	Name           string        `json:"name"`
	PolicyID       string        `json:"policy_id"`
	PolicyVersion  int64         `json:"policy_version"`
	TeamID         string        `json:"team_id"`
	TimeZone       string        `json:"time_zone"`
	HandoffMinutes int           `json:"handoff_minutes"`
	WorkloadLimit  int           `json:"workload_limit"`
	Participants   []Participant `json:"participants"`
	AbsenceRules   []AbsenceRule `json:"absence_rules"`
	Shifts         []Shift       `json:"shifts"`
	OwnerIDs       []string      `json:"owner_ids"`
}
type Event struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	ShiftID       string    `json:"shift_id"`
	ActorID       string    `json:"actor_id"`
	ParticipantID string    `json:"participant_id,omitempty"`
	Detail        string    `json:"detail"`
	CreatedAt     time.Time `json:"created_at"`
}
type Transfer struct {
	ID                string     `json:"id"`
	ShiftID           string     `json:"shift_id"`
	Kind              string     `json:"kind"`
	FromID            string     `json:"from_id"`
	RecipientID       string     `json:"recipient_id"`
	ContextRevision   string     `json:"context_revision"`
	ContextReferences []string   `json:"context_references"`
	Rationale         string     `json:"rationale"`
	ProposedBy        string     `json:"proposed_by"`
	ProposedAt        time.Time  `json:"proposed_at"`
	AcceptedBy        string     `json:"accepted_by,omitempty"`
	AcceptedAt        *time.Time `json:"accepted_at,omitempty"`
}
type Gap struct {
	Kind       string   `json:"kind"`
	ShiftID    string   `json:"shift_id,omitempty"`
	Detail     string   `json:"detail"`
	EscalateTo []string `json:"escalate_to"`
}
type Rotation struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Revision     int64  `json:"revision"`
	Input
	CreatedBy    string      `json:"created_by"`
	CreatedAt    time.Time   `json:"created_at"`
	Events       []Event     `json:"events"`
	Transfers    []Transfer  `json:"transfers"`
	CurrentShift *ShiftView  `json:"current_shift,omitempty"`
	Upcoming     []ShiftView `json:"upcoming"`
	Gaps         []Gap       `json:"gaps"`
	NonAuthority []string    `json:"non_authority"`
}
type ShiftView struct {
	Shift        Shift  `json:"shift"`
	ResponderID  string `json:"responder_id"`
	Acknowledged bool   `json:"acknowledged"`
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
func clean(xs []string, required bool) bool {
	if required && len(xs) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func valid(in Input) bool {
	if in.Name == "" || in.PolicyID == "" || in.PolicyVersion < 1 || in.TeamID == "" || in.HandoffMinutes < 1 || in.WorkloadLimit < 1 || !clean(in.OwnerIDs, true) || len(in.Participants) == 0 || len(in.Shifts) == 0 {
		return false
	}
	if _, e := time.LoadLocation(in.TimeZone); e != nil {
		return false
	}
	people := map[string]bool{}
	for _, p := range in.Participants {
		if p.UserID == "" || people[p.UserID] || p.Workload < 0 || !clean(p.Qualifications, false) {
			return false
		}
		if _, e := time.LoadLocation(p.TimeZone); e != nil {
			return false
		}
		people[p.UserID] = true
	}
	shifts := map[string]bool{}
	for _, s := range in.Shifts {
		if s.ID == "" || shifts[s.ID] || !s.EndsAt.After(s.StartsAt) || !people[s.PrimaryID] || s.ContextRevision == "" || !clean(s.RequiredQualifications, true) || !clean(s.ContextReferences, true) {
			return false
		}
		shifts[s.ID] = true
		for _, layer := range s.BackupLayers {
			if !clean(layer, true) {
				return false
			}
			for _, u := range layer {
				if !people[u] {
					return false
				}
			}
		}
	}
	for _, r := range in.AbsenceRules {
		if r.Kind == "" || r.NoticeMinutes < 0 || r.Action == "" {
			return false
		}
	}
	return true
}
func (s *Store) path(repo, rid string) string { return filepath.Join(s.root, repo, rid+".json") }
func (s *Store) save(r Rotation) error {
	p := s.path(r.RepositoryID, r.ID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(r, "", "  ")
	if e == nil {
		e = os.WriteFile(p, append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) read(repo, rid string) (Rotation, error) {
	b, e := os.ReadFile(s.path(repo, rid))
	if errors.Is(e, fs.ErrNotExist) {
		return Rotation{}, ErrNotFound
	}
	var r Rotation
	if e == nil {
		e = json.Unmarshal(b, &r)
	}
	return r, e
}
func member(r Rotation, u string) bool {
	for _, p := range r.Participants {
		if p.UserID == u {
			return true
		}
	}
	return false
}
func owner(r Rotation, u string) bool {
	for _, x := range r.OwnerIDs {
		if x == u {
			return true
		}
	}
	return false
}
func responder(r Rotation, sh Shift) string {
	u := sh.PrimaryID
	for _, t := range r.Transfers {
		if t.ShiftID == sh.ID && t.AcceptedAt != nil {
			u = t.RecipientID
		}
	}
	return u
}
func derive(r *Rotation, now time.Time) {
	r.CurrentShift = nil
	r.Upcoming = []ShiftView{}
	r.Gaps = []Gap{}
	people := map[string]Participant{}
	for _, p := range r.Participants {
		people[p.UserID] = p
	}
	acked := map[string]bool{}
	reported := map[string][]Event{}
	for _, e := range r.Events {
		if e.Kind == "acknowledged" {
			acked[e.ShiftID] = true
		}
		if e.Kind != "acknowledged" && e.Kind != "absence" {
			reported[e.ShiftID] = append(reported[e.ShiftID], e)
		}
	}
	ss := append([]Shift(nil), r.Shifts...)
	sort.Slice(ss, func(i, j int) bool { return ss[i].StartsAt.Before(ss[j].StartsAt) })
	for i, sh := range ss {
		u := responder(*r, sh)
		p := people[u]
		v := ShiftView{sh, u, acked[sh.ID]}
		if !now.Before(sh.StartsAt) && now.Before(sh.EndsAt) {
			x := v
			r.CurrentShift = &x
		}
		if sh.StartsAt.After(now) {
			r.Upcoming = append(r.Upcoming, v)
		}
		bad := !p.Member || !p.Access || !p.Available || p.Workload >= r.WorkloadLimit
		have := map[string]bool{}
		for _, q := range p.Qualifications {
			have[q] = true
		}
		for _, q := range sh.RequiredQualifications {
			if !have[q] {
				bad = true
				r.Gaps = append(r.Gaps, Gap{"missing_qualification", sh.ID, u + " lacks " + q, r.OwnerIDs})
			}
		}
		if !p.Member {
			r.Gaps = append(r.Gaps, Gap{"membership_changed", sh.ID, u + " is no longer a member", r.OwnerIDs})
		}
		if !p.Access {
			r.Gaps = append(r.Gaps, Gap{"access_revoked", sh.ID, u + " no longer has declared access", r.OwnerIDs})
		}
		if !p.Available {
			r.Gaps = append(r.Gaps, Gap{"unavailable_responder", sh.ID, u + " is unavailable", r.OwnerIDs})
		}
		if p.Workload >= r.WorkloadLimit {
			r.Gaps = append(r.Gaps, Gap{"workload_limit", sh.ID, u + " reached the rotation workload limit", r.OwnerIDs})
		}
		if bad && !hasEligibleBackup(sh, people, r.WorkloadLimit) {
			r.Gaps = append(r.Gaps, Gap{"uncovered_interval", sh.ID, "no eligible responder or backup covers this interval", r.OwnerIDs})
		}
		for _, e := range reported[sh.ID] {
			r.Gaps = append(r.Gaps, Gap{e.Kind, sh.ID, e.Detail, r.OwnerIDs})
		}
		if i > 0 && sh.StartsAt.Before(ss[i-1].EndsAt) {
			r.Gaps = append(r.Gaps, Gap{"overlapping_schedule", sh.ID, "shift overlaps " + ss[i-1].ID, r.OwnerIDs})
		}
		if i > 0 && sh.StartsAt.After(ss[i-1].EndsAt) {
			r.Gaps = append(r.Gaps, Gap{"uncovered_interval", sh.ID, ss[i-1].EndsAt.Format(time.RFC3339) + " to " + sh.StartsAt.Format(time.RFC3339), r.OwnerIDs})
		}
	}
}
func hasEligibleBackup(s Shift, ps map[string]Participant, limit int) bool {
	for _, l := range s.BackupLayers {
		for _, u := range l {
			p := ps[u]
			have := map[string]bool{}
			for _, q := range p.Qualifications {
				have[q] = true
			}
			qualified := true
			for _, q := range s.RequiredQualifications {
				if !have[q] {
					qualified = false
				}
			}
			if p.Member && p.Access && p.Available && p.Workload < limit && qualified {
				return true
			}
		}
	}
	return false
}
func (s *Store) Create(repo, actor string, in Input) (Rotation, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Rotation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := Rotation{ID: id(), RepositoryID: repo, Revision: 1, Input: in, CreatedBy: actor, CreatedAt: s.now().UTC(), NonAuthority: []string{"Response duty grants no repository, team, secret, communication, incident, deployment, environment, security, privacy, continuity, governance, or operational authority."}}
	derive(&r, s.now().UTC())
	return r, s.save(r)
}
func (s *Store) Get(repo, rid string) (Rotation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, e := s.read(repo, rid)
	if e == nil {
		derive(&r, s.now().UTC())
	}
	return r, e
}
func (s *Store) List(repo string) ([]Rotation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fs, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(fs)
	out := []Rotation{}
	for _, f := range fs {
		b, x := os.ReadFile(f)
		var r Rotation
		if x == nil {
			x = json.Unmarshal(b, &r)
		}
		if x != nil {
			return nil, x
		}
		derive(&r, s.now().UTC())
		out = append(out, r)
	}
	return out, nil
}

type TransferInput struct {
	ExpectedRevision  int64    `json:"expected_revision"`
	ShiftID           string   `json:"shift_id"`
	Kind              string   `json:"kind"`
	RecipientID       string   `json:"recipient_id"`
	ContextRevision   string   `json:"context_revision"`
	ContextReferences []string `json:"context_references"`
	Rationale         string   `json:"rationale"`
}

func (s *Store) Propose(repo, rid, actor string, in TransferInput) (Rotation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, e := s.read(repo, rid)
	if e != nil {
		return r, e
	}
	if r.Revision != in.ExpectedRevision {
		return r, ErrConflict
	}
	var sh *Shift
	for i := range r.Shifts {
		if r.Shifts[i].ID == in.ShiftID {
			sh = &r.Shifts[i]
		}
	}
	if sh == nil || !member(r, in.RecipientID) || !same(in.ContextReferences, sh.ContextReferences) || in.ContextRevision != sh.ContextRevision || in.Rationale == "" || (in.Kind != "swap" && in.Kind != "delegate" && in.Kind != "override") {
		return r, ErrInvalid
	}
	var recipient Participant
	for _, p := range r.Participants {
		if p.UserID == in.RecipientID {
			recipient = p
		}
	}
	if !recipient.Member || !recipient.Access || !recipient.Available || recipient.Workload >= r.WorkloadLimit {
		return r, ErrInvalid
	}
	have := map[string]bool{}
	for _, q := range recipient.Qualifications {
		have[q] = true
	}
	for _, q := range sh.RequiredQualifications {
		if !have[q] {
			return r, ErrInvalid
		}
	}
	from := responder(r, *sh)
	if actor != from && !owner(r, actor) {
		return r, ErrForbidden
	}
	r.Transfers = append(r.Transfers, Transfer{id(), in.ShiftID, in.Kind, from, in.RecipientID, in.ContextRevision, in.ContextReferences, in.Rationale, actor, s.now().UTC(), "", nil})
	r.Revision++
	derive(&r, s.now().UTC())
	return r, s.save(r)
}
func same(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func (s *Store) Accept(repo, rid, tid, actor string, expected int64) (Rotation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, e := s.read(repo, rid)
	if e != nil {
		return r, e
	}
	if r.Revision != expected {
		return r, ErrConflict
	}
	found := false
	now := s.now().UTC()
	for i := range r.Transfers {
		t := &r.Transfers[i]
		if t.ID == tid && t.AcceptedAt == nil {
			if t.RecipientID != actor {
				return r, ErrForbidden
			}
			t.AcceptedBy = actor
			t.AcceptedAt = &now
			found = true
		}
	}
	if !found {
		return r, ErrNotFound
	}
	r.Revision++
	derive(&r, now)
	return r, s.save(r)
}

type EventInput struct {
	ExpectedRevision int64  `json:"expected_revision"`
	ShiftID          string `json:"shift_id"`
	Kind             string `json:"kind"`
	Detail           string `json:"detail"`
}

func (s *Store) Record(repo, rid, actor string, in EventInput) (Rotation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, e := s.read(repo, rid)
	if e != nil {
		return r, e
	}
	if r.Revision != in.ExpectedRevision {
		return r, ErrConflict
	}
	if in.Detail == "" || (in.Kind != "acknowledged" && in.Kind != "absence" && in.Kind != "missed_handoff" && in.Kind != "membership_changed" && in.Kind != "access_revoked") {
		return r, ErrInvalid
	}
	var sh *Shift
	for i := range r.Shifts {
		if r.Shifts[i].ID == in.ShiftID {
			sh = &r.Shifts[i]
		}
	}
	if sh == nil {
		return r, ErrInvalid
	}
	u := responder(r, *sh)
	if actor != u && !owner(r, actor) {
		return r, ErrForbidden
	}
	r.Events = append(r.Events, Event{id(), in.Kind, in.ShiftID, actor, u, in.Detail, s.now().UTC()})
	r.Revision++
	derive(&r, s.now().UTC())
	return r, s.save(r)
}
