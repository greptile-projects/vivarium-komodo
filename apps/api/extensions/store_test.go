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
