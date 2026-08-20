package agentevaluations

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PilotInput publishes a deliberately non-authoritative candidate experience.
// Actions are restricted to inspection and drafts; authoritative publication
// remains an ordinary human-controlled repository workflow.
type PilotInput struct {
	CandidateID      string            `json:"candidate_id"`
	Repositories     []string          `json:"repositories"`
	Roles            []string          `json:"roles"`
	Participants     []string          `json:"participants"`
	Tasks            []string          `json:"tasks"`
	Actions          []string          `json:"actions"`
	MaximumCost      float64           `json:"maximum_cost"`
	Currency         string            `json:"currency"`
	ExpiresAt        time.Time         `json:"expires_at"`
	ExpectedOutcomes map[string]string `json:"expected_outcomes"`
	Purpose          string            `json:"purpose"`
}

type PilotConsent struct {
	Participant string    `json:"participant"`
	State       string    `json:"state"`
	Reason      string    `json:"reason,omitempty"`
	At          time.Time `json:"at"`
}
type PilotEvent struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"`
	Summary  string    `json:"summary"`
	Actor    string    `json:"actor"`
	Cost     float64   `json:"cost"`
	Currency string    `json:"currency"`
	At       time.Time `json:"at"`
}
type PilotSession struct {
	ID                string       `json:"id"`
	Participant       string       `json:"participant"`
	RepositoryID      string       `json:"repository_id"`
	Role              string       `json:"role"`
	Task              string       `json:"task"`
	Status            string       `json:"status"`
	CandidateRevision string       `json:"candidate_revision"`
	Drafts            []string     `json:"drafts"`
	Events            []PilotEvent `json:"events"`
	Cost              float64      `json:"cost"`
	CreatedAt         time.Time    `json:"created_at"`
}
type PilotFeedback struct {
	ID                string    `json:"id"`
	Participant       string    `json:"participant"`
	SessionID         string    `json:"session_id"`
	CandidateRevision string    `json:"candidate_revision"`
	Kind              string    `json:"kind"`
	Summary           string    `json:"summary"`
	Correction        string    `json:"correction,omitempty"`
	ExpectedOutcome   string    `json:"expected_outcome,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}
type PilotAuthority struct {
	Read                  bool `json:"read"`
	Draft                 bool `json:"draft"`
	Merge                 bool `json:"merge"`
	Deploy                bool `json:"deploy"`
	Disclose              bool `json:"disclose"`
	AuthoritativeMutation bool `json:"authoritative_mutation"`
}
type Pilot struct {
	ID                string `json:"id"`
	RepositoryID      string `json:"repository_id"`
	CandidateDigest   string `json:"candidate_digest"`
	CandidateRevision string `json:"candidate_revision"`
	PullRequestID     string `json:"pull_request_id"`
	PilotInput
	CreatedBy    string          `json:"created_by"`
	CreatedAt    time.Time       `json:"created_at"`
	State        string          `json:"state"`
	PauseReasons []string        `json:"pause_reasons"`
	Spent        float64         `json:"spent"`
	Authority    PilotAuthority  `json:"authority"`
	Consents     []PilotConsent  `json:"consents"`
	Sessions     []PilotSession  `json:"sessions"`
	Feedback     []PilotFeedback `json:"feedback"`
}

func allowedPilotAction(v string) bool { return v == "read" || v == "draft" }
func pilotInputValid(in PilotInput, now time.Time) bool {
	if in.CandidateID == "" || len(in.Repositories) == 0 || len(in.Roles) == 0 || len(in.Participants) == 0 || len(in.Tasks) == 0 || len(in.Actions) == 0 || in.MaximumCost <= 0 || in.Currency == "" || !in.ExpiresAt.After(now) || in.Purpose == "" || len(in.ExpectedOutcomes) == 0 || !validList(in.Repositories) || !validList(in.Roles) || !validList(in.Participants) || !validList(in.Tasks) || !validList(in.Actions) {
		return false
	}
	for _, a := range in.Actions {
		if !allowedPilotAction(a) {
			return false
		}
	}
	for k, v := range in.ExpectedOutcomes {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			return false
		}
	}
	return true
}
func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func derivePilot(x *Pilot, now time.Time) {
	reasons := []string{}
	if !now.Before(x.ExpiresAt) {
		reasons = append(reasons, "pilot_expired")
	}
	if x.Spent >= x.MaximumCost {
		reasons = append(reasons, "budget_exhausted")
	}
	for _, c := range x.Consents {
		if c.State == "revoked" {
			reasons = append(reasons, "consent_revoked:"+c.Participant)
		}
	}
	for _, s := range x.Sessions {
		for _, e := range s.Events {
			if e.Kind == "unsafe_behavior" {
				reasons = append(reasons, "unsafe_behavior:"+s.ID)
			}
		}
	}
	if contains(x.PauseReasons, "candidate_changed") {
		reasons = append(reasons, "candidate_changed")
	}
	if contains(x.PauseReasons, "owner_paused") {
		reasons = append(reasons, "owner_paused")
	}
	sort.Strings(reasons)
	x.PauseReasons = reasons
	if len(reasons) > 0 {
		x.State = "paused"
	} else {
		x.State = "active"
	}
}

func (s *Store) CreatePilot(repo, actor string, in PilotInput) (Pilot, error) {
	now := s.now().UTC()
	if repo == "" || actor == "" || !pilotInputValid(in, now) || !contains(in.Repositories, repo) {
		return Pilot{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var c Candidate
	if s.read("candidates", in.CandidateID, &c) != nil || c.RepositoryID != repo {
		return Pilot{}, ErrNotFound
	}
	x := Pilot{ID: id("aep_"), RepositoryID: repo, CandidateDigest: c.Digest, CandidateRevision: c.Revision, PullRequestID: c.PullRequestID, PilotInput: in, CreatedBy: actor, CreatedAt: now, State: "active", Authority: PilotAuthority{Read: contains(in.Actions, "read"), Draft: contains(in.Actions, "draft")}}
	for _, p := range in.Participants {
		x.Consents = append(x.Consents, PilotConsent{Participant: p, State: "invited", At: now})
	}
	return x, s.write("pilots", x.ID, x)
}
func (s *Store) mutatePilot(repo, pilot string, fn func(*Pilot) error) (Pilot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Pilot
	if s.read("pilots", pilot, &x) != nil || x.RepositoryID != repo {
		return x, ErrNotFound
	}
	if e := fn(&x); e != nil {
		return x, e
	}
	derivePilot(&x, s.now().UTC())
	return x, s.write("pilots", x.ID, x)
}
func (s *Store) GetPilot(repo, pilot string) (Pilot, error) {
	return s.mutatePilot(repo, pilot, func(*Pilot) error { return nil })
}
func (s *Store) ListPilots(repo string) ([]Pilot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, "pilots"))
	if e != nil {
		return nil, e
	}
	out := []Pilot{}
	for _, f := range es {
		var x Pilot
		if s.read("pilots", strings.TrimSuffix(f.Name(), ".json"), &x) == nil && x.RepositoryID == repo {
			derivePilot(&x, s.now().UTC())
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) SetPilotConsent(repo, pilot, participant, state, reason string) (Pilot, error) {
	if state != "accepted" && state != "revoked" {
		return Pilot{}, ErrInvalid
	}
	return s.mutatePilot(repo, pilot, func(x *Pilot) error {
		for i := range x.Consents {
			if x.Consents[i].Participant == participant {
				x.Consents[i] = PilotConsent{Participant: participant, State: state, Reason: reason, At: s.now().UTC()}
				return nil
			}
		}
		return ErrInvalid
	})
}
func (s *Store) ReconcilePilotCandidate(repo, pilot, currentCandidate string) (Pilot, error) {
	return s.mutatePilot(repo, pilot, func(x *Pilot) error {
		if currentCandidate != "" && currentCandidate != x.CandidateID && !contains(x.PauseReasons, "candidate_changed") {
			x.PauseReasons = append(x.PauseReasons, "candidate_changed")
		}
		return nil
	})
}

type PilotSessionInput struct {
	RepositoryID string `json:"repository_id"`
	Role         string `json:"role"`
	Task         string `json:"task"`
}

func (s *Store) StartPilotSession(repo, pilot, participant string, in PilotSessionInput) (Pilot, error) {
	return s.mutatePilot(repo, pilot, func(x *Pilot) error {
		derivePilot(x, s.now().UTC())
		if x.State != "active" || !contains(x.Participants, participant) || !contains(x.Repositories, in.RepositoryID) || !contains(x.Roles, in.Role) || !contains(x.Tasks, in.Task) {
			return ErrInvalid
		}
		accepted := false
		for _, c := range x.Consents {
			if c.Participant == participant && c.State == "accepted" {
				accepted = true
			}
		}
		if !accepted {
			return ErrInvalid
		}
		x.Sessions = append(x.Sessions, PilotSession{ID: id("aeps_"), Participant: participant, RepositoryID: in.RepositoryID, Role: in.Role, Task: in.Task, Status: "running", CandidateRevision: x.CandidateRevision, CreatedAt: s.now().UTC()})
		return nil
	})
}

type PilotEventInput struct {
	Kind     string  `json:"kind"`
	Summary  string  `json:"summary"`
	Draft    string  `json:"draft"`
	Cost     float64 `json:"cost"`
	Currency string  `json:"currency"`
}

func (s *Store) RecordPilotEvent(repo, pilot, session, actor string, in PilotEventInput) (Pilot, error) {
	allowed := map[string]bool{"guidance": true, "draft": true, "escalation": true, "policy_denial": true, "unsafe_behavior": true, "stopped": true}
	if !allowed[in.Kind] || in.Summary == "" || in.Cost < 0 {
		return Pilot{}, ErrInvalid
	}
	return s.mutatePilot(repo, pilot, func(x *Pilot) error {
		derivePilot(x, s.now().UTC())
		if x.State != "active" && in.Kind != "stopped" {
			return ErrInvalid
		}
		for i := range x.Sessions {
			q := &x.Sessions[i]
			if q.ID != session {
				continue
			}
			if actor != q.Participant && actor != x.CreatedBy {
				return ErrInvalid
			}
			if q.Status != "running" {
				return ErrInvalid
			}
			if in.Currency != "" && in.Currency != x.Currency {
				return ErrInvalid
			}
			if in.Kind == "draft" {
				if !x.Authority.Draft || in.Draft == "" {
					return ErrInvalid
				}
				q.Drafts = append(q.Drafts, in.Draft)
			}
			q.Events = append(q.Events, PilotEvent{ID: id("aepe_"), Kind: in.Kind, Summary: in.Summary, Actor: actor, Cost: in.Cost, Currency: x.Currency, At: s.now().UTC()})
			q.Cost += in.Cost
			x.Spent += in.Cost
			if in.Kind == "stopped" || in.Kind == "unsafe_behavior" {
				q.Status = "stopped"
			}
			return nil
		}
		return ErrNotFound
	})
}

type PilotFeedbackInput struct {
	SessionID         string `json:"session_id"`
	CandidateRevision string `json:"candidate_revision"`
	Kind              string `json:"kind"`
	Summary           string `json:"summary"`
	Correction        string `json:"correction"`
	ExpectedOutcome   string `json:"expected_outcome"`
}

func (s *Store) RecordPilotFeedback(repo, pilot, participant string, in PilotFeedbackInput) (Pilot, error) {
	if in.CandidateRevision == "" || in.Summary == "" || (in.Kind != "feedback" && in.Kind != "correction") {
		return Pilot{}, ErrInvalid
	}
	if in.Kind == "correction" && in.Correction == "" {
		return Pilot{}, ErrInvalid
	}
	return s.mutatePilot(repo, pilot, func(x *Pilot) error {
		if in.CandidateRevision != x.CandidateRevision {
			return ErrInvalid
		}
		found := false
		for _, q := range x.Sessions {
			if q.ID == in.SessionID && q.Participant == participant {
				found = true
			}
		}
		if !found {
			return ErrInvalid
		}
		x.Feedback = append(x.Feedback, PilotFeedback{ID: id("aepf_"), Participant: participant, SessionID: in.SessionID, CandidateRevision: in.CandidateRevision, Kind: in.Kind, Summary: in.Summary, Correction: in.Correction, ExpectedOutcome: in.ExpectedOutcome, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) ControlPilot(repo, pilot, actor, action, reason string) (Pilot, error) {
	return s.mutatePilot(repo, pilot, func(x *Pilot) error {
		if actor != x.CreatedBy || reason == "" {
			return ErrInvalid
		}
		switch action {
		case "pause":
			if !contains(x.PauseReasons, "owner_paused") {
				x.PauseReasons = append(x.PauseReasons, "owner_paused")
			}
		case "resume":
			out := []string{}
			for _, r := range x.PauseReasons {
				if r != "owner_paused" {
					out = append(out, r)
				}
			}
			x.PauseReasons = out
		default:
			return ErrInvalid
		}
		return nil
	})
}
