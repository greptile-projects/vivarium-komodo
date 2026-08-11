package deliveryteams

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type RepositoryScope struct {
	RepositoryID    string   `json:"repository_id"`
	CommitID        string   `json:"commit_id"`
	Paths           []string `json:"paths"`
	RequiredActions []string `json:"required_actions"`
}

type Assumption struct {
	ID                   string `json:"id"`
	Statement            string `json:"statement"`
	SourceStreamID       string `json:"source_stream_id,omitempty"`
	SourceStreamRevision int64  `json:"source_stream_revision,omitempty"`
}

type WorkStream struct {
	ID                 string            `json:"id"`
	Revision           int64             `json:"revision"`
	Title              string            `json:"title"`
	OwnerParticipantID string            `json:"owner_participant_id"`
	Inputs             []string          `json:"inputs"`
	ExpectedArtifacts  []string          `json:"expected_artifacts"`
	DependsOn          []string          `json:"depends_on"`
	AcceptanceCriteria []string          `json:"acceptance_criteria"`
	RepositoryScope    []RepositoryScope `json:"repository_scope"`
	IntegrationOrder   int               `json:"integration_order"`
	Budget             Budget            `json:"budget"`
	Assumptions        []Assumption      `json:"assumptions"`
}

type Blocker struct {
	Kind      string   `json:"kind"`
	StreamIDs []string `json:"stream_ids,omitempty"`
	OwnerID   string   `json:"owner_id,omitempty"`
	Detail    string   `json:"detail"`
}

type PlanAcceptance struct {
	ParticipantID string    `json:"participant_id"`
	ActorID       string    `json:"actor_id"`
	AcceptedAt    time.Time `json:"accepted_at"`
}

type PlanVersion struct {
	Version             int64            `json:"version"`
	CharterVersion      int64            `json:"charter_version"`
	Status              string           `json:"status"`
	Streams             []WorkStream     `json:"streams"`
	Blockers            []Blocker        `json:"blockers"`
	RequiredAcceptances []string         `json:"required_acceptances"`
	Acceptances         []PlanAcceptance `json:"acceptances"`
	ProposedByID        string           `json:"proposed_by_id"`
	ChangeReason        string           `json:"change_reason"`
	CreatedAt           time.Time        `json:"created_at"`
}

type Plan struct {
	Current PlanVersion   `json:"current"`
	History []PlanVersion `json:"history"`
}

type PlanInput struct {
	Streams      []WorkStream `json:"streams"`
	ChangeReason string       `json:"change_reason"`
}

func participant(v Team, participantID string) (Participant, bool) {
	for _, p := range v.Participants {
		if p.ID == participantID && p.State == "accepted" {
			return p, true
		}
	}
	return Participant{}, false
}

func normalizeStreams(v Team, streams []WorkStream) ([]WorkStream, error) {
	if len(streams) == 0 || len(streams) > 100 {
		return nil, ErrInvalid
	}
	seen := map[string]bool{}
	out := make([]WorkStream, len(streams))
	for i, raw := range streams {
		x := raw
		x.ID, x.Title, x.OwnerParticipantID = strings.TrimSpace(x.ID), strings.TrimSpace(x.Title), strings.TrimSpace(x.OwnerParticipantID)
		if x.ID == "" {
			x.ID = id()
		}
		if seen[x.ID] || x.Title == "" || len(x.Title) > 300 || x.IntegrationOrder < 1 || !validBudget(x.Budget) {
			return nil, ErrInvalid
		}
		seen[x.ID] = true
		if _, ok := participant(v, x.OwnerParticipantID); !ok {
			return nil, ErrInvalid
		}
		var ok bool
		if x.Inputs, ok = clean(x.Inputs, 1000); !ok || len(x.Inputs) == 0 {
			return nil, ErrInvalid
		}
		if x.ExpectedArtifacts, ok = clean(x.ExpectedArtifacts, 1000); !ok || len(x.ExpectedArtifacts) == 0 {
			return nil, ErrInvalid
		}
		if x.DependsOn, ok = clean(x.DependsOn, 100); !ok {
			return nil, ErrInvalid
		}
		if x.AcceptanceCriteria, ok = clean(x.AcceptanceCriteria, 1000); !ok || len(x.AcceptanceCriteria) == 0 {
			return nil, ErrInvalid
		}
		for j := range x.RepositoryScope {
			s := &x.RepositoryScope[j]
			s.RepositoryID, s.CommitID = strings.TrimSpace(s.RepositoryID), strings.TrimSpace(s.CommitID)
			if s.RepositoryID == "" || !fullCommit(s.CommitID) {
				return nil, ErrInvalid
			}
			if s.Paths, ok = clean(s.Paths, 500); !ok || len(s.Paths) == 0 {
				return nil, ErrInvalid
			}
			if s.RequiredActions, ok = clean(s.RequiredActions, 100); !ok {
				return nil, ErrInvalid
			}
		}
		if len(x.RepositoryScope) == 0 {
			return nil, ErrInvalid
		}
		for j := range x.Assumptions {
			a := &x.Assumptions[j]
			a.ID, a.Statement, a.SourceStreamID = strings.TrimSpace(a.ID), strings.TrimSpace(a.Statement), strings.TrimSpace(a.SourceStreamID)
			if a.ID == "" || a.Statement == "" || (a.SourceStreamID != "" && a.SourceStreamRevision < 1) {
				return nil, ErrInvalid
			}
		}
		x.Revision = 1
		if v.Plan != nil {
			for _, old := range v.Plan.Current.Streams {
				if old.ID == x.ID {
					x.Revision = old.Revision
					if fmt.Sprintf("%#v", old) != fmt.Sprintf("%#v", x) {
						x.Revision++
					}
					break
				}
			}
		}
		out[i] = x
	}
	return out, nil
}

