package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryobjectives"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func recoveryBody(owner string, expiry time.Time) string {
	return `{"title":"Contributor platform continuity","description":"Restore collaboration and serving state after destructive loss","owner_ids":["` + owner + `"],"resources":[{"id":"git","kind":"repository","name":"Git history","user_capability":"Contributors can clone and continue work","owner_ids":["` + owner + `"],"dependency_ids":["object-store"],"acceptable_loss":"0s","restoration_time":"2h","retention":"7y","jurisdictions":["EU","US"],"validation_criteria":["all refs resolve","signed commits remain reachable"],"feasibility":"achievable"},{"id":"collaboration","kind":"collaboration_records","resource_id":"repository-events","name":"Issues and reviews","user_capability":"Collaborators retain decisions and review history","owner_ids":[],"dependency_ids":["database"],"acceptable_loss":"5m","restoration_time":"4h","retention":"7y","jurisdictions":["EU"],"validation_criteria":["open work and attribution reconcile"],"feasibility":"impossible","feasibility_rationale":"current archive export exceeds four hours"},{"id":"runtime","kind":"deployed_service_data","resource_id":"production-primary","name":"Production data","user_capability":"Users can resume service","owner_ids":["` + owner + `"],"dependency_ids":["database"],"acceptable_loss":"5m","restoration_time":"1h","retention":"30d","jurisdictions":["EU"],"validation_criteria":["critical user journey passes"],"feasibility":"unverified"}],"dependencies":[{"id":"object-store","name":"Backup object store","kind":"storage","owner_ids":["` + owner + `"],"protected":true,"protection_reference":"replica-policy-v2"},{"id":"database","name":"Primary database","kind":"service","owner_ids":[],"protected":false}],"links":[{"kind":"service_objective","resource_id":"availability-v1","label":"collaboration availability"},{"kind":"environment","resource_id":"production","label":"production"},{"kind":"incident","resource_id":"incident-42","label":"regional loss review"},{"kind":"privacy_rule","resource_id":"retention-v3","label":"retention and residency"},{"kind":"governance","resource_id":"continuity-approvers","label":"recovery approval"}],"declared_exclusions":[{"scope":"developer-local unpushed branches","reason":"not held by the platform","approved_by":"` + owner + `"}],"exceptions":[{"id":"database-replica","scope":"database cross-region protection","reason":"migration underway","owner_id":"` + owner + `","approved_by":"` + owner + `","expires_at":"` + expiry.Format(time.RFC3339) + `"}],"exception_policy":"Exceptions require an owner, approver, rationale, and expiry","change_reason":"publish shared continuity terms"}`
}

func TestRecoveryObjectivePublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "continuity", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := recoveryobjectives.New(t.TempDir())
	mux := http.NewServeMux()
	registerRecoveryObjectivesHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/recovery-objectives"
	var created recoveryobjectives.Objective
	workflowJSON(t, server.URL, http.MethodPost, base, owner, recoveryBody("owner", time.Now().UTC().Add(10*24*time.Hour)), 201, &created)
	if created.Versions[0].AuthorID != "owner" || len(created.Versions[0].Links) != 5 || len(created.Versions[0].Exclusions) != 1 {
		t.Fatalf("missing attribution or governed context: %+v", created)
	}
	var list struct {
		Items []recoveryobjectives.Objective `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, reader, "", 200, &list)
	kinds := map[string]bool{}
	for _, b := range list.Items[0].Blockers {
		kinds[b.Kind] = true
	}
	for _, want := range []string{"missing_ownership", "impossible_target", "unverified_target", "unprotected_dependency", "missing_dependency_owner", "expiring_exception"} {
		if !kinds[want] {
			t.Fatalf("missing %s: %+v", want, list.Items[0].Blockers)
		}
	}
	body := recoveryBody("owner", time.Now().UTC().Add(60*24*time.Hour))
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, body[:1]+`"expected_version":0,`+body[1:], 409, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", reader, body[:1]+`"expected_version":1,`+body[1:], 401, nil)
}
