// Package learningexercises retains isolated, unpublished practice attempts.
package learningexercises

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/learningpathways"
)

var ErrNotFound = errors.New("learning exercise attempt not found")
var ErrInvalid = errors.New("invalid learning exercise attempt")
var ErrForbidden = errors.New("learning exercise attempt belongs to another learner")
var ErrTerminal = errors.New("learning exercise attempt is terminal")

type Bounds struct {
	Network               string  `json:"network"`
	Credentials           bool    `json:"credentials"`
	ProductionData        bool    `json:"production_data"`
	AuthoritativeBranches bool    `json:"authoritative_branches"`
	MaximumCommands       int     `json:"maximum_commands"`
	MaximumCost           float64 `json:"maximum_cost"`
}
type Event struct {
	Number     int       `json:"number"`
	Kind       string    `json:"kind"`
	Summary    string    `json:"summary"`
	Command    string    `json:"command,omitempty"`
	Output     string    `json:"output,omitempty"`
	Digest     string    `json:"digest,omitempty"`
	Cost       float64   `json:"cost,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
}
type Citation struct {
	Kind       string `json:"kind"`
	Label      string `json:"label"`
	Path       string `json:"path,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
	Revision   string `json:"revision"`
}
type HelpEntry struct {
	Number             int        `json:"number"`
	Kind               string     `json:"kind"`
	AuthorID           string     `json:"author_id"`
	AuthorKind         string     `json:"author_kind"`
	RecipientID        string     `json:"recipient_id,omitempty"`
	GuidanceKind       string     `json:"guidance_kind,omitempty"`
	Body               string     `json:"body,omitempty"`
	SharedEventNumbers []int      `json:"shared_event_numbers,omitempty"`
	SharedEvents       []Event    `json:"shared_events,omitempty"`
	Citations          []Citation `json:"citations,omitempty"`
	AgentApprovalID    string     `json:"agent_approval_id,omitempty"`
	WorkspaceAccess    string     `json:"workspace_access,omitempty"`
	LearnerAuthorized  bool       `json:"learner_authorized"`
	RecordedAt         time.Time  `json:"recorded_at"`
}
type Attempt struct {
	ID                    string                      `json:"id"`
	RepositoryID          string                      `json:"repository_id"`
	PathwayID             string                      `json:"pathway_id"`
	PathwayVersion        int64                       `json:"pathway_version"`
	ModuleID              string                      `json:"module_id"`
	ExerciseIndex         int                         `json:"exercise_index"`
	LearnerID             string                      `json:"learner_id"`
	Revision              string                      `json:"revision"`
	Detached              bool                        `json:"detached"`
	Published             bool                        `json:"published"`
	Status                string                      `json:"status"`
	Exercise              learningpathways.Exercise   `json:"exercise"`
	Grounding             []learningpathways.Resource `json:"grounding"`
	Bounds                Bounds                      `json:"bounds"`
	Events                []Event                     `json:"events"`
	HelpTimeline          []HelpEntry                 `json:"help_timeline"`
	HelpParticipants      map[string]string           `json:"help_participants"`
	AgentStates           map[string]string           `json:"agent_states"`
	HintsUsed             int                         `json:"hints_used"`
	Cost                  float64                     `json:"cost"`
	Reproducible          bool                        `json:"reproducible"`
	ReproducibilityDetail string                      `json:"reproducibility_detail"`
	CreatedAt             time.Time                   `json:"created_at"`
	UpdatedAt             time.Time                   `json:"updated_at"`
}
type HelpInput struct {
	Kind               string     `json:"kind"`
	RecipientKind      string     `json:"recipient_kind"`
	RecipientID        string     `json:"recipient_id"`
	AgentApprovalID    string     `json:"agent_approval_id"`
	GuidanceKind       string     `json:"guidance_kind"`
	Body               string     `json:"body"`
	SharedEventNumbers []int      `json:"shared_event_numbers"`
	Citations          []Citation `json:"citations"`
	WorkspaceAccess    string     `json:"workspace_access"`
	LearnerAuthorized  bool       `json:"learner_authorized"`
}
type EventInput struct {
	Kind    string  `json:"kind"`
	Summary string  `json:"summary"`
	Command string  `json:"command"`
	Output  string  `json:"output"`
	Digest  string  `json:"digest"`
	Cost    float64 `json:"cost"`
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
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func id() string { b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) Create(repo, pathway string, version int64, module string, index int, learner, revision string, exercise learningpathways.Exercise, grounding []learningpathways.Resource) (Attempt, error) {
	if repo == "" || pathway == "" || version < 1 || module == "" || index < 0 || learner == "" || revision == "" {
		return Attempt{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	max := exercise.MaximumCost
	if max == 0 {
		max = 10
	}
	n := s.now().UTC()
	a := Attempt{ID: id(), RepositoryID: repo, PathwayID: pathway, PathwayVersion: version, ModuleID: module, ExerciseIndex: index, LearnerID: learner, Revision: revision, Detached: true, Published: false, Status: "active", Exercise: exercise, Grounding: grounding, Bounds: Bounds{Network: "disabled", Credentials: false, ProductionData: false, AuthoritativeBranches: false, MaximumCommands: 100, MaximumCost: max}, Events: []Event{}, HelpTimeline: []HelpEntry{}, HelpParticipants: map[string]string{}, AgentStates: map[string]string{}, CreatedAt: n, UpdatedAt: n}
	return a, s.write(a)
}

func (s *Store) Help(repo, pathway, attempt, actor string, mentors map[string]bool, in HelpInput) (Attempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pathway, attempt)
	if e != nil {
		return a, e
	}
	if !safe(in.Body) || actor == "" {
		return a, ErrInvalid
	}
	if a.HelpParticipants == nil {
		a.HelpParticipants = map[string]string{}
	}
	if a.AgentStates == nil {
		a.AgentStates = map[string]string{}
	}
	n := s.now().UTC()
	entry := HelpEntry{Number: len(a.HelpTimeline) + 1, Kind: in.Kind, AuthorID: actor, RecipientID: strings.TrimSpace(in.RecipientID), GuidanceKind: in.GuidanceKind, Body: strings.TrimSpace(in.Body), SharedEventNumbers: in.SharedEventNumbers, Citations: in.Citations, AgentApprovalID: strings.TrimSpace(in.AgentApprovalID), WorkspaceAccess: in.WorkspaceAccess, LearnerAuthorized: in.LearnerAuthorized, RecordedAt: n}
	learner := actor == a.LearnerID
	switch in.Kind {
	case "question":
		if !learner || entry.Body == "" || (in.RecipientKind != "mentor" && in.RecipientKind != "agent") || entry.RecipientID == "" {
			return a, ErrForbidden
		}
		if in.RecipientKind == "mentor" {
			if !mentors[entry.RecipientID] {
				return a, ErrForbidden
			}
			entry.AuthorKind = "learner"
			a.HelpParticipants[entry.RecipientID] = "mentor"
		} else {
			if entry.AgentApprovalID == "" {
				return a, ErrInvalid
			}
			entry.AuthorKind = "learner"
			a.HelpParticipants[entry.RecipientID] = "agent"
			a.AgentStates[entry.RecipientID] = "active"
		}
		seen := map[int]bool{}
		for _, number := range in.SharedEventNumbers {
			if number < 1 || number > len(a.Events) || seen[number] {
				return a, ErrInvalid
			}
			seen[number] = true
			entry.SharedEvents = append(entry.SharedEvents, a.Events[number-1])
		}
	case "guidance":
		role, ok := a.HelpParticipants[actor]
		if !ok {
			return a, ErrForbidden
		}
		entry.AuthorKind = role
		if role == "agent" && a.AgentStates[actor] != "active" {
			return a, ErrForbidden
		}
		allowed := map[string]bool{"explanation": true, "hint": true, "demonstration": true, "direct_action": true}
		if !allowed[in.GuidanceKind] || entry.Body == "" || len(in.Citations) == 0 {
			return a, ErrInvalid
		}
		granted := false
		for _, prior := range a.HelpTimeline {
			if prior.Kind == "question" && prior.RecipientID == actor && prior.LearnerAuthorized {
				granted = true
			}
		}
		if role == "agent" && (in.GuidanceKind == "direct_action" || protectedLearningMaterial.MatchString(entry.Body)) {
			return a, ErrForbidden
		}
		if (in.GuidanceKind == "demonstration" || in.GuidanceKind == "direct_action") && !granted {
			return a, ErrForbidden
		}
		entry.LearnerAuthorized = granted
		for _, c := range in.Citations {
			matched := false
			for _, g := range a.Grounding {
				if c.Kind == g.Kind && c.Label == g.Label && c.Revision == a.Revision && c.Revision == g.Revision && c.Path == g.Path && c.ResourceID == g.ResourceID && g.Status != "inaccessible" {
					matched = true
				}
			}
			if !matched {
				return a, ErrInvalid
			}
		}
	case "observe", "join":
		if !mentors[actor] || a.HelpParticipants[actor] != "mentor" {
			return a, ErrForbidden
		}
		entry.AuthorKind = "mentor"
		expected := "observe"
		if in.Kind == "join" {
			expected = "join"
		}
		granted := false
		for _, prior := range a.HelpTimeline {
			if prior.Kind == "question" && prior.RecipientID == actor && prior.WorkspaceAccess == expected {
				granted = true
			}
		}
		if in.WorkspaceAccess != expected || !granted {
			return a, ErrForbidden
		}
		entry.LearnerAuthorized = true
	case "guide_agent", "pause_agent", "revoke_agent":
		if !learner || entry.RecipientID == "" || a.HelpParticipants[entry.RecipientID] != "agent" {
			return a, ErrForbidden
		}
		entry.AuthorKind = "learner"
		if in.Kind == "guide_agent" {
			if entry.Body == "" || a.AgentStates[entry.RecipientID] != "active" {
				return a, ErrInvalid
			}
		} else if in.Kind == "pause_agent" {
			a.AgentStates[entry.RecipientID] = "paused"
		} else {
			a.AgentStates[entry.RecipientID] = "revoked"
		}
	default:
		return a, ErrInvalid
	}
	a.HelpTimeline = append(a.HelpTimeline, entry)
	a.UpdatedAt = n
	return a, s.write(a)
}
func (s *Store) Get(repo, pathway, id string) (Attempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, pathway, id)
}
func (s *Store) View(repo, pathway, attempt, actor string) (Attempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pathway, attempt)
	if e != nil {
		return a, e
	}
	if actor == a.LearnerID {
		return a, nil
	}
	if _, ok := a.HelpParticipants[actor]; !ok {
		return Attempt{}, ErrNotFound
	}
	// Helpers see only the learner-selected event snapshots already copied into
	// the timeline, never the remaining workspace, exercise answer surface, or
	// uncited module context.
	a.Events = nil
	a.Grounding = nil
	a.Exercise = learningpathways.Exercise{Title: a.Exercise.Title, Kinds: a.Exercise.Kinds}
	return a, nil
}
func (s *Store) List(repo, pathway, learner string) ([]Attempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, repo, pathway)
	es, e := os.ReadDir(dir)
	if errors.Is(e, fs.ErrNotExist) {
		return []Attempt{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Attempt{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		a, e := s.read(repo, pathway, strings.TrimSuffix(x.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		if a.LearnerID == learner {
			out = append(out, a)
		}
	}
	return out, nil
}

var secret = regexp.MustCompile(`(?i)(authorization:\s*bearer|-----BEGIN [A-Z ]*PRIVATE KEY-----|\b(api[_-]?key|password|secret|token)\s*[=:]\s*\S+)`)
var protectedLearningMaterial = regexp.MustCompile(`(?i)\b(answer[ -]?key|hidden assessment|hidden test|full solution|model answer)\b`)

func safe(v string) bool { return len(v) <= 16000 && !secret.MatchString(v) }
func (s *Store) Append(repo, pathway, id, learner string, in EventInput) (Attempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pathway, id)
	if e != nil {
		return a, e
	}
	if a.LearnerID != learner {
		return a, ErrForbidden
	}
	if a.Status != "active" {
		return a, ErrTerminal
	}
	allowed := map[string]bool{"setup": true, "checkpoint": true, "command": true, "output": true, "hint": true, "check": true, "recovery": true, "complete": true}
	if !allowed[in.Kind] || strings.TrimSpace(in.Summary) == "" || !safe(in.Summary) || !safe(in.Command) || !safe(in.Output) || in.Cost < 0 {
		return a, ErrInvalid
	}
	commands := 0
	for _, v := range a.Events {
		if v.Kind == "command" {
			commands++
		}
	}
	if in.Kind == "command" && commands >= a.Bounds.MaximumCommands {
		return a, ErrInvalid
	}
	if a.Cost+in.Cost > a.Bounds.MaximumCost {
		return a, ErrInvalid
	}
	if (in.Kind == "checkpoint" || in.Kind == "check") && strings.TrimSpace(in.Digest) == "" {
		return a, ErrInvalid
	}
	n := s.now().UTC()
	a.Events = append(a.Events, Event{Number: len(a.Events) + 1, Kind: in.Kind, Summary: strings.TrimSpace(in.Summary), Command: in.Command, Output: in.Output, Digest: in.Digest, Cost: in.Cost, RecordedAt: n})
	a.Cost += in.Cost
	if in.Kind == "hint" {
		a.HintsUsed++
	}
	if in.Kind == "complete" {
		setup, checkpoint, command, check := false, false, false, false
		for _, v := range a.Events {
			setup = setup || v.Kind == "setup"
			checkpoint = checkpoint || v.Kind == "checkpoint"
			command = command || v.Kind == "command"
			check = check || v.Kind == "check"
		}
		a.Reproducible = setup && checkpoint && command && check
		if a.Reproducible {
			a.Status = "completed"
			a.ReproducibilityDetail = "Setup, commands, content-addressed checkpoint, and acceptance check were retained."
		} else {
			a.Status = "incomplete"
			a.ReproducibilityDetail = "Required setup, command, checkpoint, or acceptance-check evidence is missing."
		}
	}
	a.UpdatedAt = n
	return a, s.write(a)
}
func (s *Store) read(repo, pathway, id string) (Attempt, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, pathway, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Attempt{}, ErrNotFound
	}
	var a Attempt
	if e == nil {
		e = json.Unmarshal(b, &a)
	}
	return a, e
}
func (s *Store) write(a Attempt) error {
	d := filepath.Join(s.root, a.RepositoryID, a.PathwayID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(a, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, "attempt-*.tmp")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, filepath.Join(d, a.ID+".json"))
}
