package repositoryrestructuring

import (
	"errors"
	"testing"
	"time"
)

func TestOwnerGatedCutoverPausesLateWritesAndRequiresTopologyProof(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	input := Input{Title: "extract parser", Summary: "new authority", Sources: []Source{{RepositoryID: "mono", Revision: "source-1", OwnerIDs: []string{"source-owner"}}}, Destinations: []Destination{{ID: "parser", Name: "parser", OwnerIDs: []string{"dest-owner"}, Visibility: "public", DefaultBranch: "main"}}, Mappings: []Mapping{{ID: "code", SourceRepositoryID: "mono", SourceRevision: "source-1", SourcePaths: []string{"parser"}, DestinationID: "parser", DestinationPaths: []string{"."}, HistoryMode: "path_history", Disposition: "move", Rationale: "boundary"}}, Inventory: []InventoryItem{{ID: "main", Kind: "ref", RepositoryID: "mono", Reference: "refs/heads/main", Revision: "source-1", OwnerIDs: []string{"source-owner"}, Access: "accessible", Disposition: "move", DestinationIDs: []string{"parser"}}}, Deadline: now.Add(time.Hour), SuccessCriteria: []string{"ordinary contribution works"}, RollbackLimits: RollbackLimits{LatestTime: now.Add(2 * time.Hour), IrreversibleAfter: "archive", MaximumDataLoss: "none", RequiredRetentions: []string{"refs"}}}
	p, err := s.Create("mono", "source-owner", input)
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.AddCandidate("mono", p.ID, "source-owner", CandidateInput{MappingIDs: []string{"code"}, Repositories: []CandidateRepository{{DestinationID: "parser", ObjectDigest: "sha256:objects", DefaultRef: "refs/heads/main", DefaultCommit: "source-1", ObjectCount: 1, SizeBytes: 1, Evidence: []PreservationEvidence{{Kind: "history", Reference: "source-1", Status: "preserved", Detail: "reachable"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	candidate := p.Candidates[0]
	checks := []RehearsalCheck{}
	for domain := range rehearsalDomains {
		checks = append(checks, RehearsalCheck{Domain: domain, Status: "passed", Command: "check", Reference: "run:" + domain, Summary: "passed"})
	}
	p, err = s.AddRehearsal("mono", p.ID, "source-owner", RehearsalInput{CandidateID: candidate.ID, Environment: "isolated", Budget: 10, ObservedCost: 1, Checks: checks})
	if err != nil {
		t.Fatal(err)
	}
	rehearsal := p.Rehearsals[0]
	p, err = s.AddMigrationPlan("mono", p.ID, "source-owner", MigrationPlanInput{CandidateID: candidate.ID, Revision: "source-1", Targets: []MigrationTarget{{ID: "clone", Kind: "clone", OwnerIDs: []string{"consumer"}, Audience: "public", CurrentLocation: "mono", ReplacementLocation: "parser", ReplacementRemote: "https://example/parser", Mappings: map[string]string{"mono": "parser"}, Synchronization: []string{"git remote set-url origin"}, CompatibilityUntil: now.Add(time.Hour), State: "adopted", NextAction: "use parser"}}})
	if err != nil {
		t.Fatal(err)
	}
	migration := p.MigrationPlans[0]
	kinds := []string{"pause_writes", "activate_destinations", "transfer_ownership_policies", "publish_refs_redirects", "verify_topology", "retire_sources"}
	stages := []CutoverStage{}
	for i, k := range kinds {
		depends := []string{}
		if i > 0 {
			depends = []string{kinds[i-1]}
		}
		stages = append(stages, CutoverStage{ID: k, Kind: k, OwnerIDs: []string{"source-owner"}, DependsOn: depends})
	}
	stages[3].AtomicGroup = "authoritative-refs-and-redirects"
	p, err = s.AddCutover("mono", p.ID, "source-owner", CutoverInput{CandidateID: candidate.ID, RehearsalID: rehearsal.ID, MigrationPlanID: migration.ID, SourceRevisions: map[string]string{"mono": "source-1"}, RequiredOwnerIDs: []string{"source-owner", "dest-owner"}, WriteBoundary: "pause source and destination writes while refs publish", Stages: stages, SourceDisposition: "archived"})
	if err != nil {
		t.Fatal(err)
	}
	c := p.Cutovers[0]
	if _, err = s.ControlCutover("mono", p.ID, c.ID, "source-owner", "start", "begin", c.Version); !errors.Is(err, ErrInvalid) {
		t.Fatalf("started without approvals: %v", err)
	}
	p, _ = s.DecideCutover("mono", p.ID, c.ID, "source-owner", "approved", "source fenced", c.Version)
	c = p.Cutovers[0]
	p, _ = s.DecideCutover("mono", p.ID, c.ID, "dest-owner", "approved", "destination ready", c.Version)
	c = p.Cutovers[0]
	p, err = s.ControlCutover("mono", p.ID, c.ID, "source-owner", "start", "owners approved", c.Version)
	if err != nil {
		t.Fatal(err)
	}
	c = p.Cutovers[0]
	p, err = s.RecordCutoverSignal("mono", p.ID, c.ID, "source-owner", CutoverSignal{Kind: "late_write", ResourceID: "mono", Status: "failed", Value: 1, Summary: "one push reached the old authority"}, c.Version)
	if err != nil {
		t.Fatal(err)
	}
	c = p.Cutovers[0]
	if c.State != "paused" {
		t.Fatalf("late write did not pause cutover: %+v", c)
	}
	p, _ = s.RecordCutoverSignal("mono", p.ID, c.ID, "source-owner", CutoverSignal{Kind: "late_write", ResourceID: "mono", Status: "passed", Value: 0, Summary: "late push incorporated and fence restored"}, c.Version)
	c = p.Cutovers[0]
	p, err = s.ControlCutover("mono", p.ID, c.ID, "source-owner", "resume", "fence is current", c.Version)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range verificationSignalKinds {
		c = p.Cutovers[0]
		p, err = s.RecordCutoverSignal("mono", p.ID, c.ID, "source-owner", CutoverSignal{Kind: kind, ResourceID: "parser", Status: "passed", Summary: "verified on new topology"}, c.Version)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, stage := range kinds {
		c = p.Cutovers[0]
		p, err = s.UpdateCutoverStage("mono", p.ID, c.ID, stage, "source-owner", "active", "started "+stage, nil, c.Version)
		if err != nil {
			t.Fatalf("start %s: %v blockers=%+v", stage, err, c.Blockers)
		}
		c = p.Cutovers[0]
		p, err = s.UpdateCutoverStage("mono", p.ID, c.ID, stage, "source-owner", "succeeded", "finished "+stage, []string{"receipt:" + stage}, c.Version)
		if err != nil {
			t.Fatalf("finish %s: %v", stage, err)
		}
	}
	if got := p.Cutovers[0]; got.State != "completed" || len(got.AuthorityGranted) != 0 {
		t.Fatalf("unexpected completion: %+v", got)
	}
}
