package performancegoals

import "testing"

func TestDeliveryPolicyRequiresCurrentCertainComparisonAndRetainsRolloutOutcome(t *testing.T) {
	s, _ := New(t.TempDir())
	max := 100.0
	g, _ := s.Create("repo", "owner", VersionInput{SubjectKind: "service", Title: "fast", Workloads: []string{"search"}, Metrics: []Metric{{ID: "latency", Name: "latency", Unit: "ms", Direction: "lower", Target: Range{Maximum: &max}, EnvironmentDigest: "prod"}}, CorrectnessConstraints: []string{"correct"}, Environments: []Environment{{Name: "prod", Digest: "prod"}}, OwnerIDs: []string{"owner"}, BaselineMaxAgeDays: 30, ChangeReason: "guard"})
	g, e := s.PutDeliveryPolicy("repo", g.ID, "owner", DeliveryPolicyInput{Branch: "main", Paths: []string{"api/*"}, Thresholds: []RegressionThreshold{{MetricID: "latency", MaximumPercentRegression: 5, RequireConfidence: true}}})
	if e != nil {
		t.Fatal(e)
	}
	reqs, _ := s.AssessDelivery("repo", "pull", "candidate", "main", []string{"api/search.go"}, nil)
	if len(reqs) != 1 || reqs[0].Status != "missing" {
		t.Fatalf("missing evidence not blocked: %#v", reqs)
	}
	record := func(rev string, values ...float64) Trial {
		samples := []Sample{}
		for _, v := range values {
			samples = append(samples, Sample{Value: v})
		}
		g, _ = s.RecordTrial("repo", g.ID, "owner", TrialInput{Version: 1, Benchmark: "search", DefinitionDigest: "d", Revision: rev, Environment: Environment{Name: "prod", Digest: "prod"}, WorkloadSource: "repository_fixture", SamplingMethod: "wall_clock", Samples: samples})
		return g.Trials[len(g.Trials)-1]
	}
	b := record("base", 100, 100, 100)
	c := record("candidate", 90, 90, 90)
	g, e = s.Compare("repo", g.ID, "owner", ComparisonInput{Version: 1, BaselineTrialID: b.ID, CandidateTrialID: c.ID, PullRequestID: "pull", MetricID: "latency", CorrectnessChecks: []string{"ok"}, AffectedScenarios: []string{"search"}, Commands: []string{"bench"}, ResidualRisks: []string{"traffic"}})
	if e != nil {
		t.Fatal(e)
	}
	reqs, _ = s.AssessDelivery("repo", "pull", "candidate", "main", []string{"api/search.go"}, nil)
	if reqs[0].Status != "satisfied" {
		t.Fatalf("passing candidate blocked: %#v", reqs)
	}
	cmp := g.Comparisons[0]
	g, e = s.ObserveDelivery("repo", g.ID, "operator", DeliveryObservationInput{GoalVersion: 1, ComparisonID: cmp.ID, ReleaseID: "release", DeploymentID: "deploy", Stage: "canary", Revision: "candidate", MetricID: "latency", Value: 130, EnvironmentDigest: "prod", Health: "passing", Assumptions: []string{"representative traffic"}, Action: "pause"})
	if e != nil || g.Observations[0].Outcome != "regressed" || g.Observations[0].Action != "pause" {
		t.Fatalf("rollout evidence lost: %#v %v", g.Observations, e)
	}
}
