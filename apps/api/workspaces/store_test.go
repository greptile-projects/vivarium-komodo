package workspaces

import (
	"os"
	"testing"
	"time"
)

func TestResumeRejectsChangedFoundation(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	definition := Definition{Version: 1, Setup: []string{"true"}, Resources: ResourceLimits{CPUSeconds: 1, MemoryMB: 128, DiskMB: 128, SetupTimeoutSeconds: 1}}
	item, err := store.Create("repository", "revision", "actor", SourceContext{Type: "repository"}, Access{RepositoryID: "repository", ActorID: "actor", Permission: "repository:write"}, definition, "digest-a")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Append(item.ID, Event{Type: "log", Message: "setup"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Finish(item.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if err = ensureEnvironment(store.Environment(item.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Suspend("repository", item.ID, "actor"); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Resume("repository", item.ID, "actor", "digest-b"); err != ErrInvalidTransition {
		t.Fatalf("changed foundation error = %v", err)
	}
	resumed, err := store.Resume("repository", item.ID, "actor", "digest-a")
	if err != nil || resumed.State != Ready {
		t.Fatalf("resume = %#v, %v", resumed, err)
	}
}

func TestPresenceExpiresAndControlInterventionsUseVersions(t *testing.T) {
	store, _ := New(t.TempDir())
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	item, err := store.Create("repository", "revision", "owner", SourceContext{Type: "repository"}, Access{}, Definition{}, "digest")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Finish(item.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Observe("repository", item.ID, "peer", "files", "README.md"); err != nil {
		t.Fatal(err)
	}
	item, err = store.Grant("repository", item.ID, "owner", "peer", "human", "edit", []string{"files"})
	if err != nil {
		t.Fatal(err)
	}
	grant := item.Controls[0]
	item, err = store.Intervene("repository", item.ID, "owner", grant.ID, "pause", "", grant.Version)
	if err != nil || item.Controls[0].State != "paused" {
		t.Fatalf("pause = %#v, %v", item.Controls, err)
	}
	if _, err = store.Intervene("repository", item.ID, "owner", grant.ID, "revoke", "", grant.Version); err != ErrConflict {
		t.Fatalf("stale intervention error = %v", err)
	}
	now = now.Add(46 * time.Second)
	item, err = store.Get("repository", item.ID)
	if err != nil || len(item.Presence) != 0 {
		t.Fatalf("expired presence = %#v, %v", item.Presence, err)
	}
}

func ensureEnvironment(path string) error { return os.Mkdir(path, 0o750) }

func TestVerificationUsesLatestCurrentAttemptWithoutDiscardingFailure(t *testing.T) {
	inputs := VerificationInputs{Candidate: "candidate", Source: "source", Target: "target", Dependency: "dependency", Policy: "policy"}
	candidate := VerificationCandidate{
		Inputs: inputs,
		Criteria: []VerificationCriterion{{ID: "combined", AffectedInputs: []string{"candidate", "source", "target"}}},
		Attempts: []VerificationAttempt{
			{ID: "failed", CriterionIDs: []string{"combined"}, InputRevisions: inputs, Status: "failed"},
			{ID: "corrected", CriterionIDs: []string{"combined"}, InputRevisions: inputs, Status: "passed"},
		},
	}
	refreshVerification(&candidate)
	if candidate.Status != "passed" || len(candidate.Attempts) != 2 || candidate.Attempts[0].Status != "failed" {
		t.Fatalf("corrected evidence did not retain its failed history: %#v", candidate)
	}
}
