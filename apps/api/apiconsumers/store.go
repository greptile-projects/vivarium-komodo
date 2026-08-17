// Package apiconsumers owns API consumer registrations, scoped credentials, and synthetic sandbox evidence.
package apiconsumers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/apicontracts"
)

var ErrNotFound = errors.New("api consumer application not found")
var ErrInvalid = errors.New("invalid api consumer operation")
var ErrConflict = errors.New("api consumer application changed")
var ErrForbidden = errors.New("api consumer operation forbidden")
var ErrQuota = errors.New("api sandbox quota exceeded")

type RegistrationInput struct {
	Name                    string   `json:"name"`
	Description             string   `json:"description"`
	ConsumerProject         string   `json:"consumer_project"`
	Contact                 string   `json:"contact"`
	ContractID              string   `json:"contract_id"`
	ContractVersion         int64    `json:"contract_version"`
	Environments            []string `json:"environments"`
	Capabilities            []string `json:"capabilities"`
	CredentialLifetimeHours int      `json:"credential_lifetime_hours"`
}
type FailureRule struct {
	ID          string         `json:"id"`
	OperationID string         `json:"operation_id"`
	Status      int            `json:"status"`
	ErrorCode   string         `json:"error_code"`
	Response    map[string]any `json:"response"`
}
type ApprovalInput struct {
	ExpectedVersion         int64                     `json:"expected_version"`
	Decision                string                    `json:"decision"`
	Capabilities            []string                  `json:"capabilities"`
	Environments            []string                  `json:"environments"`
	Quota                   int                       `json:"quota"`
	CredentialLifetimeHours int                       `json:"credential_lifetime_hours"`
	SyntheticData           map[string]map[string]any `json:"synthetic_data"`
	FailureRules            []FailureRule             `json:"failure_rules"`
	Reason                  string                    `json:"reason"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Inspection struct {
	Sequence        int64             `json:"sequence"`
	OperationID     string            `json:"operation_id"`
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	RequestHeaders  map[string]string `json:"request_headers"`
	RequestBody     map[string]any    `json:"request_body"`
	ResponseStatus  int               `json:"response_status"`
	ResponseHeaders map[string]string `json:"response_headers"`
	ResponseBody    map[string]any    `json:"response_body"`
	FailureRule     string            `json:"failure_rule,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
}
type Application struct {
	ID                   string                    `json:"id"`
	RepositoryID         string                    `json:"repository_id"`
	OwnerID              string                    `json:"owner_id"`
	PendingOwnerID       string                    `json:"pending_owner_id,omitempty"`
	Registration         RegistrationInput         `json:"registration"`
	ContractName         string                    `json:"contract_name"`
	ContractLabel        string                    `json:"contract_label"`
	ContractOperations   []apicontracts.Operation  `json:"contract_operations"`
	Version              int64                     `json:"version"`
	Status               string                    `json:"status"`
	ApprovedCapabilities []string                  `json:"approved_capabilities"`
	ApprovedEnvironments []string                  `json:"approved_environments"`
	Quota                int                       `json:"quota"`
	Used                 int                       `json:"used"`
	SyntheticData        map[string]map[string]any `json:"synthetic_data"`
	FailureRules         []FailureRule             `json:"failure_rules"`
	CredentialIssuedAt   *time.Time                `json:"credential_issued_at,omitempty"`
	CredentialExpiresAt  *time.Time                `json:"credential_expires_at,omitempty"`
	CredentialState      string                    `json:"credential_state"`
	Inspections          []Inspection              `json:"inspections"`
	Events               []Event                   `json:"events"`
	CreatedAt            time.Time                 `json:"created_at"`
	UpdatedAt            time.Time                 `json:"updated_at"`
	Authority            []string                  `json:"authority"`
}
type Credential struct {
	Secret      string      `json:"secret"`
	ExpiresAt   time.Time   `json:"expires_at"`
	Application Application `json:"application"`
}
type SandboxInput struct {
	OperationID string         `json:"operation_id"`
	Body        map[string]any `json:"body"`
	Failure     string         `json:"failure,omitempty"`
}
type data struct {
	Applications []Application     `json:"applications"`
	Credentials  map[string]string `json:"credentials"`
}
type Store struct {
	root      string
	contracts *apicontracts.Store
	mu        sync.Mutex
	now       func() time.Time
}

