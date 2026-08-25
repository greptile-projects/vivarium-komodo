package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningoutcomes"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestLearningOutcomesImproveWithoutRewritingAchievement(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	store, _ := learningoutcomes.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "learn", Visibility: repositories.Public})
	mux := http.NewServeMux()
	registerLearningOutcomesHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	base := server.URL + "/repositories/" + string(repo.ID) + "/learning-pathways/backend/outcomes"
	post := func(path, body string, want int) learningoutcomes.Record {
		req, _ := http.NewRequest(http.MethodPost, base+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var got learningoutcomes.Record
		_ = json.NewDecoder(res.Body).Decode(&got)
		if res.StatusCode != want {
			t.Fatalf("%s status=%d record=%#v", path, res.StatusCode, got)
		}
		return got
	}
	post("/observations", `{"id":"setup-summary","kind":"setup_failure","module_id":"local-dev","pathway_version":1,"project_revision":"rev-1","audience":"repository","consent":"granted","count":7,"summary":"Windows setup repeatedly fails at the generated certificate step","evidence_references":["aggregate:setup/check-7"]}`, 201)
	post("/findings", `{"id":"setup-gap","kind":"setup_gap","module_id":"local-dev","summary":"The documented certificate command is not portable","observation_ids":["setup-summary"],"confidence":"supported","actor_kind":"agent"}`, 201)
	got := post("/improvements", `{"id":"portable-setup","finding_ids":["setup-gap"],"kind":"documentation","summary":"Use the repository helper on all supported platforms","base_pathway_version":1,"target_pathway_version":2,"project_revision":"rev-2","delivery_kind":"pull_request","delivery_id":"pull-42","delivery_revision":"candidate-2","review_status":"approved","reviewer_id":"owner","material":true,"requirement_changes":["Complete the portable certificate check"],"affected_learners":[{"learner_id":"learner-opaque-7","completion_evidence_id":"completion-v1-7","prior_pathway_version":1,"reason":"The setup guarantee is now assessed"}]}`, 201)
	if got.Improvements[0].AffectedLearners[0].Status != "revalidation_required" || got.Improvements[0].BasePathwayVersion != 1 {
		t.Fatalf("material impact lost: %#v", got)
	}
	got = post("/revalidations", `{"id":"revalidation-7","improvement_id":"portable-setup","learner_id":"learner-opaque-7","completion_evidence_id":"completion-v1-7","from_pathway_version":1,"to_pathway_version":2,"evidence_references":["check:portable-setup/pass"]}`, 201)
	if len(got.Revalidations) != 1 || got.Improvements[0].AffectedLearners[0].CompletionEvidenceID != "completion-v1-7" || got.GrantsAuthority {
		t.Fatalf("history or boundary changed: %#v", got)
	}
	post("/observations", `{"id":"surveillance","kind":"retention","pathway_version":2,"project_revision":"rev-2","audience":"repository","consent":"missing","count":1,"summary":"Track an individual","evidence_references":["private:event"]}`, 422)
}
