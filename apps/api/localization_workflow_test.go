package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/localeplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/localizationdelivery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/localizationverification"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/translationunits"
)

type localizedPreviewSource struct{ items map[string]previews.Preview }

func (s localizedPreviewSource) Get(repo, pull, id string) (previews.Preview, error) {
	p, ok := s.items[id]
	if !ok || p.RepositoryID != repo || p.PullRequestID != pull {
		return previews.Preview{}, previews.ErrNotFound
	}
	return p, nil
}

// TestSourceChangeToGlobalReleaseWorkflow is the black-box boundary for the
// source, linguistic ownership, exact-preview, delivery, and regional repair
// contracts. Ordinary Git, checks, review, merge, and release remain the only
// route by which translated product work becomes published source.
func TestSourceChangeToGlobalReleaseWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the localization workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	proposalStore, _ := proposals.New(t.TempDir())
	pullStore, _ := pullrequests.New(t.TempDir())
	checkStore, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	planStore, _ := localeplans.New(t.TempDir())
	unitStore, _ := translationunits.New(t.TempDir())
	verificationStore, _ := localizationverification.New(t.TempDir())
	deliveryStore, _ := localizationdelivery.New(t.TempDir())
	runner := checkruns.NewRunner(checkStore, catalog)
	previewSource := localizedPreviewSource{items: map[string]previews.Preview{}}
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, proposalStore, catalog, credentials)
	registerPullRequestsHTTP(mux, pullStore, proposalStore, catalog, credentials, nil, runner, checkStore, nil, nil, nil, nil, nil, nil, deliveryStore, verificationStore)
	registerCheckRunsHTTP(mux, checkStore, runner, pullStore, catalog, credentials, nil, nil)
	registerReleasesHTTP(mux, releaseStore, checkStore, runner, pullStore, catalog, credentials)
	registerLocalePlansHTTP(mux, planStore, catalog, credentials)
	registerTranslationUnitsHTTP(mux, unitStore, catalog, credentials, translationUnitSources{pulls: pullStore, repositories: catalog, plans: planStore})
	registerLocalizationVerificationHTTP(mux, verificationStore, catalog, credentials, localizationVerificationSources{pulls: pullStore, repositories: catalog, translations: unitStore, previews: previewSource})
	registerLocalizationDeliveryHTTP(mux, deliveryStore, catalog, credentials, localizationDeliverySources{pulls: pullStore, verification: verificationStore, releases: releaseStore, proposals: proposalStore})
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "owner", auth.Git, auth.GitRead, auth.GitWrite)
	developer := issueAccess(t, credentials, "developer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	developerGit := issueAccess(t, credentials, "developer", auth.Git, auth.GitRead, auth.GitWrite)
	translator := issueAccess(t, credentials, "translator", auth.API, auth.RepositoryRead)
	frenchReviewer := issueAccess(t, credentials, "reviewer-fr", auth.API, auth.RepositoryRead)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	agent := issueAccess(t, credentials, "codex", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agentGit := issueAccess(t, credentials, "codex", auth.Git, auth.GitRead, auth.GitWrite)
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"global-checkout","visibility":"public"}`, 201, &repository)
	for _, id := range []string{"developer", "translator", "reviewer-fr", "codex"} {
		if _, err := catalog.AddCollaborator("owner", repository.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+string(repository.ID)+"/required-checks", owner, `{"branch":"main","checks":["product"]}`, 200, nil)
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	base := gitClone(t, remote(ownerGit))
	gitOutput(t, base, "config", "user.name", "Owner")
	gitOutput(t, base, "config", "user.email", "owner@example.test")
	writeWorkflowFile(t, base, "app/en-US.json", `{"checkout":"Checkout"}`+"\n")
	writeWorkflowFile(t, base, "app/fr-FR.json", `{"checkout":"Paiement"}`+"\n")
	writeWorkflowFile(t, base, "app/ar.json", `{"checkout":"الدفع"}`+"\n")
	writeLocalizationManifests(t, base)
	gitOutput(t, base, "add", ".")
	gitOutput(t, base, "commit", "-m", "Establish localized checkout")
	baseRevision := gitOutput(t, base, "rev-parse", "HEAD")
	gitOutput(t, base, "push", "-u", "origin", "main")

	var plan localeplans.Plan
	planBody := `{"title":"Global checkout","scopes":[{"kind":"product","resource_id":"checkout","name":"Checkout"}],"locales":[{"id":"en-US","language":"English"},{"id":"fr-FR","language":"French","region":"France","fallback_locale_ids":["en-US"]},{"id":"ar","language":"Arabic","fallback_locale_ids":["en-US"]}],"terminology":[{"concept":"checkout","locale_id":"fr-FR","preferred":"Paiement"}],"formatting_requirements":[{"id":"json","description":"JSON messages","supported":true}],"journeys":[{"id":"buy","name":"Complete checkout","locale_ids":["fr-FR","ar"],"owner_ids":["translator"],"reviewer_ids":["reviewer-fr"]}],"owner_ids":["owner"],"reviewer_ids":[],"release_thresholds":[{"locale_id":"fr-FR","minimum_percent":100,"required_journey_ids":["buy"],"required_format_ids":["json"]},{"locale_id":"ar","minimum_percent":100,"required_journey_ids":["buy"],"required_format_ids":["json"]}],"resources":[{"id":"checkout","kind":"product","path":"app/en-US.json","source_revision":"` + baseRevision + `","format_id":"json","journey_ids":["buy"],"owner_ids":["translator"]}],"change_reason":"Freeze linguistic ownership"}`
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/locale-plans", owner, planBody, 201, &plan)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/localization-delivery-policies", owner, `{"name":"Checkout locales","target_branches":["main"],"paths":["app/**"],"locales":[{"locale_id":"fr-FR","audiences":["customers"],"risk_classes":["purchase"],"minimum_coverage":100,"required_checks":["fr/journey"],"required_reviewer_ids":["reviewer-fr"]},{"locale_id":"ar","audiences":["customers"],"risk_classes":["purchase"],"minimum_coverage":100,"required_checks":["ar/rtl"],"required_reviewer_ids":["reviewer-ar"]}]}`, 201, nil)

	work := gitClone(t, remote(developerGit))
	gitOutput(t, work, "config", "user.name", "Developer")
	gitOutput(t, work, "config", "user.email", "developer@example.test")
	gitOutput(t, work, "switch", "-c", "product/express-checkout")
	writeWorkflowFile(t, work, "app/en-US.json", `{"checkout":"Buy now"}`+"\n")
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Introduce express checkout copy")
	firstRevision := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "product/express-checkout")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests", developer, `{"title":"Localize express checkout","body":"Adapt the purchase action for supported regions.","source_branch":"product/express-checkout","target_branch":"main"}`, 201, &pull)
	pullBase := "/repositories/" + string(repository.ID) + "/pull-requests/" + pull.ID
	var first translationunits.Extraction
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/translation-units/extract", developer, `{"revision":"`+firstRevision+`","locale_plan_id":"`+plan.ID+`","locale_plan_version":1}`, 201, &first)
	unitID := first.Units[0].ID
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/translation-units/"+unitID+"/proposals", translator, `{"locale_id":"fr-FR","text":"Acheter"}`, 201, &first)

	// Product review evolves the source after translation has begun. Re-extraction
	// keeps the old proposal but makes it unusable for the newer exact candidate.
	writeWorkflowFile(t, work, "app/en-US.json", `{"checkout":"Buy securely now"}`+"\n")
	gitOutput(t, work, "add", "app/en-US.json")
	gitOutput(t, work, "commit", "-m", "Clarify checkout security")
	currentRevision := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "origin", "product/express-checkout")
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/synchronize", developer, `{}`, 200, &pull)
	var current translationunits.Extraction
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/translation-units/extract", developer, `{"revision":"`+currentRevision+`","locale_plan_id":"`+plan.ID+`","locale_plan_version":1}`, 201, &current)
	if len(current.Proposals) != 1 || !current.Proposals[0].Superseded {
		t.Fatalf("changed source reused linguistic evidence: %#v", current.Proposals)
	}
	unitID = current.Units[0].ID
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/translation-units/claims", translator, `{"locale_id":"fr-FR","action":"claim","reason":"Own the French purchase wording","expected_version":0}`, 201, &current)
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/translation-units/"+unitID+"/proposals", translator, `{"locale_id":"fr-FR","text":"Acheter en toute sécurité"}`, 201, &current)
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/translation-units/"+unitID+"/suggestions", developer, `{"locale_id":"ar","agent_id":"codex","text":"اشتر الآن بأمان","uncertainty":"Bidirectional SKU placement still needs regional preview.","evidence":[{"kind":"source","reference":"app/en-US.json@`+currentRevision+`","summary":"Exact source wording"},{"kind":"terminology","reference":"locale-plan:`+plan.ID+`@1","summary":"Frozen linguistic contract"}]}`, 201, &current)
	suggestionID := current.Suggestions[0].ID
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/translation-units/suggestions/"+suggestionID+"/decisions", translator, `{"decision":"edit","text":"اشتر بأمان الآن","rationale":"Natural regional word order; retain explicit uncertainty."}`, 201, &current)

	failedResults := localizationResults(false)
	var verification localizationverification.Assessment
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/localization-verification/runs", developer, `{"revision":"`+currentRevision+`","results":`+failedResults+`}`, 201, &verification)
	if verification.Checks[len(verification.Checks)-1].Status != "failed" {
		t.Fatalf("RTL failure was not retained: %#v", verification.Checks)
	}
	previewSource.items["fr-preview"] = previews.Preview{ID: "fr-preview", RepositoryID: string(repository.ID), PullRequestID: pull.ID, Revision: currentRevision, State: "ready", URL: "https://preview.test/fr/checkout"}
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/localization-verification/previews", developer, `{"preview_id":"fr-preview","locale_id":"fr-FR","revision":"`+currentRevision+`","routes":["/checkout"],"reviewer_ids":["reviewer-fr"]}`, 201, &verification)
	localizedPreviewID := verification.Previews[0].ID
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/locale-publication", owner, `{"revision":"`+currentRevision+`","expected_version":0,"locales":[{"locale_id":"fr-FR","state":"staged","audience":"customers","risk_class":"purchase","coverage":100,"fallback_locale":"en-US","reason":"French evidence is expected"},{"locale_id":"ar","state":"withdrawn","coverage":100,"fallback_locale":"en-US","reason":"Contain the retained RTL failure"}]}`, 200, nil)
	var readiness localizationdelivery.Assessment
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/localization-readiness", owner, `{"revision":"`+currentRevision+`","target_branch":"main","paths":["app/en-US.json"]}`, 200, &readiness)
	if readiness.Ready || !hasLocalizationRequirement(readiness, "review") {
		t.Fatalf("missing regional reviewer did not block: %#v", readiness)
	}
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/localization-verification/previews/"+localizedPreviewID+"/decisions", frenchReviewer, `{"route":"/checkout","decision":"approve","rationale":"French action is natural and complete at this exact preview."}`, 201, &verification)
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/localization-readiness", owner, `{"revision":"`+currentRevision+`","target_branch":"main","paths":["app/en-US.json"]}`, 200, &readiness)
	if !readiness.Ready {
		t.Fatalf("withdrawn RTL locale blocked safe French delivery: %#v", readiness)
	}
	waitForWorkflowCheck(t, server.URL, pullBase, owner, currentRevision, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", owner, `{"decision":"approve"}`, 200, nil)
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", owner, `{}`, 200, &pull)
	var release releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/releases", owner, `{"version":"v2.0.0","commit_id":"`+pull.MergeCommitID+`","notes":"French express checkout; Arabic withheld."}`, 201, &release)
	var publication localizationdelivery.Publication
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/locale-publications", owner, `{"kind":"application","resource_id":"`+release.ID+`","version":"v2.0.0","revision":"`+pull.MergeCommitID+`","locale_id":"fr-FR","state":"published","fallback_locale":"en-US","candidate_pull_request_id":"`+pull.ID+`","candidate_version":1,"provenance":["translation:`+current.Proposals[len(current.Proposals)-1].ID+`","preview:fr-preview","reviewer:reviewer-fr"],"reason":"Current French evidence passed"}`, 201, &publication)

	var finding localizationdelivery.Finding
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/locale-findings", reader, `{"publication_id":"`+publication.ID+`","kind":"cultural_mismatch","path":"/checkout","expected":"Use the conventional final-purchase phrase","observed":"The phrase sounds like browsing rather than committing","evidence":"regional support case 42"}`, 201, &finding)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/locale-findings/"+finding.ID+"/validation", owner, `{"state":"validated","rationale":"Confirmed by the named regional reviewer."}`, 201, &finding)
	var repairProposal proposals.Proposal
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/proposals", owner, `{"title":"Correct released French checkout","body":"Repair locale finding `+finding.ID+`."}`, 201, &repairProposal)
	var task proposals.Task
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/proposals/"+repairProposal.ID+"/plan/tasks", owner, `{"title":"Adapt French purchase commitment","outcome":"Correct the released locale without widening Arabic support","position":1,"status":"planned"}`, 201, &task)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+string(repository.ID)+"/proposals/"+repairProposal.ID+"/plan/tasks/"+task.ID+"/assignment", owner, `{"kind":"agent","assignee_id":"codex","mandate":"Correct only the reported French message and return through exact locale review.","repository_id":"`+string(repository.ID)+`","base_revision":"`+pull.MergeCommitID+`"}`, 200, &task)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/locale-findings/"+finding.ID+"/repair", owner, `{"owner_kind":"agent","owner_id":"codex","proposal_id":"`+repairProposal.ID+`","task_id":"`+task.ID+`","acceptance_criteria":["Use the conventional final-purchase phrase","Retain exact reviewer evidence"]}`, 201, &finding)

	repair := gitClone(t, remote(agentGit))
	gitOutput(t, repair, "config", "user.name", "Codex Agent")
	gitOutput(t, repair, "config", "user.email", "codex@agents.test")
	gitOutput(t, repair, "switch", "-c", "locales/fr-purchase")
	writeWorkflowFile(t, repair, "app/fr-FR.json", `{"checkout":"Confirmer l’achat sécurisé"}`+"\n")
	gitOutput(t, repair, "add", "app/fr-FR.json")
	gitOutput(t, repair, "commit", "-m", "Correct French purchase commitment")
	repairRevision := gitOutput(t, repair, "rev-parse", "HEAD")
	gitOutput(t, repair, "push", "-u", "origin", "locales/fr-purchase")
	var repairPull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests", agent, `{"title":"Correct released French checkout","body":"Connected to finding `+finding.ID+` and task `+task.ID+`.","source_branch":"locales/fr-purchase","target_branch":"main"}`, 201, &repairPull)
	repairBase := "/repositories/" + string(repository.ID) + "/pull-requests/" + repairPull.ID
	waitForWorkflowCheck(t, server.URL, repairBase, owner, repairRevision, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, repairBase+"/reviews/me", owner, `{"decision":"approve"}`, 200, nil)
	workflowJSON(t, server.URL, http.MethodPost, repairBase+"/merge", owner, `{}`, 200, &repairPull)
	var corrected releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/releases", owner, `{"version":"v2.0.1","commit_id":"`+repairPull.MergeCommitID+`","prior_release_id":"`+release.ID+`","notes":"Corrected French regional wording."}`, 201, &corrected)
	// The corrected publication intentionally references its own governed
	// candidate record while Arabic remains outside the support claim.
	workflowJSON(t, server.URL, http.MethodPut, repairBase+"/locale-publication", owner, `{"revision":"`+repairRevision+`","expected_version":0,"locales":[{"locale_id":"fr-FR","state":"staged","audience":"customers","risk_class":"purchase","coverage":100,"fallback_locale":"en-US","reason":"Corrected French-only release"}]}`, 200, nil)
	var correctedPublication localizationdelivery.Publication
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/locale-publications", owner, `{"kind":"application","resource_id":"`+corrected.ID+`","version":"v2.0.1","revision":"`+repairPull.MergeCommitID+`","locale_id":"fr-FR","state":"published","fallback_locale":"en-US","candidate_pull_request_id":"`+repairPull.ID+`","candidate_version":1,"provenance":["finding:`+finding.ID+`","task:`+task.ID+`","agent:codex","review:owner"],"reason":"Publish the connected regional correction"}`, 201, &correctedPublication)
	if correctedPublication.PublishedByID != "owner" || finding.Repair.OwnerID != "codex" || !strings.Contains(repairPull.Body, finding.ID) || corrected.PullRequests[0].ID != repairPull.ID {
		t.Fatalf("correction trail lost authorship or delivery provenance: finding=%#v pull=%#v release=%#v publication=%#v", finding, repairPull, corrected, correctedPublication)
	}
}

func writeLocalizationManifests(t *testing.T, dir string) {
	writeWorkflowFile(t, dir, ".komodo/localization.json", `{"schema_version":1,"source_locale":"en-US","locales":["fr-FR","ar"],"product_context":"Final purchase action","resources":[{"id":"checkout","source_path":"app/en-US.json","translation_path":"app/{locale}.json","format":"json","context":{"checkout":"Primary final-purchase button"},"screenshots":{"checkout":["https://example.test/checkout.png"]}}]}`)
	writeWorkflowFile(t, dir, ".komodo/localization-checks.json", localizationChecksManifest())
	writeWorkflowFile(t, dir, ".komodo/checks.json", `{"version":1,"checks":[{"name":"product","command":"test -f app/en-US.json && test -f app/fr-FR.json","timeout_seconds":30}]}`)
	writeWorkflowFile(t, dir, ".komodo/releases.json", `{"version":1,"builds":[{"name":"application","command":"mkdir -p dist; cp app/*.json dist/","artifacts":["dist/en-US.json","dist/fr-FR.json","dist/ar.json"]}]}`)
}

func localizationChecksManifest() string {
	kinds := []string{"variables", "pluralization", "formatting", "terminology", "links", "layout_expansion", "fallback", "journey"}
	parts := make([]string, 0, len(kinds)+1)
	for _, kind := range kinds {
		name := "fr/" + kind
		if kind == "journey" {
			name = "fr/journey"
		}
		parts = append(parts, `{"name":"`+name+`","kind":"`+kind+`","locale_id":"fr-FR","journey_id":"buy","route":"/checkout","command":"verify","interface_paths":["app/fr-FR.json"]}`)
	}
	parts = append(parts, `{"name":"ar/rtl","kind":"bidirectional_text","locale_id":"ar","journey_id":"buy","route":"/checkout","command":"verify rtl","interface_paths":["app/ar.json"]}`)
	return `{"schema_version":1,"checks":[` + strings.Join(parts, ",") + `]}`
}

func localizationResults(rtlPass bool) string {
	items := []string{}
	for _, kind := range []string{"variables", "pluralization", "formatting", "terminology", "links", "layout_expansion", "fallback", "journey"} {
		items = append(items, `"fr/`+kind+`":{"status":"passed","summary":"French `+kind+` evidence is current"}`)
	}
	status, summary := "failed", "Mixed-direction price and SKU overlap"
	if rtlPass {
		status, summary = "passed", "RTL layout is isolated"
	}
	items = append(items, `"ar/rtl":{"status":"`+status+`","summary":"`+summary+`"}`)
	return `{` + strings.Join(items, ",") + `}`
}

func hasLocalizationRequirement(a localizationdelivery.Assessment, kind string) bool {
	for _, r := range a.Requirements {
		if r.Kind == kind {
			return true
		}
	}
	return false
}
