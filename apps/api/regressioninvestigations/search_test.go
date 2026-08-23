package regressioninvestigations

import "testing"

func TestSearchRetainsAmbiguousMergeAndCitedCause(t *testing.T) {
	s, _ := New(t.TempDir())
	v, _ := s.Create("repo", "owner", Input{Title: "regression", Scope: Scope{ExpectedBehavior: "works", RegressedBehavior: "fails"}})
	v, err := s.CreateScenario("repo", v.ID, "agent", true, ScenarioDefinition{Title: "repro", Commands: []string{"test"}, Fixtures: []Fixture{{Name: "fixture", Reference: "synthetic", Classification: "synthetic"}}, EnvironmentRequirements: []string{"isolated"}, TimeoutSeconds: 10, CostLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	scenario := v.Scenarios[0].ID
	attempt := func(ref, class string) string {
		v, err = s.AddAttempt("repo", v.ID, scenario, "agent", AttemptInput{Target: Target{Kind: "revision", Reference: ref, CommitID: ref}, Environment: Environment{Image: "image@sha256:x", DefinitionDigest: "sha256:env", OS: "linux", Architecture: "amd64", Isolation: "isolated", Network: "none"}, Commands: []string{"test"}, Classification: class, Rationale: "repeatable observation", Currency: "USD", Provenance: Provenance{RunnerID: "runner", RunnerVersion: "1", ActorKind: "agent", StartedAt: "now", CompletedAt: "now", RepetitionCount: 2}})
		if err != nil {
			t.Fatal(err)
		}
		return v.Attempts[len(v.Attempts)-1].ID
	}
	goodAttempt, badAttempt := attempt("good", "expected_behavior"), attempt("merge", "regressed_behavior")
	v, err = s.CreateSearch("repo", v.ID, "owner", SearchInput{ScenarioID: scenario, GoodKey: "good", BadKey: "merge", ConfidenceTarget: .9, Revisions: []SearchRevision{{Key: "good", Kind: "commit", Revision: "good", OwnerIDs: []string{"core"}}, {Key: "side", Kind: "package_revision", Package: "parser", Revision: "2.0.0"}, {Key: "merge", Kind: "commit", Revision: "merge", Parents: []string{"good", "side"}, DiffPaths: []string{"parser.go"}, PullIDs: []string{"pr-7"}, DecisionIDs: []string{"decision-2"}}}})
	if err != nil {
		t.Fatal(err)
	}
	search := v.Searches[0]
	classify := func(key, class string, attempts []string) {
		v, err = s.ClassifyCandidate("repo", v.ID, search.ID, "owner", CandidateClassification{RevisionKey: key, Classification: class, AttemptIDs: attempts, Rationale: "reviewed result"})
		if err != nil {
			t.Fatal(err)
		}
	}
	classify("good", "working", []string{goodAttempt})
	classify("side", "invalid", nil)
	classify("merge", "regressed", []string{badAttempt})
	search = v.Searches[0]
	if search.Verdict != "multiple_or_ambiguous" || len(search.Ranges) != 1 || !contains(search.Blockers, "merge_ancestry_requires_parent_disambiguation") {
		t.Fatalf("merge became false verdict: %#v", search)
	}
	v, err = s.AddHypothesis("repo", v.ID, search.ID, "agent", CausalHypothesis{RevisionKeys: []string{"merge"}, Body: "Merge resolution bypasses parser normalization.", EvidenceIDs: []string{badAttempt}, DiffPaths: []string{"parser.go"}, Confidence: .7, ActorKind: "agent", State: "proposed"})
	if err != nil || len(v.Searches[0].Hypotheses) != 1 {
		t.Fatalf("cited hypothesis missing: %#v %v", v.Searches[0].Hypotheses, err)
	}
	_, err = s.AddHypothesis("repo", v.ID, search.ID, "agent", CausalHypothesis{RevisionKeys: []string{"merge"}, Body: "opaque guess", ActorKind: "agent", State: "proposed"})
	if err != ErrConflict {
		t.Fatalf("uncited hypothesis accepted: %v", err)
	}
}

func contains(xs []string, wanted string) bool {
	for _, x := range xs {
		if x == wanted {
			return true
		}
	}
	return false
}

func TestOwnerResponseCarriesEvidenceIntoOrdinaryWork(t *testing.T) {
	s, _ := New(t.TempDir())
	v, _ := s.Create("repo", "owner", Input{Title: "regression", Scope: Scope{ExpectedBehavior: "works", RegressedBehavior: "fails", AcceptanceCriteria: []string{"works again"}}})
	v, _ = s.CreateScenario("repo", v.ID, "agent", true, ScenarioDefinition{Title: "repro", Commands: []string{"test"}, Fixtures: []Fixture{{Name: "fixture", Reference: "synthetic", Classification: "synthetic"}}, EnvironmentRequirements: []string{"isolated"}, TimeoutSeconds: 10, CostLimit: 1})
	scenario := v.Scenarios[0].ID
	attempt := func(ref, class string) string {
		v, _ = s.AddAttempt("repo", v.ID, scenario, "agent", AttemptInput{Target: Target{Kind: "revision", Reference: ref, CommitID: ref}, Environment: Environment{Image: "image", DefinitionDigest: "sha256:env", OS: "linux", Architecture: "amd64", Isolation: "isolated", Network: "none"}, Commands: []string{"test"}, Classification: class, Rationale: "repeatable", Currency: "USD", Provenance: Provenance{RunnerID: "runner", RunnerVersion: "1", ActorKind: "agent", StartedAt: "now", CompletedAt: "now", RepetitionCount: 1}})
		return v.Attempts[len(v.Attempts)-1].ID
	}
	goodEvidence, badEvidence := attempt("good", "expected_behavior"), attempt("bad", "regressed_behavior")
	v, _ = s.CreateSearch("repo", v.ID, "owner", SearchInput{ScenarioID: scenario, GoodKey: "good", BadKey: "bad", ConfidenceTarget: .9, Revisions: []SearchRevision{{Key: "good", Kind: "commit", Revision: "good"}, {Key: "bad", Kind: "commit", Revision: "bad", Parents: []string{"good"}}}})
	search := v.Searches[0].ID
	v, _ = s.ClassifyCandidate("repo", v.ID, search, "owner", CandidateClassification{RevisionKey: "good", AttemptIDs: []string{goodEvidence}, Classification: "working", Rationale: "passing"})
	v, _ = s.ClassifyCandidate("repo", v.ID, search, "owner", CandidateClassification{RevisionKey: "bad", AttemptIDs: []string{badEvidence}, Classification: "regressed", Rationale: "failing"})
	options := []ResponseOption{}
	for _, kind := range []string{"revert", "containment", "dependency_adjustment", "forward_repair"} {
		options = append(options, ResponseOption{ID: kind, Kind: kind, Title: kind, Summary: "candidate response", Tradeoffs: []string{"preserves explicit tradeoff"}, AffectedReleases: []string{"v2"}, AffectedWork: []string{"pull:next"}, BackportTargets: []string{"v1"}, EvidenceIDs: []string{badEvidence}})
	}
	v, err := s.CreateResponse("repo", v.ID, "owner", ResponsePlanInput{SearchID: search, CulpritGoodKey: "good", CulpritBadKey: "bad", ReproductionIDs: []string{badEvidence}, Constraints: []string{"keep new API"}, AcceptanceCriteria: []string{"scenario passes"}, OriginalIntent: "support the new API", OriginalAuthorIDs: []string{"author"}, Options: options, SelectedOptionID: "forward_repair", Rationale: "preserves valid behavior"})
	if err != nil || len(v.Responses) != 1 {
		t.Fatalf("response missing: %#v %v", v.Responses, err)
	}
	v, err = s.AddResponseWork("repo", v.ID, v.Responses[0].ID, "owner", ResponseWork{Kind: "session", ResourceID: "session:repair", OwnerID: "agent", OwnerKind: "agent", OptionID: "forward_repair", Published: true, PullRequestID: "pull:repair"})
	w := v.Responses[0].Work[0]
	if err != nil || w.Intent != "support the new API" || len(w.CulpritRange) != 2 || len(w.ReproductionIDs) != 1 || w.PullRequestID != "pull:repair" {
		t.Fatalf("preloaded work incomplete: %#v %v", w, err)
	}
}

func TestCorrectionProofPreservesIntentAndReopensOnProductionDisagreement(t *testing.T) {
	s, _ := New(t.TempDir())
	v, _ := s.Create("repo", "owner", Input{Title: "regression", Scope: Scope{ExpectedBehavior: "old and new documents parse", RegressedBehavior: "old documents fail", AcceptanceCriteria: []string{"old documents parse"}}})
	v, _ = s.CreateScenario("repo", v.ID, "agent", true, ScenarioDefinition{Title: "historical document", Commands: []string{"test historical"}, Fixtures: []Fixture{{Name: "old document", Reference: "synthetic", Classification: "synthetic"}}, EnvironmentRequirements: []string{"isolated"}, TimeoutSeconds: 10, CostLimit: 1})
	scenario := v.Scenarios[0].ID
	addAttempt := func(ref, class string) string {
		v, _ = s.AddAttempt("repo", v.ID, scenario, "agent", AttemptInput{Target: Target{Kind: "revision", Reference: ref, CommitID: ref}, Environment: Environment{Image: "image", DefinitionDigest: "sha256:env", OS: "linux", Architecture: "amd64", Isolation: "isolated", Network: "none"}, Commands: []string{"test historical"}, Classification: class, Rationale: "repeatable", Currency: "USD", Provenance: Provenance{RunnerID: "runner", RunnerVersion: "1", ActorKind: "agent", StartedAt: "now", CompletedAt: "now", RepetitionCount: 1}})
		return v.Attempts[len(v.Attempts)-1].ID
	}
	good, bad, repaired := addAttempt("good", "expected_behavior"), addAttempt("bad", "regressed_behavior"), addAttempt("repair", "expected_behavior")
	v, _ = s.CreateSearch("repo", v.ID, "owner", SearchInput{ScenarioID: scenario, GoodKey: "good", BadKey: "bad", ConfidenceTarget: .9, Revisions: []SearchRevision{{Key: "good", Kind: "commit", Revision: "good"}, {Key: "bad", Kind: "commit", Revision: "bad", Parents: []string{"good"}}}})
	search := v.Searches[0].ID
	v, _ = s.ClassifyCandidate("repo", v.ID, search, "owner", CandidateClassification{RevisionKey: "good", AttemptIDs: []string{good}, Classification: "working", Rationale: "works"})
	v, _ = s.ClassifyCandidate("repo", v.ID, search, "owner", CandidateClassification{RevisionKey: "bad", AttemptIDs: []string{bad}, Classification: "regressed", Rationale: "fails"})
	options := []ResponseOption{}
	for _, kind := range []string{"revert", "containment", "dependency_adjustment", "forward_repair"} {
		options = append(options, ResponseOption{ID: kind, Kind: kind, Title: kind, Summary: "response", Tradeoffs: []string{"reviewed"}, AffectedReleases: []string{"v2"}, AffectedWork: []string{"next"}, BackportTargets: []string{"v1"}, EvidenceIDs: []string{bad}})
	}
	v, _ = s.CreateResponse("repo", v.ID, "owner", ResponsePlanInput{SearchID: search, CulpritGoodKey: "good", CulpritBadKey: "bad", ReproductionIDs: []string{bad}, Constraints: []string{"retain new syntax"}, AcceptanceCriteria: []string{"old documents parse"}, OriginalIntent: "add new document syntax", OriginalAuthorIDs: []string{"author"}, Options: options, SelectedOptionID: "forward_repair", Rationale: "preserves both"})
	response := v.Responses[0].ID
	v, _ = s.AddResponseWork("repo", v.ID, response, "owner", ResponseWork{Kind: "task", ResourceID: "task-1", OwnerID: "agent", OwnerKind: "agent", OptionID: "forward_repair"})
	work := v.Responses[0].Work[0].ID
	v, err := s.CreateCorrection("repo", v.ID, "owner", CorrectionCandidateInput{ResponseID: response, WorkID: work, Kind: "repair", Target: Target{Kind: "revision", Reference: "repair", CommitID: "repair"}, ScenarioID: scenario, AffectedChecks: []string{"parser"}, RequirementIDs: []string{"req-compat"}, ChangeCriteria: []string{"new documents parse"}, QualityPlanID: "quality-1", RequiredCheckName: "historical-document"})
	if err != nil {
		t.Fatal(err)
	}
	candidate := v.Corrections[0].ID
	checks := []ProofCheck{{Name: "parser", Kind: "check", Status: "passed"}, {Name: "req-compat", Kind: "requirement", Status: "passed"}, {Name: "old documents parse", Kind: "regression_criterion", Status: "passed"}, {Name: "new documents parse", Kind: "change_criterion", Status: "passed"}}
	partial := append([]ProofCheck{}, checks...)
	partial[3].Status = "failed"
	v, err = s.AddCorrectionProof("repo", v.ID, candidate, "agent", CorrectionProof{ScenarioAttemptID: repaired, BaselineAttemptIDs: []string{good, bad}, Checks: partial, Revision: "repair"})
	if err != nil || v.Corrections[0].State != "awaiting_proof" || !contains(v.Corrections[0].Blockers, "partial_correction") {
		t.Fatalf("partial correction became proof: %#v %v", v.Corrections[0], err)
	}
	v, err = s.AddCorrectionProof("repo", v.ID, candidate, "agent", CorrectionProof{ScenarioAttemptID: repaired, BaselineAttemptIDs: []string{good, bad}, Checks: checks, Revision: "repair"})
	if err != nil || v.Corrections[0].State != "verified" || v.Corrections[0].OriginalIntent != "add new document syntax" {
		t.Fatalf("proof incomplete: %#v %v", v.Corrections[0], err)
	}
	for _, kind := range []string{"review", "merge", "release", "deployment"} {
		v, err = s.AddCorrectionDelivery("repo", v.ID, candidate, "owner", DeliveryEvent{Kind: kind, ResourceID: kind + "-1", Revision: "repair", Status: "passed", Summary: kind + " retained exact correction provenance"})
		if err != nil {
			t.Fatal(err)
		}
	}
	v, err = s.AddCorrectionDelivery("repo", v.ID, candidate, "owner", DeliveryEvent{Kind: "outcome", ResourceID: "signal-0", Revision: "release-repair", Status: "passed", Summary: "supported environments agree"})
	if err != nil || v.Corrections[0].State != "observed" {
		t.Fatalf("complete delivery was not observed: %#v %v", v.Corrections[0], err)
	}
	v, err = s.SetStatus("repo", v.ID, "owner", "closed", "all supported corrections observed")
	if err != nil || v.Status != "closed" {
		t.Fatalf("observed correction could not close: %#v %v", v, err)
	}
	v, err = s.AddCorrectionDelivery("repo", v.ID, candidate, "owner", DeliveryEvent{Kind: "outcome", ResourceID: "signal-1", Revision: "release-repair", Status: "disagreed", Summary: "supported environment still rejects old documents"})
	if err != nil || v.Corrections[0].State != "reopened" || v.Status != "open" {
		t.Fatalf("production disagreement did not reopen: %#v %v", v.Corrections[0], err)
	}
}
