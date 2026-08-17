package apiconsumers

import (
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/apicontracts"
)

func migrationFixture(t *testing.T) (*Store, Application, string) {
	t.Helper()
	contracts, err := apicontracts.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	input := apicontracts.Input{Name: "Orders", Version: "1.0", SourceRevision: "old", DefinitionPath: "openapi.json", DefinitionFormat: "openapi", DefinitionValid: true, Operations: []apicontracts.Operation{{ID: "list", Method: "GET", Path: "/orders", Authentication: []string{"oauth"}, ResponseSchema: "Order"}}, Schemas: []apicontracts.Schema{{Name: "Order", Kind: "object"}}, Authentication: []apicontracts.Authentication{{ID: "oauth", Kind: "oauth2", Description: "scoped", Scopes: []string{"read"}}}, Environments: []apicontracts.Environment{{Name: "production", BaseURL: "https://api.test", Availability: "available"}}, OwnerIDs: []string{"producer"}, Stability: "stable", SupportPolicy: "12 months", Compatibility: apicontracts.Compatibility{Promise: "semver"}, Links: []apicontracts.Link{{Kind: "source", ResourceID: "old", Label: "source", Status: "current"}}, ChangeReason: "publish"}
	c, err := contracts.Create("repo", "producer", input)
	if err != nil {
		t.Fatal(err)
	}
	input.Version = "2.0"
	input.SourceRevision = "new"
	input.Compatibility = apicontracts.Compatibility{Promise: "semver", PreviousVersion: "1.0", BreakingChanges: []string{"removed legacy field"}, Migration: "dual read"}
	input.ChangeReason = "remove legacy field"
	c, err = contracts.Revise("repo", c.ID, "producer", 1, input)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(t.TempDir(), contracts)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Register("repo", "consumer", RegistrationInput{Name: "checkout", ConsumerProject: "shop/checkout", Contact: "checkout@example.test", ContractID: c.ID, ContractVersion: 1, Environments: []string{"production"}, Capabilities: []string{"read"}, CredentialLifetimeHours: 24})
	if err != nil {
		t.Fatal(err)
	}
	approved, err := s.Decide("repo", a.ID, "producer", ApprovalInput{ExpectedVersion: a.Version, Decision: "approved", Capabilities: []string{"read"}, Environments: []string{"production"}, Quota: 10, CredentialLifetimeHours: 12, Reason: "approved for migration fixture"})
	if err != nil {
		t.Fatal(err)
	}
	a = approved.Application
	return s, a, c.ID
}

