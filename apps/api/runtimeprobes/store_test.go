package runtimeprobes

import (
	"strings"
	"testing"
	"time"
)

func TestProbeApprovalSanitizationPartialEvidenceAndRevocation(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	p, err := s.Request("repo", "responder", RequestInput{WorkspaceID: "debug", Environment: "production", Kind: "traces", Scope: []string{"service:checkout", "route:/pay"}, Purpose: "correlate failed checkout", ConsentActorIDs: []string{"privacy-owner"}, ExpiresAt: now.Add(time.Hour), Preview: Preview{DataCategories: []string{"request metadata", "account identifiers"}, EstimatedCost: 2.5, EstimatedLoad: "low", Audience: "participants", SamplingRate: .1, RetentionHours: 24, PrivacyPolicy: "privacy-v4", SecurityPolicy: "production-debug-v2"}})
	if err != nil || p.Status != "pending_approval" || len(p.Authority) != 0 {
		t.Fatalf("unsafe request: %+v %v", p, err)
	}
	if _, err = s.Capture("repo", p.ID, "responder", CaptureInput{}); err != ErrStopped {
		t.Fatalf("unapproved capture accepted: %v", err)
	}
	p, err = s.Decide("repo", p.ID, "service-owner", "approved", "environment load and audience accepted")
	if err != nil || p.ApprovedBy != "service-owner" {
		t.Fatal(err)
	}
	p, err = s.Capture("repo", p.ID, "responder", CaptureInput{StartedAt: now.Add(-time.Minute), EndedAt: now, RecordsExpected: 3, Records: []string{"span=1 Authorization: bearer-abc email=user@example.com", "span=2 token=topsecret"}, Gaps: []string{"one sampled span timed out"}, Provenance: "collector:otel@sha256:abc"})
	if err != nil {
		t.Fatal(err)
	}
	c := p.Captures[0]
	if c.Completeness != "incomplete" || c.Status != "partial" || len(c.Gaps) != 1 || strings.Contains(strings.Join(c.SanitizedData, " "), "topsecret") || strings.Contains(strings.Join(c.SanitizedData, " "), "example.com") {
		t.Fatalf("partial evidence presented unsafely: %+v", c)
	}
	p, err = s.Control("repo", p.ID, "privacy-owner", "consent_revoked", "account identifier consent withdrawn")
	if err != nil || p.Status != "consent_revoked" {
		t.Fatal(err)
	}
	if _, err = s.Capture("repo", p.ID, "responder", CaptureInput{StartedAt: now, EndedAt: now, Provenance: "x"}); err != ErrStopped {
		t.Fatalf("revoked probe collected: %v", err)
	}
}

func TestProbeExpiryOverloadAndRepositoryDiagnosticBoundary(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Now().UTC()
	s.now = func() time.Time { return now }
	base := RequestInput{WorkspaceID: "w", Environment: "prod", Kind: "dynamic_diagnostic", Scope: []string{"pod:api"}, Purpose: "inspect queue", ExpiresAt: now.Add(time.Hour), Preview: Preview{DataCategories: []string{"aggregate queue state"}, EstimatedLoad: "moderate", Audience: "repository", SamplingRate: 1, RetentionHours: 1, PrivacyPolicy: "p", SecurityPolicy: "s"}}
	if _, e := s.Request("r", "u", base); e != ErrInvalid {
		t.Fatalf("undeclared diagnostic accepted: %v", e)
	}
	base.Diagnostic = Diagnostic{Name: "queue-depth", Path: ".komodo/diagnostics.json", Revision: "commit-a"}
	p, e := s.Request("r", "u", base)
	if e != nil {
		t.Fatal(e)
	}
	p, _ = s.Decide("r", p.ID, "owner", "approved", "bounded")
	p, e = s.Control("r", p.ID, "owner", "overload", "collector exceeded 2 percent CPU")
	if e != nil || p.Status != "overload_stopped" {
		t.Fatalf("overload did not stop: %+v %v", p, e)
	}
	base.ExpiresAt = now.Add(25 * time.Hour)
	if _, e = s.Request("r", "u", base); e != ErrInvalid {
		t.Fatalf("overlong credential window accepted: %v", e)
	}
}
