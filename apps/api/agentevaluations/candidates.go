package agentevaluations

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BoundInput is one independently invalidatable part of an assembled agent.
// Keys are stable semantic identities (prompt, tool, model, knowledge, judge,
// or scenario), while revisions are the exact content/version identities.
type BoundInput struct {
	Key      string `json:"key"`
	Revision string `json:"revision"`
}
type SuiteSelection struct {
	SuiteID      string   `json:"suite_id"`
	SuiteVersion int64    `json:"suite_version"`
	ScenarioIDs  []string `json:"scenario_ids"`
}
type CandidateInput struct {
	PullRequestID       string           `json:"pull_request_id"`
	Revision            string           `json:"revision"`
	AgentProjectID      string           `json:"agent_project_id"`
	AgentProjectVersion int64            `json:"agent_project_version"`
	Suites              []SuiteSelection `json:"suites"`
	Inputs              []BoundInput     `json:"inputs"`
	ChangeReason        string           `json:"change_reason"`
	BaselineCandidateID string           `json:"baseline_candidate_id,omitempty"`
}
type Candidate struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	CandidateInput
	Digest    string             `json:"digest"`
	CreatedBy string             `json:"created_by"`
	CreatedAt time.Time          `json:"created_at"`
	Authority Authority          `json:"authority"`
	Attempts  []CandidateAttempt `json:"attempts"`
}
type Trace struct {
	ScenarioID string `json:"scenario_id"`
	Kind       string `json:"kind"`
	Summary    string `json:"summary"`
	Digest     string `json:"digest,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}
type MetricSample struct {
	ScenarioID       string  `json:"scenario_id"`
	TaskSuccess      float64 `json:"task_success"`
	PolicyAdherence  float64 `json:"policy_adherence"`
	HumanCorrections int     `json:"human_corrections"`
	Uncertainty      float64 `json:"uncertainty"`
	LatencyMS        int64   `json:"latency_ms"`
	Cost             float64 `json:"cost"`
}
type CandidateEvaluatorDecision struct {
	DecisionInput
	Evaluator string    `json:"evaluator"`
	CreatedAt time.Time `json:"created_at"`
}
type CandidateAttemptInput struct {
	InputKeys            []string                     `json:"input_keys"`
	Environment          string                       `json:"environment"`
	SimulatedServices    []string                     `json:"simulated_services"`
	PermittedServices    []string                     `json:"permitted_services"`
	Traces               []Trace                      `json:"traces"`
	ToolActions          []ToolAction                 `json:"tool_actions"`
	Outputs              map[string]string            `json:"outputs"`
	Artifacts            []Artifact                   `json:"artifacts"`
	Samples              []MetricSample               `json:"samples"`
	EvaluatorDecisions   []CandidateEvaluatorDecision `json:"evaluator_decisions"`
	ContaminationReasons []string                     `json:"contamination_reasons"`
	ReproducibilityNotes string                       `json:"reproducibility_notes"`
}
type StatisticalLimit struct {
	Metric  string  `json:"metric"`
	Samples int     `json:"samples"`
	Mean    float64 `json:"mean"`
	Lower95 float64 `json:"lower_95"`
	Upper95 float64 `json:"upper_95"`
}
type CandidateAttempt struct {
	ID string `json:"id"`
	CandidateAttemptInput
	InputRevisions   map[string]string  `json:"input_revisions"`
	CreatedBy        string             `json:"created_by"`
	CreatedAt        time.Time          `json:"created_at"`
	Authority        Authority          `json:"authority"`
	Statistics       []StatisticalLimit `json:"statistics"`
	Contaminated     bool               `json:"contaminated"`
	Nondeterministic bool               `json:"nondeterministic"`
	ReusedFrom       string             `json:"reused_from,omitempty"`
}
type MetricDelta struct {
	Metric           string  `json:"metric"`
	Baseline         float64 `json:"baseline"`
	Candidate        float64 `json:"candidate"`
	Delta            float64 `json:"delta"`
	BaselineLower95  float64 `json:"baseline_lower_95"`
	BaselineUpper95  float64 `json:"baseline_upper_95"`
	CandidateLower95 float64 `json:"candidate_lower_95"`
	CandidateUpper95 float64 `json:"candidate_upper_95"`
}
type Comparison struct {
	BaselineCandidateID string        `json:"baseline_candidate_id"`
	CandidateID         string        `json:"candidate_id"`
	BaselineRevision    string        `json:"baseline_revision"`
	CandidateRevision   string        `json:"candidate_revision"`
	Comparable          bool          `json:"comparable"`
	Reasons             []string      `json:"reasons"`
	Deltas              []MetricDelta `json:"deltas"`
	Contamination       bool          `json:"contamination"`
	Nondeterminism      bool          `json:"nondeterminism"`
}

func candidateValid(in CandidateInput) bool {
	if in.PullRequestID == "" || in.Revision == "" || in.AgentProjectID == "" || in.AgentProjectVersion < 1 || len(in.Suites) == 0 || len(in.Inputs) == 0 || in.ChangeReason == "" {
		return false
	}
	seen := map[string]bool{}
	for _, x := range in.Inputs {
		if x.Key == "" || x.Revision == "" || seen[x.Key] {
			return false
		}
		seen[x.Key] = true
	}
	for _, x := range in.Suites {
		if x.SuiteID == "" || x.SuiteVersion < 1 || len(x.ScenarioIDs) == 0 || !validList(x.ScenarioIDs) {
			return false
		}
	}
	return true
}
func (s *Store) CreateCandidate(repo, actor string, in CandidateInput) (Candidate, error) {
	if !candidateValid(in) || repo == "" || actor == "" {
		return Candidate{}, ErrInvalid
	}
	sort.Slice(in.Inputs, func(i, j int) bool { return in.Inputs[i].Key < in.Inputs[j].Key })
	b, _ := json.Marshal(in)
	h := sha256.Sum256(b)
	x := Candidate{ID: id("aec_"), RepositoryID: repo, CandidateInput: in, Digest: hex.EncodeToString(h[:]), CreatedBy: actor, CreatedAt: s.now().UTC(), Authority: Authority{Isolated: true}}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Carry forward only attempts whose declared inputs are unchanged. A prompt
	// edit invalidates prompt-dependent evidence without discarding an attempt
	// that depended only on an unchanged tool, judge, or scenario.
	if in.BaselineCandidateID != "" {
		var prior Candidate
		if s.read("candidates", in.BaselineCandidateID, &prior) != nil || prior.RepositoryID != repo || prior.PullRequestID != in.PullRequestID {
			return Candidate{}, ErrInvalid
		}
		current := map[string]string{}
		for _, v := range in.Inputs {
			current[v.Key] = v.Revision
		}
		for _, a := range prior.Attempts {
			valid := true
			for key, revision := range a.InputRevisions {
				if current[key] != revision {
					valid = false
					break
				}
			}
			if valid {
				a.ReusedFrom = prior.ID
				x.Attempts = append(x.Attempts, a)
			}
		}
	}
	return x, s.write("candidates", x.ID, x)
}
func attemptStats(xs []MetricSample) []StatisticalLimit {
	type point struct {
		name   string
		values []float64
	}
	ps := []point{{"task_success", nil}, {"policy_adherence", nil}, {"human_corrections", nil}, {"uncertainty", nil}, {"latency_ms", nil}, {"cost", nil}}
	for _, x := range xs {
		vs := []float64{x.TaskSuccess, x.PolicyAdherence, float64(x.HumanCorrections), x.Uncertainty, float64(x.LatencyMS), x.Cost}
		for i, v := range vs {
			ps[i].values = append(ps[i].values, v)
		}
	}
	out := []StatisticalLimit{}
	for _, p := range ps {
		if len(p.values) == 0 {
			continue
		}
		mean := 0.0
		for _, v := range p.values {
			mean += v
		}
		mean /= float64(len(p.values))
		variance := 0.0
		for _, v := range p.values {
			variance += (v - mean) * (v - mean)
		}
		if len(p.values) > 1 {
			variance /= float64(len(p.values) - 1)
		}
		margin := 1.96 * math.Sqrt(variance/float64(len(p.values)))
		out = append(out, StatisticalLimit{p.name, len(p.values), mean, mean - margin, mean + margin})
	}
	return out
}
func (s *Store) RecordCandidateAttempt(repo, candidate, actor string, in CandidateAttemptInput) (Candidate, error) {
	if actor == "" || in.Environment == "" || len(in.InputKeys) == 0 || len(in.Samples) == 0 || !validList(in.InputKeys) {
		return Candidate{}, ErrInvalid
	}
	for i := range in.EvaluatorDecisions {
		if in.EvaluatorDecisions[i].Rationale == "" {
			return Candidate{}, ErrInvalid
		}
		in.EvaluatorDecisions[i].Evaluator = actor
		in.EvaluatorDecisions[i].CreatedAt = s.now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Candidate
	if s.read("candidates", candidate, &x) != nil || x.RepositoryID != repo {
		return x, ErrNotFound
	}
	bound := map[string]string{}
	for _, v := range x.Inputs {
		bound[v.Key] = v.Revision
	}
	revs := map[string]string{}
	for _, k := range in.InputKeys {
		v, ok := bound[k]
		if !ok {
			return x, ErrInvalid
		}
		revs[k] = v
	}
	stats := attemptStats(in.Samples)
	nondet := false
	for _, v := range stats {
		if v.Samples > 1 && v.Upper95-v.Lower95 > 0 {
			nondet = true
		}
	}
	a := CandidateAttempt{ID: id("aea_"), CandidateAttemptInput: in, InputRevisions: revs, CreatedBy: actor, CreatedAt: s.now().UTC(), Authority: Authority{Isolated: true}, Statistics: stats, Contaminated: len(in.ContaminationReasons) > 0, Nondeterministic: nondet}
	x.Attempts = append(x.Attempts, a)
	return x, s.write("candidates", x.ID, x)
}
func (s *Store) GetCandidate(repo, idv string) (Candidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Candidate
	if s.read("candidates", idv, &x) != nil || x.RepositoryID != repo {
		return x, ErrNotFound
	}
	return x, nil
}
func (s *Store) ListCandidates(repo, pull string) ([]Candidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, "candidates"))
	if e != nil {
		return nil, e
	}
	out := []Candidate{}
	for _, f := range es {
		var x Candidate
		if s.read("candidates", strings.TrimSuffix(f.Name(), ".json"), &x) == nil && x.RepositoryID == repo && (pull == "" || x.PullRequestID == pull) {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func latestUsable(x Candidate) (CandidateAttempt, bool) {
	for i := len(x.Attempts) - 1; i >= 0; i-- {
		if !x.Attempts[i].Contaminated {
			return x.Attempts[i], true
		}
	}
	return CandidateAttempt{}, false
}
func (s *Store) CompareCandidates(repo, baseline, candidate string) (Comparison, error) {
	b, e := s.GetCandidate(repo, baseline)
	if e != nil {
		return Comparison{}, e
	}
	c, e := s.GetCandidate(repo, candidate)
	if e != nil {
		return Comparison{}, e
	}
	out := Comparison{BaselineCandidateID: b.ID, CandidateID: c.ID, BaselineRevision: b.Revision, CandidateRevision: c.Revision, Comparable: true}
	ba, bok := latestUsable(b)
	ca, cok := latestUsable(c)
	if !bok || !cok {
		out.Comparable = false
		out.Reasons = append(out.Reasons, "both candidates require an uncontaminated attempt")
		return out, nil
	}
	bm := map[string]StatisticalLimit{}
	for _, v := range ba.Statistics {
		bm[v.Metric] = v
	}
	for _, v := range ca.Statistics {
		if p, ok := bm[v.Metric]; ok {
			out.Deltas = append(out.Deltas, MetricDelta{v.Metric, p.Mean, v.Mean, v.Mean - p.Mean, p.Lower95, p.Upper95, v.Lower95, v.Upper95})
		}
	}
	out.Contamination = ba.Contaminated || ca.Contaminated
	out.Nondeterminism = ba.Nondeterministic || ca.Nondeterministic
	if len(out.Deltas) == 0 {
		out.Comparable = false
		out.Reasons = append(out.Reasons, "no shared measured dimensions")
	}
	return out, nil
}
