package apiconsumers

import (
	"encoding/json"
	"strings"
	"time"
)

// IntegrationWork is a credential-free, revision-exact brief for ordinary
// human or agent project work. ResourceID links the independently governed
// task, session, or workspace without granting it authority.
type IntegrationWork struct {
	ID                     string               `json:"id"`
	Kind                   string               `json:"kind"`
	OwnerKind              string               `json:"owner_kind"`
	OwnerID                string               `json:"owner_id"`
	ConsumerRepositoryID   string               `json:"consumer_repository_id"`
	ConsumerRevision       string               `json:"consumer_revision"`
	ResourceID             string               `json:"resource_id,omitempty"`
	ContractID             string               `json:"contract_id"`
	ContractVersion        int64                `json:"contract_version"`
	ContractSourceRevision string               `json:"contract_source_revision"`
	DefinitionPath         string               `json:"definition_path"`
	SDKReferences          []string             `json:"sdk_references,omitempty"`
	ExampleReferences      []string             `json:"example_references,omitempty"`
	Sandbox                SandboxConfiguration `json:"sandbox"`
	AcceptanceCriteria     []string             `json:"acceptance_criteria"`
	CreatedBy              string               `json:"created_by"`
	CreatedAt              time.Time            `json:"created_at"`
}

type SandboxConfiguration struct {
	Environments       []string `json:"environments"`
	Capabilities       []string `json:"capabilities"`
	Operations         []string `json:"operations"`
	FailureScenarios   []string `json:"failure_scenarios,omitempty"`
	SyntheticOnly      bool     `json:"synthetic_only"`
	CredentialIncluded bool     `json:"credential_included"`
}

type WorkInput struct {
	Kind                 string   `json:"kind"`
	OwnerKind            string   `json:"owner_kind"`
	OwnerID              string   `json:"owner_id"`
	ConsumerRepositoryID string   `json:"consumer_repository_id"`
	ConsumerRevision     string   `json:"consumer_revision"`
	ResourceID           string   `json:"resource_id"`
	SDKReferences        []string `json:"sdk_references"`
	ExampleReferences    []string `json:"example_references"`
	AcceptanceCriteria   []string `json:"acceptance_criteria"`
}

type EvidenceArtifact struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
}
type ScenarioResult struct {
	Name                 string             `json:"name"`
	Kind                 string             `json:"kind"`
	Status               string             `json:"status"`
	SanitizedRequest     map[string]any     `json:"sanitized_request,omitempty"`
	SanitizedResponse    map[string]any     `json:"sanitized_response,omitempty"`
	Logs                 []string           `json:"logs,omitempty"`
	Artifacts            []EvidenceArtifact `json:"artifacts,omitempty"`
	Coverage             []string           `json:"coverage,omitempty"`
	Cost                 float64            `json:"cost"`
	InaccessibleEvidence []string           `json:"inaccessible_evidence,omitempty"`
}
type Verification struct {
	ID                     string           `json:"id"`
	PullRequestID          string           `json:"pull_request_id"`
	CandidateRepositoryID  string           `json:"candidate_repository_id"`
	CandidateRevision      string           `json:"candidate_revision"`
	ContractID             string           `json:"contract_id"`
	ContractVersion        int64            `json:"contract_version"`
	ContractSourceRevision string           `json:"contract_source_revision"`
	Results                []ScenarioResult `json:"results"`
	ProducerPassed         bool             `json:"producer_passed"`
	ConsumerPassed         bool             `json:"consumer_passed"`
	Agreement              string           `json:"agreement"`
	AuthoredBy             string           `json:"authored_by"`
	CreatedAt              time.Time        `json:"created_at"`
}
type VerificationInput struct {
	PullRequestID         string           `json:"pull_request_id"`
	CandidateRepositoryID string           `json:"candidate_repository_id"`
	CandidateRevision     string           `json:"candidate_revision"`
	Results               []ScenarioResult `json:"results"`
}

func contractSnapshot(s *Store, a Application) (source, path string, ok bool) {
	c, err := s.contracts.Get(a.RepositoryID, a.Registration.ContractID)
	if err != nil {
		return "", "", false
	}
	v, found := findVersion(c, a.Registration.ContractVersion)
	if !found {
		return "", "", false
	}
	return v.SourceRevision, v.DefinitionPath, true
}
func safeStrings(xs []string) bool {
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || sensitive(x) {
			return false
		}
	}
	return true
}
func sensitive(v string) bool {
	x := strings.ToLower(v)
	for _, k := range []string{"authorization", "credential", "password", "private_key", "access_token", "vka_"} {
		if strings.Contains(x, k) {
			return true
		}
	}
	return false
}
func safeValue(v any) bool { b, err := json.Marshal(v); return err == nil && !sensitive(string(b)) }

