package main

import (
	"bytes"
	"encoding/json"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/historyremediations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHistoryRemediationHTTPIsRestrictedAndOwnerOpened(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "repair", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repo.ID, "responder")
	credentials, _ := auth.New(t.TempDir())
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	responder := issueAccess(t, credentials, "responder", auth.API, auth.RepositoryRead)
	stranger := issueAccess(t, credentials, "stranger", auth.API, auth.RepositoryRead)
	s, _ := historyremediations.New(t.TempDir())
	mux := http.NewServeMux()
	registerHistoryRemediationsHTTP(mux, s, repos, credentials)
	in := historyremediations.Input{Title: "Restricted repair", Source: historyremediations.Source{Kind: "support_case", ID: "case-1"}, ContentDescription: "Named object contains unsafe material; bytes omitted.", Reason: "Remove the unsafe material from all named distribution surfaces.", Audience: "response_team", ResponseOwnerIDs: []string{"responder"}, Objects: []historyremediations.Object{{ID: "o", RepositoryID: string(repo.ID), Kind: "blob", ObjectID: "deadbeef", Match: "confirmed", AttributedTo: "owner"}}, Scope: []historyremediations.Scope{{Kind: "repository", RepositoryID: string(repo.ID), Reference: string(repo.ID)}}, Evidence: []historyremediations.Evidence{{ID: "e", Kind: "case_attachment_digest", Reference: "attachment-1", Digest: "sha256:123", Summary: "Attachment digest corresponds to the named object; content omitted.", Status: "available", RecordedBy: "owner"}}, Approvals: []historyremediations.Approval{{Kind: "repository_owner", OwnerID: "owner", Required: true, Status: "pending"}}}
	b, _ := json.Marshal(in)
	r := httptest.NewRequest(http.MethodPost, "/repositories/"+string(repo.ID)+"/history-remediations", bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer "+owner)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("create=%d %s", w.Code, w.Body.String())
	}
	var x historyremediations.Remediation
	_ = json.Unmarshal(w.Body.Bytes(), &x)
	reach := historyremediations.ReachabilityInput{CopyKind: "fork", Reference: "fork:responder/repair", RepositoryID: "fork-repository", Revision: "feedface", ObjectIDs: []string{"deadbeef"}, Status: "independently_controlled", ControlledBy: "responder", Summary: "The fork advertises a ref that reaches the affected object ID.", Uncertainty: "The fork owner must verify non-advertised refs.", Citations: []historyremediations.Citation{{Kind: "ref_advertisement", Reference: "snapshot:fork-1", Digest: "sha256:refs", Access: "available"}}}
	b, _ = json.Marshal(reach)
	r = httptest.NewRequest(http.MethodPost, "/repositories/"+string(repo.ID)+"/history-remediations/"+x.ID+"/reachability", bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer "+responder)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 201 || !bytes.Contains(w.Body.Bytes(), []byte(`"independently_controlled"`)) {
		t.Fatalf("reachability=%d %s", w.Code, w.Body.String())
	}
	for _, tc := range []struct {
		token string
		want  int
	}{{responder, 200}, {stranger, 200}} {
		r = httptest.NewRequest(http.MethodGet, "/repositories/"+string(repo.ID)+"/history-remediations", nil)
		r.Header.Set("Authorization", "Bearer "+tc.token)
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("list=%d", w.Code)
		}
		if tc.token == stranger && bytes.Contains(w.Body.Bytes(), []byte(x.ID)) {
			t.Fatalf("restricted remediation leaked: %s", w.Body.String())
		}
	}
}
