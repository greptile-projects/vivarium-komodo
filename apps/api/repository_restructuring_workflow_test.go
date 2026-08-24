package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositoryrestructuring"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestRepositoryTopologyChangeToContinuingCollaboration is the black-box
// boundary for extracting an independently owned component from a monorepo.
// It uses stock Git and the public HTTP surface behind view=restructuring while
// preserving each contributor and consumer's independent authority.
func TestRepositoryTopologyChangeToContinuingCollaboration(t *testing.T) {
	requireGit(t)
	gitRoot := t.TempDir()
	gitOutput(t, gitRoot, "init", "-b", "main")
	gitOutput(t, gitRoot, "config", "user.name", "Original Maintainer")
	gitOutput(t, gitRoot, "config", "user.email", "maintainer@example.test")
	if err := os.MkdirAll(filepath.Join(gitRoot, "shared", "parser"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitRoot, "shared", "parser", "parser.go"), []byte("package parser\n\nfunc Parse() string { return \"v1\" }\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitRoot, "app.go"), []byte("package app\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, gitRoot, "add", ".")
	gitOutput(t, gitRoot, "commit", "-m", "Establish monorepo parser and application")
	baseCommit := gitOutput(t, gitRoot, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(gitRoot, "shared", "parser", "parser.go"), []byte("package parser\n\nfunc Parse() string { return \"v2\" }\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, gitRoot, "commit", "-am", "Open cross-cutting parser change")
	pullCommit := gitOutput(t, gitRoot, "rev-parse", "HEAD")
	if baseCommit == pullCommit {
		t.Fatal("stock Git did not retain the open change")
	}

	objects, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), objects)
	credentials, _ := auth.New(t.TempDir())
	store, _ := repositoryrestructuring.New(t.TempDir())
	mono, _ := repos.Create("maintainer", repositories.Metadata{Name: "studio", Visibility: repositories.Public})
	opened, _ := repos.Open(mono.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	sourceRevision, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor Original Maintainer <maintainer@example.test> 1 +0000\ncommitter Original Maintainer <maintainer@example.test> 1 +0000\n\n%s\n", tree, pullCommit)))
	actors := []string{"maintainer", "component-owner", "pull-author", "app-owner", "agent-owner", "package-owner", "docs-owner", "workflow-owner", "release-owner", "peer-owner"}
	for _, actor := range actors[1:] {
		if _, err := repos.AddCollaborator("maintainer", mono.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	tokens := map[string]string{}
	for _, actor := range actors {
		tokens[actor] = issueAccess(t, credentials, actor, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	}
	mux := http.NewServeMux()
	registerRepositoryRestructuringHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	root := "/repositories/" + string(mono.ID) + "/restructuring-plans"
	request := func(method, path, actor string, value any, want int, out any) {
		t.Helper()
		body := ""
		if value != nil {
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			body = string(encoded)
		}
		workflowJSON(t, server.URL, method, path, tokens[actor], body, want, out)
	}
	due := time.Now().UTC().Add(30 * 24 * time.Hour)
	input := repositoryrestructuring.Input{
		Title: "Extract the shared parser", Summary: "Give the parser independent ownership without losing monorepo collaboration.",
		Sources:      []repositoryrestructuring.Source{{RepositoryID: string(mono.ID), Revision: string(sourceRevision), OwnerIDs: []string{"maintainer"}, Role: "monorepo"}},
		Destinations: []repositoryrestructuring.Destination{{ID: "parser", Name: "parser", OwnerIDs: []string{"component-owner"}, Visibility: "public", DefaultBranch: "main", RetainedIdentities: []string{"package:studio-parser"}}, {ID: "app", Name: "studio-app", OwnerIDs: []string{"app-owner"}, Visibility: "public", DefaultBranch: "main"}},
		Mappings:     []repositoryrestructuring.Mapping{{ID: "parser-history", SourceRepositoryID: string(mono.ID), SourceRevision: string(sourceRevision), SourcePaths: []string{"shared/parser"}, DestinationID: "parser", DestinationPaths: []string{"."}, HistoryMode: "path_history", Disposition: "move", Rationale: "preserve component ancestry"}, {ID: "app-history", SourceRepositoryID: string(mono.ID), SourceRevision: string(sourceRevision), SourcePaths: []string{"app.go"}, DestinationID: "app", DestinationPaths: []string{"app.go"}, HistoryMode: "path_history", Disposition: "move", Rationale: "retain application history"}},
		Inventory:    []repositoryrestructuring.InventoryItem{{ID: "open-pull", Kind: "pull_request", RepositoryID: string(mono.ID), Reference: "pull:42", Revision: string(sourceRevision), OwnerIDs: []string{"pull-author"}, Access: "accessible", Disposition: "split", DestinationIDs: []string{"parser", "app"}}, {ID: "package", Kind: "package", RepositoryID: string(mono.ID), Reference: "package:studio", Revision: string(sourceRevision), OwnerIDs: []string{"package-owner"}, Access: "accessible", Disposition: "redirect", DestinationIDs: []string{"parser"}}, {ID: "federated", Kind: "federated_relationship", RepositoryID: string(mono.ID), Reference: "peer:offline", Revision: string(sourceRevision), OwnerIDs: []string{"peer-owner"}, Access: "inaccessible", Disposition: "remain", Reason: "peer is unavailable"}},
		Deadline:     due, SuccessCriteria: []string{"history and builds pass", "ordinary change ships from parser"}, RollbackLimits: repositoryrestructuring.RollbackLimits{LatestTime: due, IrreversibleAfter: "redirect retirement", MaximumDataLoss: "none", RequiredRetentions: []string{"pull discussion", "review attribution"}},
	}
	var plan repositoryrestructuring.Plan
	request(http.MethodPost, root, "maintainer", input, http.StatusCreated, &plan)

	badCandidate := repositoryrestructuring.CandidateInput{MappingIDs: []string{"parser-history", "app-history"}, Repositories: []repositoryrestructuring.CandidateRepository{{DestinationID: "parser", ObjectDigest: "sha256:collision", DefaultRef: "refs/heads/main", DefaultCommit: pullCommit, ObjectCount: 4, SizeBytes: 512, Evidence: []repositoryrestructuring.PreservationEvidence{{Kind: "history", Reference: baseCommit, Status: "preserved", Detail: "path ancestry retained"}, {Kind: "signature", Reference: pullCommit, Status: "changed", Detail: "candidate commit no longer verifies"}}}}, Issues: []repositoryrestructuring.PreservationEvidence{{Kind: "path_collision", Reference: "README.md", Status: "changed", Detail: "both boundaries supplied README.md"}}, AssemblyCost: 5, RequiredDecisions: []string{"select README owner", "repair signed lineage"}}
	request(http.MethodPost, root+"/"+plan.ID+"/candidates", "maintainer", badCandidate, http.StatusCreated, &plan)
	goodCandidate := badCandidate
	goodCandidate.Repositories = []repositoryrestructuring.CandidateRepository{{DestinationID: "parser", ObjectDigest: "sha256:parser-objects", DefaultRef: "refs/heads/main", DefaultCommit: pullCommit, ObjectCount: 3, SizeBytes: 448, Evidence: []repositoryrestructuring.PreservationEvidence{{Kind: "history", Reference: baseCommit, Source: baseCommit, Candidate: pullCommit, Status: "preserved", Detail: "path ancestry and authorship retained"}, {Kind: "signature", Reference: pullCommit, Status: "preserved", Digest: "sha256:verified-signature", Detail: "replacement tag verifies preserved tree"}}}, {DestinationID: "app", ObjectDigest: "sha256:app-objects", DefaultRef: "refs/heads/main", DefaultCommit: pullCommit, ObjectCount: 2, SizeBytes: 256, Evidence: []repositoryrestructuring.PreservationEvidence{{Kind: "history", Reference: pullCommit, Status: "preserved", Detail: "application ancestry retained"}}}}
	goodCandidate.Issues, goodCandidate.RequiredDecisions = nil, nil
	request(http.MethodPost, root+"/"+plan.ID+"/candidates", "maintainer", goodCandidate, http.StatusCreated, &plan)
	candidateID := plan.Candidates[1].ID

	checks := func(packageStatus string) []repositoryrestructuring.RehearsalCheck {
		out := []repositoryrestructuring.RehearsalCheck{}
		for _, domain := range []string{"git_clone", "git_fetch", "git_push", "build", "checks", "package_resolution", "api_resolution", "documentation", "workspaces", "consumer_journey"} {
			status := "passed"
			if domain == "package_resolution" {
				status = packageStatus
			}
			out = append(out, repositoryrestructuring.RehearsalCheck{Domain: domain, Status: status, Command: "stock-" + domain, Reference: "run:" + domain, Digest: "sha256:" + domain, Summary: domain + " against candidate", Cost: 1})
		}
		return out
	}
	request(http.MethodPost, root+"/"+plan.ID+"/rehearsals", "maintainer", repositoryrestructuring.RehearsalInput{CandidateID: candidateID, Environment: "networkless extraction rehearsal", Budget: 12, ObservedCost: 10, Checks: checks("failed"), Issues: []repositoryrestructuring.PreservationEvidence{{Kind: "package_release", Reference: "studio-parser@2.0.0-rc1", Status: "missing", Detail: "registry rejected the first release"}}}, http.StatusCreated, &plan)
	request(http.MethodPost, root+"/"+plan.ID+"/rehearsals", "maintainer", repositoryrestructuring.RehearsalInput{CandidateID: candidateID, Environment: "networkless extraction rehearsal", Budget: 12, ObservedCost: 9, Checks: checks("passed")}, http.StatusCreated, &plan)
	passingRehearsal := plan.Rehearsals[1].ID

	badWork := repositoryrestructuring.WorkMappingInput{InventoryItemID: "open-pull", SourceRevision: "obsolete-review-revision", Kind: "pull_request", Authorship: []string{"pull-author"}, Discussion: []string{"pull:42#discussion"}, Reviews: []repositoryrestructuring.WorkReview{{ActorID: "reviewer", Revision: "obsolete", Decision: "approved", Reference: "pull:42#review"}}, Dependencies: []string{"package contract"}, AcceptanceCriteria: []string{"parser and app change together"}, ContextAudience: "repository", Destinations: []repositoryrestructuring.WorkDestination{{DestinationID: "parser", Kind: "pull_request", Reference: "parser/pull:1", Revision: "parser-pr-1"}, {DestinationID: "app", Kind: "pull_request", Reference: "app/pull:1", Revision: "app-pr-1"}}}
	request(http.MethodPost, root+"/"+plan.ID+"/work-mappings", "maintainer", badWork, http.StatusCreated, &plan)
	work := badWork
	work.SourceRevision = string(sourceRevision)
	work.Reviews = []repositoryrestructuring.WorkReview{{ActorID: "reviewer", Revision: string(sourceRevision), Decision: "changes_requested", Reference: "pull:42#review-current"}}
	work.Destinations[0].ContributionID = "parser-contribution"
	work.Destinations[1].ContributionID = "app-contribution"
	work.Destinations[1].DependsOn = []string{"parser-contribution"}
	request(http.MethodPost, root+"/"+plan.ID+"/work-mappings", "maintainer", work, http.StatusCreated, &plan)
	workID, workVersion := plan.WorkMappings[1].ID, plan.WorkMappings[1].Version
	request(http.MethodPost, root+"/"+plan.ID+"/work-mappings/"+workID+"/decisions", "pull-author", map[string]any{"decision": "approved", "reason": "current review intent is retained", "expected_version": workVersion}, http.StatusCreated, &plan)
	for _, outcome := range []struct{ actor, destination, revision, reference string }{{"component-owner", "parser", "parser-change-2", "parser/pull:2"}, {"app-owner", "app", "app-change-2", "app/pull:2"}} {
		mapping := plan.WorkMappings[1]
		request(http.MethodPost, root+"/"+plan.ID+"/work-mappings/"+workID+"/outcomes", outcome.actor, map[string]any{"destination_id": outcome.destination, "status": "continued", "revision": outcome.revision, "reference": outcome.reference, "reason": "ordinary destination contribution accepted", "expected_version": mapping.Version}, http.StatusCreated, &plan)
	}

	targets := []repositoryrestructuring.MigrationTarget{}
	for _, target := range []struct{ id, kind, owner, old, next, state string }{{"human", "dependency", "app-owner", "pkg:studio/parser", "pkg:studio-parser", "unmigrated"}, {"agent", "dependency", "agent-owner", "module:studio/parser", "module:studio-parser", "unmigrated"}, {"package", "package", "package-owner", "package:studio", "package:studio-parser", "blocked"}, {"docs", "documentation", "docs-owner", "docs:studio/parser", "docs:parser", "planned"}, {"workflow", "workflow", "workflow-owner", "workflow:monorepo-release", "workflow:parser-release", "planned"}, {"release", "deployment", "release-owner", "release:studio", "release:parser", "planned"}, {"peer", "federated_follower", "peer-owner", "fed:studio", "fed:parser", "unavailable"}} {
		targets = append(targets, repositoryrestructuring.MigrationTarget{ID: target.id, Kind: target.kind, OwnerIDs: []string{target.owner}, Audience: "public", CurrentLocation: target.old, ReplacementLocation: target.next, Mappings: map[string]string{target.old: target.next}, Synchronization: []string{"update through ordinary owner-controlled contribution"}, CompatibilityUntil: due, State: target.state, NextAction: "owner verifies new entry point"})
	}
	request(http.MethodPost, root+"/"+plan.ID+"/migration-plans", "maintainer", repositoryrestructuring.MigrationPlanInput{CandidateID: candidateID, Revision: string(sourceRevision), Targets: targets}, http.StatusCreated, &plan)
	migrationID := plan.MigrationPlans[0].ID
	request(http.MethodPost, root+"/"+plan.ID+"/migration-plans/"+migrationID+"/events", "maintainer", map[string]any{"target_id": "agent", "state": "adopted", "revision": "agent-consumer-2", "pull_request_reference": "agent/pull:7", "next_action": "use new package"}, http.StatusForbidden, nil)
	for _, target := range targets {
		actor := target.OwnerIDs[0]
		request(http.MethodPost, root+"/"+plan.ID+"/migration-plans/"+migrationID+"/events", actor, map[string]any{"target_id": target.ID, "state": "adopted", "revision": target.ID + "-revision-2", "pull_request_reference": target.ID + "/pull:2", "release_reference": target.ID + ":v2", "evidence": map[string]string{"verification": "sha256:" + target.ID}, "next_action": "verified on new authority"}, http.StatusCreated, &plan)
	}

	stages := []repositoryrestructuring.CutoverStage{}
	previous := ""
	for _, kind := range []string{"pause_writes", "activate_destinations", "transfer_ownership_policies", "publish_refs_redirects", "verify_topology", "retire_sources"} {
		stage := repositoryrestructuring.CutoverStage{ID: kind, Kind: kind, OwnerIDs: []string{"maintainer"}}
		if previous != "" {
			stage.DependsOn = []string{previous}
		}
		if kind == "publish_refs_redirects" {
			stage.AtomicGroup = "git-package-docs-workflow-release"
		}
		stages = append(stages, stage)
		previous = kind
	}
	cutoverInput := repositoryrestructuring.CutoverInput{CandidateID: candidateID, RehearsalID: passingRehearsal, MigrationPlanID: migrationID, SourceRevisions: map[string]string{string(mono.ID): string(sourceRevision)}, RequiredOwnerIDs: []string{"maintainer", "component-owner", "app-owner"}, WriteBoundary: "fence monorepo pushes while authoritative refs publish", Stages: stages, SourceDisposition: "archived"}
	request(http.MethodPost, root+"/"+plan.ID+"/cutovers", "maintainer", cutoverInput, http.StatusCreated, &plan)
	approveAndStart := func(index int) {
		for _, actor := range []string{"maintainer", "component-owner", "app-owner"} {
			cut := plan.Cutovers[index]
			request(http.MethodPost, root+"/"+plan.ID+"/cutovers/"+cut.ID+"/approvals", actor, map[string]any{"decision": "approved", "reason": "owned boundary is ready", "expected_version": cut.Version}, http.StatusCreated, &plan)
		}
		cut := plan.Cutovers[index]
		request(http.MethodPost, root+"/"+plan.ID+"/cutovers/"+cut.ID+"/controls", "maintainer", map[string]any{"kind": "start", "reason": "all owners approved", "expected_version": cut.Version}, http.StatusCreated, &plan)
	}
	approveAndStart(0)
	first := plan.Cutovers[0]
	request(http.MethodPost, root+"/"+plan.ID+"/cutovers/"+first.ID+"/signals", "maintainer", map[string]any{"kind": "permissions", "resource_id": "parser", "status": "failed", "summary": "package publisher mapping omitted release owner", "expected_version": first.Version}, http.StatusCreated, &plan)
	first = plan.Cutovers[0]
	request(http.MethodPost, root+"/"+plan.ID+"/cutovers/"+first.ID+"/controls", "maintainer", map[string]any{"kind": "rollback", "reason": "restore source entry points while permissions are corrected", "expected_version": first.Version}, http.StatusCreated, &plan)
	if plan.Cutovers[0].State != "rolled_back" {
		t.Fatal("permission mismatch did not retain rollback")
	}

	request(http.MethodPost, root+"/"+plan.ID+"/cutovers", "maintainer", cutoverInput, http.StatusCreated, &plan)
	approveAndStart(1)
	cut := plan.Cutovers[1]
	request(http.MethodPost, root+"/"+plan.ID+"/cutovers/"+cut.ID+"/signals", "maintainer", map[string]any{"kind": "late_write", "resource_id": string(mono.ID), "status": "failed", "value": 1, "summary": "concurrent stock Git push reached old main", "expected_version": cut.Version}, http.StatusCreated, &plan)
	cut = plan.Cutovers[1]
	request(http.MethodPost, root+"/"+plan.ID+"/cutovers/"+cut.ID+"/signals", "maintainer", map[string]any{"kind": "late_write", "resource_id": string(mono.ID), "status": "passed", "value": 0, "summary": "push incorporated and fence restored", "expected_version": cut.Version}, http.StatusCreated, &plan)
	cut = plan.Cutovers[1]
	request(http.MethodPost, root+"/"+plan.ID+"/cutovers/"+cut.ID+"/controls", "maintainer", map[string]any{"kind": "resume", "reason": "concurrent push safely incorporated", "expected_version": cut.Version}, http.StatusCreated, &plan)
	for _, kind := range []string{"build", "release", "permissions", "links", "supported_consumers", "ordinary_contribution"} {
		cut = plan.Cutovers[1]
		request(http.MethodPost, root+"/"+plan.ID+"/cutovers/"+cut.ID+"/signals", "maintainer", map[string]any{"kind": kind, "resource_id": "parser", "status": "passed", "revision": pullCommit, "summary": kind + " verified after new-structure change", "expected_version": cut.Version}, http.StatusCreated, &plan)
	}
	for _, stage := range stages {
		cut = plan.Cutovers[1]
		request(http.MethodPost, root+"/"+plan.ID+"/cutovers/"+cut.ID+"/stages/"+stage.ID, "maintainer", map[string]any{"state": "active", "summary": "starting " + stage.Kind, "expected_version": cut.Version}, http.StatusCreated, &plan)
		cut = plan.Cutovers[1]
		request(http.MethodPost, root+"/"+plan.ID+"/cutovers/"+cut.ID+"/stages/"+stage.ID, "maintainer", map[string]any{"state": "succeeded", "summary": "completed " + stage.Kind, "evidence": []string{"receipt:" + stage.Kind}, "expected_version": cut.Version}, http.StatusCreated, &plan)
	}
	completed := plan.Cutovers[1]
	if completed.State != "completed" || len(completed.AuthorityGranted) != 0 || len(plan.Candidates) != 2 || plan.Rehearsals[0].Status != "blocked" || plan.WorkMappings[0].Status != "blocked" || len(plan.MigrationPlans[0].Events) != len(targets) {
		t.Fatalf("restructuring trail incomplete: state=%s blockers=%#v", completed.State, completed.Blockers)
	}
}
