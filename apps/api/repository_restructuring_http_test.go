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
	in := repositoryrestructuring.Input{Title: "Extract parser", Summary: "Make the shared parser independently authoritative.", Sources: []repositoryrestructuring.Source{{RepositoryID: string(repo.ID), Revision: string(commit), OwnerIDs: []string{"owner"}, Role: "primary"}}, Destinations: []repositoryrestructuring.Destination{{ID: "parser", Name: "parser", OwnerIDs: []string{"parser-team"}, Visibility: "public", DefaultBranch: "main", RetainedIdentities: []string{"package:parser"}}, {ID: "app", Name: "app", OwnerIDs: []string{"owner"}, Visibility: "private", DefaultBranch: "trunk"}}, Mappings: []repositoryrestructuring.Mapping{{ID: "parser-code", SourceRepositoryID: string(repo.ID), SourceRevision: string(commit), SourcePaths: []string{"pkg/parser"}, DestinationID: "parser", DestinationPaths: []string{"."}, HistoryMode: "path_history", Disposition: "move", Rationale: "preserve parser changes"}}, Inventory: []repositoryrestructuring.InventoryItem{{ID: "main", Kind: "ref", RepositoryID: string(repo.ID), Reference: "refs/heads/main", Revision: string(commit), OwnerIDs: []string{"owner"}, Access: "accessible", Disposition: "split", DestinationIDs: []string{"parser", "app"}}, {ID: "pull-8", Kind: "pull_request", RepositoryID: string(repo.ID), Reference: "pull:8", Revision: string(commit), OwnerIDs: []string{"contributor"}, Access: "ambiguous", Disposition: "unresolved", Reason: "touches both boundaries"}, {ID: "peer", Kind: "federated_relationship", RepositoryID: string(repo.ID), Reference: "peer:offline", Revision: string(commit), OwnerIDs: []string{"peer-owner"}, Access: "inaccessible", Disposition: "remain", Reason: "peer unavailable"}}, Deadline: due, SuccessCriteria: []string{"both projects clone and build"}, RollbackLimits: repositoryrestructuring.RollbackLimits{LatestTime: due.Add(24 * time.Hour), IrreversibleAfter: "source archive", MaximumDataLoss: "none", RequiredRetentions: []string{"pull discussions"}}}
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
	bad := in
	bad.Sources[0].Revision = "missing"
	bad.Mappings[0].SourceRevision = "missing"
	b, _ = json.Marshal(bad)
	workflowJSON(t, server.URL, http.MethodPost, base, writer, string(b), 422, nil)
}
