package infrastructureplans

import (
	"sort"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/infrastructurestate"
)

// ApplyVerification distinguishes provider completion from proven convergence.
// Every attempt remains append-only, including partial and failed attempts.
type ApplyVerification struct {
	ID             string               `json:"id"`
	ObservationIDs []string             `json:"observation_ids"`
	Resources      []ResourceComparison `json:"resources"`
	Measures       []OutcomeMeasure     `json:"measures"`
	State          string               `json:"state"`
	Converged      bool                 `json:"converged"`
	Blockers       []string             `json:"blockers"`
	VerifiedByID   string               `json:"verified_by_id"`
	CreatedAt      time.Time            `json:"created_at"`
	NonAuthority   []string             `json:"non_authority"`
}

type ResourceComparison struct {
	ResourceID        string `json:"resource_id"`
	ObservationID     string `json:"observation_id"`
	ExpectedAction    string `json:"expected_action"`
	ObservedState     string `json:"observed_state"`
	Status            string `json:"status"`
	EvidenceReference string `json:"evidence_reference"`
	Detail            string `json:"detail"`
}

type OutcomeMeasure struct {
	Kind              string `json:"kind"`
	Status            string `json:"status"`
	EvidenceReference string `json:"evidence_reference"`
	Detail            string `json:"detail"`
}

type VerificationInput struct {
	ObservationIDs []string             `json:"observation_ids"`
	Resources      []ResourceComparison `json:"resources"`
	Measures       []OutcomeMeasure     `json:"measures"`
}

// DriftAssessment is a permission-bounded observation of divergence, not a
// provider mutation. Actions link divergence to governed human work.
type DriftAssessment struct {
	ID               string         `json:"id"`
	ObservationIDs   []string       `json:"observation_ids"`
	State            string         `json:"state"`
	Findings         []DriftFinding `json:"findings"`
	CredentialStatus string         `json:"credential_status"`
	Actions          []DriftAction  `json:"actions"`
	ObservedByID     string         `json:"observed_by_id"`
	CreatedAt        time.Time      `json:"created_at"`
	NonAuthority     []string       `json:"non_authority"`
}

type DriftFinding struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Status     string `json:"status"`
	Cause      string `json:"cause,omitempty"`
	Evidence   string `json:"evidence"`
	Detail     string `json:"detail"`
}

type DriftInput struct {
	ObservationIDs []string       `json:"observation_ids"`
	Findings       []DriftFinding `json:"findings"`
}

type DriftAction struct {
	ID                string    `json:"id"`
	Kind              string    `json:"kind"`
	OwnerKind         string    `json:"owner_kind"`
	OwnerID           string    `json:"owner_id"`
	Reference         string    `json:"reference"`
	SourceRevision    string    `json:"source_revision,omitempty"`
	EnvironmentPolicy string    `json:"environment_policy,omitempty"`
	State             string    `json:"state"`
	Rationale         string    `json:"rationale"`
	CreatedByID       string    `json:"created_by_id"`
	CreatedAt         time.Time `json:"created_at"`
	NonAuthority      []string  `json:"non_authority"`
}

type DriftActionInput struct {
	Kind              string `json:"kind"`
	OwnerKind         string `json:"owner_kind"`
	OwnerID           string `json:"owner_id"`
	Reference         string `json:"reference"`
	SourceRevision    string `json:"source_revision"`
	EnvironmentPolicy string `json:"environment_policy"`
	Rationale         string `json:"rationale"`
}

func observationsForPlan(defs Definitions, repo string, refs []DefinitionRef, ids []string) (map[string]infrastructurestate.Observation, bool) {
	wanted := map[string]bool{}
	for _, value := range ids {
		if value == "" || wanted[value] {
			return nil, false
		}
		wanted[value] = true
	}
	if len(wanted) == 0 {
		return nil, false
	}
	found := map[string]infrastructurestate.Observation{}
	for _, ref := range refs {
		d, err := defs.Get(repo, ref.ID)
		if err != nil {
			return nil, false
		}
		for _, o := range d.Observations {
			if wanted[o.ID] && o.DefinitionVersion == ref.Version {
				found[o.ID] = o
			}
		}
	}
	return found, len(found) == len(wanted)
}

