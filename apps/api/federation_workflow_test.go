package main

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributionopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributorpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/federatedagents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/federation"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

type federationWorkflowInstance struct {
	server       *httptest.Server
	federation   *federation.Store
	repositories *repositories.Store
	pulls        *pullrequests.Store
	credentials  *auth.Store
	agents       *federatedagents.Store
}

// TestFederationCompleteHumanAgentWorkflow is the black-box boundary for
// milestone 28. It deliberately uses two TLS HTTP applications, independently
// persisted stores, and stock Git. Remote identities remain subjects, imported
// source is immutable, and accepted history survives loss of current trust.
func TestFederationCompleteHumanAgentWorkflow(t *testing.T) {
	requireGit(t)
	previousTransport := http.DefaultTransport
	http.DefaultTransport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // test-only peer certificates
	t.Cleanup(func() { http.DefaultTransport = previousTransport })

	upstream := newFederationWorkflowInstance(t, "upstream")
	home := newFederationWorkflowInstance(t, "home")
	defer upstream.server.Close()
	defer home.server.Close()

	maintainer := issueAccess(t, upstream.credentials, "maintainer", auth.API, auth.ProfileRead, auth.ProfileWrite, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, upstream.credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	developer := issueAccess(t, home.credentials, "developer", auth.API, auth.ProfileRead, auth.ProfileWrite, auth.RepositoryRead, auth.RepositoryWrite)
	developerGit := issueAccess(t, home.credentials, "developer", auth.Git, auth.GitRead, auth.GitWrite)

	workflowJSON(t, upstream.server.URL, http.MethodPost, "/federation/actors", maintainer, `{"kind":"user","id":"maintainer","display_name":"Remote Maintainer"}`, http.StatusCreated, nil)
	workflowJSON(t, home.server.URL, http.MethodPost, "/federation/actors", developer, `{"kind":"user","id":"developer","display_name":"Developer"}`, http.StatusCreated, nil)
	workflowJSON(t, home.server.URL, http.MethodPost, "/federation/actors", developer, `{"kind":"agent","id":"codex","display_name":"Locally approved Codex"}`, http.StatusCreated, nil)
	observeAndTrust(t, home, upstream)
	observeAndTrust(t, upstream, home)

	var project struct {
		ID string `json:"id"`
	}
	workflowJSON(t, upstream.server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"federated-project","visibility":"public"}`, http.StatusCreated, &project)
	clone := gitClone(t, authenticatedGitURL(upstream.server.URL, project.ID, maintainerGit))
	gitOutput(t, clone, "config", "user.name", "Remote Maintainer")
	gitOutput(t, clone, "config", "user.email", "maintainer@example.test")
	if err := os.WriteFile(filepath.Join(clone, "README.md"), []byte("# Federated project\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, clone, "add", "README.md")
	gitOutput(t, clone, "commit", "-m", "Initialize upstream")
	targetTip := gitOutput(t, clone, "rev-parse", "HEAD")
	gitOutput(t, clone, "push", "-u", "origin", "main")

	ref := "repository:" + project.ID + "@" + upstream.server.URL
	var followed federation.RemoteRepository
	workflowJSON(t, home.server.URL, http.MethodPost, "/federation/repositories/resolutions", developer, `{"reference":"`+ref+`","follow":true}`, http.StatusOK, &followed)
	if !followed.Followed || followed.Status != "current" {
		t.Fatalf("followed observation = %#v", followed)
	}

	var fork struct {
		ID string `json:"id"`
	}
	workflowJSON(t, home.server.URL, http.MethodPost, "/federation/repositories/forks", developer, `{"reference":"`+ref+`","branch":"main","name":"federated-project-fork"}`, http.StatusCreated, &fork)
	workflowJSON(t, home.server.URL, http.MethodPost, "/federation/repositories/"+fork.ID+"/sync", developer, `{"branch":"main"}`, http.StatusOK, nil)
	local := gitClone(t, authenticatedGitURL(home.server.URL, fork.ID, developerGit))
	gitOutput(t, local, "config", "user.name", "Home Developer")
	gitOutput(t, local, "config", "user.email", "developer@example.test")
	gitOutput(t, local, "switch", "-c", "feature/federated-guide")
	if err := os.WriteFile(filepath.Join(local, "GUIDE.md"), []byte("Verified across two communities.\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, local, "add", "GUIDE.md")
	gitOutput(t, local, "commit", "-m", "Add federation guide")
	sourceTip := gitOutput(t, local, "rev-parse", "HEAD")
	gitOutput(t, local, "push", "-u", "origin", "feature/federated-guide")

	var remotePull pullrequests.PullRequest
	proposalBody := `{"source_branch":"feature/federated-guide","source_commit_id":"` + sourceTip + `","target_branch":"main","target_commit_id":"` + targetTip + `","title":"Add federation guide","body":"A change from the home community.","context":{"source_pull_reference":"pull-request:contribution@` + home.server.URL + `#repository=` + fork.ID + `"}}`
	workflowJSON(t, home.server.URL, http.MethodPost, "/federation/repositories/"+fork.ID+"/proposals", developer, proposalBody, http.StatusCreated, &remotePull)
	if remotePull.AuthorID != "user:developer@"+home.server.URL || remotePull.FederatedContext == nil {
		t.Fatalf("remote pull lost provenance: %#v", remotePull)
	}
	pullBase := "/repositories/" + project.ID + "/pull-requests/" + remotePull.ID
	targetPullRef := federatedPullReference(upstream.server.URL, project.ID, remotePull.ID)

	workflowJSON(t, upstream.server.URL, http.MethodPost, pullBase+"/federated-events", maintainer, `{"idempotency_key":"maintainer-question","target_instance":"`+home.server.URL+`","target_reference":"pull-request:contribution@`+home.server.URL+`#repository=`+fork.ID+`","kind":"discussion","revision":"`+sourceTip+`","body":"Please record the verification command.","audience":"participants"}`, http.StatusCreated, nil)
	workflowJSON(t, home.server.URL, http.MethodPost, "/repositories/"+fork.ID+"/pull-requests/contribution/federated-events", developer, `{}`, http.StatusNotFound, nil)

	var sessionResponse struct {
		Session    federatedagents.Session `json:"session"`
		Credential struct {
			Token string `json:"token"`
		} `json:"credential"`
	}
	sessionBody := `{"agent":"codex","purpose":"verify the proposed guide","instructions":"Add locally governed verification evidence","target_pull_reference":"` + targetPullRef + `","revision":"` + sourceTip + `","branch":"feature/federated-guide","paths":["GUIDE.md"],"evidence":["go test ./..."]}`
	workflowJSON(t, home.server.URL, http.MethodPost, "/federation/repositories/"+fork.ID+"/agent-sessions", developer, sessionBody, http.StatusCreated, &sessionResponse)
	agentClone := gitClone(t, authenticatedGitURL(home.server.URL, fork.ID, sessionResponse.Credential.Token))
	gitOutput(t, agentClone, "config", "user.name", "Codex Agent")
	gitOutput(t, agentClone, "config", "user.email", "codex@home.example")
	gitOutput(t, agentClone, "switch", "feature/federated-guide")
	if err := os.WriteFile(filepath.Join(agentClone, "GUIDE.md"), []byte("Verified across two communities.\n\nTest: go test ./...\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, agentClone, "add", "GUIDE.md")
	gitOutput(t, agentClone, "commit", "-m", "Record agent verification")
	agentTip := gitOutput(t, agentClone, "rev-parse", "HEAD")
	gitOutput(t, agentClone, "push", "origin", "feature/federated-guide")
	publication := `{"summary":"Verified the guide locally","commands":["go test ./..."],"evidence":["tests passed"],"costs":{"tokens":"1200"},"residual_concerns":["Upstream candidate remains immutable"]}`
	workflowJSON(t, home.server.URL, http.MethodPost, "/federation/repositories/"+fork.ID+"/agent-sessions/"+sessionResponse.Session.ID+"/publication", sessionResponse.Credential.Token, publication, http.StatusCreated, nil)
	var exchanged struct {
		Items []federation.PullRequestEvent `json:"items"`
	}
	workflowJSON(t, upstream.server.URL, http.MethodGet, pullBase+"/federated-events", maintainer, "", http.StatusOK, &exchanged)
	if len(exchanged.Items) != 1 || exchanged.Items[0].ActorSubject != "agent:codex@"+home.server.URL || exchanged.Items[0].Current || exchanged.Items[0].Revision != agentTip {
		t.Fatalf("agent observation = %#v", exchanged.Items)
	}

	workflowJSON(t, upstream.server.URL, http.MethodPut, pullBase+"/reviews/me", maintainer, `{"decision":"approve","body":"Verified remote provenance and evidence."}`, http.StatusOK, nil)
	var merged pullrequests.PullRequest
	workflowJSON(t, upstream.server.URL, http.MethodPost, pullBase+"/merge", maintainer, `{}`, http.StatusOK, &merged)
	if merged.MergeCommitID == "" || merged.SourceCommitID != sourceTip {
		t.Fatalf("merged candidate = %#v", merged)
	}
	var receipts struct {
		Items []federation.MergeReceipt `json:"items"`
	}
	workflowJSON(t, home.server.URL, http.MethodGet, "/federation/contribution-receipts", developer, "", http.StatusOK, &receipts)
	if len(receipts.Items) != 1 || receipts.Items[0].MergeCommitID != merged.MergeCommitID || receipts.Items[0].AuthorSubject != remotePull.AuthorID {
		t.Fatalf("receipt = %#v", receipts.Items)
	}

	// Exact duplicate delivery is harmless. A rotated, chained identity remains
	// verifiable after rediscovery; an outage retains the last document; and
	// revocation contains future authority without erasing the receipt.
	replayed, err := home.federation.PutMergeReceipt(receipts.Items[0])
	if err != nil || replayed.ID != receipts.Items[0].ID {
		t.Fatalf("receipt replay = %#v, %v", replayed, err)
	}
	oldDoc, _ := upstream.federation.Document()
	rotated, err := upstream.federation.Rotate()
	if err != nil || rotated.PreviousDigest != federation.Digest(oldDoc) {
		t.Fatalf("rotation = %#v, %v", rotated, err)
	}
	if _, err = home.federation.Observe(upstream.server.URL+"/.well-known/komodo-federation", rotated, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = home.federation.Observe(upstream.server.URL+"/.well-known/komodo-federation", federation.Document{}, errors.New("temporary outage")); err != nil {
		t.Fatal(err)
	}
	peer, _ := home.federation.Peer(upstream.server.URL)
	if peer.Status != "unreachable" || peer.LastDocument == nil || peer.LastDocument.KeyID != rotated.KeyID {
		t.Fatalf("outage state = %#v", peer)
	}
	if _, err = home.federation.Observe(upstream.server.URL+"/.well-known/komodo-federation", rotated, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = home.federation.Trust(upstream.server.URL, "revoke"); err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, home.server.URL, http.MethodGet, "/federation/contribution-receipts", developer, "", http.StatusOK, &receipts)
	if len(receipts.Items) != 1 || receipts.Items[0].CurrentTrust != "revoked" || receipts.Items[0].Verification != "verified_peer_signature" {
		t.Fatalf("retained revoked receipt = %#v", receipts.Items)
	}
}

func newFederationWorkflowInstance(t *testing.T, name string) *federationWorkflowInstance {
	t.Helper()
	server := httptest.NewUnstartedServer(nil)
	instanceURL := "https://" + server.Listener.Addr().String()
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	proposalStore, _ := proposals.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	activityStore, _ := activities.New(t.TempDir(), userStore)
	releaseStore, _ := releases.New(t.TempDir())
	pathways, _ := contributorpathways.New(t.TempDir())
	issueStore, _ := issues.New(t.TempDir())
	opportunities, _ := contributionopportunities.New(t.TempDir())
	agents, _ := federatedagents.New(t.TempDir())
	config := federation.Config{Instance: instanceURL, Operators: []federation.Operator{{Name: name + " operator", Contact: "mailto:" + name + "@example.test"}}, Capabilities: []string{"identity.discovery", "repository.discovery", "repository.contributions", "pull_request.exchange", "repository.contribution_receipts"}, Endpoints: federation.Endpoints{Discovery: instanceURL + "/.well-known/komodo-federation", Actors: instanceURL + "/federation/actors/{kind}/{id}", Repositories: instanceURL + "/federation/repositories/{id}", RepositoryObjects: instanceURL + "/federation/repositories/{id}/objects", Contributions: instanceURL + "/federation/contributions", PullRequestEvents: instanceURL + "/federation/pull-request-events", ContributionReceipts: instanceURL + "/federation/contribution-receipts"}}
	fed, err := federation.New(t.TempDir(), config)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerGitHTTP(mux, catalog, credentials)
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, proposalStore, catalog, credentials, activityStore, fed)
	registerFederationHTTP(mux, fed, credentials)
	registerFederatedRepositoriesHTTP(mux, fed, catalog, pulls, releaseStore, pathways, issueStore, opportunities, activityStore, credentials)
	registerFederatedAgentSessionsHTTP(mux, agents, fed, catalog, credentials)
	server.Config.Handler = mux
	server.StartTLS()
	return &federationWorkflowInstance{server: server, federation: fed, repositories: catalog, pulls: pulls, credentials: credentials, agents: agents}
}

func observeAndTrust(t *testing.T, local, remote *federationWorkflowInstance) {
	t.Helper()
	doc, err := remote.federation.Document()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = local.federation.Observe(remote.server.URL+"/.well-known/komodo-federation", doc, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = local.federation.Trust(remote.server.URL, "trust"); err != nil {
		t.Fatal(err)
	}
}

func authenticatedGitURL(origin, repositoryID, token string) string {
	u, _ := url.Parse(origin + "/repositories/" + repositoryID)
	u.User = url.UserPassword("git", token)
	return u.String()
}
