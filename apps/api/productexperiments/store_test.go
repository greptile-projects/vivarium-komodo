package productexperiments

import "testing"

func plan(signal string) PlanInput {
	return PlanInput{Title: "Safer onboarding", Source: Source{Kind: "issue", ID: "issue-1"}, Hypothesis: "Guidance improves activation without increasing support load", Variants: []Variant{{ID: "control", Name: "Current", Control: true}, {ID: "guided", Name: "Guided"}}, Audience: Audience{Description: "New repository owners", Eligibility: []string{"created repository in 7 days"}, Exclusions: []string{"staff"}, Consent: "product_analytics", EstimatedSize: 500}, Measures: []Measure{{ID: "activation", Name: "Activation", Kind: "success", SignalID: signal, SignalVersion: 1, Aggregation: "conversion rate", Threshold: "+5%"}, {ID: "support", Name: "Support contacts", Kind: "guardrail", SignalID: signal, SignalVersion: 1, Aggregation: "count per user", Threshold: "no more than +2%"}}, MinimumEvidence: "100 users per variant and 95% confidence", DurationHours: 168, OwnerIDs: []string{"owner"}, ParticipantIDs: []string{"owner", "analyst"}, StopConditions: []string{"support contacts increase 2%"}, Assumptions: []string{"signal identity is stable"}, OverlapKeys: []string{"onboarding:new-owner"}, ChangeReason: "Agree before exposure"}
}
func TestPlanReadinessTracksVersionedSignalsApprovalsAndAssumptions(t *testing.T) {
	s, _ := New(t.TempDir())
	sig, e := s.CreateSignal("repo", "owner", SignalVersion{Name: "activation", Description: "Repository becomes active", Unit: "users", Event: "repository.activated", PermittedAudiences: []string{"product_analytics"}, Instrumented: true, ChangeReason: "approved telemetry"})
	if e != nil {
		t.Fatal(e)
	}
	x, e := s.Create("repo", "owner", plan(sig.ID))
	if e != nil {
		t.Fatal(e)
	}
	if x.Ready || len(x.Blockers) != 2 {
		t.Fatalf("expected participant blockers: %+v", x.Blockers)
	}
	x, _ = s.Approve("repo", x.ID, "owner", "approved", "")
	x, _ = s.Approve("repo", x.ID, "analyst", "approved", "")
	if !x.Ready {
		t.Fatalf("approved plan not ready: %+v", x.Blockers)
	}
	x, _ = s.ChangeAssumption("repo", x.ID, "analyst", "signal identity is stable", "event renamed upstream")
	if x.Ready || x.Blockers[0].Kind != "changed_assumption" {
		t.Fatalf("changed assumption hidden: %+v", x.Blockers)
	}
	p := plan(sig.ID)
	p.Assumptions = []string{"new event is stable"}
	p.ChangeReason = "Address signal rename"
	x, e = s.Revise("repo", x.ID, "owner", 1, p)
	if e != nil {
		t.Fatal(e)
	}
	if x.Ready {
		t.Fatal("old approvals incorrectly approved revision")
	}
}
func TestOverlapAndSignalPolicyStayExplicit(t *testing.T) {
	s, _ := New(t.TempDir())
	sig, _ := s.CreateSignal("repo", "owner", SignalVersion{Name: "activation", Description: "Activation", Unit: "users", Event: "activated", PermittedAudiences: []string{"research_consent"}, Instrumented: false, ChangeReason: "draft"})
	first, _ := s.Create("repo", "owner", plan(sig.ID))
	second, _ := s.Create("repo", "owner", plan(sig.ID))
	second, _ = s.Get("repo", second.ID)
	kinds := map[string]bool{}
	for _, b := range second.Blockers {
		kinds[b.Kind] = true
	}
	if !kinds["missing_instrumentation"] || !kinds["ineligible_audience"] || !kinds["overlapping_experiment"] {
		t.Fatalf("missing derived blockers %+v (first %s)", second.Blockers, first.ID)
	}
}

