package apiconsumers

import (
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/apicontracts"
)

func integrationApplication(t *testing.T) (*Store, Application) {
	t.Helper()
	contracts, err := apicontracts.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, err := contracts.Create("producer", "producer", apicontracts.Input{Name: "Orders", Version: "1.0", SourceRevision: "producer-sha", DefinitionPath: "api/openapi.json", DefinitionFormat: "openapi", DefinitionValid: true, ValidationSummary: "valid", Operations: []apicontracts.Operation{{ID: "list", Method: "GET", Path: "/orders", Authentication: []string{"oauth"}, ResponseSchema: "Order"}}, Schemas: []apicontracts.Schema{{Name: "Order", Kind: "object"}}, Authentication: []apicontracts.Authentication{{ID: "oauth", Kind: "oauth2", Description: "scoped", Scopes: []string{"orders:read"}}}, Environments: []apicontracts.Environment{{Name: "sandbox", BaseURL: "https://sandbox.test", Availability: "available"}}, OwnerIDs: []string{"producer"}, Stability: "stable", SupportPolicy: "12 months", Compatibility: apicontracts.Compatibility{Promise: "semver"}, Links: []apicontracts.Link{{Kind: "source", ResourceID: "producer-sha", Label: "source", Status: "current"}}, ChangeReason: "publish"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(t.TempDir(), contracts)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Register("producer", "consumer", RegistrationInput{Name: "shop", ConsumerProject: "consumer/repo", Contact: "dev@example.test", ContractID: c.ID, ContractVersion: 1, Environments: []string{"sandbox"}, Capabilities: []string{"orders:read"}, CredentialLifetimeHours: 24})
	if err != nil {
		t.Fatal(err)
	}
	approved, err := s.Decide("producer", a.ID, "producer", ApprovalInput{ExpectedVersion: a.Version, Decision: "approved", Capabilities: []string{"orders:read"}, Environments: []string{"sandbox"}, Quota: 10, CredentialLifetimeHours: 12, SyntheticData: map[string]map[string]any{"list": {"id": "synthetic"}}, FailureRules: []FailureRule{{ID: "unavailable", OperationID: "list", Status: 503, ErrorCode: "unavailable"}}, Reason: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	return s, approved.Application
}

func TestIntegrationWorkFreezesReviewContextWithoutCredential(t *testing.T) {
	s, a := integrationApplication(t)
	w, err := s.CreateWork("producer", a.ID, "consumer", false, WorkInput{Kind: "workspace", OwnerKind: "agent", OwnerID: "agent:consumer", ConsumerRepositoryID: "consumer-repo", ConsumerRevision: "consumer-sha", ResourceID: "workspace-1", SDKReferences: []string{"sdk/go@abc"}, ExampleReferences: []string{"examples/list.go@abc"}, AcceptanceCriteria: []string{"producer conformance and consumer tests pass"}})
	if err != nil {
		t.Fatal(err)
	}
	if w.ContractSourceRevision != "producer-sha" || w.ConsumerRevision != "consumer-sha" || !w.Sandbox.SyntheticOnly || w.Sandbox.CredentialIncluded || len(w.Sandbox.FailureScenarios) != 1 {
		t.Fatalf("unsafe or incomplete brief: %+v", w)
	}
	if _, err = s.CreateWork("producer", a.ID, "consumer", false, WorkInput{Kind: "task", OwnerKind: "human", OwnerID: "consumer", ConsumerRepositoryID: "consumer-repo", ConsumerRevision: "sha", SDKReferences: []string{"Authorization: Bearer vka_secret"}, AcceptanceCriteria: []string{"pass"}}); err != ErrInvalid {
		t.Fatalf("secret-bearing preload accepted: %v", err)
	}
}

func TestVerificationRequiresBothIndependentlyDefinedSuitesAndRejectsSecrets(t *testing.T) {
	s, a := integrationApplication(t)
	v, err := s.RecordVerification("producer", a.ID, "consumer", false, VerificationInput{PullRequestID: "pull-1", CandidateRepositoryID: "consumer-repo", CandidateRevision: "consumer-candidate", Results: []ScenarioResult{{Name: "provider schema", Kind: "producer_conformance", Status: "passed", SanitizedRequest: map[string]any{"path": "/orders"}, SanitizedResponse: map[string]any{"status": 200}, Logs: []string{"schema matched"}, Coverage: []string{"list"}, Artifacts: []EvidenceArtifact{{Name: "conformance.json", Digest: "sha256:abc", MediaType: "application/json", Size: 12}}, Cost: 0.12}, {Name: "client decoding", Kind: "consumer_test", Status: "passed", Logs: []string{"decoded synthetic order"}, Coverage: []string{"OrdersClient.List"}, InaccessibleEvidence: []string{"private fixture omitted"}, Cost: 0.03}}})
	if err != nil {
		t.Fatal(err)
	}
	if v.Agreement != "demonstrated" || !v.ProducerPassed || !v.ConsumerPassed || v.ContractSourceRevision != "producer-sha" {
		t.Fatalf("agreement not derived: %+v", v)
	}
	_, err = s.RecordVerification("producer", a.ID, "consumer", false, VerificationInput{PullRequestID: "pull-2", CandidateRepositoryID: "consumer-repo", CandidateRevision: "sha", Results: []ScenarioResult{{Name: "leak", Kind: "consumer_test", Status: "failed", Logs: []string{"Authorization: Bearer vka_secret"}}}})
	if err != ErrInvalid {
		t.Fatalf("credential entered reusable evidence: %v", err)
	}
}
