package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/performancegoals"
	pi "github.com/greptile-projects/vivarium-komodo/apps/api/performanceinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestPerformanceInvestigationExplainsEvidenceWithoutLeakingIt(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "profiles", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "analyst")
	opened, _ := catalog.Open(repo.ID)
	blob, _ := opened.WriteObject(storage.BlobObject, []byte("package search\nfunc Query() {}\n"))
	tree, _ := opened.WriteObject(storage.TreeObject, append([]byte("100644 search.go\x00"), objectIDBytes(t, blob)...))
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor Owner <owner@example.com> 1 +0000\ncommitter Owner <owner@example.com> 1 +0000\n\nprofile search\n", tree)))
	goals, _ := performancegoals.New(t.TempDir())
	max := 250.0
	g, _ := goals.Create(string(repo.ID), "owner", performancegoals.VersionInput{SubjectKind: "service", Title: "Search latency", Workloads: []string{"captured search"}, Metrics: []performancegoals.Metric{{ID: "p95", Name: "latency", Unit: "ms", Direction: "lower", Target: performancegoals.Range{Maximum: &max}, EnvironmentDigest: "prod-v1"}}, CorrectnessConstraints: []string{"same results"}, Environments: []performancegoals.Environment{{Name: "production", Digest: "prod-v1"}}, OwnerIDs: []string{"owner"}, BaselineMaxAgeDays: 30, ChangeReason: "diagnose"})
	g, _ = goals.RecordTrial(string(repo.ID), g.ID, "analyst", performancegoals.TrialInput{Version: 1, Benchmark: "search", DefinitionDigest: "definition", Revision: string(commit), Environment: performancegoals.Environment{Name: "production", Digest: "prod-v1"}, WorkloadSource: "sanitized_production_capture", SamplingMethod: "profile", Samples: []performancegoals.Sample{{Value: 400}, {Value: 420}}, Evidence: []performancegoals.Evidence{{Kind: "profile", Name: "cpu.pb", SHA256: "profile"}}})
	store, _ := pi.New(t.TempDir())
	mux := http.NewServeMux()
	registerPerformanceInvestigationsHTTP(mux, store, goals, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	analyst := issueAccess(t, credentials, "analyst", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	base := "/repositories/" + string(repo.ID) + "/performance-investigations"
	body := `{"goal_id":"` + g.ID + `","goal_version":1,"title":"Search CPU diagnosis","question":"Why does search spend CPU?","owner_ids":["owner"],"evidence":[{"trial_id":"` + g.Trials[0].ID + `","revision":"` + string(commit) + `","workload_source":"sanitized_production_capture","environment_digest":"prod-v1","visibility":"participants"}]}`
	var investigation pi.Investigation
	workflowJSON(t, server.URL, http.MethodPost, base, analyst, body, 201, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/participants", analyst, `{"user_id":"owner"}`, 200, &investigation)
	entry := `{"kind":"flame_graph","title":"Parser dominates","body":"Parser frames account for most sampled CPU.","audience":"participants","flamegraph":"root;search;parse 72","uncertainty":"Sampling omits blocked time.","citations":[{"kind":"profile","trial_id":"` + g.Trials[0].ID + `"},{"kind":"symbol","path":"search.go","symbol":"Query"}]}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/entries", analyst, entry, 201, &investigation)
	if investigation.Entries[0].ActorID != "analyst" || investigation.Entries[0].Flamegraph == "" {
		t.Fatalf("diagnosis evidence lost: %+v", investigation)
	}
	leak := `{"kind":"hypothesis","title":"Public claim","body":"Leaks restricted evidence.","audience":"repository","citations":[{"kind":"trial","trial_id":"` + g.Trials[0].ID + `"}]}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/entries", analyst, leak, 422, nil)
	challenge := `{"kind":"challenge","title":"Check allocator frames","body":"The comparison does not separate allocation cost.","audience":"participants","challenges":"` + investigation.Entries[0].ID + `","uncertainty":"Need an allocation profile.","citations":[{"kind":"profile","trial_id":"` + g.Trials[0].ID + `"}]}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/entries", owner, challenge, 201, &investigation)
	if investigation.Entries[1].ActorID != "owner" || investigation.Entries[1].Challenges != investigation.Entries[0].ID {
		t.Fatal("owner challenge not retained")
	}
	// A later goal contract makes the exact-version explanation visibly stale.
	_, _ = goals.Revise(string(repo.ID), g.ID, "owner", 1, performancegoals.VersionInput{SubjectKind: "service", Title: "Search latency", Workloads: []string{"new workload"}, Metrics: []performancegoals.Metric{{ID: "p95", Name: "latency", Unit: "ms", Direction: "lower", Target: performancegoals.Range{Maximum: &max}, EnvironmentDigest: "prod-v1"}}, CorrectnessConstraints: []string{"same results"}, Environments: []performancegoals.Environment{{Name: "production", Digest: "prod-v1"}}, OwnerIDs: []string{"owner"}, BaselineMaxAgeDays: 30, ChangeReason: "new workload"})
	workflowJSON(t, server.URL, http.MethodGet, base+"/"+investigation.ID, owner, "", 200, &investigation)
	if !investigation.Entries[0].Stale || investigation.Entries[0].StaleReasons[0] != "goal_version_changed" {
		t.Fatalf("changed contract did not stale finding: %+v", investigation.Entries[0])
	}
}