func TestImplementationWorkIsRevisionExactAndReviewSafe(t *testing.T) {
	s, _ := New(t.TempDir())
	sig, _ := s.CreateSignal("repo", "owner", SignalVersion{Name: "activation", Description: "Activation", Unit: "users", Event: "repository.activated", Properties: []string{"variant"}, PermittedAudiences: []string{"product_analytics"}, Instrumented: true, ChangeReason: "reviewed event"})
	x, _ := s.Create("repo", "owner", plan(sig.ID))
	x, err := s.AddWorkItem("repo", x.ID, "owner", WorkItem{Kind: "workspace", OwnerKind: "agent", OwnerID: "codex", VariantIDs: []string{"guided"}, ResourceID: "ws_1", Revision: "abc123"})
	if err != nil || len(x.WorkItems) != 1 || x.WorkItems[0].PlanVersion != 1 {
		t.Fatalf("work item: %+v %v", x.WorkItems, err)
	}
	in := ImplementationInput{PullRequestID: "pr_1", VariantIDs: []string{"control", "guided"}, EventDefinitions: []EventDefinition{{SignalID: sig.ID, SignalVersion: 1, Event: "repository.activated", Properties: []string{"variant"}}, {SignalID: sig.ID, SignalVersion: 1, Event: "repository.activated", Properties: []string{"variant"}}}, ExposureRules: []string{"eligible owners deterministically assigned"}, PrivacyClassification: "consented product analytics", RemovalPlan: "delete flag and event after decision", CheckNames: map[string]string{"assignment": "experiment/assignment", "metric_capture": "experiment/metrics", "variant_isolation": "experiment/isolation", "fallback": "experiment/fallback"}}
	x, err = s.AddImplementation("repo", x.ID, "owner", "deadbeef", in)
	if err != nil || len(x.Implementations) != 1 || x.Implementations[0].SourceCommitID != "deadbeef" || !x.Implementations[0].Current {
		t.Fatalf("implementation: %+v %v", x.Implementations, err)
	}
	p := plan(sig.ID)
	p.ChangeReason = "revise audience"
	x, _ = s.Revise("repo", x.ID, "owner", 1, p)
	if x.Implementations[0].Current {
		t.Fatal("old implementation remained current after plan revision")
	}
	bad := in
	bad.CheckNames = map[string]string{"assignment": "only-one"}
	if _, err := s.AddImplementation("repo", x.ID, "owner", "new", bad); err != ErrInvalid {
		t.Fatalf("missing repository checks accepted: %v", err)
	}
}

