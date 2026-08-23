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
