package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositoryrestructuring"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestRepositoryRestructuringPlanPublicBoundary(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := repositoryrestructuring.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "monolith", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repo.ID, "collaborator")
	opened, _ := repos.Open(repo.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor A <a@x> 1 +0000\ncommitter A <a@x> 1 +0000\n\nshape\n", tree)))
	writer := issueAccess(t, credentials, "collaborator", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader-agent", auth.API, auth.RepositoryRead)
	mux := http.NewServeMux()
	registerRepositoryRestructuringHTTP(mux, plans, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/restructuring-plans"
	due := time.Now().UTC().Add(30 * 24 * time.Hour)
	in := repositoryrestructuring.Input{Title: "Extract parser", Summary: "Make the shared parser independently authoritative.", Sources: []repositoryrestructuring.Source{{RepositoryID: string(repo.ID), Revision: string(commit), OwnerIDs: []string{"owner"}, Role: "primary"}}, Destinations: []repositoryrestructuring.Destination{{ID: "parser", Name: "parser", OwnerIDs: []string{"parser-team"}, Visibility: "public", DefaultBranch: "main", RetainedIdentities: []string{"package:parser"}}, {ID: "app", Name: "app", OwnerIDs: []string{"app-team"}, Visibility: "private", DefaultBranch: "trunk"}}, Mappings: []repositoryrestructuring.Mapping{{ID: "parser-code", SourceRepositoryID: string(repo.ID), SourceRevision: string(commit), SourcePaths: []string{"pkg/parser"}, DestinationID: "parser", DestinationPaths: []string{"."}, HistoryMode: "path_history", Disposition: "move", Rationale: "preserve parser changes"}}, Inventory: []repositoryrestructuring.InventoryItem{{ID: "main", Kind: "ref", RepositoryID: string(repo.ID), Reference: "refs/heads/main", Revision: string(commit), OwnerIDs: []string{"owner"}, Access: "accessible", Disposition: "split", DestinationIDs: []string{"parser", "app"}}, {ID: "issue-7", Kind: "issue", RepositoryID: string(repo.ID), Reference: "issue:7", Revision: string(commit), OwnerIDs: []string{"collaborator"}, Access: "accessible", Disposition: "split", DestinationIDs: []string{"parser", "app"}}, {ID: "pull-8", Kind: "pull_request", RepositoryID: string(repo.ID), Reference: "pull:8", Revision: string(commit), OwnerIDs: []string{"contributor"}, Access: "ambiguous", Disposition: "unresolved", Reason: "touches both boundaries"}, {ID: "peer", Kind: "federated_relationship", RepositoryID: string(repo.ID), Reference: "peer:offline", Revision: string(commit), OwnerIDs: []string{"peer-owner"}, Access: "inaccessible", Disposition: "remain", Reason: "peer unavailable"}}, Deadline: due, SuccessCriteria: []string{"both projects clone and build"}, RollbackLimits: repositoryrestructuring.RollbackLimits{LatestTime: due.Add(24 * time.Hour), IrreversibleAfter: "source archive", MaximumDataLoss: "none", RequiredRetentions: []string{"pull discussions"}}}
	b, _ := json.Marshal(in)
	var plan repositoryrestructuring.Plan
	workflowJSON(t, server.URL, http.MethodPost, base, writer, string(b), 201, &plan)
	if len(plan.Blockers) != 3 || len(plan.AuthorityGranted) != 0 {
		t.Fatalf("blockers or authority lost: %#v", plan)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/findings", reader, fmt.Sprintf(`{"actor_kind":"read_only_agent","summary":"The open pull crosses both destinations.","impact":"Review intent must be divided without reusing approval.","affected_item_ids":["pull-8"],"uncertainty":"Contributor decision is pending.","citations":[{"repository_id":%q,"reference":"pull:8","revision":%q,"path":"pkg/parser"}]}`, repo.ID, commit), 201, &plan)
	if len(plan.Findings) != 1 || plan.Findings[0].ActorID != "reader-agent" || len(plan.AuthorityGranted) != 0 {
		t.Fatalf("read-only finding boundary lost: %#v", plan)
	}
	candidate := repositoryrestructuring.CandidateInput{MappingIDs: []string{"parser-code"}, Repositories: []repositoryrestructuring.CandidateRepository{{DestinationID: "parser", ObjectDigest: "sha256:objects", DefaultRef: "refs/heads/main", DefaultCommit: string(commit), ObjectCount: 3, SizeBytes: 512, Evidence: []repositoryrestructuring.PreservationEvidence{{Kind: "file_history", Reference: "pkg/parser", Source: string(commit), Candidate: string(commit), Status: "preserved", Digest: "sha256:history", Detail: "selected file ancestry is reachable"}, {Kind: "authorship", Reference: string(commit), Status: "preserved", Detail: "author and committer identities match"}, {Kind: "signature", Reference: string(commit), Status: "not_applicable", Detail: "source commit is unsigned"}, {Kind: "tag", Reference: "v1.0.0", Status: "preserved", Detail: "annotated tag target is retained"}, {Kind: "license_provenance", Reference: "LICENSE", Status: "preserved", Digest: "sha256:license", Detail: "license blob and source attribution match"}}}}, CrossRepositoryLinks: []repositoryrestructuring.PreservationEvidence{{Kind: "package_link", Reference: "app->parser", Status: "changed", Detail: "consumer must resolve the new package identity"}}, Issues: []repositoryrestructuring.PreservationEvidence{{Kind: "path_collision", Reference: "parser/README.md", Status: "changed", Detail: "two mapped files require an owner choice"}}, AssemblyCost: 8, RequiredDecisions: []string{"Choose the parser README source"}}
	b, _ = json.Marshal(candidate)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/candidates", reader, string(b), 401, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/candidates", writer, string(b), 201, &plan)
	if len(plan.Candidates) != 1 || len(plan.Candidates[0].AuthorityGranted) != 0 || plan.Candidates[0].Repositories[0].Evidence[0].Status != "preserved" {
		t.Fatalf("candidate provenance lost: %#v", plan.Candidates)
	}
	migration := repositoryrestructuring.MigrationPlanInput{CandidateID: plan.Candidates[0].ID, Revision: string(commit), Targets: []repositoryrestructuring.MigrationTarget{
		{ID: "clone", Kind: "clone", OwnerIDs: []string{"collaborator"}, Audience: "public", CurrentLocation: "https://git.example/monolith", ReplacementLocation: "https://git.example/parser", RedirectSignature: "ed25519:signed-redirect", Mappings: map[string]string{"refs/heads/main": "refs/heads/main"}, Synchronization: []string{"git remote set-url origin https://git.example/parser", "git fetch --prune origin"}, CompatibilityUntil: due, State: "redirect_ready", NextAction: "developers should update origin and fetch"},
		{ID: "consumer", Kind: "dependency", OwnerIDs: []string{"consumer-owner"}, Audience: "repository", CurrentLocation: "pkg:monolith/parser", ReplacementLocation: "pkg:parser", Mappings: map[string]string{"monolith/parser": "parser"}, Synchronization: []string{"update the lockfile through an ordinary pull request"}, CompatibilityUntil: due, State: "unmigrated", NextAction: "consumer owner should open a dependency pull request"},
		{ID: "peer", Kind: "federated_follower", OwnerIDs: []string{"peer-owner"}, Audience: "public", CurrentLocation: "fed:monolith", ReplacementLocation: "fed:parser", Mappings: map[string]string{"repository": "parser"}, Synchronization: []string{"verify the signed discovery document before following"}, CompatibilityUntil: due, State: "unavailable", NextAction: "retry when the independently governed peer is reachable"},
		{ID: "stale-docs", Kind: "documentation", OwnerIDs: []string{"docs-owner"}, Audience: "public", CurrentLocation: "https://git.example/parser", ReplacementLocation: "https://git.example/parser", Mappings: map[string]string{"old-guide": "new-guide"}, Synchronization: []string{"replace links through an ordinary documentation pull request"}, CompatibilityUntil: due, State: "planned", NextAction: "renew access and resolve the colliding redirect", CredentialReference: "credential:docs", CredentialExpiresAt: time.Now().UTC().Add(-time.Hour)},
	}}
	b, _ = json.Marshal(migration)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/migration-plans", writer, string(b), 201, &plan)
	if len(plan.MigrationPlans) != 1 || len(plan.MigrationPlans[0].Blockers) != 5 || len(plan.MigrationPlans[0].AuthorityGranted) != 0 {
		t.Fatalf("migration visibility or authority lost: %#v", plan.MigrationPlans)
	}
	consumerOwner := issueAccess(t, credentials, "consumer-owner", auth.API, auth.RepositoryRead)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/migration-plans/"+plan.MigrationPlans[0].ID+"/events", writer, `{"target_id":"consumer","state":"adopted","revision":"consumer-2","pull_request_reference":"consumer/pull:9","next_action":"release the independently reviewed consumer revision"}`, 403, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/migration-plans/"+plan.MigrationPlans[0].ID+"/events", consumerOwner, `{"target_id":"consumer","state":"adopted","revision":"consumer-2","pull_request_reference":"consumer/pull:9","release_reference":"consumer:v2","evidence":{"lockfile":"sha256:lock"},"next_action":"use the new package identity"}`, 201, &plan)
	if plan.MigrationPlans[0].Targets[1].State != "adopted" || plan.MigrationPlans[0].Events[0].ActorID != "consumer-owner" {
		t.Fatalf("owner propagation outcome lost: %#v", plan.MigrationPlans[0])
	}
	checks := []repositoryrestructuring.RehearsalCheck{}
	for _, domain := range []string{"git_clone", "git_fetch", "git_push", "build", "checks", "package_resolution", "api_resolution", "documentation", "workspaces", "consumer_journey"} {
		status := "passed"
		summary := domain + " worked through the public surface"
		if domain == "package_resolution" {
			status = "failed"
			summary = "consumer still resolves the monolith package"
		}
		checks = append(checks, repositoryrestructuring.RehearsalCheck{Domain: domain, Status: status, Command: "stock-" + domain, Reference: "run:" + domain, Digest: "sha256:" + domain, Summary: summary, Cost: 1})
	}
	rehearsal := repositoryrestructuring.RehearsalInput{CandidateID: plan.Candidates[0].ID, Environment: "networkless candidate sandbox", Budget: 9, ObservedCost: 10, Checks: checks, Issues: []repositoryrestructuring.PreservationEvidence{{Kind: "duplicated_history", Reference: "commit:shared", Status: "changed", Detail: "shared commit occurs in both candidate object graphs"}, {Kind: "unmovable_resource", Reference: "peer:offline", Status: "missing", Detail: "federated peer cannot be rehearsed"}}, RequiredDecisions: []string{"Select package compatibility alias", "Ask peer owner to verify links"}}
	b, _ = json.Marshal(rehearsal)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/rehearsals", writer, string(b), 201, &plan)
	if len(plan.Rehearsals) != 1 || plan.Rehearsals[0].Status != "blocked" || len(plan.Rehearsals[0].Blockers) != 6 || len(plan.Rehearsals[0].AuthorityGranted) != 0 {
		t.Fatalf("rehearsal blockers hidden: %#v", plan.Rehearsals)
	}
	work := repositoryrestructuring.WorkMappingInput{InventoryItemID: "issue-7", SourceRevision: string(commit), Kind: "issue", Authorship: []string{"reporter", "collaborator"}, Discussion: []string{"issue:7#comment-2"}, Reviews: []repositoryrestructuring.WorkReview{{ActorID: "reviewer", Revision: string(commit), Decision: "changes_requested", Reference: "issue:7#review"}}, Dependencies: []string{"decision:boundary"}, AcceptanceCriteria: []string{"parser and application behavior remain compatible"}, ContextAudience: "repository", Destinations: []repositoryrestructuring.WorkDestination{{DestinationID: "parser", Kind: "pull_request", Reference: "parser/pull:2", Revision: "parser-head", ContributionID: "contribution-parser"}, {DestinationID: "app", Kind: "pull_request", Reference: "app/pull:3", Revision: "app-head", ContributionID: "contribution-app", DependsOn: []string{"contribution-parser"}}}}
	b, _ = json.Marshal(work)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/work-mappings", writer, string(b), 201, &plan)
	if len(plan.WorkMappings) != 1 || plan.WorkMappings[0].Status != "proposed" || len(plan.WorkMappings[0].AuthorityGranted) != 0 {
		t.Fatalf("work context lost: %#v", plan.WorkMappings)
	}
	mapping := plan.WorkMappings[0]
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/work-mappings/"+mapping.ID+"/decisions", writer, fmt.Sprintf(`{"decision":"approved","reason":"intent and split accepted","expected_version":%d}`, mapping.Version), 201, &plan)
	if plan.WorkMappings[0].Status != "approved" {
		t.Fatalf("owner approval did not apply: %#v", plan.WorkMappings[0])
	}
	parserOwner := issueAccess(t, credentials, "parser-team", auth.API, auth.RepositoryRead)
	mapping = plan.WorkMappings[0]
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/work-mappings/"+mapping.ID+"/outcomes", parserOwner, fmt.Sprintf(`{"destination_id":"parser","status":"continued","revision":"parser-head","reference":"parser/pull:2","reason":"destination owner admitted contribution","expected_version":%d}`, mapping.Version), 201, &plan)
	appOwner := issueAccess(t, credentials, "app-team", auth.API, auth.RepositoryRead)
	mapping = plan.WorkMappings[0]
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/work-mappings/"+mapping.ID+"/outcomes", appOwner, fmt.Sprintf(`{"destination_id":"app","status":"continued","revision":"app-head","reference":"app/pull:3","reason":"dependent contribution admitted","expected_version":%d}`, mapping.Version), 201, &plan)
	if plan.WorkMappings[0].Status != "continued" {
		t.Fatalf("connected work did not continue: %#v", plan.WorkMappings[0])
	}
	bad := in
	bad.Sources[0].Revision = "missing"
	bad.Mappings[0].SourceRevision = "missing"
	b, _ = json.Marshal(bad)
	workflowJSON(t, server.URL, http.MethodPost, base, writer, string(b), 422, nil)
}
