package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/infrastructurestate"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func infrastructureBody(owner string) string {
	return `{"name":"Project runtime","description":"Declared infrastructure supporting collaboration","source_revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","definition_path":"infra/project.yaml","format":"terraform","owner_ids":["` + owner + `"],"environments":[{"id":"production","name":"Production","tier":"production","regions":["eu-west-1"],"owner_ids":["` + owner + `"],"release_id":"release-7"}],"resources":[{"id":"api","kind":"service","name":"Public API","provider":"cloud","provider_resource":"service/api","owner_ids":["` + owner + `"],"depends_on":["db"],"environments":["production"],"configuration":[{"name":"database credential","source":"secret manager reference","secret_backed":true,"classification":"restricted"}],"constraints":[{"kind":"cost","commitment":"under 2000 USD monthly"},{"kind":"capacity","commitment":"1000 requests per second"},{"kind":"security","commitment":"private network ingress"},{"kind":"privacy","commitment":"personal data remains in EU","reference":"privacy-v3"},{"kind":"reliability","commitment":"99.9% availability","reference":"slo-v2"},{"kind":"continuity","commitment":"restore within two hours","reference":"recovery-v1"},{"kind":"regional","commitment":"EU serving"}]},{"id":"db","kind":"data_store","name":"Primary database","provider":"cloud","provider_resource":"database/shared","owner_ids":[],"depends_on":[],"environments":["production"],"configuration":[],"constraints":[]},{"id":"duplicate-db","kind":"data_store","name":"Conflicting database claim","provider":"cloud","provider_resource":"database/shared","owner_ids":["` + owner + `"],"depends_on":[],"environments":["production"],"configuration":[],"constraints":[]}],"change_reason":"publish operational intent"}`
}

func TestInfrastructureStatePublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "infra", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := infrastructurestate.New(t.TempDir())
	mux := http.NewServeMux()
	registerInfrastructureStateHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/infrastructure-definitions"
	var created infrastructurestate.Definition
	workflowJSON(t, server.URL, http.MethodPost, base, owner, infrastructureBody("owner"), 201, &created)
	if created.Versions[0].AuthorID != "owner" || len(created.NonAuthority) != 2 {
		t.Fatalf("missing provenance/non-authority: %+v", created)
	}
	kinds := map[string]bool{}
	for _, g := range created.Gaps {
		kinds[g.Kind] = true
	}
	for _, want := range []string{"missing_ownership", "conflicting_ownership", "secret_backed_value", "missing_observation"} {
		if !kinds[want] {
			t.Fatalf("missing %s: %+v", want, created.Gaps)
		}
	}
	now := time.Now().UTC()
	observation := `{"definition_version":1,"source_revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","environment_id":"production","release_id":"release-7","provider":"cloud","provider_accessible":false,"evidence_reference":"attestation:sha256:abc","observed_at":"` + now.Add(-2*time.Hour).Format(time.RFC3339) + `","valid_until":"` + now.Add(-time.Hour).Format(time.RFC3339) + `","resources":[{"resource_id":"api","provider_resource":"service/api","kind":"service","status":"running","region":"eu-west-1","capacity":"redacted provider class","configuration_state":"redacted"},{"provider_resource":"network/orphan","kind":"network","status":"running","configuration_state":"unknown"}],"summary":"sanitized provider inventory"}`
	var observed infrastructurestate.Definition
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/observations", owner, observation, 201, &observed)
	kinds = map[string]bool{}
	for _, g := range observed.Gaps {
		kinds[g.Kind] = true
	}
	for _, want := range []string{"stale_observation", "inaccessible_provider", "unmanaged_resource"} {
		if !kinds[want] {
			t.Fatalf("missing %s: %+v", want, observed.Gaps)
		}
	}
	raw := infrastructureBody("owner")
	if strings.Contains(raw, "secret-value") || strings.Contains(observation, "secret-value") {
		t.Fatal("fixture exposed secret")
	}
	var list struct {
		Items []infrastructurestate.Definition `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, reader, "", 200, &list)
	if len(list.Items) != 1 || len(list.Items[0].Observations) != 1 {
		t.Fatalf("reader cannot inspect inventory: %+v", list)
	}
	revision := infrastructureBody("owner")
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, revision[:1]+`"expected_version":0,`+revision[1:], 409, nil)
	workflowJSON(t, server.URL, http.MethodPost, base, reader, infrastructureBody("reader"), 401, nil)
	leaking := strings.Replace(observation, "sanitized provider inventory", "token=do-not-retain", 1)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/observations", owner, leaking, 422, nil)
}
