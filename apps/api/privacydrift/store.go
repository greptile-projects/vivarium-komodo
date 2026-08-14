// Package privacydrift retains sanitized production privacy drift and remediation evidence.
package privacydrift

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrInvalid = errors.New("invalid privacy drift record")
var ErrNotFound = errors.New("privacy drift record not found")

type MonitorInput struct {
	Name              string   `json:"name"`
	CommitmentID      string   `json:"commitment_id"`
	CommitmentVersion int64    `json:"commitment_version"`
	DataUseIDs        []string `json:"data_use_ids"`
	SignalKinds       []string `json:"signal_kinds"`
	ReleaseID         string   `json:"release_id"`
	ReleaseRevision   string   `json:"release_revision"`
	EnvironmentID     string   `json:"environment_id"`
	ExtensionID       string   `json:"extension_id,omitempty"`
	OwnerIDs          []string `json:"owner_ids"`
	ParticipantIDs    []string `json:"participant_ids"`
	RetentionDays     int      `json:"retention_days"`
}
type Monitor struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	MonitorInput
	CreatedByID string    `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type Evidence struct {
	SignalReference string    `json:"signal_reference"`
	Metric          string    `json:"metric"`
	AggregateCount  int64     `json:"aggregate_count"`
	WindowStart     time.Time `json:"window_start"`
	WindowEnd       time.Time `json:"window_end"`
	Digest          string    `json:"digest"`
	Summary         string    `json:"summary"`
	Sanitized       bool      `json:"sanitized"`
}
type SignalInput struct {
	MonitorID string   `json:"monitor_id"`
	Kind      string   `json:"kind"`
	DataUseID string   `json:"data_use_id"`
	Observed  string   `json:"observed"`
	Expected  string   `json:"expected"`
	Evidence  Evidence `json:"evidence"`
}
type Event struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	ActorID      string    `json:"actor_id"`
	Summary      string    `json:"summary"`
	TargetIDs    []string  `json:"target_ids,omitempty"`
	ResourceKind string    `json:"resource_kind,omitempty"`
	ResourceID   string    `json:"resource_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
