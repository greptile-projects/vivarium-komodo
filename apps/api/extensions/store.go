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
	ID           string     `json:"id"`
	ExtensionID  string     `json:"extension_id"`
	RepositoryID string     `json:"repository_id"`
	InstallerID  string     `json:"installer_id"`
	Permissions  []string   `json:"permissions"`
	EventTypes   []string   `json:"event_types"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	Authority    Authority  `json:"authority"`
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
	a, e := s.Preview(ext, repo, perms, ev)
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
	i := Installation{ID: id("ins"), ExtensionID: ext, RepositoryID: repo, InstallerID: installer, Permissions: perms, EventTypes: ev, Status: "active", CreatedAt: now, Authority: a}
	d.Installations = append(d.Installations, i)
	return i, s.save(d)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Installation{}, e
	}
	for n := range d.Installations {
		i := &d.Installations[n]
		if i.ID == installation && i.RepositoryID == repo {
			if i.Status != "active" {
				return Installation{}, ErrInvalid
			}
			now := s.now().UTC()
			i.Status = "revoked"
			i.RevokedAt = &now
			i.Authority.Permissions = []string{}
			i.Authority.EventTypes = []string{}
			return *i, s.save(d)
		}
	}
	return Installation{}, ErrNotFound
}
