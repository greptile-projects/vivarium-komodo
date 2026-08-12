package federation

import "testing"

func config(origin string) Config {
	return Config{Instance: origin, Operators: []Operator{{Name: "Operator", Contact: "mailto:ops@example.test"}}, Capabilities: []string{"identity.discovery"}, Endpoints: Endpoints{Discovery: origin + "/.well-known/komodo-federation", Actors: origin + "/federation/actors/{kind}/{id}"}}
}

func TestSignedVersionedIdentityAndRotation(t *testing.T) {
	s, err := New(t.TempDir(), config("https://one.example"))
	if err != nil {
		t.Fatal(err)
	}
	first, _ := s.Document()
	if Verify(first) != nil {
		t.Fatal("initial identity did not verify")
	}
	actor, err := s.PublishActor("agent", "reviewer", "Review agent", "")
	if err != nil || actor.Subject != "agent:reviewer@https://one.example" {
		t.Fatalf("actor %#v %v", actor, err)
	}
	published, _ := s.Document()
	if published.Version != 2 || published.PreviousDigest != Digest(first) || Verify(published) != nil {
		t.Fatalf("published %#v", published)
	}
	rotated, err := s.Rotate()
	if err != nil || rotated.KeyID == published.KeyID || rotated.Keys[1].Status != "retired" || Verify(rotated) != nil {
		t.Fatalf("rotation %#v %v", rotated, err)
	}
}

func TestPeerTrustChangesAndFailuresRemainExplicit(t *testing.T) {
	local, _ := New(t.TempDir(), config("https://local.example"))
	remote, _ := New(t.TempDir(), config("https://remote.example"))
	doc, _ := remote.Document()
	peer, err := local.Observe("https://remote.example/.well-known/komodo-federation", doc, nil)
	if err != nil || peer.Trust != "untrusted" || peer.Status != "reachable" {
		t.Fatalf("peer %#v %v", peer, err)
	}
	peer, err = local.Trust(doc.Instance, "trust")
	if err != nil || peer.Trust != "trusted" {
		t.Fatal(err)
	}
	changed := doc
	changed.Capabilities = []string{"identity.discovery", "unexpected"}
	changed.Signature = "bad"
	if _, err = local.Observe(peer.DiscoveryURL, changed, nil); err == nil {
		t.Fatal("accepted forged change")
	}
	peer, _ = local.Observe(peer.DiscoveryURL, Document{}, ErrInvalid)
	if peer.Status != "unreachable" || peer.Trust != "trusted" || peer.LastError == "" {
		t.Fatalf("failure %#v", peer)
	}
	peer, _ = local.Trust(doc.Instance, "revoke")
	if peer.Trust != "revoked" || peer.RevokedAt == nil {
		t.Fatalf("revocation %#v", peer)
	}
}
