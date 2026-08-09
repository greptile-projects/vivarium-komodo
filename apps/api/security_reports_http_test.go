package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityreports"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

func TestPrivateSecurityReportWorkflowAndIsolation(t *testing.T) {
	userStore, _ := users.New(t.TempDir())
	reporter, _ := userStore.Create(users.Profile{Handle: "researcher", DisplayName: "Researcher"})
	maintainer, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	responder, _ := userStore.Create(users.Profile{Handle: "responder", DisplayName: "Responder"})
	stranger, _ := userStore.Create(users.Profile{Handle: "stranger", DisplayName: "Stranger"})
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	repo, _ := catalog.Create(string(maintainer.ID), repositories.Metadata{Name: "library", Visibility: repositories.Public})
	credentials, _ := auth.New(t.TempDir())
	reporterToken := issueAccess(t, credentials, string(reporter.ID), auth.API, auth.RepositoryRead)
	maintainerToken := issueAccess(t, credentials, string(maintainer.ID), auth.API, auth.RepositoryRead)
	responderToken := issueAccess(t, credentials, string(responder.ID), auth.API, auth.RepositoryRead)
	strangerToken := issueAccess(t, credentials, string(stranger.ID), auth.API, auth.RepositoryRead)
	store, _ := securityreports.New(t.TempDir())
	mux := http.NewServeMux()
	registerSecurityReportsHTTP(mux, store, catalog, userStore, credentials)
	requestJSON := func(method, path, token string, body any) (int, []byte) {
		var input []byte
		if body != nil {
			input, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(input))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code, w.Body.Bytes()
	}
	create := map[string]any{"title": "Parser memory corruption", "summary": "A crafted package may overwrite adjacent memory.", "contact": map[string]string{"channel": "email", "value": "security@example.test"}, "affected_repositories": []map[string]any{{"repository_id": string(repo.ID), "versions": []string{"1.4.x", "2.0.0-beta"}}}, "evidence": []map[string]string{{"title": "Minimal reproducer", "kind": "reproduction", "description": "Run the attached byte sequence against parser entrypoint."}}}
	code, body := requestJSON(http.MethodPost, "/security-reports", reporterToken, create)
	if code != 201 {
		t.Fatalf("create=%d %s", code, body)
	}
	var report securityreports.Report
	_ = json.Unmarshal(body, &report)
	if report.Severity != "unknown" || report.EmbargoState != "requested" || report.Contact.Value == "" {
		t.Fatalf("report=%#v", report)
	}
	if code, _ = requestJSON(http.MethodGet, "/security-reports/"+report.ID, strangerToken, nil); code != 404 {
		t.Fatalf("stranger read=%d", code)
	}
	code, body = requestJSON(http.MethodGet, "/security-reports", maintainerToken, nil)
	if code != 200 || bytes.Contains(body, []byte("security@example.test")) || bytes.Contains(body, []byte("crafted package")) {
		t.Fatalf("collection leaked detail=%d %s", code, body)
	}
	code, body = requestJSON(http.MethodPatch, "/security-reports/"+report.ID+"/triage", maintainerToken, map[string]string{"severity": "critical", "embargo_state": "active"})
	if code != 200 {
		t.Fatalf("triage=%d %s", code, body)
	}
	code, body = requestJSON(http.MethodPost, "/security-reports/"+report.ID+"/team", maintainerToken, map[string]string{"user_id": string(responder.ID)})
	if code != 201 {
		t.Fatalf("invite=%d %s", code, body)
	}
	code, body = requestJSON(http.MethodPost, "/security-reports/"+report.ID+"/messages", reporterToken, map[string]string{"body": "I can join a private call if the reproducer is unclear."})
	if code != 201 {
		t.Fatalf("reporter message=%d %s", code, body)
	}
	code, body = requestJSON(http.MethodGet, "/security-reports/"+report.ID, responderToken, nil)
	if code != 200 {
		t.Fatalf("responder read=%d %s", code, body)
	}
	_ = json.Unmarshal(body, &report)
	if len(report.Messages) != 1 || len(report.Audit) < 5 || report.Audit[len(report.Audit)-1].Type != "access.viewed" {
		t.Fatalf("audit=%#v messages=%#v", report.Audit, report.Messages)
	}
	code, body = requestJSON(http.MethodPost, "/security-reports/"+report.ID+"/resources", responderToken, map[string]any{"kind": "commit", "repository_id": string(repo.ID), "revision": "deadbeef", "label": "Parser bounds change", "details": "Candidate introduction point"})
	if code != 201 {
		t.Fatalf("link=%d %s", code, body)
	}
	_ = json.Unmarshal(body, &report)
	linkID := report.ResourceLinks[0].ID
	code, body = requestJSON(http.MethodPost, "/security-reports/"+report.ID+"/findings", responderToken, map[string]any{"type": "hypothesis", "body": "The bounds regression begins at this change.", "evidence_ids": []string{linkID}})
	if code != 201 {
		t.Fatalf("hypothesis=%d %s", code, body)
	}
	code, body = requestJSON(http.MethodPut, "/security-reports/"+report.ID+"/impact", responderToken, map[string]any{"repository_id": string(repo.ID), "version": "1.4.x", "environment": "production", "state": "confirmed", "rationale": "Production artifacts contain the linked change.", "evidence_ids": []string{linkID}})
	if code != 200 {
		t.Fatalf("impact=%d %s", code, body)
	}
	code, body = requestJSON(http.MethodPost, "/security-reports/"+report.ID+"/investigations", responderToken, map[string]any{"agent": "codex", "mandate": "Determine affected shipped lines without proposing a repair.", "evidence_ids": []string{linkID}})
	if code != 201 {
		t.Fatalf("delegate=%d %s", code, body)
	}
	var delegated struct {
		Report           securityreports.Report `json:"report"`
		WorkerCredential string                 `json:"worker_credential"`
	}
	_ = json.Unmarshal(body, &delegated)
	if delegated.WorkerCredential == "" || delegated.Report.Investigations[0].CredentialDigest != "" {
		t.Fatalf("delegation leaked credential: %s", body)
	}
	code, body = requestJSON(http.MethodGet, "/security-investigations/context", delegated.WorkerCredential, nil)
	if code != 200 || bytes.Contains(body, []byte("security@example.test")) || !bytes.Contains(body, []byte(linkID)) {
		t.Fatalf("worker context=%d %s", code, body)
	}
	code, body = requestJSON(http.MethodPost, "/security-investigations/records", delegated.WorkerCredential, map[string]any{"type": "finding", "body": "The supported 1.4 line contains the change.", "uncertainty": "Build provenance for staging remains unverified.", "evidence_ids": []string{linkID}})
	if code != 201 {
		t.Fatalf("agent finding=%d %s", code, body)
	}
	code, body = requestJSON(http.MethodPost, "/security-reports/"+report.ID+"/investigations/"+delegated.Report.Investigations[0].ID+"/control", responderToken, map[string]string{"action": "cancel", "message": "Scope answered"})
	if code != 200 {
		t.Fatalf("cancel=%d %s", code, body)
	}
	code, _ = requestJSON(http.MethodGet, "/security-investigations/context", delegated.WorkerCredential, nil)
	if code != 401 {
		t.Fatalf("revoked worker=%d", code)
	}
	code, _ = requestJSON(http.MethodDelete, "/security-reports/"+report.ID+"/team/"+string(responder.ID), maintainerToken, nil)
	if code != 200 {
		t.Fatalf("remove=%d", code)
	}
	code, _ = requestJSON(http.MethodGet, "/security-reports/"+report.ID, responderToken, nil)
	if code != 404 {
		t.Fatalf("revoked read=%d", code)
	}
}

func TestSecurityReportRejectsInaccessibleAffectedRepository(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "private", Visibility: repositories.Private})
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	token := issueAccess(t, credentials, "outsider", auth.API, auth.RepositoryRead)
	store, _ := securityreports.New(t.TempDir())
	mux := http.NewServeMux()
	registerSecurityReportsHTTP(mux, store, catalog, userStore, credentials)
	body, _ := json.Marshal(map[string]any{"title": "Issue", "summary": "Details", "contact": map[string]string{"channel": "email", "value": "a@b.test"}, "affected_repositories": []map[string]any{{"repository_id": string(repo.ID), "versions": []string{"1"}}}, "evidence": []map[string]string{{"title": "Proof", "kind": "description", "description": "Details"}}})
	r := httptest.NewRequest(http.MethodPost, "/security-reports", bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 422 {
		t.Fatalf("create=%d %s", w.Code, w.Body.String())
	}
}
