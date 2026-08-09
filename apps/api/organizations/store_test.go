package organizations

import "testing"

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
