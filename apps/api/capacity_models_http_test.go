package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacitymodels"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestCapacityModelPublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "forecast", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "agent")
	reader := issueAccess(t, credentials, "agent", auth.API, auth.RepositoryRead)
	store, _ := capacitymodels.New(t.TempDir())
	mux := http.NewServeMux()
	registerCapacityModelsHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/capacity-models"
	start := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	end := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	future := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	body := `{"objective_id":"objective-1","objective_version":2,"title":"Launch saturation","release_id":"release-7","release_revision":"0123456789abcdef","forecast_window":{"starts_at":"` + end + `","ends_at":"` + future + `"},"method":"linear regression over sanitized request and pool observations","author_kind":"read_only_agent","evidence":[{"id":"usage","kind":"usage","resource_id":"signal-1","revision":"v4","observation_window":{"starts_at":"` + start + `","ends_at":"` + end + `"},"visibility":"repository","summary":"steady growth","sanitized":true,"instrumentation_version":"otel-1"},{"id":"infra","kind":"infrastructure","resource_id":"inventory-2","revision":"sha256:abc","observation_window":{"starts_at":"` + start + `","ends_at":"` + end + `"},"visibility":"inaccessible","summary":"must not project","sanitized":false,"instrumentation_version":"otel-2","anomalous":true,"anomaly_reason":"regional failover"}],"assumptions":[{"id":"growth","statement":"launch doubles traffic","evidence_ids":["usage"],"owner_id":"product","uncertainty":"plus or minus 20 percent"}],"workload_segments":[{"id":"interactive","name":"interactive API","demand":1200,"unit":"requests/second","evidence_ids":["usage"]}],"saturation_points":[{"id":"pool","resource":"database pool","metric":"connections","capacity":100,"unit":"connections","saturates_at":"` + future + `","headroom_percent":8,"evidence_ids":["usage","infra"],"explanation":"requests per connection cross the observed pool limit"}],"scenarios":[{"id":"planned","name":"planned launch","demand_multiplier":2,"probability":"medium","saturation_ids":["pool"],"cost_curve":[{"demand":1200,"cost":4500,"currency":"USD","period":"month"}]}],"uncertainty":"instrumentation transition widens interval","provenance":["notebook:sha256:def"]}`
	var model capacitymodels.Model
	workflowJSON(t, server.URL, http.MethodPost, base, reader, body, 201, &model)
	if model.AuthorID != "agent" || model.ReleaseRevision != "0123456789abcdef" || len(model.Gaps) != 3 || model.Evidence[1].Summary != "" {
		t.Fatalf("model lost provenance or permission gaps: %+v", model)
	}
	challenge := `{"expected_revision":1,"conclusion_id":"pool","body":"Failover should be excluded from the baseline.","evidence_ids":["infra"],"author_kind":"human"}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+model.ID+"/challenges", reader, challenge, 201, &model)
	if model.Revision != 2 || len(model.Challenges) != 1 || len(model.Gaps) != 4 {
		t.Fatalf("challenge did not preserve disagreement: %+v", model)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+model.ID+"/challenges", reader, challenge, http.StatusConflict, nil)
}
