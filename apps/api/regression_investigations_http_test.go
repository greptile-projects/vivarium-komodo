package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/regressioninvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type regressionTestReleases struct{ release releases.Release }

func (s regressionTestReleases) Get(repo, id string) (releases.Release, error) {
	if repo == s.release.RepositoryID && id == s.release.ID {
		return s.release, nil
	}
	return releases.Release{}, releases.ErrNotFound
}

type regressionTestBuilds struct{ runs []checkruns.Run }

func (s regressionTestBuilds) List(string, string) ([]checkruns.Run, error) { return s.runs, nil }
func (s regressionTestBuilds) Get(string, string, string) (checkruns.Run, error) {
	return checkruns.Run{}, os.ErrNotExist
}
func (s regressionTestBuilds) OpenArtifact(string, string, string, string) (checkruns.Artifact, *os.File, error) {
	return checkruns.Artifact{}, nil, os.ErrNotExist
}

func regressionBody(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRegressionInvestigationDefinesSharedComparableBoundary(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	store, _ := ri.New(t.TempDir())
	repository, _ := repos.Create("owner", repositories.Metadata{Name: "regression", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repository.ID, "collaborator")
	opened, _ := repos.Open(repository.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	good, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor A <a@example.test> 1 +0000\ncommitter A <a@example.test> 1 +0000\n\ngood\n", tree)))
	bad, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor A <a@example.test> 2 +0000\ncommitter A <a@example.test> 2 +0000\n\nbad\n", tree, good)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/good", ObjectID: good})
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: bad})
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	peer := issueAccess(t, credentials, "collaborator", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerRegressionInvestigationsHTTP(mux, store, repos, credentials, nil, nil)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repository.ID) + "/regression-investigations"
	body := `{"title":"Review navigation stopped","source":{"kind":"failed_check","resource_id":"check-42","revision":"` + string(bad) + `"},"scope":{"expected_behavior":"Review opens from the keyboard","regressed_behavior":"The shortcut does nothing","known_good":{"kind":"revision","reference":"good"},"known_bad":{"kind":"revision","reference":"main"},"environments":["linux/chromium"],"comparability":"Same synthetic fixture and browser major version","severity":"high","owner_ids":["owner"],"acceptance_criteria":["shortcut opens review"]},"evidence":[{"kind":"reproduction","resource_id":"repro-7","summary":"Credential-free keyboard reproduction","audience":"repository"}]}`
	var investigation ri.Investigation
	workflowJSON(t, server.URL, http.MethodPost, base, peer, body, 201, &investigation)
	if investigation.Scope.KnownGood.CommitID != string(good) || investigation.Scope.KnownBad.CommitID != string(bad) || len(investigation.Blockers) != 0 || investigation.Evidence[0].ActorID != "collaborator" || investigation.ScopeChanges[0].ActorID != "collaborator" {
		t.Fatalf("boundary was not retained: %#v", investigation)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/entries", owner, `{"kind":"hypothesis","body":"The navigation refactor changed shortcut routing."}`, 201, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/status", owner, `{"status":"ready","reason":"The team agrees on the bounded history and success condition."}`, 200, &investigation)
	if investigation.Status != "ready" || len(investigation.Entries) != 2 || investigation.Entries[0].ActorID != "owner" {
		t.Fatalf("collaboration trail missing: %#v", investigation)
	}
	newBad, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor A <a@example.test> 3 +0000\ncommitter A <a@example.test> 3 +0000\n\nnew bad\n", tree, bad)))
	_ = opened.UpdateReference(storage.Reference{Name: "refs/heads/main", ObjectID: newBad})
	workflowJSON(t, server.URL, http.MethodGet, base+"/"+investigation.ID, peer, "", 200, &investigation)
	if len(investigation.StaleInputs) != 1 || investigation.StaleInputs[0] != "known_bad_revision_changed" {
		t.Fatalf("branch movement was not exposed: %#v", investigation.StaleInputs)
	}
	workflowJSON(t, server.URL, http.MethodPost, base, peer, `{"title":"Reversed","source":{"kind":"issue","resource_id":"1"},"scope":{"known_good":{"kind":"revision","reference":"main"},"known_bad":{"kind":"revision","reference":"good"}}}`, 422, nil)
}

