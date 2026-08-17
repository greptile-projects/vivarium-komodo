package apiconsumers

import (
	"sort"
	"time"
)

// ContractMigration is the producer-governed, consumer-visible record that
// turns an incompatible contract publication into bounded ecosystem work.
// References point at existing work systems and grant no authority themselves.
type ContractMigration struct {
	ID              string                `json:"id"`
	RepositoryID    string                `json:"repository_id"`
	ContractID      string                `json:"contract_id"`
	FromVersions    []int64               `json:"from_versions"`
	TargetVersion   int64                 `json:"target_version"`
	Kind            string                `json:"kind"`
	Title           string                `json:"title"`
	Changes         []ClassifiedChange    `json:"changes"`
	EvolutionPlanID string                `json:"evolution_plan_id,omitempty"`
	Stages          []MigrationStage      `json:"stages"`
	CurrentStage    int                   `json:"current_stage"`
	Affected        []AffectedApplication `json:"affected_applications"`
	Version         int64                 `json:"version"`
	Status          string                `json:"status"`
	Authority       []string              `json:"authority"`
	CreatedBy       string                `json:"created_by"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
}
type ClassifiedChange struct {
	ID             string   `json:"id"`
	Classification string   `json:"classification"`
	Summary        string   `json:"summary"`
	Operations     []string `json:"operations,omitempty"`
	Schemas        []string `json:"schemas,omitempty"`
}
type MigrationStage struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	Deadline               time.Time `json:"deadline"`
	RequireAcknowledgement bool      `json:"require_acknowledgement"`
	RequireDualVersionTest bool      `json:"require_dual_version_test"`
	RequireAttestation     bool      `json:"require_attestation"`
	RequireZeroTraffic     bool      `json:"require_zero_traffic"`
}
type WorkReference struct {
	Kind         string `json:"kind"`
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id,omitempty"`
	Revision     string `json:"revision,omitempty"`
}
type ConsumerAcknowledgement struct {
	Decision  string          `json:"decision"`
	Reason    string          `json:"reason"`
	Work      []WorkReference `json:"work,omitempty"`
	ActorID   string          `json:"actor_id"`
	CreatedAt time.Time       `json:"created_at"`
}
type DualVersionTest struct {
	OldVersion        int64     `json:"old_version"`
	TargetVersion     int64     `json:"target_version"`
	CandidateRevision string    `json:"candidate_revision"`
	OldPassed         bool      `json:"old_passed"`
	TargetPassed      bool      `json:"target_passed"`
	VerificationIDs   []string  `json:"verification_ids"`
	Summary           string    `json:"summary"`
	ActorID           string    `json:"actor_id"`
	CreatedAt         time.Time `json:"created_at"`
}
type MigrationException struct {
	ID             string     `json:"id"`
	Reason         string     `json:"reason"`
	Scope          string     `json:"scope"`
	ExpiresAt      time.Time  `json:"expires_at"`
	Status         string     `json:"status"`
	RequestedBy    string     `json:"requested_by"`
	DecidedBy      string     `json:"decided_by,omitempty"`
	DecisionReason string     `json:"decision_reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
}
type MigrationAttestation struct {
	TargetVersion    int64           `json:"target_version"`
	ConsumerRevision string          `json:"consumer_revision"`
	Work             []WorkReference `json:"work,omitempty"`
	VerificationIDs  []string        `json:"verification_ids"`
	TrafficEnded     bool            `json:"traffic_ended"`
	Summary          string          `json:"summary"`
	ActorID          string          `json:"actor_id"`
	CreatedAt        time.Time       `json:"created_at"`
}
type AffectedApplication struct {
	ApplicationID    string                   `json:"application_id"`
	Name             string                   `json:"name"`
	OwnerID          string                   `json:"owner_id"`
	Contact          string                   `json:"contact"`
	ContractVersion  int64                    `json:"contract_version"`
	AccessStatus     string                   `json:"access_status"`
	Acknowledgement  *ConsumerAcknowledgement `json:"acknowledgement,omitempty"`
	Tests            []DualVersionTest        `json:"dual_version_tests,omitempty"`
	Exceptions       []MigrationException     `json:"exceptions,omitempty"`
	Attestation      *MigrationAttestation    `json:"attestation,omitempty"`
	LatestUsageCount *int64                   `json:"latest_usage_count,omitempty"`
	LatestUsageAt    *time.Time               `json:"latest_usage_at,omitempty"`
	Blockers         []string                 `json:"blockers"`
	Ready            bool                     `json:"ready"`
}
type MigrationInput struct {
	ContractID      string             `json:"contract_id"`
	FromVersions    []int64            `json:"from_versions"`
	TargetVersion   int64              `json:"target_version"`
	Kind            string             `json:"kind"`
	Title           string             `json:"title"`
	Changes         []ClassifiedChange `json:"changes"`
	EvolutionPlanID string             `json:"evolution_plan_id"`
	Stages          []MigrationStage   `json:"stages"`
}

func validWork(xs []WorkReference) bool {
	for _, x := range xs {
		if !map[string]bool{"evolution_task": true, "integration_work": true, "fork": true, "agent_session": true, "delivery_team": true, "pull_request": true}[x.Kind] || x.ID == "" {
			return false
		}
	}
	return true
}
func containsVersion(xs []int64, n int64) bool {
	for _, x := range xs {
		if x == n {
			return true
		}
	}
	return false
}
func (s *Store) CreateMigration(repo, actor string, in MigrationInput) (ContractMigration, error) {
	if actor == "" || in.ContractID == "" || len(in.FromVersions) == 0 || in.TargetVersion < 1 || !map[string]bool{"new_version": true, "deprecation": true}[in.Kind] || in.Title == "" || len(in.Changes) == 0 || len(in.Stages) == 0 {
		return ContractMigration{}, ErrInvalid
	}
	c, err := s.contracts.Get(repo, in.ContractID)
	if err != nil {
		return ContractMigration{}, ErrInvalid
	}
	if _, ok := findVersion(c, in.TargetVersion); !ok {
		return ContractMigration{}, ErrInvalid
	}
	seen := map[int64]bool{}
	for _, n := range in.FromVersions {
		if n == in.TargetVersion || seen[n] {
			return ContractMigration{}, ErrInvalid
		}
		if _, ok := findVersion(c, n); !ok {
			return ContractMigration{}, ErrInvalid
		}
		seen[n] = true
	}
	for _, x := range in.Changes {
		if x.ID == "" || x.Summary == "" || !map[string]bool{"breaking": true, "compatible": true, "behavioral": true, "unknown": true}[x.Classification] {
			return ContractMigration{}, ErrInvalid
		}
	}
	last := time.Time{}
	ids := map[string]bool{}
	for _, x := range in.Stages {
		if x.ID == "" || x.Name == "" || ids[x.ID] || x.Deadline.IsZero() || (!last.IsZero() && !x.Deadline.After(last)) {
			return ContractMigration{}, ErrInvalid
		}
		ids[x.ID] = true
		last = x.Deadline
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return ContractMigration{}, e
	}
	now := s.now().UTC()
	if !in.Stages[0].Deadline.After(now) {
		return ContractMigration{}, ErrInvalid
	}
	m := ContractMigration{ID: ident("apimigration"), RepositoryID: repo, ContractID: in.ContractID, FromVersions: in.FromVersions, TargetVersion: in.TargetVersion, Kind: in.Kind, Title: in.Title, Changes: in.Changes, EvolutionPlanID: in.EvolutionPlanID, Stages: in.Stages, CurrentStage: 0, Version: 1, Status: "active", Authority: []string{}, CreatedBy: actor, CreatedAt: now, UpdatedAt: now}
	for _, a := range d.Applications {
		if a.RepositoryID == repo && a.Registration.ContractID == in.ContractID && containsVersion(in.FromVersions, a.Registration.ContractVersion) {
			m.Affected = append(m.Affected, AffectedApplication{ApplicationID: a.ID, Name: a.Registration.Name, OwnerID: a.OwnerID, Contact: a.Registration.Contact, ContractVersion: a.Registration.ContractVersion, AccessStatus: a.Status})
		}
	}
	sort.Slice(m.Affected, func(i, j int) bool { return m.Affected[i].ApplicationID < m.Affected[j].ApplicationID })
	d.Migrations = append(d.Migrations, m)
	return s.deriveMigration(m, d.Applications, now), s.save(d)
}

func (s *Store) deriveMigration(m ContractMigration, apps []Application, now time.Time) ContractMigration {
	stage := m.Stages[m.CurrentStage]
	all := true
	for i := range m.Affected {
		a := &m.Affected[i]
		a.Blockers = []string{}
		var app *Application
		for j := range apps {
			if apps[j].ID == a.ApplicationID {
				app = &apps[j]
			}
		}
		if app != nil {
			a.OwnerID = app.OwnerID
			a.Contact = app.Registration.Contact
			a.AccessStatus = app.Status
			for j := len(app.Observations) - 1; j >= 0; j-- {
				o := app.Observations[j]
				if o.Kind == "usage" && o.UsageCount != nil {
					a.LatestUsageCount = o.UsageCount
					t := o.WindowEnd
					a.LatestUsageAt = &t
					break
				}
			}
		}
		activeException := false
		for _, x := range a.Exceptions {
			if x.Status == "approved" && now.Before(x.ExpiresAt) {
				activeException = true
			}
		}
		if stage.RequireAcknowledgement && a.Acknowledgement == nil {
			a.Blockers = append(a.Blockers, "owner_unacknowledged")
		} else if stage.RequireAcknowledgement && a.Acknowledgement.Decision != "acknowledged" {
			a.Blockers = append(a.Blockers, "consumer_changes_requested")
		}
		if stage.RequireDualVersionTest {
			pass := false
			failed := false
			for _, x := range a.Tests {
				if x.TargetVersion == m.TargetVersion && !x.CreatedAt.Before(m.CreatedAt) {
					pass = x.OldPassed && x.TargetPassed
					if !pass {
						failed = true
					}
				}
			}
			if !pass {
				if failed {
					a.Blockers = append(a.Blockers, "dual_version_test_failed")
				} else {
					a.Blockers = append(a.Blockers, "dual_version_test_missing")
				}
			}
		}
		if stage.RequireAttestation && a.Attestation == nil {
			a.Blockers = append(a.Blockers, "migration_not_attested")
		}
		if stage.RequireZeroTraffic && (a.LatestUsageCount == nil || a.LatestUsageAt == nil || a.LatestUsageAt.Before(m.CreatedAt) || *a.LatestUsageCount > 0) {
			a.Blockers = append(a.Blockers, "remaining_or_unknown_traffic")
		}
		if app == nil || map[string]bool{"revoked": true, "expired": true}[a.AccessStatus] {
			a.Blockers = append(a.Blockers, "consumer_access_revoked")
		}
		if now.After(stage.Deadline) && a.Acknowledgement == nil {
			a.Blockers = append(a.Blockers, "owner_unresponsive")
		}
		if activeException {
			if m.CurrentStage == len(m.Stages)-1 {
				a.Blockers = []string{"active_exception"}
			} else {
				a.Blockers = []string{}
			}
		}
		a.Ready = len(a.Blockers) == 0
		if !a.Ready {
			all = false
		}
	}
	if all && m.CurrentStage == len(m.Stages)-1 && m.Status != "retired" {
		m.Status = "ready_for_retirement"
	}
	return m
}
func (s *Store) GetMigration(repo, id, actor string, producer bool) (ContractMigration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return ContractMigration{}, e
	}
	for _, m := range d.Migrations {
		if m.RepositoryID == repo && m.ID == id {
			if !producer {
				filtered := []AffectedApplication{}
				for _, a := range m.Affected {
					if a.OwnerID == actor {
						filtered = append(filtered, a)
					}
				}
				if len(filtered) == 0 {
					return ContractMigration{}, ErrForbidden
				}
				m.Affected = filtered
			}
			return s.deriveMigration(m, d.Applications, s.now().UTC()), nil
		}
	}
	return ContractMigration{}, ErrNotFound
}
func (s *Store) mutateMigration(repo, id, application, actor string, producer bool, fn func(*ContractMigration, *AffectedApplication, time.Time) error) (ContractMigration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return ContractMigration{}, e
	}
	for i := range d.Migrations {
		m := &d.Migrations[i]
		if m.RepositoryID != repo || m.ID != id {
			continue
		}
		var a *AffectedApplication
		if !producer {
			for j := range m.Affected {
				if m.Affected[j].ApplicationID == application && m.Affected[j].OwnerID == actor {
					a = &m.Affected[j]
				}
			}
			if a == nil {
				return ContractMigration{}, ErrForbidden
			}
		}
		now := s.now().UTC()
		if e = fn(m, a, now); e != nil {
			return ContractMigration{}, e
		}
		m.Version++
		m.UpdatedAt = now
		if e = s.save(d); e != nil {
			return ContractMigration{}, e
		}
		out := *m
		if !producer {
			out.Affected = []AffectedApplication{*a}
		}
		return s.deriveMigration(out, d.Applications, now), nil
	}
	return ContractMigration{}, ErrNotFound
}
func (s *Store) AcknowledgeMigration(repo, id, application, actor, decision, reason string, work []WorkReference) (ContractMigration, error) {
	if !map[string]bool{"acknowledged": true, "changes_requested": true}[decision] || reason == "" || !validWork(work) {
		return ContractMigration{}, ErrInvalid
	}
	return s.mutateMigration(repo, id, application, actor, false, func(m *ContractMigration, a *AffectedApplication, now time.Time) error {
		a.Acknowledgement = &ConsumerAcknowledgement{Decision: decision, Reason: reason, Work: work, ActorID: actor, CreatedAt: now}
		return nil
	})
}
func (s *Store) RecordDualVersionTest(repo, id, application, actor string, in DualVersionTest) (ContractMigration, error) {
	if in.CandidateRevision == "" || in.Summary == "" || len(in.VerificationIDs) == 0 {
		return ContractMigration{}, ErrInvalid
	}
	return s.mutateMigration(repo, id, application, actor, false, func(m *ContractMigration, a *AffectedApplication, now time.Time) error {
		if in.TargetVersion != m.TargetVersion || !containsVersion(m.FromVersions, in.OldVersion) {
			return ErrInvalid
		}
		in.ActorID = actor
		in.CreatedAt = now
		a.Tests = append(a.Tests, in)
		return nil
	})
}
func (s *Store) RequestMigrationException(repo, id, application, actor, reason, scope string, expires time.Time) (ContractMigration, error) {
	if reason == "" || scope == "" || expires.IsZero() {
		return ContractMigration{}, ErrInvalid
	}
	return s.mutateMigration(repo, id, application, actor, false, func(m *ContractMigration, a *AffectedApplication, now time.Time) error {
		if !expires.After(now) || expires.After(m.Stages[len(m.Stages)-1].Deadline) {
			return ErrInvalid
		}
		a.Exceptions = append(a.Exceptions, MigrationException{ID: ident("exception"), Reason: reason, Scope: scope, ExpiresAt: expires, Status: "requested", RequestedBy: actor, CreatedAt: now})
		return nil
	})
}
func (s *Store) DecideMigrationException(repo, id, actor, application, exception, decision, reason string) (ContractMigration, error) {
	if !map[string]bool{"approved": true, "denied": true}[decision] || reason == "" {
		return ContractMigration{}, ErrInvalid
	}
	return s.mutateMigration(repo, id, "", actor, true, func(m *ContractMigration, _ *AffectedApplication, now time.Time) error {
		for i := range m.Affected {
			if m.Affected[i].ApplicationID == application {
				for j := range m.Affected[i].Exceptions {
					if m.Affected[i].Exceptions[j].ID == exception && m.Affected[i].Exceptions[j].Status == "requested" {
						x := &m.Affected[i].Exceptions[j]
						x.Status = decision
						x.DecidedBy = actor
						x.DecisionReason = reason
						x.DecidedAt = &now
						return nil
					}
				}
			}
		}
		return ErrNotFound
	})
}
func (s *Store) AttestMigration(repo, id, application, actor string, in MigrationAttestation) (ContractMigration, error) {
	if in.ConsumerRevision == "" || in.Summary == "" || len(in.VerificationIDs) == 0 || !validWork(in.Work) {
		return ContractMigration{}, ErrInvalid
	}
	return s.mutateMigration(repo, id, application, actor, false, func(m *ContractMigration, a *AffectedApplication, now time.Time) error {
		if in.TargetVersion != m.TargetVersion {
			return ErrInvalid
		}
		pass := false
		for _, x := range a.Tests {
			if x.TargetVersion == m.TargetVersion && x.OldPassed && x.TargetPassed {
				pass = true
			}
		}
		if !pass {
			return ErrConflict
		}
		in.ActorID = actor
		in.CreatedAt = now
		a.Attestation = &in
		return nil
	})
}
func (s *Store) AdvanceMigration(repo, id, actor string, expected int64) (ContractMigration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return ContractMigration{}, e
	}
	for i := range d.Migrations {
		m := &d.Migrations[i]
		if m.RepositoryID != repo || m.ID != id {
			continue
		}
		if m.Version != expected {
			return ContractMigration{}, ErrConflict
		}
		now := s.now().UTC()
		derived := s.deriveMigration(*m, d.Applications, now)
		for _, a := range derived.Affected {
			if !a.Ready {
				return ContractMigration{}, ErrConflict
			}
		}
		if m.CurrentStage < len(m.Stages)-1 {
			m.CurrentStage++
		} else {
			m.Status = "retired"
		}
		m.Version++
		m.UpdatedAt = now
		if e = s.save(d); e != nil {
			return ContractMigration{}, e
		}
		return s.deriveMigration(*m, d.Applications, now), nil
	}
	return ContractMigration{}, ErrNotFound
}
