package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenanceassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancegraphs"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancepolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestProvenanceAssessmentFindingsCollaborationAndSelectiveStaleness(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "project", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repo.ID, "reader")
	opened, _ := repos.Open(repo.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor A <a@example.test> 1 +0000\ncommitter A <a@example.test> 1 +0000\n\ncandidate\n", tree)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})

	graphs, _ := provenancegraphs.New(t.TempDir())
	graph, _ := graphs.Create(provenancegraphs.Graph{RepositoryID: string(repo.ID), Revision: string(commit), DeclarationPath: ".komodo/provenance.json", DeclarationSHA256: "decl-1", CreatedByID: "owner", Nodes: []provenancegraphs.Node{{ID: "file:copied", Kind: "file", Label: "copied.go", Audience: "repository", License: "GPL-3.0", Confidence: .6, Obligations: []string{"provide source offer"}, Claims: []string{"origin=unknown"}}}, Gaps: []provenancegraphs.Gap{{Kind: "missing_origin", Subject: "file:copied", Detail: "no exact citation"}}})
	policies, _ := provenancepolicies.New(t.TempDir())
	policyInput := provenancepolicies.Input{
		Name: "acceptance", Description: "review before history", ChangeReason: "initial", OwnerIDs: []string{"owner"},
		Rules:                []provenancepolicies.MaterialRule{{Kind: "source", Origins: []string{"original"}, Licenses: []string{"Apache-2.0"}, Uses: []string{"public_distribution"}, Attribution: []string{"Copyright Example"}, Attestations: []string{"DCO"}, ReviewOwnerIDs: []string{"owner"}}},
		DistributionContexts: []provenancepolicies.DistributionContext{{ID: "public", Audience: "public", Uses: []string{"public_distribution"}, Licenses: []string{"Apache-2.0"}, NoticeRequired: true, OwnerIDs: []string{"owner"}}},
	}
	policy, _ := policies.Create("repository", string(repo.ID), "owner", policyInput)
	assessments, _ := provenanceassessments.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	mux := http.NewServeMux()
	registerProvenanceAssessmentsHTTP(mux, assessments, graphs, policies, pulls, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	base := "/repositories/" + string(repo.ID) + "/provenance-assessments"
	body, _ := json.Marshal(map[string]any{"candidate_kind": "release_candidate", "candidate_id": "v1", "revision": commit, "graph_id": graph.ID, "policy_id": policy.ID, "policy_version": 1, "distribution_targets": []string{"public"}})
	var made provenanceassessments.View
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(body), http.StatusCreated, &made)
	kinds := map[string]bool{}
	for _, finding := range made.Blockers {
		kinds[finding.Kind] = true
	}
	for _, want := range []string{"unattributed_material", "incompatible_license", "required_attribution", "contributor_attestation", "source_offer_obligation", "required_notice", "owner_acknowledgement"} {
		if !kinds[want] {
			t.Errorf("missing %s in %#v", want, made.Blockers)
		}
	}

	annotation := map[string]any{"expected_revision": made.RevisionNumber, "actor_kind": "agent", "annotation": map[string]any{"kind": "origin_evidence", "finding_id": made.Findings[0].ID, "body": "Found the upstream revision", "citation": "https://example.test/upstream/commit", "origin": "upstream"}}
	body, _ = json.Marshal(annotation)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+made.ID+"/annotations", reader, string(body), http.StatusCreated, &made)
	if len(made.Annotations) != 1 || made.Annotations[0].ActorKind != "agent" {
		t.Fatalf("cited agent evidence was not retained: %#v", made)
	}

	policyInput.ChangeReason = "new license review"
	policy, _ = policies.Revise("repository", string(repo.ID), policy.ID, "owner", 1, policyInput)
	var stale provenanceassessments.View
	workflowJSON(t, server.URL, http.MethodGet, base+"/"+made.ID, reader, "", http.StatusOK, &stale)
	if stale.Status != "stale" || len(stale.StaleInputKeys) != 1 || stale.StaleInputKeys[0].Kind != "policy" {
		t.Fatalf("policy-only invalidation was not selective: %#v (policy %#v)", stale, policy)
	}

	decision := map[string]any{"expected_revision": made.RevisionNumber, "decision": map[string]any{"finding_id": made.Findings[0].ID, "decision": "exception", "rationale": "replace before public release", "expires_at": time.Now().Add(24 * time.Hour)}}
	body, _ = json.Marshal(decision)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+made.ID+"/decisions", reader, string(body), http.StatusForbidden, nil)
}
