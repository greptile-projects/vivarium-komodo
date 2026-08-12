package governance

import (
	"strings"
	"time"
)

type ResourceHandoff struct {
	Resource     string     `json:"resource"`
	FromID       string     `json:"from_id"`
	ToID         string     `json:"to_id"`
	State        string     `json:"state"`
	ApprovedByID string     `json:"approved_by_id,omitempty"`
	ApprovedAt   *time.Time `json:"approved_at,omitempty"`
}

type StewardshipEvent struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type StewardshipCase struct {
	ID                string             `json:"id"`
	Kind              string             `json:"kind"`
	Role              string             `json:"role"`
	FormerStandingID  string             `json:"former_standing_id,omitempty"`
	NomineeStandingID string             `json:"nominee_standing_id,omitempty"`
	DecisionReceiptID string             `json:"decision_receipt_id,omitempty"`
	Reason            string             `json:"reason"`
	State             string             `json:"state"`
	EmergencyScope    []string           `json:"emergency_scope"`
	ExpiresAt         *time.Time         `json:"expires_at,omitempty"`
	ReviewDueAt       *time.Time         `json:"review_due_at,omitempty"`
	Appeal            string             `json:"appeal,omitempty"`
	AppealState       string             `json:"appeal_state,omitempty"`
	Handoffs          []ResourceHandoff  `json:"resource_handoffs"`
	Events            []StewardshipEvent `json:"events"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

type StewardshipInput struct {
	Kind              string            `json:"kind"`
	Role              string            `json:"role"`
	FormerStandingID  string            `json:"former_standing_id,omitempty"`
	NomineeStandingID string            `json:"nominee_standing_id,omitempty"`
	DecisionReceiptID string            `json:"decision_receipt_id,omitempty"`
	Reason            string            `json:"reason"`
	EmergencyScope    []string          `json:"emergency_scope,omitempty"`
	ExpiresAt         *time.Time        `json:"expires_at,omitempty"`
	ReviewDueAt       *time.Time        `json:"review_due_at,omitempty"`
	ResourceHandoffs  []ResourceHandoff `json:"resource_handoffs,omitempty"`
}

type GovernanceHealth struct {
	GeneratedAt           time.Time `json:"generated_at"`
	Vacancies             []string  `json:"vacancies"`
	ExpiringTerms         []string  `json:"expiring_terms"`
	UnresolvedHandoffs    []string  `json:"unresolved_handoffs"`
	QuorumLoss            []string  `json:"quorum_loss"`
	DeadlockedCases       []string  `json:"deadlocked_cases"`
	OpenAppeals           []string  `json:"open_appeals"`
	ActiveEmergencyPowers []string  `json:"active_emergency_powers"`
}

func standingIndex(v Charter, standingID string) int {
	for i := range v.Standings {
		if v.Standings[i].ID == standingID {
			return i
		}
	}
	return -1
}

func (s *Store) OpenStewardship(t, scope, actor string, in StewardshipInput) (Charter, error) {
	validKind := in.Kind == "nomination" || in.Kind == "election" || in.Kind == "term_expiry" || in.Kind == "recall" || in.Kind == "succession" || in.Kind == "deadlock" || in.Kind == "emergency"
	if !validKind || !clean(in.Role) || !clean(in.Reason) {
		return Charter{}, ErrInvalid
	}
	if in.Kind == "emergency" && (len(in.EmergencyScope) == 0 || in.ExpiresAt == nil || in.ReviewDueAt == nil || !in.ExpiresAt.After(s.now()) || in.ExpiresAt.After(s.now().Add(30*24*time.Hour))) {
		return Charter{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(t, scope)
	if err != nil {
		return v, err
	}
	if v.Current.State != "active" {
		return v, ErrConflict
	}
	if _, ok := roleFor(v, in.Role); !ok {
		return v, ErrInvalid
	}
	if in.FormerStandingID != "" && standingIndex(v, in.FormerStandingID) < 0 {
		return v, ErrInvalid
	}
	if in.NomineeStandingID != "" {
		i := standingIndex(v, in.NomineeStandingID)
		if i < 0 || v.Standings[i].State != "active" || v.Standings[i].Role != in.Role {
			return v, ErrConflict
		}
	}
	now := s.now().UTC()
	state := "open"
	if in.Kind == "emergency" {
		state = "active"
	}
	c := StewardshipCase{ID: id(), Kind: in.Kind, Role: in.Role, FormerStandingID: in.FormerStandingID, NomineeStandingID: in.NomineeStandingID, DecisionReceiptID: in.DecisionReceiptID, Reason: strings.TrimSpace(in.Reason), State: state, EmergencyScope: append([]string(nil), in.EmergencyScope...), ExpiresAt: in.ExpiresAt, ReviewDueAt: in.ReviewDueAt, Handoffs: append([]ResourceHandoff(nil), in.ResourceHandoffs...), CreatedAt: now, UpdatedAt: now}
	for i := range c.Handoffs {
		c.Handoffs[i].State = "pending_owner_approval"
	}
	c.Events = []StewardshipEvent{{Sequence: 1, Type: "opened", ActorID: actor, Reason: c.Reason, CreatedAt: now}}
	v.Stewardship = append(v.Stewardship, c)
	v.UpdatedAt = now
	return v, s.write(v)
}

func (s *Store) TransitionStewardship(t, scope, caseID, actor, action, reason, resource string) (Charter, error) {
	if !clean(reason) {
		return Charter{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(t, scope)
	if e != nil {
		return v, e
	}
	for i := range v.Stewardship {
		c := &v.Stewardship[i]
		if c.ID != caseID {
			continue
		}
		now := s.now().UTC()
		if c.Kind == "emergency" && c.ExpiresAt != nil && !now.Before(*c.ExpiresAt) && c.State == "active" {
			c.State = "relinquished"
			c.Events = append(c.Events, StewardshipEvent{Sequence: int64(len(c.Events) + 1), Type: "automatically_relinquished", ActorID: "system", Reason: "emergency term elapsed", CreatedAt: now})
		}
		switch action {
		case "complete":
			if c.State != "open" || c.NomineeStandingID == "" || (c.Kind != "nomination" && c.DecisionReceiptID == "") {
				return v, ErrConflict
			}
			c.State = "completed"
			if c.FormerStandingID != "" {
				j := standingIndex(v, c.FormerStandingID)
				if j >= 0 {
					v.Standings[j].State = "recalled"
					v.Standings[j].OperationalAuthority = []string{}
					v.Standings[j].Events = append(v.Standings[j].Events, StandingEvent{Sequence: int64(len(v.Standings[j].Events) + 1), Type: "stewardship_removed", ActorID: actor, Reason: reason, CreatedAt: now})
				}
			}
		case "approve_handoff":
			found := false
			for j := range c.Handoffs {
				if c.Handoffs[j].Resource == resource && c.Handoffs[j].State == "pending_owner_approval" {
					c.Handoffs[j].State = "approved_external_action_required"
					c.Handoffs[j].ApprovedByID = actor
					c.Handoffs[j].ApprovedAt = &now
					found = true
				}
			}
			if !found {
				return v, ErrConflict
			}
		case "appeal":
			if c.AppealState != "" {
				return v, ErrConflict
			}
			c.Appeal = strings.TrimSpace(reason)
			c.AppealState = "open"
		case "resolve_appeal":
			if c.AppealState != "open" {
				return v, ErrConflict
			}
			c.AppealState = "resolved"
		case "review", "relinquish":
			if c.Kind != "emergency" || c.State != "active" {
				return v, ErrConflict
			}
			if action == "review" {
				c.State = "reviewed"
			} else {
				c.State = "relinquished"
			}
		case "deadlock":
			if c.State != "open" {
				return v, ErrConflict
			}
			c.State = "deadlocked"
		default:
			return v, ErrInvalid
		}
		c.UpdatedAt = now
		c.Events = append(c.Events, StewardshipEvent{Sequence: int64(len(c.Events) + 1), Type: action, ActorID: actor, Reason: strings.TrimSpace(reason), CreatedAt: now})
		v.UpdatedAt = now
		return v, s.write(v)
	}
	return v, ErrNotFound
}

func (s *Store) Health(t, scope string) (GovernanceHealth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(t, scope)
	if e != nil {
		return GovernanceHealth{}, e
	}
	now := s.now().UTC()
	h := GovernanceHealth{GeneratedAt: now}
	active := map[string]int{}
	for _, x := range v.Standings {
		if x.State == "active" {
			active[x.Role]++
			if x.TermEndsAt != nil && x.TermEndsAt.Before(now.Add(30*24*time.Hour)) {
				h.ExpiringTerms = append(h.ExpiringTerms, x.ID)
			}
		}
	}
	for _, r := range v.Current.Roles {
		if active[r.Name] < r.MinimumMembers {
			h.Vacancies = append(h.Vacancies, r.Name)
		}
	}
	for _, d := range v.Current.DecisionClasses {
		n := 0
		for _, r := range d.EligibleRoles {
			n += active[r]
		}
		if n < d.Quorum {
			h.QuorumLoss = append(h.QuorumLoss, d.Name)
		}
	}
	for _, c := range v.Stewardship {
		for _, x := range c.Handoffs {
			if x.State != "completed" {
				h.UnresolvedHandoffs = append(h.UnresolvedHandoffs, c.ID+":"+x.Resource)
			}
		}
		if c.State == "deadlocked" {
			h.DeadlockedCases = append(h.DeadlockedCases, c.ID)
		}
		if c.AppealState == "open" {
			h.OpenAppeals = append(h.OpenAppeals, c.ID)
		}
		if c.Kind == "emergency" && c.State == "active" && c.ExpiresAt != nil && now.Before(*c.ExpiresAt) {
			h.ActiveEmergencyPowers = append(h.ActiveEmergencyPowers, c.ID)
		}
	}
	return h, nil
}
