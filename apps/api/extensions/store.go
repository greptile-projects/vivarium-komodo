// Package extensions owns external integration identities and repository grants.
package extensions

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound  = errors.New("extension not found")
	ErrInvalid   = errors.New("invalid extension")
	ErrForbidden = errors.New("forbidden")
)

type Endpoint struct {
	URL               string     `json:"url"`
	VerificationToken string     `json:"verification_token,omitempty"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
}
type RotationPolicy struct {
	IntervalDays     int  `json:"interval_days"`
	OverlapHours     int  `json:"overlap_hours"`
	ContactOnFailure bool `json:"contact_on_failure"`
}
type Extension struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	OwnerID              string         `json:"owner_id"`
	OperatorContact      string         `json:"operator_contact"`
	Capabilities         []string       `json:"capabilities"`
	Callback             Endpoint       `json:"callback"`
	Actions              Endpoint       `json:"actions"`
	RequestedPermissions []string       `json:"requested_permissions"`
	EventTypes           []string       `json:"event_types"`
	RotationPolicy       RotationPolicy `json:"rotation_policy"`
	Status               string         `json:"status"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}
type Input struct {
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	OperatorContact      string         `json:"operator_contact"`
	Capabilities         []string       `json:"capabilities"`
	CallbackURL          string         `json:"callback_url"`
	ActionURL            string         `json:"action_url"`
	RequestedPermissions []string       `json:"requested_permissions"`
	EventTypes           []string       `json:"event_types"`
	RotationPolicy       RotationPolicy `json:"rotation_policy"`
}
type Installation struct {
	ID                  string               `json:"id"`
	ExtensionID         string               `json:"extension_id"`
	RepositoryID        string               `json:"repository_id"`
	InstallerID         string               `json:"installer_id"`
	Permissions         []string             `json:"permissions"`
	EventTypes          []string             `json:"event_types"`
	ResourceTypes       []string             `json:"resource_types"`
	CapabilityDecisions []CapabilityDecision `json:"capability_decisions"`
	Settings            map[string]string    `json:"settings,omitempty"`
	Version             int64                `json:"version"`
	Events              []InstallationEvent  `json:"events"`
	Status              string               `json:"status"`
	CreatedAt           time.Time            `json:"created_at"`
	RevokedAt           *time.Time           `json:"revoked_at,omitempty"`
	Authority           Authority            `json:"authority"`
}
type CapabilityDecision struct {
	Capability string `json:"capability"`
	Decision   string `json:"decision"`
}
type InstallationEvent struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type GrantInput struct {
	Permissions         []string             `json:"permissions"`
	EventTypes          []string             `json:"event_types"`
	ResourceTypes       []string             `json:"resource_types"`
	CapabilityDecisions []CapabilityDecision `json:"capability_decisions"`
	Settings            map[string]string    `json:"settings"`
}
type Authority struct {
	ActorID          string   `json:"actor_id"`
	PrincipalType    string   `json:"principal_type"`
	RepositoryID     string   `json:"repository_id"`
	Permissions      []string `json:"permissions"`
	EventTypes       []string `json:"event_types"`
	CanImpersonate   bool     `json:"can_impersonate"`
	CredentialIssued bool     `json:"credential_issued"`
	Warnings         []string `json:"warnings"`
}
type fileData struct {
	Extensions    []Extension    `json:"extensions"`
	Installations []Installation `json:"installations"`
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
func id(prefix string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}
func cleanURL(raw string) bool {
	u, e := url.Parse(strings.TrimSpace(raw))
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == ""
}
func listOK(xs []string, allowed map[string]bool) bool {
	if len(xs) == 0 || len(xs) > 30 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range xs {
		if !allowed[x] || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}

var permissions = map[string]bool{"metadata:read": true, "contents:read": true, "issues:read": true, "issues:write": true, "pull_requests:read": true, "pull_requests:write": true, "checks:write": true}
var events = map[string]bool{"repository.created": true, "push": true, "issue.opened": true, "issue.updated": true, "pull_request.opened": true, "pull_request.updated": true, "check.requested": true}
var resourceTypes = map[string]bool{"metadata": true, "contents": true, "issues": true, "pull_requests": true, "checks": true}

func valid(in Input) bool {
	mail, mailErr := url.Parse("mailto:" + in.OperatorContact)
	return strings.TrimSpace(in.Name) != "" && len(in.Name) <= 100 && strings.Contains(in.OperatorContact, "@") && mailErr == nil && mail != nil && cleanURL(in.CallbackURL) && cleanURL(in.ActionURL) && len(in.Capabilities) > 0 && listOK(in.RequestedPermissions, permissions) && listOK(in.EventTypes, events) && in.RotationPolicy.IntervalDays >= 1 && in.RotationPolicy.IntervalDays <= 365 && in.RotationPolicy.OverlapHours >= 0 && in.RotationPolicy.OverlapHours <= 168
}
func (s *Store) load() (fileData, error) {
	var d fileData
	b, e := os.ReadFile(filepath.Join(s.root, "extensions.json"))
	if errors.Is(e, os.ErrNotExist) {
		return d, nil
	}
	if e == nil {
		e = json.Unmarshal(b, &d)
	}
	return d, e
}
func (s *Store) save(d fileData) error {
	b, e := json.MarshalIndent(d, "", "  ")
	if e != nil {
		return e
	}
	t, e := os.CreateTemp(s.root, "extensions-*.tmp")
	if e != nil {
		return e
	}
	n := t.Name()
	defer os.Remove(n)
	_, e = t.Write(b)
	if e == nil {
		e = t.Sync()
	}
	if ce := t.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(s.root, "extensions.json"))
	}
	return e
}
func (s *Store) Create(owner string, in Input) (Extension, error) {
	if owner == "" || !valid(in) {
		return Extension{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Extension{}, e
	}
	now := s.now().UTC()
	x := Extension{ID: id("ext"), Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description), OwnerID: owner, OperatorContact: strings.TrimSpace(in.OperatorContact), Capabilities: in.Capabilities, Callback: Endpoint{URL: in.CallbackURL, VerificationToken: id("verify")}, Actions: Endpoint{URL: in.ActionURL, VerificationToken: id("verify")}, RequestedPermissions: in.RequestedPermissions, EventTypes: in.EventTypes, RotationPolicy: in.RotationPolicy, Status: "pending_verification", CreatedAt: now, UpdatedAt: now}
	d.Extensions = append(d.Extensions, x)
	return x, s.save(d)
}
func (s *Store) List(owner string) ([]Extension, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	out := []Extension{}
	for _, x := range d.Extensions {
		if x.OwnerID == owner {
			out = append(out, x)
		}
	}
	return out, e
}
func (s *Store) Get(ext string) (Extension, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Extension{}, e
	}
	for _, x := range d.Extensions {
		if x.ID == ext {
			return x, nil
		}
	}
	return Extension{}, ErrNotFound
}
func (s *Store) Verify(ext, owner, endpoint, token string) (Extension, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Extension{}, e
	}
	for i := range d.Extensions {
		x := &d.Extensions[i]
		if x.ID != ext {
			continue
		}
		if x.OwnerID != owner {
			return Extension{}, ErrForbidden
		}
		now := s.now().UTC()
		var p *Endpoint
		if endpoint == "callback" {
			p = &x.Callback
		} else if endpoint == "actions" {
			p = &x.Actions
		} else {
			return Extension{}, ErrInvalid
		}
		if p.VerificationToken != token {
			return Extension{}, ErrInvalid
		}
		p.VerifiedAt = &now
		p.VerificationToken = ""
		if x.Callback.VerifiedAt != nil && x.Actions.VerifiedAt != nil {
			x.Status = "verified"
		}
		x.UpdatedAt = now
		return *x, s.save(d)
	}
	return Extension{}, ErrNotFound
}
func (s *Store) Transfer(ext, owner, next, reason string) (Extension, error) {
	if next == "" || strings.TrimSpace(reason) == "" {
		return Extension{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return Extension{}, err
	}
	for n := range d.Extensions {
		x := &d.Extensions[n]
		if x.ID != ext {
			continue
		}
		if x.OwnerID != owner {
			return Extension{}, ErrForbidden
		}
		x.OwnerID = next
		x.UpdatedAt = s.now().UTC()
		return *x, s.save(d)
	}
	return Extension{}, ErrNotFound
}
func authority(x Extension, repo string, perms, ev []string) Authority {
	w := []string{}
	if x.Status != "verified" {
		w = append(w, "endpoint_ownership_unverified")
	}
	return Authority{ActorID: x.ID, PrincipalType: "extension", RepositoryID: repo, Permissions: perms, EventTypes: ev, CanImpersonate: false, CredentialIssued: false, Warnings: w}
}
func subset(got, want []string) bool {
	m := map[string]bool{}
	for _, x := range want {
		m[x] = true
	}
	for _, x := range got {
		if !m[x] {
			return false
		}
	}
	return true
}
func validateGrant(x Extension, in GrantInput) bool {
	if !listOK(in.Permissions, permissions) || !listOK(in.EventTypes, events) || !listOK(in.ResourceTypes, resourceTypes) || !subset(in.Permissions, x.RequestedPermissions) || !subset(in.EventTypes, x.EventTypes) || len(in.CapabilityDecisions) != len(x.Capabilities) || len(in.Settings) > 20 {
		return false
	}
	decisions := map[string]string{}
	for _, d := range in.CapabilityDecisions {
		decisions[d.Capability] = d.Decision
	}
	for _, capability := range x.Capabilities {
		if decisions[capability] != "approved" && decisions[capability] != "denied" {
			return false
		}
	}
	for key, value := range in.Settings {
		if strings.TrimSpace(key) == "" || len(key) > 100 || len(value) > 1000 || strings.Contains(strings.ToLower(key), "secret") || strings.Contains(strings.ToLower(key), "token") || strings.Contains(strings.ToLower(key), "password") {
			return false
		}
	}
	return true
}
func (s *Store) Preview(ext, repo string, perms, ev []string) (Authority, error) {
	x, e := s.Get(ext)
	if e != nil {
		return Authority{}, e
	}
	if !listOK(perms, permissions) || !listOK(ev, events) || !subset(perms, x.RequestedPermissions) || !subset(ev, x.EventTypes) {
		return Authority{}, ErrInvalid
	}
	return authority(x, repo, perms, ev), nil
}
func (s *Store) Install(ext, repo, installer string, perms, ev []string) (Installation, error) {
	return s.InstallGrant(ext, repo, installer, GrantInput{Permissions: perms, EventTypes: ev, ResourceTypes: []string{"metadata"}, CapabilityDecisions: approvedCapabilities(s, ext)})
}
func approvedCapabilities(s *Store, ext string) []CapabilityDecision {
	x, err := s.Get(ext)
	if err != nil {
		return nil
	}
	out := make([]CapabilityDecision, 0, len(x.Capabilities))
	for _, c := range x.Capabilities {
		out = append(out, CapabilityDecision{Capability: c, Decision: "approved"})
	}
	return out
}
func (s *Store) InstallGrant(ext, repo, installer string, in GrantInput) (Installation, error) {
	x, e := s.Get(ext)
	if e != nil || !validateGrant(x, in) {
		if e != nil {
			return Installation{}, e
		}
		return Installation{}, ErrInvalid
	}
	a, e := s.Preview(ext, repo, in.Permissions, in.EventTypes)
	if e != nil {
		return Installation{}, e
	}
	if len(a.Warnings) > 0 {
		return Installation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Installation{}, e
	}
	now := s.now().UTC()
	i := Installation{ID: id("ins"), ExtensionID: ext, RepositoryID: repo, InstallerID: installer, Permissions: in.Permissions, EventTypes: in.EventTypes, ResourceTypes: in.ResourceTypes, CapabilityDecisions: in.CapabilityDecisions, Settings: in.Settings, Version: 1, Events: []InstallationEvent{{Sequence: 1, Type: "installed", ActorID: installer, CreatedAt: now}}, Status: "active", CreatedAt: now, Authority: a}
	d.Installations = append(d.Installations, i)
	return i, s.save(d)
}
func (s *Store) Update(repo, installation, actor, action, reason string, expected int64, grant *GrantInput) (Installation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return Installation{}, err
	}
	for n := range d.Installations {
		i := &d.Installations[n]
		if i.ID != installation || i.RepositoryID != repo {
			continue
		}
		if expected != i.Version {
			return Installation{}, ErrInvalid
		}
		x := Extension{}
		for _, candidate := range d.Extensions {
			if candidate.ID == i.ExtensionID {
				x = candidate
			}
		}
		now := s.now().UTC()
		switch action {
		case "suspend":
			if i.Status != "active" {
				return Installation{}, ErrInvalid
			}
			i.Status = "suspended"
			i.Authority.Permissions = []string{}
			i.Authority.EventTypes = []string{}
		case "resume":
			if i.Status != "suspended" {
				return Installation{}, ErrInvalid
			}
			i.Status = "active"
			i.Authority = authority(x, repo, i.Permissions, i.EventTypes)
		case "upgrade":
			if i.Status == "removed" || grant == nil || !validateGrant(x, *grant) {
				return Installation{}, ErrInvalid
			}
			i.Permissions = grant.Permissions
			i.EventTypes = grant.EventTypes
			i.ResourceTypes = grant.ResourceTypes
			i.CapabilityDecisions = grant.CapabilityDecisions
			i.Settings = grant.Settings
			if i.Status == "active" {
				i.Authority = authority(x, repo, i.Permissions, i.EventTypes)
			}
		case "remove":
			if i.Status == "removed" {
				return Installation{}, ErrInvalid
			}
			i.Status = "removed"
			i.RevokedAt = &now
			i.Authority.Permissions = []string{}
			i.Authority.EventTypes = []string{}
		default:
			return Installation{}, ErrInvalid
		}
		i.Version++
		i.Events = append(i.Events, InstallationEvent{Sequence: int64(len(i.Events) + 1), Type: action, ActorID: actor, Reason: strings.TrimSpace(reason), CreatedAt: now})
		return *i, s.save(d)
	}
	return Installation{}, ErrNotFound
}
func (s *Store) ListInstallations(repo string) ([]Installation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	o := []Installation{}
	for _, i := range d.Installations {
		if i.RepositoryID == repo {
			o = append(o, i)
		}
	}
	return o, e
}
func (s *Store) Revoke(repo, installation, actor string) (Installation, error) {
	items, err := s.ListInstallations(repo)
	if err != nil {
		return Installation{}, err
	}
	for _, i := range items {
		if i.ID == installation {
			out, e := s.Update(repo, installation, actor, "remove", "", i.Version, nil)
			if e == nil {
				out.Status = "revoked"
			}
			return out, e
		}
	}
	return Installation{}, ErrNotFound
}
