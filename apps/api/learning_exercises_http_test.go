package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningexercises"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestLearnerPracticesExactProjectRevisionWithoutSharedAuthority(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pathways, _ := learningpathways.New(t.TempDir())
	attempts, _ := learningexercises.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "practice", Visibility: repositories.Public})
	opened, _ := repos.Open(repo.ID)
	blob, _ := opened.WriteObject(storage.BlobObject, []byte("package practice\n"))
	tree, _ := opened.WriteObject(storage.TreeObject, append([]byte("100644 practice.go\x00"), objectIDBytes(t, blob)...))
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor O <o@example.test> 1 +0000\ncommitter O <o@example.test> 1 +0000\n\nPractice\n", tree)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	exercise := learningpathways.Exercise{Title: "Debug the parser", Kinds: []string{"exploration", "commands", "debugging", "tests", "api", "small_change"}, Instructions: "Inspect, reproduce, make an unpublished fix, and rerun the focused test.", AcceptanceCriteria: []string{"Focused test passes"}, Tools: []learningpathways.Tool{{Name: "go", Version: "1.25.0"}}, Data: []learningpathways.Data{{Name: "malformed-request.json", Kind: "synthetic", Digest: "sha256:fixture"}}, SetupCommands: []string{"go test ./..."}, MaximumCost: 3}
	_, err := pathways.Publish(string(repo.ID), "backend", "owner", 0, learningpathways.VersionInput{Role: "Backend contributor", Outcome: "Debug safely", Prerequisites: []string{"Go"}, Objectives: []string{"Recover from mistakes"}, SupportedRevisions: []string{string(commit)}, Modules: []learningpathways.Module{{ID: "parser", Title: "Parser", WhyItMatters: "Requests depend on it", Objectives: []string{"Debug"}, ExpectedEffortMinutes: 30, Exercises: []learningpathways.Exercise{exercise}, Resources: []learningpathways.Resource{{Kind: "symbol", Label: "Parser", Path: "practice.go", Symbol: "Parse", Revision: string(commit)}}}}, MentorIDs: []string{"owner"}, ExpectedEffortMinutes: 30, LearnerEnvironments: []learningpathways.Environment{{Name: "browser workspace", Requirement: "none", Supported: true}}, CompletionEvidence: []string{"check"}, ChangeReason: "safe practice"})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerLearningExercisesHTTP(mux, attempts, pathways, repos, credentials, nil)
	server := httptest.NewServer(mux)
	defer server.Close()
	learner := issueAccess(t, credentials, "learner", auth.API, auth.RepositoryRead)
	base := server.URL + "/repositories/" + string(repo.ID) + "/learning-pathways/backend/attempts"
	request := func(method, url, body string) (*http.Response, learningexercises.Attempt) {
		t.Helper()
		req, _ := http.NewRequest(method, url, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+learner)
		req.Header.Set("Content-Type", "application/json")
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		var a learningexercises.Attempt
		_ = json.NewDecoder(res.Body).Decode(&a)
		res.Body.Close()
		return res, a
	}
	res, a := request("POST", base, fmt.Sprintf(`{"pathway_version":1,"module_id":"parser","exercise_index":0}`))
	if res.StatusCode != 201 || a.Revision != string(commit) || !a.Detached || a.Published || a.Bounds.Network != "disabled" || a.Bounds.Credentials || a.Bounds.ProductionData || a.Bounds.AuthoritativeBranches || len(a.Exercise.Tools) != 1 {
		t.Fatalf("unsafe launch status=%d attempt=%#v", res.StatusCode, a)
	}
	res, _ = request("POST", base+"/"+a.ID+"/events", `{"kind":"output","summary":"token=learner-secret"}`)
	if res.StatusCode != 422 {
		t.Fatalf("credential-shaped output status=%d", res.StatusCode)
	}
	for _, event := range []string{`{"kind":"setup","summary":"Synthetic fixture loaded"}`, `{"kind":"command","summary":"Ran focused test","command":"go test ./...","output":"failed"}`, `{"kind":"hint","summary":"Inspect the error boundary"}`, `{"kind":"checkpoint","summary":"Saved learner-only change","digest":"sha256:checkpoint"}`, `{"kind":"recovery","summary":"Restored the checkpoint after a mistake"}`, `{"kind":"check","summary":"Focused test passes","output":"ok","digest":"sha256:check","cost":1.25}`, `{"kind":"complete","summary":"Exercise complete"}`} {
		res, a = request("POST", base+"/"+a.ID+"/events", event)
		if res.StatusCode != 201 {
			t.Fatalf("event status=%d %#v", res.StatusCode, a)
		}
	}
	if a.Status != "completed" || !a.Reproducible || a.HintsUsed != 1 || a.Cost != 1.25 || len(a.Events) != 7 || a.Published {
		t.Fatalf("incomplete evidence %#v", a)
	}
	help := func(token, body string, want int) learningexercises.Attempt {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, base+"/"+a.ID+"/help", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var got learningexercises.Attempt
		_ = json.NewDecoder(response.Body).Decode(&got)
		if response.StatusCode != want {
			t.Fatalf("help status=%d want=%d body=%s", response.StatusCode, want, body)
		}
		return got
	}
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead)
	a = help(learner, `{"kind":"question","recipient_kind":"mentor","recipient_id":"owner","body":"Why does this boundary reject the request?","shared_event_numbers":[2,4],"workspace_access":"join","learner_authorized":true}`, 201)
	if len(a.HelpTimeline) != 1 || len(a.HelpTimeline[0].SharedEvents) != 2 || len(a.HelpTimeline[0].SharedEvents) == len(a.Events) {
		t.Fatalf("unbounded shared state %#v", a.HelpTimeline)
	}
	citation := fmt.Sprintf(`{"kind":"symbol","label":"Parser","path":"practice.go","revision":%q}`, commit)
	a = help(owner, `{"kind":"guidance","guidance_kind":"hint","body":"Compare the parser error path with the rejected input.","citations":[`+citation+`]}`, 201)
	a = help(owner, `{"kind":"join","workspace_access":"join"}`, 201)
	if len(a.HelpTimeline) != 3 || a.HelpTimeline[1].AuthorID != "owner" || a.HelpTimeline[1].GuidanceKind != "hint" || !a.HelpTimeline[2].LearnerAuthorized {
		t.Fatalf("missing accountable help %#v", a.HelpTimeline)
	}
	req, _ := http.NewRequest(http.MethodGet, base+"/"+a.ID, nil)
	req.Header.Set("Authorization", "Bearer "+owner)
	mentorView, _ := http.DefaultClient.Do(req)
	var projected learningexercises.Attempt
	_ = json.NewDecoder(mentorView.Body).Decode(&projected)
	mentorView.Body.Close()
	if mentorView.StatusCode != 200 || len(projected.Events) != 0 || len(projected.Grounding) != 0 || len(projected.HelpTimeline[0].SharedEvents) != 2 {
		t.Fatalf("unsafe mentor projection status=%d %#v", mentorView.StatusCode, projected)
	}
	help(owner, `{"kind":"guidance","guidance_kind":"hint","body":"Authorization: Bearer leaked","citations":[`+citation+`]}`, 422)
	res, _ = request("POST", base+"/"+a.ID+"/events", `{"kind":"output","summary":"Authorization: Bearer leaked"}`)
	if res.StatusCode != 409 {
		t.Fatalf("terminal attempt should reject changes: %d", res.StatusCode)
	}
	res, _ = request("POST", base, `{"pathway_version":1,"module_id":"missing","exercise_index":0}`)
	if res.StatusCode != 422 {
		t.Fatalf("invalid module status=%d", res.StatusCode)
	}
}