func New(root string, contracts *apicontracts.Store) (*Store, error) {
	if root == "" || contracts == nil {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, contracts: contracts, now: time.Now}, e
}
func ident(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + "_" + hex.EncodeToString(b[:])
}
func digest(v string) string  { d := sha256.Sum256([]byte(v)); return hex.EncodeToString(d[:]) }
func (s *Store) file() string { return filepath.Join(s.root, "consumer-applications.json") }
func (s *Store) load() (data, error) {
	b, e := os.ReadFile(s.file())
	if errors.Is(e, fs.ErrNotExist) {
		return data{Credentials: map[string]string{}}, nil
	}
	var d data
	if e != nil {
		return d, e
	}
	if json.Unmarshal(b, &d) != nil {
		return d, ErrInvalid
	}
	if d.Credentials == nil {
		d.Credentials = map[string]string{}
	}
	return d, nil
}
func (s *Store) save(d data) error {
	b, e := json.MarshalIndent(d, "", "  ")
	if e == nil {
		e = os.WriteFile(s.file(), append(b, '\n'), 0640)
	}
	return e
}
func unique(xs []string) bool {
	if len(xs) == 0 {
		return false
	}
	m := map[string]bool{}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || m[x] {
			return false
		}
		m[x] = true
	}
	return true
}
func subset(xs []string, allowed map[string]bool) bool {
	if !unique(xs) {
		return false
	}
	for _, x := range xs {
		if !allowed[x] {
			return false
		}
	}
	return true
}
func event(a *Application, actor, kind, reason string, now time.Time) {
	a.Events = append(a.Events, Event{Sequence: int64(len(a.Events) + 1), Type: kind, ActorID: actor, Reason: reason, CreatedAt: now})
	a.UpdatedAt = now
	a.Version++
}
func findVersion(c apicontracts.Contract, n int64) (apicontracts.Version, bool) {
	for _, v := range c.Versions {
		if v.Number == n {
			return v, true
		}
	}
	return apicontracts.Version{}, false
}

