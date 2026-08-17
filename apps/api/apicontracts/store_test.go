package apicontracts

import "testing"

func contract(version string) Input {
	return Input{Name: "Payments", Version: version, Description: "Charge customers", SourceRevision: "abc123", DefinitionPath: "api/openapi.json", DefinitionFormat: "openapi", DefinitionValid: true, ValidationSummary: "valid", Operations: []Operation{{ID: "createCharge", Method: "POST", Path: "/charges", Summary: "Create", Authentication: []string{"oauth"}, RequestSchema: "Charge", ResponseSchema: "Charge", ErrorCodes: []string{"declined"}}}, Schemas: []Schema{{Name: "Charge", Kind: "object", Fields: []Field{{Name: "id", Type: "string", Required: true, Description: "stable id"}}}}, Errors: []APIError{{Code: "declined", HTTPStatus: 402, Meaning: "card declined"}}, Authentication: []Authentication{{ID: "oauth", Kind: "oauth2", Description: "client credentials", Scopes: []string{"charges:write"}}}, Environments: []Environment{{Name: "production", BaseURL: "https://api.example.test", Availability: "available"}}, Limits: []Limit{{Name: "requests", Value: 100, Unit: "minute", Scope: "application"}}, OwnerIDs: []string{"payments-team"}, Stability: "stable", SupportPolicy: "Security fixes for 12 months", Compatibility: Compatibility{Promise: "semantic versioning"}, Links: []Link{{Kind: "source", ResourceID: "abc123", Label: "reviewed implementation", Status: "current"}, {Kind: "release", ResourceID: "r1", Label: "v1", Status: "current"}, {Kind: "documentation", ResourceID: "docs1", Revision: "abc123", Label: "guide", Status: "current"}, {Kind: "data_use", ResourceID: "privacy1", Label: "payment handling", Status: "current"}}, ChangeReason: "publish reviewed API"}
}
func TestVersionsComparisonAndExplicitGaps(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	x, e := s.Create("repo", "owner", contract("1.0.0"))
	if e != nil {
		t.Fatal(e)
	}
	next := contract("2.0.0")
	next.DefinitionValid = false
	next.ValidationSummary = "unresolved schema reference"
	next.Operations = nil
	next.Operations = []Operation{{ID: "listCharges", Method: "GET", Path: "/charges", Summary: "List", Authentication: []string{"oauth"}, ResponseSchema: "Charge", ErrorCodes: []string{"declined"}}}
	next.Links[1].Status = "unreleased"
	next.Links[2].Status = "stale"
	next.Environments[0].Availability = "degraded"
	next.Compatibility.BreakingChanges = []string{"createCharge removed"}
	x, e = s.Revise("repo", x.ID, "owner", 1, next)
	if e != nil {
		t.Fatal(e)
	}
	k := map[string]bool{}
	for _, g := range x.Gaps {
		k[g.Kind] = true
	}
	for _, want := range []string{"invalid_definition", "unreleased_implementation", "stale_documentation", "environment_degraded"} {
		if !k[want] {
			t.Fatalf("missing gap %s: %#v", want, x.Gaps)
		}
	}
	c, e := s.Compare("repo", x.ID, 1, 2)
	if e != nil || !c.Breaking || len(c.RemovedOperations) != 1 || c.RemovedOperations[0] != "createCharge" {
		t.Fatalf("comparison: %#v %v", c, e)
	}
}
func TestRejectsStructurallyInvalidPublication(t *testing.T) {
	s, _ := New(t.TempDir())
	in := contract("1")
	in.Operations[0].ResponseSchema = "missing"
	if _, e := s.Create("repo", "owner", in); e != ErrInvalid {
		t.Fatalf("got %v", e)
	}
}
