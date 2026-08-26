package main

import (
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalcontracts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignalContractReviewBoundary(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "signals", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reviewer")
	writer := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reviewer", auth.API, auth.RepositoryRead)
	store, _ := signalcontracts.New(t.TempDir())
	mux := http.NewServeMux()
	registerSignalContractsHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	body := `{"name":"checkout.retry","signal_type":"metric","purpose":"decide whether to roll back checkout","schema":[{"name":"retry_count","type":"integer","description":"number of retries","unit":"requests"},{"name":"user_email","type":"string","description":"customer identifier","sensitive":true}],"unit":"requests","dimensions":[{"name":"release","source":"deployment.release","bounded":true,"maximum_values":20},{"name":"customer_id","source":"request.customer","bounded":false,"sensitive":true}],"sampling":{"strategy":"probabilistic","rate":0.1,"rationale":"control hot-path overhead"},"aggregation":{"method":"sum","window_seconds":60,"temporality":"delta"},"correlation":[{"field":"trace_id","target":"checkout traces","propagation":"w3c"}],"retention_days":30,"expected_volume":{"events_per_second":100,"bytes_per_event":200,"peak_multiplier":3},"quality_thresholds":{"maximum_delay_seconds":60,"minimum_completeness":0.99,"maximum_error_rate":0.01},"owner_ids":["telemetry"],"consumers":["on-call","release automation"],"source_symbols":[{"path":"checkout/retry.go","symbol":"recordRetry","revision":"abc123","accessible":true}],"service_boundaries":[{"service":"checkout-api","boundary":"request handler","revision":"svc-7","owner_id":"checkout"}],"collector":{"kind":"otel_collector","id":"prod-collector","revision":"v2","supported":false},"dependencies":[{"kind":"schema","id":"checkout-events","revision":"v4","supported":false}],"impact":{"privacy":"email may identify a customer","security":"restricted telemetry role","residency":"EU stays in EU","performance":"under 1ms at p99","cardinality":"release capped at 20","storage":"51.8GB raw monthly","cost":"estimated ingest and retention","retention_days":30,"residency_regions":["eu-west"],"monthly_storage_bytes":51840000000,"monthly_cost":125.5,"currency":"USD"},"alternatives":[{"id":"aggregate","name":"service aggregate","description":"omit customer dimension","sampling":{"strategy":"all","rate":1,"rationale":"low aggregate volume"},"aggregation":{"method":"sum","window_seconds":300,"temporality":"delta"},"retention_days":14,"expected_volume":{"events_per_second":2,"bytes_per_event":80,"peak_multiplier":2},"impact":{"monthly_storage_bytes":193536000,"monthly_cost":9,"currency":"USD"},"evidence":[{"uri":"doc:load-test","revision":"7","claim":"aggregate stays below 2 eps","accessible":true}]}],"assumptions":["retry volume follows requests"],"change_reason":"review before instrumentation"}`
	var c signalcontracts.Contract
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/signal-contracts", writer, body, 201, &c)
	if c.CurrentVersion != 1 || !c.Blocked || c.Complete || len(c.Comparison) != 2 {
		t.Fatalf("review state missing: %+v", c)
	}
	challenge := `{"version":1,"agent":true,"assumption":"retry volume follows requests","position":"load test shows retry amplification","evidence":[{"uri":"artifact:load-9","revision":"sha256:123","claim":"retries amplify fivefold","accessible":true}]}`
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/signal-contracts/"+c.ID+"/challenges", reader, challenge, 201, &c)
	if len(c.Challenges) != 1 || c.Challenges[0].AuthorID != "reviewer" || !c.Challenges[0].Agent {
		t.Fatalf("reader challenge missing: %+v", c.Challenges)
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/signal-contracts/"+c.ID+"/versions", writer, body[:1]+`"expected_version":0,`+body[1:], 409, nil)
}
