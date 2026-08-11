package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deliveryteams"
	"github.com/greptile-projects/vivarium-komodo/apps/api/investigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestDecisionTeamToReleasedOutcomeWorkflow is the black-box boundary for a
// mixed delivery team. Team collaboration crosses public HTTP and contribution
// state crosses stock Git; only fixture identities and already-established
// contexts are installed through their stores.
func TestDecisionTeamToReleasedOutcomeWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is required for the delivery-team workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	orgs, _ := organizations.New(t.TempDir())
	decisionsStore, _ := decisions.New(t.TempDir())
	investigationStore, _ := investigations.New(t.TempDir())
	teams, _ := deliveryteams.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	runner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, catalog, credentials, nil, runner, checks)
	registerCheckRunsHTTP(mux, checks, runner, pulls, catalog, credentials, nil, nil)
	registerReleasesHTTP(mux, releaseStore, checks, runner, pulls, catalog, credentials)
	registerDeliveryTeamsHTTP(mux, teams, catalog, credentials, orgs, deliveryExecutionStores{investigations: investigationStore, decisions: decisionsStore}, pulls, runner)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	developer := issueAccess(t, credentials, "developer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	developerGit := issueAccess(t, credentials, "developer", auth.Git, auth.GitRead, auth.GitWrite)
	organization, err := orgs.Create("maintainer", "delivery-lab", "Delivery lab", "")
	if err != nil {
		t.Fatal(err)
	}
	organization, _ = orgs.Invite(organization.ID, "maintainer", "developer")
	organization, _ = orgs.Accept(organization.ID, "developer")
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"team-service","visibility":"private"}`, http.StatusCreated, &repository)
	if _, err = catalog.TransferOwner(repository.ID, "user", "maintainer", "organization", organization.ID, "maintainer"); err != nil {
		t.Fatal(err)
	}
	if _, err = catalog.AddCollaborator("maintainer", repository.ID, "developer"); err != nil {
		t.Fatal(err)
	}

	agents := make([]organizations.Agent, 3)
	for i, slug := range []string{"research-agent", "implementation-agent", "verification-agent"} {
		_, agents[i], err = orgs.RegisterAgent(organization.ID, "maintainer", organizations.Agent{Slug: slug, Name: slug, Capabilities: []string{"contents:read", "candidate_branch:write"}, OperatorIDs: []string{"maintainer"}, Visibility: "internal"})
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = orgs.GrantRole(organization.ID, "maintainer", organizations.RoleGrant{PrincipalKind: "agent", PrincipalID: agents[i].ID, Role: "contributor", Resources: []organizations.ResourceRef{{Kind: "repository", ID: string(repository.ID), RepositoryID: string(repository.ID)}}, Reason: "Bounded delivery-team specialization", ExpiresAt: time.Now().UTC().Add(24 * time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
	}

	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	clone := gitClone(t, remote(maintainerGit))
	gitOutput(t, clone, "config", "user.name", "Maintainer")
	gitOutput(t, clone, "config", "user.email", "maintainer@example.com")
	writeWorkflowFile(t, clone, "README.md", "# Team service\n")
	writeWorkflowFile(t, clone, ".komodo/checks.json", `{"version":1,"checks":[{"name":"team","command":"test -f README.md"}]}`)
	writeWorkflowFile(t, clone, ".komodo/releases.json", `{"version":1,"builds":[{"name":"team-record","command":"mkdir -p dist; cp agent.txt human.txt dist/","artifacts":["dist/agent.txt","dist/human.txt"]}]}`)
	gitOutput(t, clone, "add", ".")
	gitOutput(t, clone, "commit", "-m", "Record accepted decision baseline")
	baseRevision := gitOutput(t, clone, "rev-parse", "HEAD")
	gitOutput(t, clone, "push", "-u", "origin", "main")

	// The decision is an established, accepted upstream resource. Its detailed
	// uncertainty-to-commitment proof lives in decision_workflow_test.go.
	decision, err := decisionsStore.Create(string(repository.ID), "maintainer", "Adopt split validation", decisions.Context{Kind: "repository", ID: string(repository.ID)}, decisions.ScopeInput{Question: "How should validation be split?", Constraints: []string{"Retain human verification"}, SuccessMeasures: []string{"team check passes"}, ParticipantIDs: []string{"maintainer", "developer"}, OwnerID: "maintainer", ChangeSummary: "Accepted split"})
	if err != nil {
		t.Fatal(err)
	}
	decisionEvidence := decisions.Evidence{Kind: "code", RepositoryID: string(repository.ID), Revision: baseRevision, Path: "README.md", URL: "/repositories/" + string(repository.ID) + "?ref=" + baseRevision, Summary: "Exact accepted baseline", ObservedAt: time.Now().UTC()}
	decision, err = decisionsStore.AddAlternative(string(repository.ID), decision.ID, "maintainer", "Specialized delivery with human verification", []decisions.Claim{{Kind: "outcome", Body: "Parallel specialists deliver with an explicit human handoff."}}, []decisions.Evidence{decisionEvidence})
	if err != nil {
		t.Fatal(err)
	}
	decision, err = decisionsStore.Publish(string(repository.ID), decision.ID, "maintainer", decision.Alternatives[0].ID, nil, "Specialization shortens parallel work while the developer verifies handoffs.", []string{"Coordination overhead is retained."}, nil, []string{"Human verifies agent output."}, nil, []decisions.Evidence{decisionEvidence})
	if err != nil || decision.State != "published" {
		t.Fatalf("accepted decision fixture: %v %#v", err, decision)
	}

	for _, title := range []string{"Research exact behavior", "Implement accepted behavior", "Human verification"} {
		if _, err = investigationStore.Create(string(repository.ID), title, title, baseRevision, baseRevision, "maintainer", ""); err != nil {
			t.Fatal(err)
		}
	}
	contexts, _ := investigationStore.List(string(repository.ID))

	teamBase := "/repositories/" + string(repository.ID) + "/delivery-teams"
	var team deliveryteams.Team
	create := `{"name":"Decision delivery team","source":{"kind":"decision","id":"` + decision.ID + `","title":"Adopt split validation"},"charter":{"outcome":"Deliver the accepted validation decision","success_measures":["team check passes","human verifies the agent handoff"],"operating_principles":["cite disputed findings","preserve authority boundaries"],"total_budget":{"hours":24,"cost_units":100,"agent_runs":6},"default_escalation":"maintainer"}}`
	workflowJSON(t, server.URL, http.MethodPost, teamBase, maintainer, create, http.StatusCreated, &team)
	teamBase += "/" + team.ID

	invite := func(kind, principal, role string, budget string) string {
		body := `{"expected_version":` + itoa(team.Version) + `,"kind":"` + kind + `","principal_id":"` + principal + `","role":"` + role + `","why":"Complementary specialist","responsibilities":["deliver scoped evidence"],"budget":` + budget + `,"requested_actions":["contents:read","candidate_branch:write"]}`
		workflowJSON(t, server.URL, http.MethodPost, teamBase+"/participants", maintainer, body, http.StatusCreated, &team)
		return team.Participants[len(team.Participants)-1].ID
	}
	developerParticipant := invite("human", "developer", "human verifier", `{"hours":8,"cost_units":10,"agent_runs":0}`)
	researchParticipant := invite("agent", agents[0].ID, "research specialist", `{"hours":5,"cost_units":25,"agent_runs":2}`)
	implementationParticipant := invite("agent", agents[1].ID, "implementation specialist", `{"hours":8,"cost_units":40,"agent_runs":2}`)
	verificationParticipant := invite("agent", agents[2].ID, "verification specialist", `{"hours":3,"cost_units":25,"agent_runs":2}`)
	accept := func(id, bearer string) {
		workflowJSON(t, server.URL, http.MethodPost, teamBase+"/participants/"+id+"/response", bearer, `{"expected_version":`+itoa(team.Version)+`,"response":"accepted"}`, http.StatusOK, &team)
	}
	accept(developerParticipant, developer)
	accept(researchParticipant, maintainer)
	accept(implementationParticipant, maintainer)
	accept(verificationParticipant, maintainer)

	streams := `[{"id":"research","title":"Resolve behavior","owner_participant_id":"` + researchParticipant + `","inputs":["accepted decision"],"expected_artifacts":["cited finding"],"acceptance_criteria":["dispute resolved"],"repository_scope":[{"repository_id":"` + string(repository.ID) + `","commit_id":"` + baseRevision + `","paths":["README.md"],"required_actions":["contents:read"]}],"integration_order":1,"budget":{"hours":5,"cost_units":25,"agent_runs":2},"assumptions":[]},{"id":"implementation","title":"Implement outcome","owner_participant_id":"` + implementationParticipant + `","inputs":["resolved finding"],"expected_artifacts":["agent.txt"],"depends_on":["research"],"acceptance_criteria":["agent output reviewed"],"repository_scope":[{"repository_id":"` + string(repository.ID) + `","commit_id":"` + baseRevision + `","paths":["agent.txt"],"required_actions":["contents:read","candidate_branch:write"]}],"integration_order":2,"budget":{"hours":8,"cost_units":40,"agent_runs":2},"assumptions":[{"id":"finding","statement":"research is resolved","source_stream_id":"research","source_stream_revision":1}]},{"id":"verification","title":"Verify and integrate","owner_participant_id":"` + developerParticipant + `","inputs":["agent handoff"],"expected_artifacts":["human.txt"],"depends_on":["implementation"],"acceptance_criteria":["team check passes"],"repository_scope":[{"repository_id":"` + string(repository.ID) + `","commit_id":"` + baseRevision + `","paths":["human.txt"],"required_actions":["contents:read","candidate_branch:write"]}],"integration_order":3,"budget":{"hours":8,"cost_units":10,"agent_runs":0},"assumptions":[{"id":"implementation","statement":"agent output is reviewable","source_stream_id":"implementation","source_stream_revision":1}]}]`
	workflowJSON(t, server.URL, http.MethodPost, teamBase+"/plan/versions", maintainer, `{"expected_version":`+itoa(team.Version)+`,"change_reason":"Parallelize accepted decision delivery","streams":`+streams+`}`, http.StatusCreated, &team)
	for _, id := range []string{researchParticipant, implementationParticipant, developerParticipant} {
		bearer := maintainer
		if id == developerParticipant {
			bearer = developer
		}
		workflowJSON(t, server.URL, http.MethodPost, teamBase+"/plan/versions/1/acceptances", bearer, `{"expected_version":`+itoa(team.Version)+`,"participant_id":"`+id+`"}`, http.StatusCreated, &team)
	}
	if team.Plan.Current.Status != "accepted" {
		t.Fatalf("plan not accepted: %#v", team.Plan.Current)
	}

	contextByStream := map[string]investigations.Investigation{"research": contexts[0], "implementation": contexts[1], "verification": contexts[2]}
	ownerByStream := map[string]string{"research": researchParticipant, "implementation": implementationParticipant, "verification": developerParticipant}
	bearerByStream := map[string]string{"research": maintainer, "implementation": maintainer, "verification": developer}
	for _, stream := range []string{"research", "implementation", "verification"} {
		c := contextByStream[stream]
		body := `{"expected_version":` + itoa(team.Version) + `,"participant_id":"` + ownerByStream[stream] + `","context":{"kind":"investigation","id":"` + c.ID + `","repository_id":"` + string(repository.ID) + `","revision":"` + baseRevision + `"}}`
		workflowJSON(t, server.URL, http.MethodPost, teamBase+"/streams/"+stream+"/contexts", bearerByStream[stream], body, http.StatusCreated, &team)
	}

	postEntry := func(stream, participant, bearer, kind, summary, path string) string {
		c := contextByStream[stream]
		body := `{"expected_version":` + itoa(team.Version) + `,"participant_id":"` + participant + `","stream_id":"` + stream + `","kind":"` + kind + `","summary":"` + summary + `","context":{"kind":"investigation","id":"` + c.ID + `","repository_id":"` + string(repository.ID) + `","revision":"` + baseRevision + `"},"citations":[{"repository_id":"` + string(repository.ID) + `","revision":"` + baseRevision + `","path":"` + path + `","resource_kind":"blob","resource_id":"` + path + `"}]}`
		workflowJSON(t, server.URL, http.MethodPost, teamBase+"/timeline", bearer, body, http.StatusCreated, &team)
		return team.Timeline[len(team.Timeline)-1].ID
	}
	disputed := postEntry("research", researchParticipant, maintainer, "finding", "Validation must be agent-only", "README.md")
	resolved := postEntry("research", researchParticipant, maintainer, "decision", "Developer challenge accepted: retain explicit human verification", "README.md")
	_ = disputed
	status := func(stream, participant, bearer, state, action string, hours, runs int) {
		body := `{"expected_version":` + itoa(team.Version) + `,"participant_id":"` + participant + `","reported_revision":"` + baseRevision + `","status":"` + state + `","active_action":"` + action + `","predicted_next_action":"publish scoped output","resource_use":{"hours":` + itoa(int64(hours)) + `,"cost_units":10,"agent_runs":` + itoa(int64(runs)) + `},"access_state":"active","output_state":"clean"}`
		workflowJSON(t, server.URL, http.MethodPost, teamBase+"/streams/"+stream+"/status", bearer, body, http.StatusOK, &team)
	}
	status("research", researchParticipant, maintainer, "completed", "resolve challenged finding", 3, 1)
	status("implementation", implementationParticipant, maintainer, "failed", "produce agent output", 2, 1)
	workflowJSON(t, server.URL, http.MethodPost, teamBase+"/controls", maintainer, `{"expected_version":`+itoa(team.Version)+`,"stream_id":"implementation","action":"reassign","instruction":"Preserve the accepted finding and redirect bounded implementation","target_participant_id":"`+verificationParticipant+`"}`, http.StatusCreated, &team)
	status("implementation", verificationParticipant, maintainer, "completed", "complete redirected implementation", 5, 2)
	implementationEvidence := postEntry("implementation", implementationParticipant, maintainer, "artifact", "Redirected agent output is ready for human verification", "agent.txt")

	c := contextByStream["implementation"]
	handoffBody := `{"expected_version":` + itoa(team.Version) + `,"participant_id":"` + implementationParticipant + `","stream_id":"implementation","to_participant_id":"` + developerParticipant + `","input_entry_ids":["` + implementationEvidence + `"],"context":{"kind":"investigation","id":"` + c.ID + `","repository_id":"` + string(repository.ID) + `","revision":"` + baseRevision + `"},"acceptance_criteria":["reproduce the team check"],"residual_uncertainty":["release build not yet observed"]}`
	workflowJSON(t, server.URL, http.MethodPost, teamBase+"/handoffs", maintainer, handoffBody, http.StatusCreated, &team)
	handoffID := team.Handoffs[len(team.Handoffs)-1].ID
	workflowJSON(t, server.URL, http.MethodPost, teamBase+"/handoffs/"+handoffID+"/acceptance", developer, `{"expected_version":`+itoa(team.Version)+`,"participant_id":"`+developerParticipant+`","note":"Verified exact inputs and retained uncertainty"}`, http.StatusOK, &team)
	status("verification", developerParticipant, developer, "completed", "verify handoff", 4, 0)
	verificationEvidence := postEntry("verification", developerParticipant, developer, "artifact", "Human verification reproduces the expected result", "human.txt")

	// Materialize the two contribution branches after the governed evidence is
	// accepted. Both descend from the exact plan revision.
	agentClone := gitClone(t, remote(developerGit))
	gitOutput(t, agentClone, "config", "user.name", "Team agents")
	gitOutput(t, agentClone, "config", "user.email", "agents@example.com")
	gitOutput(t, agentClone, "switch", "-c", "team/research")
	writeWorkflowFile(t, agentClone, "README.md", "# Team service\n\nHuman verification is required.\n")
	gitOutput(t, agentClone, "add", "README.md")
	gitOutput(t, agentClone, "commit", "-m", "Resolve disputed validation finding")
	gitOutput(t, agentClone, "push", "-u", "origin", "team/research")
	gitOutput(t, agentClone, "switch", "-c", "team/agent")
	writeWorkflowFile(t, agentClone, "agent.txt", "redirected agent output\n")
	writeWorkflowFile(t, agentClone, "human.txt", "pending human verification\n")
	gitOutput(t, agentClone, "add", ".")
	gitOutput(t, agentClone, "commit", "-m", "Add redirected agent output")
	gitOutput(t, agentClone, "push", "-u", "origin", "team/agent")
	humanClone := gitClone(t, remote(developerGit))
	gitOutput(t, humanClone, "config", "user.name", "Developer")
	gitOutput(t, humanClone, "config", "user.email", "developer@example.com")
	gitOutput(t, humanClone, "switch", "-c", "team/human", "origin/team/agent")
	writeWorkflowFile(t, humanClone, "human.txt", "human verified agent handoff\n")
	gitOutput(t, humanClone, "add", "human.txt")
	gitOutput(t, humanClone, "commit", "-m", "Verify agent handoff")
	gitOutput(t, humanClone, "push", "-u", "origin", "team/human")

	contributions := `[{"stream_id":"research","source_branch":"team/agent","target_branch":"main","title":"Resolve disputed team finding","summary":"Retain the challenged and resolved evidence","evidence_entry_ids":["` + resolved + `"],"criteria":[{"criterion":"dispute resolved","status":"met","evidence_entry_ids":["` + resolved + `"]}]},{"stream_id":"implementation","source_branch":"team/agent","target_branch":"main","title":"Implement redirected output","summary":"Publish bounded redirected agent work","evidence_entry_ids":["` + implementationEvidence + `"],"handoff_ids":["` + handoffID + `"],"criteria":[{"criterion":"agent output reviewed","status":"met","evidence_entry_ids":["` + implementationEvidence + `"]}]},{"stream_id":"verification","source_branch":"team/human","target_branch":"main","title":"Verify complete team outcome","summary":"Human verification completes the accepted decision","evidence_entry_ids":["` + verificationEvidence + `"],"criteria":[{"criterion":"team check passes","status":"met","evidence_entry_ids":["` + verificationEvidence + `"]}]}]`
	contributions = strings.Replace(contributions, `"source_branch":"team/agent"`, `"source_branch":"team/research"`, 1)
	workflowJSON(t, server.URL, http.MethodPost, teamBase+"/integration/reconciliations", maintainer, `{"expected_version":`+itoa(team.Version)+`,"contributions":`+contributions+`}`, http.StatusCreated, &team)
	integration := team.Integrations[len(team.Integrations)-1]
	if integration.Status != "ready" {
		t.Fatalf("integration blocked: %#v", integration.Blockers)
	}
	var published struct {
		Team deliveryteams.Team       `json:"team"`
		Pull pullrequests.PullRequest `json:"pull_request"`
	}
	publishedPulls := []pullrequests.PullRequest{}
	for _, stream := range []string{"research", "implementation", "verification"} {
		workflowJSON(t, server.URL, http.MethodPost, teamBase+"/integration/reconciliations/"+integration.ID+"/streams/"+stream+"/pull-request", maintainer, `{"expected_version":`+itoa(team.Version)+`}`, http.StatusCreated, &published)
		team = published.Team
		publishedPulls = append(publishedPulls, published.Pull)
	}
	for _, pull := range publishedPulls {
		base := "/repositories/" + string(repository.ID) + "/pull-requests/" + pull.ID
		waitForWorkflowCheck(t, server.URL, base, maintainer, pull.SourceCommitID, checkruns.Succeeded)
		workflowJSON(t, server.URL, http.MethodPut, base+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
		workflowJSON(t, server.URL, http.MethodPost, base+"/merge", maintainer, `{}`, http.StatusOK, &published.Pull)
	}
	var release releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/releases", maintainer, `{"version":"v1.0.0","commit_id":"`+published.Pull.MergeCommitID+`","notes":"Release the governed delivery-team outcome"}`, http.StatusCreated, &release)
	waitForReleaseArtifact(t, server.URL, string(repository.ID), release.ID, maintainer)

	// Removal revokes team participation without erasing accepted evidence,
	// controls, costs, handoff verification, or published review links.
	workflowJSON(t, server.URL, http.MethodDelete, teamBase+"/participants/"+implementationParticipant, maintainer, `{"expected_version":`+itoa(team.Version)+`,"reason":"Specialist engagement completed"}`, http.StatusOK, &team)
	before := len(team.Timeline)
	workflowJSON(t, server.URL, http.MethodPost, teamBase+"/timeline", maintainer, `{"expected_version":`+itoa(team.Version)+`,"participant_id":"`+implementationParticipant+`","stream_id":"implementation","kind":"finding","summary":"must be rejected","context":{"kind":"investigation","id":"`+c.ID+`","repository_id":"`+string(repository.ID)+`","revision":"`+baseRevision+`"},"citations":[{"repository_id":"`+string(repository.ID)+`","revision":"`+baseRevision+`","path":"agent.txt","resource_kind":"blob","resource_id":"agent.txt"}]}`, http.StatusForbidden, nil)
	if len(team.Timeline) != before || len(team.Controls) == 0 || team.Handoffs[0].Status != "accepted" || len(team.Integrations) == 0 || release.PullRequests == nil {
		t.Fatalf("completed team record lost governance evidence: team=%#v release=%#v", team, release)
	}
}

func itoa(v int64) string { return fmt.Sprintf("%d", v) }