func (s *Store) CreateWork(repo, application, actor string, writer bool, in WorkInput) (IntegrationWork, error) {
	if !map[string]bool{"task": true, "session": true, "workspace": true}[in.Kind] || !map[string]bool{"human": true, "agent": true}[in.OwnerKind] || in.OwnerID == "" || in.ConsumerRepositoryID == "" || in.ConsumerRevision == "" || !safeStrings(in.SDKReferences) || !safeStrings(in.ExampleReferences) || len(in.AcceptanceCriteria) == 0 || !safeStrings(in.AcceptanceCriteria) {
		return IntegrationWork{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return IntegrationWork{}, e
	}
	for i := range d.Applications {
		a := &d.Applications[i]
		if a.RepositoryID != repo || a.ID != application {
			continue
		}
		if !writer && a.OwnerID != actor {
			return IntegrationWork{}, ErrForbidden
		}
		source, path, ok := contractSnapshot(s, *a)
		if !ok {
			return IntegrationWork{}, ErrInvalid
		}
		ops := []string{}
		for _, o := range a.ContractOperations {
			ops = append(ops, o.ID)
		}
		failures := []string{}
		for _, f := range a.FailureRules {
			failures = append(failures, f.ID)
		}
		now := s.now().UTC()
		x := IntegrationWork{ID: ident("apiwork"), Kind: in.Kind, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, ConsumerRepositoryID: in.ConsumerRepositoryID, ConsumerRevision: in.ConsumerRevision, ResourceID: in.ResourceID, ContractID: a.Registration.ContractID, ContractVersion: a.Registration.ContractVersion, ContractSourceRevision: source, DefinitionPath: path, SDKReferences: in.SDKReferences, ExampleReferences: in.ExampleReferences, Sandbox: SandboxConfiguration{Environments: a.ApprovedEnvironments, Capabilities: a.ApprovedCapabilities, Operations: ops, FailureScenarios: failures, SyntheticOnly: true, CredentialIncluded: false}, AcceptanceCriteria: in.AcceptanceCriteria, CreatedBy: actor, CreatedAt: now}
		a.IntegrationWork = append(a.IntegrationWork, x)
		event(a, actor, "integration_work_created", x.ID, now)
		return x, s.save(d)
	}
	return IntegrationWork{}, ErrNotFound
}

func (s *Store) RecordVerification(repo, application, actor string, writer bool, in VerificationInput) (Verification, error) {
	if in.PullRequestID == "" || in.CandidateRepositoryID == "" || in.CandidateRevision == "" || len(in.Results) == 0 {
		return Verification{}, ErrInvalid
	}
	producer, consumer := false, false
	for _, r := range in.Results {
		if r.Name == "" || !map[string]bool{"producer_conformance": true, "consumer_test": true}[r.Kind] || !map[string]bool{"passed": true, "failed": true}[r.Status] || r.Cost < 0 || !safeValue(r.SanitizedRequest) || !safeValue(r.SanitizedResponse) || !safeStrings(r.Logs) || !safeStrings(r.Coverage) || !safeStrings(r.InaccessibleEvidence) {
			return Verification{}, ErrInvalid
		}
		for _, a := range r.Artifacts {
			if a.Name == "" || a.Digest == "" || a.Size < 0 || sensitive(a.Name) {
				return Verification{}, ErrInvalid
			}
		}
		if r.Status == "passed" && r.Kind == "producer_conformance" {
			producer = true
		}
		if r.Status == "passed" && r.Kind == "consumer_test" {
			consumer = true
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Verification{}, e
	}
	for i := range d.Applications {
		a := &d.Applications[i]
		if a.RepositoryID != repo || a.ID != application {
			continue
		}
		if !writer && a.OwnerID != actor {
			return Verification{}, ErrForbidden
		}
		source, _, ok := contractSnapshot(s, *a)
		if !ok {
			return Verification{}, ErrInvalid
		}
		agreement := "incomplete"
		if producer && consumer {
			agreement = "demonstrated"
		}
		now := s.now().UTC()
		x := Verification{ID: ident("apiverify"), PullRequestID: in.PullRequestID, CandidateRepositoryID: in.CandidateRepositoryID, CandidateRevision: in.CandidateRevision, ContractID: a.Registration.ContractID, ContractVersion: a.Registration.ContractVersion, ContractSourceRevision: source, Results: in.Results, ProducerPassed: producer, ConsumerPassed: consumer, Agreement: agreement, AuthoredBy: actor, CreatedAt: now}
		a.Verifications = append(a.Verifications, x)
		event(a, actor, "integration_verified", x.ID, now)
		return x, s.save(d)
	}
	return Verification{}, ErrNotFound
}