func TestMigrationRequiresVisibleTestedPathAndZeroTrafficBeforeRetirement(t *testing.T) {
	s, a, cid := migrationFixture(t)
	now := time.Now().UTC()
	m, err := s.CreateMigration("repo", "producer", MigrationInput{ContractID: cid, FromVersions: []int64{1}, TargetVersion: 2, Kind: "deprecation", Title: "Orders v1 retirement", EvolutionPlanID: "evolution-1", Changes: []ClassifiedChange{{ID: "field", Classification: "breaking", Summary: "legacy field removed", Operations: []string{"list"}}}, Stages: []MigrationStage{{ID: "prepare", Name: "Consumer preparation", Deadline: now.Add(24 * time.Hour), RequireAcknowledgement: true, RequireDualVersionTest: true}, {ID: "sunset", Name: "Traffic sunset", Deadline: now.Add(48 * time.Hour), RequireAttestation: true, RequireZeroTraffic: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Affected) != 1 || m.Affected[0].OwnerID != "consumer" || m.Affected[0].Ready {
		t.Fatalf("affected discovery/blockers = %+v", m.Affected)
	}
	if _, err = s.AdvanceMigration("repo", m.ID, "producer", m.Version); err != ErrConflict {
		t.Fatalf("unacknowledged migration advanced: %v", err)
	}
	m, err = s.AcknowledgeMigration("repo", m.ID, m.Affected[0].ApplicationID, "consumer", "acknowledged", "fork and agent work accepted", []WorkReference{{Kind: "evolution_task", ID: "task-1"}, {Kind: "fork", ID: "fork-1"}, {Kind: "agent_session", ID: "agent-1"}, {Kind: "delivery_team", ID: "team-1"}})
	if err != nil {
		t.Fatal(err)
	}
	m, err = s.RecordDualVersionTest("repo", m.ID, m.Affected[0].ApplicationID, "consumer", DualVersionTest{OldVersion: 1, TargetVersion: 2, CandidateRevision: "consumer-candidate", OldPassed: true, TargetPassed: false, VerificationIDs: []string{"verification-old", "verification-new"}, Summary: "new decoder failed"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Affected[0].Ready || m.Affected[0].Blockers[0] != "dual_version_test_failed" {
		t.Fatalf("failed migration test did not block: %+v", m.Affected[0])
	}
	m, err = s.RecordDualVersionTest("repo", m.ID, m.Affected[0].ApplicationID, "consumer", DualVersionTest{OldVersion: 1, TargetVersion: 2, CandidateRevision: "consumer-fixed", OldPassed: true, TargetPassed: true, VerificationIDs: []string{"verification-old", "verification-new-fixed"}, Summary: "both candidates pass"})
	if err != nil {
		t.Fatal(err)
	}
	m, err = s.AdvanceMigration("repo", m.ID, "producer", m.Version)
	if err != nil {
		t.Fatal(err)
	}
	zero := int64(0)
	usageEnd := time.Now().UTC()
	_, err = s.RecordObservation("repo", a.ID, "consumer", false, ObservationInput{Kind: "usage", Audience: "shared", ReleaseID: "checkout-v2", Environment: "production", WindowStart: usageEnd.Add(-time.Hour), WindowEnd: usageEnd, UsageCount: &zero, Summary: "v1 traffic drained"})
	if err != nil {
		t.Fatal(err)
	}
	m, err = s.AttestMigration("repo", m.ID, m.Affected[0].ApplicationID, "consumer", MigrationAttestation{TargetVersion: 2, ConsumerRevision: "consumer-fixed", Work: []WorkReference{{Kind: "pull_request", ID: "pull-7"}}, VerificationIDs: []string{"verification-new-fixed"}, TrafficEnded: true, Summary: "production client migrated"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != "ready_for_retirement" || !m.Affected[0].Ready {
		t.Fatalf("retirement not governed by evidence: %+v", m)
	}
	m, err = s.AdvanceMigration("repo", m.ID, "producer", m.Version)
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != "retired" {
		t.Fatalf("status = %s", m.Status)
	}
}

func TestMigrationExceptionIsBoundedAndConsumerProjectionIsPrivate(t *testing.T) {
	s, _, cid := migrationFixture(t)
	now := time.Now().UTC()
	m, err := s.CreateMigration("repo", "producer", MigrationInput{ContractID: cid, FromVersions: []int64{1}, TargetVersion: 2, Kind: "new_version", Title: "v2 rollout", Changes: []ClassifiedChange{{ID: "behavior", Classification: "behavioral", Summary: "pagination changes"}}, Stages: []MigrationStage{{ID: "prepare", Name: "prepare", Deadline: now.Add(time.Hour), RequireAcknowledgement: true}, {ID: "retire", Name: "retire", Deadline: now.Add(2 * time.Hour), RequireZeroTraffic: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.RequestMigrationException("repo", m.ID, m.Affected[0].ApplicationID, "consumer", "vendor release pending", "list operation", now.Add(3*time.Hour)); err != ErrInvalid {
		t.Fatalf("unbounded exception accepted: %v", err)
	}
	m, err = s.RequestMigrationException("repo", m.ID, m.Affected[0].ApplicationID, "consumer", "vendor release pending", "list operation", now.Add(90*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	x := m.Affected[0].Exceptions[0]
	m, err = s.DecideMigrationException("repo", m.ID, "producer", m.Affected[0].ApplicationID, x.ID, "approved", "bounded compatibility window")
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.GetMigration("repo", m.ID, "consumer", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Affected) != 1 || view.Affected[0].Contact != "checkout@example.test" {
		t.Fatalf("consumer projection = %+v", view.Affected)
	}
	if _, err = s.GetMigration("repo", m.ID, "stranger", false); err != ErrForbidden {
		t.Fatalf("unaffected reader saw migration: %v", err)
	}
	consumer, err := s.ListMigrations("repo", "consumer", false)
	if err != nil || len(consumer) != 1 || len(consumer[0].Affected) != 1 {
		t.Fatalf("consumer migration list = %+v, %v", consumer, err)
	}
	stranger, err := s.ListMigrations("repo", "stranger", false)
	if err != nil || len(stranger) != 0 {
		t.Fatalf("unaffected reader migration list = %+v, %v", stranger, err)
	}
}
