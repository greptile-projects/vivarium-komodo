package agentdiscovery

import (
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprofiles"
)

func TestExplainableAudienceProjectedComparison(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now }
	profile := agentprofiles.Profile{ID: "agt_one", Handle: "review-helper", CurrentVersion: 2, Versions: []agentprofiles.Version{{Number: 2, Input: agentprofiles.Input{DisplayName: "Review Helper", SupportedTasks: []string{"code review"}, RequestedCapabilities: []string{"contents:read"}, Execution: agentprofiles.Execution{Regions: []string{"EU"}, Boundary: "operator EU runtime"}, DataUse: agentprofiles.DataTerms{Purposes: []string{"requested review"}, Retention: "24 hours", TrainingUse: "never used for training"}, Pricing: agentprofiles.Pricing{Currency: "USD", Amount: 2}, Availability: "weekdays UTC"}}}}
	_, err = s.AddEvidence("repo", "owner", EvidenceInput{ProfileID: profile.ID, ProfileVersion: 1, Audience: "public", Kind: "evaluation", Workflow: "code review", Summary: "Passed a sanitized review suite", SourceType: "evaluation", SourceID: "eval_public", ComparableTags: []string{"Go"}, Result: "passed", ObservedAt: now.Add(-200 * 24 * time.Hour)}, profile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AddEvidence("repo", "owner", EvidenceInput{ProfileID: profile.ID, ProfileVersion: 2, Audience: "repository", Kind: "outcome", Workflow: "code review", Summary: "Accepted private fix", SourceType: "pull_request", SourceID: "private_pull", ComparableTags: []string{"Go"}, Result: "accepted", ConflictOfInterest: "operator funded the evaluation", ObservedAt: now}, profile)
	if err != nil {
		t.Fatal(err)
	}
	x, err := s.Search("repo", "reader", SearchInput{ContextType: "issue", ContextID: "private-issue", PublicSummary: "Review a Go change", Audience: "public", Workflow: "code review", RequiredPermissions: []string{"contents:read"}, AllowedBoundaries: []string{"EU"}, RequiredPolicyTerms: []string{"never used for training"}, ComparableTags: []string{"Go"}, MaximumCost: 3, Currency: "USD", AvailabilityTerms: []string{"UTC"}}, []agentprofiles.Profile{profile})
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Matches) != 1 || !x.Matches[0].Eligible || len(x.Matches[0].Evidence) != 2 || len(x.Matches[0].StaleEvidence) != 1 || len(x.Matches[0].Conflicts) != 1 {
		t.Fatalf("incomplete comparison: %+v", x.Matches)
	}
	public, err := s.Get(x.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if public.ContextID != "" || public.CreatedBy != "" || public.RepositoryID != "" || len(public.Matches[0].Evidence) != 1 || len(public.Matches[0].MissingEvidence) != 1 || len(public.Matches[0].Conflicts) != 0 || public.Matches[0].Evidence[0].SourceID != "eval_public" {
		t.Fatalf("private context or evidence leaked: %+v", public)
	}
}