func TestAudiencePolicyGovernsExactReleaseConsentAndStableAssignment(t *testing.T) {
	s, _ := New(t.TempDir())
	sig, _ := s.CreateSignal("repo", "owner", SignalVersion{Name: "activation", Description: "Activation", Unit: "users", Event: "repository.activated", Properties: []string{"variant", "completed"}, PermittedAudiences: []string{"product_analytics"}, Instrumented: true, ChangeReason: "reviewed"})
	x, _ := s.Create("repo", "owner", plan(sig.ID))
	impl := ImplementationInput{PullRequestID: "pr", VariantIDs: []string{"control", "guided"}, EventDefinitions: []EventDefinition{{SignalID: sig.ID, SignalVersion: 1, Event: "repository.activated", Properties: []string{"variant"}}, {SignalID: sig.ID, SignalVersion: 1, Event: "repository.activated", Properties: []string{"variant"}}}, ExposureRules: []string{"stable"}, PrivacyClassification: "consented", RemovalPlan: "remove", CheckNames: map[string]string{"assignment": "a", "metric_capture": "m", "variant_isolation": "i", "fallback": "f"}}
	var implErr error
	x, implErr = s.AddImplementation("repo", x.ID, "owner", "released", impl)
	if implErr != nil {
		t.Fatal(implErr)
	}
	in := AudiencePolicyInput{ExpectedPlanVersion: 1, ReleaseID: "rel", VariantIDs: []string{"control", "guided"}, MutualExclusionGroup: "onboarding", Eligibility: EligibilityPolicy{ConsentClass: "product_analytics", Regions: []string{"EU"}, RequiredAttributes: []string{"new_owner"}, ExcludedAttributes: []string{"staff"}}, Allocation: []Allocation{{VariantID: "control", BasisPoints: 5000}, {VariantID: "guided", BasisPoints: 5000}}, Collection: []CollectionField{{SignalID: sig.ID, SignalVersion: 1, Properties: []string{"variant"}}}, RetentionDays: 30, ApproverIDs: []string{"owner", "privacy"}, ChangeReason: "bounded audience"}
	x, err := s.PutAudiencePolicy("repo", x.ID, "owner", "rel", "released", in)
	if err != nil || x.AudiencePolicies[0].Ready {
		t.Fatalf("unapproved policy: %#v %v", x.AudiencePolicies, err)
	}
	x, _ = s.ApproveAudiencePolicy("repo", x.ID, "owner", "approved", "release owner")
	x, _ = s.ApproveAudiencePolicy("repo", x.ID, "privacy", "approved", "minimal collection")
	if !x.AudiencePolicies[0].Ready {
		t.Fatalf("policy blockers: %#v", x.AudiencePolicies[0].Blockers)
	}
	x, err = s.Assign("repo", x.ID, "owner", AssignmentInput{Subject: "private-user-id", Region: "EU", ConsentClasses: []string{"product_analytics"}, Attributes: []string{"new_owner"}})
	if err != nil || len(x.AudiencePolicies[0].Assignments) != 1 || x.AudiencePolicies[0].Assignments[0].SubjectDigest == "private-user-id" || x.AudiencePolicies[0].Assignments[0].VariantID == "" {
		t.Fatalf("assignment: %#v %v", x.AudiencePolicies[0].Assignments, err)
	}
	x, _ = s.Assign("repo", x.ID, "owner", AssignmentInput{Subject: "private-user-id", Region: "EU", ConsentClasses: []string{"product_analytics"}, Attributes: []string{"new_owner"}})
	if len(x.AudiencePolicies[0].Assignments) != 1 {
		t.Fatal("assignment was not stable and idempotent")
	}
	x, _ = s.Assign("repo", x.ID, "owner", AssignmentInput{Subject: "without-consent", Region: "EU", Attributes: []string{"new_owner"}})
	if x.AudiencePolicies[0].Assignments[1].Decision != "excluded" || x.AudiencePolicies[0].Assignments[1].VariantID != "" {
		t.Fatalf("consent exclusion: %#v", x.AudiencePolicies[0].Assignments[1])
	}
}

func TestAudiencePolicyRejectsBiasedOrUnauthorizedAllocation(t *testing.T) {
	s, _ := New(t.TempDir())
	sig, _ := s.CreateSignal("repo", "owner", SignalVersion{Name: "signal", Description: "signal", Unit: "users", Event: "event", Properties: []string{"allowed"}, PermittedAudiences: []string{"consent"}, Instrumented: true, ChangeReason: "reviewed"})
	p := plan(sig.ID)
	p.Audience.Consent = "consent"
	x, _ := s.Create("repo", "owner", p)
	in := AudiencePolicyInput{ExpectedPlanVersion: 1, ReleaseID: "rel", VariantIDs: []string{"control", "guided"}, MutualExclusionGroup: "group", Eligibility: EligibilityPolicy{ConsentClass: "consent"}, Allocation: []Allocation{{VariantID: "control", BasisPoints: 9000}, {VariantID: "guided", BasisPoints: 900}}, Collection: []CollectionField{{SignalID: sig.ID, SignalVersion: 1, Properties: []string{"secret"}}}, RetentionDays: 30, ApproverIDs: []string{"owner"}, ChangeReason: "test"}
	if _, err := s.PutAudiencePolicy("repo", x.ID, "owner", "rel", "commit", in); err != ErrInvalid {
		t.Fatalf("biased incomplete allocation accepted: %v", err)
	}
	in.Allocation[1].BasisPoints = 1000
	x, err := s.PutAudiencePolicy("repo", x.ID, "owner", "rel", "commit", in)
	if err != nil || x.AudiencePolicies[0].Blockers[0].Kind != "stale_release" {
		t.Fatalf("release blocker: %#v %v", x.AudiencePolicies, err)
	}
	found := false
	for _, b := range x.AudiencePolicies[0].Blockers {
		found = found || b.Kind == "unauthorized_collection"
	}
	if !found {
		t.Fatalf("unauthorized property was hidden: %#v", x.AudiencePolicies[0].Blockers)
	}
}

