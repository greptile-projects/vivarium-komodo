package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/extensions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

// TestExtensionInstallToGovernedCollaborationWorkflow is the black-box boundary
// for developer registration, repository-owner consent, signed pull-request
// delivery, revision-bound extension evidence and action invocation, ordinary
// review/check policy, replay, renewed capability consent, and revocation.
func TestExtensionInstallToGovernedCollaborationWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the extension workflow")
	}

	var callbackMu sync.Mutex
	callbackCalls := 0
	callbackBodies := [][]byte{}
	callbackHeaders := []http.Header{}
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		callbackMu.Lock()
		callbackCalls++
		call := callbackCalls
		callbackBodies = append(callbackBodies, body)
		callbackHeaders = append(callbackHeaders, r.Header.Clone())
		callbackMu.Unlock()
		if call == 1 {
			http.Error(w, "retry", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer endpoint.Close()
	previousTransport := http.DefaultTransport
	http.DefaultTransport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // test-only endpoint
	defer func() { http.DefaultTransport = previousTransport }()

	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	people, _ := users.New(t.TempDir())
	activity, _ := activities.New(t.TempDir(), people)
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	extensionStore, _ := extensions.New(t.TempDir())
	orgs, _ := organizations.New(t.TempDir())
	runner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, catalog, credentials, activity, runner, checks)
	registerCheckRunsHTTP(mux, checks, runner, pulls, catalog, credentials, nil, activity)
	registerExtensionsHTTP(mux, extensionStore, catalog, orgs, credentials, activity, pulls)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	developer := issueAccess(t, credentials, "developer", auth.API, auth.ProfileRead, auth.ProfileWrite, auth.RepositoryRead, auth.RepositoryWrite)
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "owner", auth.Git, auth.GitRead, auth.GitWrite)
	contributor := issueAccess(t, credentials, "contributor", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	contributorGit := issueAccess(t, credentials, "contributor", auth.Git, auth.GitRead, auth.GitWrite)

	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"extension-loop","visibility":"private"}`, http.StatusCreated, &repository)
	if _, err := catalog.AddCollaborator("owner", repository.ID, "contributor"); err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+string(repository.ID)+"/required-checks", owner, `{"branch":"main","checks":["project"]}`, http.StatusOK, nil)

	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		value.User = url.UserPassword("git", token)
		return value.String()
	}
	baseClone := gitClone(t, remote(ownerGit))
	gitOutput(t, baseClone, "config", "user.name", "Owner")
	gitOutput(t, baseClone, "config", "user.email", "owner@example.com")
	writeWorkflowFile(t, baseClone, "README.md", "# Extension collaboration\n")
	writeWorkflowFile(t, baseClone, ".komodo/checks.json", `{"version":1,"checks":[{"name":"project","command":"test -f repaired.txt"}]}`)
	writeWorkflowFile(t, baseClone, "repaired.txt", "baseline\n")
	gitOutput(t, baseClone, "add", ".")
	gitOutput(t, baseClone, "commit", "-m", "Initialize extension example")
	gitOutput(t, baseClone, "push", "-u", "origin", "main")

	registration := `{"name":"Sample repair assistant","description":"Annotates candidate repairs","operator_contact":"extensions@example.com","capabilities":["annotate candidate","repair action"],"callback_url":"` + endpoint.URL + `","action_url":"` + endpoint.URL + `","requested_permissions":["metadata:read","pull_requests:read","checks:write"],"event_types":["pull_request.created"],"rotation_policy":{"interval_days":30,"overlap_hours":24,"contact_on_failure":true}}`
	var extension extensions.Extension
	workflowJSON(t, server.URL, http.MethodPost, "/extensions", developer, registration, http.StatusCreated, &extension)
	callbackToken, actionToken := extension.Callback.VerificationToken, extension.Actions.VerificationToken
	workflowJSON(t, server.URL, http.MethodPost, "/extensions/"+extension.ID+"/endpoint-verifications", developer, `{"endpoint":"callback","token":"`+callbackToken+`"}`, http.StatusOK, &extension)
	workflowJSON(t, server.URL, http.MethodPost, "/extensions/"+extension.ID+"/endpoint-verifications", developer, `{"endpoint":"actions","token":"`+actionToken+`"}`, http.StatusOK, &extension)

	repoBase := "/repositories/" + string(repository.ID)
	grant := `{"extension_id":"` + extension.ID + `","permissions":["metadata:read","pull_requests:read","checks:write"],"event_types":["pull_request.created"],"resource_types":["pull_requests"],"capability_decisions":[{"capability":"annotate candidate","decision":"approved"},{"capability":"repair action","decision":"denied"}],"settings":{"mode":"strict"}}`
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/extension-authority-previews", owner, `{"extension_id":"`+extension.ID+`","permissions":["metadata:read","pull_requests:read","checks:write"],"event_types":["pull_request.created"]}`, http.StatusOK, nil)
	var installation extensions.Installation
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/extension-installations", owner, grant, http.StatusCreated, &installation)
	if installation.RepositoryID != string(repository.ID) || installation.Authority.CanImpersonate || installation.Authority.CredentialIssued {
		t.Fatalf("installation escaped repository-only authority: %#v", installation.Authority)
	}
	var issued struct {
		Token         string `json:"token"`
		SigningSecret string `json:"signing_secret"`
	}
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/extension-installations/"+installation.ID+"/credentials", owner, `{}`, http.StatusCreated, &issued)

	work := gitClone(t, remote(contributorGit))
	gitOutput(t, work, "config", "user.name", "Contributor")
	gitOutput(t, work, "config", "user.email", "contributor@example.com")
	gitOutput(t, work, "switch", "-c", "repair/sample")
	writeWorkflowFile(t, work, "repaired.txt", "candidate repair\n")
	gitOutput(t, work, "commit", "-am", "Propose extension-assisted repair")
	revision := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "repair/sample")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/pull-requests", contributor, `{"title":"Extension-assisted repair","source_branch":"repair/sample","target_branch":"main"}`, http.StatusCreated, &pull)
	pullBase := repoBase + "/pull-requests/" + pull.ID
	waitForWorkflowCheck(t, server.URL, pullBase, owner, revision, checkruns.Succeeded)

	var deliveryList struct {
		Items []extensions.Delivery `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, repoBase+"/extension-installations/"+installation.ID+"/deliveries", contributor, "", http.StatusOK, &deliveryList)
	if len(deliveryList.Items) != 1 || deliveryList.Items[0].EventType != "pull_request.created" {
		t.Fatalf("pull request event was not scoped into one delivery: %#v", deliveryList.Items)
	}
	deliveryPath := repoBase + "/extension-installations/" + installation.ID + "/deliveries/" + deliveryList.Items[0].ID + "/attempts"
	workflowJSON(t, server.URL, http.MethodPost, deliveryPath, owner, `{"replay":false}`, http.StatusOK, nil)
	var replayed extensions.Delivery
	workflowJSON(t, server.URL, http.MethodPost, deliveryPath, owner, `{"replay":true}`, http.StatusOK, &replayed)
	callbackMu.Lock()
	gotBody := callbackBodies[1]
	gotHeaders := callbackHeaders[1]
	callbackMu.Unlock()
	timestamp := gotHeaders.Get("X-Komodo-Timestamp")
	signature := gotHeaders.Get("X-Komodo-Signature-256")
	mac := hmac.New(sha256.New, []byte(issued.SigningSecret))
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(gotBody)
	if signature != "v1="+hex.EncodeToString(mac.Sum(nil)) || replayed.Status != "delivered" || len(replayed.Attempts) != 2 {
		t.Fatalf("signed replay contract failed: status=%s attempts=%d headers=%v", replayed.Status, len(replayed.Attempts), gotHeaders)
	}

	resource := `{"type":"pull_request","id":"` + pull.ID + `","revision":"` + revision + `"}`
	checkBody := `{"idempotency_key":"sample-check","resource":` + resource + `,"kind":"check","state":"passed","title":"Sample extension analysis passed","annotations":[{"path":"repaired.txt","start_line":1,"end_line":1,"message":"Repair is bounded","level":"notice"}],"artifacts":[{"name":"analysis.json","media_type":"application/json","url":"https://example.com/artifacts/analysis.json","digest":"sha256:abc","size":42}]}`
	var contribution extensions.Contribution
	extensionJSON(t, server.URL, repoBase+"/extension-contributions", issued.Token, checkBody, http.StatusCreated, &contribution)
	if contribution.Resource.Revision != revision || len(contribution.Annotations) != 1 || len(contribution.Artifacts) != 1 || contribution.PolicyEffect != "advisory_only" {
		t.Fatalf("extension evidence lost revision or governance: %#v", contribution)
	}

	upgrade := `{"action":"upgrade","reason":"Owner consents to repair actions","expected_version":2,"grant":{"permissions":["metadata:read","pull_requests:read","checks:write"],"event_types":["pull_request.created"],"resource_types":["pull_requests"],"capability_decisions":[{"capability":"annotate candidate","decision":"approved"},{"capability":"repair action","decision":"approved"}],"settings":{"mode":"strict"}}}`
	workflowJSON(t, server.URL, http.MethodPatch, repoBase+"/extension-installations/"+installation.ID, contributor, upgrade, http.StatusForbidden, nil)
	workflowJSON(t, server.URL, http.MethodPatch, repoBase+"/extension-installations/"+installation.ID, owner, upgrade, http.StatusOK, &installation)
	actionBody := `{"idempotency_key":"repair-action","resource":` + resource + `,"name":"repair","label":"Apply suggested repair","description":"Request a bounded repair","inputs":[{"name":"path","label":"Path","type":"text","required":true}],"effects":[{"kind":"extension_request","description":"Ask the extension to repair the selected path"}]}`
	var action extensions.Action
	extensionJSON(t, server.URL, repoBase+"/extension-actions", issued.Token, actionBody, http.StatusCreated, &action)
	var invocation extensions.Invocation
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/extension-installations/"+installation.ID+"/actions/"+action.ID+"/invocations", contributor, `{"inputs":{"path":"repaired.txt"}}`, http.StatusAccepted, &invocation)
	if invocation.ActorID != "contributor" || invocation.Resource.Revision != revision {
		t.Fatalf("repair invocation lost collaborator or revision attribution: %#v", invocation)
	}

	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	var merged pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", owner, `{}`, http.StatusOK, &merged)
	if merged.Status != pullrequests.Merged {
		t.Fatalf("ordinary policy did not merge updated change: %#v", merged)
	}

	workflowJSON(t, server.URL, http.MethodDelete, repoBase+"/extension-installations/"+installation.ID, owner, "", http.StatusOK, &installation)
	extensionJSON(t, server.URL, repoBase+"/extension-contributions", issued.Token, strings.Replace(checkBody, "sample-check", "after-uninstall", 1), http.StatusForbidden, nil)
	var retained struct {
		Items []extensions.Installation `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, repoBase+"/extension-installations", contributor, "", http.StatusOK, &retained)
	if len(retained.Items) != 1 || retained.Items[0].Status != "removed" || len(retained.Items[0].Authority.Permissions) != 0 || len(retained.Items[0].Contributions) != 1 || len(retained.Items[0].Actions) != 1 || len(retained.Items[0].Deliveries) != 1 || retained.Items[0].Events[len(retained.Items[0].Events)-1].Type != "remove" {
		t.Fatalf("uninstall erased evidence or retained authority: %#v", retained.Items)
	}
}

func extensionJSON(t *testing.T, origin, path, token, body string, want int, output any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, origin+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != want {
		contents, _ := io.ReadAll(response.Body)
		t.Fatalf("POST %s = %d, want %d: %s", path, response.StatusCode, want, contents)
	}
	if output != nil && json.NewDecoder(response.Body).Decode(output) != nil {
		t.Fatalf("decode POST %s", path)
	}
}
