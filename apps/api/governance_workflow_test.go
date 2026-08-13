package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/governance"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestCommunityGovernanceWorkflow is the black-box boundary for the complete
// charter-to-delivery-to-renewed-stewardship loop. Governance uses public HTTP,
// delivery uses stock Git and ordinary pull/check/release policy, and no
// governance record is treated as a repository credential or merge grant.
func TestCommunityGovernanceWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is required for the governance workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	orgs, _ := organizations.New(t.TempDir())
	governanceStore, _ := governance.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	runner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, catalog, credentials, nil, runner, checks)
	registerCheckRunsHTTP(mux, checks, runner, pulls, catalog, credentials, nil, nil)
	registerReleasesHTTP(mux, releaseStore, checks, runner, pulls, catalog, credentials)
	registerGovernanceHTTP(mux, governanceStore, catalog, orgs, credentials)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "owner", auth.Git, auth.GitRead, auth.GitWrite)
	contributor := issueAccess(t, credentials, "contributor", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	contributorGit := issueAccess(t, credentials, "contributor", auth.Git, auth.GitRead, auth.GitWrite)
	successor := issueAccess(t, credentials, "successor", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agentGit := issueAccess(t, credentials, "delivery-agent", auth.Git, auth.GitRead, auth.GitWrite)
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"community-runtime","visibility":"private"}`, http.StatusCreated, &repository)
	for _, actor := range []string{"contributor", "successor", "delivery-agent"} {
		if _, err := catalog.AddCollaborator("owner", repository.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	baseline := gitClone(t, remote(ownerGit))
	gitOutput(t, baseline, "config", "user.name", "Repository owner")
	gitOutput(t, baseline, "config", "user.email", "owner@example.com")
	writeWorkflowFile(t, baseline, "README.md", "# Community runtime\n")
	writeWorkflowFile(t, baseline, ".komodo/checks.json", `{"version":1,"checks":[{"name":"community-policy","command":"grep -q 'human verified' DELIVERY.md"}]}`)
	writeWorkflowFile(t, baseline, ".komodo/releases.json", `{"version":1,"builds":[{"name":"community-record","command":"mkdir -p dist; cp DELIVERY.md dist/DELIVERY.md","artifacts":["dist/DELIVERY.md"]}]}`)
	gitOutput(t, baseline, "add", ".")
	gitOutput(t, baseline, "commit", "-m", "Establish community project")
	baseRevision := gitOutput(t, baseline, "rev-parse", "HEAD")
	gitOutput(t, baseline, "push", "-u", "origin", "main")

	governanceBase := "/repositories/" + string(repository.ID) + "/governance-charter"
	charterBody := `{"expected_version":0,"title":"Community stewardship charter","purpose":"Let proven contributors decide transparently while repository controls remain separate.","roles":[{"name":"maintainer","purpose":"Steward community decisions","eligibility":["proven contribution or review"],"responsibilities":["deliberate with evidence","preserve dissent","renew stewardship"],"minimum_members":2,"term_days":365}],"decision_classes":[{"name":"community-initiative","description":"Initiatives and leadership transitions","eligible_roles":["maintainer"],"participation":"attributable ballots","quorum":2,"threshold":50,"protected_resources":["branches:main","releases"]}],"participation_rules":["agents may analyze but never vote","recusals leave an attributable record"],"protected_resources":["branches:main","releases"],"procedures":{"removal":"owner action after governed recall","succession":"election followed by separate resource handoff","vacancy":"time-bounded emergency recovery"},"amendment_policy":{"eligible_roles":["maintainer"],"notice_days":1,"quorum":2,"threshold":67},"change_reason":"Adopt community governance"}`
	var charter governance.Charter
	workflowJSON(t, server.URL, http.MethodPost, governanceBase, owner, charterBody, http.StatusCreated, &charter)
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/approvals", owner, `{"version":1,"note":"Repository policy remains authoritative"}`, http.StatusCreated, &charter)
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/activation", owner, `{"version":1}`, http.StatusOK, &charter)
	invite := func(principal, kind, reference string) string {
		body := `{"version":1,"principal_id":"` + principal + `","role":"maintainer","evidence":[{"kind":"` + kind + `","reference":"` + reference + `","summary":"Verified project work"}],"available_nominations":["maintainer"],"available_appeals":["stewardship"]}`
		workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/standings", owner, body, http.StatusCreated, &charter)
		return charter.Standings[len(charter.Standings)-1].ID
	}
	ownerStanding := invite("owner", "ownership", "repository:"+string(repository.ID))
	contributorStanding := invite("contributor", "contribution", "commit:"+baseRevision)
	successorStanding := invite("successor", "review", "commit:"+baseRevision)
	for _, x := range []struct{ id, actor, token string }{{ownerStanding, "owner", owner}, {contributorStanding, "contributor", contributor}, {successorStanding, "successor", successor}} {
		workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/standings/"+x.id+"/accept", x.token, `{"reason":"Accept charter responsibilities"}`, http.StatusOK, &charter)
	}
	if len(charter.Standings[1].OperationalAuthority) != 0 {
		t.Fatal("standing unexpectedly granted repository authority")
	}

	// A recusal removes a ballot from the live electorate and produces an
	// explicit failed-quorum result without changing repository availability.
	var failed governance.GovernedProposal
	open := func(title, kind string) governance.GovernedProposal {
		body := `{"kind":"` + kind + `","title":"` + title + `","summary":"Evidence-backed community change","scope":"repository delivery","decision_class":"community-initiative","alternatives":[{"id":"adopt","title":"Adopt","description":"Proceed under ordinary delivery policy","implementation_effects":["reviewed repository change"]},{"id":"decline","title":"Decline","description":"Retain current behavior"}],"evidence":[{"kind":"code","reference":"commit:` + baseRevision + `","summary":"Exact reviewed baseline"}],"affected_resources":["branches:main","releases"],"disclosure_requirements":["conflicts"],"implementation_effects":["human and agent delivery"],"discussion_hours":1}`
		var p governance.GovernedProposal
		workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/proposals", contributor, body, http.StatusCreated, &p)
		return p
	}
	failed = open("Rejected quorum rehearsal", "initiative")
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/proposals/"+failed.ID+"/ballots", owner, `{"choice":"adopt","reason":"Proceed"}`, http.StatusCreated, &failed)
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/standings/"+contributorStanding+"/recuse", contributor, `{"reason":"Conflict disclosed"}`, http.StatusOK, &charter)
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/proposals/"+failed.ID+"/tally", owner, `{"close_early":true}`, http.StatusOK, &failed)
	if failed.Tally.QuorumMet || failed.State != "rejected" {
		t.Fatalf("failed quorum was not retained: %#v", failed.Tally)
	}
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/standings/"+contributorStanding+"/resume", contributor, `{"reason":"Conflict ended"}`, http.StatusOK, &charter)

	initiative := open("Deliver auditable retry guidance", "initiative")
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/proposals/"+initiative.ID+"/discussion", successor, `{"body":"Prefer adoption only with human verification.","citations":[{"kind":"code","reference":"commit:`+baseRevision+`","summary":"Baseline lacks delivery guidance"}]}`, http.StatusCreated, &initiative)
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/proposals/"+initiative.ID+"/ballots", contributor, `{"choice":"adopt","reason":"Measured need"}`, http.StatusCreated, &initiative)
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/proposals/"+initiative.ID+"/ballots", owner, `{"choice":"adopt","reason":"Bounded by review"}`, http.StatusCreated, &initiative)
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/proposals/"+initiative.ID+"/ballots", successor, `{"choice":"decline","reason":"Dissent: rollout evidence is incomplete"}`, http.StatusCreated, &initiative)
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/proposals/"+initiative.ID+"/tally", owner, `{"close_early":true}`, http.StatusOK, &initiative)
	if initiative.State != "approved" || initiative.Tally.Counts["decline"] != 1 || initiative.DecisionReceipt.AuthorityGranted {
		t.Fatalf("approved initiative lost dissent or authority boundary: %#v", initiative)
	}

	// Agent and human work use pre-existing collaborator grants. The final pull,
	// check, owner review, merge, and release are ordinary repository resources.
	agentWork := gitClone(t, remote(agentGit))
	gitOutput(t, agentWork, "config", "user.name", "Delivery agent")
	gitOutput(t, agentWork, "config", "user.email", "agent@example.com")
	gitOutput(t, agentWork, "switch", "-c", "governance/delivery")
	writeWorkflowFile(t, agentWork, "DELIVERY.md", "agent drafted retry guidance\n")
	gitOutput(t, agentWork, "add", "DELIVERY.md")
	gitOutput(t, agentWork, "commit", "-m", "Draft governed delivery guidance")
	gitOutput(t, agentWork, "push", "-u", "origin", "governance/delivery")
	humanWork := gitClone(t, remote(contributorGit))
	gitOutput(t, humanWork, "config", "user.name", "Proven contributor")
	gitOutput(t, humanWork, "config", "user.email", "contributor@example.com")
	gitOutput(t, humanWork, "switch", "-c", "governance/verified", "origin/governance/delivery")
	writeWorkflowFile(t, humanWork, "DELIVERY.md", "agent drafted retry guidance; human verified against the accepted evidence\n")
	gitOutput(t, humanWork, "add", "DELIVERY.md")
	gitOutput(t, humanWork, "commit", "-m", "Verify governed delivery guidance")
	gitOutput(t, humanWork, "push", "-u", "origin", "governance/verified")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests", contributor, `{"title":"Deliver approved community initiative","body":"Human-verified agent work linked to governance proposal `+initiative.ID+`","source_branch":"governance/verified","target_branch":"main"}`, http.StatusCreated, &pull)
	pullBase := "/repositories/" + string(repository.ID) + "/pull-requests/" + pull.ID
	waitForWorkflowCheck(t, server.URL, pullBase, contributor, pull.SourceCommitID, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", owner, `{}`, http.StatusOK, &pull)
	var release releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/releases", owner, `{"version":"v1.0.0","commit_id":"`+pull.MergeCommitID+`","notes":"Governed initiative delivered through ordinary policy"}`, http.StatusCreated, &release)
	waitForReleaseArtifact(t, server.URL, string(repository.ID), release.ID, owner)
	implementation := `{"expected_receipt_digest":"` + initiative.DecisionReceipt.Digest + `","artifact_kind":"initiative","resource_ref":"pull_request:` + pull.ID + `","detail":"Owner linked the reviewed and released delivery","scope":"repository delivery","affected_resources":["branches:main","releases"],"implementation_effects":["human and agent delivery","reviewed repository change"]}`
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/proposals/"+initiative.ID+"/implementation", owner, implementation, http.StatusCreated, &initiative)

	// The community elects a successor; completion removes derived standing but
	// the repository owner must separately approve the attributable handoff.
	election := open("Elect successor maintainer", "leadership_nomination")
	for _, ballot := range []struct{ token, choice string }{{owner, "adopt"}, {contributor, "adopt"}, {successor, "adopt"}} {
		workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/proposals/"+election.ID+"/ballots", ballot.token, `{"choice":"`+ballot.choice+`"}`, http.StatusCreated, &election)
	}
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/proposals/"+election.ID+"/tally", owner, `{"close_early":true}`, http.StatusOK, &election)
	handoff := `{"kind":"election","role":"maintainer","former_standing_id":"` + ownerStanding + `","nominee_standing_id":"` + successorStanding + `","decision_receipt_id":"` + election.DecisionReceipt.ID + `","reason":"Community elected the successor","resource_handoffs":[{"resource":"repository:` + string(repository.ID) + `","from_id":"owner","to_id":"successor"}]}`
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/stewardship", owner, handoff, http.StatusCreated, &charter)
	caseID := charter.Stewardship[len(charter.Stewardship)-1].ID
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/stewardship/"+caseID+"/complete", owner, `{"reason":"Election receipt verified"}`, http.StatusOK, &charter)
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/stewardship/"+caseID+"/approve_handoff", owner, `{"reason":"Owner approves external transfer step","resource":"repository:`+string(repository.ID)+`"}`, http.StatusOK, &charter)
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/stewardship/"+caseID+"/appeal", owner, `{"reason":"Appeal records disputed transition timing"}`, http.StatusOK, &charter)

	expires := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	reviewDue := time.Now().UTC().Add(12 * time.Hour).Format(time.RFC3339)
	emergency := `{"kind":"emergency","role":"maintainer","reason":"Preserve project availability during the appealed handoff","emergency_scope":["triage availability"],"expires_at":"` + expires + `","review_due_at":"` + reviewDue + `"}`
	workflowJSON(t, server.URL, http.MethodPost, governanceBase+"/stewardship", owner, emergency, http.StatusCreated, &charter)
	var health governance.GovernanceHealth
	workflowJSON(t, server.URL, http.MethodGet, governanceBase+"/health", owner, "", http.StatusOK, &health)
	if len(health.OpenAppeals) != 1 || len(health.ActiveEmergencyPowers) != 1 || len(health.UnresolvedHandoffs) != 1 {
		t.Fatalf("exceptional governance health is incomplete: %#v", health)
	}
	if inspected, err := catalog.Inspect(repository.ID); err != nil || inspected.OwnerID != "owner" || release.CommitID != pull.MergeCommitID || !strings.Contains(pull.Body, initiative.ID) {
		t.Fatalf("governance altered authority or lost delivery history: repo=%#v release=%#v pull=%#v err=%v", inspected, release, pull, err)
	}
}
