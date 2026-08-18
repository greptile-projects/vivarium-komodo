// Package runtimeinvestigations retains shared, cited explanations of live
// behavior. It stores references to sanitized evidence, never provider access,
// secrets, credentials, or mutation authority.
package runtimeinvestigations

import (
	"crypto/rand"
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
)

var ErrNotFound = errors.New("runtime investigation not found")
var ErrInvalid = errors.New("invalid runtime investigation")
var ErrForbidden = errors.New("runtime investigation forbidden")

type Evidence struct {
	ID         string `json:"id"`
	ProbeID    string `json:"probe_id"`
	CaptureID  string `json:"capture_id,omitempty"`
	Kind       string `json:"kind"`
	Summary    string `json:"summary"`
	Audience   string `json:"audience"`
	Accessible bool   `json:"accessible"`
	Reason     string `json:"reason,omitempty"`
}
type Correlation struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	ResourceID   string `json:"resource_id"`
	Revision     string `json:"revision"`
	Path         string `json:"path,omitempty"`
	Symbol       string `json:"symbol,omitempty"`
	Relationship string `json:"relationship"`
	Status       string `json:"status"`
	Reason       string `json:"reason,omitempty"`
}
type Citation struct {
	EvidenceID    string `json:"evidence_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	ClaimID       string `json:"claim_id,omitempty"`
}
type Claim struct {
	ID             string     `json:"id"`
	Kind           string     `json:"kind"`
	Body           string     `json:"body"`
	Uncertainty    string     `json:"uncertainty,omitempty"`
	ActorID        string     `json:"actor_id"`
	AgentSessionID string     `json:"agent_session_id,omitempty"`
	Verdict        string     `json:"verdict,omitempty"`
	Citations      []Citation `json:"citations"`
	Status         string     `json:"status"`
	Stale          bool       `json:"stale"`
	BlockedReasons []string   `json:"blocked_reasons,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
type OwnerRequest struct {
	ID              string    `json:"id"`
	OwnerKind       string    `json:"owner_kind"`
	OwnerID         string    `json:"owner_id"`
	Question        string    `json:"question"`
	Status          string    `json:"status"`
	RequestedBy     string    `json:"requested_by"`
	ResponseClaimID string    `json:"response_claim_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
type AgentSession struct {
	ID             string    `json:"id"`
	AgentID        string    `json:"agent_id"`
	Mandate        string    `json:"mandate"`
	Status         string    `json:"status"`
	AuthorizedBy   string    `json:"authorized_by"`
	TokenDigest    string    `json:"token_digest,omitempty"`
	EvidenceIDs    []string  `json:"evidence_ids"`
	CorrelationIDs []string  `json:"correlation_ids"`
	Guidance       []string  `json:"guidance"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type Event struct {
	Sequence   int64     `json:"sequence"`
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	ActorID    string    `json:"actor_id"`
	Detail     string    `json:"detail"`
	CreatedAt  time.Time `json:"created_at"`
}
type Investigation struct {
	ID            string         `json:"id"`
	RepositoryID  string         `json:"repository_id"`
	WorkspaceID   string         `json:"workspace_id"`
	Revision      string         `json:"revision"`
	Title         string         `json:"title"`
	Question      string         `json:"question"`
	Audience      string         `json:"audience"`
	CreatorID     string         `json:"creator_id"`
	Participants  []string       `json:"participants"`
	Evidence      []Evidence     `json:"evidence"`
	Correlations  []Correlation  `json:"correlations"`
	Claims        []Claim        `json:"claims"`
	OwnerRequests []OwnerRequest `json:"owner_requests"`
	AgentSessions []AgentSession `json:"agent_sessions"`
	Events        []Event        `json:"events"`
	Authority     []string       `json:"authority"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}
type CreateInput struct {
	WorkspaceID  string        `json:"workspace_id"`
	Revision     string        `json:"revision"`
	Title        string        `json:"title"`
	Question     string        `json:"question"`
	Audience     string        `json:"audience"`
	Participants []string      `json:"participants"`
	Evidence     []Evidence    `json:"evidence"`
	Correlations []Correlation `json:"correlations"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

// MarshalJSON is the reader projection: bearer digests are durable internal
// state and are never part of an investigation response.
func (v Investigation) MarshalJSON() ([]byte, error) {
	type public Investigation
	copy := v
	copy.AgentSessions = append([]AgentSession(nil), v.AgentSessions...)
	for i := range copy.AgentSessions {
		copy.AgentSessions[i].TokenDigest = ""
	}
	return json.Marshal(public(copy))
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, ErrInvalid
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	return &Store{root: root, now: time.Now}, nil
}
func id() string             { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func token() string          { b := make([]byte, 32); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func digest(v string) string { x := sha256.Sum256([]byte(v)); return hex.EncodeToString(x[:]) }
func uniq(xs []string) []string {
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
func has(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func (s *Store) event(v *Investigation, kind, resource, actor, detail string) {
	now := s.now().UTC()
	v.Events = append(v.Events, Event{Sequence: int64(len(v.Events) + 1), Kind: kind, ResourceID: resource, ActorID: actor, Detail: detail, CreatedAt: now})
	v.UpdatedAt = now
}

func (s *Store) Create(repo, actor string, in CreateInput) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || actor == "" || in.WorkspaceID == "" || in.Revision == "" || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Question) == "" || !map[string]bool{"repository": true, "participants": true}[in.Audience] || len(in.Evidence) == 0 || len(in.Correlations) == 0 {
		return Investigation{}, ErrInvalid
	}
	for i := range in.Evidence {
		e := &in.Evidence[i]
		if e.ProbeID == "" || e.Kind == "" || e.Summary == "" || !map[string]bool{"repository": true, "participants": true}[e.Audience] || (!e.Accessible && e.Reason == "") {
			return Investigation{}, ErrInvalid
		}
		e.ID = id()
	}
	allowed := map[string]bool{"symbol": true, "commit": true, "dependency": true, "configuration": true, "infrastructure": true, "deployment": true, "known_issue": true}
	for i := range in.Correlations {
		c := &in.Correlations[i]
		if !allowed[c.Kind] || c.ResourceID == "" || c.Revision == "" || c.Relationship == "" || !map[string]bool{"resolved": true, "inaccessible": true}[c.Status] || (c.Status == "inaccessible" && c.Reason == "") {
			return Investigation{}, ErrInvalid
		}
		c.ID = id()
	}
	now := s.now().UTC()
	v := Investigation{ID: id(), RepositoryID: repo, WorkspaceID: in.WorkspaceID, Revision: in.Revision, Title: strings.TrimSpace(in.Title), Question: strings.TrimSpace(in.Question), Audience: in.Audience, CreatorID: actor, Participants: uniq(append(in.Participants, actor)), Evidence: in.Evidence, Correlations: in.Correlations, Authority: []string{}, CreatedAt: now, UpdatedAt: now}
	s.event(&v, "investigation.opened", v.ID, actor, "Shared explanation opened without runtime or mutation authority.")
	return v, s.write(v)
}
func (s *Store) Get(repo, x string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	s.derive(&v)
	return v, nil
}
func (s *Store) List(repo, workspace string) ([]Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Investigation{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		v, e := s.read(strings.TrimSuffix(f.Name(), ".json"))
		if e == nil && v.RepositoryID == repo && (workspace == "" || v.WorkspaceID == workspace) {
			s.derive(&v)
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) AddClaim(repo, x, actor string, in Claim) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if !has(v.Participants, actor) {
		return Investigation{}, ErrForbidden
	}
	return s.addClaim(v, actor, "", in)
}
func (s *Store) addClaim(v Investigation, actor, session string, in Claim) (Investigation, error) {
	if !map[string]bool{"hypothesis": true, "query": true, "finding": true, "challenge": true, "uncertainty": true, "support": true}[in.Kind] || strings.TrimSpace(in.Body) == "" || len(in.Citations) == 0 {
		return Investigation{}, ErrInvalid
	}
	for _, c := range in.Citations {
		if c.EvidenceID == "" && c.CorrelationID == "" && c.ClaimID == "" {
			return Investigation{}, ErrInvalid
		}
		if c.EvidenceID != "" && !evidence(v, c.EvidenceID) {
			return Investigation{}, ErrInvalid
		}
		if c.CorrelationID != "" && !correlation(v, c.CorrelationID) {
			return Investigation{}, ErrInvalid
		}
		if c.ClaimID != "" && !claim(v, c.ClaimID) {
			return Investigation{}, ErrInvalid
		}
	}
	in.ID = id()
	in.ActorID = actor
	in.AgentSessionID = session
	in.CreatedAt = s.now().UTC()
	in.Stale = false
	v.Claims = append(v.Claims, in)
	s.event(&v, "claim."+in.Kind, in.ID, actor, in.Body)
	s.derive(&v)
	return v, s.write(v)
}
func (s *Store) RequestOwner(repo, x, actor string, in OwnerRequest) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if !has(v.Participants, actor) || !map[string]bool{"code": true, "service": true, "privacy": true, "security": true}[in.OwnerKind] || in.OwnerID == "" || in.Question == "" {
		return Investigation{}, ErrInvalid
	}
	in.ID = id()
	in.Status = "requested"
	in.RequestedBy = actor
	in.CreatedAt = s.now().UTC()
	v.OwnerRequests = append(v.OwnerRequests, in)
	s.event(&v, "owner_input.requested", in.ID, actor, in.OwnerKind+":"+in.OwnerID)
	return v, s.write(v)
}
func (s *Store) StartAgent(repo, x, actor, agent, mandate string, evidenceIDs, correlationIDs []string, expires time.Time) (Investigation, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, "", ErrNotFound
	}
	now := s.now().UTC()
	if !has(v.Participants, actor) || agent == "" || mandate == "" || !expires.After(now) || expires.After(now.Add(24*time.Hour)) {
		return Investigation{}, "", ErrInvalid
	}
	for _, z := range evidenceIDs {
		if !evidence(v, z) {
			return Investigation{}, "", ErrInvalid
		}
	}
	for _, z := range correlationIDs {
		if !correlation(v, z) {
			return Investigation{}, "", ErrInvalid
		}
	}
	secret := token()
	a := AgentSession{ID: id(), AgentID: agent, Mandate: mandate, Status: "active", AuthorizedBy: actor, TokenDigest: digest(secret), EvidenceIDs: uniq(evidenceIDs), CorrelationIDs: uniq(correlationIDs), ExpiresAt: expires.UTC(), CreatedAt: now, UpdatedAt: now}
	v.AgentSessions = append(v.AgentSessions, a)
	s.event(&v, "agent.started", a.ID, actor, agent)
	return v, secret, s.write(v)
}
func (s *Store) ControlAgent(repo, x, session, actor, action, guidance string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if !has(v.Participants, actor) || !map[string]bool{"guide": true, "pause": true, "resume": true, "revoke": true}[action] {
		return Investigation{}, ErrInvalid
	}
	found := false
	for i := range v.AgentSessions {
		a := &v.AgentSessions[i]
		if a.ID != session {
			continue
		}
		found = true
		switch action {
		case "guide":
			if strings.TrimSpace(guidance) == "" {
				return Investigation{}, ErrInvalid
			}
			a.Guidance = append(a.Guidance, guidance)
		case "pause":
			a.Status = "paused"
		case "resume":
			if a.Status != "paused" {
				return Investigation{}, ErrInvalid
			}
			a.Status = "active"
		case "revoke":
			a.Status = "revoked"
			a.TokenDigest = ""
		}
		a.UpdatedAt = s.now().UTC()
	}
	if !found {
		return Investigation{}, ErrNotFound
	}
	s.event(&v, "agent."+action, session, actor, guidance)
	return v, s.write(v)
}
func (s *Store) AgentContext(secret string) (Investigation, AgentSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, a, e := s.agent(secret)
	if e != nil {
		return v, a, e
	}
	s.derive(&v)
	return v, a, nil
}
func (s *Store) AgentClaim(secret string, in Claim) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, a, e := s.agent(secret)
	if e != nil {
		return Investigation{}, e
	}
	for _, c := range in.Citations {
		if c.EvidenceID != "" && !has(a.EvidenceIDs, c.EvidenceID) {
			return Investigation{}, ErrForbidden
		}
		if c.CorrelationID != "" && !has(a.CorrelationIDs, c.CorrelationID) {
			return Investigation{}, ErrForbidden
		}
	}
	return s.addClaim(v, a.AgentID, a.ID, in)
}
func (s *Store) agent(secret string) (Investigation, AgentSession, error) {
	if secret == "" {
		return Investigation{}, AgentSession{}, ErrForbidden
	}
	es, e := os.ReadDir(s.root)
	if e != nil {
		return Investigation{}, AgentSession{}, e
	}
	d := digest(secret)
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		v, e := s.read(strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			continue
		}
		for _, a := range v.AgentSessions {
			if a.TokenDigest == d {
				if a.Status != "active" || !a.ExpiresAt.After(s.now().UTC()) {
					return Investigation{}, AgentSession{}, ErrForbidden
				}
				return v, a, nil
			}
		}
	}
	return Investigation{}, AgentSession{}, ErrForbidden
}
func (s *Store) derive(v *Investigation) {
	for i := range v.Claims {
		c := &v.Claims[i]
		c.Status = "proposed"
		if c.Kind == "finding" || c.Kind == "support" || c.Verdict == "supported" {
			c.Status = "supported"
		}
		c.Stale = false
		c.BlockedReasons = nil
		if c.Verdict == "disputed" {
			c.Status = "disputed"
		}
		for _, z := range c.Citations {
			if z.EvidenceID != "" {
				for _, e := range v.Evidence {
					if e.ID == z.EvidenceID && !e.Accessible {
						c.Status = "blocked"
						c.BlockedReasons = append(c.BlockedReasons, e.Reason)
					}
				}
			}
			if z.CorrelationID != "" {
				for _, r := range v.Correlations {
					if r.ID == z.CorrelationID && r.Status == "inaccessible" {
						c.Status = "blocked"
						c.BlockedReasons = append(c.BlockedReasons, r.Reason)
					}
					if r.ID == z.CorrelationID && (r.Kind == "symbol" || r.Kind == "commit") && r.Revision != v.Revision {
						c.Stale = true
						c.Status = "stale"
					}
				}
			}
		}
		for _, other := range v.Claims {
			for _, z := range other.Citations {
				if z.ClaimID == c.ID && other.Kind == "challenge" {
					c.Status = "disputed"
				}
			}
		}
	}
}
func evidence(v Investigation, x string) bool {
	for _, e := range v.Evidence {
		if e.ID == x {
			return true
		}
	}
	return false
}
func correlation(v Investigation, x string) bool {
	for _, e := range v.Correlations {
		if e.ID == x {
			return true
		}
	}
	return false
}
func claim(v Investigation, x string) bool {
	for _, e := range v.Claims {
		if e.ID == x {
			return true
		}
	}
	return false
}
func (s *Store) read(x string) (Investigation, error) {
	var v Investigation
	b, e := os.ReadFile(filepath.Join(s.root, x+".json"))
	if e != nil {
		return v, e
	}
	e = json.Unmarshal(b, &v)
	return v, e
}
func (s *Store) write(v Investigation) error {
	type durable Investigation
	b, e := json.MarshalIndent(durable(v), "", "  ")
	if e != nil {
		return e
	}
	p := filepath.Join(s.root, "."+v.ID+".tmp")
	if e = os.WriteFile(p, b, 0600); e != nil {
		return e
	}
	return os.Rename(p, filepath.Join(s.root, v.ID+".json"))
}