type Repair struct {
	ID                 string    `json:"id"`
	OwnerKind          string    `json:"owner_kind"`
	OwnerID            string    `json:"owner_id"`
	ProposalID         string    `json:"proposal_id"`
	TaskID             string    `json:"task_id"`
	BaseRevision       string    `json:"base_revision"`
	AcceptanceCriteria []string  `json:"acceptance_criteria"`
	CreatedByID        string    `json:"created_by_id"`
	CreatedAt          time.Time `json:"created_at"`
}
type Signal struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	SignalInput
	ReleaseID        string    `json:"release_id"`
	ReleaseRevision  string    `json:"release_revision"`
	EnvironmentID    string    `json:"environment_id"`
	ExtensionID      string    `json:"extension_id,omitempty"`
	OwnerIDs         []string  `json:"owner_ids"`
	State            string    `json:"state"`
	ReportedByID     string    `json:"reported_by_id"`
	CreatedAt        time.Time `json:"created_at"`
	Events           []Event   `json:"events"`
	Repair           *Repair   `json:"repair,omitempty"`
	AuthorityGranted bool      `json:"authority_granted"`
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
func newid() string                      { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo string) string { return filepath.Join(s.root, repo+".json") }
func (s *Store) read(repo string) ([]Monitor, []Signal, error) {
	b, e := os.ReadFile(s.path(repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Monitor{}, []Signal{}, nil
	}
	if e != nil {
		return nil, nil, e
	}
	var x struct {
		Monitors []Monitor `json:"monitors"`
		Signals  []Signal  `json:"signals"`
	}
	if json.Unmarshal(b, &x) != nil {
		return nil, nil, ErrInvalid
	}
	return x.Monitors, x.Signals, nil
}
func (s *Store) write(repo string, m []Monitor, d []Signal) error {
	b, e := json.MarshalIndent(struct {
		Monitors []Monitor `json:"monitors"`
		Signals  []Signal  `json:"signals"`
	}{m, d}, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(repo), b, 0640)
	}
	return e
}
func list(v []string, required bool) bool {
	if (required && len(v) == 0) || len(v) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range v {
		x = strings.TrimSpace(x)
		if x == "" || len(x) > 300 || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}

var kinds = map[string]bool{"undeclared_flow": true, "excessive_retention": true, "failed_deletion": true, "consent_mismatch": true, "unexpected_recipient": true}

func (s *Store) CreateMonitor(repo, actor string, in MonitorInput) (Monitor, error) {
	if repo == "" || actor == "" || strings.TrimSpace(in.Name) == "" || in.CommitmentID == "" || in.CommitmentVersion < 1 || !list(in.DataUseIDs, true) || !list(in.SignalKinds, true) || !list(in.OwnerIDs, true) || !list(in.ParticipantIDs, false) || in.ReleaseID == "" || in.ReleaseRevision == "" || in.EnvironmentID == "" || in.RetentionDays < 1 || in.RetentionDays > 365 {
		return Monitor{}, ErrInvalid
	}
	for _, k := range in.SignalKinds {
		if !kinds[k] {
			return Monitor{}, ErrInvalid
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, d, e := s.read(repo)
	if e != nil {
		return Monitor{}, e
	}
	x := Monitor{ID: newid(), RepositoryID: repo, MonitorInput: in, CreatedByID: actor, CreatedAt: s.now().UTC()}
	m = append(m, x)
	return x, s.write(repo, m, d)
}
func monitor(ms []Monitor, id string) (Monitor, bool) {
	for _, m := range ms {
		if m.ID == id {
			return m, true
		}
	}
	return Monitor{}, false
}
func contains(v []string, x string) bool {
	for _, y := range v {
		if y == x {
			return true
		}
	}
	return false
}
func (s *Store) Report(repo, actor string, in SignalInput) (Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, d, e := s.read(repo)
	if e != nil {
		return Signal{}, e
	}
	mo, ok := monitor(m, in.MonitorID)
	ev := in.Evidence
	if !ok || !contains(mo.SignalKinds, in.Kind) || !contains(mo.DataUseIDs, in.DataUseID) || strings.TrimSpace(in.Observed) == "" || strings.TrimSpace(in.Expected) == "" || !ev.Sanitized || ev.SignalReference == "" || ev.Metric == "" || ev.Digest == "" || ev.Summary == "" || ev.AggregateCount < 0 || ev.WindowStart.IsZero() || !ev.WindowEnd.After(ev.WindowStart) || len(ev.Summary) > 2000 {
		return Signal{}, ErrInvalid
	}
	x := Signal{ID: newid(), RepositoryID: repo, SignalInput: in, ReleaseID: mo.ReleaseID, ReleaseRevision: mo.ReleaseRevision, EnvironmentID: mo.EnvironmentID, ExtensionID: mo.ExtensionID, OwnerIDs: append([]string{}, mo.OwnerIDs...), State: "detected", ReportedByID: actor, CreatedAt: s.now().UTC(), Events: []Event{}, AuthorityGranted: false}
	d = append(d, x)
	return x, s.write(repo, m, d)
}
func (s *Store) List(repo string) ([]Monitor, []Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo)
}
func (s *Store) AddEvent(repo, signal, actor, kind, summary, resourceKind, resourceID string, targets []string) (Signal, Event, error) {
	allowed := map[string]bool{"contain": true, "notify": true, "private_incident": true, "exception_requested": true, "exception_approved": true, "exception_rejected": true, "resolved": true}
	if !allowed[kind] || actor == "" || strings.TrimSpace(summary) == "" || len(summary) > 4000 || !list(targets, false) {
		return Signal{}, Event{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, d, e := s.read(repo)
	if e != nil {
		return Signal{}, Event{}, e
	}
	for i := range d {
		if d[i].ID != signal {
			continue
		}
		if kind == "notify" {
			mo, _ := monitor(m, d[i].MonitorID)
			for _, t := range targets {
				if !contains(mo.OwnerIDs, t) && !contains(mo.ParticipantIDs, t) {
					return Signal{}, Event{}, ErrInvalid
				}
			}
		}
		if (kind == "private_incident" || strings.HasPrefix(kind, "exception_")) && (resourceKind == "" || resourceID == "") {
			return Signal{}, Event{}, ErrInvalid
		}
		x := Event{ID: newid(), Kind: kind, ActorID: actor, Summary: summary, TargetIDs: targets, ResourceKind: resourceKind, ResourceID: resourceID, CreatedAt: s.now().UTC()}
		d[i].Events = append(d[i].Events, x)
		if kind == "contain" {
			d[i].State = "contained"
		}
		if kind == "resolved" {
			d[i].State = "resolved"
		}
		return d[i], x, s.write(repo, m, d)
	}
	return Signal{}, Event{}, ErrNotFound
}
func (s *Store) LinkRepair(repo, signal, actor string, r Repair) (Signal, error) {
	if r.OwnerID == "" || !map[string]bool{"human": true, "agent": true}[r.OwnerKind] || r.ProposalID == "" || r.TaskID == "" || r.BaseRevision == "" || !list(r.AcceptanceCriteria, true) {
		return Signal{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, d, e := s.read(repo)
	if e != nil {
		return Signal{}, e
	}
	for i := range d {
		if d[i].ID == signal {
			if d[i].Repair != nil {
				return Signal{}, ErrInvalid
			}
			r.ID = newid()
			r.CreatedByID = actor
			r.CreatedAt = s.now().UTC()
			d[i].Repair = &r
			d[i].State = "repairing"
			return d[i], s.write(repo, m, d)
		}
	}
	return Signal{}, ErrNotFound
}
