package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenanceassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancebundles"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancegraphs"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancepolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestSoftwareProvenanceWorkflow is the black-box boundary for the complete
// contribution-to-verifiable-distribution loop. It crosses public provenance,
// Git, release, package-registry, and consumer-verification surfaces.
func TestSoftwareProvenanceWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm is required for the package consumer boundary")
	}
	gitStore, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	policies, _ := provenancepolicies.New(t.TempDir())
	graphs, _ := provenancegraphs.New(t.TempDir())
	assessments, _ := provenanceassessments.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	builds, _ := checkruns.New(t.TempDir())
	packages, _ := packagecatalog.New(t.TempDir())
	bundles, _ := provenancebundles.New(t.TempDir())
	orgs, _ := organizations.New(t.TempDir())

	repo, _ := repos.Create("owner", repositories.Metadata{Name: "trusted-sdk", Visibility: repositories.Public})
	for _, actor := range []string{"human", "agent", "reviewer"} {
		_, _ = repos.AddCollaborator("owner", repo.ID, actor)
	}
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reviewer := issueAccess(t, credentials, "reviewer", auth.API, auth.RepositoryRead)
	gitToken := issueAccess(t, credentials, "owner", auth.Git, auth.GitRead, auth.GitWrite)

	mux := http.NewServeMux()
	registerGitHTTP(mux, repos, credentials)
	registerProvenancePoliciesHTTP(mux, policies, repos, orgs, credentials)
	registerProvenanceGraphsHTTP(mux, graphs, repos, credentials)
	registerProvenanceAssessmentsHTTP(mux, assessments, graphs, policies, pulls, repos, credentials)
	registerProvenanceBundlesHTTP(mux, bundles, releaseStore, builds, graphs, assessments, packages, repos, credentials)
	registerPackagesHTTP(mux, packages, releaseStore, builds, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	root := "/repositories/" + string(repo.ID)

	remote, _ := url.Parse(server.URL + root)
	remote.User = url.UserPassword("git", gitToken)
	clone := gitClone(t, remote.String())
	gitOutput(t, clone, "config", "user.name", "Human Contributor")
	gitOutput(t, clone, "config", "user.email", "human@example.test")
	writeWorkflowFile(t, clone, "index.js", "module.exports = {provenance: 'candidate'};\n")
	writeWorkflowFile(t, clone, "copied.js", "export const copied = 'unattributed';\n")
	writeWorkflowFile(t, clone, "generated.js", "export const generated = 'old-tool';\n")
	badDeclaration := provenanceDeclaration(t, clone, "GPL-3.0", "generator-v1", true)
	writeWorkflowFile(t, clone, ".komodo/provenance.json", badDeclaration)
	gitOutput(t, clone, "add", ".")
	gitOutput(t, clone, "commit", "-m", "Human contribution with federated and generated inputs")
	gitOutput(t, clone, "push", "-u", "origin", "main")
	badRevision := gitOutput(t, clone, "rev-parse", "HEAD")

	policyInput := provenancepolicies.Input{Name: "Public distribution provenance", Description: "Accept attributable local, agent, and federated Apache inputs", ChangeReason: "establish release gate", OwnerIDs: []string{"owner"}, Rules: []provenancepolicies.MaterialRule{
		{Kind: "source", Origins: []string{"local", "agent", "federated"}, Licenses: []string{"Apache-2.0"}, Uses: []string{"public_distribution"}, Attestations: []string{"DCO"}, ReviewOwnerIDs: []string{"owner"}},
		{Kind: "generated_code", Origins: []string{"agent"}, Licenses: []string{"Apache-2.0"}, Uses: []string{"public_distribution"}, ReviewOwnerIDs: []string{"owner"}},
		{Kind: "package", Origins: []string{"registry", "private"}, Licenses: []string{"Apache-2.0"}, Uses: []string{"public_distribution"}, ReviewOwnerIDs: []string{"owner"}},
		{Kind: "build_input", Origins: []string{"local"}, Licenses: []string{"Apache-2.0"}, Uses: []string{"build"}, ReviewOwnerIDs: []string{"owner"}},
	}, DistributionContexts: []provenancepolicies.DistributionContext{{ID: "public", Audience: "public", Uses: []string{"public_distribution"}, Licenses: []string{"Apache-2.0"}, NoticeRequired: true, OwnerIDs: []string{"owner"}}}, Links: []provenancepolicies.Link{{Kind: "contributor_pathway", Reference: "local:human"}, {Kind: "agent_contract", Reference: "agent:replacement", Revision: "v2"}, {Kind: "contribution_boundary", Reference: "peer:upstream", Boundary: "federated"}}}
	var policy provenancepolicies.Policy
	workflowValue(t, server.URL, http.MethodPost, root+"/provenance-policies", owner, policyInput, http.StatusCreated, &policy)

	var badGraph provenancegraphs.Graph
	workflowValue(t, server.URL, http.MethodPost, root+"/provenance-graphs", owner, map[string]string{"revision": badRevision}, http.StatusCreated, &badGraph)
	var blocked provenanceassessments.View
	workflowValue(t, server.URL, http.MethodPost, root+"/provenance-assessments", owner, map[string]any{"candidate_kind": "release_candidate", "candidate_id": "v1.0.0-rc1", "revision": badRevision, "graph_id": badGraph.ID, "policy_id": policy.ID, "policy_version": 1, "distribution_targets": []string{"public"}}, http.StatusCreated, &blocked)
	if blocked.Ready || !findingKinds(blocked)["incompatible_license"] || !findingKinds(blocked)["unattributed_material"] {
		t.Fatalf("incompatible dependency and unattributed fragment did not block: %#v", blocked)
	}
	assessmentBase := root + "/provenance-assessments/" + blocked.ID
	workflowValue(t, server.URL, http.MethodPost, assessmentBase+"/annotations", reviewer, map[string]any{"expected_revision": blocked.RevisionNumber, "actor_kind": "agent", "annotation": map[string]any{"kind": "challenge", "finding_id": blocked.Findings[0].ID, "body": "scanner match is false; exact upstream differs", "citation": "federated:upstream@reviewed", "origin": "federated", "audience": "repository"}}, http.StatusCreated, &blocked)
	workflowValue(t, server.URL, http.MethodPost, assessmentBase+"/annotations", agent, map[string]any{"expected_revision": blocked.RevisionNumber, "actor_kind": "human", "annotation": map[string]any{"kind": "challenge", "finding_id": blocked.Findings[0].ID, "body": "human disputes the automated authorship label", "citation": "commit:" + badRevision, "audience": "restricted"}}, http.StatusCreated, &blocked)
	workflowValue(t, server.URL, http.MethodPost, assessmentBase+"/decisions", owner, map[string]any{"expected_revision": blocked.RevisionNumber, "decision": map[string]any{"finding_id": blocked.Findings[0].ID, "decision": "exception", "rationale": "expired exception must not bypass replacement", "expires_at": time.Now().Add(-time.Hour)}}, http.StatusUnprocessableEntity, nil)

	for _, finding := range blocked.Findings {
		if finding.Kind != "incompatible_license" && finding.Kind != "unattributed_material" {
			continue
		}
		strategy, ownerKind := "replace", "agent"
		if finding.Kind == "incompatible_license" {
			ownerKind = "human"
		}
		var made struct {
			Assessment provenanceassessments.View   `json:"assessment"`
			Repair     provenanceassessments.Repair `json:"repair"`
		}
		workflowValue(t, server.URL, http.MethodPost, assessmentBase+"/findings/"+finding.ID+"/repairs", owner, map[string]any{"expected_revision": blocked.RevisionNumber, "repair": map[string]any{"strategy": strategy, "owner_kind": ownerKind, "owner_id": ownerKind, "acceptance_criteria": []string{"Apache-2.0 origin and exact checks"}, "links": []map[string]string{{"kind": "branch", "resource_id": "repair/provenance"}}}}, http.StatusCreated, &made)
		blocked = made.Assessment
		workflowValue(t, server.URL, http.MethodPost, assessmentBase+"/repairs/"+made.Repair.ID+"/progress", agent, map[string]any{"expected_revision": blocked.RevisionNumber, "status": "completed", "summary": "replacement authored without restricted evidence"}, http.StatusCreated, &blocked)
	}

	writeWorkflowFile(t, clone, "copied.js", "export const replacement = 'agent-authored';\n")
	writeWorkflowFile(t, clone, "generated.js", "export const generated = 'generator-v2';\n")
	writeWorkflowFile(t, clone, ".komodo/provenance.json", provenanceDeclaration(t, clone, "Apache-2.0", "generator-v2", false))
	gitOutput(t, clone, "add", ".")
	gitOutput(t, clone, "commit", "-m", "Agent and human provenance replacements reviewed")
	gitOutput(t, clone, "push", "origin", "main")
	finalRevision := gitOutput(t, clone, "rev-parse", "HEAD")
	var graph provenancegraphs.Graph
	workflowValue(t, server.URL, http.MethodPost, root+"/provenance-graphs", owner, map[string]string{"revision": finalRevision}, http.StatusCreated, &graph)
	if graph.DeclarationSHA256 == badGraph.DeclarationSHA256 {
		t.Fatal("changed generator did not invalidate the graph input")
	}
	var current provenanceassessments.View
	workflowValue(t, server.URL, http.MethodPost, root+"/provenance-assessments", owner, map[string]any{"candidate_kind": "release_candidate", "candidate_id": "v1.0.0", "revision": finalRevision, "graph_id": graph.ID, "policy_id": policy.ID, "policy_version": 1, "distribution_targets": []string{"public"}}, http.StatusCreated, &current)
	for _, finding := range current.Findings {
		workflowValue(t, server.URL, http.MethodPost, root+"/provenance-assessments/"+current.ID+"/decisions", owner, map[string]any{"expected_revision": current.RevisionNumber, "decision": map[string]any{"finding_id": finding.ID, "decision": "resolved", "rationale": "exact replacement, notice, review, and attestation verified"}}, http.StatusCreated, &current)
	}
	if !current.Ready {
		t.Fatalf("current provenance gate did not pass: %#v", current.Blockers)
	}

	artifactBytes := npmArtifact(t)
	release, _ := releaseStore.Create(releases.CreateParams{RepositoryID: string(repo.ID), Version: "v1.0.0", CommitID: finalRevision, CreatedByID: "owner"})
	run, _ := builds.Create(string(repo.ID), "release:"+release.ID, finalRevision, checkruns.Definition{Name: "package", Command: "npm pack"})
	run, _ = builds.Start(run.ID)
	artifact, _ := builds.AddArtifact(run.ID, "dist/trusted-sdk.tgz", "application/gzip", artifactBytes)
	run, _ = builds.Complete(run.ID, 0, false, "")
	var pkg packagecatalog.Version
	workflowValue(t, server.URL, http.MethodPost, root+"/packages", owner, map[string]any{"name": "trusted-sdk", "version": "1.0.0", "release_id": release.ID, "build_run_id": run.ID, "artifact_id": artifact.ID, "platform": map[string]string{"os": "linux", "arch": "amd64"}, "license": "Apache-2.0", "visibility": "public"}, http.StatusCreated, &pkg)

	var bundle provenancebundles.Bundle
	sourceDigest := sha256.Sum256([]byte(finalRevision))
	bundleInput := map[string]any{"audience": "public", "graph_id": graph.ID, "assessment_id": current.ID, "artifacts": []map[string]any{{"id": artifact.ID, "name": "trusted-sdk.tgz", "sha256": artifact.SHA256, "size": artifact.Size, "media_type": artifact.MediaType, "build_run_id": run.ID}}, "components": []map[string]any{{"kind": "package", "name": "trusted-sdk", "version": "1.0.0", "sha256": artifact.SHA256, "license": "Apache-2.0", "origin": "local-human-agent-federated", "dependencies": []string{"transitive-core@2.0.0"}, "notices": []string{"upstream attribution"}, "attestation_ids": []string{"source-1", "build-1"}}}, "licenses": []string{"Apache-2.0"}, "notices": []string{"NOTICE: local, agent, and federated contributors"}, "source_attestations": []map[string]string{{"id": "source-1", "kind": "source", "subject_sha256": hex.EncodeToString(sourceDigest[:]), "issuer": "reviewer", "reference": "review:" + finalRevision}}, "build_attestations": []map[string]string{{"id": "build-1", "kind": "build", "subject_sha256": artifact.SHA256, "issuer": "builder", "reference": "run:" + run.ID}}}
	workflowValue(t, server.URL, http.MethodPost, root+"/releases/"+release.ID+"/provenance-bundles", owner, bundleInput, http.StatusCreated, &bundle)
	var verification struct {
		Verified bool `json:"verified"`
	}
	workflowValue(t, server.URL, http.MethodPost, "/provenance-bundles/"+bundle.ID+"/verify", "", nil, http.StatusOK, &verification)
	if !verification.Verified {
		t.Fatal("consumer could not independently verify the signed bundle")
	}
	var packageProvenance struct {
		Total int `json:"total_count"`
	}
	workflowValue(t, server.URL, http.MethodGet, "/packages/"+pkg.ID+"/provenance", "", nil, http.StatusOK, &packageProvenance)
	if packageProvenance.Total != 1 {
		t.Fatalf("package digest did not resolve its bundle: %#v", packageProvenance)
	}

	consumer := t.TempDir()
	if err := os.WriteFile(filepath.Join(consumer, "package.json"), []byte(`{"private":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("npm", "install", "--ignore-scripts", "--no-audit", "--no-fund", "--registry", server.URL+"/package-registry", "@owner/trusted-sdk@1.0.0")
	cmd.Dir = consumer
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("npm install: %v\n%s", err, output)
	}

	for _, notice := range []map[string]string{
		{"kind": "attestation_revoked", "subject": "federated-upstream", "detail": "upstream revoked its source attestation", "evidence_reference": "upstream:revocation", "action": "reassess"},
		{"kind": "origin_gap", "subject": "copied-fragment", "detail": "post-release origin discovery requires correction", "evidence_reference": "discovery:2", "action": "open governed replacement"},
	} {
		workflowValue(t, server.URL, http.MethodPost, root+"/provenance-bundles/"+bundle.ID+"/observations", owner, notice, http.StatusCreated, &bundle)
	}
	if bundle.TrustStatus != "attention_required" || len(bundle.TrustNotices) != 2 || !bundles.Verify(bundle) {
		t.Fatalf("post-release recovery rewrote or failed to qualify signed provenance: %#v", bundle)
	}

	var publicGraphs struct {
		Items []provenancegraphs.Graph `json:"items"`
	}
	workflowValue(t, server.URL, http.MethodGet, root+"/provenance-graphs", "", nil, http.StatusOK, &publicGraphs)
	privateHidden := false
	for _, g := range publicGraphs.Items {
		for _, n := range g.Nodes {
			if n.ID == "dep:private" && n.Label == "inaccessible provenance node" {
				privateHidden = true
			}
		}
	}
	if !privateHidden {
		t.Fatal("private dependency provenance leaked through the public graph")
	}
}

func findingKinds(v provenanceassessments.View) map[string]bool {
	out := map[string]bool{}
	for _, finding := range v.Blockers {
		out[finding.Kind] = true
	}
	return out
}

func provenanceDeclaration(t *testing.T, clone, dependencyLicense, generator string, missingFragment bool) string {
	t.Helper()
	digest := func(name string) string {
		b, err := os.ReadFile(filepath.Join(clone, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		return hex.EncodeToString(sum[:])
	}
	fragmentCitations := []map[string]string{{"path": "copied.js", "blob_sha256": digest("copied.js")}}
	if missingFragment {
		fragmentCitations = nil
	}
	value := map[string]any{"nodes": []map[string]any{
		{"id": "source:index", "kind": "file", "label": "index.js by human", "audience": "public", "license": "Apache-2.0", "confidence": 1, "citations": []map[string]string{{"path": "index.js", "blob_sha256": digest("index.js")}}, "claims": []string{"origin=local", "DCO"}},
		{"id": "fragment:copied", "kind": "fragment", "label": "copied fragment", "audience": "repository", "license": "Apache-2.0", "confidence": .7, "citations": fragmentCitations, "claims": []string{"origin=agent", "DCO"}},
		{"id": "tool:generator", "kind": "tool", "label": generator, "audience": "public", "license": "Apache-2.0", "confidence": 1, "citations": []map[string]string{{"revision": generator}}, "claims": []string{"origin=local"}},
		{"id": "generated:client", "kind": "file", "label": "generated.js", "audience": "public", "license": "Apache-2.0", "confidence": 1, "transformation": "generated", "citations": []map[string]string{{"path": "generated.js", "blob_sha256": digest("generated.js")}}, "claims": []string{"origin=agent"}},
		{"id": "dep:transitive", "kind": "dependency", "label": "federated transitive package", "audience": "public", "license": dependencyLicense, "confidence": 1, "citations": []map[string]string{{"revision": "2.0.0"}}, "claims": []string{"origin=registry"}},
		{"id": "dep:private", "kind": "dependency", "label": "private build dependency", "audience": "restricted", "license": "Apache-2.0", "confidence": 1, "citations": []map[string]string{{"revision": "private-7"}}, "claims": []string{"origin=private"}},
	}, "edges": []map[string]string{{"from": "tool:generator", "to": "generated:client", "kind": "generated"}, {"from": "dep:transitive", "to": "source:index", "kind": "dependency"}, {"from": "dep:private", "to": "tool:generator", "kind": "build_input"}}}
	b, _ := json.Marshal(value)
	return string(b)
}

func npmArtifact(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	files := map[string]string{"package/package.json": `{"name":"@owner/trusted-sdk","version":"1.0.0","main":"index.js"}`, "package/index.js": "module.exports={provenance:'verified'};\n"}
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
