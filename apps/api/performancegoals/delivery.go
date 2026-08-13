package performancegoals

import (
	"path"
	"strings"
	"time"
)

type RegressionThreshold struct {
	MetricID                 string  `json:"metric_id"`
	MaximumPercentRegression float64 `json:"maximum_percent_regression"`
	RequireConfidence        bool    `json:"require_confidence"`
}
type DeliveryPolicy struct {
	ID          string                `json:"id"`
	GoalVersion int64                 `json:"goal_version"`
	Branch      string                `json:"branch"`
	Paths       []string              `json:"paths"`
	RiskClasses []string              `json:"risk_classes"`
	Thresholds  []RegressionThreshold `json:"thresholds"`
	ActorID     string                `json:"actor_id"`
	CreatedAt   time.Time             `json:"created_at"`
}
type DeliveryObservation struct {
	ID                 string    `json:"id"`
	GoalVersion        int64     `json:"goal_version"`
	ComparisonID       string    `json:"comparison_id"`
	ReleaseID          string    `json:"release_id"`
	DeploymentID       string    `json:"deployment_id"`
	Stage              string    `json:"stage"`
	Revision           string    `json:"revision"`
	MetricID           string    `json:"metric_id"`
	Value              float64   `json:"value"`
	EnvironmentDigest  string    `json:"environment_digest"`
	Health             string    `json:"health"`
	Assumptions        []string  `json:"assumptions"`
	Uncertainty        string    `json:"uncertainty,omitempty"`
	Outcome            string    `json:"outcome"`
	Action             string    `json:"action,omitempty"`
	LinkedResourceKind string    `json:"linked_resource_kind,omitempty"`
	LinkedResourceID   string    `json:"linked_resource_id,omitempty"`
	ActorID            string    `json:"actor_id"`
	CreatedAt          time.Time `json:"created_at"`
}
type DeliveryPolicyInput struct {
	Branch      string                `json:"branch"`
	Paths       []string              `json:"paths"`
	RiskClasses []string              `json:"risk_classes"`
	Thresholds  []RegressionThreshold `json:"thresholds"`
}
type DeliveryObservationInput struct {
	GoalVersion        int64    `json:"goal_version"`
	ComparisonID       string   `json:"comparison_id"`
	ReleaseID          string   `json:"release_id"`
	DeploymentID       string   `json:"deployment_id"`
	Stage              string   `json:"stage"`
	Revision           string   `json:"revision"`
	MetricID           string   `json:"metric_id"`
	Value              float64  `json:"value"`
	EnvironmentDigest  string   `json:"environment_digest"`
	Health             string   `json:"health"`
	Assumptions        []string `json:"assumptions"`
	Uncertainty        string   `json:"uncertainty"`
	Action             string   `json:"action"`
	LinkedResourceKind string   `json:"linked_resource_kind"`
	LinkedResourceID   string   `json:"linked_resource_id"`
}
type DeliveryRequirement struct {
	GoalID       string `json:"goal_id"`
	PolicyID     string `json:"policy_id"`
	MetricID     string `json:"metric_id"`
	Status       string `json:"status"`
	Detail       string `json:"detail"`
	ComparisonID string `json:"comparison_id,omitempty"`
}

