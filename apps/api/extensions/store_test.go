package extensions

import "testing"

func input() Input {
	return Input{Name: "Build observer", Description: "Reports checks", OperatorContact: "ops@example.com", Capabilities: []string{"annotate checks"}, CallbackURL: "https://example.com/events", ActionURL: "https://example.com/actions", RequestedPermissions: []string{"metadata:read", "checks:write"}, EventTypes: []string{"push", "check.requested"}, RotationPolicy: RotationPolicy{IntervalDays: 30, OverlapHours: 24, ContactOnFailure: true}}
}
func TestRegistrationVerificationAndAuthority(t *testing.T) {
	s, _ := New(t.TempDir())
	if !valid(input()) {
		t.Fatalf("fixture invalid: %#v callback=%v action=%v", input(), cleanURL(input().CallbackURL), cleanURL(input().ActionURL))
	}
	x, e := s.Create("owner", input())
	if e != nil || x.ID == "" || x.ID == "owner" || x.Status != "pending_verification" {
		t.Fatalf("create: %#v %v", x, e)
	}
	if _, e = s.Install(x.ID, "repo", "owner", []string{"metadata:read"}, []string{"push"}); e == nil {
		t.Fatal("unverified extension installed")
	}
	x, e = s.Verify(x.ID, "owner", "callback", x.Callback.VerificationToken)
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.Verify(x.ID, "owner", "actions", x.Actions.VerificationToken)
	if e != nil || x.Status != "verified" {
		t.Fatalf("verify: %#v %v", x, e)
	}
	a, e := s.Preview(x.ID, "repo", []string{"metadata:read"}, []string{"push"})
	if e != nil || a.ActorID != x.ID || a.CanImpersonate || a.CredentialIssued {
		t.Fatalf("authority: %#v %v", a, e)
	}
	i, e := s.Install(x.ID, "repo", "owner", a.Permissions, a.EventTypes)
	if e != nil || i.Authority.ActorID == "owner" {
		t.Fatalf("install: %#v %v", i, e)
	}
	i, e = s.Revoke("repo", i.ID, "owner")
	if e != nil || i.Status != "revoked" || len(i.Authority.Permissions) > 0 {
		t.Fatalf("revoke: %#v %v", i, e)
	}
}
func TestRejectsUnsafeContract(t *testing.T) {
	s, _ := New(t.TempDir())
	in := input()
	in.CallbackURL = "http://localhost/hook"
	if _, e := s.Create("owner", in); e == nil {
		t.Fatal("accepted insecure endpoint")
	}
	in = input()
	in.RequestedPermissions = []string{"admin"}
	if _, e := s.Create("owner", in); e == nil {
		t.Fatal("accepted unknown permission")
	}
}
func TestInstallationGovernance(t *testing.T) {
	s, _ := New(t.TempDir())
	x, _ := s.Create("publisher", input())
	x, _ = s.Verify(x.ID, "publisher", "callback", x.Callback.VerificationToken)
	x, _ = s.Verify(x.ID, "publisher", "actions", x.Actions.VerificationToken)
	grant := GrantInput{Permissions: []string{"metadata:read"}, EventTypes: []string{"push"}, ResourceTypes: []string{"issues"}, CapabilityDecisions: []CapabilityDecision{{Capability: "annotate checks", Decision: "denied"}}, Settings: map[string]string{"project": "api"}}
	i, err := s.InstallGrant(x.ID, "repo", "owner", grant)
	if err != nil || i.Version != 1 || i.Events[0].ActorID != "owner" {
		t.Fatalf("install: %#v %v", i, err)
	}
	i, err = s.Update("repo", i.ID, "owner", "suspend", "maintenance", 1, nil)
	if err != nil || i.Status != "suspended" || len(i.Authority.Permissions) != 0 {
		t.Fatalf("suspend: %#v %v", i, err)
	}
	if _, err = s.Update("repo", i.ID, "owner", "resume", "", 1, nil); err == nil {
		t.Fatal("accepted stale version")
	}
	i, err = s.Update("repo", i.ID, "owner", "resume", "", 2, nil)
	if err != nil || i.Status != "active" || len(i.Authority.Permissions) != 1 {
		t.Fatalf("resume: %#v %v", i, err)
	}
	bad := grant
	bad.Settings = map[string]string{"api_token": "nope"}
	if _, err = s.Update("repo", i.ID, "owner", "upgrade", "", 3, &bad); err == nil {
		t.Fatal("accepted secret setting")
	}
	other, _ := s.InstallGrant(x.ID, "other", "owner", grant)
	i, err = s.Update("repo", i.ID, "owner", "remove", "retired", 3, nil)
	if err != nil || i.Status != "removed" {
		t.Fatalf("remove: %#v %v", i, err)
	}
	listed, _ := s.ListInstallations("other")
	if len(listed) != 1 || listed[0].ID != other.ID || listed[0].Status != "active" {
		t.Fatal("disturbed unrelated installation")
	}
}
