package organizations

import (
	"encoding/json"
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

func TestVersionedPoliciesPreviewActivationAndExpiringExceptions(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 9, 20, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	o, _ := s.Create("owner", "quality", "Quality", "")
	o, _ = s.Invite(o.ID, "owner", "maintainer")
	o, _ = s.Accept(o.ID, "maintainer")
	_, team, _ := s.CreateTeam(o.ID, "owner", Team{Slug: "runtime", Name: "Runtime", Visibility: "internal"})
	o, _, _ = s.AddResponsibility(o.ID, team.ID, "owner", Responsibility{RepositoryID: "repo-a", Area: "**", Visibility: "internal"}, team.Version)
	draft := PolicyVersion{Name: "Shared delivery bar", Targets: []PolicyTarget{{Kind: "team", ID: team.ID}}, Rules: []PolicyRule{
		{ID: "checks", Domain: "required_checks", Enforcement: "required", Config: json.RawMessage(`{"names":["test"]}`)},
		{ID: "agents", Domain: "agent_authority", Enforcement: "required", Config: json.RawMessage(`{"approved_only":true}`)},
	}}
	_, draft, err := s.DraftPolicy(o.ID, "owner", "", draft)
	if err != nil || draft.State != "draft" || draft.Version != 1 {
		t.Fatalf("draft = %#v, err=%v", draft, err)
	}
	preview, _ := s.EffectivePolicy(o.ID, "repo-a", &draft)
	if len(preview) != 2 || preview[0].Target.ID != team.ID {
		t.Fatalf("preview = %#v", preview)
	}
	activeRules, _ := s.EffectivePolicy(o.ID, "repo-a", nil)
	if len(activeRules) != 0 {
		t.Fatal("draft affected repository before activation")
	}
	_, active, err := s.ActivatePolicy(o.ID, "owner", draft.ID, 1)
	if err != nil || active.ActivatedByID != "owner" {
		t.Fatalf("active = %#v, err=%v", active, err)
	}
	_, request, err := s.RequestPolicyException(o.ID, "maintainer", PolicyException{PolicyID: draft.ID, PolicyVersion: 1, RuleID: "checks", RepositoryID: "repo-a", Reason: "legacy runner migration", ExpiresAt: now.Add(48 * time.Hour)})
	if err != nil || request.State != "pending" {
		t.Fatalf("request = %#v, err=%v", request, err)
	}
	rules, _ := s.EffectivePolicy(o.ID, "repo-a", nil)
	if rules[0].Exception != nil {
		t.Fatal("pending exception silently weakened policy")
	}
	_, approved, err := s.ResolvePolicyException(o.ID, request.ID, "owner", "approved")
	if err != nil || approved.ResolvedByID != "owner" {
		t.Fatalf("approval = %#v, err=%v", approved, err)
	}
	rules, _ = s.EffectivePolicy(o.ID, "repo-a", nil)
	if rules[0].Exception == nil || rules[0].Rule.Enforcement != "required" {
		t.Fatalf("effective explanation = %#v", rules)
	}
	now = now.Add(72 * time.Hour)
	rules, _ = s.EffectivePolicy(o.ID, "repo-a", nil)
	if rules[0].Exception != nil {
		t.Fatal("expired exception remained effective")
	}
	_, next, err := s.DraftPolicy(o.ID, "owner", draft.ID, PolicyVersion{Name: "Stronger bar", Targets: []PolicyTarget{{Kind: "organization"}}, Rules: []PolicyRule{{ID: "review", Domain: "reviews", Enforcement: "required", Config: json.RawMessage(`{"owner_approvals":2}`)}}})
	if err != nil || next.Version != 2 {
		t.Fatalf("next = %#v, err=%v", next, err)
	}
	stillActive, _ := s.EffectivePolicy(o.ID, "repo-a", nil)
	if stillActive[0].PolicyVersion != 1 {
		t.Fatal("new draft invalidated active work")
	}
}

func TestPortfolioInitiativesExposeCrossRepositoryBlockersAndReassignment(t *testing.T) {
	s, _ := New(t.TempDir())
	o, _ := s.Create("owner", "portfolio", "Portfolio", "")
	o, _ = s.Invite(o.ID, "owner", "developer")
	o, _ = s.Accept(o.ID, "developer")
	_, team, _ := s.CreateTeam(o.ID, "owner", Team{Slug: "platform", Name: "Platform", Visibility: "internal"})
	_, grant, err := s.GrantRole(o.ID, "owner", RoleGrant{PrincipalKind: "team", PrincipalID: team.ID, Role: "contributor", Resources: []ResourceRef{{Kind: "repository", ID: "repo-a"}}, Reason: "Deliver the initiative", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	_, initiative, err := s.CreateInitiative(o.ID, "owner", Initiative{Title: "Ship shared identity", Outcome: "All services use the new identity contract", Sources: []ResourceRef{{Kind: "proposal", ID: "proposal-a", RepositoryID: "repo-a"}, {Kind: "incident", ID: "incident-b", RepositoryID: "repo-b"}}})
	if err != nil {
		t.Fatal(err)
	}
	_, first, err := s.PutInitiativeItem(o.ID, initiative.ID, "", "owner", InitiativeItem{Title: "Publish contract", Outcome: "Provider release is available", RepositoryID: "repo-a", Source: ResourceRef{Kind: "proposal_task", ID: "task-a", RepositoryID: "repo-a"}, AssigneeKind: "team", AssigneeID: team.ID, State: "in_progress", UpcomingReleaseIDs: []string{"release-a"}, NextDecision: "Approve the compatibility exception"})
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := s.PutInitiativeItem(o.ID, initiative.ID, "", "developer", InitiativeItem{Title: "Adopt contract", Outcome: "Consumer is migrated", RepositoryID: "repo-b", Source: ResourceRef{Kind: "incident_action", ID: "action-b", RepositoryID: "repo-b"}, AssigneeKind: "human", AssigneeID: "developer", DependsOn: []string{first.ID}, State: "planned", Contributions: []ResourceRef{{Kind: "pull_request", ID: "pull-b", RepositoryID: "repo-b"}}})
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.InitiativeView(o.ID, func(repo string) bool { return repo == "repo-a" || repo == "repo-b" })
	if err != nil || len(view) != 1 || len(view[0].Items) != 2 || len(view[0].Items[1].BlockedBy) != 1 || view[0].Items[1].BlockedBy[0] != first.ID {
		t.Fatalf("view = %#v, err=%v", view, err)
	}
	if view[0].Items[0].NeedsReassignment {
		t.Fatalf("active scoped team access was not recognized: %#v", view[0].Items[0])
	}
	if _, _, err = s.RevokeRole(o.ID, grant.ID, "owner"); err != nil {
		t.Fatal(err)
	}
	view, _ = s.InitiativeView(o.ID, func(repo string) bool { return true })
	if !view[0].Items[0].NeedsReassignment {
		t.Fatal("revoked team access did not request reassignment")
	}
	if _, err = s.Remove(o.ID, "owner", "developer"); err != nil {
		t.Fatal(err)
	}
	view, _ = s.InitiativeView(o.ID, func(repo string) bool { return repo != "repo-b" })
	second = view[0].Items[1]
	if !second.NeedsReassignment || second.BlockedBy[len(second.BlockedBy)-1] != "reassignment" || second.NextDecision == "" {
		t.Fatalf("reassignment = %#v", second)
	}
	if _, _, err = s.PutInitiativeItem(o.ID, initiative.ID, first.ID, "owner", InitiativeItem{Title: "cycle", Outcome: "cycle", RepositoryID: "repo-a", Source: ResourceRef{Kind: "proposal_task", ID: "task-a", RepositoryID: "repo-a"}, AssigneeKind: "team", AssigneeID: team.ID, DependsOn: []string{second.ID}}); err != ErrInvalid {
		t.Fatalf("cycle accepted: %v", err)
	}
}
