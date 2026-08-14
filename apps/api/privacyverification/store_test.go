package privacyverification

import (
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
)

func TestCurrentSyntheticEvidenceAcknowledgementAndExceptions(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	p, err := s.Create("repo", "maintainer", PolicyInput{Name: "Runtime privacy", CommitmentID: "commitment", CommitmentVersion: 2, TargetBranches: []string{"main"}, Paths: []string{"app"}, RequiredChecks: []string{"privacy/journey"}, RequiredDimensions: []string{"consent", "deletion", "recipient"}, PrivacyOwnerIDs: []string{"privacy-owner"}})
	if err != nil {
		t.Fatal(err)
	}
	run := checkruns.Run{ID: "run", CommitID: "candidate", State: checkruns.Succeeded, Definition: checkruns.Definition{Name: "privacy/journey", Privacy: &checkruns.PrivacySpec{JourneyIDs: []string{"account-lifecycle"}, Dimensions: []string{"consent", "deletion"}, Inputs: []string{"app/privacy.go"}, CommitmentIDs: []string{"commitment"}, SyntheticData: true, RequiresPreview: true}}}
	a, err := s.Assess("repo", "pull", "candidate", "main", []string{"app/privacy.go"}, []checkruns.Run{run})
	if err != nil || a.Ready {
		t.Fatalf("missing owner and recipient coverage must block: %#v %v", a, err)
	}
	if _, err = s.Acknowledge("repo", "pull", p.ID, "preview", "candidate", "accept", "synthetic evidence matches the commitment", "other"); err == nil {
		t.Fatal("non-owner acknowledged")
	}
	if _, err = s.Acknowledge("repo", "pull", p.ID, "preview", "candidate", "accept", "synthetic evidence matches the commitment", "privacy-owner"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Except("repo", "pull", p.ID, "candidate", "recipient is covered by the queued follow-up", "privacy-owner", nil, []string{"recipient"}, FollowUp{Kind: "issue", ResourceID: "issue-7"}, now.Add(7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	a, _ = s.Assess("repo", "pull", "candidate", "main", []string{"app/privacy.go"}, []checkruns.Run{run})
	if !a.Ready || len(a.ActiveExceptions) != 1 {
		t.Fatalf("bounded exception and acknowledgement did not satisfy policy: %#v", a)
	}
	a, _ = s.Assess("repo", "pull", "next", "main", []string{"app/privacy.go"}, []checkruns.Run{run})
	if a.Ready {
		t.Fatal("stale evidence and acknowledgement satisfied a later revision")
	}
	now = now.Add(8 * 24 * time.Hour)
	a, _ = s.Assess("repo", "pull", "candidate", "main", []string{"app/privacy.go"}, []checkruns.Run{run})
	if a.Ready {
		t.Fatal("expired exception satisfied missing coverage")
	}
}

func TestPolicyOnlyAppliesToMatchingBranchAndPath(t *testing.T) {
	s, _ := New(t.TempDir())
	_, _ = s.Create("repo", "owner", PolicyInput{Name: "Privacy", CommitmentID: "c", CommitmentVersion: 1, TargetBranches: []string{"main"}, Paths: []string{"service"}, RequiredChecks: []string{"p"}, RequiredDimensions: []string{"collection"}, PrivacyOwnerIDs: []string{"owner"}})
	a, _ := s.Assess("repo", "pull", "r", "main", []string{"docs/readme.md"}, nil)
	if !a.Ready || len(a.AppliedPolicyIDs) != 0 {
		t.Fatalf("unaffected change was blocked: %#v", a)
	}
}
