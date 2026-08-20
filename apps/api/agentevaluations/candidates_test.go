package agentevaluations

import "testing"

func candidateInput(revision, prompt, baseline string) CandidateInput {
	return CandidateInput{PullRequestID: "pull-1", Revision: revision, AgentProjectID: "contract", AgentProjectVersion: 1, Suites: []SuiteSelection{{SuiteID: "suite", SuiteVersion: 1, ScenarioIDs: []string{"repair"}}}, Inputs: []BoundInput{{Key: "prompt:system", Revision: prompt}, {Key: "tool:shell", Revision: "tool-1"}, {Key: "scenario:suite:repair", Revision: "scenario-1"}}, ChangeReason: "measure candidate", BaselineCandidateID: baseline}
}

func TestCandidateComparisonRetainsLimitsAndVisibleRisk(t *testing.T) {
	s, _ := New(t.TempDir())
	base, err := s.CreateCandidate("repo", "owner", candidateInput("rev-1", "prompt-1", ""))
	if err != nil {
		t.Fatal(err)
	}
	base, err = s.RecordCandidateAttempt("repo", base.ID, "runner", CandidateAttemptInput{InputKeys: []string{"tool:shell", "scenario:suite:repair"}, Environment: "networkless", SimulatedServices: []string{"issues"}, Samples: []MetricSample{{ScenarioID: "repair", TaskSuccess: .5, PolicyAdherence: 1, HumanCorrections: 2, Uncertainty: .4, LatencyMS: 100, Cost: 1}, {ScenarioID: "repair", TaskSuccess: .7, PolicyAdherence: 1, HumanCorrections: 1, Uncertainty: .2, LatencyMS: 120, Cost: 1.2}}, Traces: []Trace{{ScenarioID: "repair", Kind: "tool", Summary: "bounded repair"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !base.Attempts[0].Authority.Isolated || !base.Attempts[0].Nondeterministic || len(base.Attempts[0].Statistics) != 6 {
		t.Fatalf("missing isolation/statistical limits: %+v", base.Attempts[0])
	}
	candidate, err := s.CreateCandidate("repo", "owner", candidateInput("rev-2", "prompt-2", base.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidate.Attempts) != 1 || candidate.Attempts[0].ReusedFrom != base.ID {
		t.Fatalf("unaffected evidence was not retained: %+v", candidate.Attempts)
	}
	candidate, _ = s.RecordCandidateAttempt("repo", candidate.ID, "runner", CandidateAttemptInput{InputKeys: []string{"prompt:system", "scenario:suite:repair"}, Environment: "networkless", Samples: []MetricSample{{ScenarioID: "repair", TaskSuccess: .9, PolicyAdherence: 1, HumanCorrections: 0, Uncertainty: .1, LatencyMS: 80, Cost: .8}}, ContaminationReasons: []string{"scenario answer appeared in context"}})
	if !candidate.Attempts[1].Contaminated {
		t.Fatal("contamination was hidden")
	}
	comparison, err := s.CompareCandidates("repo", base.ID, candidate.ID)
	if err != nil || !comparison.Comparable || !comparison.Nondeterminism || len(comparison.Deltas) != 6 {
		t.Fatalf("comparison missing dimensions: %v %+v", err, comparison)
	}
}

func TestCandidateSelectiveInvalidationDropsAffectedAttempt(t *testing.T) {
	s, _ := New(t.TempDir())
	base, _ := s.CreateCandidate("repo", "owner", candidateInput("rev-1", "prompt-1", ""))
	base, _ = s.RecordCandidateAttempt("repo", base.ID, "runner", CandidateAttemptInput{InputKeys: []string{"prompt:system"}, Environment: "isolated", Samples: []MetricSample{{TaskSuccess: 1}}})
	next, _ := s.CreateCandidate("repo", "owner", candidateInput("rev-2", "prompt-2", base.ID))
	if len(next.Attempts) != 0 {
		t.Fatalf("changed prompt evidence remained current: %+v", next.Attempts)
	}
}
