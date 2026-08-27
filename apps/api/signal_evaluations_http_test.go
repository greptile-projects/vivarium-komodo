package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/observabilitygaps"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalevaluations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestSignalEvaluationPublicBoundary(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "signals", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reader")
	writer := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	gaps, _ := observabilitygaps.New(t.TempDir())
	now := time.Now().UTC()
	gap, _ := gaps.Create(string(repo.ID), "owner", observabilitygaps.Input{Origin: observabilitygaps.Origin{Kind: "incident", ResourceID: "incident-1", Revision: "2"}, Question: "why retries?", Behavior: "checkout retries", Audience: []string{"response"}, Decision: "rollback", Services: []string{"checkout"}, Journeys: []string{"purchase"}, Timeliness: observabilitygaps.Timeliness{MaximumDelaySeconds: 60, DecisionWindow: "incident"}, OwnerIDs: []string{"owner"}, SuccessCriteria: []string{"identify provider"}, ChangeReason: "open question"})
	s, _ := signalevaluations.New(t.TempDir())
	mux := http.NewServeMux()
	registerSignalEvaluationsHTTP(mux, s, gaps, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/observability-gaps/" + gap.ID + "/signal-evaluations"
	body := `{"gap_version":1,"title":"retry evidence","signal_ids":["retry"],"signals":[{"id":"retry","contract_id":"contract","contract_version":2,"rollout_id":"rollout","revision":"collector-4","kind":"metric"}],"queries":[{"id":"q1","expression":"retry by provider","window_start":"` + now.Add(-time.Hour).Format(time.RFC3339) + `","window_end":"` + now.Format(time.RFC3339) + `","signal_ids":["retry"],"release_ids":["release-4"],"deployment_ids":["deploy-4"],"code_revisions":["abc"],"dependency_revisions":["provider@2"],"journey_ids":["purchase"],"result_digest":"sha256:r"}],"citations":[{"id":"c1","query_id":"q1","source":"telemetry://retry","revision":"collector-4","digest":"sha256:e","accessible":true}]}`
	var e signalevaluations.Evaluation
	workflowJSON(t, server.URL, http.MethodPost, base, writer, body, 201, &e)
	finding := `{"expected_revision":1,"kind":"supported","statement":"provider failures explain retries","citation_ids":["c1"],"uncertainty":"regional sample","reproduction":"execute q1 at its pinned window","criteria":{"identify provider":"passed"}}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+base+"/"+e.ID+"/findings", bytes.NewBufferString(finding))
	req.Header.Set("Authorization", "Bearer "+reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actor-Kind", "read_only_agent")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("finding status = %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
		t.Fatal(err)
	}
	if e.Findings[0].ActorKind != "read_only_agent" || e.CriteriaStatus["identify provider"] != "passed" {
		t.Fatalf("lost bounded agent finding: %#v", e)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+e.ID+"/lifecycle-decisions", writer, `{"expected_revision":2,"action":"archive","signal_ids":["retry"],"rationale":"question answered","policy_id":"retention","policy_revision":"3","approved_by_id":"privacy-owner","consumers":[{"kind":"alert","id":"retry-alert","revision":"5","owner_id":"response","impact":"must migrate","acknowledged":false}],"historical_meaning":"retry ratio under collector-4","provenance_refs":["contract@2"]}`, 201, &e)
	if e.Lifecycles[0].Applied || len(e.Blockers) == 0 {
		t.Fatalf("hidden consumer or stop proof did not block archive: %#v", e)
	}
}
