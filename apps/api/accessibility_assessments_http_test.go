package main

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilityassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitybarriers"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
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
	mux := http.NewServeMux()
	registerAccessibilityAssessmentsHTTP(mux, assessments, catalog, credentials, accessibilityAssessmentSources{pulls: pulls, runs: runs, previews: previewStore, barriers: barriers, repositories: catalog})
	server := httptest.NewServer(mux)
	defer server.Close()
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	specialist := issueAccess(t, credentials, "specialist", auth.API, auth.RepositoryRead)
	base := "/repositories/" + string(record.ID) + "/pull-requests/" + pull.ID + "/accessibility-assessments"
	var a accessibilityassessments.Assessment
	workflowJSON(t, server.URL, http.MethodPost, base, owner, `{"revision":"`+string(commit)+`","scenarios":[{"id":"checkout","name":"Complete checkout","journey":"Review and pay","affected_audiences":["keyboard users","screen reader users"],"required_evaluations":["semantics","keyboard","contrast"],"source_locations":[{"path":"checkout.html"}]}]}`, 201, &a)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+a.ID+"/automation", owner, `{"run_id":"`+run.ID+`"}`, 201, &a)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+a.ID+"/findings", specialist, `{"scenario_id":"checkout","evaluation":"keyboard","result":"barrier","severity":"high","affected_audiences":["keyboard users"],"source_locations":[{"path":"checkout.html","start_line":1,"end_line":1}],"summary":"Focus disappears after activation","uncertainty":"Chromium only","requires_human_evaluation":true,"citation":{"kind":"preview","resource_id":"`+preview.ID+`"}}`, 201, &a)
	if len(a.Gaps) != 0 || len(a.Automation) != 1 || len(a.Findings) != 1 || a.Findings[0].ActorID != "specialist" || a.Findings[0].Locations[0].BlobID != string(page) {
		t.Fatalf("assessment projection lost evidence: %#v", a)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+a.ID+"/findings/"+a.Findings[0].ID+"/decisions", owner, `{"outcome":"false_positive","rationale":"The cited node is hidden while focus moves to the visible dialog"}`, 201, &a)
	if len(a.Findings[0].Decisions) != 1 || a.Findings[0].Decisions[0].ActorID != "owner" {
		t.Fatalf("judgment attribution lost: %#v", a)
	}
}
