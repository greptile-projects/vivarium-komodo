package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityreports"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

type quietStewardshipIncidents struct{}

func (quietStewardshipIncidents) List(string) ([]incidents.Incident, error) { return nil, nil }
func (quietStewardshipIncidents) Get(string, string) (incidents.Incident, error) {
	return incidents.Incident{}, incidents.ErrNotFound
}

type quietStewardshipSecurity struct{}

func (quietStewardshipSecurity) Get(string, string, func(string) bool) (securityreports.Report, error) {
	return securityreports.Report{}, securityreports.ErrNotFound
}
func (quietStewardshipSecurity) ListVisible(string, func(string) bool) ([]securityreports.Report, error) {
	return nil, nil
}

type stewardshipActivitySink struct{ events []activities.Event }

func (s *stewardshipActivitySink) Record(in activities.Input) (activities.Event, error) {
	e := activities.Event{RepositoryID: in.RepositoryID, ActorID: in.ActorID, Type: in.Type, Resource: in.Resource, TargetUserID: in.TargetUserID, Metadata: in.Metadata}
	s.events = append(s.events, e)
	return e, nil
}
func (s *stewardshipActivitySink) List(repositoryID string) ([]activities.Event, error) {
	items := []activities.Event{}
	for _, event := range s.events {
		if event.RepositoryID == repositoryID {
			items = append(items, event)
		}
	}
	return items, nil
}

