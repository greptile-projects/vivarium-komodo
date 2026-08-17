package apiconsumers

import (
	"testing"
	"time"
)

func TestOperationalEvidenceProjectionAndSharedDiagnosis(t *testing.T) {
	s, a := integrationApplication(t)
	start := time.Now().UTC().Add(-time.Hour)
	end := start.Add(time.Hour)
	latency := float64(840)
	conformant := false
	shared, err := s.RecordObservation("producer", a.ID, "consumer", false, ObservationInput{Kind: "error", Audience: "shared", ReleaseID: "release-1", Environment: "sandbox", OperationID: "list", WindowStart: start, WindowEnd: end, ErrorCode: "decode_failed", SchemaConformant: &conformant, Summary: "Client rejected the sanitized response shape", InaccessibleEvidence: []string{"private payload retained by consumer"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.RecordObservation("producer", a.ID, "producer", true, ObservationInput{Kind: "latency", Audience: "producer", ReleaseID: "release-1", Environment: "sandbox", OperationID: "list", WindowStart: start, WindowEnd: end, LatencyMilliseconds: &latency, Summary: "Provider aggregate latency"})
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := s.Get("producer", a.ID, "consumer", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(consumer.Observations) != 1 || consumer.Observations[0].ID != shared.ID {
		t.Fatalf("producer-only evidence crossed boundary: %#v", consumer.Observations)
	}
	i, err := s.OpenInvestigation("producer", a.ID, "consumer", false, InvestigationInput{Title: "list response decoding failure", ObservationIDs: []string{shared.ID}})
	if err != nil {
		t.Fatal(err)
	}
	i, err = s.InviteAgent("producer", a.ID, i.ID, "producer", true, "agent:diagnostic")
	if err != nil {
		t.Fatal(err)
	}
	if i.Invitations[0].Scope != "read_only_sanitized_evidence_and_thread" {
		t.Fatalf("agent scope expanded: %#v", i.Invitations)
	}
	view, err := s.GetInvestigation("producer", a.ID, i.ID, "agent:diagnostic", false)
	if err != nil || len(view.Evidence) != 1 || view.Evidence[0].ID != shared.ID {
		t.Fatalf("invited agent cannot read bounded evidence: %#v %v", view, err)
	}
	i, err = s.AddEntry("producer", a.ID, i.ID, "agent:diagnostic", false, EntryInput{Kind: "finding", Body: "The synthetic failure reproduces at the contract boundary", EvidenceIDs: []string{shared.ID}})
	if err != nil {
		t.Fatal(err)
	}
	i, err = s.Reproduce("producer", a.ID, i.ID, "agent:diagnostic", false, SandboxInput{OperationID: "list", Failure: "unavailable", Body: map[string]any{"synthetic": true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(i.Reproductions) != 1 || i.Reproductions[0].Inspection.RequestHeaders["authorization"] != "[REDACTED]" {
		t.Fatalf("unsafe reproduction: %#v", i.Reproductions)
	}
	i, err = s.AddEntry("producer", a.ID, i.ID, "producer", true, EntryInput{Kind: "decision", Body: "Published schema and implementation differ", EvidenceIDs: []string{shared.ID}, Classification: "contract"})
	if err != nil {
		t.Fatal(err)
	}
	i, err = s.RouteChange("producer", a.ID, i.ID, "producer", true, RouteInput{DefectOwner: "provider", ResourceKind: "issue", ResourceID: "issue-7", RepositoryID: "producer", Revision: "producer-sha"})
	if err != nil {
		t.Fatal(err)
	}
	if i.Status != "routed" || i.ChangeRoutes[0].ResourceID != "issue-7" {
		t.Fatalf("defect not governed: %#v", i)
	}
}

func TestOperationalEvidenceRejectsSecretsAndWrongSideAudience(t *testing.T) {
	s, a := integrationApplication(t)
	start := time.Now().UTC().Add(-time.Hour)
	base := ObservationInput{Kind: "usage", Audience: "shared", ReleaseID: "r1", Environment: "sandbox", WindowStart: start, WindowEnd: start.Add(time.Hour), Summary: "Authorization: Bearer vka_secret"}
	if _, e := s.RecordObservation("producer", a.ID, "consumer", false, base); e != ErrInvalid {
		t.Fatalf("secret accepted: %v", e)
	}
	base.Summary = "bounded usage"
	base.Audience = "producer"
	if _, e := s.RecordObservation("producer", a.ID, "consumer", false, base); e != ErrForbidden {
		t.Fatalf("consumer published producer-private evidence: %v", e)
	}
}
