package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/propagationcampaigns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestPropagationSourceChangeToEcosystemCoverage is the black-box boundary for
// one proven repair reaching maintained branches and independently owned
// consumers. It crosses the public HTTP surface used by view=propagation and
// retains stock Git and federated contribution references without transferring
// any target's review, merge, release, or deployment authority to the campaign.
func TestPropagationSourceChangeToEcosystemCoverage(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	store, _ := propagationcampaigns.New(t.TempDir())
	sourceRepo, _ := repos.Create("coordinator", repositories.Metadata{Name: "secure-parser", Visibility: repositories.Public})
	consumerRepo, _ := repos.Create("consumer-owner", repositories.Metadata{Name: "independent-consumer", Visibility: repositories.Public})

	sourceBase := propagationCommit(t, repos, sourceRepo.ID, "Original Author", "parser before security repair", "")
	sourceRepair := propagationCommit(t, repos, sourceRepo.ID, "Security Maintainer", "reject ambiguous headers", string(sourceBase))
	consumerBase := propagationCommit(t, repos, consumerRepo.ID, "Consumer Owner", "independent parser implementation", "")
	consumerAdaptation := propagationCommit(t, repos, consumerRepo.ID, "Consumer Contributor", "adapt ambiguous-header defense", string(consumerBase))
	if sourceRepair == consumerAdaptation {
		t.Fatal("stock Git histories unexpectedly collapsed across repositories")
	}

	actors := []string{"analyst", "agent", "stable-owner", "consumer-owner", "divergent-owner", "failed-owner", "upstream-owner", "inaccessible-owner"}
	for _, actor := range actors {
		if _, err := repos.AddCollaborator("coordinator", sourceRepo.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	tokens := map[string]string{}
	for _, actor := range append([]string{"coordinator"}, actors...) {
		tokens[actor] = issueAccess(t, credentials, actor, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	}
	mux := http.NewServeMux()
	registerPropagationCampaignsHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	root := "/repositories/" + string(sourceRepo.ID) + "/propagation-campaigns"
	now, due := time.Now().UTC(), time.Now().UTC().Add(14*24*time.Hour)
	target := func(id, owner, disposition, reason string) propagationcampaigns.Target {
		return propagationcampaigns.Target{ID: id, RepositoryID: string(sourceRepo.ID), ReleaseLine: id, OwnerIDs: []string{owner}, Deadline: due, Disposition: disposition, DispositionReason: reason, Authority: propagationcampaigns.Authority{OwnerIDs: []string{owner}, Access: "requested", Basis: "named target owner; campaign grants no authority", ObservedAt: now}}
	}
	targets := []propagationcampaigns.Target{
		target("current", "stable-owner", "pending", ""),
		target("stable", "stable-owner", "pending", ""),
		target("divergent", "divergent-owner", "pending", ""),
		target("failed-adaptation", "failed-owner", "pending", ""),
		target("upstream", "upstream-owner", "pending", ""),
		target("already-fixed", "consumer-owner", "already_equivalent", "independent proof predates the campaign"),
		target("inaccessible", "inaccessible-owner", "inaccessible", "federated repository cannot currently be read"),
	}
	targets[2].DependsOn = []string{"stable"}
	targets[4].RepositoryID, targets[4].RepositoryReference = "", "https://forge.example/upstream/parser"
	targets[5].RepositoryID, targets[5].RepositoryReference = string(consumerRepo.ID), "https://peer.example/consumer"
	targets[6].RepositoryID, targets[6].RepositoryReference = "", "https://offline.example/parser"
	in := propagationcampaigns.Input{Title: "Propagate ambiguous-header repair", Intent: "reject ambiguous headers without rejecting valid legacy headers", AcceptanceCriteria: []string{"ambiguous headers rejected", "valid legacy headers accepted"}, Source: propagationcampaigns.Source{Kind: "security_repair", RepositoryID: string(sourceRepo.ID), ResourceID: "security-repair:42", Revision: string(sourceRepair), CommitIDs: []string{string(sourceRepair)}, EvidenceReferences: []string{"scenario:ambiguous-header", "review:security-42"}}, Targets: targets, CompletionPolicy: propagationcampaigns.CompletionPolicy{Mode: "all_supported", AllowEquivalent: true, ExceptionRequiresOwner: true}}
	var campaign propagationcampaigns.Campaign
	propagationRequest(t, server.URL, http.MethodPost, root, tokens["coordinator"], in, http.StatusCreated, &campaign)
	if len(campaign.Blockers) != 1 || campaign.Blockers[0].Kind != "inaccessible" {
		t.Fatalf("inaccessible consumer was not retained: %#v", campaign.Blockers)
	}

	comparisons := func(revision string, proof bool) []propagationcampaigns.Comparison {
		out := []propagationcampaigns.Comparison{}
		for _, kind := range []string{"history", "symbols", "dependencies", "interfaces", "schemas", "prior_fixes", "release_commitments"} {
			out = append(out, propagationcampaigns.Comparison{Kind: kind, SourceSummary: "security repair at " + string(sourceRepair), TargetSummary: "target evidence at " + revision, Conclusion: "different", BehavioralProof: proof && kind == "prior_fixes", Citations: []propagationcampaigns.Citation{{Kind: kind, Reference: "evidence:" + kind, Revision: revision}}})
		}
		return out
	}
	assess := func(id, revision, class string) string {
		t.Helper()
		body := propagationcampaigns.AssessmentInput{TargetRevision: revision, SourceRevision: string(sourceRepair), Classification: class, Rationale: "revision-exact comparison for " + id, Comparisons: comparisons(revision, class == "already_satisfied"), Risks: []string{"supported users require equivalent behavior"}, AssumptionsStillHold: true}
		propagationRequest(t, server.URL, http.MethodPost, root+"/"+campaign.ID+"/targets/"+id+"/assessments", tokens["analyst"], body, http.StatusCreated, &campaign)
		return campaign.Assessments[len(campaign.Assessments)-1].ID
	}
	currentAssessment := assess("current", string(sourceBase), "directly_applicable")
	staleStable := assess("stable", "stable-base-1", "adaptation_required")
	stableAssessment := assess("stable", "stable-base-2", "adaptation_required")
	divergentAssessment := assess("divergent", "divergent-base", "conflicting")
	failedAssessment := assess("failed-adaptation", "failed-base", "adaptation_required")
	upstreamAssessment := assess("upstream", string(consumerBase), "adaptation_required")
	if !campaign.Assessments[len(campaign.Assessments)-5].Stale || staleStable == stableAssessment {
		t.Fatal("new target revision did not selectively stale the old stable assessment")
	}

	contribute := func(id, assessment, mode, ownerKind, owner, pull, federation string) string {
		t.Helper()
		deviations := []string(nil)
		if mode == "adapted" {
			deviations = []string{"use the target parser boundary while preserving source behavior"}
		}
		body := propagationcampaigns.ContributionInput{AssessmentID: assessment, Mode: mode, Rationale: "locally governed contribution", SourceAuthorIDs: []string{"security-maintainer"}, RelevantCommitIDs: []string{string(sourceRepair)}, Constraints: []string{"retain local API"}, AcceptanceCriteria: in.AcceptanceCriteria, Deviations: deviations, ContextReferences: []string{"assessment:" + assessment}, Tasks: []propagationcampaigns.ContributionTask{{ID: "implement", Title: "Apply proven behavior", OwnerKind: ownerKind, OwnerID: owner, Scope: []string{"parser and tests"}, AcceptanceCriteria: in.AcceptanceCriteria, PullRequestID: pull, FederatedPullRef: federation}}}
		propagationRequest(t, server.URL, http.MethodPost, root+"/"+campaign.ID+"/targets/"+id+"/contributions", tokens["coordinator"], body, http.StatusCreated, &campaign)
		return campaign.Contributions[len(campaign.Contributions)-1].ID
	}
	contributions := map[string]string{
		"current":           contribute("current", currentAssessment, "direct", "human", "security-maintainer", "pull:current", ""),
		"stable":            contribute("stable", stableAssessment, "adapted", "agent", "agent:stable", "pull:stable", ""),
		"divergent":         contribute("divergent", divergentAssessment, "adapted", "human", "divergent-owner", "pull:divergent", ""),
		"failed-adaptation": contribute("failed-adaptation", failedAssessment, "adapted", "agent", "agent:repair", "pull:failed", ""),
		"upstream":          contribute("upstream", upstreamAssessment, "adapted", "human", "consumer-contributor", "", "federated-pull:peer/17"),
	}

	spec := propagationcampaigns.EquivalenceSpecificationInput{SourceRevision: string(sourceRepair), Environment: "networkless-go-1.25", MaximumCost: 8, Currency: "USD", TimeoutSeconds: 300, Scenarios: []propagationcampaigns.EquivalenceScenario{{ID: "ambiguous", Behavior: "ambiguous headers are rejected", SourceEvidence: []string{"scenario:ambiguous-header"}, Commands: []string{"go test ./... -run AmbiguousHeader"}, RequiredCoverage: []string{"ambiguous-rejection"}, OrdinaryCheckNames: []string{"security"}}, {ID: "legacy", Behavior: "valid legacy headers remain accepted", SourceEvidence: []string{"regression:legacy-header"}, Commands: []string{"go test ./... -run LegacyHeader"}, RequiredCoverage: []string{"legacy-compatibility"}, OrdinaryCheckNames: []string{"regression"}}}}
	propagationRequest(t, server.URL, http.MethodPost, root+"/"+campaign.ID+"/equivalence-specifications", tokens["coordinator"], spec, http.StatusCreated, &campaign)
	specID := campaign.EquivalenceSpecifications[0].ID
	assessmentIDs := map[string]string{"current": currentAssessment, "stable": stableAssessment, "divergent": divergentAssessment, "failed-adaptation": failedAssessment, "upstream": upstreamAssessment}
	attempt := func(id, revision, status, residual string) string {
		t.Helper()
		evidence := []propagationcampaigns.ScenarioEvidence{}
		for _, scenario := range spec.Scenarios {
			evidence = append(evidence, propagationcampaigns.ScenarioEvidence{ScenarioID: scenario.ID, Status: status, Commands: scenario.Commands, OrdinaryChecks: scenario.OrdinaryCheckNames, Logs: []string{"sanitized test result: " + status}, Artifacts: []propagationcampaigns.Artifact{{Name: scenario.ID + "-junit", Digest: "sha256:" + id + scenario.ID, MediaType: "application/xml", Size: 42}}, Coverage: scenario.RequiredCoverage, ResidualDifference: residual})
		}
		body := propagationcampaigns.EquivalenceAttemptInput{SpecificationID: specID, AssessmentID: assessmentIDs[id], ContributionID: contributions[id], SourceRevision: string(sourceRepair), TargetRevision: revision, AdaptationRevision: "adaptation:" + revision, Environment: spec.Environment, BoundInputs: []propagationcampaigns.BoundInput{{Key: "source", Revision: string(sourceRepair)}, {Key: "target", Revision: revision}, {Key: "dependency:parser", Revision: "locked-1"}}, Evidence: evidence, Cost: 2.5, Currency: "USD", DurationSeconds: 40}
		propagationRequest(t, server.URL, http.MethodPost, root+"/"+campaign.ID+"/targets/"+id+"/equivalence-attempts", tokens["agent"], body, http.StatusCreated, &campaign)
		return campaign.EquivalenceAttempts[len(campaign.EquivalenceAttempts)-1].ID
	}
	owners := map[string]string{"current": "stable-owner", "stable": "stable-owner", "divergent": "divergent-owner", "failed-adaptation": "failed-owner", "upstream": "upstream-owner"}
	revisions := map[string]string{"current": string(sourceBase), "stable": "stable-base-2", "divergent": "divergent-base", "failed-adaptation": "failed-base", "upstream": string(consumerBase)}
	failedAttempt := attempt("failed-adaptation", revisions["failed-adaptation"], "failed", "first adaptation weakened legacy compatibility")
	propagationRequest(t, server.URL, http.MethodPost, root+"/"+campaign.ID+"/targets/failed-adaptation/equivalence-attempts/"+failedAttempt+"/decisions", tokens["failed-owner"], map[string]string{"decision": "rejected", "rationale": "failed behavior cannot ship"}, http.StatusCreated, &campaign)
	for _, id := range []string{"current", "stable", "divergent", "failed-adaptation", "upstream"} {
		residual := ""
		if id == "divergent" || id == "upstream" {
			residual = "implementation differs; externally observable behavior matches"
		}
		attemptID := attempt(id, revisions[id], "passed", residual)
		propagationRequest(t, server.URL, http.MethodPost, root+"/"+campaign.ID+"/targets/"+id+"/equivalence-attempts/"+attemptID+"/decisions", tokens[owners[id]], map[string]string{"decision": "accepted", "rationale": "local checks and equivalent-behavior evidence passed"}, http.StatusCreated, &campaign)
	}

	deliver := func(id, owner, kind, status, resource, summary string, users int64) {
		t.Helper()
		body := propagationcampaigns.DeliveryEventInput{Kind: kind, Status: status, ResourceReference: resource, Revision: "delivery:" + id, Summary: summary}
		if users > 0 {
			body.SupportedUsers, body.ReachedUsers, body.ExposureUnit = users, users, "supported installations"
		}
		if kind == "outcome" {
			body.Outcome = "equivalent protected behavior observed"
		}
		propagationRequest(t, server.URL, http.MethodPost, root+"/"+campaign.ID+"/targets/"+id+"/delivery-events", tokens[owner], body, http.StatusCreated, &campaign)
	}
	for _, id := range []string{"current", "stable", "divergent", "failed-adaptation", "upstream"} {
		for _, kind := range []string{"review", "queue", "merge"} {
			deliver(id, owners[id], kind, "succeeded", kind+":"+id, "ordinary local governance passed", 0)
		}
		if id == "upstream" {
			deliver(id, owners[id], "release", "rejected", "federated-pull:peer/17", "upstream maintainer rejected the proposed release", 0)
			continue
		}
		if id == "stable" {
			deliver(id, owners[id], "release", "failed", "release:stable-1", "package publication failed", 0)
		}
		deliver(id, owners[id], "release", "succeeded", "release:"+id, "attested target release published", 0)
		deliver(id, owners[id], "deploy", "succeeded", "deployment:"+id, "governed rollout completed", 25)
		deliver(id, owners[id], "outcome", "succeeded", "observation:"+id, "supported users exhibit the repaired behavior", 25)
	}
	exception := func(id, owner, reason string) {
		body := propagationcampaigns.DeliveryEventInput{Kind: "exception", Status: "succeeded", ResourceReference: "exception:" + id, Revision: "decision:" + id, Summary: "owner bounded unresolved target", ExceptionReason: reason, ExceptionExpires: time.Now().UTC().Add(7 * 24 * time.Hour)}
		propagationRequest(t, server.URL, http.MethodPost, root+"/"+campaign.ID+"/targets/"+id+"/delivery-events", tokens[owner], body, http.StatusCreated, &campaign)
	}
	exception("upstream", "upstream-owner", "maintain safe local fallback while upstream reconsiders")
	exception("inaccessible", "inaccessible-owner", "unsupported users are isolated pending restored federation access")
	propagationRequest(t, server.URL, http.MethodGet, root+"/"+campaign.ID, tokens["coordinator"], nil, http.StatusOK, &campaign)
	if !campaign.Coverage.Complete || campaign.Coverage.SupportedUsers != 100 || campaign.Coverage.ReachedUsers != 100 {
		t.Fatalf("campaign did not prove contained ecosystem coverage: %#v", campaign.Coverage)
	}
	if len(campaign.Assessments) != 6 || !campaign.Assessments[1].Stale || len(campaign.EquivalenceAttempts) != 6 || !campaign.EquivalenceAttempts[0].Stale || campaign.Contributions[4].Tasks[0].FederatedPullRef == "" {
		t.Fatalf("retained collaboration trail is incomplete: assessments=%#v attempts=%#v contributions=%#v", campaign.Assessments, campaign.EquivalenceAttempts, campaign.Contributions)
	}
	if campaign.Coverage.Targets[4].State != "excepted" || campaign.Coverage.Targets[6].State != "excepted" {
		t.Fatalf("rejection or inaccessible containment was lost: %#v", campaign.Coverage.Targets)
	}
}

func propagationCommit(t *testing.T, repos *repositories.Store, id storage.ID, author, message, parent string) storage.ObjectID {
	t.Helper()
	repo, err := repos.Open(id)
	if err != nil {
		t.Fatal(err)
	}
	tree, _ := repo.WriteObject(storage.TreeObject, nil)
	body := "tree " + string(tree) + "\n"
	if parent != "" {
		body += "parent " + parent + "\n"
	}
	body += fmt.Sprintf("author %s <author@example.test> 1 +0000\ncommitter %s <author@example.test> 1 +0000\n\n%s\n", author, author, message)
	commit, err := repo.WriteObject(storage.CommitObject, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return commit
}

func propagationRequest(t *testing.T, server, method, path, token string, body any, status int, out any) {
	t.Helper()
	payload := ""
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload = string(encoded)
	}
	workflowJSON(t, server, method, path, token, payload, status, out)
}
