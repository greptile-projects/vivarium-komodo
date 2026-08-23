package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workflowcomponents"
)

func TestWorkflowComponentsPublishPinAndRetainHistory(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	provider, _ := repos.Create("provider", repositories.Metadata{Name: "practices", Visibility: repositories.Public})
	consumer, _ := repos.Create("consumer", repositories.Metadata{Name: "product", Visibility: repositories.Public})
	providerToken := issueAccess(t, credentials, "provider", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	consumerToken := issueAccess(t, credentials, "consumer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := workflowcomponents.New(t.TempDir())
	mux := http.NewServeMux()
	registerWorkflowComponentsHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	publish := workflowcomponents.PublishInput{Name: "reviewed-repair", Version: "1.0.0", Summary: "Prepare a bounded repair", PackageVersionID: "package-v1", SourceRepositoryID: string(provider.ID), SourceRevision: "provider-commit-1", SourcePath: "components/repair.json", ArtifactDigest: "sha256:" + strings.Repeat("a", 64), Attestation: "release signature verified", Inputs: []workflowcomponents.Field{{Name: "issue", Type: "string", Required: true}}, Outputs: []workflowcomponents.Field{{Name: "proposal", Type: "string", Required: true}}, RequestedCapabilities: []string{"proposal:draft", "pull:open"}, Compatibility: workflowcomponents.Compatibility{Engine: "komodo-workflows", MinimumVersion: "1.0.0"}, DataUse: workflowcomponents.DataUse{Classes: []string{"repository_metadata"}, Purposes: []string{"repair"}, Retention: "execution lifetime"}, Tests: []workflowcomponents.TestEvidence{{Name: "bounded repair", Revision: "provider-commit-1", Status: "passed", Attestation: "isolated test run 9"}}, Support: workflowcomponents.Support{Policy: "maintained major", Contact: "https://provider.test/support"}, PublisherSubject: "user:provider@https://provider.test", PublisherInstance: "https://provider.test", FederationDocumentDigest: "sha256:" + strings.Repeat("b", 64), Visibility: "public"}
	b, _ := json.Marshal(publish)
	var component workflowcomponents.Component
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(provider.ID)+"/workflow-components", providerToken, string(b), 201, &component)
	install := workflowcomponents.InstallInput{ComponentID: component.ID, PullRequestID: "consumer-pr-7", PullRevision: "consumer-commit-1", Configuration: map[string]any{"labels": []any{"accepted"}}, Permissions: []workflowcomponents.PermissionMapping{{Requested: "proposal:draft", LocalPermission: "proposal:create", Resource: "repository:" + string(consumer.ID)}, {Requested: "pull:open", LocalPermission: "pull:create", Resource: "repository:" + string(consumer.ID)}}, Health: workflowcomponents.Health{Publisher: "unchanged", Trust: "trusted", Peer: "available", Vulnerability: "clear", Compatibility: "compatible"}, Reason: "adopt reviewed practice"}
	b, _ = json.Marshal(install)
	var installed workflowcomponents.Installation
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(consumer.ID)+"/workflow-component-installations", consumerToken, string(b), 201, &installed)
	if installed.State != "installed" || installed.GrantsAuthority || installed.Revisions[0].Component.SourceRevision != "provider-commit-1" {
		t.Fatalf("installation %#v", installed)
	}
	install.Health = workflowcomponents.Health{Publisher: "changed", Trust: "revoked", Peer: "unavailable", Vulnerability: "affected", Compatibility: "breaking"}
	install.PullRequestID = "consumer-pr-8"
	install.PullRevision = "consumer-commit-2"
	install.Reason = "evaluate upgrade while retaining pin"
	body := struct {
		ExpectedRevision int64 `json:"expected_revision"`
		workflowcomponents.InstallInput
	}{1, install}
	b, _ = json.Marshal(body)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(consumer.ID)+"/workflow-component-installations/"+installed.ID+"/revisions", consumerToken, string(b), 201, &installed)
	if installed.State != "attention_required" || len(installed.Blockers) != 5 || len(installed.Revisions) != 2 || installed.Revisions[0].Component.SourceRevision != "provider-commit-1" {
		t.Fatalf("retained history %#v", installed)
	}
}
