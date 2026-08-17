package apiconsumers

import (
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/apicontracts"
)

func fixture(t *testing.T) (*Store, apicontracts.Contract) {
	t.Helper()
	cs, _ := apicontracts.New(t.TempDir())
	in := apicontracts.Input{Name: "Orders", Version: "1.0", SourceRevision: "abc", DefinitionPath: "openapi.json", DefinitionFormat: "openapi", DefinitionValid: true, ValidationSummary: "valid", Operations: []apicontracts.Operation{{ID: "list", Method: "GET", Path: "/orders", Authentication: []string{"oauth"}, ResponseSchema: "Order", ErrorCodes: []string{"denied"}}}, Schemas: []apicontracts.Schema{{Name: "Order", Kind: "object"}}, Errors: []apicontracts.APIError{{Code: "denied", HTTPStatus: 403, Meaning: "denied"}}, Authentication: []apicontracts.Authentication{{ID: "oauth", Kind: "oauth2", Description: "scoped", Scopes: []string{"orders:read", "orders:write"}}}, Environments: []apicontracts.Environment{{Name: "sandbox", BaseURL: "https://sandbox.test", Availability: "available"}}, OwnerIDs: []string{"producer"}, Stability: "stable", SupportPolicy: "12 months", Compatibility: apicontracts.Compatibility{Promise: "semver"}, Links: []apicontracts.Link{{Kind: "source", ResourceID: "abc", Label: "source", Status: "current"}, {Kind: "release", ResourceID: "r", Label: "release", Status: "current"}, {Kind: "documentation", ResourceID: "d", Label: "docs", Status: "current"}, {Kind: "data_use", ResourceID: "p", Label: "privacy", Status: "current"}}, ChangeReason: "publish"}
	c, e := cs.Create("repo", "producer", in)
	if e != nil {
		t.Fatal(e)
	}
	s, e := New(t.TempDir(), cs)
	if e != nil {
		t.Fatal(e)
	}
	return s, c
}
func TestApprovalSandboxRotationExposureAndOwnership(t *testing.T) {
	s, c := fixture(t)
	a, e := s.Register("repo", "consumer", RegistrationInput{Name: "shop", ConsumerProject: "consumer/repo", Contact: "dev@example.test", ContractID: c.ID, ContractVersion: 1, Environments: []string{"sandbox"}, Capabilities: []string{"orders:read"}, CredentialLifetimeHours: 48})
	if e != nil {
		t.Fatal(e)
	}
	cred, e := s.Decide("repo", a.ID, "producer", ApprovalInput{ExpectedVersion: 1, Decision: "approved", Capabilities: []string{"orders:read"}, Environments: []string{"sandbox"}, Quota: 1, CredentialLifetimeHours: 24, SyntheticData: map[string]map[string]any{"list": {"id": "synthetic-order"}}, FailureRules: []FailureRule{{ID: "denied", OperationID: "list", Status: 403, ErrorCode: "denied"}}, Reason: "least privilege approved"})
	if e != nil || cred.Secret != "" || cred.Application.Status != "approved" {
		t.Fatalf("approval: %#v %v", cred, e)
	}
	cred, e = s.Consent("repo", a.ID, "consumer", "accepted narrowed sandbox terms", cred.Application.Version)
	if e != nil || cred.Secret == "" {
		t.Fatalf("consent: %#v %v", cred, e)
	}
	x, e := s.Sandbox(a.ID, cred.Secret, SandboxInput{OperationID: "list", Failure: "denied", Body: map[string]any{"page": 1}})
	if e != nil || x.ResponseStatus != 403 || x.RequestHeaders["authorization"] != "[REDACTED]" {
		t.Fatalf("inspection: %#v %v", x, e)
	}
	if _, e = s.Sandbox(a.ID, cred.Secret, SandboxInput{OperationID: "list"}); e != ErrQuota {
		t.Fatalf("quota got %v", e)
	}
	if _, e = s.Control("repo", a.ID, "consumer", "report_exposure", "posted in log", false); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Sandbox(a.ID, cred.Secret, SandboxInput{OperationID: "list"}); e != ErrForbidden {
		t.Fatalf("exposed token got %v", e)
	}
	rotated, e := s.Rotate("repo", a.ID, "consumer", "recovered after exposure", false)
	if e != nil || rotated.Secret == cred.Secret {
		t.Fatalf("rotation: %#v %v", rotated, e)
	}
	if _, e = s.Transfer("repo", a.ID, "consumer", "next-owner", "maintainer changed", false, false); e != nil {
		t.Fatal(e)
	}
	moved, e := s.Transfer("repo", a.ID, "next-owner", "", "accepted responsibility", true, false)
	if e != nil || moved.Status != "pending" || moved.CredentialState != "not_issued" {
		t.Fatalf("transfer: %#v %v", moved, e)
	}
	if _, e = s.Sandbox(a.ID, rotated.Secret, SandboxInput{OperationID: "list"}); e != ErrForbidden {
		t.Fatalf("transferred token got %v", e)
	}
}
func TestDenialAndReapplicationRetainAttribution(t *testing.T) {
	s, c := fixture(t)
	a, _ := s.Register("repo", "consumer", RegistrationInput{Name: "shop", ConsumerProject: "consumer/repo", Contact: "dev@example.test", ContractID: c.ID, ContractVersion: 1, Environments: []string{"sandbox"}, Capabilities: []string{"orders:write"}, CredentialLifetimeHours: 24})
	denied, e := s.Decide("repo", a.ID, "producer", ApprovalInput{ExpectedVersion: 1, Decision: "denied", Reason: "write is unnecessary"})
	if e != nil || denied.Application.Status != "denied" {
		t.Fatal(e)
	}
	reapplied, e := s.Control("repo", a.ID, "consumer", "reapply", "requesting a revised scope", false)
	if e != nil || reapplied.Status != "pending" || len(reapplied.Events) != 3 {
		t.Fatalf("reapply %#v %v", reapplied, e)
	}
}
