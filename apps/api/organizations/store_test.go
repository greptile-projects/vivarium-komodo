package organizations

import (
	"testing"
	"time"
)

func TestMembershipAndTransferRequireAcceptance(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	o, err := s.Create("owner", "platform", "Platform", "Shared work")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Invite(o.ID, "owner", "member"); err != nil {
		t.Fatal(err)
	}
	if s.IsMember(o.ID, "member") {
		t.Fatal("an invitation must not grant membership")
	}
	if _, err = s.Accept(o.ID, "member"); err != nil {
		t.Fatal(err)
	}
	if !s.IsMember(o.ID, "member") {
		t.Fatal("accepted member is not recognized")
	}
	_, transfer, err := s.RequestTransfer(o.ID, "member", Transfer{RepositoryID: "repo", FromKind: "user", FromID: "member", ToKind: "organization", ToID: o.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.ResolveTransfer(o.ID, transfer.ID, "member", "accepted"); err != ErrForbidden {
		t.Fatalf("member accepted organization control: %v", err)
	}
	o, transfer, err = s.ResolveTransfer(o.ID, transfer.ID, "owner", "accepted")
	if err != nil {
		t.Fatal(err)
	}
	if transfer.State != "accepted" || len(o.Events) != 5 {
		t.Fatalf("transfer evidence = %#v, events=%d", transfer, len(o.Events))
	}
	if _, err = s.Remove(o.ID, "owner", "member"); err != nil {
		t.Fatal(err)
	}
	if s.IsMember(o.ID, "member") {
		t.Fatal("removed member retained access")
	}
}

func TestScopedRoleRequestsEffectiveAccessAndRevocation(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	o, _ := s.Create("owner", "delivery", "Delivery", "")
	o, _ = s.Invite(o.ID, "owner", "developer")
	o, _ = s.Accept(o.ID, "developer")
	_, team, _ := s.CreateTeam(o.ID, "owner", Team{Slug: "runtime", Name: "Runtime", Visibility: "internal"})
	o, _ = s.InviteTeamMember(o.ID, team.ID, "owner", "developer", "member", team.Version)
	_, _ = s.AcceptTeam(o.ID, team.ID, "developer", 2)
	request := AccessRequest{PrincipalKind: "team", PrincipalID: team.ID, Role: "contributor", Resources: []ResourceRef{{Kind: "repository", ID: "repo-a"}, {Kind: "environment", ID: "production", RepositoryID: "repo-a"}}, Exceptions: []string{"deploy:production"}, Reason: "Repair the runtime across its owned surfaces", ExpiresAt: now.Add(8 * time.Hour)}
	_, requested, err := s.RequestAccess(o.ID, "developer", request)
	if err != nil || requested.State != "pending" {
		t.Fatalf("request = %#v, err=%v", requested, err)
	}
	_, resolved, grant, err := s.ResolveAccessRequest(o.ID, requested.ID, "owner", "approved")
	if err != nil || resolved.GrantID != grant.ID {
		t.Fatalf("approval = %#v %#v, err=%v", resolved, grant, err)
	}
	effective, _ := s.EffectiveAccess(o.ID, "developer")
	if len(effective) != 1 || effective[0].Role != "contributor" || effective[0].Exceptions[0] != "deploy:production" {
		t.Fatalf("effective access = %#v", effective)
	}
	if _, _, err = s.AttachCredential(o.ID, grant.ID, "developer", "credential-a"); err != nil {
		t.Fatal(err)
	}
	_, revoked, err := s.RevokeRole(o.ID, grant.ID, "owner")
	if err != nil || len(revoked.CredentialIDs) != 1 || revoked.CredentialUsers["credential-a"] != "developer" {
		t.Fatalf("revoked = %#v, err=%v", revoked, err)
	}
	effective, _ = s.EffectiveAccess(o.ID, "developer")
	if len(effective) != 0 {
		t.Fatalf("revoked access remained effective: %#v", effective)
	}
	if _, _, err = s.GrantRole(o.ID, "developer", RoleGrant{PrincipalKind: "team", PrincipalID: team.ID, Role: "viewer", Resources: []ResourceRef{{Kind: "repository", ID: "repo-a"}}, Reason: "self elevation", ExpiresAt: now.Add(time.Hour)}); err != ErrForbidden {
		t.Fatalf("member self-grant = %v", err)
	}
}

func TestNestedTeamsResponsibilitiesAgentsAndConcurrency(t *testing.T) {
	s, _ := New(t.TempDir())
	o, _ := s.Create("owner", "acme", "Acme", "")
	o, _ = s.Invite(o.ID, "owner", "maintainer")
	o, _ = s.Accept(o.ID, "maintainer")
	o, _ = s.Invite(o.ID, "owner", "developer")
	_, _ = s.Accept(o.ID, "developer")
	_, platform, err := s.CreateTeam(o.ID, "owner", Team{Slug: "platform", Name: "Platform", Visibility: "public"})
	if err != nil {
		t.Fatal(err)
	}
	_, runtime, err := s.CreateTeam(o.ID, "owner", Team{Slug: "runtime", Name: "Runtime", ParentID: platform.ID, Visibility: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.InviteTeamMember(o.ID, platform.ID, "owner", "maintainer", "maintainer", platform.Version); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AcceptTeam(o.ID, platform.ID, "maintainer", 2); err != nil {
		t.Fatal(err)
	}
	if _, err = s.InviteTeamMember(o.ID, runtime.ID, "maintainer", "developer", "member", runtime.Version); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AcceptTeam(o.ID, runtime.ID, "developer", 2); err != nil {
		t.Fatal(err)
	}
	if _, err = s.InviteTeamMember(o.ID, runtime.ID, "owner", "maintainer", "member", 1); err != ErrConflict {
		t.Fatalf("stale edit = %v", err)
	}
	_, responsibility, err := s.AddResponsibility(o.ID, runtime.ID, "maintainer", Responsibility{RepositoryID: "repo", Area: "src/runtime/**", Visibility: "public"}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if responsibility.CreatedByID != "maintainer" {
		t.Fatal("responsibility attribution missing")
	}
	_, agent, err := s.RegisterAgent(o.ID, "owner", Agent{Slug: "release-bot", Name: "Release bot", Capabilities: []string{"release:inspect", "checks:read"}, OperatorIDs: []string{"maintainer"}, Visibility: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if agent.Version != 1 || agent.CreatedByID != "owner" {
		t.Fatal("agent governance evidence missing")
	}
	o, _ = s.Get(o.ID)
	directory := DirectoryFor(o, false)
	if len(directory.Teams) != 2 || len(directory.Agents) != 1 || len(directory.EffectiveMembers[platform.ID]) != 2 {
		t.Fatalf("directory = %#v", directory)
	}
	foundNested := false
	for _, m := range directory.EffectiveMembers[platform.ID] {
		if m.UserID == "developer" && len(m.ViaTeamIDs) == 2 && m.ViaTeamIDs[1] == runtime.ID {
			foundNested = true
		}
	}
	if !foundNested {
		t.Fatalf("nested explanation = %#v", directory.EffectiveMembers)
	}
	if _, err = s.Remove(o.ID, "owner", "maintainer"); err != nil {
		t.Fatal(err)
	}
	o, _ = s.Get(o.ID)
	if len(o.Agents[0].OperatorIDs) != 0 {
		t.Fatal("removed member remained an agent operator")
	}
}