func (s *Store) PutDeliveryPolicy(repo, gid, actor string, in DeliveryPolicyInput) (Goal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, e := s.read(repo, gid)
	if e != nil {
		return g, e
	}
	if actor == "" || in.Branch == "" || len(in.Thresholds) == 0 {
		return g, ErrInvalid
	}
	metrics := map[string]bool{}
	for _, m := range g.Versions[len(g.Versions)-1].Metrics {
		metrics[m.ID] = true
	}
	for _, t := range in.Thresholds {
		if !metrics[t.MetricID] || t.MaximumPercentRegression < 0 {
			return g, ErrInvalid
		}
	}
	for _, p := range in.Paths {
		if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
			return g, ErrInvalid
		}
	}
	p := DeliveryPolicy{ID: id(), GoalVersion: g.CurrentVersion, Branch: in.Branch, Paths: in.Paths, RiskClasses: in.RiskClasses, Thresholds: in.Thresholds, ActorID: actor, CreatedAt: s.now().UTC()}
	g.Policies = append(g.Policies, p)
	return g, s.write(g)
}
func policyApplies(p DeliveryPolicy, branch string, paths, risks []string) bool {
	if p.Branch != branch {
		return false
	}
	if len(p.Paths) == 0 && len(p.RiskClasses) == 0 {
		return true
	}
	for _, want := range p.Paths {
		for _, got := range paths {
			if ok, _ := path.Match(want, got); ok || strings.HasPrefix(got, strings.TrimSuffix(want, "*")) {
				return true
			}
		}
	}
	for _, want := range p.RiskClasses {
		for _, got := range risks {
			if want == got {
				return true
			}
		}
	}
	return false
}
func (s *Store) AssessDelivery(repo, pull, revision, branch string, paths, risks []string) ([]DeliveryRequirement, error) {
	goals, e := s.List(repo)
	if e != nil {
		return nil, e
	}
	out := []DeliveryRequirement{}
	for _, g := range goals {
		for _, p := range g.Policies {
			if p.GoalVersion != g.CurrentVersion || !policyApplies(p, branch, paths, risks) {
				continue
			}
			for _, t := range p.Thresholds {
				r := DeliveryRequirement{GoalID: g.ID, PolicyID: p.ID, MetricID: t.MetricID, Status: "missing", Detail: "No current exact-revision comparison satisfies this performance policy."}
				for i := len(g.Comparisons) - 1; i >= 0; i-- {
					c := g.Comparisons[i]
					if c.Version == g.CurrentVersion && c.PullRequestID == pull && c.MetricID == t.MetricID {
						r.ComparisonID = c.ID
						candidateRevision := ""
						for _, trial := range g.Trials {
							if trial.ID == c.CandidateTrialID {
								candidateRevision = trial.Revision
							}
						}
						if candidateRevision != revision {
							r.Status = "stale"
							r.Detail = "Comparison evidence does not describe the current pull request revision."
							break
						}
						r.Status = "satisfied"
						r.Detail = "Current candidate comparison is within the allowed regression threshold."
						if c.PercentChange > t.MaximumPercentRegression {
							r.Status = "regressed"
							r.Detail = "Candidate exceeds the allowed regression threshold."
						}
						if t.RequireConfidence && c.Confidence95.Maximum != nil && *c.Confidence95.Maximum > t.MaximumPercentRegression {
							r.Status = "uncertain"
							r.Detail = "Confidence interval does not exclude a disallowed regression."
						}
						if len(c.CorrectnessFailures) > 0 {
							r.Status = "correctness_failed"
							r.Detail = "Candidate violates one or more declared correctness constraints."
						}
						break
					}
				}
				out = append(out, r)
			}
		}
	}
	return out, nil
}
func (s *Store) ObserveDelivery(repo, gid, actor string, in DeliveryObservationInput) (Goal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, e := s.read(repo, gid)
	if e != nil {
		return g, e
	}
	if actor == "" || in.GoalVersion != g.CurrentVersion || in.ComparisonID == "" || in.ReleaseID == "" || in.DeploymentID == "" || in.Stage == "" || in.Revision == "" || in.MetricID == "" || in.EnvironmentDigest == "" {
		return g, ErrInvalid
	}
	var c *Comparison
	for i := range g.Comparisons {
		if g.Comparisons[i].ID == in.ComparisonID {
			c = &g.Comparisons[i]
		}
	}
	if c == nil || c.Version != in.GoalVersion || c.MetricID != in.MetricID {
		return g, ErrInvalid
	}
	v := g.Versions[len(g.Versions)-1]
	var m *Metric
	for i := range v.Metrics {
		if v.Metrics[i].ID == in.MetricID {
			m = &v.Metrics[i]
		}
	}
	if m == nil {
		return g, ErrInvalid
	}
	outcome := "healthy"
	if in.Health != "passing" || in.Uncertainty != "" || in.EnvironmentDigest != m.EnvironmentDigest {
		outcome = "uncertain"
	}
	if (m.Target.Maximum != nil && in.Value > *m.Target.Maximum) || (m.Target.Minimum != nil && in.Value < *m.Target.Minimum) {
		outcome = "regressed"
	}
	if outcome != "healthy" && in.Action != "pause" && in.Action != "restore" && in.Action != "repair" && in.Action != "decision_revisit" {
		return g, ErrInvalid
	}
	if (in.Action == "repair" && in.LinkedResourceKind != "issue") || (in.Action == "decision_revisit" && in.LinkedResourceKind != "decision") {
		return g, ErrInvalid
	}
	o := DeliveryObservation{ID: id(), GoalVersion: in.GoalVersion, ComparisonID: in.ComparisonID, ReleaseID: in.ReleaseID, DeploymentID: in.DeploymentID, Stage: in.Stage, Revision: in.Revision, MetricID: in.MetricID, Value: in.Value, EnvironmentDigest: in.EnvironmentDigest, Health: in.Health, Assumptions: in.Assumptions, Uncertainty: in.Uncertainty, Outcome: outcome, Action: in.Action, LinkedResourceKind: in.LinkedResourceKind, LinkedResourceID: in.LinkedResourceID, ActorID: actor, CreatedAt: s.now().UTC()}
	g.Observations = append(g.Observations, o)
	return g, s.write(g)
}