func fullCommit(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, c := range value {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func deriveBlockers(v Team, streams []WorkStream) []Blocker {
	blockers := []Blocker{}
	byID := map[string]WorkStream{}
	for _, x := range streams {
		byID[x.ID] = x
	}
	for _, x := range streams {
		p, _ := participant(v, x.OwnerParticipantID)
		if x.Budget.Hours > p.Budget.Hours || x.Budget.CostUnits > p.Budget.CostUnits || x.Budget.AgentRuns > p.Budget.AgentRuns {
			blockers = append(blockers, Blocker{Kind: "owner_budget_exceeded", StreamIDs: []string{x.ID}, OwnerID: x.OwnerParticipantID, Detail: "stream budget exceeds its owner's accepted budget"})
		}
		for _, dep := range x.DependsOn {
			d, ok := byID[dep]
			if !ok {
				blockers = append(blockers, Blocker{Kind: "missing_dependency", StreamIDs: []string{x.ID}, OwnerID: x.OwnerParticipantID, Detail: "dependency " + dep + " is absent"})
				continue
			}
			if d.IntegrationOrder >= x.IntegrationOrder {
				blockers = append(blockers, Blocker{Kind: "integration_order_conflict", StreamIDs: []string{dep, x.ID}, OwnerID: x.OwnerParticipantID, Detail: "dependency must integrate before its consumer"})
			}
		}
		for _, a := range x.Assumptions {
			if a.SourceStreamID == "" {
				continue
			}
			s, ok := byID[a.SourceStreamID]
			if !ok || s.Revision != a.SourceStreamRevision {
				blockers = append(blockers, Blocker{Kind: "upstream_assumption_changed", StreamIDs: []string{a.SourceStreamID, x.ID}, OwnerID: x.OwnerParticipantID, Detail: "upstream stream revision no longer matches assumption " + a.ID})
			}
		}
		for _, scope := range x.RepositoryScope {
			for _, action := range scope.RequiredActions {
				found := false
				for _, held := range p.Access.Actions {
					if action == held {
						found = true
					}
				}
				if !found {
					blockers = append(blockers, Blocker{Kind: "unavailable_access", StreamIDs: []string{x.ID}, OwnerID: x.OwnerParticipantID, Detail: action + " is not available to the owner"})
				}
			}
		}
	}
	for i, a := range streams {
		for _, b := range streams[i+1:] {
			pairDone := false
			for _, as := range a.RepositoryScope {
				for _, bs := range b.RepositoryScope {
					if as.RepositoryID != bs.RepositoryID {
						continue
					}
					if as.CommitID != bs.CommitID {
						blockers = append(blockers, Blocker{Kind: "incompatible_revision", StreamIDs: []string{a.ID, b.ID}, Detail: "shared repository scope starts from different commits"})
						pairDone = true
						break
					}
					for _, ap := range as.Paths {
						for _, bp := range bs.Paths {
							if ap == bp || strings.HasPrefix(ap, bp+"/") || strings.HasPrefix(bp, ap+"/") {
								blockers = append(blockers, Blocker{Kind: "overlapping_scope", StreamIDs: []string{a.ID, b.ID}, Detail: "repository paths overlap at " + ap + " and " + bp})
								pairDone = true
								break
							}
						}
						if pairDone {
							break
						}
					}
					if pairDone {
						break
					}
				}
				if pairDone {
					break
				}
			}
		}
	}
	total := Budget{}
	for _, x := range streams {
		total.Hours += x.Budget.Hours
		total.CostUnits += x.Budget.CostUnits
		total.AgentRuns += x.Budget.AgentRuns
	}
	if total.Hours > v.Charter.TotalBudget.Hours || total.CostUnits > v.Charter.TotalBudget.CostUnits || total.AgentRuns > v.Charter.TotalBudget.AgentRuns {
		blockers = append(blockers, Blocker{Kind: "team_budget_exceeded", Detail: "combined stream budgets exceed the charter ceiling"})
	}
	sort.Slice(blockers, func(i, j int) bool { return blockers[i].Kind+blockers[i].Detail < blockers[j].Kind+blockers[j].Detail })
	return blockers
}

func requiredOwners(v Team, streams []WorkStream) []string {
	m := map[string]bool{}
	for _, x := range streams {
		m[x.OwnerParticipantID] = true
	}
	if v.Plan != nil {
		for _, x := range v.Plan.Current.Streams {
			m[x.OwnerParticipantID] = true
		}
	}
	out := []string{}
	for x := range m {
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}

func (s *Store) ProposePlan(repo, team, actor, actingParticipantID string, version int64, in PlanInput) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, team)
	if e != nil {
		return v, e
	}
	if version != v.Version {
		return v, ErrConflict
	}
	if actor != v.OrganizerID {
		p, ok := participant(v, actingParticipantID)
		if !ok || (p.Kind == "human" && p.PrincipalID != actor) {
			return v, ErrForbidden
		}
	}
	streams, e := normalizeStreams(v, in.Streams)
	if e != nil {
		return v, e
	}
	now := s.now().UTC()
	pv := int64(1)
	if v.Plan != nil {
		pv = v.Plan.Current.Version + 1
	}
	plan := PlanVersion{Version: pv, CharterVersion: v.Charter.Version, Status: "pending_acceptance", Streams: streams, Blockers: deriveBlockers(v, streams), RequiredAcceptances: requiredOwners(v, streams), Acceptances: []PlanAcceptance{}, ProposedByID: actor, ChangeReason: strings.TrimSpace(in.ChangeReason), CreatedAt: now}
	for _, pid := range plan.RequiredAcceptances {
		p, _ := participant(v, pid)
		if (p.Kind == "human" && p.PrincipalID == actor) || pid == actingParticipantID {
			plan.Acceptances = append(plan.Acceptances, PlanAcceptance{ParticipantID: pid, ActorID: actor, AcceptedAt: now})
		}
	}
	if len(plan.Acceptances) == len(plan.RequiredAcceptances) && len(plan.Blockers) == 0 {
		plan.Status = "accepted"
	}
	if v.Plan == nil {
		v.Plan = &Plan{}
	}
	v.Plan.Current = plan
	v.Plan.History = append(v.Plan.History, plan)
	addEvent(&v, "plan.proposed", actor, "", fmt.Sprintf("plan version %d: %s", pv, plan.ChangeReason), now)
	return v, s.write(v)
}

func (s *Store) AcceptPlan(repo, team, participantID, actor string, version, planVersion int64) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, team)
	if e != nil {
		return v, e
	}
	if version != v.Version {
		return v, ErrConflict
	}
	if v.Plan == nil || v.Plan.Current.Version != planVersion || v.Plan.Current.Status != "pending_acceptance" {
		return v, ErrConflict
	}
	p, ok := participant(v, participantID)
	if !ok || (p.Kind == "human" && p.PrincipalID != actor) {
		return v, ErrForbidden
	}
	required := false
	for _, x := range v.Plan.Current.RequiredAcceptances {
		if x == participantID {
			required = true
		}
	}
	if !required {
		return v, ErrForbidden
	}
	for _, x := range v.Plan.Current.Acceptances {
		if x.ParticipantID == participantID {
			return v, ErrConflict
		}
	}
	now := s.now().UTC()
	v.Plan.Current.Acceptances = append(v.Plan.Current.Acceptances, PlanAcceptance{ParticipantID: participantID, ActorID: actor, AcceptedAt: now})
	if len(v.Plan.Current.Acceptances) == len(v.Plan.Current.RequiredAcceptances) && len(v.Plan.Current.Blockers) == 0 {
		v.Plan.Current.Status = "accepted"
	}
	v.Plan.History[len(v.Plan.History)-1] = v.Plan.Current
	addEvent(&v, "plan.accepted", actor, participantID, fmt.Sprintf("plan version %d", planVersion), now)
	return v, s.write(v)
}