// TestSignalToStewardedImprovementWorkflow composes the complete stewardship
// boundary through public HTTP and stock Git: bounded authorization, two
// evidence-backed findings, collaborative prioritization, governed agent work,
// verification, review, merge, release, outcome accounting, and revocation.
func TestSignalToStewardedImprovementWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the stewardship workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	people, _ := users.New(t.TempDir())
	orgs, _ := organizations.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	runner := checkruns.NewRunner(checks, catalog)
	activity := &stewardshipActivitySink{}
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerProposalTaskSessionsHTTP(mux, plans, sessions, catalog, credentials, activity, pulls, runner)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, activity, runner, checks)
	registerCheckRunsHTTP(mux, checks, runner, pulls, catalog, credentials, sessions, activity)
	registerReleasesHTTP(mux, releaseStore, checks, runner, pulls, catalog, credentials)
	registerOrganizationsHTTP(mux, orgs, catalog, people, nil, releaseStore, pulls, quietStewardshipIncidents{}, plans, nil, quietStewardshipSecurity{}, credentials, activity)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	operator := issueAccess(t, credentials, "operator", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	var organization organizations.Organization
	workflowJSON(t, server.URL, http.MethodPost, "/organizations", maintainer, `{"slug":"runtime","name":"Runtime","description":"Maintained with bounded agent help."}`, http.StatusCreated, &organization)
	// Membership creation itself is setup; every stewardship decision below is
	// exercised through its public endpoint.
	organization, _ = orgs.Invite(organization.ID, "maintainer", "operator")
	organization, _ = orgs.Accept(organization.ID, "operator")
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/organizations/"+organization.ID+"/repositories", maintainer, `{"name":"service","visibility":"private"}`, http.StatusCreated, &repository)

	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		value.User = url.UserPassword("git", token)
		return value.String()
	}
	clone := gitClone(t, remote(maintainerGit))
	gitOutput(t, clone, "config", "user.name", "Maintainer")
	gitOutput(t, clone, "config", "user.email", "maintainer@example.com")
	writeWorkflowFile(t, clone, "README.md", "# Service\n\nRequests use an unlimited retry loop.\n")
	writeWorkflowFile(t, clone, "retry.conf", "max_attempts=0\n")
	writeWorkflowFile(t, clone, ".komodo/checks.json", `{"version":1,"checks":[{"name":"stewardship","command":"grep -qx 'max_attempts=3' retry.conf"}]}`)
	writeWorkflowFile(t, clone, ".komodo/releases.json", `{"version":1,"builds":[{"name":"config","command":"mkdir -p dist; cp retry.conf dist/retry.conf","artifacts":["dist/retry.conf"]}]}`)
	gitOutput(t, clone, "add", ".")
	gitOutput(t, clone, "commit", "-m", "Initialize retry policy")
	baseRevision := gitOutput(t, clone, "rev-parse", "HEAD")
	gitOutput(t, clone, "push", "-u", "origin", "main")

	var agent organizations.Agent
	workflowJSON(t, server.URL, http.MethodPost, "/organizations/"+organization.ID+"/agents", maintainer, `{"slug":"caretaker","name":"Caretaker","capabilities":["checks:read","proposals:write"],"operator_ids":["operator"],"visibility":"internal"}`, http.StatusCreated, &agent)
	starts, expires := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339)
	var mandate organizations.StewardshipMandate
	mandateBody := `{"title":"Keep runtime retries safe","desired_outcomes":["Retries remain bounded and verified"],"scopes":[{"repository_id":"` + string(repository.ID) + `","branches":["main"]}],"trusted_signals":["usage.threshold","dependency.notice"],"exclusions":["security embargoes"],"budget":{"max_hours_per_month":8,"max_runs_per_day":2},"schedule":{"starts_at":"` + starts + `","expires_at":"` + expires + `","cadence":"continuous"},"agent_id":"` + agent.ID + `","allowed_actions":["inspect evidence","draft proposal","implement on candidate branch"],"required_human_decisions":["merge","release"]}`
	workflowJSON(t, server.URL, http.MethodPost, "/organizations/"+organization.ID+"/stewardship-mandates", maintainer, mandateBody, http.StatusCreated, &mandate)
	mandateBase := "/organizations/" + organization.ID + "/stewardship-mandates/" + mandate.ID + "/versions/1"
	workflowJSON(t, server.URL, http.MethodPost, mandateBase+"/accept", operator, `{}`, http.StatusOK, &mandate)
	var preview struct {
		Authority bool `json:"authority_created_by_mandate"`
		Write     bool `json:"mandate_write_authority"`
		Merge     bool `json:"mandate_merge_authority"`
	}
	workflowJSON(t, server.URL, http.MethodGet, mandateBase+"/preview", maintainer, "", http.StatusOK, &preview)
	if preview.Authority || preview.Write || preview.Merge {
		t.Fatal("stewardship mandate silently created authority")
	}
	workflowJSON(t, server.URL, http.MethodPut, mandateBase+"/work-policy", maintainer, `{"expected_version":0,"rules":[{"class":"usage_regression","mode":"auto_start","max_risk":"medium","max_runs_per_day":2,"max_hours_per_month":8,"priority":90,"min_confidence":0.8,"required_evidence":["usage_event"]}]}`, http.StatusOK, nil)

	evaluations := "/organizations/" + organization.ID + "/stewardship-opportunities/evaluations"
	observed := time.Now().UTC().Add(-time.Second).Format(time.RFC3339)
	var useful organizations.StewardshipOpportunity
	usefulBody := `{"deduplication_key":"unbounded-retry","mandate_id":"` + mandate.ID + `","mandate_version":1,"repository_id":"` + string(repository.ID) + `","class":"usage_regression","title":"Bound retries after timeout growth","summary":"Usage evidence shows unlimited retries amplify timeout load.","severity":"high","expected_value":"Reduce repeated requests while keeping transient recovery.","confidence":0.94,"affected_owner_ids":["maintainer"],"affected_revisions":["` + baseRevision + `"],"in_scope_reason":"The mandate requires runtime retries to remain bounded and verified.","signal":"usage.threshold","citations":[{"kind":"usage_event","resource_id":"usage-2026-08-10","repository_id":"` + string(repository.ID) + `","revision":"` + baseRevision + `","summary":"Retry volume rose 41 percent after upstream timeouts.","observed_at":"` + observed + `"}]}`
	workflowJSON(t, server.URL, http.MethodPost, evaluations, operator, usefulBody, http.StatusCreated, &useful)
	var dismissed organizations.StewardshipOpportunity
	dismissedBody := strings.ReplaceAll(usefulBody, `"unbounded-retry"`, `"replace-client"`)
	dismissedBody = strings.ReplaceAll(dismissedBody, `"usage_regression"`, `"dependency_cleanup"`)
	dismissedBody = strings.ReplaceAll(dismissedBody, `"Bound retries after timeout growth"`, `"Replace the HTTP client"`)
	dismissedBody = strings.ReplaceAll(dismissedBody, `"usage.threshold"`, `"dependency.notice"`)
	workflowJSON(t, server.URL, http.MethodPost, evaluations, operator, dismissedBody, http.StatusCreated, &dismissed)
	opportunityBase := "/organizations/" + organization.ID + "/stewardship-opportunities/"
	workflowJSON(t, server.URL, http.MethodPost, opportunityBase+dismissed.ID+"/decisions", maintainer, `{"action":"dismiss","reason":"The supported client is current; the cited load belongs to retry policy instead."}`, http.StatusOK, &dismissed)
	workflowJSON(t, server.URL, http.MethodPost, opportunityBase+useful.ID+"/comments", maintainer, `{"body":"Prefer three attempts and prove the configured bound in the repository check."}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, opportunityBase+useful.ID+"/decisions", maintainer, `{"action":"rank","rank":1}`, http.StatusOK, &useful)
	workflowJSON(t, server.URL, http.MethodPost, opportunityBase+useful.ID+"/work-decisions", operator, `{"mode":"auto_start","risk":"medium","hours":2,"expected_decision_version":0,"expected_policy_version":1}`, http.StatusOK, &useful)

	var promoted struct {
		Opportunity organizations.StewardshipOpportunity `json:"opportunity"`
		Proposal    proposals.Proposal                   `json:"proposal"`
		Tasks       []proposals.Task                     `json:"tasks"`
	}
	promotion := `{"title":"Bound runtime retries","body":"Deliver the evidence-backed retry improvement.","base_revision":"` + baseRevision + `","tasks":[{"title":"Set a bounded retry policy","outcome":"retry.conf limits attempts to three","owner_kind":"agent","owner_id":"codex","risk":"medium","completion_criteria":["Retry attempts are limited to three","The repository stewardship check passes"],"verification_plan":["Run the stewardship check"],"depends_on":[]}]}`
	workflowJSON(t, server.URL, http.MethodPost, opportunityBase+useful.ID+"/promotion", maintainer, promotion, http.StatusCreated, &promoted)
	task := promoted.Tasks[0]
	planBase := "/repositories/" + string(repository.ID) + "/proposals/" + promoted.Proposal.ID + "/plan/tasks/" + task.ID
	workflowJSON(t, server.URL, http.MethodPut, planBase+"/assignment", maintainer, `{"kind":"agent","assignee_id":"codex","mandate":"Implement only the accepted bounded retry outcome.","base_revision":"`+baseRevision+`"}`, http.StatusOK, &task)
	var started struct {
		Session    changesessions.Session         `json:"session"`
		Run        changesessions.Run             `json:"run"`
		Credential struct{ Token, Branch string } `json:"credential"`
	}
	workflowJSON(t, server.URL, http.MethodPost, planBase+"/change-sessions", maintainer, `{"expected_assignment_id":"`+task.Assignment.ID+`"}`, http.StatusCreated, &started)
	runBase := planBase + "/change-sessions/" + started.Session.ID + "/runs/" + started.Run.ID
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/interventions", maintainer, `{"type":"guidance","message":"Keep the documentation aligned with the exact configured value."}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/events", started.Credential.Token, `{"type":"run.started","metadata":{"status":"Applying the prioritized retry bound","estimated_cost":"2 agent-hours"}}`, http.StatusCreated, nil)

	agentClone := gitClone(t, remote(started.Credential.Token))
	gitOutput(t, agentClone, "config", "user.name", "Caretaker Agent")
	gitOutput(t, agentClone, "config", "user.email", "caretaker@agents.local")
	gitOutput(t, agentClone, "switch", strings.TrimPrefix(started.Credential.Branch, "refs/heads/"))
	writeWorkflowFile(t, agentClone, "retry.conf", "max_attempts=3\n")
	writeWorkflowFile(t, agentClone, "README.md", "# Service\n\nRequests retry at most three times.\n")
	gitOutput(t, agentClone, "add", "README.md", "retry.conf")
	gitOutput(t, agentClone, "commit", "-m", "Bound retry attempts")
	agentRevision := gitOutput(t, agentClone, "rev-parse", "HEAD")
	gitOutput(t, agentClone, "push", "origin", strings.TrimPrefix(started.Credential.Branch, "refs/heads/"))
	var publication struct {
		Pull pullrequests.PullRequest `json:"pull_request"`
	}
	delivery := `{"expected_assignment_id":"` + task.Assignment.ID + `","session_id":"` + started.Session.ID + `","title":"Bound runtime retries","target_branch":"main","delivery_evidence":{"reasoning":"The usage signal points to unlimited retries; three attempts preserves transient recovery while bounding load.","commands":["grep -qx 'max_attempts=3' retry.conf"],"residual_risks":["Retry timing remains controlled by the existing client."],"completion_criteria":[{"criterion":"Retry attempts are limited to three","status":"met","evidence":"retry.conf sets max_attempts=3"},{"criterion":"The repository stewardship check passes","status":"met","evidence":"The commit-bound stewardship check verifies the exact setting."}]}}`
	workflowJSON(t, server.URL, http.MethodPost, planBase+"/contributions", started.Credential.Token, delivery, http.StatusCreated, &publication)
	if publication.Pull.SourceCommitID != agentRevision || publication.Pull.ReasoningContext == nil || publication.Pull.DeliveryEvidence == nil {
		t.Fatalf("review handoff lost stewardship provenance: %#v", publication.Pull)
	}
	pullBase := "/repositories/" + string(repository.ID) + "/pull-requests/" + publication.Pull.ID
	waitForWorkflowCheck(t, server.URL, pullBase, maintainer, agentRevision, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
	var merged pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", maintainer, `{}`, http.StatusOK, &merged)
	var release releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/releases", maintainer, `{"version":"v1.0.1","commit_id":"`+merged.MergeCommitID+`","notes":"Deliver the steward-discovered retry bound."}`, http.StatusCreated, &release)
	build, artifact := waitForReleaseArtifact(t, server.URL, string(repository.ID), release.ID, maintainer)

	outcome := `{"implementation":"succeeded","verification":"passed","release":"released","actual_hours":2,"runs":1,"false_positive":false,"summary":"The bounded retry policy was reviewed, verified, merged, and released.","evidence":[{"kind":"release","resource_id":"` + release.ID + `","repository_id":"` + string(repository.ID) + `","revision":"` + merged.MergeCommitID + `","summary":"Verified release ` + release.Version + ` contains build ` + build.ID + ` and artifact ` + artifact.ID + `.","observed_at":"` + time.Now().UTC().Format(time.RFC3339) + `"}],"goal_results":[{"goal":"Retries remain bounded and verified","state":"advanced","evidence":"The merged release sets max_attempts=3 and its commit-bound check passed."}]}`
	workflowJSON(t, server.URL, http.MethodPost, opportunityBase+useful.ID+"/outcomes", maintainer, outcome, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, mandateBase+"/revoke", maintainer, `{}`, http.StatusOK, &mandate)
	var report organizations.StewardshipReport
	workflowJSON(t, server.URL, http.MethodGet, mandateBase+"/report", maintainer, "", http.StatusOK, &report)
	if mandate.State != "revoked" || dismissed.State != "dismiss" || report.ResourceUse["actual_hours"] != 2 || report.ReleaseResults["released"] != 1 || report.GoalProgress["Retries remain bounded and verified"]["advanced"] != 1 {
		t.Fatalf("continuous stewardship record is incomplete: mandate=%s dismissed=%s report=%#v", mandate.State, dismissed.State, report)
	}
	var restored changesessions.Session
	workflowJSON(t, server.URL, http.MethodGet, planBase+"/change-sessions/"+started.Session.ID, maintainer, "", http.StatusOK, &restored)
	if len(restored.Events) < 4 || restored.ContributionPullRequestID != publication.Pull.ID || len(activity.events) < 2 {
		t.Fatalf("human and agent decision trail is incomplete: session=%#v activity=%#v", restored, activity.events)
	}
	verified := gitClone(t, remote(maintainerGit))
	assertFile(t, filepath.Join(verified, "retry.conf"), "max_attempts=3\n", 0)
}
