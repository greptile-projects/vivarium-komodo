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

func TestPropagationCampaignPublicBoundary(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	campaigns, _ := propagationcampaigns.New(t.TempDir())
	repository, _ := repos.Create("owner", repositories.Metadata{Name: "source", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repository.ID, "collaborator")
	opened, _ := repos.Open(repository.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor A <a@x> 1 +0000\ncommitter A <a@x> 1 +0000\n\nrepair\n", tree)))
	token := issueAccess(t, credentials, "collaborator", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPropagationCampaignsHTTP(mux, campaigns, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repository.ID) + "/propagation-campaigns"
	due := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	observed := time.Now().UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"title":"Propagate parser repair","intent":"Keep behavior equivalent","acceptance_criteria":["legacy input works"],"source":{"kind":"regression_correction","repository_id":%q,"resource_id":"correction-1","revision":%q,"commit_ids":[%q]},"targets":[{"id":"stable","repository_id":%q,"release_line":"v2","deadline":%q,"disposition":"pending","authority":{"owner_ids":["owner"],"access":"requested","basis":"target owner remains authoritative","observed_at":%q}},{"id":"peer","repository_reference":"https://peer.example/lib","release_line":"v1","deadline":%q,"depends_on":["stable"],"disposition":"inaccessible","disposition_reason":"peer unavailable","authority":{"access":"unknown","basis":"federated reference only","observed_at":%q}}],"completion_policy":{"mode":"all_supported","exception_requires_owner":true}}`, repository.ID, commit, commit, repository.ID, due, observed, due, observed)
	var campaign propagationcampaigns.Campaign
	workflowJSON(t, server.URL, http.MethodPost, base, token, body, 201, &campaign)
	if len(campaign.Blockers) != 1 || campaign.Blockers[0].Kind != "inaccessible" {
		t.Fatalf("explicit target lost: %#v", campaign)
	}
	workflowJSON(t, server.URL, http.MethodGet, base+"/"+campaign.ID, token, "", 200, &campaign)
	comparisons := make([]propagationcampaigns.Comparison, 0, 7)
	for _, kind := range []string{"history", "symbols", "dependencies", "interfaces", "schemas", "prior_fixes", "release_commitments"} {
		comparisons = append(comparisons, propagationcampaigns.Comparison{Kind: kind, SourceSummary: "source", TargetSummary: "target", Conclusion: "different", Citations: []propagationcampaigns.Citation{{Kind: kind, Reference: "evidence:" + kind, Revision: "target-1"}}})
	}
	assessmentBody, _ := json.Marshal(propagationcampaigns.AssessmentInput{TargetRevision: "target-1", SourceRevision: string(commit), Classification: "adaptation_required", Rationale: "The stable line has a different parser interface.", Comparisons: comparisons, Risks: []string{"release compatibility"}, Uncertainty: "Runtime behavior remains to be tested."})
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+campaign.ID+"/targets/stable/assessments", token, string(assessmentBody), 201, &campaign)
	assessmentID := campaign.Assessments[0].ID
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+campaign.ID+"/targets/stable/assessments/"+assessmentID+"/findings", token, `{"actor_kind":"read_only_agent","summary":"The release promise excludes the old schema.","uncertainty":"Peer package is unavailable.","citations":[{"kind":"release_commitments","reference":"release:v2","revision":"target-1"}]}`, 201, &campaign)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+campaign.ID+"/targets/stable/assessments/"+assessmentID+"/acknowledgements", token, `{"decision":"acknowledged","rationale":"reviewed"}`, 403, nil)
	ownerToken := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+campaign.ID+"/targets/stable/assessments/"+assessmentID+"/acknowledgements", ownerToken, `{"decision":"changes_requested","rationale":"Cite the supported schema guarantee."}`, 201, &campaign)
	if len(campaign.Assessments[0].Findings) != 1 || len(campaign.Assessments[0].Acknowledgements) != 1 {
		t.Fatalf("assessment collaboration lost: %#v", campaign.Assessments[0])
	}
	contribution := propagationcampaigns.ContributionInput{AssessmentID: assessmentID, Mode: "adapted", Rationale: "Use the stable parser boundary.", SourceAuthorIDs: []string{"repair-author"}, RelevantCommitIDs: []string{string(commit)}, Constraints: []string{"stable schema remains unchanged"}, AcceptanceCriteria: []string{"legacy input works"}, Deviations: []string{"replace the current-line hook with the stable adapter"}, ContextReferences: []string{"assessment:" + assessmentID}, Tasks: []propagationcampaigns.ContributionTask{{ID: "repair", Title: "Adapt the repair", OwnerKind: "agent", OwnerID: "stable-agent", Scope: []string{"parser and tests"}, AcceptanceCriteria: []string{"legacy input works"}, TaskID: "task:stable", SessionID: "session:stable", WorkspaceID: "workspace:stable"}, {ID: "review", Title: "Review stable behavior", OwnerKind: "human", OwnerID: "owner", DependsOn: []string{"repair"}, Scope: []string{"review only"}, AcceptanceCriteria: []string{"intent retained"}, PullRequestID: "pull:stable"}}}
	contributionBody, _ := json.Marshal(contribution)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+campaign.ID+"/targets/stable/contributions", token, string(contributionBody), 201, &campaign)
	if len(campaign.Contributions) != 1 || campaign.Contributions[0].SourceIntent != campaign.Intent || len(campaign.Contributions[0].AuthorityGranted) != 0 {
		t.Fatalf("contribution provenance or authority boundary lost: %#v", campaign.Contributions)
	}
	spec := propagationcampaigns.EquivalenceSpecificationInput{SourceRevision: string(commit), Environment: "networkless", MaximumCost: 2, Currency: "USD", TimeoutSeconds: 60, Scenarios: []propagationcampaigns.EquivalenceScenario{{ID: "legacy", Behavior: "legacy input works", SourceEvidence: []string{"correction-1"}, Commands: []string{"go test ./..."}, RequiredCoverage: []string{"legacy"}, OrdinaryCheckNames: []string{"unit"}}}}
	specBody, _ := json.Marshal(spec)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+campaign.ID+"/equivalence-specifications", token, string(specBody), 201, &campaign)
	attempt := propagationcampaigns.EquivalenceAttemptInput{SpecificationID: campaign.EquivalenceSpecifications[0].ID, AssessmentID: assessmentID, ContributionID: campaign.Contributions[0].ID, SourceRevision: string(commit), TargetRevision: "target-1", AdaptationRevision: "target-adaptation-1", Environment: "networkless", BoundInputs: []propagationcampaigns.BoundInput{{Key: "source", Revision: string(commit)}, {Key: "target", Revision: "target-1"}, {Key: "dependency:parser", Revision: "v2"}}, Evidence: []propagationcampaigns.ScenarioEvidence{{ScenarioID: "legacy", Status: "passed", Commands: []string{"go test ./..."}, OrdinaryChecks: []string{"unit"}, Logs: []string{"PASS"}, Artifacts: []propagationcampaigns.Artifact{{Name: "junit", Digest: "sha256:abc", MediaType: "application/xml", Size: 10}}, Coverage: []string{"legacy"}, ResidualDifference: "stable adapter differs internally"}}, Cost: 1, Currency: "USD", DurationSeconds: 12}
	attemptBody, _ := json.Marshal(attempt)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+campaign.ID+"/targets/stable/equivalence-attempts", token, string(attemptBody), 201, &campaign)
	if len(campaign.EquivalenceAttempts) != 1 || !campaign.EquivalenceAttempts[0].Passing || campaign.EquivalenceAttempts[0].Evidence[0].ResidualDifference == "" {
		t.Fatalf("equivalence matrix lost: %#v", campaign.EquivalenceAttempts)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+campaign.ID+"/targets/stable/equivalence-attempts/"+campaign.EquivalenceAttempts[0].ID+"/decisions", ownerToken, `{"decision":"accepted","rationale":"behavioral proof reviewed"}`, 201, &campaign)
	delivery := base + "/" + campaign.ID + "/targets/stable/delivery-events"
	workflowJSON(t, server.URL, http.MethodPost, delivery, token, `{"kind":"review","status":"succeeded","resource_reference":"pull:stable","revision":"target-adaptation-1","summary":"owner review passed"}`, 403, nil)
	for _, event := range []string{
		`{"kind":"review","status":"succeeded","resource_reference":"pull:stable","revision":"target-adaptation-1","summary":"ordinary review passed"}`,
		`{"kind":"queue","status":"succeeded","resource_reference":"queue:stable","revision":"target-adaptation-1","summary":"required checks admitted the pull"}`,
		`{"kind":"merge","status":"succeeded","resource_reference":"pull:stable","revision":"merge-stable-1","summary":"target owner merged"}`,
		`{"kind":"release","status":"succeeded","resource_reference":"release:v2.4.1","revision":"merge-stable-1","summary":"attested stable release published"}`,
		`{"kind":"deploy","status":"failed","resource_reference":"deployment:stable-1","revision":"merge-stable-1","summary":"canary health failed","supported_users":100,"reached_users":10,"exposure_unit":"maintained installations"}`,
	} {
		workflowJSON(t, server.URL, http.MethodPost, delivery, ownerToken, event, 201, &campaign)
	}
	if !campaign.Coverage.Targets[0].Paused || campaign.Coverage.ReachedUsers != 10 || campaign.Coverage.Complete {
		t.Fatalf("failed target was not selectively paused: %#v", campaign.Coverage)
	}
	workflowJSON(t, server.URL, http.MethodPost, delivery, ownerToken, `{"kind":"deploy","status":"succeeded","resource_reference":"deployment:stable-2","revision":"merge-stable-1","summary":"replacement canary healthy","supported_users":100,"reached_users":100,"exposure_unit":"maintained installations"}`, 201, &campaign)
	workflowJSON(t, server.URL, http.MethodPost, delivery, ownerToken, `{"kind":"outcome","status":"succeeded","resource_reference":"observation:stable-2","revision":"merge-stable-1","summary":"required behavior observed","supported_users":100,"reached_users":100,"exposure_unit":"maintained installations","outcome":"legacy input succeeds without regression"}`, 201, &campaign)
	if campaign.Coverage.ReachedUsers != 100 || campaign.Coverage.Targets[0].ObservedOutcome == "" {
		t.Fatalf("delivered exposure was not projected: %#v", campaign.Coverage)
	}
	workflowJSON(t, server.URL, http.MethodPost, delivery, ownerToken, `{"kind":"consumer_discovered","status":"succeeded","resource_reference":"inventory:runtime","revision":"merge-stable-1","summary":"runtime inventory found another supported consumer","consumer_reference":"consumer:hidden"}`, 201, &campaign)
	if campaign.Coverage.Complete || campaign.Coverage.Targets[0].NextActions[0] != "add and assess the newly discovered consumer" {
		t.Fatalf("new consumer was hidden by delivery: %#v", campaign.Coverage)
	}
	bad := fmt.Sprintf(`{"title":"bad","intent":"bad","acceptance_criteria":["x"],"source":{"kind":"policy_change","repository_id":%q,"resource_id":"p","revision":"missing","commit_ids":["missing"]},"targets":[]}`, repository.ID)
	workflowJSON(t, server.URL, http.MethodPost, base, token, bad, 422, nil)
}
