package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestSecurityRepairHTTPScopesAndRevokesEmbargoedBranch(t *testing.T) {
	userStore, _ := users.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "repair-owner", DisplayName: "Repair owner"})
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	repo, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "secure-library", Visibility: repositories.Public})
	opened, _ := catalog.Open(repo.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	commit, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\nauthor Owner <owner@example.test> 0 +0000\ncommitter Owner <owner@example.test> 0 +0000\n\nbase\n"))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	credentials, _ := auth.New(t.TempDir())
	token := issueAccess(t, credentials, string(owner.ID), auth.API, auth.RepositoryRead)
	store, _ := securityreports.New(t.TempDir())
	report, _ := store.Create(securityreports.CreateInput{ActorID: string(owner.ID), Title: "private parser issue", Summary: "details", Contact: securityreports.Contact{Channel: "email", Value: "safe@example.test"}, Affected: []securityreports.AffectedRepository{{RepositoryID: string(repo.ID), Versions: []string{"1.x"}}}})
	_, _ = store.Triage(report.ID, securityreports.TriageInput{ActorID: string(owner.ID), EmbargoState: "active"}, func(string) bool { return true })
	mux := http.NewServeMux()
	registerSecurityReportsHTTP(mux, store, catalog, userStore, credentials)
	request := func(method, path, bearer string, input any) (int, []byte) {
		body, _ := json.Marshal(input)
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+bearer)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code, w.Body.Bytes()
	}
	code, body := request(http.MethodPost, "/security-reports/"+report.ID+"/repairs", token, map[string]any{"repository_id": string(repo.ID), "version": "1.x", "outcome": "remove unsafe parser path", "base_revision": string(commit)})
	if code != 201 {
		t.Fatalf("repair=%d %s", code, body)
	}
	var created struct {
		Repair securityreports.RepairTask `json:"repair"`
	}
	_ = json.Unmarshal(body, &created)
	if !strings.HasPrefix(created.Repair.Branch, "refs/heads/embargo/") {
		t.Fatalf("branch=%q", created.Repair.Branch)
	}
	code, body = request(http.MethodPost, "/security-reports/"+report.ID+"/repairs/"+created.Repair.ID+"/sessions", token, map[string]string{"kind": "agent", "assignee_id": "codex", "mandate": "make the bounded repair"})
	if code != 201 {
		t.Fatalf("session=%d %s", code, body)
	}
	var delegated struct {
		Session       securityreports.RepairSession `json:"session"`
		GitCredential string                        `json:"git_credential"`
	}
	_ = json.Unmarshal(body, &delegated)
	grant, err := credentials.Authenticate(delegated.GitCredential, auth.GitWrite)
	if err != nil || grant.RepositoryID != string(repo.ID) || grant.Branch != created.Repair.Branch {
		t.Fatalf("grant=%#v err=%v", grant, err)
	}
	verificationPath := "/security-reports/" + report.ID + "/repairs/" + created.Repair.ID + "/verification"
	for _, action := range []map[string]any{
		{"action": "begin", "revision": string(commit)},
		{"action": "gate", "revision": string(commit), "kind": "required_check", "name": "supported line", "attempt_id": "check-1", "definition_digest": strings.Repeat("1", 64), "state": "passed"},
		{"action": "gate", "revision": string(commit), "kind": "security_reproduction", "name": "private regression", "attempt_id": "secret-1", "definition_digest": strings.Repeat("2", 64), "state": "passed"},
		{"action": "approve", "revision": string(commit), "decision": "approve", "summary": "Exact candidate reviewed."},
		{"action": "integrate", "revision": string(commit), "integration_entry_id": "queue-1", "integration_commit_id": strings.Repeat("c", 40)},
		{"action": "attest", "revision": string(commit), "release_id": "release-1", "version": "1.x", "artifact_id": "artifact-1", "artifact_sha256": strings.Repeat("d", 64)},
	} {
		code, body = request(http.MethodPost, verificationPath, token, action)
		if code != 200 {
			t.Fatalf("verification action %#v=%d %s", action, code, body)
		}
	}
	if bytes.Contains(body, []byte(`"command":`)) || bytes.Contains(body, []byte(`"logs":`)) || !bytes.Contains(body, []byte(`"state":"attested"`)) {
		t.Fatalf("unsafe or incomplete verification response: %s", body)
	}
	code, _ = request(http.MethodDelete, "/security-reports/"+report.ID+"/repairs/"+created.Repair.ID+"/sessions/"+delegated.Session.ID, token, nil)
	if code != 200 {
		t.Fatalf("revoke=%d", code)
	}
	if _, err = credentials.Authenticate(delegated.GitCredential, auth.GitWrite); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("credential survived revoke: %v", err)
	}
}
