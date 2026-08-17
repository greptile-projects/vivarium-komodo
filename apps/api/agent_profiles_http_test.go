package main

import (
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprofiles"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const agentProfileBody = `{"handle":"review-helper","display_name":"Review Helper","summary":"Reviews bounded changes","ownership":"Example Operator LLC","supported_tasks":["code review"],"tools":["git","static analysis"],"models":[{"provider":"example","name":"reasoner","version":"2026-08-01","training_cutoff":"2026-01"}],"execution":{"runtime":"isolated container","regions":["EU"],"remote":true,"boundary":"source and prompts leave the platform for the operator runtime","isolation":"ephemeral per session"},"data_use":{"context_used":["permitted repository blobs"],"purposes":["requested review"],"retention":"deleted within 24 hours","training_use":"never used for training","deletion_process":"automatic deletion with support escalation"},"subprocessors":[{"name":"Compute Co","purpose":"inference hosting","location":"EU","data":["prompts","source excerpts"]}],"pricing":{"model":"usage","currency":"USD","amount":0.02,"unit":"tool second","resource_requirements":["network egress"]},"requested_capabilities":["contents:read","discussion:write"],"availability":"weekdays 00:00-23:00 UTC","support":{"contact":"support@example.test","hours":"24x5","response_target":"4 hours"},"change_reason":"initial publication"}`

func TestAgentProfilePublicContract(t *testing.T) {
	creds, _ := auth.New(t.TempDir())
	us, _ := users.New(t.TempDir())
	_, _ = us.Create(users.Profile{Handle: "person", DisplayName: "Person"})
	s, _ := agentprofiles.New(t.TempDir())
	mux := http.NewServeMux()
	registerAgentProfilesHTTP(mux, s, creds, us)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	operator := issueAccess(t, creds, "operator", auth.API, auth.RepositoryWrite)
	var made agentprofiles.Profile
	workflowJSON(t, srv.URL, http.MethodPost, "/agent-profiles", operator, agentProfileBody, 201, &made)
	if made.ID == "" || made.OperatorID != "operator" || made.OperatorClaimsVerified || made.AuthorityGranted || len(made.PlatformVerifiedEvidence) != 2 {
		t.Fatalf("unsafe or incomplete profile: %+v", made)
	}
	var list struct {
		Items []agentprofiles.Profile `json:"items"`
	}
	workflowJSON(t, srv.URL, http.MethodGet, "/agent-profiles", "", "", 200, &list)
	if len(list.Items) != 1 || list.Items[0].Versions[0].DataUse.TrainingUse == "" {
		t.Fatalf("public projection missing terms: %+v", list)
	}
	revision := strings.Replace(agentProfileBody, `"handle":"review-helper",`, `"expected_version":1,`, 1)
	workflowJSON(t, srv.URL, http.MethodPost, "/agent-profiles/"+made.ID+"/versions", operator, revision, 201, &made)
	if made.CurrentVersion != 2 || len(made.Versions) != 2 {
		t.Fatal("history not retained")
	}
	workflowJSON(t, srv.URL, http.MethodPost, "/agent-profiles/"+made.ID+"/versions", operator, revision, 409, nil)
	impersonation := `{"handle":"person",` + agentProfileBody[len(`{"handle":"review-helper",`):]
	workflowJSON(t, srv.URL, http.MethodPost, "/agent-profiles", operator, impersonation, 409, nil)
}
