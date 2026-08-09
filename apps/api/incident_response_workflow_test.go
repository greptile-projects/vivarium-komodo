package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestSignalToLearningIncidentResponseWorkflow proves that responders can move
// from a failed production signal through agent-assisted diagnosis, independently
// approved recovery, resolution, and a verified corrective deployment using only
// public HTTP, incident-worker, and stock Git surfaces.
func TestSignalToLearningIncidentResponseWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the incident response workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	deploymentStore, _ := deployments.New(t.TempDir())
	incidentStore, _ := incidents.New(t.TempDir())
	runner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerProposalTaskSessionsHTTP(mux, plans, sessions, catalog, credentials, nil, pulls, runner, checks)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, nil, runner, checks)
	registerCheckRunsHTTP(mux, checks, runner, pulls, catalog, credentials, sessions, nil)
	registerReleasesHTTP(mux, releaseStore, checks, runner, pulls, catalog, credentials)
	registerDeploymentsHTTP(mux, deploymentStore, releaseStore, checks, catalog, credentials, nil, sessions, pulls)
	registerIncidentsHTTP(mux, incidentStore, deploymentStore, releaseStore, pulls, catalog, credentials, plans, checks)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	commander := issueAccess(t, credentials, "commander", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	commanderGit := issueAccess(t, credentials, "commander", auth.Git, auth.GitRead, auth.GitWrite)
	responder := issueAccess(t, credentials, "responder", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	responderGit := issueAccess(t, credentials, "responder", auth.Git, auth.GitRead, auth.GitWrite)
	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", commander, `{"name":"incident-ready-service","visibility":"public"}`, http.StatusCreated, &repository)
	if _, err := catalog.AddCollaborator("commander", storage.ID(repository.ID), "responder"); err != nil {
		t.Fatal(err)
	}
	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		value.User = url.UserPassword("git", token)
		return value.String()
	}

	ownerClone := gitClone(t, remote(commanderGit))
	gitOutput(t, ownerClone, "config", "user.name", "Incident Commander")
	gitOutput(t, ownerClone, "config", "user.email", "commander@example.com")
	writeWorkflowFile(t, ownerClone, "README.md", "# Incident-ready service\n")
	writeWorkflowFile(t, ownerClone, ".komodo/checks.json", `{"version":1,"checks":[{"name":"service-contract","command":"test -f runbook.md"}]}`)
	writeWorkflowFile(t, ownerClone, ".komodo/releases.json", `{"version":1,"builds":[{"name":"package","command":"mkdir -p dist; cp health.txt dist/service","artifacts":["dist/service"]}]}`)
	writeWorkflowFile(t, ownerClone, ".komodo/deployments.json", `{"version":1,"environments":[{"name":"production","stages":[{"name":"rollout","health":[{"name":"service-ready","command":"grep -qx healthy \"$KOMODO_ARTIFACT_PATH\""}]}]}]}`)
	writeWorkflowFile(t, ownerClone, "health.txt", "healthy\n")
	writeWorkflowFile(t, ownerClone, "runbook.md", "# Service recovery\n")
	gitOutput(t, ownerClone, "add", ".")
	gitOutput(t, ownerClone, "commit", "-m", "Initialize incident-ready service")
	knownGoodCommit := gitOutput(t, ownerClone, "rev-parse", "HEAD")
	gitOutput(t, ownerClone, "push", "-u", "origin", "main")

	var v1 releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", commander, `{"version":"v1.0.0","commit_id":"`+knownGoodCommit+`","notes":"Known-good service."}`, http.StatusCreated, &v1)
	v1Build, v1Artifact := waitForReleaseArtifact(t, server.URL, repository.ID, v1.ID, commander)
	var environment deployments.Environment
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/environments", commander, `{"name":"production","position":1,"command":"printf deployed","required_approvals":1,"concurrency":1}`, http.StatusCreated, &environment)
	v1Start := promoteAndApprove(t, server.URL, repository.ID, responder, commander, environment.ID, v1.ID, v1Build.ID, v1Artifact.ID)
	v1Deployment := waitForDeployment(t, server.URL, repository.ID, v1Start.ID, commander, "succeeded")

	work := gitClone(t, remote(responderGit))
	gitOutput(t, work, "config", "user.name", "Operations Responder")
	gitOutput(t, work, "config", "user.email", "responder@example.com")
	gitOutput(t, work, "switch", "-c", "risk/regression")
	writeWorkflowFile(t, work, "health.txt", "unhealthy\n")
	gitOutput(t, work, "commit", "-am", "Introduce production regression")
	gitOutput(t, work, "push", "-u", "origin", "risk/regression")
	badPull := deliveryPull(t, server.URL, repository.ID, responder, commander, "Risky rollout", "risk/regression")
	var v2 releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", commander, `{"version":"v1.1.0","commit_id":"`+badPull.MergeCommitID+`","prior_release_id":"`+v1.ID+`","notes":"Degraded release."}`, http.StatusCreated, &v2)
	v2Build, v2Artifact := waitForReleaseArtifact(t, server.URL, repository.ID, v2.ID, commander)
	failedStart := promoteAndApprove(t, server.URL, repository.ID, responder, commander, environment.ID, v2.ID, v2Build.ID, v2Artifact.ID)
	failed := waitForDeployment(t, server.URL, repository.ID, failedStart.ID, commander, "failed")
	failedSignal := incidentHealthEvent(t, failed, "failed")

	incidentBase := "/repositories/" + repository.ID + "/incidents"
	var incident incidents.Incident
	workflowJSON(t, server.URL, http.MethodPost, incidentBase, responder, `{"title":"Production availability degraded","summary":"The v1.1 rollout failed its service readiness signal.","severity":"critical","roles":{"commander":"commander","operations":"responder"},"affected":[{"repository_id":"`+repository.ID+`","environment_id":"`+environment.ID+`"}],"source_signal":{"repository_id":"`+repository.ID+`","deployment_id":"`+failed.ID+`","event_sequence":`+workflowInt(failedSignal.Sequence)+`}}`, http.StatusCreated, &incident)
	incidentURL := incidentBase + "/" + incident.ID
	workflowJSON(t, server.URL, http.MethodPost, incidentURL+"/updates", commander, `{"audience":"public","message":"Responders are investigating a failed production rollout."}`, http.StatusCreated, &incident)
	publicUpdate := incident.Timeline[len(incident.Timeline)-1].Sequence
	workflowJSON(t, server.URL, http.MethodPost, incidentURL+"/acknowledgements", responder, `{"update_sequence":`+workflowInt(publicUpdate)+`}`, http.StatusCreated, nil)

	workflowJSON(t, server.URL, http.MethodPost, incidentURL+"/evidence", responder, `{"kind":"health_signal","repository_id":"`+repository.ID+`","resource_id":"`+failed.ID+`","event_sequence":`+workflowInt(failedSignal.Sequence)+`,"title":"Failed production readiness signal","audience":"participants"}`, http.StatusCreated, &incident)
	evidenceID := incident.Evidence[0].ID
	var delegated struct {
		Incident         incidents.Incident `json:"incident"`
		WorkerCredential string             `json:"worker_credential"`
	}
	workflowJSON(t, server.URL, http.MethodPost, incidentURL+"/investigations", commander, `{"agent":"codex","mandate":"Determine whether the release artifact explains the readiness failure without changing production.","evidence_ids":["`+evidenceID+`"],"revisions":[{"repository_id":"`+repository.ID+`","commit_id":"`+badPull.MergeCommitID+`"}],"operational_access":[{"repository_id":"`+repository.ID+`","kind":"health_signals","resource_id":"`+failed.ID+`"}]}`, http.StatusCreated, &delegated)
	workflowJSON(t, server.URL, http.MethodGet, "/incident-investigations/context", delegated.WorkerCredential, "", http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodGet, "/incident-investigations/operational/"+failed.ID, delegated.WorkerCredential, "", http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, "/incident-investigations/records", delegated.WorkerCredential, `{"type":"finding","message":"The deployed artifact contains the unhealthy marker introduced by the reviewed source revision.","evidence_ids":["`+evidenceID+`"]}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, incidentURL+"/investigations/"+delegated.Incident.Investigations[0].ID+"/control", responder, `{"action":"guide","message":"Confirm the known-good artifact is available for governed restore."}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, "/incident-investigations/records", delegated.WorkerCredential, `{"type":"finding","message":"The prior attested release remains available and is the lowest-risk recovery candidate.","evidence_ids":["`+evidenceID+`"]}`, http.StatusCreated, nil)

	workflowJSON(t, server.URL, http.MethodPost, incidentURL+"/mitigations", responder, `{"kind":"restore_release","title":"Restore the known-good release","description":"Roll back through ordinary deployment governance, preserving approval and health evidence.","repository_id":"`+repository.ID+`","environment_id":"`+environment.ID+`","deployment_id":"`+failed.ID+`","evidence_ids":["`+evidenceID+`"],"recovery_criteria":[{"name":"service-ready"}]}`, http.StatusCreated, &incident)
	mitigationID := incident.Mitigations[0].ID
	workflowJSON(t, server.URL, http.MethodPost, incidentURL+"/mitigations/"+mitigationID+"/decision", commander, `{"decision":"approve","reason":"Independent review confirms the immutable known-good artifact and bounded rollback."}`, http.StatusOK, nil)
	var rollback struct {
		Deployment  deployments.Deployment `json:"deployment"`
		KnownGoodID string                 `json:"known_good_deployment_id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/deployments/"+failed.ID+"/recovery", responder, `{"action":"rollback"}`, http.StatusCreated, &rollback)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/deployments/"+rollback.Deployment.ID+"/approvals", commander, `{}`, http.StatusOK, nil)
	restored := waitForDeployment(t, server.URL, repository.ID, rollback.Deployment.ID, commander, "succeeded")
	workflowJSON(t, server.URL, http.MethodPost, incidentURL+"/mitigations/"+mitigationID+"/execution", responder, `{"outcome":"started","resource_type":"deployment","resource_id":"`+restored.ID+`","message":"Governed rollback completed with independent approval."}`, http.StatusOK, nil)
	healthySignal := incidentHealthEvent(t, restored, "passed")
	workflowJSON(t, server.URL, http.MethodPost, incidentURL+"/mitigations/"+mitigationID+"/verification", commander, `{"results":[{"name":"service-ready","deployment_id":"`+restored.ID+`","event_sequence":`+workflowInt(healthySignal.Sequence)+`}]}`, http.StatusOK, &incident)
	if incident.Mitigations[0].State != "recovered" || restored.SourceCommitID != knownGoodCommit || restored.RecoveryOfID != failed.ID || rollback.KnownGoodID != v1Deployment.ID {
		t.Fatalf("recovery trail is incomplete: mitigation=%#v deployment=%#v", incident.Mitigations[0], restored)
	}

	workflowJSON(t, server.URL, http.MethodPost, incidentURL+"/findings", responder, `{"kind":"conclusion","body":"The release changed the packaged health marker; governed restore returned the declared signal to healthy.","query":"compare v1.0.0 and v1.1.0 health.txt","evidence_ids":["`+evidenceID+`"],"audience":"public"}`, http.StatusCreated, &incident)
	conclusionID := incident.Findings[len(incident.Findings)-1].ID
	due := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	workflowJSON(t, server.URL, http.MethodPost, incidentURL+"/resolution", commander, `{"impact_summary":"Production readiness was degraded during the v1.1 rollout.","timeline_summary":"Detection, diagnosis, independent rollback approval, and healthy verification remained in one incident record.","contributing_factors":["The service contract did not guard the packaged health marker."],"conclusion_ids":["`+conclusionID+`"],"proposal_title":"Prevent unhealthy release artifacts","proposal_body":"Carry the incident conclusion into a verified repository guard.","commitments":[{"title":"Strengthen the service contract","outcome":"The repository check rejects unhealthy packaged markers and documents the response.","owner_id":"responder","kind":"human","mandate":"Add a release guard and retain incident context in the runbook.","base_revision":"`+badPull.MergeCommitID+`","due_at":"`+due+`"}]}`, http.StatusCreated, &incident)
	if incident.Status != "resolved" || incident.Resolution == nil || len(incident.Resolution.CorrectiveWork) != 1 {
		t.Fatalf("incident resolution did not establish corrective ownership: %#v", incident)
	}
	corrective := incident.Resolution.CorrectiveWork[0]
	planBase := "/repositories/" + repository.ID + "/proposals/" + corrective.ProposalID + "/plan"
	var plan proposals.Plan
	workflowJSON(t, server.URL, http.MethodGet, planBase, responder, "", http.StatusOK, &plan)
	task := orchestrationTask(t, plan, corrective.TaskID)

	gitOutput(t, work, "fetch", "origin", "main")
	gitOutput(t, work, "switch", "-C", "corrective/health-guard", "origin/main")
	writeWorkflowFile(t, work, "health.txt", "healthy\n")
	writeWorkflowFile(t, work, "runbook.md", "# Service recovery\n\nRelease artifacts must retain the healthy service marker.\n")
	writeWorkflowFile(t, work, ".komodo/checks.json", `{"version":1,"checks":[{"name":"service-contract","command":"test -f runbook.md && grep -qx healthy health.txt"}]}`)
	gitOutput(t, work, "add", "health.txt", "runbook.md", ".komodo/checks.json")
	gitOutput(t, work, "commit", "-m", "Prevent unhealthy release artifacts")
	gitOutput(t, work, "push", "-u", "origin", "corrective/health-guard")
	var publication struct {
		Pull pullrequests.PullRequest `json:"pull_request"`
	}
	workflowJSON(t, server.URL, http.MethodPost, planBase+"/tasks/"+task.ID+"/contributions", responder, `{"expected_assignment_id":"`+task.Assignment.ID+`","title":"Strengthen the service contract","source_branch":"corrective/health-guard","target_branch":"main"}`, http.StatusCreated, &publication)
	pullBase := "/repositories/" + repository.ID + "/pull-requests/" + publication.Pull.ID
	waitForWorkflowCheck(t, server.URL, pullBase, responder, publication.Pull.SourceCommitID, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", commander, `{"decision":"approve"}`, http.StatusOK, nil)
	var merged pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", commander, `{}`, http.StatusOK, &merged)

	var v3 releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", commander, `{"version":"v1.1.1","commit_id":"`+merged.MergeCommitID+`","prior_release_id":"`+v2.ID+`","notes":"Incident corrective guard."}`, http.StatusCreated, &v3)
	v3Build, v3Artifact := waitForReleaseArtifact(t, server.URL, repository.ID, v3.ID, commander)
	correctedStart := promoteAndApprove(t, server.URL, repository.ID, responder, commander, environment.ID, v3.ID, v3Build.ID, v3Artifact.ID)
	corrected := waitForDeployment(t, server.URL, repository.ID, correctedStart.ID, commander, "succeeded")
	workflowJSON(t, server.URL, http.MethodPost, incidentURL+"/resolution/reconcile", commander, `{}`, http.StatusOK, &incident)
	tracked := incident.Resolution.CorrectiveWork[0]
	if !containsString(tracked.PullRequestIDs, merged.ID) || !containsString(tracked.ReleaseIDs, v3.ID) || !containsString(tracked.DeploymentIDs, corrected.ID) || tracked.State != "deployed:succeeded" || tracked.CheckState != string(checkruns.Succeeded) {
		t.Fatalf("incident lost corrective delivery provenance: %#v", tracked)
	}
	if len(incident.Investigations) != 1 || len(incident.Investigations[0].Records) != 3 || incident.Mitigations[0].Decisions[0].ActorID != "commander" || incident.Resolution.ResolvedByID != "commander" {
		t.Fatalf("continuous response attribution is incomplete: %#v", incident)
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/deployments/"+corrected.ID+"/control", delegated.WorkerCredential, `{"action":"pause","reason":"worker must not control production"}`, http.StatusUnauthorized, nil)
	var public incidents.Incident
	workflowJSON(t, server.URL, http.MethodGet, incidentURL, "", "", http.StatusOK, &public)
	if len(public.Investigations) != 0 || len(public.Evidence) != 0 || len(public.Findings) != 1 || public.Findings[0].Audience != "public" {
		t.Fatalf("public response view did not preserve audience boundaries: %#v", public)
	}
}

func incidentHealthEvent(t *testing.T, deployment deployments.Deployment, outcome string) deployments.Event {
	t.Helper()
	for _, event := range deployment.Events {
		if event.Type == "health.completed" && event.Outcome == outcome {
			return event
		}
	}
	t.Fatalf("deployment %s has no %s health event: %#v", deployment.ID, outcome, deployment.Events)
	return deployments.Event{}
}

func workflowInt(value int64) string { return strconv.FormatInt(value, 10) }