func TestLiveRunStagesControlAndDeterministicContainment(t *testing.T) {
	s, _ := New(t.TempDir())
	sig, _ := s.CreateSignal("repo", "owner", SignalVersion{Name: "activation", Description: "Activation", Unit: "users", Event: "activated", Properties: []string{"variant"}, PermittedAudiences: []string{"product_analytics"}, Instrumented: true, ChangeReason: "ready"})
	x, _ := s.Create("repo", "owner", plan(sig.ID))
	for _, actor := range []string{"owner", "analyst"} {
		x, _ = s.Approve("repo", x.ID, actor, "approved", "")
	}
	impl := ImplementationInput{PullRequestID: "pr", VariantIDs: []string{"control", "guided"}, EventDefinitions: []EventDefinition{{SignalID: sig.ID, SignalVersion: 1, Event: "activated"}, {SignalID: sig.ID, SignalVersion: 1, Event: "activated"}}, ExposureRules: []string{"stable"}, PrivacyClassification: "consented", RemovalPlan: "remove", CheckNames: map[string]string{"assignment": "a", "metric_capture": "m", "variant_isolation": "i", "fallback": "f"}}
	x, _ = s.AddImplementation("repo", x.ID, "owner", "commit", impl)
	policy := AudiencePolicyInput{ExpectedPlanVersion: 1, ReleaseID: "release", VariantIDs: []string{"control", "guided"}, MutualExclusionGroup: "onboarding", Eligibility: EligibilityPolicy{ConsentClass: "product_analytics"}, Allocation: []Allocation{{VariantID: "control", BasisPoints: 5000}, {VariantID: "guided", BasisPoints: 5000}}, Collection: []CollectionField{{SignalID: sig.ID, SignalVersion: 1, Properties: []string{"variant"}}}, RetentionDays: 30, ApproverIDs: []string{"owner"}, ChangeReason: "launch"}
	x, _ = s.PutAudiencePolicy("repo", x.ID, "owner", "release", "commit", policy)
	x, _ = s.ApproveAudiencePolicy("repo", x.ID, "owner", "approved", "")
	stages := []RunStage{{Name: "canary", MaxExposure: 500, Allocation: policy.Allocation}, {Name: "broader", MaxExposure: 2500, Allocation: []Allocation{{VariantID: "control", BasisPoints: 4000}, {VariantID: "guided", BasisPoints: 6000}}}}
	x, err := s.Launch("repo", x.ID, "owner", "production", "deployment", stages)
	if err != nil {
		t.Fatal(err)
	}
	run := x.Runs[0]
	x, err = s.Advance("repo", x.ID, run.ID, "analyst", "credible canary")
	if err != nil || x.Runs[0].CurrentStage != 2 {
		t.Fatalf("advance: %#v %v", x.Runs, err)
	}
	x, err = s.Observe("repo", x.ID, run.ID, "analyst", ObservationInput{ExposureByVariant: map[string]int{"control": 120, "guided": 130}, MeasureValues: map[string]float64{"activation": .42}, Uncertainty: map[string]float64{"activation": .04}, DataQuality: "healthy", OperationalHealth: "healthy", InstrumentationHealth: "healthy", ConsentHealth: "valid", GuardrailBreached: true, CostUnits: 8, Evidence: []string{"dashboard:1"}})
	if err != nil || x.Runs[0].Status != "contained" || !x.Runs[0].Observations[0].ContainmentTriggered || x.Runs[0].ContainmentReason != "guardrail breach" {
		t.Fatalf("containment: %#v %v", x.Runs[0], err)
	}
	if _, err = s.Control("repo", x.ID, run.ID, "owner", "resume", "unsafe"); err != ErrConflict {
		t.Fatalf("contained run resumed: %v", err)
	}
	if len(x.Runs[0].Stages) != 2 || len(x.Runs[0].Observations) != 1 {
		t.Fatal("containment discarded evidence")
	}
}

