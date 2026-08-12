package governance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ProposalAlternative struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Effects     []string `json:"implementation_effects"`
}
type ProposalEvidence struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Summary   string `json:"summary"`
}
type ProposalComment struct {
	ID        string             `json:"id"`
	ActorID   string             `json:"actor_id"`
	ActorKind string             `json:"actor_kind"`
	Body      string             `json:"body"`
	Citations []ProposalEvidence `json:"citations"`
	CreatedAt time.Time          `json:"created_at"`
}
type Ballot struct {
	ID             string    `json:"id"`
	ActorID        string    `json:"actor_id"`
	Choice         string    `json:"choice,omitempty"`
	ChoiceDigest   string    `json:"choice_digest"`
	Abstain        bool      `json:"abstain"`
	Reason         string    `json:"reason,omitempty"`
	CastAt         time.Time `json:"cast_at"`
	EligibleAtCast bool      `json:"eligible_at_cast"`
}
type Tally struct {
	Electorate      []string       `json:"electorate"`
	EligibleCount   int            `json:"eligible_count"`
	CountedBallots  int            `json:"counted_ballots"`
	Abstentions     int            `json:"abstentions"`
	ExcludedBallots []string       `json:"excluded_ballots"`
	Counts          map[string]int `json:"counts"`
	QuorumRequired  int            `json:"quorum_required"`
	QuorumMet       bool           `json:"quorum_met"`
	Threshold       int            `json:"threshold"`
	ThresholdMet    bool           `json:"threshold_met"`
	Winner          string         `json:"winner,omitempty"`
	Outcome         string         `json:"outcome"`
	Digest          string         `json:"digest"`
	ComputedAt      time.Time      `json:"computed_at"`
}
type Contest struct {
	ID           string             `json:"id"`
	ActorID      string             `json:"actor_id"`
	Reason       string             `json:"reason"`
	Evidence     []ProposalEvidence `json:"evidence"`
	State        string             `json:"state"`
	Resolution   string             `json:"resolution,omitempty"`
	ResolvedByID string             `json:"resolved_by_id,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	ResolvedAt   *time.Time         `json:"resolved_at,omitempty"`
}
type GovernedProposal struct {
	ID                     string                `json:"id"`
	Kind                   string                `json:"kind"`
	Title                  string                `json:"title"`
	Summary                string                `json:"summary"`
	Scope                  string                `json:"scope"`
	CharterVersion         int64                 `json:"charter_version"`
	DecisionClass          string                `json:"decision_class"`
	Alternatives           []ProposalAlternative `json:"alternatives"`
	Evidence               []ProposalEvidence    `json:"evidence"`
	AffectedResources      []string              `json:"affected_resources"`
	DisclosureRequirements []string              `json:"disclosure_requirements"`
	ImplementationEffects  []string              `json:"implementation_effects"`
	EligibleRoles          []string              `json:"eligible_roles"`
	Participation          string                `json:"participation"`
	Quorum                 int                   `json:"quorum"`
	Threshold              int                   `json:"threshold"`
	SecretBallot           bool                  `json:"secret_ballot"`
	OpensAt                time.Time             `json:"opens_at"`
	ClosesAt               time.Time             `json:"closes_at"`
	State                  string                `json:"state"`
	CreatedByID            string                `json:"created_by_id"`
	CreatedAt              time.Time             `json:"created_at"`
	Comments               []ProposalComment     `json:"discussion"`
	Ballots                []Ballot              `json:"ballots"`
	Tally                  *Tally                `json:"tally,omitempty"`
	Contests               []Contest             `json:"contests"`
}
type ProposalInput struct {
	Kind                   string                `json:"kind"`
	Title                  string                `json:"title"`
	Summary                string                `json:"summary"`
	Scope                  string                `json:"scope"`
	DecisionClass          string                `json:"decision_class"`
	Alternatives           []ProposalAlternative `json:"alternatives"`
	Evidence               []ProposalEvidence    `json:"evidence"`
	AffectedResources      []string              `json:"affected_resources"`
	DisclosureRequirements []string              `json:"disclosure_requirements"`
	ImplementationEffects  []string              `json:"implementation_effects"`
	SecretBallot           bool                  `json:"secret_ballot"`
	DiscussionHours        int                   `json:"discussion_hours"`
}

var proposalKinds = map[string]bool{"technical_decision": true, "initiative": true, "policy_exception": true, "funding_request": true, "resource_request": true, "leadership_nomination": true, "charter_amendment": true}

func activeEligible(c Charter, roles []string, now time.Time) []string {
	rm := map[string]bool{}
	for _, r := range roles {
		rm[r] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range c.Standings {
		if rm[s.Role] && s.State == "active" && (s.TermEndsAt == nil || now.Before(*s.TermEndsAt)) && !seen[s.PrincipalID] {
			seen[s.PrincipalID] = true
			out = append(out, s.PrincipalID)
		}
	}
	sort.Strings(out)
	return out
}
func contains(v []string, x string) bool {
	for _, s := range v {
		if s == x {
			return true
		}
	}
	return false
}
func validEvidence(v []ProposalEvidence) bool {
	for _, e := range v {
		if !clean(e.Kind) || !clean(e.Reference) || !clean(e.Summary) {
			return false
		}
	}
	return true
}
func (s *Store) OpenProposal(t, scope, actor string, in ProposalInput) (GovernedProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.read(t, scope)
	if e != nil {
		return GovernedProposal{}, e
	}
	now := s.now().UTC()
	var class DecisionClass
	found := false
	for _, d := range c.Current.DecisionClasses {
		if d.Name == in.DecisionClass {
			class = d
			found = true
		}
	}
	electorate := activeEligible(c, class.EligibleRoles, now)
	if c.Current.State != "active" || !found || !contains(electorate, actor) || !proposalKinds[in.Kind] || !clean(in.Title) || !clean(in.Summary) || !clean(in.Scope) || len(in.Alternatives) < 2 || len(in.Evidence) == 0 || !validEvidence(in.Evidence) || len(in.AffectedResources) == 0 || in.DiscussionHours < 1 || in.DiscussionHours > 24*90 {
		return GovernedProposal{}, ErrInvalid
	}
	if in.Kind == "charter_amendment" {
		class.EligibleRoles = c.Current.AmendmentPolicy.EligibleRoles
		class.Quorum = c.Current.AmendmentPolicy.Quorum
		class.Threshold = c.Current.AmendmentPolicy.Threshold
		electorate = activeEligible(c, class.EligibleRoles, now)
		if !contains(electorate, actor) || in.DiscussionHours < c.Current.AmendmentPolicy.NoticeDays*24 {
			return GovernedProposal{}, ErrConflict
		}
	}
	seen := map[string]bool{}
	for i := range in.Alternatives {
		a := &in.Alternatives[i]
		if !clean(a.Title) || !clean(a.Description) {
			return GovernedProposal{}, ErrInvalid
		}
		if a.ID == "" {
			a.ID = id()
		}
		if seen[a.ID] {
			return GovernedProposal{}, ErrInvalid
		}
		seen[a.ID] = true
	}
	p := GovernedProposal{ID: id(), Kind: in.Kind, Title: strings.TrimSpace(in.Title), Summary: strings.TrimSpace(in.Summary), Scope: strings.TrimSpace(in.Scope), CharterVersion: c.ActiveVersion, DecisionClass: class.Name, Alternatives: in.Alternatives, Evidence: in.Evidence, AffectedResources: in.AffectedResources, DisclosureRequirements: in.DisclosureRequirements, ImplementationEffects: in.ImplementationEffects, EligibleRoles: class.EligibleRoles, Participation: class.Participation, Quorum: class.Quorum, Threshold: class.Threshold, SecretBallot: in.SecretBallot, OpensAt: now, ClosesAt: now.Add(time.Duration(in.DiscussionHours) * time.Hour), State: "open", CreatedByID: actor, CreatedAt: now, Comments: []ProposalComment{}, Ballots: []Ballot{}, Contests: []Contest{}}
	if e = s.writeProposal(t, scope, p); e != nil {
		return GovernedProposal{}, e
	}
	return p, nil
}
func (s *Store) proposalPath(t, scope, p string) string {
	return filepath.Join(s.root, t, scope+"-proposals", p+".json")
}
func (s *Store) writeProposal(t, scope string, p GovernedProposal) error {
	b, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	path := s.proposalPath(t, scope, p.ID)
	if e = os.MkdirAll(filepath.Dir(path), 0750); e != nil {
		return e
	}
	tmp := path + ".tmp"
	if e = os.WriteFile(tmp, b, 0640); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func (s *Store) readProposal(t, scope, p string) (GovernedProposal, error) {
	b, e := os.ReadFile(s.proposalPath(t, scope, p))
	if errors.Is(e, os.ErrNotExist) {
		return GovernedProposal{}, ErrNotFound
	}
	var v GovernedProposal
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) ListProposals(t, scope string) ([]GovernedProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Dir(s.proposalPath(t, scope, "x"))
	xs, e := os.ReadDir(dir)
	if errors.Is(e, os.ErrNotExist) {
		return []GovernedProposal{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []GovernedProposal{}
	for _, x := range xs {
		v, e := s.readProposal(t, scope, strings.TrimSuffix(x.Name(), ".json"))
		if e == nil {
			redact(&v)
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func redact(v *GovernedProposal) {
	if v.SecretBallot && v.State == "open" {
		for i := range v.Ballots {
			v.Ballots[i].Choice = ""
			v.Ballots[i].Reason = ""
		}
	}
}
func (s *Store) GetProposal(t, scope, p string) (GovernedProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.readProposal(t, scope, p)
	if e == nil {
		redact(&v)
	}
	return v, e
}
func (s *Store) Discuss(t, scope, p, actor, kind, body string, cites []ProposalEvidence) (GovernedProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.readProposal(t, scope, p)
	if e != nil {
		return v, e
	}
	if v.State != "open" || !clean(body) || (kind == "agent" && len(cites) == 0) || !validEvidence(cites) {
		return v, ErrInvalid
	}
	v.Comments = append(v.Comments, ProposalComment{ID: id(), ActorID: actor, ActorKind: kind, Body: strings.TrimSpace(body), Citations: cites, CreatedAt: s.now().UTC()})
	e = s.writeProposal(t, scope, v)
	return v, e
}
func (s *Store) Cast(t, scope, p, actor, choice, reason string, abstain bool) (GovernedProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.readProposal(t, scope, p)
	if e != nil {
		return v, e
	}
	c, e := s.read(t, scope)
	if e != nil {
		return v, e
	}
	now := s.now().UTC()
	eligible := contains(activeEligible(c, v.EligibleRoles, now), actor)
	if v.State != "open" || !now.Before(v.ClosesAt) || !eligible || strings.HasPrefix(actor, "agent:") || actor == "codex" {
		return v, ErrConflict
	}
	for _, b := range v.Ballots {
		if b.ActorID == actor {
			return v, ErrConflict
		}
	}
	if abstain {
		choice = ""
	} else {
		ok := false
		for _, a := range v.Alternatives {
			ok = ok || a.ID == choice
		}
		if !ok {
			return v, ErrInvalid
		}
	}
	sum := sha256.Sum256([]byte(v.ID + "\x00" + actor + "\x00" + choice + "\x00" + reason))
	v.Ballots = append(v.Ballots, Ballot{ID: id(), ActorID: actor, Choice: choice, ChoiceDigest: hex.EncodeToString(sum[:]), Abstain: abstain, Reason: strings.TrimSpace(reason), CastAt: now, EligibleAtCast: true})
	e = s.writeProposal(t, scope, v)
	redact(&v)
	return v, e
}
func (s *Store) Finalize(t, scope, p string, force bool) (GovernedProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.readProposal(t, scope, p)
	if e != nil {
		return v, e
	}
	c, e := s.read(t, scope)
	if e != nil {
		return v, e
	}
	now := s.now().UTC()
	if v.State != "open" || (!force && now.Before(v.ClosesAt)) {
		return v, ErrConflict
	}
	elect := activeEligible(c, v.EligibleRoles, now)
	counts := map[string]int{}
	excluded := []string{}
	counted, abstain := 0, 0
	for _, b := range v.Ballots {
		if !contains(elect, b.ActorID) {
			excluded = append(excluded, b.ActorID)
			continue
		}
		if b.Abstain {
			abstain++
			continue
		}
		counts[b.Choice]++
		counted++
	}
	winner, max, tie := "", 0, false
	for _, a := range v.Alternatives {
		n := counts[a.ID]
		if n > max {
			winner, max, tie = a.ID, n, false
		} else if n == max && n > 0 {
			tie = true
		}
	}
	quorumMet := counted+abstain >= v.Quorum
	thresholdMet := counted > 0 && !tie && max*100 >= v.Threshold*counted
	outcome := "rejected"
	if quorumMet && thresholdMet {
		outcome = "approved"
	}
	raw, _ := json.Marshal([]any{v.ID, elect, counts, abstain, excluded, v.Quorum, v.Threshold, outcome})
	sum := sha256.Sum256(raw)
	v.Tally = &Tally{Electorate: elect, EligibleCount: len(elect), CountedBallots: counted, Abstentions: abstain, ExcludedBallots: excluded, Counts: counts, QuorumRequired: v.Quorum, QuorumMet: quorumMet, Threshold: v.Threshold, ThresholdMet: thresholdMet, Winner: winner, Outcome: outcome, Digest: hex.EncodeToString(sum[:]), ComputedAt: now}
	v.State = outcome
	e = s.writeProposal(t, scope, v)
	return v, e
}
func (s *Store) Contest(t, scope, p, actor, reason string, evidence []ProposalEvidence) (GovernedProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.readProposal(t, scope, p)
	if e != nil {
		return v, e
	}
	if v.Tally == nil || !clean(reason) || len(evidence) == 0 || !validEvidence(evidence) {
		return v, ErrInvalid
	}
	v.Contests = append(v.Contests, Contest{ID: id(), ActorID: actor, Reason: reason, Evidence: evidence, State: "open", CreatedAt: s.now().UTC()})
	v.State = "contested"
	e = s.writeProposal(t, scope, v)
	return v, e
}
func (s *Store) ResolveContest(t, scope, p, cid, actor, resolution string) (GovernedProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.readProposal(t, scope, p)
	if e != nil {
		return v, e
	}
	if !clean(resolution) {
		return v, ErrInvalid
	}
	for i := range v.Contests {
		if v.Contests[i].ID == cid && v.Contests[i].State == "open" {
			now := s.now().UTC()
			v.Contests[i].State = "resolved"
			v.Contests[i].Resolution = resolution
			v.Contests[i].ResolvedByID = actor
			v.Contests[i].ResolvedAt = &now
			v.State = v.Tally.Outcome
			e = s.writeProposal(t, scope, v)
			return v, e
		}
	}
	return v, ErrNotFound
}
