package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/docscollections"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/supportquestions"
)

// TestDeveloperSupportWorkflow is the black-box boundary for the complete
// package-question-to-tested-guidance-or-improved-product loop. It composes the
// public support API with stock Git and ordinary proposal, check, review,
// merge, and release authority while retaining corrections and attribution.
func TestDeveloperSupportWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for support verification")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	packages, _ := packagecatalog.New(t.TempDir())
	documentation, _ := docscollections.New(t.TempDir())
	issueStore, _ := issues.New(t.TempDir())
	support, _ := supportquestions.New(t.TempDir())
	checkRunner := checkruns.NewRunner(checks, catalog)
	verificationRunner := supportquestions.NewVerificationRunner(support, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, nil, checkRunner, checks)
	registerCheckRunsHTTP(mux, checks, checkRunner, pulls, catalog, credentials, nil, nil)
	registerReleasesHTTP(mux, releaseStore, checks, checkRunner, pulls, catalog, credentials)
	registerGitHTTP(mux, catalog, credentials)
	registerSupportQuestionsHTTP(mux, support, catalog, credentials, supportSources{releases: releaseStore, packages: packages, docs: documentation, issues: issueStore, proposals: plans, docsTasks: documentation}, verificationRunner)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	asker := issueAccess(t, credentials, "package-user", auth.API, auth.RepositoryRead)
	agent := issueAccess(t, credentials, "support-agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	agentGit := issueAccess(t, credentials, "support-agent", auth.Git, auth.GitRead, auth.GitWrite)
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"integrated-sdk","visibility":"public"}`, http.StatusCreated, &repository)
	if _, err := catalog.AddCollaborator("maintainer", repository.ID, "support-agent"); err != nil {
		t.Fatal(err)
	}
	base := "/repositories/" + string(repository.ID)
	workflowJSON(t, server.URL, http.MethodPut, base+"/required-checks", owner, `{"branch":"main","checks":["integration"]}`, http.StatusOK, nil)
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + base)
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	work := gitClone(t, remote(ownerGit))
	gitOutput(t, work, "config", "user.name", "Maintainer")
	gitOutput(t, work, "config", "user.email", "maintainer@example.test")
	writeWorkflowFile(t, work, "docs/integration.md", "# Integration\n\nSet `SDK_MODE=bounded` before invoking the client.\n")
	writeWorkflowFile(t, work, "sdk/client.sh", "#!/bin/sh\ntest \"$SDK_MODE\" = bounded\n")
	writeWorkflowFile(t, work, ".komodo/checks.json", `{"version":1,"checks":[{"name":"integration","command":"grep -q SDK_MODE docs/integration.md && grep -q SDK_MODE sdk/client.sh"}]}`)
	writeWorkflowFile(t, work, ".komodo/releases.json", `{"version":1,"builds":[{"name":"sdk","command":"mkdir -p dist; cp sdk/client.sh dist/client.sh","artifacts":["dist/client.sh"]}]}`)
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Release the bounded SDK integration")
	initialRevision := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "main")
	var initialRelease releases.Release
	workflowJSON(t, server.URL, http.MethodPost, base+"/releases", owner, `{"version":"v1.0.0","commit_id":"`+initialRevision+`","notes":"Initial SDK"}`, http.StatusCreated, &initialRelease)
	artifact := []byte("sdk package")
	digest := sha256.Sum256(artifact)
	pkg, err := packages.Publish(packagecatalog.PublishParams{OwnerID: "maintainer", Name: "sdk", Version: "1.0.0", RepositoryID: string(repository.ID), ReleaseID: initialRelease.ID, SourceCommitID: initialRevision, ArtifactID: "sdk-artifact", ArtifactPath: "sdk.tgz", ArtifactMediaType: "application/gzip", ArtifactSize: int64(len(artifact)), ExpectedSHA256: hex.EncodeToString(digest[:]), Build: packagecatalog.BuildAttestation{RunID: "release-build", BuildName: "sdk", CompletedAt: time.Now()}, Platform: packagecatalog.Platform{OS: "linux", Arch: "amd64"}, PublisherID: "maintainer", Visibility: "public"}, bytes.NewReader(artifact))
	if err != nil {
		t.Fatal(err)
	}

	questionBody := `{"title":"How do I initialize the SDK?","question":"The client exits before the first request.","subject":{"kind":"package","resource_id":"` + pkg.ID + `"},"software_version":"1.0.0","environment":"linux shell","goal":"initialize the SDK client","attempted_steps":["invoke sdk/client.sh"],"urgency":"normal","audience":"public","contact":{"preference":"thread"},"evidence":[{"kind":"log","name":"private.log","media_type":"text/plain","content":"dG9rZW49cHJpdmF0ZQ==","visibility":"maintainers"}]}`
	var first supportquestions.Question
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions", asker, questionBody, http.StatusCreated, &first)
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+first.ID+"/comments", owner, `{"body":"Is SDK_MODE set before the process starts?"}`, http.StatusCreated, &first)
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+first.ID+"/comments", asker, `{"body":"No; that missing initialization step is the difference."}`, http.StatusCreated, &first)
	answerBody := func(answer, supersedes, command, summary string) string {
		return `{"answer_id":"` + answer + `","supersedes_id":"` + supersedes + `","author_kind":"agent","summary":"` + summary + `","instructions":["` + command + `"],"applicable_versions":["1.0.0"],"uncertainty":"Only the released Linux shell integration is covered.","claims":[{"text":"SDK_MODE is required before client initialization.","mode":"verified","citations":[{"kind":"source","revision":"` + initialRevision + `","path":"docs/integration.md","line_start":3,"line_end":3,"label":"released integration guide","visibility":"public"}]}]}`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+first.ID+"/answers", agent, answerBody("", "", "grep -qx missing docs/integration.md", "Set the required mode"), http.StatusCreated, &first)
	answerID, staleRevision := first.Answers[0].ID, first.Answers[0].CurrentID
	failed := launchSupportVerification(t, server.URL, base, first.ID, answerID, staleRevision, initialRevision, "1.0.0", asker)
	if failed.Attempt.State != "failed" {
		t.Fatalf("bad advice did not fail cleanly: %#v", failed)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+first.ID+"/answers/"+answerID+"/feedback", owner, `{"revision_id":"`+staleRevision+`","kind":"challenge","body":"The command tests a marker that the released guide never contains."}`, http.StatusCreated, &first)
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+first.ID+"/answers", agent, answerBody(answerID, staleRevision, "grep -q SDK_MODE docs/integration.md", "Export the required mode before initialization"), http.StatusCreated, &first)
	currentRevision := first.Answers[0].CurrentID
	workflowJSON(t, server.URL, http.MethodGet, base+"/support-questions/"+first.ID+"/verifications/"+failed.Attempt.ID, asker, "", http.StatusOK, &failed)
	passed := launchSupportVerification(t, server.URL, base, first.ID, answerID, currentRevision, initialRevision, "1.0.0", owner)
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+first.ID+"/solutions", asker, `{"answer_id":"`+answerID+`","answer_revision_id":"`+currentRevision+`","verification_id":"`+passed.Attempt.ID+`","title":"Initialize the SDK client","summary":"Export SDK_MODE=bounded before starting the client.","applicable_versions":["1.0.0"],"limitations":["Linux shell only"],"audience":"public","links":[{"kind":"package","resource_id":"`+pkg.ID+`","label":"SDK 1.0.0"},{"kind":"release","resource_id":"`+initialRelease.ID+`","label":"v1.0.0"}]}`, http.StatusCreated, &first)

	// A duplicate remains attributable, but maintainers merge it into the tested
	// solution rather than allowing two diverging search results.
	var duplicate supportquestions.Question
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions", asker, strings.Replace(questionBody, "How do I initialize the SDK?", "SDK initialization exits early", 1), http.StatusCreated, &duplicate)
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+duplicate.ID+"/answers", agent, answerBody("", "", "grep -q SDK_MODE docs/integration.md", "Use the existing initialization solution"), http.StatusCreated, &duplicate)
	dupVerification := launchSupportVerification(t, server.URL, base, duplicate.ID, duplicate.Answers[0].ID, duplicate.Answers[0].CurrentID, initialRevision, "1.0.0", owner)
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+duplicate.ID+"/solutions", owner, `{"answer_id":"`+duplicate.Answers[0].ID+`","answer_revision_id":"`+duplicate.Answers[0].CurrentID+`","verification_id":"`+dupVerification.Attempt.ID+`","title":"Duplicate SDK initialization","summary":"Same tested mode requirement.","applicable_versions":["1.0.0"],"audience":"public"}`, http.StatusCreated, &duplicate)
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+duplicate.ID+"/solutions/"+duplicate.Solutions[0].ID+"/events", owner, `{"type":"merge","reason":"Same version and integration goal","target_question_id":"`+first.ID+`","target_solution_id":"`+first.Solutions[0].ID+`"}`, http.StatusCreated, &duplicate)

	// A distinct request proves a real code and documentation gap, then carries
	// the user's need into ordered human-agent work and ordinary delivery gates.
	var gap supportquestions.Question
	gapBody := `{"title":"How can I inspect the selected SDK mode?","question":"There is no diagnostic command or documented example.","subject":{"kind":"package","resource_id":"` + pkg.ID + `"},"software_version":"1.0.0","environment":"linux shell","goal":"print the effective mode during integration debugging","attempted_steps":["run sdk/client.sh --show-mode"],"urgency":"normal","audience":"public","contact":{"preference":"thread"}}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions", asker, gapBody, http.StatusCreated, &gap)
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+gap.ID+"/comments", owner, `{"body":"The released client has no diagnostic flag; this needs code and guidance together."}`, http.StatusCreated, &gap)
	commentID := gap.Discussion[0].ID
	var improved struct {
		Question    supportquestions.Question    `json:"question"`
		Improvement supportquestions.Improvement `json:"improvement"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+gap.ID+"/improvements", owner, `{"classification":"missing_example","target_kind":"proposal","title":"Add SDK mode diagnostics and guidance","acceptance_criteria":["--show-mode prints the effective mode","the integration guide includes a tested example"],"discussion_ids":["`+commentID+`"],"work":[{"title":"Implement diagnostic mode","owner_kind":"agent","owner_id":"support-agent","acceptance_criteria":["integration check passes"]},{"title":"Review the developer guidance","owner_kind":"human","owner_id":"maintainer","acceptance_criteria":["example is version-bound"],"depends_on":[1]}]}`, http.StatusCreated, &improved)
	plan, _ := plans.GetPlan(string(repository.ID), improved.Improvement.TargetID)
	if len(plan.Tasks) != 2 || plan.Tasks[0].OwnerKind != "agent" || plan.Tasks[1].OwnerKind != "human" || plan.Tasks[1].DependsOn[0] != plan.Tasks[0].ID {
		t.Fatalf("connected human-agent work missing: %#v", plan.Tasks)
	}
	agentWork := gitClone(t, remote(agentGit))
	gitOutput(t, agentWork, "config", "user.name", "Support Agent")
	gitOutput(t, agentWork, "config", "user.email", "agent@example.test")
	gitOutput(t, agentWork, "switch", "-c", "support/show-mode")
	writeWorkflowFile(t, agentWork, "sdk/client.sh", "#!/bin/sh\nif test \"$1\" = --show-mode; then printf '%s\\n' \"${SDK_MODE:-unset}\"; exit; fi\ntest \"$SDK_MODE\" = bounded\n")
	writeWorkflowFile(t, agentWork, "docs/integration.md", "# Integration\n\nSet `SDK_MODE=bounded` before invoking the client. Diagnose it with `SDK_MODE=bounded sh sdk/client.sh --show-mode`.\n")
	writeWorkflowFile(t, agentWork, ".komodo/checks.json", `{"version":1,"checks":[{"name":"integration","command":"test \"$(SDK_MODE=bounded sh sdk/client.sh --show-mode)\" = bounded && grep -q -- --show-mode docs/integration.md"}]}`)
	gitOutput(t, agentWork, "add", ".")
	gitOutput(t, agentWork, "commit", "-m", "Add SDK mode diagnostics and guidance")
	candidate := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push", "-u", "origin", "support/show-mode")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, base+"/pull-requests", agent, `{"title":"Add SDK mode diagnostics","body":"Implements support proposal `+improved.Improvement.TargetID+`.","source_branch":"support/show-mode","target_branch":"main","proposal_id":"`+improved.Improvement.TargetID+`"}`, http.StatusCreated, &pull)
	pullBase := base + "/pull-requests/" + pull.ID
	check := waitForWorkflowCheck(t, server.URL, pullBase, owner, candidate, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", owner, `{}`, http.StatusOK, &pull)
	var fixedRelease releases.Release
	workflowJSON(t, server.URL, http.MethodPost, base+"/releases", owner, `{"version":"v1.1.0","commit_id":"`+pull.MergeCommitID+`","prior_release_id":"`+initialRelease.ID+`","notes":"SDK mode diagnostics and tested guidance"}`, http.StatusCreated, &fixedRelease)
	for _, link := range []string{
		`{"kind":"pull_request","resource_id":"` + pull.ID + `","state":"succeeded","revision":"` + pull.MergeCommitID + `","summary":"Reviewed and merged"}`,
		`{"kind":"check","resource_id":"` + check.ID + `","state":"succeeded","revision":"` + candidate + `","summary":"Integration example passed"}`,
		`{"kind":"release","resource_id":"` + fixedRelease.ID + `","state":"published","revision":"` + pull.MergeCommitID + `","summary":"Shipped in v1.1.0"}`,
	} {
		workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+gap.ID+"/improvements/"+improved.Improvement.ID+"/links", owner, link, http.StatusCreated, &gap)
	}
	updatedAnswer := `{"author_kind":"agent","summary":"Use the shipped diagnostic flag","instructions":["SDK_MODE=bounded sh sdk/client.sh --show-mode"],"applicable_versions":["1.1.0"],"uncertainty":"Only the released shell client is covered.","claims":[{"text":"The diagnostic prints the effective SDK mode.","mode":"verified","citations":[{"kind":"source","revision":"` + pull.MergeCommitID + `","path":"docs/integration.md","line_start":3,"line_end":3,"label":"released diagnostic example","visibility":"public"},{"kind":"release","resource_id":"` + fixedRelease.ID + `","revision":"v1.1.0","label":"shipped release","visibility":"public"}]}]}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+gap.ID+"/answers", agent, updatedAnswer, http.StatusCreated, &gap)
	finalVerification := launchSupportVerification(t, server.URL, base, gap.ID, gap.Answers[0].ID, gap.Answers[0].CurrentID, pull.MergeCommitID, "1.1.0", asker)
	workflowJSON(t, server.URL, http.MethodPost, base+"/support-questions/"+gap.ID+"/solutions", owner, `{"answer_id":"`+gap.Answers[0].ID+`","answer_revision_id":"`+gap.Answers[0].CurrentID+`","verification_id":"`+finalVerification.Attempt.ID+`","title":"Inspect the effective SDK mode","summary":"Use the released --show-mode diagnostic.","applicable_versions":["1.1.0"],"audience":"public","links":[{"kind":"release","resource_id":"`+fixedRelease.ID+`","label":"v1.1.0"}]}`, http.StatusCreated, &gap)
	workflowJSON(t, server.URL, http.MethodGet, base+"/support-questions/"+gap.ID, asker, "", http.StatusOK, &gap)

	var anonymous supportquestions.Question
	workflowJSON(t, server.URL, http.MethodGet, base+"/support-questions/"+first.ID, "", "", http.StatusOK, &anonymous)
	if anonymous.Evidence[0].Content != "" || len(first.Answers[0].Revisions) != 2 || failed.Attempt.State != "failed" || !failed.Stale || passed.Attempt.State != "passed" || duplicate.Solutions[0].Status != "merged" || len(gap.Improvements[0].Links) != 3 || gap.Solutions[0].Notifications[0].Recipient != "package-user" {
		t.Fatalf("support trail lost privacy, correction, delivery, or notification: private=%q revisions=%d failed=%#v passed=%#v duplicate=%s improvement=%#v notifications=%#v", anonymous.Evidence[0].Content, len(first.Answers[0].Revisions), failed, passed, duplicate.Solutions[0].Status, gap.Improvements, gap.Solutions[0].Notifications)
	}
}

