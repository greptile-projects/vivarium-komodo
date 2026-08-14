// Package translationunits retains revision-exact localization extraction and proposals.
package translationunits

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
)

var (
	ErrNotFound  = errors.New("translation extraction not found")
	ErrInvalid   = errors.New("invalid translation extraction")
	ErrConflict  = errors.New("translation work changed")
	ErrForbidden = errors.New("translation work forbidden")
)

type Location struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	BlobID string `json:"blob_id"`
}
type Unit struct {
	ID             string                      `json:"id"`
	ResourceID     string                      `json:"resource_id"`
	Key            string                      `json:"key"`
	Message        string                      `json:"message"`
	PriorMessage   string                      `json:"prior_message,omitempty"`
	Context        string                      `json:"context,omitempty"`
	ScreenshotURLs []string                    `json:"screenshot_urls,omitempty"`
	Variables      []string                    `json:"variables,omitempty"`
	PluralRule     string                      `json:"plural_rule,omitempty"`
	Location       Location                    `json:"source_location"`
	Change         string                      `json:"change"`
	Translations   map[string]TranslationState `json:"translations"`
}
type TranslationState struct {
	Text          string `json:"text,omitempty"`
	Status        string `json:"status"`
	SourceMessage string `json:"source_message,omitempty"`
	ActorID       string `json:"actor_id,omitempty"`
	ProposalID    string `json:"proposal_id,omitempty"`
}
type Proposal struct {
	ID            string    `json:"id"`
	UnitID        string    `json:"unit_id"`
	LocaleID      string    `json:"locale_id"`
	Text          string    `json:"text"`
	SourceMessage string    `json:"source_message"`
	Revision      string    `json:"revision"`
	ActorID       string    `json:"actor_id"`
	CreatedAt     time.Time `json:"created_at"`
	Superseded    bool      `json:"superseded"`
	Origin        string    `json:"origin,omitempty"`
}
type Claim struct {
	ID          string    `json:"id"`
	LocaleID    string    `json:"locale_id"`
	ActorID     string    `json:"actor_id"`
	Status      string    `json:"status"`
	HandoffToID string    `json:"handoff_to_id,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
}
type Discussion struct {
	ID        string    `json:"id"`
	UnitID    string    `json:"unit_id"`
	LocaleID  string    `json:"locale_id"`
	ActorID   string    `json:"actor_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
type Evidence struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Summary   string `json:"summary"`
}
type Suggestion struct {
	ID            string     `json:"id"`
	UnitID        string     `json:"unit_id"`
	LocaleID      string     `json:"locale_id"`
	AgentID       string     `json:"agent_id"`
	RequestedByID string     `json:"requested_by_id"`
	Text          string     `json:"text"`
	Status        string     `json:"status"`
	DecisionByID  string     `json:"decision_by_id,omitempty"`
	Rationale     string     `json:"rationale,omitempty"`
	Evidence      []Evidence `json:"evidence"`
	Uncertainty   string     `json:"uncertainty"`
	Revision      string     `json:"revision"`
	CreatedAt     time.Time  `json:"created_at"`
	DecidedAt     *time.Time `json:"decided_at,omitempty"`
}
type Review struct {
	ID         string    `json:"id"`
	ProposalID string    `json:"proposal_id"`
	UnitID     string    `json:"unit_id"`
	LocaleID   string    `json:"locale_id"`
	ReviewerID string    `json:"reviewer_id"`
	Decision   string    `json:"decision"`
	Rationale  string    `json:"rationale"`
	CreatedAt  time.Time `json:"created_at"`
}
type Terminology struct {
	Concept   string   `json:"concept"`
	Preferred string   `json:"preferred"`
	Context   string   `json:"context,omitempty"`
	Avoid     []string `json:"avoid,omitempty"`
}
type Extraction struct {
	ID                string                   `json:"id"`
	RepositoryID      string                   `json:"repository_id"`
	PullRequestID     string                   `json:"pull_request_id"`
	Revision          string                   `json:"revision"`
	TargetRevision    string                   `json:"target_revision"`
	SourceLocale      string                   `json:"source_locale"`
	Locales           []string                 `json:"locales"`
	ConfigPath        string                   `json:"config_path"`
	ConfigBlobID      string                   `json:"config_blob_id"`
	Units             []Unit                   `json:"units"`
	Proposals         []Proposal               `json:"proposals"`
	Claims            []Claim                  `json:"claims,omitempty"`
	Discussion        []Discussion             `json:"discussion,omitempty"`
	Suggestions       []Suggestion             `json:"suggestions,omitempty"`
	Reviews           []Review                 `json:"reviews,omitempty"`
	PlanID            string                   `json:"locale_plan_id,omitempty"`
	PlanVersion       int64                    `json:"locale_plan_version,omitempty"`
	ProductContext    string                   `json:"product_context,omitempty"`
	Terminology       map[string][]Terminology `json:"terminology,omitempty"`
	ReviewerIDs       map[string][]string      `json:"reviewer_ids,omitempty"`
	Protected         bool                     `json:"protected"`
	Embargoed         bool                     `json:"embargoed"`
	PermittedActorIDs []string                 `json:"permitted_actor_ids,omitempty"`
	CreatedByID       string                   `json:"created_by_id"`
	CreatedAt         time.Time                `json:"created_at"`
}
type Input struct {
	Revision, TargetRevision, SourceLocale, ConfigPath, ConfigBlobID string
	Locales                                                          []string
	Units                                                            []Unit
	PlanID, ProductContext                                           string
	PlanVersion                                                      int64
	Terminology                                                      map[string][]Terminology
	ReviewerIDs                                                      map[string][]string
	Protected, Embargoed                                             bool
	PermittedActorIDs                                                []string
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
func newID() string                            { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, pull string) string { return filepath.Join(s.root, repo, pull+".json") }
func (s *Store) load(repo, pull string) (Extraction, error) {
	var x Extraction
	b, e := os.ReadFile(s.path(repo, pull))
	if os.IsNotExist(e) {
		return x, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) save(x Extraction) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.PullRequestID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.PullRequestID), b, 0640)
	}
	return e
}
func (s *Store) Create(repo, pull, actor string, in Input) (Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || pull == "" || actor == "" || in.Revision == "" || in.TargetRevision == "" || in.SourceLocale == "" || in.ConfigBlobID == "" || len(in.Locales) == 0 {
		return Extraction{}, ErrInvalid
	}
	old, _ := s.load(repo, pull)
	proposals := old.Proposals
	currentMessages := map[string]string{}
	for _, unit := range in.Units {
		currentMessages[unit.ID] = unit.Message
	}
	for i := range proposals {
		if currentMessages[proposals[i].UnitID] != proposals[i].SourceMessage {
			proposals[i].Superseded = true
		}
	}
	x := Extraction{ID: newID(), RepositoryID: repo, PullRequestID: pull, Revision: in.Revision, TargetRevision: in.TargetRevision, SourceLocale: in.SourceLocale, Locales: in.Locales, ConfigPath: in.ConfigPath, ConfigBlobID: in.ConfigBlobID, Units: in.Units, Proposals: proposals, PlanID: in.PlanID, PlanVersion: in.PlanVersion, ProductContext: in.ProductContext, Terminology: in.Terminology, ReviewerIDs: in.ReviewerIDs, Protected: in.Protected, Embargoed: in.Embargoed, PermittedActorIDs: in.PermittedActorIDs, CreatedByID: actor, CreatedAt: s.now().UTC()}
	x.Claims = old.Claims
	x.Discussion = old.Discussion
	x.Suggestions = old.Suggestions
	x.Reviews = old.Reviews
	if old.Revision != "" && old.Revision != in.Revision {
		for i := range x.Suggestions {
			if x.Suggestions[i].Status == "pending_human_review" {
				x.Suggestions[i].Status = "superseded"
			}
		}
		latest := map[string]Claim{}
		for _, c := range x.Claims {
			if c.Version >= latest[c.LocaleID].Version {
				latest[c.LocaleID] = c
			}
		}
		for locale, c := range latest {
			if c.Status == "claimed" {
				x.Claims = append(x.Claims, Claim{ID: newID(), LocaleID: locale, ActorID: c.ActorID, Status: "superseded", Reason: "source revision changed", Version: c.Version + 1, CreatedAt: s.now().UTC()})
			}
		}
	}
	applyProposals(&x)
	return x, s.save(x)
}
func applyProposals(x *Extraction) {
	latest := map[string]Proposal{}
	for _, p := range x.Proposals {
		if !p.Superseded {
			latest[p.UnitID+"\x00"+p.LocaleID] = p
		}
	}
	for i := range x.Units {
		if x.Units[i].Translations == nil {
			x.Units[i].Translations = map[string]TranslationState{}
		}
		for _, l := range x.Locales {
			if p, ok := latest[x.Units[i].ID+"\x00"+l]; ok {
				x.Units[i].Translations[l] = TranslationState{Text: p.Text, Status: "proposed", SourceMessage: p.SourceMessage, ActorID: p.ActorID, ProposalID: p.ID}
			}
		}
	}
}
func (s *Store) Get(repo, pull string) (Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(repo, pull)
}
func (s *Store) List(repo string) ([]Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	out := []Extraction{}
	for _, f := range files {
		var x Extraction
		b, er := os.ReadFile(f)
		if er == nil {
			er = json.Unmarshal(b, &x)
		}
		if er != nil {
			return nil, er
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) ListAuthorized(repo, actor string) ([]Extraction, error) {
	items, e := s.List(repo)
	if e != nil {
		return nil, e
	}
	out := []Extraction{}
	for _, x := range items {
		if allowed(x, actor) {
			out = append(out, x)
		}
	}
	return out, nil
}
func (s *Store) Propose(repo, pull, actor, unit, locale, text string) (Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, pull)
	if e != nil {
		return x, e
	}
	if !allowed(x, actor) {
		return x, ErrForbidden
	}
	text = strings.TrimSpace(text)
	found := false
	var message string
	for _, u := range x.Units {
		if u.ID == unit && u.Change != "removed" {
			found = true
			message = u.Message
		}
	}
	validLocale := false
	for _, l := range x.Locales {
		if l == locale {
			validLocale = true
		}
	}
	if actor == "" || !found || !validLocale || text == "" || len(text) > 10000 {
		return x, ErrInvalid
	}
	for i := range x.Proposals {
		if x.Proposals[i].UnitID == unit && x.Proposals[i].LocaleID == locale && !x.Proposals[i].Superseded {
			x.Proposals[i].Superseded = true
		}
	}
	x.Proposals = append(x.Proposals, Proposal{ID: newID(), UnitID: unit, LocaleID: locale, Text: text, SourceMessage: message, Revision: x.Revision, ActorID: actor, Origin: "human", CreatedAt: s.now().UTC()})
	applyProposals(&x)
	return x, s.save(x)
}

func allowed(x Extraction, actor string) bool {
	if !x.Protected && !x.Embargoed {
		return true
	}
	if actor == x.CreatedByID {
		return true
	}
	for _, id := range x.PermittedActorIDs {
		if id == actor {
			return true
		}
	}
	return false
}
func validUnitLocale(x Extraction, unit, locale string) bool {
	okU := false
	for _, u := range x.Units {
		okU = okU || (u.ID == unit && u.Change != "removed")
	}
	okL := false
	for _, l := range x.Locales {
		okL = okL || l == locale
	}
	return okU && okL
}
func (s *Store) Authorized(repo, pull, actor string) (Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, pull)
	if e == nil && !allowed(x, actor) {
		e = ErrForbidden
	}
	return x, e
}
func (s *Store) Claim(repo, pull, actor, locale, action, handoff, reason string, expected int64) (Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, pull)
	if e != nil {
		return x, e
	}
	if !allowed(x, actor) {
		return x, ErrForbidden
	}
	current := int64(0)
	active := ""
	for _, c := range x.Claims {
		if c.LocaleID == locale && c.Version >= current {
			current = c.Version
			if c.Status == "claimed" {
				active = c.ActorID
			} else {
				active = ""
			}
		}
	}
	if expected != current {
		return x, ErrConflict
	}
	valid := false
	for _, l := range x.Locales {
		valid = valid || l == locale
	}
	if !valid || !map[string]bool{"claim": true, "release": true, "handoff": true}[action] {
		return x, ErrInvalid
	}
	status := "released"
	if action == "claim" {
		if active != "" && active != actor {
			return x, ErrConflict
		}
		status = "claimed"
	} else if active != actor {
		return x, ErrForbidden
	}
	if action == "handoff" {
		if handoff == "" || !allowed(x, handoff) {
			return x, ErrInvalid
		}
		status = "handed_off"
	}
	x.Claims = append(x.Claims, Claim{ID: newID(), LocaleID: locale, ActorID: actor, Status: status, HandoffToID: handoff, Reason: reason, Version: current + 1, CreatedAt: s.now().UTC()})
	if action == "handoff" {
		x.Claims = append(x.Claims, Claim{ID: newID(), LocaleID: locale, ActorID: handoff, Status: "claimed", Reason: "accepted handoff", Version: current + 2, CreatedAt: s.now().UTC()})
	}
	return x, s.save(x)
}
func (s *Store) Discuss(repo, pull, actor, unit, locale, body string) (Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, pull)
	body = strings.TrimSpace(body)
	if e != nil {
		return x, e
	}
	if !allowed(x, actor) {
		return x, ErrForbidden
	}
	if !validUnitLocale(x, unit, locale) || body == "" || len(body) > 5000 {
		return x, ErrInvalid
	}
	x.Discussion = append(x.Discussion, Discussion{ID: newID(), UnitID: unit, LocaleID: locale, ActorID: actor, Body: body, CreatedAt: s.now().UTC()})
	return x, s.save(x)
}
func (s *Store) Suggest(repo, pull, requester, agent, unit, locale, text, uncertainty string, evidence []Evidence) (Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, pull)
	if e != nil {
		return x, e
	}
	if !allowed(x, requester) || !allowed(x, agent) {
		return x, ErrForbidden
	}
	if !validUnitLocale(x, unit, locale) || agent == "" || strings.TrimSpace(text) == "" || strings.TrimSpace(uncertainty) == "" || len(evidence) == 0 {
		return x, ErrInvalid
	}
	for _, v := range evidence {
		if v.Kind == "" || v.Reference == "" || v.Summary == "" {
			return x, ErrInvalid
		}
	}
	x.Suggestions = append(x.Suggestions, Suggestion{ID: newID(), UnitID: unit, LocaleID: locale, AgentID: agent, RequestedByID: requester, Text: text, Evidence: evidence, Uncertainty: uncertainty, Status: "pending_human_review", Revision: x.Revision, CreatedAt: s.now().UTC()})
	return x, s.save(x)
}
func (s *Store) DecideSuggestion(repo, pull, actor, id, decision, text, rationale string) (Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, pull)
	if e != nil {
		return x, e
	}
	if !allowed(x, actor) {
		return x, ErrForbidden
	}
	if !map[string]bool{"approve": true, "edit": true, "reject": true, "escalate": true}[decision] || strings.TrimSpace(rationale) == "" {
		return x, ErrInvalid
	}
	for i := range x.Suggestions {
		q := &x.Suggestions[i]
		if q.ID != id {
			continue
		}
		if q.Status != "pending_human_review" || q.AgentID == actor {
			return x, ErrForbidden
		}
		if decision == "edit" && strings.TrimSpace(text) == "" {
			return x, ErrInvalid
		}
		now := s.now().UTC()
		q.Status = map[string]string{"approve": "approved", "edit": "edited", "reject": "rejected", "escalate": "escalated"}[decision]
		q.DecisionByID = actor
		q.Rationale = rationale
		q.DecidedAt = &now
		if decision == "approve" || decision == "edit" {
			chosen := q.Text
			if decision == "edit" {
				chosen = text
			}
			for j := range x.Proposals {
				if x.Proposals[j].UnitID == q.UnitID && x.Proposals[j].LocaleID == q.LocaleID && !x.Proposals[j].Superseded {
					x.Proposals[j].Superseded = true
				}
			}
			var msg string
			for _, u := range x.Units {
				if u.ID == q.UnitID {
					msg = u.Message
				}
			}
			x.Proposals = append(x.Proposals, Proposal{ID: newID(), UnitID: q.UnitID, LocaleID: q.LocaleID, Text: chosen, SourceMessage: msg, Revision: x.Revision, ActorID: actor, Origin: "agent_suggestion_" + decision, CreatedAt: now})
			applyProposals(&x)
		}
		return x, s.save(x)
	}
	return x, ErrNotFound
}
func (s *Store) ReviewProposal(repo, pull, actor, proposal, decision, rationale string) (Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, pull)
	if e != nil {
		return x, e
	}
	if !allowed(x, actor) {
		return x, ErrForbidden
	}
	if !map[string]bool{"approve": true, "reject": true, "request_changes": true, "escalate": true}[decision] || rationale == "" {
		return x, ErrInvalid
	}
	var p Proposal
	found := false
	for _, v := range x.Proposals {
		if v.ID == proposal && !v.Superseded {
			p = v
			found = true
		}
	}
	if !found {
		return x, ErrNotFound
	}
	required := x.ReviewerIDs[p.LocaleID]
	if len(required) > 0 {
		ok := false
		for _, id := range required {
			ok = ok || id == actor
		}
		if !ok {
			return x, ErrForbidden
		}
	}
	x.Reviews = append(x.Reviews, Review{ID: newID(), ProposalID: p.ID, UnitID: p.UnitID, LocaleID: p.LocaleID, ReviewerID: actor, Decision: decision, Rationale: rationale, CreatedAt: s.now().UTC()})
	return x, s.save(x)
}
