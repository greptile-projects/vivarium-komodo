package previews

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAttemptRetainsImmutableAttestationAndLifecycle(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	definition := Definition{Version: 1, Start: "serve", Port: 3000, Resources: Resources{CPUSeconds: 60, MemoryMB: 256, DiskMB: 512, BuildTimeoutSeconds: 30, LifetimeMinutes: 15}}
	item, err := store.Create(Preview{RepositoryID: "repo", SourceRepositoryID: "fork", PullRequestID: "pull", Revision: "revision-one", CreatorID: "collaborator", Definition: definition, Configuration: map[string]string{"MODE": "secret-review-value"}, Attestation: Attestation{CommitID: "revision-one", DefinitionDigest: "definition", ConfigurationDigest: "configuration"}})
	if err != nil {
		t.Fatal(err)
	}
	if item.State != "setting_up" || len(item.Events) != 1 || item.ExpiresAt.IsZero() {
		t.Fatalf("created preview = %#v", item)
	}
	if _, err = store.Transition(item.ID, "ready", "/preview", "", 4000); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("repo", "pull", item.ID)
	if err != nil || got.Revision != "revision-one" || got.Attestation.DefinitionDigest != "definition" || got.State != "ready" || got.LocalPort != 4000 {
		t.Fatalf("stored preview = %#v, %v", got, err)
	}
	raw, _ := json.Marshal(got)
	if !json.Valid(raw) || strings.Contains(string(raw), "secret-review-value") {
		t.Fatal("configuration value leaked into durable/public preview")
	}
}

func TestFindingRetainsExactContextRedactsEvidenceAndLinksDuplicate(t *testing.T) {
	store, _ := New(t.TempDir())
	item, _ := store.Create(Preview{RepositoryID: "repo", PullRequestID: "pull", Revision: "candidate-one", Definition: Definition{Resources: Resources{LifetimeMinutes: 60}}})
	input := Finding{Route: "/checkout?tab=payment&token=private", Title: "Card form fails", Description: "Submitting the form did not advance.", ReproductionSteps: []string{"Open checkout", "Submit the form"}, Evidence: []Evidence{{Kind: "console", Name: "console.txt", MediaType: "text/plain", Content: base64.StdEncoding.EncodeToString([]byte("ready\nAuthorization: Bearer private\nfailed"))}}}
	created, err := store.AddFinding("repo", "pull", item.ID, "stakeholder", input)
	if err != nil || len(created.Findings) != 1 {
		t.Fatalf("finding = %#v, %v", created, err)
	}
	finding := created.Findings[0]
	if finding.Revision != "candidate-one" || finding.Route != "/checkout?tab=payment&token=%5Bredacted%5D" || !finding.Evidence[0].Redacted || finding.Evidence[0].Content != "" {
		t.Fatalf("retained context = %#v", finding)
	}
	_, body, err := store.ReadEvidence("repo", "pull", item.ID, finding.ID, finding.Evidence[0].ID)
	if err != nil || strings.Contains(string(body), "private") || !strings.Contains(string(body), "[redacted sensitive field]") {
		t.Fatalf("redacted body = %q, %v", body, err)
	}
	duplicate, err := store.AddFinding("repo", "pull", item.ID, "reviewer", Finding{Route: "/checkout?token=other&tab=payment", Title: "card form fails", Description: "Still fails.", ReproductionSteps: []string{"Submit"}})
	if err != nil || duplicate.Findings[1].DuplicateOf != finding.ID {
		t.Fatalf("duplicate = %#v, %v", duplicate.Findings, err)
	}
	updated, err := store.UpdateFinding("repo", "pull", item.ID, finding.ID, "maintainer", "bug", "resolved", "", []string{duplicate.Findings[1].ID})
	if err != nil || updated.Findings[0].Status != "resolved" || updated.Findings[0].Classification != "bug" || len(updated.Findings[0].RelatedFindingIDs) != 1 {
		t.Fatalf("updated finding = %#v, %v", updated.Findings[0], err)
	}
}

func TestFindingLinksScopedWorkAndRepairProvenance(t *testing.T) {
	store, _ := New(t.TempDir())
	item, _ := store.Create(Preview{RepositoryID: "repo", PullRequestID: "pull", Revision: "observed", Definition: Definition{Resources: Resources{LifetimeMinutes: 60}}})
	created, _ := store.AddFinding("repo", "pull", item.ID, "reporter", Finding{Route: "/", Title: "Broken", Description: "Observed failure", ReproductionSteps: []string{"Submit"}, Evidence: []Evidence{{Kind: "console", Name: "console.txt", MediaType: "text/plain", Content: base64.StdEncoding.EncodeToString([]byte("failed"))}}})
	f := created.Findings[0]
	linked, err := store.LinkFindingWork("repo", "pull", item.ID, f.ID, "maintainer", FindingWork{Kind: "workspace", ProposalID: "proposal", TaskID: "task", WorkspaceID: "workspace", OwnerKind: "human", OwnerID: "developer", AcceptanceCriteria: []string{"Submission succeeds"}, EvidenceIDs: []string{f.Evidence[0].ID}})
	if err != nil || linked.Work == nil || linked.Work.WorkspaceID != "workspace" || linked.Work.CreatedByID != "maintainer" {
		t.Fatalf("linked work = %#v, %v", linked.Work, err)
	}
	if _, err = store.LinkFindingWork("repo", "pull", item.ID, f.ID, "maintainer", FindingWork{Kind: "task", OwnerID: "developer", AcceptanceCriteria: []string{"again"}}); err == nil {
		t.Fatal("finding accepted replacement work")
	}
	repaired, err := store.RecordRepair("repo", "pull", item.ID, f.ID, "developer", RepairPublication{Revision: "fixed", CommitIDs: []string{"fixed"}, Commands: []string{"bun test"}, Checks: []string{"web"}, AuthorIDs: []string{"developer"}, WorkspaceID: "workspace", PreviewID: "next-preview"})
	if err != nil || repaired.Status != "resolved" || len(repaired.Repairs) != 1 || repaired.Repairs[0].PublishedByID != "developer" || repaired.History[len(repaired.History)-1].Type != "finding.repair_published" {
		t.Fatalf("repair = %#v, %v", repaired, err)
	}
}

func TestInvitationExpiresRevokesAndAuditsWithoutAuthority(t *testing.T) {
	store, _ := New(t.TempDir())
	store.now = func() time.Time { return time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC) }
	item, _ := store.Create(Preview{RepositoryID: "repo", PullRequestID: "pull", Definition: Definition{Resources: Resources{LifetimeMinutes: 60}}})
	invited, err := store.Invite("repo", "pull", item.ID, "owner", Invitation{UserID: "customer", Role: "test", SourceKind: "issue", SourceID: "issue-1", ExpiresAt: store.now().Add(30 * time.Minute)})
	if err != nil || len(invited.Invitations) != 1 || len(invited.AccessEvents) != 1 {
		t.Fatalf("invite = %#v, %v", invited, err)
	}
	_, grant, err := store.Authorize("repo", "pull", item.ID, "customer")
	if err != nil || grant.Role != "test" {
		t.Fatalf("authorize = %#v, %v", grant, err)
	}
	revoked, err := store.Revoke("repo", "pull", item.ID, grant.ID, "owner")
	if err != nil || revoked.Invitations[0].RevokedAt == nil || len(revoked.AccessEvents) != 3 {
		t.Fatalf("revoke = %#v, %v", revoked, err)
	}
	if _, _, err = store.Authorize("repo", "pull", item.ID, "customer"); err == nil {
		t.Fatal("revoked invitation authorized")
	}
}