func TestRegressionInvestigationExposesMissingInputsAndScopeHistory(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	store, _ := ri.New(t.TempDir())
	repository, _ := repos.Create("owner", repositories.Metadata{Name: "gaps", Visibility: repositories.Public})
	opened, _ := repos.Open(repository.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor A <a@x> 1 +0000\ncommitter A <a@x> 1 +0000\n\nbase\n", tree)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerRegressionInvestigationsHTTP(mux, store, repos, credentials, nil, nil)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repository.ID) + "/regression-investigations"
	var v ri.Investigation
	workflowJSON(t, server.URL, http.MethodPost, base, token, `{"title":"Unbounded report","source":{"kind":"issue","resource_id":"issue-9"},"scope":{}}`, 201, &v)
	if len(v.Blockers) != 9 {
		t.Fatalf("missing inputs hidden: %#v", v.Blockers)
	}
	scope := `{"expected_version":1,"reason":"Support confirmed both endpoints and the affected runtime.","scope":{"expected_behavior":"works","regressed_behavior":"fails","known_good":{"kind":"revision","reference":"main"},"known_bad":{"kind":"revision","reference":"main"},"environments":["test"],"comparability":"same fixture","severity":"medium","owner_ids":["owner"],"acceptance_criteria":["works"]}}`
	workflowJSON(t, server.URL, http.MethodPut, base+"/"+v.ID+"/scope", token, scope, 200, &v)
	if v.Version != 2 || len(v.Blockers) != 0 || len(v.ScopeChanges) != 2 || v.ScopeChanges[1].Reason == "" {
		t.Fatalf("scope history incomplete: %#v", v)
	}
}

func TestRegressionScenarioRetainsComparableHistoricalAttempts(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	store, _ := ri.New(t.TempDir())
	repository, _ := repos.Create("owner", repositories.Metadata{Name: "history", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repository.ID, "agent-1")
	opened, _ := repos.Open(repository.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	good, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor A <a@x> 1 +0000\ncommitter A <a@x> 1 +0000\n\ngood\n", tree)))
	bad, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor A <a@x> 2 +0000\ncommitter A <a@x> 2 +0000\n\nbad\n", tree, good)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/good", ObjectID: good})
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: bad})
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "agent-1", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerRegressionInvestigationsHTTP(mux, store, repos, credentials, regressionTestReleases{release: releases.Release{ID: "release-1", RepositoryID: string(repository.ID), Version: "v1.0.0", CommitID: string(bad)}}, regressionTestBuilds{runs: []checkruns.Run{{ID: "build-1", Definition: checkruns.Definition{Name: "build"}, State: checkruns.Succeeded}}})
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repository.ID) + "/regression-investigations"
	var v ri.Investigation
	workflowJSON(t, server.URL, http.MethodPost, base, owner, `{"title":"Historical parser behavior","source":{"kind":"issue","resource_id":"issue-1"},"scope":{"expected_behavior":"legacy input parses","regressed_behavior":"legacy input is rejected","known_good":{"kind":"revision","reference":"good"},"known_bad":{"kind":"revision","reference":"main"},"environments":["linux"],"comparability":"same synthetic input","severity":"high","owner_ids":["owner"],"acceptance_criteria":["classification is stable"]}}`, 201, &v)
	definition := ri.ScenarioDefinition{Title: "Parse legacy fixture", Inputs: []ri.ScenarioInput{{Name: "fixture", Kind: "artifact_reference", Value: "artifact:legacy-v1"}}, Commands: []string{"bun install --frozen-lockfile", "bun test parser"}, Fixtures: []ri.Fixture{{Name: "legacy", Reference: "artifact:legacy-v1", Classification: "synthetic"}}, EnvironmentRequirements: []string{"networkless", "linux/amd64"}, TimeoutSeconds: 300, CostLimit: 5}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+v.ID+"/scenarios", agent, regressionBody(t, map[string]any{"derived": true, "definition": definition}), 201, &v)
	if len(v.Scenarios) != 1 || v.Scenarios[0].Definition.ExpectedBehavior != "legacy input parses" || v.Scenarios[0].CreatedByID != "agent-1" {
		t.Fatalf("derived scenario incomplete: %#v", v.Scenarios)
	}
	environment := ri.Environment{Image: "registry.test/parser@sha256:abc", DefinitionDigest: "sha256:environment", OS: "linux", Architecture: "amd64", Isolation: "isolated", Network: "none", Toolchain: map[string]string{"bun": "1.2.0"}, DependencyLockDigest: "sha256:lock", SetupCommands: []string{"bun install --frozen-lockfile"}}
	provenance := ri.Provenance{RunnerID: "runner-7", RunnerVersion: "3", ActorKind: "agent", StartedAt: "2026-08-23T10:00:00Z", CompletedAt: "2026-08-23T10:00:12Z", RepetitionCount: 3}
	attempt := ri.AttemptInput{Target: ri.Target{Kind: "revision", Reference: "good"}, Environment: environment, Inputs: definition.Inputs, Commands: definition.Commands, Outputs: []string{"legacy accepted"}, Logs: []string{"3 repetitions agreed"}, Artifacts: []ri.Artifact{{Name: "results.json", Digest: "sha256:results", MediaType: "application/json", Size: 42}}, Classification: "expected_behavior", Rationale: "All repetitions observed the declared expected behavior.", Cost: 1.25, Currency: "USD", Provenance: provenance}
	path := base + "/" + v.ID + "/scenarios/" + v.Scenarios[0].ID + "/attempts"
	workflowJSON(t, server.URL, http.MethodPost, path, agent, regressionBody(t, attempt), 201, &v)
	attempt.Target = ri.Target{Kind: "dependency_combination", Reference: "main", Dependencies: map[string]string{"parser-lib": "2.0.0", "runtime": "4.1.0"}}
	attempt.Classification, attempt.Rationale, attempt.Outputs = "missing_dependencies", "The historical registry no longer serves parser-lib 2.0.0.", nil
	workflowJSON(t, server.URL, http.MethodPost, path, owner, regressionBody(t, attempt), 201, &v)
	attempt.Target = ri.Target{Kind: "revision", Reference: "main"}
	attempt.Classification = "flaky"
	attempt.Rationale = "One of three isolated repetitions accepted the fixture."
	attempt.Provenance.RepetitionCount = 3
	workflowJSON(t, server.URL, http.MethodPost, path, agent, regressionBody(t, attempt), 201, &v)
	attempt.Target = ri.Target{Kind: "release", Reference: "release-1"}
	attempt.Classification = "regressed_behavior"
	attempt.Rationale = "The verified release consistently rejects the fixture."
	workflowJSON(t, server.URL, http.MethodPost, path, agent, regressionBody(t, attempt), 201, &v)
	if v.Attempts[3].Target.AttestationDigest == "" || v.Attempts[3].Target.ReleaseID != "release-1" {
		t.Fatalf("verified release provenance missing: %#v", v.Attempts[3].Target)
	}
	if len(v.Attempts) != 4 || v.Attempts[0].Target.CommitID != string(good) || v.Attempts[1].Target.CommitID != string(bad) || v.Attempts[1].Classification != "missing_dependencies" || v.Attempts[2].Classification != "flaky" || v.Attempts[0].Environment.DefinitionDigest == "" || len(v.Attempts[0].Artifacts) != 1 {
		t.Fatalf("historical attempts lost distinctions: %#v", v.Attempts)
	}
	attempt.Target = ri.Target{Kind: "release", Reference: "unattested"}
	attempt.Classification = "untestable_revision"
	attempt.Rationale = "No attestation"
	workflowJSON(t, server.URL, http.MethodPost, path, agent, regressionBody(t, attempt), 422, nil)
}
