package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/protectionplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryobjectives"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestProtectionPlansExposeProofNotProtectedContents(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "continuity", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	objectives, _ := recoveryobjectives.New(t.TempDir())
	objective, _ := objectives.Create(string(repo.ID), "owner", recoveryobjectives.VersionInput{Title: "Git continuity", Description: "Retain committed history", OwnerIDs: []string{"owner"}, Resources: []recoveryobjectives.Resource{{ID: "git", Kind: "repository", Name: "Git", UserCapability: "clone", OwnerIDs: []string{"owner"}, AcceptableLoss: "0s", RestorationTime: "1h", Retention: "1y", Jurisdictions: []string{"EU"}, ValidationCriteria: []string{"refs resolve"}, Feasibility: "achievable"}}, ExceptionPolicy: "owner approval", ChangeReason: "initial"})
	plans, _ := protectionplans.New(t.TempDir())
	mux := http.NewServeMux()
	registerProtectionPlansHTTP(mux, plans, objectives, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/protection-plans"
	body := `{"objective_id":"` + objective.ID + `","objective_version":1,"resource_ids":["git"],"environment_id":"production","mode":"snapshot","schedule":"hourly","maximum_age_seconds":7200,"encryption":"AES-256-GCM","key_reference":"kms:continuity","access_scope":["recovery-owner"],"destinations":[{"id":"vault-eu","kind":"object_store","region":"eu-west","jurisdiction":"EU","authorized":true}],"retention":"1y","checksum_algorithm":"sha256","validation_criteria":["decrypt manifest","resolve refs"],"cost_limit":10,"currency":"USD","change_reason":"protect exact committed state"}`
	var plan protectionplans.Plan
	workflowJSON(t, server.URL, http.MethodPost, base, owner, body, 201, &plan)
	now := time.Now().UTC()
	capture := `{"idempotency_key":"provider-run-1","plan_version":1,"started_at":"` + now.Add(-time.Minute).Format(time.RFC3339) + `","captured_at":"` + now.Format(time.RFC3339) + `","resources":[{"resource_id":"git","source_version":"commit-abc","provenance":"repository:main@commit-abc","dependency_versions":{"object-store":"v2"},"object_count":42,"byte_count":4096,"checksum":"sha256:manifest","complete":true,"source_state":"committed"}],"validation":{"completeness_verified":true,"checksum_verified":true,"decryption_verified":true,"key_available":true,"destination_authorized":true,"validated_at":"` + now.Format(time.RFC3339) + `","evidence_digest":"sha256:validation"},"cost":2.5}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/captures", owner, capture, 201, &plan)
	if len(plan.Captures) != 1 || !plan.Captures[0].Recoverable || plan.LatestRecoverableCaptureID == "" || len(plan.MissingResources) != 0 {
		t.Fatalf("verified capture not recoverable: %+v", plan)
	}
	bad := strings.ReplaceAll(strings.ReplaceAll(capture, "provider-run-1", "provider-run-2"), `"key_available":true`, `"key_available":false`)
	bad = strings.ReplaceAll(bad, `"source_state":"committed"`, `"source_state":"deleted"`)
	bad = strings.ReplaceAll(bad, `"complete":true`, `"complete":false`)
	bad = strings.ReplaceAll(bad, `"checksum_verified":true`, `"checksum_verified":false`)
	bad = strings.ReplaceAll(bad, `"destination_authorized":true`, `"destination_authorized":false`)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/captures", owner, bad, 201, &plan)
	for _, want := range []string{"incomplete_capture", "corruption_or_checksum_unverified", "key_unavailable", "unauthorized_destination", "deleted_or_uncommitted_source"} {
		if plan.Captures[1].Recoverable || !containsFailure(plan.Captures[1].Failures, want) {
			t.Fatalf("unsafe capture missing %s: %+v", want, plan.Captures[1])
		}
	}
	if plan.Captures[1].Recoverable {
		t.Fatalf("unsafe capture presented as recoverable: %+v", plan.Captures[1])
	}
	var listed struct {
		Items []protectionplans.Plan `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, reader, "", 200, &listed)
	encodedBytes, err := json.Marshal(listed)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(encodedBytes)
	if strings.Contains(encoded, "protected payload") || listed.Items[0].Captures[0].ActorID != "owner" {
		t.Fatalf("reader proof projection: %s", encoded)
	}
	workflowJSON(t, server.URL, http.MethodPost, base, reader, body, 401, nil)
}

func containsFailure(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
