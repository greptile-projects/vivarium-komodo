package runtimeinvestigations

import (
	"testing"
	"time"
)

func TestSharedExplanationIsCitedChallengeableAndAgentBounded(t *testing.T) {
	s, _ := New(t.TempDir())
	v, err := s.Create("repo", "owner", CreateInput{WorkspaceID: "debug-1", Revision: "commit-1", Title: "Trace checkout", Question: "Why did checkout time out?", Audience: "participants", Participants: []string{"peer"}, Evidence: []Evidence{{ProbeID: "probe-1", CaptureID: "capture-1", Kind: "traces", Summary: "sanitized span stops before dependency return", Audience: "participants", Accessible: true}, {ProbeID: "probe-2", Kind: "logs", Summary: "privacy-restricted dependency logs", Audience: "participants", Accessible: false, Reason: "privacy owner denied access"}}, Correlations: []Correlation{{Kind: "symbol", ResourceID: "repo", Revision: "commit-1", Path: "checkout.go", Symbol: "Checkout", Relationship: "span names this exact handler", Status: "resolved"}, {Kind: "infrastructure", ResourceID: "payments-network", Revision: "definition:4", Relationship: "handler calls this dependency", Status: "inaccessible", Reason: "provider observation unavailable"}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.AddClaim("repo", v.ID, "owner", Claim{Kind: "hypothesis", Body: "Checkout blocked on the payment dependency.", Uncertainty: "dependency response is unavailable", Citations: []Citation{{EvidenceID: v.Evidence[0].ID}, {CorrelationID: v.Correlations[0].ID}}})
	if err != nil || v.Claims[0].Status != "proposed" {
		t.Fatalf("claim = %#v, %v", v.Claims, err)
	}
	v, _ = s.AddClaim("repo", v.ID, "peer", Claim{Kind: "challenge", Body: "The trace proves timing but not the dependency cause.", Citations: []Citation{{ClaimID: v.Claims[0].ID}}})
	if v.Claims[0].Status != "disputed" {
		t.Fatalf("challenge did not derive dispute: %#v", v.Claims)
	}
	v, _ = s.AddClaim("repo", v.ID, "owner", Claim{Kind: "finding", Body: "Provider state cannot confirm the suspected path.", Citations: []Citation{{EvidenceID: v.Evidence[1].ID}, {CorrelationID: v.Correlations[1].ID}}})
	if v.Claims[2].Status != "blocked" || len(v.Claims[2].BlockedReasons) != 2 {
		t.Fatalf("blocked claim = %#v", v.Claims[2])
	}
	v, _ = s.RequestOwner("repo", v.ID, "peer", OwnerRequest{OwnerKind: "security", OwnerID: "security-owner", Question: "Can sanitized connection metadata be shared?"})
	if v.OwnerRequests[0].Status != "requested" {
		t.Fatalf("owner request = %#v", v.OwnerRequests)
	}
	v, secret, err := s.StartAgent("repo", v.ID, "owner", "agent:reader", "Test the named execution-path hypothesis", []string{v.Evidence[0].ID}, []string{v.Correlations[0].ID}, time.Now().Add(time.Hour))
	if err != nil || secret == "" || len(v.Authority) != 0 {
		t.Fatalf("agent start = %#v %q %v", v, secret, err)
	}
	context, session, err := s.AgentContext(secret)
	if err != nil || session.Status != "active" || len(context.Evidence) != 2 {
		t.Fatalf("agent context = %#v %#v %v", context, session, err)
	}
	v, err = s.AgentClaim(secret, Claim{Kind: "finding", Body: "The selected span enters Checkout at the pinned symbol.", Citations: []Citation{{EvidenceID: v.Evidence[0].ID}, {CorrelationID: v.Correlations[0].ID}}})
	if err != nil || v.Claims[len(v.Claims)-1].AgentSessionID == "" {
		t.Fatalf("agent claim = %#v %v", v.Claims, err)
	}
	v, _ = s.ControlAgent("repo", v.ID, session.ID, "owner", "guide", "Compare the retry branch.")
	v, _ = s.ControlAgent("repo", v.ID, session.ID, "owner", "pause", "")
	if _, _, err = s.AgentContext(secret); err != ErrForbidden {
		t.Fatalf("paused credential remained active: %v", err)
	}
	v, _ = s.ControlAgent("repo", v.ID, session.ID, "owner", "revoke", "")
	if v.AgentSessions[0].TokenDigest != "" {
		t.Fatal("revocation did not invalidate credential")
	}
}
