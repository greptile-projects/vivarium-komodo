package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/performancegoals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestPerformanceGoalContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	owner, collab := "owner", "collab"
	repo, _ := catalog.Create(owner, repositories.Metadata{Name: "speed", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator(owner, repo.ID, collab)
	ownerToken := issueAccess(t, credentials, owner, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	collabToken := issueAccess(t, credentials, collab, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := performancegoals.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	mux := http.NewServeMux()
	registerPerformanceGoalsHTTP(mux, store, catalog, releaseStore, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := server.URL + "/repositories/" + string(repo.ID) + "/performance-goals"
	old := time.Now().UTC().Add(-60 * 24 * time.Hour).Format(time.RFC3339)
	body := `{"subject_kind":"api","subject_id":"GET /search","title":"Keep search predictably fast","workloads":["100 concurrent repository searches"],"metrics":[{"id":"p95","name":"response latency","unit":"ms","direction":"lower","baseline":420,"target":{"maximum":250},"budget":300,"environment_digest":"linux-amd64","baseline_measured_at":"` + old + `","baseline_source":"preview pv1"}],"correctness_constraints":["Results remain complete and permission filtered"],"supported_environments":[{"name":"Production Linux","os":"linux","architecture":"amd64","digest":"linux-amd64"}],"owner_ids":["` + owner + `","` + collab + `"],"links":[{"kind":"issue","resource_id":"issue-1"},{"kind":"decision","resource_id":"decision-1"}],"baseline_max_age_days":30,"change_reason":"Agree before optimizing"}`
	var goal performancegoals.Goal
	workflowJSON(t, server.URL, http.MethodPost, base[len(server.URL):], ownerToken, body, 201, &goal)
	if goal.CurrentVersion != 1 || goal.Statuses[0].State != "missing_measurement" {
		t.Fatalf("unexpected goal: %+v", goal)
	}
	measurement := `{"version":1,"metric_id":"p95","value":230,"environment_digest":"mac-arm64","source":"benchmark run 44"}`
	workflowJSON(t, server.URL, http.MethodPost, base[len(server.URL):]+"/"+goal.ID+"/measurements", collabToken, measurement, 201, &goal)
	if goal.Statuses[0].State != "incomparable_environment" || goal.Measurements[0].ActorID != collab {
		t.Fatalf("measurement must remain explicitly incomparable and attributable: %+v", goal)
	}
	var listed struct {
		Items []performancegoals.Goal `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base[len(server.URL):], collabToken, "", 200, &listed)
	if len(listed.Items) != 1 {
		t.Fatal("goal not listed")
	}
	workflowJSON(t, server.URL, http.MethodPost, base[len(server.URL):]+"/"+goal.ID+"/versions", ownerToken, body[:1]+`"expected_version":0,`+body[1:], http.StatusConflict, nil)
}

func TestPerformanceTrialRetainsComparableEvidence(t *testing.T) {
	store, _ := performancegoals.New(t.TempDir())
	max := 250.0
	goal, err := store.Create("repo", "owner", performancegoals.VersionInput{SubjectKind: "service", Title: "fast", Workloads: []string{"captured search"}, Metrics: []performancegoals.Metric{{ID: "p95", Name: "latency", Unit: "ms", Direction: "lower", Target: performancegoals.Range{Maximum: &max}, EnvironmentDigest: "env1"}}, CorrectnessConstraints: []string{"same results"}, Environments: []performancegoals.Environment{{Name: "linux", Digest: "env1"}}, OwnerIDs: []string{"owner"}, BaselineMaxAgeDays: 30, ChangeReason: "investigate"})
	if err != nil {
		t.Fatal(err)
	}
	goal, err = store.RecordTrial("repo", goal.ID, "collab", performancegoals.TrialInput{Version: 1, Benchmark: "search", DefinitionDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Revision: "deadbeef", Environment: performancegoals.Environment{Name: "linux", Digest: "env1"}, WorkloadSource: "sanitized_production_capture", InputDigests: []string{"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}, WarmupRuns: 2, SamplingMethod: "wall_clock", Samples: []performancegoals.Sample{{Value: 100}, {Value: 120}, {Value: 110}}, ResourceProfile: performancegoals.ResourceProfile{CPUSeconds: 1.2, PeakMemoryMB: 42}, Evidence: []performancegoals.Evidence{{Kind: "trace", Name: "trace.json", MediaType: "application/json", SHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", Content: "{}"}}, Cost: 0.04})
	if err != nil {
		t.Fatal(err)
	}
	trial := goal.Trials[0]
	if trial.Mean != 110 || trial.Variance != 100 || trial.ActorID != "collab" || trial.WorkloadSource != "sanitized_production_capture" {
		t.Fatalf("incomplete evidence: %+v", trial)
	}
	_, err = store.RecordTrial("repo", goal.ID, "collab", performancegoals.TrialInput{Version: 1, Benchmark: "search", DefinitionDigest: "x", Revision: "deadbeef", Environment: performancegoals.Environment{Name: "linux", Digest: "env1"}, WorkloadSource: "repository_fixture", SamplingMethod: "wall_clock", Samples: []performancegoals.Sample{{Value: 1}, {Value: 2}}, Evidence: []performancegoals.Evidence{{Kind: "log", Name: "out", SHA256: "x", Content: "Authorization: Bearer secret"}}})
	if !errors.Is(err, performancegoals.ErrInvalid) {
		t.Fatal("credential-like evidence accepted")
	}
}

func TestPerformanceComparisonBindsComparableExactRevisions(t *testing.T) {
	store, _ := performancegoals.New(t.TempDir())
	max := 250.0
	goal, _ := store.Create("repo", "owner", performancegoals.VersionInput{SubjectKind: "service", Title: "fast", Workloads: []string{"search"}, Metrics: []performancegoals.Metric{{ID: "latency", Name: "latency", Unit: "ms", Direction: "lower", Target: performancegoals.Range{Maximum: &max}, EnvironmentDigest: "env"}}, CorrectnessConstraints: []string{"same results"}, Environments: []performancegoals.Environment{{Name: "linux", Digest: "env"}}, OwnerIDs: []string{"owner"}, BaselineMaxAgeDays: 30, ChangeReason: "optimize"})
	record := func(revision string, samples []performancegoals.Sample, cpu, memory, cost float64) performancegoals.Trial {
		goal, _ = store.RecordTrial("repo", goal.ID, "owner", performancegoals.TrialInput{Version: 1, Benchmark: "search", DefinitionDigest: "definition", Revision: revision, Environment: performancegoals.Environment{Name: "linux", Digest: "env"}, WorkloadSource: "repository_fixture", SamplingMethod: "wall_clock", Samples: samples, ResourceProfile: performancegoals.ResourceProfile{CPUSeconds: cpu, PeakMemoryMB: memory}, Cost: cost})
		return goal.Trials[len(goal.Trials)-1]
	}
	base := record("base", []performancegoals.Sample{{Value: 100}, {Value: 110}, {Value: 90}}, 3, 40, .03)
	candidate := record("candidate", []performancegoals.Sample{{Value: 70}, {Value: 80}, {Value: 75}}, 2, 48, .02)
	goal, err := store.Compare("repo", goal.ID, "analyst", performancegoals.ComparisonInput{Version: 1, BaselineTrialID: base.ID, CandidateTrialID: candidate.ID, PullRequestID: "pull", MetricID: "latency", CorrectnessChecks: []string{"results identical"}, AffectedScenarios: []string{"search"}, Commands: []string{"go test ./..."}, ResidualRisks: []string{"memory rises"}})
	if err != nil {
		t.Fatal(err)
	}
	c := goal.Comparisons[0]
	if c.MeanChange != -25 || c.PercentChange != -25 || c.PeakMemoryMBChange != 8 || c.CostChange > -.009 || c.ActorID != "analyst" || c.Confidence95.Maximum == nil {
		t.Fatalf("comparison lost tradeoffs: %+v", c)
	}
}
