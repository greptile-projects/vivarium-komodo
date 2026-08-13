// Package productlearning retains reciprocal, consent-aware learning after delivery.
package productlearning

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("product learning record not found")
var ErrInvalid = errors.New("invalid product learning record")
var ErrConflict = errors.New("product learning conflict")

type Link struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Label      string `json:"label"`
	Public     bool   `json:"public"`
}
type UpdateInput struct {
	Kind           string   `json:"kind"`
	Summary        string   `json:"summary"`
	Rationale      string   `json:"rationale"`
	Audience       string   `json:"audience"`
	FeedbackIDs    []string `json:"feedback_ids"`
	StakeholderIDs []string `json:"stakeholder_ids"`
	Links          []Link   `json:"links"`
}
type Update struct {
	ID string `json:"id"`
	UpdateInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type ResponseInput struct {
	Outcome  string   `json:"outcome"`
	Body     string   `json:"body"`
	Evidence []string `json:"evidence"`
	Dissent  bool     `json:"dissent"`
}
type Response struct {
	ID         string `json:"id"`
	UpdateID   string `json:"update_id"`
	FeedbackID string `json:"feedback_id"`
	ResponseInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Departure struct {
	ActorID    string    `json:"actor_id"`
	FeedbackID string    `json:"feedback_id,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
type LessonInput struct {
	ExpectedOutcomes       []string `json:"expected_outcomes"`
	ObservedOutcomes       []string `json:"observed_outcomes"`
	Lessons                []string `json:"lessons"`
	Dissent                []string `json:"dissent"`
	ResultingWork          []Link   `json:"resulting_work"`
	RoadmapID              string   `json:"roadmap_id"`
	RoadmapVersion         int64    `json:"roadmap_version"`
	OpportunityDisposition string   `json:"opportunity_disposition"`
	ChangeReason           string   `json:"change_reason"`
	ExpectedRevision       int64    `json:"expected_revision"`
}
type Lesson struct {
	Version int64 `json:"version"`
	LessonInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Record struct {
	ID                   string      `json:"id"`
	RepositoryID         string      `json:"repository_id"`
	DeliveryID           string      `json:"delivery_id"`
	RoadmapID            string      `json:"roadmap_id"`
	OutcomeID            string      `json:"outcome_id"`
	OpportunityID        string      `json:"opportunity_id"`
	OpportunityVersion   int64       `json:"opportunity_version"`
	Updates              []Update    `json:"updates"`
	Responses            []Response  `json:"responses"`
	Departures           []Departure `json:"departures"`
	Lessons              []Lesson    `json:"lessons"`
	CurrentRevision      int64       `json:"current_revision"`
	OperationalAuthority bool        `json:"operational_authority"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`
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
func (s *Store) Ensure(repo, delivery, roadmap, outcome, opp string, ov int64) (Record, error) {
	if repo == "" || delivery == "" || roadmap == "" || outcome == "" || opp == "" || ov < 1 {
		return Record{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, e := s.read(repo, delivery); e == nil {
		return v, nil
	}
	n := s.now().UTC()
	v := Record{ID: "learning_" + token(), RepositoryID: repo, DeliveryID: delivery, RoadmapID: roadmap, OutcomeID: outcome, OpportunityID: opp, OpportunityVersion: ov, Updates: []Update{}, Responses: []Response{}, Departures: []Departure{}, Lessons: []Lesson{}, CreatedAt: n, UpdatedAt: n}
	return v, s.write(v)
}
func (s *Store) Get(repo, delivery string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, delivery)
}
func (s *Store) Publish(repo, delivery, actor string, in UpdateInput) (Record, error) {
	if actor == "" || !validUpdate(in) {
		return Record{}, ErrInvalid
	}
	return s.mutate(repo, delivery, func(v *Record) error {
		n := s.now().UTC()
		v.Updates = append(v.Updates, Update{ID: "update_" + token(), UpdateInput: in, AuthorID: actor, CreatedAt: n})
		v.UpdatedAt = n
		return nil
	})
}
func (s *Store) Respond(repo, delivery, update, feedback, actor string, in ResponseInput) (Record, error) {
	if actor == "" || feedback == "" || !map[string]bool{"improved": true, "not_improved": true, "mixed": true, "unknown": true}[in.Outcome] || strings.TrimSpace(in.Body) == "" || len(in.Body) > 65536 || len(in.Evidence) > 20 {
		return Record{}, ErrInvalid
	}
	return s.mutate(repo, delivery, func(v *Record) error {
		found := false
		for _, u := range v.Updates {
			if u.ID == update && contains(u.FeedbackIDs, feedback) {
				found = true
			}
		}
		if !found {
			return ErrNotFound
		}
		for _, x := range v.Responses {
			if x.UpdateID == update && x.FeedbackID == feedback {
				return ErrConflict
			}
		}
		n := s.now().UTC()
		v.Responses = append(v.Responses, Response{ID: "response_" + token(), UpdateID: update, FeedbackID: feedback, ResponseInput: in, AuthorID: actor, CreatedAt: n})
		v.UpdatedAt = n
		return nil
	})
}
func (s *Store) Leave(repo, delivery, actor, feedback, reason string) (Record, error) {
	if actor == "" || len(reason) > 2000 {
		return Record{}, ErrInvalid
	}
	return s.mutate(repo, delivery, func(v *Record) error {
		for _, x := range v.Departures {
			if x.ActorID == actor && x.FeedbackID == feedback {
				return ErrConflict
			}
		}
		n := s.now().UTC()
		v.Departures = append(v.Departures, Departure{ActorID: actor, FeedbackID: feedback, Reason: strings.TrimSpace(reason), CreatedAt: n})
		v.UpdatedAt = n
		return nil
	})
}
func (s *Store) RecordLesson(repo, delivery, actor string, in LessonInput) (Record, error) {
	if actor == "" || in.ExpectedRevision < 0 || len(in.ExpectedOutcomes) == 0 || len(in.ObservedOutcomes) == 0 || len(in.Lessons) == 0 || strings.TrimSpace(in.ChangeReason) == "" || !map[string]bool{"open": true, "fulfilled": true, "unsupported": true}[in.OpportunityDisposition] || in.RoadmapID == "" || in.RoadmapVersion < 1 {
		return Record{}, ErrInvalid
	}
	return s.mutate(repo, delivery, func(v *Record) error {
		if v.CurrentRevision != in.ExpectedRevision {
			return ErrConflict
		}
		n := s.now().UTC()
		v.CurrentRevision++
		v.Lessons = append(v.Lessons, Lesson{Version: v.CurrentRevision, LessonInput: in, AuthorID: actor, CreatedAt: n})
		v.UpdatedAt = n
		return nil
	})
}
func validUpdate(in UpdateInput) bool {
	return map[string]bool{"decision": true, "preview": true, "delivery": true, "rejection": true, "measured_outcome": true}[in.Kind] && strings.TrimSpace(in.Summary) != "" && len(in.Summary) <= 4000 && strings.TrimSpace(in.Rationale) != "" && len(in.Rationale) <= 65536 && map[string]bool{"public": true, "repository": true, "participants": true}[in.Audience] && (len(in.FeedbackIDs) > 0 || len(in.StakeholderIDs) > 0) && len(in.FeedbackIDs) <= 100 && len(in.StakeholderIDs) <= 100 && len(in.Links) <= 30
}
func contains(v []string, x string) bool {
	for _, y := range v {
		if x == y {
			return true
		}
	}
	return false
}
func (s *Store) mutate(repo, d string, fn func(*Record) error) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, d)
	if e != nil {
		return v, e
	}
	if e = fn(&v); e != nil {
		return Record{}, e
	}
	return v, s.write(v)
}
func (s *Store) read(repo, d string) (Record, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, d+".json"))
	if e != nil {
		return Record{}, ErrNotFound
	}
	var v Record
	if json.Unmarshal(b, &v) != nil || v.RepositoryID != repo || v.DeliveryID != d {
		return Record{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) write(v Record) error {
	d := filepath.Join(s.root, v.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	t, e := os.CreateTemp(d, ".learning-*")
	if e != nil {
		return e
	}
	name := t.Name()
	defer os.Remove(name)
	if _, e = t.Write(b); e == nil {
		e = t.Chmod(0600)
	}
	if ce := t.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(name, filepath.Join(d, v.DeliveryID+".json"))
	}
	return e
}
func token() string { b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b) }