func (s *Store) VerifyExecution(repo, pull, plan, execution, actor string, in VerificationInput) (Plan, error) {
	if actor == "" || len(in.Resources) == 0 || len(in.Measures) == 0 {
		return Plan{}, ErrInvalid
	}
	return s.mutate(repo, pull, plan, func(p *Plan) error {
		observations, ok := observationsForPlan(s.definitions, repo, p.Input.Definitions, in.ObservationIDs)
		if !ok {
			return ErrInvalid
		}
		var x *Execution
		for i := range p.Executions {
			if p.Executions[i].ID == execution {
				x = &p.Executions[i]
			}
		}
		if x == nil {
			return ErrNotFound
		}
		if x.State != "succeeded" && x.State != "verified" && x.State != "diverged" {
			return ErrInvalid
		}
		steps := map[string]string{}
		for _, st := range x.Steps {
			steps[st.ResourceID] = st.Action
		}
		seen := map[string]bool{}
		blockers := []string{}
		for i := range in.Resources {
			r := &in.Resources[i]
			expected, exists := steps[r.ResourceID]
			if !exists || seen[r.ResourceID] || observations[r.ObservationID].ID == "" || r.ExpectedAction != expected || !map[string]bool{"matched": true, "partial": true, "failed": true}[r.Status] || r.ObservedState == "" || r.EvidenceReference == "" || r.Detail == "" || secretShaped(r.ObservedState+r.EvidenceReference+r.Detail) {
				return ErrInvalid
			}
			o := observations[r.ObservationID]
			if o.EnvironmentID != x.EnvironmentID || o.ObservedAt.Before(*x.CompletedAt) {
				return ErrInvalid
			}
			observed, configuration := false, ""
			for _, resource := range o.Resources {
				if resource.ResourceID == r.ResourceID {
					observed, configuration = true, resource.ConfigurationState
				}
			}
			matches := o.ProviderAccessible && ((expected == "destroy" && !observed) || (expected != "destroy" && observed && configuration == "matching"))
			if r.Status == "matched" && !matches {
				return ErrInvalid
			}
			seen[r.ResourceID] = true
			if r.Status != "matched" {
				blockers = append(blockers, "resource_"+r.Status+":"+r.ResourceID)
			}
		}
		for id := range steps {
			if !seen[id] {
				blockers = append(blockers, "resource_unverified:"+id)
			}
		}
		required := map[string]bool{"service": false, "security": false, "privacy": false, "cost": false, "continuity": false}
		for _, m := range in.Measures {
			if _, exists := required[m.Kind]; !exists || required[m.Kind] || !map[string]bool{"passed": true, "partial": true, "failed": true}[m.Status] || m.EvidenceReference == "" || m.Detail == "" || secretShaped(m.EvidenceReference+m.Detail) {
				return ErrInvalid
			}
			required[m.Kind] = true
			if m.Status != "passed" {
				blockers = append(blockers, m.Kind+"_"+m.Status)
			}
		}
		for kind, present := range required {
			if !present {
				blockers = append(blockers, kind+"_unverified")
			}
		}
		sort.Strings(blockers)
		now := s.now().UTC()
		state := "converged"
		if len(blockers) > 0 {
			state = "not_converged"
		}
		v := ApplyVerification{ID: id(), ObservationIDs: append([]string{}, in.ObservationIDs...), Resources: in.Resources, Measures: in.Measures, State: state, Converged: len(blockers) == 0, Blockers: blockers, VerifiedByID: actor, CreatedAt: now, NonAuthority: []string{"verification is retained evidence and grants no provider, deployment, credential, policy, or environment authority"}}
		x.Verifications = append(x.Verifications, v)
		if v.Converged {
			x.State = "verified"
		} else {
			x.State = "diverged"
		}
		x.event("verification_"+state, actor, "", "post-apply outcomes compared with reviewed intent", now)
		return nil
	})
}