func TestEvidenceDecisionTasksAndCleanupRetainLearning(t *testing.T) {
	s, _ := New(t.TempDir())
	sig, _ := s.CreateSignal("repo", "owner", SignalVersion{Name: "activation", Description: "Activation", Unit: "users", Event: "activated", Properties: []string{"variant"}, PermittedAudiences: []string{"product_analytics"}, Instrumented: true, ChangeReason: "ready"})
	x, _ := s.Create("repo", "owner", plan(sig.ID))
	x.Runs = []Run{{ID: "run", PlanVersion: 1, Status: "stopped", Observations: []Observation{{ID: "obs", MeasureValues: map[string]float64{"activation": .5}, Uncertainty: map[string]float64{"activation": .03}}}}}
	if err := write(s.path("repo", "experiments", x.ID), x); err != nil {
		t.Fatal(err)
	}
	x, err := s.AddAnalysis("repo", x.ID, "analyst", AnalysisInput{RunID: "run", ObservationID: "obs", EvidenceState: "threshold_reached", Summary: "Guided wins overall, with a weaker small-team segment", SegmentEffects: []SegmentEffect{{Segment: "small teams", VariantID: "guided", MeasureID: "activation", Effect: .02, Uncertainty: .04, SampleSize: 80}}, Exclusions: []string{"staff"}, Guardrails: []GuardrailOutcome{{MeasureID: "support", Status: "passed", Value: .01, Uncertainty: .01}}, Interpretation: Interpretation{Summary: "Effect is credible but heterogeneous", ActorKind: "agent", ActorID: "agent:analyst", Evidence: []string{"aggregate:1"}, Uncertainty: "small-team effect crosses zero"}, Dissent: []Dissent{{ActorID: "owner", Position: "roll out gradually", Evidence: []string{"segment:small"}}}, AggregatedEvidence: []string{"aggregate:1"}})
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.Decide("repo", x.ID, "owner", DecisionInput{ExpectedVersion: 0, AnalysisID: x.Analyses[0].ID, Outcome: "adopt_variant", AdoptedVariantID: "guided", Rationale: "threshold met with protected rollout", UserProtections: []string{"honor existing consent"}, Tasks: []OutcomeTask{{Kind: "rollout", Title: "ship guided", OwnerID: "owner", RequiredActions: []string{"remove control flag", "stop collection"}}}, ChangeReason: "turn learning into product"})
	if err != nil {
		t.Fatal(err)
	}
	d := x.Decisions[0]
	x, err = s.CompleteOutcomeTask("repo", x.ID, d.ID, d.Tasks[0].ID, "owner", TaskCompletion{PullRequestID: "pr", ReleaseID: "rel", DeploymentID: "dep", Evidence: []string{"checks:passed"}})
	if err != nil || !x.Decisions[0].Complete {
		t.Fatalf("task: %#v %v", x.Decisions, err)
	}
	x, err = s.CompleteCleanup("repo", x.ID, d.ID, "owner")
	if err != nil || x.Cleanup == nil || !x.Cleanup.CollectionStopped || x.Cleanup.AggregatedEvidenceRetained[0] != "aggregate:1" {
		t.Fatalf("cleanup: %#v %v", x.Cleanup, err)
	}
	if _, err = s.Assign("repo", x.ID, "owner", AssignmentInput{Subject: "later"}); err != ErrConflict {
		t.Fatalf("assignment after cleanup: %v", err)
	}
}
