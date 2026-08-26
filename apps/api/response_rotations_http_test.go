package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responserotations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestResponseRotationDutyAndContextTransfer(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "response-duty", Visibility: repositories.Private})
	_, _ = repos.AddCollaborator("owner", repo.ID, "alice")
	_, _ = repos.AddCollaborator("owner", repo.ID, "bob")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	alice := issueAccess(t, credentials, "alice", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	bob := issueAccess(t, credentials, "bob", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := responserotations.New(t.TempDir())
	mux := http.NewServeMux()
	registerResponseRotationsHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	now := time.Now().UTC()
	start := now.Add(-time.Hour).Format(time.RFC3339)
	end := now.Add(time.Hour).Format(time.RFC3339)
	nextStart := now.Add(90 * time.Minute).Format(time.RFC3339)
	nextEnd := now.Add(3 * time.Hour).Format(time.RFC3339)
	body := fmt.Sprintf(`{"name":"API primary","policy_id":"policy-1","policy_version":1,"team_id":"operators","time_zone":"UTC","handoff_minutes":30,"workload_limit":2,"participants":[{"user_id":"alice","time_zone":"America/Los_Angeles","qualifications":["operations","database"],"available":true,"member":true,"access":true,"workload":1},{"user_id":"bob","time_zone":"Europe/London","qualifications":["operations","database"],"available":true,"member":true,"access":true,"workload":0}],"absence_rules":[{"kind":"planned","notice_minutes":1440,"action":"offer to first eligible backup"}],"shifts":[{"id":"current","starts_at":"%s","ends_at":"%s","primary_id":"alice","backup_layers":[["bob"]],"required_qualifications":["operations","database"],"context_revision":"ctx-7","context_references":["incident:12","dashboard:api"]},{"id":"next","starts_at":"%s","ends_at":"%s","primary_id":"bob","backup_layers":[["alice"]],"required_qualifications":["operations"],"context_revision":"ctx-8","context_references":["runbook:api"]}],"owner_ids":["owner"]}`, start, end, nextStart, nextEnd)
	base := "/repositories/" + string(repo.ID) + "/response-rotations"
	var rotation responserotations.Rotation
	workflowJSON(t, server.URL, http.MethodPost, base, owner, body, 201, &rotation)
	if rotation.CurrentShift == nil || rotation.CurrentShift.ResponderID != "alice" || len(rotation.Upcoming) != 1 || len(rotation.Gaps) != 1 {
		t.Fatalf("duty projection missing current/upcoming/gap: %+v", rotation)
	}
	ack := fmt.Sprintf(`{"expected_revision":%d,"shift_id":"current","kind":"acknowledged","detail":"reviewed active incident and dashboard"}`, rotation.Revision)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+rotation.ID+"/events", alice, ack, 201, &rotation)
	proposal := fmt.Sprintf(`{"expected_revision":%d,"shift_id":"current","kind":"delegate","recipient_id":"bob","context_revision":"ctx-7","context_references":["incident:12","dashboard:api"],"rationale":"planned absence"}`, rotation.Revision)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+rotation.ID+"/transfers", alice, proposal, 201, &rotation)
	if rotation.CurrentShift.ResponderID != "alice" {
		t.Fatal("unaccepted transfer changed responsibility")
	}
	tr := rotation.Transfers[0]
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+rotation.ID+"/transfers/"+tr.ID+"/accept", alice, fmt.Sprintf(`{"expected_revision":%d}`, rotation.Revision), http.StatusForbidden, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+rotation.ID+"/transfers/"+tr.ID+"/accept", bob, fmt.Sprintf(`{"expected_revision":%d}`, rotation.Revision), 200, &rotation)
	if rotation.CurrentShift.ResponderID != "bob" || rotation.Transfers[0].AcceptedBy != "bob" || rotation.Transfers[0].ContextRevision != "ctx-7" || len(rotation.NonAuthority) == 0 {
		t.Fatalf("accepted exact-context transfer missing: %+v", rotation)
	}
}
