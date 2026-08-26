package main

import (
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacityrehearsals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCapacityRehearsalPublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "scale", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "agent")
	reader := issueAccess(t, credentials, "agent", auth.API, auth.RepositoryRead)
	store, _ := capacityrehearsals.New(t.TempDir())
	mux := http.NewServeMux()
	registerCapacityRehearsalsHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/capacity-rehearsals"
	body := `{"objective_id":"objective-1","objective_version":2,"model_id":"model-1","model_revision":1,"title":"Launch scaling options","definition_path":".komodo/capacity/launch.json","definition_revision":"abc123","environment_id":"sandbox-1","environment_revision":"env-sha","environment_class":"isolated","coordinated_load_key":"launch-window","limits":{"max_duration_seconds":120,"max_virtual_users":500,"max_requests_per_second":2000,"max_cost":50,"currency":"USD"},"candidates":[{"id":"vertical","name":"larger node","approach":"vertical","release_id":"release-1","release_revision":"r1","infrastructure_plan_id":"infra-1","infrastructure_revision":"i1","schema_id":"schema-1","schema_revision":"s1","dependency_configuration_id":"deps-1","dependency_revision":"d1"},{"id":"cache","name":"bounded cache","approach":"caching","release_id":"release-2","release_revision":"r2","infrastructure_plan_id":"infra-2","infrastructure_revision":"i2","schema_id":"schema-1","schema_revision":"s1","dependency_configuration_id":"deps-2","dependency_revision":"d2"}],"scenarios":[{"id":"peak","name":"launch peak with dependency loss","kind":"load_and_failure","workload_source":"synthetic","demand":1500,"demand_unit":"requests/second","duration_seconds":60,"failure":"dependency latency 500ms","correctness_criteria":["no lost writes"]}],"owner_ids":["platform"]}`
	var r capacityrehearsals.Rehearsal
	workflowJSON(t, server.URL, http.MethodPost, base, reader, body, 201, &r)
	if len(r.Gaps) != 2 || r.EnvironmentClass != "isolated" {
		t.Fatalf("missing initial proof gaps: %+v", r)
	}
	now := time.Now().UTC()
	start := now.Add(-time.Minute).Format(time.RFC3339)
	end := now.Format(time.RFC3339)
	noisy := `{"expected_revision":1,"candidate_id":"vertical","scenario_id":"peak","actor_kind":"agent","started_at":"` + start + `","ended_at":"` + end + `","environment_revision":"env-sha","workload_digest":"sha256:work","repetitions":1,"noise_percent":18,"status":"completed","metrics":{"throughput":1400,"throughput_unit":"requests/second","latency_p95_ms":180,"error_rate":0.001,"saturation":82,"saturation_unit":"percent","recovery_seconds":4,"correctness_passed":true,"resources":{"cpu":82},"cost":12,"currency":"USD"},"logs":["sanitized:run"],"artifact_digests":["sha256:artifact"]}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+r.ID+"/attempts", reader, noisy, 201, &r)
	if r.Attempts[0].Proof || r.Attempts[0].Classification != "noisy" {
		t.Fatalf("noisy run presented as proof: %+v", r.Attempts[0])
	}
	valid := `{"expected_revision":2,"candidate_id":"cache","scenario_id":"peak","actor_kind":"human","started_at":"` + start + `","ended_at":"` + end + `","environment_revision":"env-sha","workload_digest":"sha256:work","repetitions":3,"noise_percent":3,"status":"completed","metrics":{"throughput":1550,"throughput_unit":"requests/second","latency_p50_ms":40,"latency_p95_ms":90,"latency_p99_ms":130,"error_rate":0.0001,"saturation":64,"saturation_unit":"percent","recovery_seconds":2,"correctness_passed":true,"resources":{"cpu":64,"memory":70},"carbon_grams":22,"cost":9,"currency":"USD"},"logs":["sanitized:run"],"artifact_digests":["sha256:artifact2"]}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+r.ID+"/attempts", reader, valid, 201, &r)
	if !r.Attempts[1].Proof || r.Attempts[1].Classification != "valid" || len(r.Gaps) != 1 {
		t.Fatalf("valid evidence not derived: %+v", r)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+r.ID+"/attempts", reader, valid, http.StatusConflict, nil)
}
