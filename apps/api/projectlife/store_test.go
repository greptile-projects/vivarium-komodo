package projectlife

import (
	"errors"
	"testing"
	"time"
)

func TestPublicLifeRetainsEvidenceWorkAndResolvedDisposition(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	in := Input{IncubatorID: "inc", AlternativeID: "alt", BoundaryID: "boundary", BoundaryRevision: 2, DeliveryID: "delivery", DeliveryRevision: 9, ReadinessID: "ready", ReadinessRevision: 15, LaunchRevision: "release-1", Audience: "public", OwnerIDs: []string{"owner"}}
	v, err := s.Create("owner", in)
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.Publish(v.ID, "owner", v.Revision, Publication{Kind: "release", Revision: "release-1", Reference: "release:r1", Digest: "sha256:release", Audience: "public", Attestation: "ordinary checks, review, and owner approval"})
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.Observe(v.ID, "owner", v.Revision, Signal{Kind: "adoption", Measure: "weekly users", Value: 12, Unit: "users", EvidenceReference: "metric:users", EvidenceDigest: "sha256:users", ObservedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.AddFeedback(v.ID, "owner", v.Revision, Feedback{Audience: "public", Summary: "publish a Go example", EvidenceReference: "support:q1"})
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.AddWork(v.ID, "owner", v.Revision, Work{FeedbackID: v.Feedback[0].ID, Kind: "agent", OwnerID: "agent:docs", Title: "draft example", Reference: "task:t1"})
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.ReviseRoadmap(v.ID, "owner", v.Revision, RoadmapChange{FeedbackID: v.Feedback[0].ID, Revision: "roadmap-2", Summary: "prioritize adoption docs", EvidenceReferences: []string{"support:q1", "metric:users"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Decide(v.ID, "owner", v.Revision, Disposition{State: "archived", Reason: "merge learning elsewhere"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("unresolved archive accepted: %v", err)
	}
	v, err = s.Decide(v.ID, "owner", v.Revision, Disposition{State: "graduated", TargetReference: "organization:compiler", Reason: "sustained use", Obligations: []Obligation{{Kind: "repository", ResourceReference: "repo:compiler", Resolution: "transferred to accountable organization owners", EvidenceReference: "decision:d1"}}})
	if err != nil || v.Disposition.State != "graduated" || len(v.Blockers) != 0 || v.AuthorityGranted {
		t.Fatalf("unsafe project life result: %#v, %v", v, err)
	}
}

func TestPublicationIsLaunchRevisionExact(t *testing.T) {
	s, _ := New(t.TempDir())
	v, _ := s.Create("owner", Input{IncubatorID: "i", AlternativeID: "a", BoundaryID: "b", BoundaryRevision: 1, DeliveryID: "d", DeliveryRevision: 1, ReadinessID: "r", ReadinessRevision: 1, LaunchRevision: "one", Audience: "limited", OwnerIDs: []string{"owner"}})
	_, err := s.Publish(v.ID, "owner", v.Revision, Publication{Kind: "documentation", Revision: "two", Reference: "docs", Digest: "sha256:x", Audience: "limited", Attestation: "reviewed"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("revision drift accepted: %v", err)
	}
}
