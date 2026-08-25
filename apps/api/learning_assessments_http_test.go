package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestPracticalLearningAssessmentRequiresDemonstratedCurrentWork(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pathways, _ := learningpathways.New(t.TempDir())
	assessments, _ := learningassessments.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "learn", Visibility: repositories.Public})
	opened, _ := repos.Open(repo.ID)
	blob, _ := opened.WriteObject(storage.BlobObject, []byte("package learn\n"))
	tree, _ := opened.WriteObject(storage.TreeObject, append([]byte("100644 learn.go\x00"), objectIDBytes(t, blob)...))
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor O <o@example.test> 1 +0000\ncommitter O <o@example.test> 1 +0000\n\nLearn\n", tree)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	exercise := learningpathways.Exercise{Title: "Repair", Kinds: []string{"debugging"}, Instructions: "Repair independently", AcceptanceCriteria: []string{"checks pass"}, Tools: []learningpathways.Tool{{Name: "go", Version: "1.25"}}, Data: []learningpathways.Data{{Name: "fixture", Kind: "synthetic", Digest: "sha256:data"}}, MaximumCost: 2}
	_, _ = pathways.Publish(string(repo.ID), "backend", "owner", 0, learningpathways.VersionInput{Role: "Backend", Outcome: "repair", Prerequisites: []string{"Go"}, Objectives: []string{"debug"}, SupportedRevisions: []string{string(commit)}, Modules: []learningpathways.Module{{ID: "debug", Title: "Debug", WhyItMatters: "reliability", Objectives: []string{"repair"}, ExpectedEffortMinutes: 30, Exercises: []learningpathways.Exercise{exercise}, Resources: []learningpathways.Resource{{Kind: "symbol", Label: "learn", Path: "learn.go", Revision: string(commit)}}}}, MentorIDs: []string{"reviewer"}, ExpectedEffortMinutes: 30, LearnerEnvironments: []learningpathways.Environment{{Name: "workspace", Requirement: "none", Supported: true}}, CompletionEvidence: []string{"assessment"}, ChangeReason: "initial"})
	mux := http.NewServeMux()
	registerLearningAssessmentsHTTP(mux, assessments, pathways, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	tokens := map[string]string{
		"owner":        issueAccess(t, credentials, "owner", auth.API, auth.RepositoryWrite),
		"learner":      issueAccess(t, credentials, "learner", auth.API, auth.RepositoryRead),
		"reviewer":     issueAccess(t, credentials, "reviewer", auth.API, auth.RepositoryRead),
		"appeal-owner": issueAccess(t, credentials, "appeal-owner", auth.API, auth.RepositoryRead),
	}
	base := server.URL + "/repositories/" + string(repo.ID) + "/learning-pathways/backend/assessments"
	call := func(actor, method, url, body string, want int) learningassessments.Assessment {
		t.Helper()
		req, _ := http.NewRequest(method, url, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokens[actor])
		req.Header.Set("Content-Type", "application/json")
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		var got learningassessments.Assessment
		_ = json.NewDecoder(res.Body).Decode(&got)
		if res.StatusCode != want {
			t.Fatalf("%s %s status=%d want=%d", method, url, res.StatusCode, want)
		}
		return got
	}
	definition := fmt.Sprintf(`{"id":"debug-readiness","title":"Debug readiness","summary":"Demonstrate a project repair","pathway_version":1,"revision":%q,"criteria":[{"id":"diagnosis","title":"Diagnosis","description":"Explain the exact fault","required":true,"human_judgment_required":true}],"protected_cases":[{"id":"edge","title":"Unseen malformed input","digest":"sha256:hidden","material":"the private answer and hidden fixture"}],"checks":[{"name":"focused","required":true}],"owner_ids":["owner"],"reviewer_ids":["reviewer"],"maximum_attempts":2,"appeal_owner_ids":["appeal-owner"]}`, commit)
	a := call("owner", "POST", base, definition, 201)
	encoded, _ := json.Marshal(a)
	if strings.Contains(string(encoded), "private answer") || len(a.ProtectedCaseMetadata) != 1 || a.Definition.ProtectedCases[0].Material != "" {
		t.Fatalf("protected case leaked: %s", encoded)
	}
	attemptBody := fmt.Sprintf(`{"revision":%q,"workspace_digest":"sha256:workspace","reproduction_commands":["go test ./..."],"assistance":["mentor gave one conceptual hint"],"accommodation_request":"extra time"}`, commit)
	a = call("learner", "POST", base+"/debug-readiness/attempts", attemptBody, 201)
	attempt := a.Attempts[0].ID
	a = call("owner", "POST", base+"/debug-readiness/attempts/"+attempt+"/accommodation", `{"status":"approved","rationale":"Preserves the same rubric"}`, 201)
	a = call("learner", "POST", base+"/debug-readiness/attempts/"+attempt+"/evidence", `{"kind":"repository_check","summary":"Focused check passed twice","reference":"check-run:1","digest":"sha256:check","check_name":"focused","check_status":"pass","flaky":true}`, 201)
	a = call("learner", "POST", base+"/debug-readiness/attempts/"+attempt+"/evidence", `{"kind":"explanation","summary":"Fault isolated without copying","reference":"artifact:explanation","digest":"sha256:explanation"}`, 201)
	judgment := `{"outcome":"pass","feedback":"Diagnosis matches the observed fault","rubric":[{"criterion_id":"diagnosis","decision":"pass","rationale":"Cites the learner explanation and check","evidence_numbers":[1,2]}],"integrity":{"copied_solution":false,"agent_overreach":false}}`
	a = call("reviewer", "POST", base+"/debug-readiness/attempts/"+attempt+"/judgments", judgment, 201)
	if a.Attempts[0].CompletionSupported || !assessmentContains(a.Attempts[0].Blockers, "flaky_check:focused") {
		t.Fatalf("flaky evidence supported completion: %#v", a.Attempts[0])
	}
	a = call("learner", "POST", base+"/debug-readiness/attempts/"+attempt+"/appeals", `{"reason":"The repeated run is stable","evidence_references":["check-run:2"]}`, 201)
	appeal := a.Attempts[0].Appeals[0].ID
	a = call("appeal-owner", "POST", base+"/debug-readiness/attempts/"+attempt+"/appeals/"+appeal, `{"decision":"reassessment","rationale":"Submit a stable repository check"}`, 201)
	if a.Attempts[0].Appeals[0].DecidedBy != "appeal-owner" {
		t.Fatalf("appeal not accountable: %#v", a.Attempts[0].Appeals)
	}
	a = call("learner", "POST", base+"/debug-readiness/attempts/"+attempt+"/evidence", `{"kind":"repository_check","summary":"Focused check passed repeatedly","reference":"check-run:2","digest":"sha256:stable","check_name":"focused","check_status":"pass","flaky":false}`, 201)
	a = call("reviewer", "POST", base+"/debug-readiness/attempts/"+attempt+"/judgments", judgment, 201)
	if !a.Attempts[0].CompletionSupported || a.Attempts[0].Status != "passed" {
		t.Fatalf("current demonstrated work did not pass: %#v", a.Attempts[0])
	}
	// A changed project revision prevents a fresh attempt against stale assessment material.
	blob2, _ := opened.WriteObject(storage.BlobObject, []byte("package learn\n// changed\n"))
	tree2, _ := opened.WriteObject(storage.TreeObject, append([]byte("100644 learn.go\x00"), objectIDBytes(t, blob2)...))
	commit2, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor O <o@example.test> 2 +0000\ncommitter O <o@example.test> 2 +0000\n\nChange\n", tree2, commit)))
	_ = opened.UpdateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit2})
	call("learner", "POST", base+"/debug-readiness/attempts", attemptBody, 422)
}

func assessmentContains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
