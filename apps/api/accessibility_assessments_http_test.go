package main

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilityassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitybarriers"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitycommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestAccessibilityAssessmentCombinesAutomationAndAccountableExperience(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	record, _ := catalog.Create("owner", repositories.Metadata{Name: "inclusive", Visibility: repositories.Public})
	repository, _ := catalog.Open(record.ID)
	page, _ := repository.WriteObject(storage.BlobObject, []byte("<button>Pay</button>\n"))
	rawPage, _ := hex.DecodeString(string(page))
	treeData := append([]byte("100644 checkout.html\x00"), rawPage...)
	root, _ := repository.WriteObject(storage.TreeObject, treeData)
	commit, _ := repository.WriteObject(storage.CommitObject, []byte("tree "+string(root)+"\nauthor A <a@example.com> 1 +0000\ncommitter A <a@example.com> 1 +0000\n\ncheckout\n"))
	pulls, _ := pullrequests.New(t.TempDir())
	pull, err := pulls.Create(pullrequests.CreateParams{RepositoryID: string(record.ID), AuthorID: "owner", Title: "Accessible checkout", SourceBranch: "feature", TargetBranch: "main", SourceCommitID: string(commit), TargetCommitID: string(commit)})
	if err != nil {
		t.Fatal(err)
	}
	runs, _ := checkruns.New(t.TempDir())
	run, _ := runs.Create(string(record.ID), pull.ID, string(commit), checkruns.Definition{Name: "checkout-a11y", Accessibility: &checkruns.AccessibilitySpec{ScenarioIDs: []string{"checkout"}, Evaluations: []string{"semantics", "contrast"}, Inputs: []string{"checkout.html"}, AffectedAudiences: []string{"screen reader users"}, RequiresHumanEvaluation: []string{"keyboard"}}})
	started, _ := runs.Start(run.ID)
	run, _ = runs.Complete(started.ID, 0, false, "")
	previewStore, _ := previews.New(t.TempDir())
	preview, _ := previewStore.Create(previews.Preview{RepositoryID: string(record.ID), PullRequestID: pull.ID, Revision: string(commit), Definition: previews.Definition{Resources: previews.Resources{LifetimeMinutes: 60}}})
	barriers, _ := accessibilitybarriers.New(t.TempDir())
	assessments, _ := accessibilityassessments.New(t.TempDir())
	commitments, _ := accessibilitycommitments.New(t.TempDir())
	commitment, _ := commitments.Create(string(record.ID), "owner", accessibilitycommitments.VersionInput{Title: "Checkout access", Scopes: []accessibilitycommitments.Scope{{Kind: "component", ResourceID: "checkout", Name: "Checkout"}}, Standards: []accessibilitycommitments.Standard{{ID: "wcag", Name: "WCAG", Version: "2.2", Level: "AA"}}, AssistiveTechnologies: []accessibilitycommitments.AssistiveTechnology{{ID: "keyboard", Name: "Keyboard", Platform: "web"}}, TargetAudiences: []string{"keyboard users"}, Scenarios: []accessibilitycommitments.Scenario{{ID: "checkout", Name: "Complete checkout", ScopeIDs: []string{"component:checkout"}, StandardIDs: []string{"wcag"}, AssistiveTechnologyIDs: []string{"keyboard"}}}, SeverityPolicy: []accessibilitycommitments.SeverityRule{{Severity: "high", Definition: "Cannot complete checkout", ReviewEffect: "block_review"}}, OwnerIDs: []string{"owner"}, Links: []accessibilitycommitments.Link{{Kind: "preview", ResourceID: preview.ID}}, ChangeReason: "Define the repair contract"})
	plans, _ := proposals.New(t.TempDir())
	proposal, _ := plans.Create(string(record.ID), "owner", "Accessible checkout", "Repair confirmed barriers")
	mux := http.NewServeMux()
	registerAccessibilityAssessmentsHTTP(mux, assessments, catalog, credentials, accessibilityAssessmentSources{pulls: pulls, runs: runs, previews: previewStore, barriers: barriers, repositories: catalog, commitments: commitments, plans: plans})
	server := httptest.NewServer(mux)
	defer server.Close()
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	specialist := issueAccess(t, credentials, "specialist", auth.API, auth.RepositoryRead)
	base := "/repositories/" + string(record.ID) + "/pull-requests/" + pull.ID + "/accessibility-assessments"
	var a accessibilityassessments.Assessment
	workflowJSON(t, server.URL, http.MethodPost, base, owner, `{"revision":"`+string(commit)+`","commitment_id":"`+commitment.ID+`","commitment_version":1,"scenarios":[{"id":"checkout","name":"Complete checkout","journey":"Review and pay","affected_audiences":["keyboard users","screen reader users"],"required_evaluations":["semantics","keyboard","contrast"],"source_locations":[{"path":"checkout.html"}]}]}`, 201, &a)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+a.ID+"/automation", owner, `{"run_id":"`+run.ID+`"}`, 201, &a)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+a.ID+"/findings", specialist, `{"scenario_id":"checkout","evaluation":"keyboard","result":"barrier","severity":"high","affected_audiences":["keyboard users"],"source_locations":[{"path":"checkout.html","start_line":1,"end_line":1}],"summary":"Focus disappears after activation","uncertainty":"Chromium only","requires_human_evaluation":true,"citation":{"kind":"preview","resource_id":"`+preview.ID+`"}}`, 201, &a)
	if len(a.Gaps) != 0 || len(a.Automation) != 1 || len(a.Findings) != 1 || a.Findings[0].ActorID != "specialist" || a.Findings[0].Locations[0].BlobID != string(page) {
		t.Fatalf("assessment projection lost evidence: %#v", a)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+a.ID+"/findings/"+a.Findings[0].ID+"/decisions", owner, `{"outcome":"confirmed","rationale":"Keyboard-only reproduction confirms focus is lost"}`, 201, &a)
	if len(a.Findings[0].Decisions) != 1 || a.Findings[0].Decisions[0].ActorID != "owner" {
		t.Fatalf("judgment attribution lost: %#v", a)
	}
	var linked struct {
		Repair accessibilityassessments.Repair `json:"repair"`
		Task   proposals.Task                  `json:"task"`
	}
	work := base + "/" + a.ID + "/findings/" + a.Findings[0].ID + "/repairs"
	workflowJSON(t, server.URL, http.MethodPost, work, owner, `{"kind":"task","proposal_id":"`+proposal.ID+`","title":"Restore checkout focus","owner_kind":"agent","owner_id":"codex","commitment_id":"`+commitment.ID+`","commitment_version":1,"acceptance_criteria":["Focus remains visible after activation"],"component_guidance":["Use the shared checkout focus target"],"evidence_ids":[]}`, 201, &linked)
	if linked.Task.BaseRevision != string(commit) || linked.Task.ReasoningContext == nil || linked.Task.ReasoningContext.Kind != "accessibility_repair" || linked.Repair.OwnerID != "codex" {
		t.Fatalf("repair work lost governed context: %#v", linked)
	}
	repairPull, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(record.ID), ProposalID: proposal.ID, TaskID: linked.Task.ID, AuthorID: "owner", Title: "Restore focus", SourceBranch: "repair", TargetBranch: "main", SourceCommitID: string(commit), TargetCommitID: string(commit)})
	repairPreview, _ := previewStore.Create(previews.Preview{RepositoryID: string(record.ID), PullRequestID: repairPull.ID, Revision: string(commit), Definition: previews.Definition{Resources: previews.Resources{LifetimeMinutes: 60}}})
	delivery := work + "/" + linked.Repair.ID + "/delivery"
	workflowJSON(t, server.URL, http.MethodPost, delivery, owner, `{"pull_request_id":"`+repairPull.ID+`","revision":"`+string(commit)+`","preview_id":"`+repairPreview.ID+`","design_changes":["Keep focus visible"],"code_changes":["Use the shared focus target"],"interaction_tradeoffs":["Wait for the dialog before moving focus"],"content_tradeoffs":["Retain the existing payment label"]}`, 201, &struct{}{})
	got, _ := assessments.Get(string(record.ID), pull.ID, a.ID)
	if got.Findings[0].Repair.Delivery.PreviewID != repairPreview.ID || len(got.Findings[0].Repair.Progress) != 2 {
		t.Fatalf("delivery did not report to finding: %#v", got.Findings[0].Repair)
	}
}