type supportWorkflowVerification struct {
	Attempt supportquestions.VerificationAttempt `json:"attempt"`
	Stale   bool                                 `json:"stale"`
}

func launchSupportVerification(t *testing.T, origin, base, question, answer, revision, source, version, token string) supportWorkflowVerification {
	t.Helper()
	body := `{"answer_id":"` + answer + `","answer_revision_id":"` + revision + `","source_revision":"` + source + `","software_version":"` + version + `","environment":{"name":"linux shell","image_digest":"sha256:clean-linux","tools":["sh","grep"],"resources":{"cpu_seconds":10,"memory_mb":128,"disk_mb":128}},"dependencies":{"sdk":"` + version + `"},"sanitized_inputs":[],"cost_units":0.25}`
	var launched supportWorkflowVerification
	workflowJSON(t, origin, http.MethodPost, base+"/support-questions/"+question+"/verifications", token, body, http.StatusCreated, &launched)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		workflowJSON(t, origin, http.MethodGet, base+"/support-questions/"+question+"/verifications/"+launched.Attempt.ID, token, "", http.StatusOK, &launched)
		if launched.Attempt.State == "passed" || launched.Attempt.State == "failed" {
			return launched
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("support verification %s did not finish: %#v", launched.Attempt.ID, launched)
	return launched
}