func (s *Store) MonitorExecution(repo, pull, plan, execution, actor string, in DriftInput) (Plan, error) {
	if actor == "" {
		return Plan{}, ErrInvalid
	}
	return s.mutate(repo, pull, plan, func(p *Plan) error {
		observations, ok := observationsForPlan(s.definitions, repo, p.Input.Definitions, in.ObservationIDs)
		if !ok {
			return ErrInvalid
		}
		var x *Execution
		for i := range p.Executions {
			if p.Executions[i].ID == execution {
				x = &p.Executions[i]
			}
		}
		if x == nil {
			return ErrNotFound
		}
		if len(x.Verifications) == 0 {
			return ErrInvalid
		}
		for _, f := range in.Findings {
			if !map[string]bool{"configuration_drift": true, "unmanaged_change": true, "failed_cleanup": true, "provider_loss": true}[f.Kind] || !map[string]bool{"open": true, "resolved": true, "unknown": true}[f.Status] || f.Evidence == "" || f.Detail == "" || secretShaped(f.Cause+f.Evidence+f.Detail) {
				return ErrInvalid
			}
		}
		findings := append([]DriftFinding{}, in.Findings...)
		steps := map[string]string{}
		for _, step := range x.Steps {
			steps[step.ResourceID] = step.Action
		}
		for _, observation := range observations {
			if observation.EnvironmentID != x.EnvironmentID {
				return ErrInvalid
			}
			if !observation.ProviderAccessible {
				findings = append(findings, DriftFinding{Kind: "provider_loss", Status: "open", Evidence: observation.EvidenceReference, Detail: observation.Provider + " was inaccessible"})
			}
			seen := map[string]bool{}
			for _, resource := range observation.Resources {
				if resource.ResourceID == "" {
					findings = append(findings, DriftFinding{Kind: "unmanaged_change", Status: "open", Evidence: observation.EvidenceReference, Detail: resource.ProviderResource + " is not declared"})
					continue
				}
				seen[resource.ResourceID] = true
				if resource.ConfigurationState == "drifted" {
					findings = append(findings, DriftFinding{Kind: "configuration_drift", ResourceID: resource.ResourceID, Status: "open", Evidence: observation.EvidenceReference, Detail: "observed configuration differs from the reviewed definition"})
				}
			}
			for resource, action := range steps {
				if action == "destroy" && seen[resource] {
					findings = append(findings, DriftFinding{Kind: "failed_cleanup", ResourceID: resource, Status: "open", Evidence: observation.EvidenceReference, Detail: "destroyed resource remains observable"})
				}
			}
		}
		credential := "valid"
		now := s.now().UTC()
		if !x.Credential.ExpiresAt.After(now) {
			credential = "expired"
		} else if x.Credential.ExpiresAt.Before(now.Add(2 * time.Hour)) {
			credential = "expiring"
		}
		state := "matching"
		for _, f := range findings {
			if f.Status != "resolved" {
				state = "drifted"
			}
		}
		if credential != "valid" {
			state = "drifted"
		}
		m := DriftAssessment{ID: id(), ObservationIDs: append([]string{}, in.ObservationIDs...), State: state, Findings: findings, CredentialStatus: credential, Actions: []DriftAction{}, ObservedByID: actor, CreatedAt: now, NonAuthority: []string{"monitoring is permission-bounded evidence and grants no provider, credential, deployment, exception, incident, review, or repair authority"}}
		x.Monitoring = append(x.Monitoring, m)
		if state == "drifted" {
			x.State = "diverged"
		}
		x.event("monitoring_"+state, actor, "", "declared and observed infrastructure compared", now)
		return nil
	})
}

func (s *Store) OpenDriftAction(repo, pull, plan, execution, assessment, actor string, in DriftActionInput) (Plan, error) {
	if actor == "" || !map[string]bool{"incident": true, "exception": true, "repair": true, "adopt": true, "restore": true}[in.Kind] || !map[string]bool{"human": true, "agent": true}[in.OwnerKind] || in.OwnerID == "" || in.Reference == "" || in.Rationale == "" || secretShaped(in.Reference+in.Rationale+in.EnvironmentPolicy) {
		return Plan{}, ErrInvalid
	}
	if in.Kind == "adopt" && (in.SourceRevision == "" || !strings.HasPrefix(in.Reference, "pull:")) {
		return Plan{}, ErrInvalid
	}
	if in.Kind == "restore" && in.EnvironmentPolicy == "" {
		return Plan{}, ErrInvalid
	}
	return s.mutateExecution(repo, pull, plan, execution, func(x *Execution) error {
		for i := range x.Monitoring {
			if x.Monitoring[i].ID == assessment {
				if x.Monitoring[i].State != "drifted" {
					return ErrInvalid
				}
				now := s.now().UTC()
				state := "open"
				if in.Kind == "adopt" {
					state = "pending_review"
				}
				if in.Kind == "restore" {
					state = "pending_environment_approval"
				}
				a := DriftAction{ID: id(), Kind: in.Kind, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, Reference: in.Reference, SourceRevision: in.SourceRevision, EnvironmentPolicy: in.EnvironmentPolicy, State: state, Rationale: in.Rationale, CreatedByID: actor, CreatedAt: now, NonAuthority: []string{"action does not rewrite external change or grant review, merge, provider, credential, deployment, exception, incident, or environment authority"}}
				x.Monitoring[i].Actions = append(x.Monitoring[i].Actions, a)
				x.event("drift_action_"+in.Kind, actor, "", strings.ReplaceAll(in.Kind, "_", " ")+" linked to accountable work", now)
				return nil
			}
		}
		return ErrNotFound
	})
}
