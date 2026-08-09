package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityreports"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

// TestReportToDisclosureSecurityRemediationWorkflow proves that a report can
// cross private diagnosis, human and agent repair, exact-candidate verification,
// and coordinated disclosure without appearing on an ordinary public surface
// before both supported lines are safe to publish.
func TestReportToDisclosureSecurityRemediationWorkflow(t *testing.T) {
	requireGit(t)
	userStore, _ := users.New(t.TempDir())
	reporter, _ := userStore.Create(users.Profile{Handle: "outside-researcher", DisplayName: "Outside Researcher"})
	maintainer, _ := userStore.Create(users.Profile{Handle: "security-maintainer", DisplayName: "Security Maintainer"})
	responder, _ := userStore.Create(users.Profile{Handle: "release-responder", DisplayName: "Release Responder"})
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	reports, _ := securityreports.New(t.TempDir())
	activity, _ := activities.New(t.TempDir(), userStore)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerRepositoryBrowserHTTP(mux, catalog, credentials)
	registerGitHTTP(mux, catalog, credentials)
	registerActivitiesHTTP(mux, activity, catalog, credentials)
	registerSecurityReportsHTTP(mux, reports, catalog, userStore, credentials, activity)
	server := httptest.NewServer(mux)
	defer server.Close()

	reporterAPI := issueAccess(t, credentials, string(reporter.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerAPI := issueAccess(t, credentials, string(maintainer.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, string(maintainer.ID), auth.Git, auth.GitRead, auth.GitWrite)
	responderAPI := issueAccess(t, credentials, string(responder.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainerAPI, `{"name":"supported-parser","visibility":"public"}`, http.StatusCreated, &repository)
	if _, err := catalog.AddCollaborator(string(maintainer.ID), storage.ID(repository.ID), string(responder.ID)); err != nil {
		t.Fatal(err)
	}
	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		value.User = url.UserPassword("git", token)
		return value.String()
	}

	maintainerClone := gitClone(t, remote(maintainerGit))
	gitOutput(t, maintainerClone, "config", "user.name", "Security Maintainer")
	gitOutput(t, maintainerClone, "config", "user.email", "maintainer@example.test")
	writeWorkflowFile(t, maintainerClone, "parser.txt", "supported parser\n")
	gitOutput(t, maintainerClone, "add", "parser.txt")
	gitOutput(t, maintainerClone, "commit", "-m", "Establish supported parser lines")
	baseCommit := gitOutput(t, maintainerClone, "rev-parse", "HEAD")
	gitOutput(t, maintainerClone, "push", "-u", "origin", "main")
	gitOutput(t, maintainerClone, "branch", "support/1.x")
	gitOutput(t, maintainerClone, "push", "origin", "support/1.x")

	createBody := `{"title":"Confidential parser boundary flaw","summary":"A crafted input crosses a parser boundary.","contact":{"channel":"email","value":"researcher-private@example.test"},"affected_repositories":[{"repository_id":"` + repository.ID + `","versions":["1.x","2.x"]}],"evidence":[{"title":"Private reproducer","kind":"reproduction","description":"Confidential trigger bytes and observed corruption."}]}`
	var report securityreports.Report
	workflowJSON(t, server.URL, http.MethodPost, "/security-reports", reporterAPI, createBody, http.StatusCreated, &report)
	reportPath := "/security-reports/" + report.ID
	workflowJSON(t, server.URL, http.MethodPatch, reportPath+"/triage", maintainerAPI, `{"severity":"critical","embargo_state":"active"}`, http.StatusOK, &report)
	workflowJSON(t, server.URL, http.MethodPost, reportPath+"/team", maintainerAPI, `{"user_id":"`+string(responder.ID)+`"}`, http.StatusCreated, &report)
	workflowJSON(t, server.URL, http.MethodPost, reportPath+"/resources", responderAPI, `{"kind":"release_artifact","repository_id":"`+repository.ID+`","resource_id":"supported-artifacts","revision":"`+baseCommit+`","label":"Affected supported artifacts","details":"Both maintained release lines contain the parser boundary."}`, http.StatusCreated, &report)
	evidenceID := report.ResourceLinks[0].ID
	workflowJSON(t, server.URL, http.MethodPut, reportPath+"/impact", responderAPI, `{"repository_id":"`+repository.ID+`","version":"1.x","environment":"production","state":"confirmed","rationale":"Attested 1.x artifacts contain the affected parser.","evidence_ids":["`+evidenceID+`"]}`, http.StatusOK, &report)
	workflowJSON(t, server.URL, http.MethodPut, reportPath+"/impact", responderAPI, `{"repository_id":"`+repository.ID+`","version":"2.x","environment":"production","state":"confirmed","rationale":"Attested 2.x artifacts contain the affected parser.","evidence_ids":["`+evidenceID+`"]}`, http.StatusOK, &report)

	var investigation struct {
		Report           securityreports.Report `json:"report"`
		WorkerCredential string                 `json:"worker_credential"`
	}
	workflowJSON(t, server.URL, http.MethodPost, reportPath+"/investigations", responderAPI, `{"agent":"codex","mandate":"Assess affected supported lines from selected release evidence.","evidence_ids":["`+evidenceID+`"]}`, http.StatusCreated, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, "/security-investigations/records", investigation.WorkerCredential, `{"type":"finding","body":"Both maintained lines share the affected boundary and need independent fixes.","uncertainty":"Downstream repackaging remains outside the selected evidence.","evidence_ids":["`+evidenceID+`"]}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, reportPath+"/investigations/"+investigation.Report.Investigations[0].ID+"/control", responderAPI, `{"action":"cancel","message":"Supported-line impact is established; revoke the bounded credential."}`, http.StatusOK, &report)

	createRepair := func(version string, dependencies []string) securityreports.RepairTask {
		dependencyJSON, _ := json.Marshal(dependencies)
		var created struct {
			Repair securityreports.RepairTask `json:"repair"`
		}
		body := `{"repository_id":"` + repository.ID + `","version":"` + version + `","outcome":"Reject the confidential boundary trigger on ` + version + `.","base_revision":"` + baseCommit + `","dependency_ids":` + string(dependencyJSON) + `}`
		workflowJSON(t, server.URL, http.MethodPost, reportPath+"/repairs", maintainerAPI, body, http.StatusCreated, &created)
		return created.Repair
	}
	lineOne := createRepair("1.x", nil)
	lineTwo := createRepair("2.x", []string{lineOne.ID})

	type repairSession struct {
		Session       securityreports.RepairSession `json:"session"`
		GitCredential string                        `json:"git_credential"`
	}
	startRepair := func(task securityreports.RepairTask, kind, assignee string) repairSession {
		var started repairSession
		body := `{"kind":"` + kind + `","assignee_id":"` + assignee + `","mandate":"Implement only the bounded repair for ` + task.Version + `."}`
		workflowJSON(t, server.URL, http.MethodPost, reportPath+"/repairs/"+task.ID+"/sessions", maintainerAPI, body, http.StatusCreated, &started)
		return started
	}
	human := startRepair(lineOne, "human", string(responder.ID))
	agent := startRepair(lineTwo, "agent", "codex")
	if human.GitCredential == "" || agent.GitCredential == "" || human.GitCredential == agent.GitCredential {
		t.Fatal("repair sessions did not receive distinct one-time Git credentials")
	}

	pushRepair := func(task securityreports.RepairTask, session repairSession, author, email string) string {
		clone := gitClone(t, remote(maintainerGit))
		gitOutput(t, clone, "config", "user.name", author)
		gitOutput(t, clone, "config", "user.email", email)
		writeWorkflowFile(t, clone, "parser.txt", "supported parser with private boundary guard for "+task.Version+"\n")
		gitOutput(t, clone, "add", "parser.txt")
		gitOutput(t, clone, "commit", "-m", "Harden supported parser line")
		commit := gitOutput(t, clone, "rev-parse", "HEAD")
		gitOutput(t, clone, "remote", "set-url", "origin", remote(session.GitCredential))
		gitOutput(t, clone, "push", "origin", "HEAD:"+task.Branch)
		return commit
	}
	lineOneCommit := pushRepair(lineOne, human, "Release Responder", "responder@example.test")
	lineTwoCommit := pushRepair(lineTwo, agent, "Codex Security Agent", "codex@example.test")
	recordRepair := func(task securityreports.RepairTask, session repairSession, revision, decision string) {
		path := reportPath + "/repairs/" + task.ID + "/sessions/" + session.Session.ID + "/records"
		workflowJSON(t, server.URL, http.MethodPost, path, maintainerAPI, `{"type":"branch_update","body":"Bounded fix published to the private repair ref.","revision":"`+revision+`"}`, http.StatusCreated, &report)
		workflowJSON(t, server.URL, http.MethodPost, path, responderAPI, `{"type":"review","body":"Exact candidate removes the boundary flaw on this supported line.","revision":"`+revision+`","decision":"`+decision+`"}`, http.StatusCreated, &report)
	}
	recordRepair(lineOne, human, lineOneCommit, "approve")
	recordRepair(lineTwo, agent, lineTwoCommit, "approve")

	// None of the confidential title, refs, commits, or advisory may be exposed
	// by the ordinary collaboration surfaces while verification is in progress.
	workflowJSON(t, server.URL, http.MethodGet, "/security-advisories/KSA-2026-0001", "", "", http.StatusNotFound, nil)
	if advertised := gitOutput(t, maintainerClone, "ls-remote", remote(maintainerGit)); strings.Contains(advertised, "embargo/") || strings.Contains(advertised, lineOneCommit) || strings.Contains(advertised, lineTwoCommit) {
		t.Fatalf("pre-disclosure Git advertisement leaked repair state: %s", advertised)
	}
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+repository.ID+"/commits/"+lineOneCommit, "", "", http.StatusNotFound, nil)
	var before struct {
		Items []activities.Event `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+repository.ID+"/activity", "", "", http.StatusOK, &before)
	for _, event := range before.Items {
		if strings.Contains(event.Type, "security") || strings.Contains(event.Resource.ID, report.ID) {
			t.Fatalf("ordinary activity leaked the embargoed report: %#v", event)
		}
	}

	verify := func(task securityreports.RepairTask, revision, suffix string) {
		path := reportPath + "/repairs/" + task.ID + "/verification"
		actions := []string{
			`{"action":"begin","revision":"` + revision + `"}`,
			`{"action":"gate","revision":"` + revision + `","kind":"required_check","name":"supported-line suite","attempt_id":"check-` + suffix + `","definition_digest":"` + strings.Repeat("1", 64) + `","state":"passed"}`,
			`{"action":"gate","revision":"` + revision + `","kind":"security_reproduction","name":"private boundary regression","attempt_id":"repro-` + suffix + `","definition_digest":"` + strings.Repeat("2", 64) + `","state":"passed"}`,
			`{"action":"approve","revision":"` + revision + `","decision":"approve","summary":"Maintainer reviewed the exact candidate and safe evidence."}`,
			`{"action":"integrate","revision":"` + revision + `","integration_entry_id":"queue-` + suffix + `","integration_commit_id":"` + revision + `"}`,
			`{"action":"attest","revision":"` + revision + `","release_id":"release-` + suffix + `","version":"` + task.Version + `","artifact_id":"artifact-` + suffix + `","artifact_sha256":"` + strings.Repeat(suffix, 64) + `"}`,
		}
		for _, action := range actions {
			workflowJSON(t, server.URL, http.MethodPost, path, maintainerAPI, action, http.StatusOK, &report)
		}
	}
	verify(lineOne, lineOneCommit, "a")
	verify(lineTwo, lineTwoCommit, "b")
	publishedRefs := map[string]string{lineOne.ID: "refs/heads/security/1.x-fixed", lineTwo.ID: "refs/heads/security/2.x-fixed"}
	publishedJSON, _ := json.Marshal(publishedRefs)
	prepare := `{"advisory_id":"KSA-2026-0001","summary":"Parser boundary validation was incomplete in supported releases.","upgrade_guidance":"Upgrade 1.x to release-a and 2.x to release-b immediately.","credits":["Outside Researcher","Security Maintainer","Release Responder","Codex"],"published_refs":` + string(publishedJSON) + `}`
	workflowJSON(t, server.URL, http.MethodPost, reportPath+"/disclosure", maintainerAPI, prepare, http.StatusCreated, &report)
	workflowJSON(t, server.URL, http.MethodPost, reportPath+"/disclosure/publish", maintainerAPI, `{}`, http.StatusOK, &report)

	if report.EmbargoState != "lifted" || report.Disclosure == nil || report.Disclosure.State != "published" || len(report.Disclosure.Branches) != 2 || len(report.Audit) < 20 {
		t.Fatalf("private evidence trail did not reach complete disclosure: %#v", report)
	}
	var advisory securityreports.Report
	workflowJSON(t, server.URL, http.MethodGet, "/security-advisories/KSA-2026-0001", "", "", http.StatusOK, &advisory)
	encoded, _ := json.Marshal(advisory)
	for _, secret := range []string{"researcher-private@example.test", "Confidential trigger bytes", "embargo/", report.ID, evidenceID} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("public advisory leaked private value %q: %s", secret, encoded)
		}
	}
	if advisory.Disclosure == nil || len(advisory.Disclosure.Branches) != 2 || advisory.Disclosure.UpgradeGuidance == "" || len(advisory.Disclosure.Credits) != 4 {
		t.Fatalf("public advisory is not actionable and attributable: %#v", advisory)
	}
	advertised := gitOutput(t, maintainerClone, "ls-remote", remote(maintainerGit))
	for _, publicRef := range publishedRefs {
		if !strings.Contains(advertised, publicRef) {
			t.Fatalf("published fix ref %s is not available to users: %s", publicRef, advertised)
		}
	}
	var after struct {
		Items []activities.Event `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+repository.ID+"/activity", "", "", http.StatusOK, &after)
	foundUpgrade := false
	for _, event := range after.Items {
		foundUpgrade = foundUpgrade || event.Type == "security_advisory.published" && event.Metadata["upgrade_guidance"] != ""
	}
	if !foundUpgrade {
		t.Fatalf("affected collaborators did not receive public upgrade activity: %#v", after.Items)
	}
}