func (s *Store) Register(repo, actor string, in RegistrationInput) (Application, error) {
	if actor == "" || in.Name == "" || in.ConsumerProject == "" || in.Contact == "" || in.ContractID == "" || in.CredentialLifetimeHours < 1 || in.CredentialLifetimeHours > 2160 || !unique(in.Environments) || !unique(in.Capabilities) {
		return Application{}, ErrInvalid
	}
	c, e := s.contracts.Get(repo, in.ContractID)
	if e != nil {
		return Application{}, ErrInvalid
	}
	v, ok := findVersion(c, in.ContractVersion)
	if !ok || !v.DefinitionValid {
		return Application{}, ErrInvalid
	}
	envs := map[string]bool{}
	for _, x := range v.Environments {
		if x.Availability == "available" {
			envs[x.Name] = true
		}
	}
	caps := map[string]bool{}
	for _, x := range v.Authentication {
		for _, p := range x.Scopes {
			caps[p] = true
		}
	}
	if !subset(in.Environments, envs) || !subset(in.Capabilities, caps) {
		return Application{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Application{}, e
	}
	now := s.now().UTC()
	a := Application{ID: ident("app"), RepositoryID: repo, OwnerID: actor, Registration: in, ContractName: v.Name, ContractLabel: v.Version, ContractOperations: v.Operations, Version: 1, Status: "pending", CredentialState: "not_issued", CreatedAt: now, UpdatedAt: now, Authority: []string{}}
	a.Events = []Event{{Sequence: 1, Type: "registration_requested", ActorID: actor, CreatedAt: now}}
	d.Applications = append(d.Applications, a)
	return a, s.save(d)
}
func (s *Store) List(repo, actor string, writer bool) ([]Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return nil, e
	}
	changed := s.expire(&d)
	out := []Application{}
	for _, a := range d.Applications {
		if a.RepositoryID == repo && (writer || a.OwnerID == actor || a.PendingOwnerID == actor) {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if changed {
		e = s.save(d)
	}
	return out, nil
}
func (s *Store) Get(repo, id, actor string, writer bool) (Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Application{}, e
	}
	changed := s.expire(&d)
	for _, a := range d.Applications {
		if a.RepositoryID == repo && a.ID == id {
			if !writer && a.OwnerID != actor && a.PendingOwnerID != actor {
				return Application{}, ErrForbidden
			}
			if changed {
				e = s.save(d)
			}
			return a, e
		}
	}
	return Application{}, ErrNotFound
}

func (s *Store) expire(d *data) bool {
	now := s.now().UTC()
	changed := false
	for i := range d.Applications {
		a := &d.Applications[i]
		if a.Status == "active" && a.CredentialExpiresAt != nil && !now.Before(*a.CredentialExpiresAt) {
			a.Status = "expired"
			a.CredentialState = "expired"
			delete(d.Credentials, a.ID)
			event(a, "platform", "expired", "credential lifetime elapsed", now)
			changed = true
		}
	}
	return changed
}
func issue(d *data, a *Application, now time.Time) (Credential, error) {
	token := "vka_" + ident("secret")
	d.Credentials[a.ID] = digest(token)
	hours := a.Registration.CredentialLifetimeHours
	if hours < 1 {
		hours = 24
	}
	expires := now.Add(time.Duration(hours) * time.Hour)
	a.CredentialIssuedAt = &now
	a.CredentialExpiresAt = &expires
	a.CredentialState = "active"
	return Credential{Secret: token, ExpiresAt: expires, Application: *a}, nil
}
func (s *Store) Decide(repo, id, actor string, in ApprovalInput) (Credential, error) {
	if actor == "" || in.ExpectedVersion < 1 || !map[string]bool{"approved": true, "denied": true}[in.Decision] || in.Reason == "" {
		return Credential{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Credential{}, e
	}
	for i := range d.Applications {
		a := &d.Applications[i]
		if a.RepositoryID != repo || a.ID != id {
			continue
		}
		if a.Version != in.ExpectedVersion || a.Status != "pending" {
			return Credential{}, ErrConflict
		}
		now := s.now().UTC()
		if in.Decision == "denied" {
			a.Status = "denied"
			a.CredentialState = "not_issued"
			event(a, actor, "denied", in.Reason, now)
			return Credential{Application: *a}, s.save(d)
		}
		reqCaps := map[string]bool{}
		for _, x := range a.Registration.Capabilities {
			reqCaps[x] = true
		}
		reqEnvs := map[string]bool{}
		for _, x := range a.Registration.Environments {
			reqEnvs[x] = true
		}
		if !subset(in.Capabilities, reqCaps) || !subset(in.Environments, reqEnvs) || in.Quota < 1 || in.Quota > 10000 || in.CredentialLifetimeHours < 1 || in.CredentialLifetimeHours > a.Registration.CredentialLifetimeHours {
			return Credential{}, ErrInvalid
		}
		for _, r := range in.FailureRules {
			if r.ID == "" || r.OperationID == "" || r.Status < 400 || r.Status > 599 {
				return Credential{}, ErrInvalid
			}
		}
		a.Status = "approved"
		a.ApprovedCapabilities = in.Capabilities
		a.ApprovedEnvironments = in.Environments
		a.Quota = in.Quota
		a.SyntheticData = in.SyntheticData
		a.FailureRules = in.FailureRules
		a.Registration.CredentialLifetimeHours = in.CredentialLifetimeHours
		event(a, actor, "approved", in.Reason, now)
		return Credential{Application: *a}, s.save(d)
	}
	return Credential{}, ErrNotFound
}

func (s *Store) Consent(repo, id, actor, reason string, expected int64) (Credential, error) {
	if actor == "" || reason == "" || expected < 1 {
		return Credential{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Credential{}, e
	}
	for i := range d.Applications {
		a := &d.Applications[i]
		if a.RepositoryID == repo && a.ID == id {
			if a.OwnerID != actor {
				return Credential{}, ErrForbidden
			}
			if a.Status != "approved" || a.Version != expected {
				return Credential{}, ErrConflict
			}
			now := s.now().UTC()
			a.Status = "active"
			event(a, actor, "terms_accepted", reason, now)
			credential, e := issue(&d, a, now)
			if e == nil {
				a.Events = append(a.Events, Event{Sequence: int64(len(a.Events) + 1), Type: "credential_issued", ActorID: actor, CreatedAt: now})
				credential.Application = *a
				e = s.save(d)
			}
			return credential, e
		}
	}
	return Credential{}, ErrNotFound
}
func (s *Store) Rotate(repo, id, actor, reason string, writer bool) (Credential, error) {
	if actor == "" || reason == "" {
		return Credential{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Credential{}, e
	}
	for i := range d.Applications {
		a := &d.Applications[i]
		if a.RepositoryID == repo && a.ID == id {
			if !writer && a.OwnerID != actor {
				return Credential{}, ErrForbidden
			}
			if a.Status != "active" {
				return Credential{}, ErrConflict
			}
			now := s.now().UTC()
			event(a, actor, "credential_rotated", reason, now)
			c, e := issue(&d, a, now)
			c.Application = *a
			if e == nil {
				e = s.save(d)
			}
			return c, e
		}
	}
	return Credential{}, ErrNotFound
}
func (s *Store) Control(repo, id, actor, action, reason string, writer bool) (Application, error) {
	if actor == "" || reason == "" || !map[string]bool{"revoke": true, "report_exposure": true, "expire": true, "reapply": true}[action] {
		return Application{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Application{}, e
	}
	for i := range d.Applications {
		a := &d.Applications[i]
		if a.RepositoryID == repo && a.ID == id {
			if !writer && a.OwnerID != actor {
				return Application{}, ErrForbidden
			}
			if action == "revoke" && !writer {
				return Application{}, ErrForbidden
			}
			now := s.now().UTC()
			switch action {
			case "reapply":
				if a.Status != "denied" && a.Status != "revoked" && a.Status != "expired" {
					return Application{}, ErrConflict
				}
				a.Status = "pending"
				a.CredentialState = "not_issued"
			case "report_exposure":
				if a.Status != "active" {
					return Application{}, ErrConflict
				}
				a.CredentialState = "exposed"
				delete(d.Credentials, a.ID)
			case "expire":
				if !writer {
					return Application{}, ErrForbidden
				}
				a.Status = "expired"
				a.CredentialState = "expired"
				delete(d.Credentials, a.ID)
			case "revoke":
				a.Status = "revoked"
				a.CredentialState = "revoked"
				delete(d.Credentials, a.ID)
			}
			event(a, actor, action, reason, now)
			return *a, s.save(d)
		}
	}
	return Application{}, ErrNotFound
}
func (s *Store) Transfer(repo, id, actor, target, reason string, accept, writer bool) (Application, error) {
	if actor == "" || reason == "" {
		return Application{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Application{}, e
	}
	for i := range d.Applications {
		a := &d.Applications[i]
		if a.RepositoryID == repo && a.ID == id {
			now := s.now().UTC()
			if !accept {
				if a.OwnerID != actor && !writer {
					return Application{}, ErrForbidden
				}
				if target == "" || target == a.OwnerID {
					return Application{}, ErrInvalid
				}
				a.PendingOwnerID = target
				event(a, actor, "ownership_transfer_requested", reason, now)
			} else {
				if a.PendingOwnerID != actor {
					return Application{}, ErrForbidden
				}
				a.OwnerID = actor
				a.PendingOwnerID = ""
				a.Status = "pending"
				a.CredentialState = "not_issued"
				delete(d.Credentials, a.ID)
				event(a, actor, "ownership_transfer_accepted", reason, now)
			}
			return *a, s.save(d)
		}
	}
	return Application{}, ErrNotFound
}
func (s *Store) Sandbox(application, token string, in SandboxInput) (Inspection, error) {
	if application == "" || token == "" || in.OperationID == "" {
		return Inspection{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Inspection{}, e
	}
	got := digest(token)
	for i := range d.Applications {
		a := &d.Applications[i]
		if a.ID != application {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(d.Credentials[a.ID])) != 1 {
			continue
		}
		now := s.now().UTC()
		if a.Status != "active" || a.CredentialState != "active" || a.CredentialExpiresAt == nil || !now.Before(*a.CredentialExpiresAt) {
			return Inspection{}, ErrForbidden
		}
		if a.Used >= a.Quota {
			return Inspection{}, ErrQuota
		}
		var op *apicontracts.Operation
		for j := range a.ContractOperations {
			if a.ContractOperations[j].ID == in.OperationID {
				op = &a.ContractOperations[j]
			}
		}
		if op == nil {
			return Inspection{}, ErrInvalid
		}
		resp := a.SyntheticData[in.OperationID]
		if resp == nil {
			resp = map[string]any{"synthetic": true, "operation": in.OperationID}
		}
		status := 200
		failure := ""
		if in.Failure != "" {
			for _, r := range a.FailureRules {
				if r.ID == in.Failure && r.OperationID == in.OperationID {
					status = r.Status
					failure = r.ID
					resp = r.Response
					if resp == nil {
						resp = map[string]any{"error": r.ErrorCode}
					}
				}
			}
			if failure == "" {
				return Inspection{}, ErrInvalid
			}
		}
		a.Used++
		x := Inspection{Sequence: int64(len(a.Inspections) + 1), OperationID: op.ID, Method: op.Method, Path: op.Path, RequestHeaders: map[string]string{"authorization": "[REDACTED]", "content-type": "application/json"}, RequestBody: in.Body, ResponseStatus: status, ResponseHeaders: map[string]string{"content-type": "application/json", "x-sandbox": "synthetic"}, ResponseBody: resp, FailureRule: failure, CreatedAt: now}
		a.Inspections = append(a.Inspections, x)
		a.UpdatedAt = now
		if e = s.save(d); e != nil {
			return Inspection{}, e
		}
		return x, nil
	}
	return Inspection{}, ErrForbidden
}
