package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/regressioninvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestRegressionReportToSustainedRecovery is the black-box boundary for the
// complete "this used to work" collaboration loop. It deliberately retains
// evidence that cannot narrow history instead of turning it into a private,
// falsely precise bisect result.
func TestRegressionReportToSustainedRecovery(t *testing.T) {
	git, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	regressions, _ := ri.New(t.TempDir())
	repository, _ := repos.Create("maintainer", repositories.Metadata{Name: "parser", Visibility: repositories.Private})
	for _, id := range []string{"reporter", "agent"} {
		if _, err := repos.AddCollaborator("maintainer", repository.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	repo, _ := repos.Open(repository.ID)
	tree, _ := repo.WriteObject(storage.TreeObject, nil)
	commit := func(message string, parents ...storage.ObjectID) storage.ObjectID {
		body := "tree " + string(tree) + "\n"
		for _, parent := range parents {
			body += "parent " + string(parent) + "\n"
		}
		body += "author Developer <developer@example.test> 1 +0000\ncommitter Developer <developer@example.test> 1 +0000\n\n" + message + "\n"
		id, e := repo.WriteObject(storage.CommitObject, []byte(body))
		if e != nil {
			t.Fatal(e)
		}
		return id
	}
	good := commit("legacy documents parse")
	flaky := commit("update dependency", good)
	unbuildable := commit("historical toolchain no longer available", flaky)
	side := commit("preserve the new strict syntax", good)
	merge := commit("merge strict syntax and dependency work", unbuildable, side)
	repair := commit("accept legacy headers while retaining strict syntax", merge)
	backport := commit("backport legacy header compatibility", good)
	_ = repo.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: merge})
	_ = repo.CreateReference(storage.Reference{Name: "refs/heads/last-working", ObjectID: good})
	_ = repo.CreateReference(storage.Reference{Name: "refs/heads/repair", ObjectID: repair})
	_ = repo.CreateReference(storage.Reference{Name: "refs/heads/backport", ObjectID: backport})

	reporter := issueAccess(t, credentials, "reporter", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerRegressionInvestigationsHTTP(mux, regressions, repos, credentials, nil, nil)
	server := httptest.NewServer(mux)
	defer server.Close()
	root := "/repositories/" + string(repository.ID) + "/regression-investigations"

	var investigation ri.Investigation
	workflowJSON(t, server.URL, http.MethodPost, root, reporter, regressionBody(t, map[string]any{
		"title": "Legacy headers stopped parsing", "source": map[string]any{"kind": "support_thread", "resource_id": "support:482", "revision": "message:7"},
		"scope":    map[string]any{"expected_behavior": "legacy and strict headers parse", "regressed_behavior": "legacy headers fail", "known_good": map[string]any{"kind": "revision", "reference": "last-working"}, "known_bad": map[string]any{"kind": "revision", "reference": "main"}, "environments": []string{"linux/amd64", "protected-production"}, "comparability": "same synthetic legacy document and parser entry point", "severity": "high", "owner_ids": []string{"maintainer"}, "acceptance_criteria": []string{"legacy headers parse"}},
		"evidence": []map[string]any{{"kind": "support_thread", "resource_id": "support:482", "revision": "message:7", "summary": "User impact began after v2 rollout", "audience": "repository"}},
	}), http.StatusCreated, &investigation)
	if investigation.CreatorID != "reporter" || investigation.Scope.KnownGood.CommitID != string(good) {
		t.Fatalf("attributed user report lost its historical boundary: %#v", investigation)
	}

	definition := ri.ScenarioDefinition{Title: "Parse preserved legacy document", Inputs: []ri.ScenarioInput{{Name: "document", Kind: "artifact_reference", Value: "fixture:legacy-v1"}}, Commands: []string{"parser-test fixture:legacy-v1"}, Fixtures: []ri.Fixture{{Name: "legacy document", Reference: "fixture:legacy-v1", Classification: "synthetic"}}, EnvironmentRequirements: []string{"networkless", "revision-matched toolchain"}, TimeoutSeconds: 120, CostLimit: 4}
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+investigation.ID+"/scenarios", maintainer, regressionBody(t, map[string]any{"derived": true, "definition": definition}), http.StatusCreated, &investigation)
	scenario := investigation.Scenarios[0]
	path := root + "/" + investigation.ID + "/scenarios/" + scenario.ID + "/attempts"
	addAttempt := func(target storage.ObjectID, class, rationale string, repetitions int64, dependencies map[string]string) string {
		t.Helper()
		kind := "revision"
		if dependencies != nil {
			kind = "dependency_combination"
		}
		in := ri.AttemptInput{Target: ri.Target{Kind: kind, Reference: string(target), Dependencies: dependencies}, Environment: ri.Environment{Image: "registry.test/parser@sha256:environment", DefinitionDigest: "sha256:environment", OS: "linux", Architecture: "amd64", Isolation: "isolated", Network: "none", Toolchain: map[string]string{"go": "1.25"}, DependencyLockDigest: "sha256:lock"}, Inputs: definition.Inputs, Commands: definition.Commands, Classification: class, Rationale: rationale, Cost: 1, Currency: "USD", Provenance: ri.Provenance{RunnerID: "regression-runner", RunnerVersion: "4", ActorKind: "agent", StartedAt: "2026-08-23T20:00:00Z", CompletedAt: "2026-08-23T20:00:01Z", RepetitionCount: repetitions}}
		workflowJSON(t, server.URL, http.MethodPost, path, agent, regressionBody(t, in), http.StatusCreated, &investigation)
		return investigation.Attempts[len(investigation.Attempts)-1].ID
	}
	goodRun := addAttempt(good, "expected_behavior", "Earlier revision consistently preserves the user behavior.", 3, nil)
	flakyRun := addAttempt(flaky, "flaky", "The midpoint disagreed across five identical repetitions.", 5, map[string]string{"parser-core": "2.1.0"})
	unbuildableRun := addAttempt(unbuildable, "untestable_revision", "The pinned compiler artifact is no longer buildable.", 1, nil)
	sideRun := addAttempt(side, "expected_behavior", "The side parent preserves both old and strict syntax.", 2, nil)
	badRun := addAttempt(merge, "regressed_behavior", "The merge result consistently rejects the legacy fixture.", 3, nil)

	graph := []ri.SearchRevision{
		{Key: string(good), Kind: "commit", Revision: string(good), Summary: "last supported release", OwnerIDs: []string{"maintainer"}},
		{Key: string(flaky), Kind: "commit", Revision: string(flaky), Parents: []string{string(good)}, Summary: "dependency update", DiffPaths: []string{"go.mod"}},
		{Key: "parser-core@2.1.0", Kind: "package_revision", Package: "parser-core", Revision: "2.1.0", Parents: []string{string(flaky)}, Summary: "selected dependency history"},
		{Key: string(unbuildable), Kind: "commit", Revision: string(unbuildable), Parents: []string{string(flaky)}, Summary: "unbuildable midpoint"},
		{Key: string(side), Kind: "commit", Revision: string(side), Parents: []string{string(good)}, Summary: "strict syntax intent", DecisionIDs: []string{"decision:strict-syntax"}},
		{Key: string(merge), Kind: "commit", Revision: string(merge), Parents: []string{string(unbuildable), string(side)}, Summary: "merge resolution", DiffPaths: []string{"parser/header.go"}, PullIDs: []string{"pull:strict-parser"}, OwnerIDs: []string{"maintainer"}},
	}
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+investigation.ID+"/searches", agent, regressionBody(t, ri.SearchInput{ScenarioID: scenario.ID, GoodKey: string(good), BadKey: string(merge), Revisions: graph, ConfidenceTarget: .7}), http.StatusCreated, &investigation)
	search := investigation.Searches[0]
	classify := func(key, class string, ids []string, rationale string) {
		workflowJSON(t, server.URL, http.MethodPost, root+"/"+investigation.ID+"/searches/"+search.ID+"/classifications", maintainer, regressionBody(t, ri.CandidateClassification{RevisionKey: key, AttemptIDs: ids, Classification: class, Rationale: rationale}), http.StatusCreated, &investigation)
	}
	classify(string(good), "working", []string{goodRun}, "Release evidence establishes the boundary.")
	classify(string(flaky), "flaky", []string{flakyRun}, "Nondeterminism cannot support a culprit claim.")
	classify("parser-core@2.1.0", "invalid", []string{flakyRun}, "The dependency correlation was the initial false culprit.")
	classify(string(unbuildable), "invalid", []string{unbuildableRun}, "An unbuildable commit is not passing or failing evidence.")
	classify(string(side), "working", []string{sideRun}, "The strict-syntax parent still accepts legacy headers.")
	classify(string(merge), "regressed", []string{badRun}, "Only the resolved merge tree reproduces the failure.")
	search = investigation.Searches[0]
	if search.Verdict != "multiple_or_ambiguous" || len(search.Ranges) != 1 || search.Ranges[0].BadKey != string(merge) {
		t.Fatalf("merge uncertainty or supported transition was lost: %#v", search)
	}
	hypotheses := root + "/" + investigation.ID + "/searches/" + search.ID + "/hypotheses"
	workflowJSON(t, server.URL, http.MethodPost, hypotheses, agent, regressionBody(t, ri.CausalHypothesis{RevisionKeys: []string{"parser-core@2.1.0"}, Body: "The dependency update introduced the regression.", EvidenceIDs: []string{flakyRun}, DiffPaths: []string{"go.mod"}, Confidence: .4, ActorKind: "agent", State: "rejected"}), http.StatusCreated, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, hypotheses, maintainer, regressionBody(t, ri.CausalHypothesis{RevisionKeys: []string{string(merge)}, Body: "Merge resolution bypassed legacy header normalization while both valid parents preserved it.", EvidenceIDs: []string{sideRun, badRun}, DiffPaths: []string{"parser/header.go"}, Confidence: .9, ActorKind: "human", State: "supported"}), http.StatusCreated, &investigation)

	options := []ri.ResponseOption{
		{ID: "revert", Kind: "revert", Title: "Revert merge", Summary: "Fast but removes strict syntax", Tradeoffs: []string{"reverts valid intent"}, AffectedReleases: []string{"v2.0"}, AffectedWork: []string{"main"}, BackportTargets: []string{"v1.9"}, EvidenceIDs: []string{badRun}},
		{ID: "contain", Kind: "containment", Title: "Pause rollout", Summary: "Contain affected production", Tradeoffs: []string{"temporarily pauses rollout"}, AffectedReleases: []string{"v2.0"}, AffectedWork: []string{"deployment:production"}, EvidenceIDs: []string{badRun}},
		{ID: "dependency", Kind: "dependency_adjustment", Title: "Pin dependency", Summary: "Rejected false cause", Tradeoffs: []string{"does not fix merge behavior"}, AffectedReleases: []string{"v2.0"}, AffectedWork: []string{"main"}, EvidenceIDs: []string{flakyRun}},
		{ID: "forward", Kind: "forward_repair", Title: "Restore normalization", Summary: "Preserve strict syntax and legacy input", Tradeoffs: []string{"requires two supported-line deliveries"}, AffectedReleases: []string{"v2.0"}, AffectedWork: []string{"main"}, BackportTargets: []string{"v1.9"}, EvidenceIDs: []string{badRun}},
	}
	responseInput := ri.ResponsePlanInput{SearchID: search.ID, CulpritGoodKey: string(side), CulpritBadKey: string(merge), ReproductionIDs: []string{badRun}, Constraints: []string{"retain strict syntax", "pause affected rollout"}, AcceptanceCriteria: []string{"legacy headers parse"}, OriginalIntent: "accept strict header syntax", OriginalAuthorIDs: []string{"original-author"}, Options: options, SelectedOptionID: "forward", Rationale: "Contain immediately, then repair without discarding valid syntax."}
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+investigation.ID+"/responses", agent, regressionBody(t, responseInput), http.StatusForbidden, nil)
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+investigation.ID+"/responses", maintainer, regressionBody(t, responseInput), http.StatusCreated, &investigation)
	response := investigation.Responses[0]
	workPath := root + "/" + investigation.ID + "/responses/" + response.ID + "/work"
	workflowJSON(t, server.URL, http.MethodPost, workPath, maintainer, regressionBody(t, ri.ResponseWork{Kind: "session", ResourceID: "session:repair", OwnerID: "agent", OwnerKind: "agent", OptionID: "forward", Published: true, PullRequestID: "pull:repair"}), http.StatusCreated, &investigation)
	response = investigation.Responses[0]
	work := response.Work[0]

	// Revoking the reporter's repository evidence access blocks future reads but
	// does not erase their report or attribution from the owner's durable trail.
	if err := repos.RemoveCollaborator("maintainer", repository.ID, "reporter"); err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, server.URL, http.MethodGet, root+"/"+investigation.ID, reporter, "", http.StatusNotFound, nil)
	workflowJSON(t, server.URL, http.MethodGet, root+"/"+investigation.ID, maintainer, "", http.StatusOK, &investigation)
	if investigation.Evidence[0].ActorID != "reporter" {
		t.Fatal("access revocation erased attributed user impact")
	}

	repairRun := addAttempt(repair, "expected_behavior", "Forward repair passes old and strict syntax.", 3, nil)
	backportRun := addAttempt(backport, "expected_behavior", "Supported-line backport passes preserved behavior.", 3, nil)
	checks := []ri.ProofCheck{{Name: "parser", Kind: "check", Status: "passed"}, {Name: "compatibility", Kind: "requirement", Status: "passed"}, {Name: "legacy headers parse", Kind: "regression_criterion", Status: "passed"}, {Name: "strict syntax remains accepted", Kind: "change_criterion", Status: "passed"}}
	createCorrection := func(kind string, target storage.ObjectID, run string) string {
		in := ri.CorrectionCandidateInput{ResponseID: response.ID, WorkID: work.ID, Kind: kind, Target: ri.Target{Kind: "revision", Reference: string(target)}, ScenarioID: scenario.ID, AffectedChecks: []string{"parser"}, RequirementIDs: []string{"compatibility"}, ChangeCriteria: []string{"strict syntax remains accepted"}, QualityPlanID: "quality:parser", RequiredCheckName: "legacy-header-regression"}
		workflowJSON(t, server.URL, http.MethodPost, root+"/"+investigation.ID+"/corrections", maintainer, regressionBody(t, in), http.StatusCreated, &investigation)
		candidate := investigation.Corrections[len(investigation.Corrections)-1]
		proof := ri.CorrectionProof{ScenarioAttemptID: run, BaselineAttemptIDs: []string{goodRun, badRun}, Checks: checks, Revision: string(target)}
		workflowJSON(t, server.URL, http.MethodPost, root+"/"+investigation.ID+"/corrections/"+candidate.ID+"/proofs", agent, regressionBody(t, proof), http.StatusCreated, &investigation)
		return candidate.ID
	}
	repairCandidate := createCorrection("repair", repair, repairRun)
	backportCandidate := createCorrection("backport", backport, backportRun)
	delivery := func(candidate, kind, resource, revision, status, summary string) {
		workflowJSON(t, server.URL, http.MethodPost, root+"/"+investigation.ID+"/corrections/"+candidate+"/delivery", maintainer, regressionBody(t, ri.DeliveryEvent{Kind: kind, ResourceID: resource, Revision: revision, Status: status, Summary: summary}), http.StatusCreated, &investigation)
	}
	// The revert fails review because it removes valid behavior; recovery stays
	// on the selected forward path and remains visible in the append-only trail.
	delivery(repairCandidate, "review", "review:revert", string(repair), "failed", "Revert failed preserved-intent review.")
	delivery(repairCandidate, "review", "review:repair", string(repair), "passed", "Distinct maintainer approved the forward repair.")
	for _, candidate := range []struct {
		id       string
		revision storage.ObjectID
		release  string
	}{{repairCandidate, repair, "release:v2.0.1"}, {backportCandidate, backport, "release:v1.9.4"}} {
		delivery(candidate.id, "review", "review:"+candidate.release, string(candidate.revision), "passed", "Distinct maintainer approved the exact supported-line candidate.")
		delivery(candidate.id, "merge", "merge:"+candidate.release, string(candidate.revision), "passed", "Ordinary protected merge completed.")
		delivery(candidate.id, "release", candidate.release, string(candidate.revision), "passed", "Attested supported-line release published.")
		delivery(candidate.id, "deployment", "deployment:"+candidate.release, string(candidate.revision), "passed", "Governed rollout resumed after containment.")
		delivery(candidate.id, "outcome", "signal:"+candidate.release, candidate.release, "passed", "Supported environment matches historical proof.")
	}
	// A later production disagreement reopens the exact repair, then a corrected
	// rollout observation recovers it without deleting the failed observation.
	delivery(repairCandidate, "outcome", "signal:production-mismatch", "release:v2.0.1", "disagreed", "Production image did not match the verified artifact.")
	if investigation.Status != "open" {
		t.Fatal("production mismatch did not reopen the investigation")
	}
	delivery(repairCandidate, "outcome", "signal:production-corrected", "release:v2.0.1", "passed", "Corrected deployment now matches the verified artifact and preserved intent.")
	workflowJSON(t, server.URL, http.MethodPost, root+"/"+investigation.ID+"/status", maintainer, `{"status":"closed","reason":"forward repair and backport are released, observed, and retained as required regression coverage"}`, http.StatusOK, &investigation)
	if investigation.Status != "closed" || len(investigation.Corrections) != 2 || investigation.Corrections[0].State != "observed" || investigation.Corrections[1].State != "observed" || investigation.Corrections[0].RequiredCheckName != "legacy-header-regression" {
		t.Fatalf("sustained repair/backport outcome is incomplete: %#v", investigation.Corrections)
	}
	if len(investigation.Corrections[0].Delivery) < 7 || investigation.Responses[0].Work[0].CreatedByID != "maintainer" || investigation.Searches[0].Hypotheses[0].State != "rejected" {
		t.Fatalf("recovery erased reasoning, ownership, or failed delivery: %#v", investigation)
	}
}
