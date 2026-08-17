// Package agentdiscovery owns explainable, audience-projected agent comparisons.
package agentdiscovery

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

	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprofiles"
)

var ErrNotFound = errors.New("agent discovery not found")
var ErrInvalid = errors.New("invalid agent discovery")

type EvidenceInput struct {
	ProfileID          string    `json:"profile_id"`
	ProfileVersion     int64     `json:"profile_version"`
	Audience           string    `json:"audience"`
	Kind               string    `json:"kind"`
	Workflow           string    `json:"workflow"`
	Summary            string    `json:"summary"`
	SourceType         string    `json:"source_type"`
	SourceID           string    `json:"source_id"`
	ComparableTags     []string  `json:"comparable_tags"`
	Result             string    `json:"result"`
	CostAmount         float64   `json:"cost_amount,omitempty"`
	CostCurrency       string    `json:"cost_currency,omitempty"`
	ConflictOfInterest string    `json:"conflict_of_interest,omitempty"`
	ObservedAt         time.Time `json:"observed_at"`
}
type Evidence struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	CreatedBy    string `json:"created_by"`
	EvidenceInput
	CreatedAt time.Time `json:"created_at"`
}

type SearchInput struct {
	ContextType         string   `json:"context_type"`
	ContextID           string   `json:"context_id"`
	PublicSummary       string   `json:"public_summary,omitempty"`
	Audience            string   `json:"audience"`
	Workflow            string   `json:"workflow"`
	RequiredPermissions []string `json:"required_permissions"`
	AllowedBoundaries   []string `json:"allowed_boundaries"`
	RequiredPolicyTerms []string `json:"required_policy_terms"`
	ComparableTags      []string `json:"comparable_tags"`
	MaximumCost         float64  `json:"maximum_cost,omitempty"`
	Currency            string   `json:"currency,omitempty"`
	AvailabilityTerms   []string `json:"availability_terms"`
}
type Factor struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Explanation string `json:"explanation"`
}
type Match struct {
	ProfileID       string     `json:"profile_id"`
	Handle          string     `json:"handle"`
	DisplayName     string     `json:"display_name"`
	ProfileVersion  int64      `json:"profile_version"`
	Eligible        bool       `json:"eligible"`
	Factors         []Factor   `json:"factors"`
	Evidence        []Evidence `json:"evidence"`
	MissingEvidence []string   `json:"missing_evidence"`
	StaleEvidence   []string   `json:"stale_evidence"`
	Conflicts       []string   `json:"conflicts_of_interest"`
}
type Search struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	CreatedBy    string `json:"created_by"`
	SearchInput
	CreatedAt time.Time `json:"created_at"`
	Matches   []Match   `json:"matches"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(filepath.Join(a, "evidence"), 0750)
		if e == nil {
			e = os.MkdirAll(filepath.Join(a, "searches"), 0750)
		}
	}
	return &Store{root: a, now: time.Now}, e
}
func makeID(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}
func listOK(xs []string) bool {
	if len(xs) > 100 {
		return false
	}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" {
			return false
		}
	}
	return true
}
func (s *Store) AddEvidence(repo, actor string, in EvidenceInput, profile agentprofiles.Profile) (Evidence, error) {
	if in.ProfileID != profile.ID || in.ProfileVersion < 1 || in.ProfileVersion > profile.CurrentVersion || !map[string]bool{"public": true, "repository": true}[in.Audience] || !map[string]bool{"evaluation": true, "outcome": true}[in.Kind] || in.Workflow == "" || in.Summary == "" || in.SourceType == "" || in.SourceID == "" || in.Result == "" || in.CostAmount < 0 || !listOK(in.ComparableTags) {
		return Evidence{}, ErrInvalid
	}
	if in.ObservedAt.IsZero() {
		in.ObservedAt = s.now().UTC()
	}
	e := Evidence{ID: makeID("aev_"), RepositoryID: repo, CreatedBy: actor, EvidenceInput: in, CreatedAt: s.now().UTC()}
	s.mu.Lock()
	defer s.mu.Unlock()
	return e, s.write("evidence", e.ID, e)
}
func (s *Store) Search(repo, actor string, in SearchInput, profiles []agentprofiles.Profile) (Search, error) {
	contexts := map[string]bool{"task": true, "proposal": true, "issue": true, "decision": true, "incident": true, "stewardship_mandate": true, "team_role": true}
	if !contexts[in.ContextType] || in.ContextID == "" || !map[string]bool{"public": true, "repository": true}[in.Audience] || in.Workflow == "" || in.MaximumCost < 0 || !listOK(in.RequiredPermissions) || !listOK(in.AllowedBoundaries) || !listOK(in.RequiredPolicyTerms) || !listOK(in.ComparableTags) || !listOK(in.AvailabilityTerms) {
		return Search{}, ErrInvalid
	}
	evidence, _ := s.listEvidence(repo, true)
	now := s.now().UTC()
	out := Search{ID: makeID("ase_"), RepositoryID: repo, CreatedBy: actor, SearchInput: in, CreatedAt: now}
	for _, p := range profiles {
		v := p.Versions[len(p.Versions)-1]
		m := Match{ProfileID: p.ID, Handle: p.Handle, DisplayName: v.DisplayName, ProfileVersion: v.Number, Eligible: true}
		supported := containsFold(v.SupportedTasks, in.Workflow)
		m.Factors = append(m.Factors, factor("supported workflow", supported, "profile declares workflow "+in.Workflow, "workflow is not declared by the current profile"))
		m.Eligible = m.Eligible && supported
		perm := allFold(v.RequestedCapabilities, in.RequiredPermissions)
		m.Factors = append(m.Factors, factor("effective permissions", perm, "requested capabilities fit the task allowance", "agent requests capabilities outside the task allowance"))
		m.Eligible = m.Eligible && perm
		boundary := len(in.AllowedBoundaries) == 0 || containsAnyFold(append(v.Execution.Regions, v.Execution.Boundary), in.AllowedBoundaries)
		m.Factors = append(m.Factors, factor("deployment boundary", boundary, "execution boundary is permitted", "execution boundary is not permitted"))
		m.Eligible = m.Eligible && boundary
		policy := allText(append(append(v.DataUse.Purposes, v.DataUse.TrainingUse), v.DataUse.Retention), in.RequiredPolicyTerms)
		m.Factors = append(m.Factors, factor("policy compatibility", policy, "disclosures contain every required policy term", "one or more required policy terms are absent"))
		m.Eligible = m.Eligible && policy
		cost := in.MaximumCost == 0 || (strings.EqualFold(in.Currency, v.Pricing.Currency) && v.Pricing.Amount <= in.MaximumCost)
		m.Factors = append(m.Factors, factor("cost", cost, "listed price is within the declared limit", "listed price or currency exceeds the declared limit"))
		m.Eligible = m.Eligible && cost
		avail := allText([]string{v.Availability}, in.AvailabilityTerms)
		m.Factors = append(m.Factors, factor("availability", avail, "availability contains every required term", "availability evidence is incomplete"))
		m.Eligible = m.Eligible && avail
		for _, e := range evidence {
			if e.ProfileID == p.ID && containsFold(e.ComparableTags, in.ComparableTags...) && strings.EqualFold(e.Workflow, in.Workflow) {
				m.Evidence = append(m.Evidence, e)
				if e.ProfileVersion != v.Number || now.Sub(e.ObservedAt) > 180*24*time.Hour {
					m.StaleEvidence = append(m.StaleEvidence, e.ID)
				}
				if e.ConflictOfInterest != "" {
					m.Conflicts = append(m.Conflicts, e.ConflictOfInterest)
				}
			}
		}
		if !hasKind(m.Evidence, "evaluation") {
			m.MissingEvidence = append(m.MissingEvidence, "verified evaluation on comparable work")
		}
		if !hasKind(m.Evidence, "outcome") {
			m.MissingEvidence = append(m.MissingEvidence, "attributed outcome on comparable work")
		}
		out.Matches = append(out.Matches, m)
	}
	sort.Slice(out.Matches, func(i, j int) bool { return out.Matches[i].Handle < out.Matches[j].Handle })
	s.mu.Lock()
	defer s.mu.Unlock()
	return out, s.write("searches", out.ID, out)
}
func factor(n string, ok bool, yes, no string) Factor {
	if ok {
		return Factor{n, "match", yes}
	}
	return Factor{n, "conflict", no}
}
func containsFold(xs []string, ys ...string) bool {
	for _, y := range ys {
		for _, x := range xs {
			if strings.Contains(strings.ToLower(x), strings.ToLower(y)) {
				return true
			}
		}
	}
	return len(ys) == 0
}
func containsAnyFold(xs, ys []string) bool {
	for _, y := range ys {
		if containsFold(xs, y) {
			return true
		}
	}
	return false
}
func allFold(requested, allowed []string) bool {
	for _, x := range requested {
		if !containsFold(allowed, x) {
			return false
		}
	}
	return true
}
func allText(have, need []string) bool {
	for _, x := range need {
		if !containsFold(have, x) {
			return false
		}
	}
	return true
}
func hasKind(es []Evidence, k string) bool {
	for _, e := range es {
		if e.Kind == k {
			return true
		}
	}
	return false
}
func (s *Store) Get(id string, repositoryReader bool) (Search, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Search
	if s.read("searches", id, &x) != nil {
		return x, ErrNotFound
	}
	if x.Audience != "public" && !repositoryReader {
		return Search{}, ErrNotFound
	}
	if !repositoryReader {
		x.ContextID = ""
		x.CreatedBy = ""
		x.RepositoryID = ""
		for i := range x.Matches {
			var visible []Evidence
			visibleIDs := map[string]bool{}
			for _, e := range x.Matches[i].Evidence {
				if e.Audience == "public" {
					visible = append(visible, e)
					visibleIDs[e.ID] = true
				}
			}
			x.Matches[i].Evidence = visible
			x.Matches[i].Conflicts = nil
			for _, e := range visible {
				if e.ConflictOfInterest != "" {
					x.Matches[i].Conflicts = append(x.Matches[i].Conflicts, e.ConflictOfInterest)
				}
			}
			var stale []string
			for _, evidenceID := range x.Matches[i].StaleEvidence {
				if visibleIDs[evidenceID] {
					stale = append(stale, evidenceID)
				}
			}
			x.Matches[i].StaleEvidence = stale
			x.Matches[i].MissingEvidence = nil
			if !hasKind(visible, "evaluation") {
				x.Matches[i].MissingEvidence = append(x.Matches[i].MissingEvidence, "public verified evaluation on comparable work")
			}
			if !hasKind(visible, "outcome") {
				x.Matches[i].MissingEvidence = append(x.Matches[i].MissingEvidence, "public attributed outcome on comparable work")
			}
		}
	}
	return x, nil
}
func (s *Store) listEvidence(repo string, reader bool) ([]Evidence, error) {
	es, e := os.ReadDir(filepath.Join(s.root, "evidence"))
	if e != nil {
		return nil, e
	}
	out := []Evidence{}
	for _, f := range es {
		var x Evidence
		if s.read("evidence", strings.TrimSuffix(f.Name(), ".json"), &x) == nil && x.RepositoryID == repo && (reader || x.Audience == "public") {
			out = append(out, x)
		}
	}
	return out, nil
}
func (s *Store) write(kind, id string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	p := filepath.Join(s.root, kind, id+".json")
	return os.WriteFile(p, b, 0640)
}
func (s *Store) read(kind, id string, v any) error {
	return func() error {
		b, e := os.ReadFile(filepath.Join(s.root, kind, id+".json"))
		if e != nil {
			return e
		}
		return json.Unmarshal(b, v)
	}()
}
