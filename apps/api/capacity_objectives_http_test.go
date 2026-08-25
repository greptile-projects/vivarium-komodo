package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacityobjectives"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestCapacityObjectivePublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "capacity", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "collab")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	collab := issueAccess(t, credentials, "collab", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := capacityobjectives.New(t.TempDir())
	mux := http.NewServeMux()
	registerCapacityObjectivesHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/capacity-objectives"
	start := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	end := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
	expires := time.Now().UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	body := `{"subject_kind":"api","subject_id":"public-api","title":"Launch capacity","description":"Serve planned adoption","demand_forecasts":[{"id":"launch","segment":"active teams","demand":1000,"unit":"requests/second","starts_at":"` + start + `","ends_at":"` + end + `","confidence":"estimated"}],"traffic_shapes":[{"name":"launch","pattern":"bursty","peak_multiplier":2,"duration_minutes":30}],"seasonality":[{"name":"weekday","schedule":"Mon-Fri","multiplier":1.2}],"service_levels":[{"id":"slo","kind":"availability","scope":"api","operator":"at_least","value":99.9,"unit":"percent","source":"slo-1"}],"bottleneck_thresholds":[{"id":"pool","kind":"connections","scope":"db","operator":"at_most","value":100,"unit":"connections"}],"dependency_limits":[{"dependency":"database","metric":"connections","maximum":100,"unit":"connections","owner_id":"db-owner"}],"regions":["eu-west-1"],"owner_ids":["owner","collab"],"budget_amount":5000,"budget_currency":"USD","lead_time_days":45,"signals":[{"name":"request rate","required":true,"owner_id":"operator"}],"assumptions":[{"id":"growth","statement":"launch drives adoption","owner_id":"product","expires_at":"` + expires + `"}],"success_criteria":["headroom above 30%"],"rollback_criteria":["errors exceed 1%"],"links":[{"kind":"product_roadmap","resource_id":"roadmap-1"},{"kind":"infrastructure","resource_id":"inventory-1"},{"kind":"funding","resource_id":"fund-1"}],"change_reason":"agree before scaling"}`
	var objective capacityobjectives.Objective
	workflowJSON(t, server.URL, http.MethodPost, base, collab, body, 201, &objective)
	if objective.CurrentVersion != 1 || objective.Versions[0].AuthorID != "collab" || len(objective.Gaps) != 3 {
		t.Fatalf("objective lost attribution or explicit uncertainty: %+v", objective)
	}
	var list struct {
		Items []capacityobjectives.Objective `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, owner, "", 200, &list)
	if len(list.Items) != 1 || len(list.Items[0].NonAuthority) == 0 {
		t.Fatalf("objective unavailable or authoritative: %+v", list)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+objective.ID+"/versions", owner, body[:1]+`"expected_version":0,`+body[1:], http.StatusConflict, nil)
}
