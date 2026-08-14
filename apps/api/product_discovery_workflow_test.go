package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productfeedback"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productlearning"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productroadmaps"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/roadmapdelivery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/roadmapvalidations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type noProductDiscoveryOrganizations struct{}

func (noProductDiscoveryOrganizations) IsMember(string, string) bool { return false }

// TestProductDiscoveryWorkflow is the black-box boundary for the complete
// released-product-feedback-to-measured-learning loop. It uses public HTTP and
// stock Git, retains human and agent attribution, and proves that privacy,
// consent, challenge, rejection, missed targets, and failed measures survive
// accountable replanning rather than becoming implicit product authority.
func TestProductDiscoveryWorkflow(t *testing.T) {
	requireGit(t)
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	feedback, _ := productfeedback.New(t.TempDir())
	opportunities, _ := productopportunities.New(t.TempDir())
	roadmaps, _ := productroadmaps.New(t.TempDir())
	validations, _ := roadmapvalidations.New(t.TempDir())
	deliveries, _ := roadmapdelivery.New(t.TempDir())
	learning, _ := productlearning.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerGitHTTP(mux, catalog, credentials)
	registerProductFeedbackHTTP(mux, feedback, catalog, credentials, feedbackSources{organizations: noProductDiscoveryOrganizations{}})
	registerProductOpportunitiesHTTP(mux, opportunities, catalog, credentials, opportunitySources{feedback: feedback})
	registerProductRoadmapsHTTP(mux, roadmaps, opportunities, catalog, credentials)
	registerRoadmapValidationsHTTP(mux, validations, roadmaps, feedback, catalog, credentials)
	registerRoadmapDeliveryHTTP(mux, deliveries, roadmaps, plans, catalog, credentials)
	registerProductLearningHTTP(mux, learning, deliveries, feedback, opportunities, roadmaps, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "product-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "discovery-agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reporter := issueAccess(t, credentials, "public-reporter", auth.API, auth.RepositoryRead)
	privateReporter := issueAccess(t, credentials, "private-reporter", auth.API, auth.RepositoryRead)
	reader := issueAccess(t, credentials, "project-reader", auth.API, auth.RepositoryRead)
	ownerGit := issueAccess(t, credentials, "product-owner", auth.Git, auth.GitRead, auth.GitWrite)
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"released-review","visibility":"public"}`, http.StatusCreated, &repository)
	if _, err := catalog.AddCollaborator("product-owner", repository.ID, "discovery-agent"); err != nil {
		t.Fatal(err)
	}
	remoteURL, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
	remoteURL.User = url.UserPassword("git", ownerGit)
	work := gitClone(t, remoteURL.String())
	gitOutput(t, work, "config", "user.name", "Product Owner")
	gitOutput(t, work, "config", "user.email", "owner@example.com")
	writeWorkflowFile(t, work, "review.txt", "released review experience\n")
	gitOutput(t, work, "add", "review.txt")
	gitOutput(t, work, "commit", "-m", "Release the existing review experience")
	baseRevision := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "main")

	feedbackBase := "/repositories/" + string(repository.ID) + "/product-feedback"
	var publicNeed productfeedback.Feedback
	workflowJSON(t, server.URL, http.MethodPost, feedbackBase, reporter, `{"context":{"kind":"project","label":"v1 released review experience"},"need":"The released review page does not explain what is blocking me","desired_outcome":"See one actionable next step","frequency":"weekly","impact":"Contributions wait for days","audience":"public","identity_visibility":"audience","contact_preference":"discussion","consent":{"research":true,"product_updates":true},"evidence":[{"kind":"quote","name":"redacted-session.txt","media_type":"text/plain","content":"[redacted] waited three days without knowing why","visibility":"audience","redacted":true}]}`, http.StatusCreated, &publicNeed)
	var privateNeed productfeedback.Feedback
	workflowJSON(t, server.URL, http.MethodPost, feedbackBase, privateReporter, `{"context":{"kind":"project","label":"v1 released review experience"},"need":"Keyboard navigation gets lost in the released review page","desired_outcome":"Complete review without losing focus","frequency":"daily","impact":"I cannot finish the task","audience":"repository","identity_visibility":"maintainers","contact_preference":"email","contact_value":"private@example.test","consent":{"research":true,"product_updates":true},"evidence":[{"kind":"quote","name":"redacted-accessibility.txt","media_type":"text/plain","content":"[redacted] focus disappeared after submit","visibility":"maintainers","redacted":true}]}`, http.StatusCreated, &privateNeed)
	var projected productfeedback.Feedback
	workflowJSON(t, server.URL, http.MethodGet, feedbackBase+"/"+privateNeed.ID, reader, "", http.StatusOK, &projected)
	if projected.ReporterID != "" || projected.ContactValue != "" || projected.Evidence[0].Content != "" {
		t.Fatalf("private identity, contact, or evidence leaked: %#v", projected)
	}

	revision := strconv.FormatInt(publicNeed.UpdatedAt.UnixNano(), 10)
	opportunityBody := fmt.Sprintf(`{"title":"Make review blockers actionable","need":"Contributors cannot recover from opaque review blockers","affected_audiences":["contributors","keyboard users"],"severity":"high","reach":"some","confidence":"medium","expected_value":"Shorter and more accessible review completion","uncertainty":["accessibility evidence is currently a minority report"],"sources":[{"kind":"feedback","resource_id":%q,"captured_revision":%q,"relevance":"Direct report from the released experience","position":"supporting"},{"kind":"usage_evidence","resource_id":"abandoned-review-query","captured_revision":"sha256:bounded-aggregate","relevance":"Aggregate exits corroborate the need without identifying users","position":"supporting"},{"kind":"support_signal","resource_id":"resolved-without-change","captured_revision":"sha256:bounded-support","relevance":"Some users recovered without a product change","position":"contradicting"}],"change_reason":"Human synthesis preserves supporting and contradictory evidence"}`, publicNeed.ID, revision)
	var opportunity productopportunities.Opportunity
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/product-opportunities", owner, opportunityBody, http.StatusCreated, &opportunity)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/product-opportunities/"+opportunity.ID+"/notes", agent, `{"kind":"challenge","source_kind":"support_signal","resource_id":"resolved-without-change","body":"The aggregate may hide keyboard users; keep this uncertainty explicit."}`, http.StatusCreated, &opportunity)

	rejectedBody := fmt.Sprintf(`{"title":"Replace all review controls","need":"Assume every review needs a new control surface","affected_audiences":["all contributors"],"severity":"medium","reach":"unknown","confidence":"low","expected_value":"Potentially faster reviews","uncertainty":["No direct evidence supports a wholesale replacement"],"sources":[{"kind":"feedback","resource_id":%q,"captured_revision":%q,"relevance":"The report supports clarity but not a wholesale replacement","position":"contradicting"}],"change_reason":"Retain a rejected alternative instead of hiding it"}`, publicNeed.ID, revision)
	var rejected productopportunities.Opportunity
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/product-opportunities", agent, rejectedBody, http.StatusCreated, &rejected)

	now := time.Now().UTC()
	roadmapBody := fmt.Sprintf(`{"name":"Transparent review direction","goals":["make review recovery clear and accessible"],"capacity_units":5,"outcomes":[{"id":"actionable-review","opportunity_id":%q,"opportunity_version":1,"title":"Actionable review blockers","decision":"accepted","status":"planned","owner_id":"product-owner","owner_available":true,"target_horizon":%q,"success_measures":["completion improves","keyboard guardrail passes"],"goals":["review clarity"],"risks":["focus regression"],"capacity_units":5,"sequence":1,"rationale":"Direct feedback and bounded usage evidence justify a test."},{"id":"replace-review","opportunity_id":%q,"opportunity_version":1,"title":"Replace every review control","decision":"rejected","status":"rejected","success_measures":[],"goals":["review clarity"],"risks":["unsupported scope"],"capacity_units":0,"sequence":2,"rationale":"Evidence contradicts the breadth of this alternative."}],"change_reason":"Publish the accepted and rejected choices with rationale"}`, opportunity.ID, now.Add(-time.Hour).Format(time.RFC3339Nano), rejected.ID)
	var roadmap productroadmaps.Roadmap
	roadmapBase := "/repositories/" + string(repository.ID) + "/product-roadmaps"
	workflowJSON(t, server.URL, http.MethodPost, roadmapBase, owner, roadmapBody, http.StatusCreated, &roadmap)
	if !discoveryContains(roadmap.Versions[0].Blockers, "slipped_target:actionable-review") {
		t.Fatalf("missed target was not explicit: %#v", roadmap.Versions[0].Blockers)
	}
	roadmapBody = fmt.Sprintf(`{"expected_version":1,"name":"Transparent review direction","goals":["make review recovery clear and accessible"],"capacity_units":5,"outcomes":[{"id":"actionable-review","opportunity_id":%q,"opportunity_version":1,"title":"Actionable review blockers","decision":"accepted","status":"planned","owner_id":"product-owner","owner_available":true,"target_horizon":%q,"success_measures":["completion improves","keyboard guardrail passes"],"goals":["review clarity"],"risks":["focus regression"],"capacity_units":5,"sequence":1,"rationale":"Replanned after the target slipped, without erasing it."},{"id":"replace-review","opportunity_id":%q,"opportunity_version":1,"title":"Replace every review control","decision":"rejected","status":"rejected","success_measures":[],"goals":["review clarity"],"risks":["unsupported scope"],"capacity_units":0,"sequence":2,"rationale":"Evidence still contradicts this alternative."}],"change_reason":"Move the missed target forward explicitly"}`, opportunity.ID, now.Add(7*24*time.Hour).Format(time.RFC3339Nano), rejected.ID)
	workflowJSON(t, server.URL, http.MethodPost, roadmapBase+"/"+roadmap.ID+"/versions", owner, roadmapBody, http.StatusCreated, &roadmap)

	validationBase := roadmapBase + "/" + roadmap.ID + "/validations"
	validationBody := fmt.Sprintf(`{"outcome_id":"actionable-review","kind":"prototype","title":"Actionable blocker preview","hypothesis":"One next step improves completion without breaking keyboard access","measures":[{"name":"completion","kind":"success","feedback_ids":[%q],"threshold":"4 of 5 complete"},{"name":"keyboard access","kind":"guardrail","feedback_ids":[%q],"threshold":"no blocking focus loss"}],"activity":{"kind":"preview","revision":%q,"scope":"released review blocker panel only","starts_at":%q,"ends_at":%q},"change_reason":"Validate direction with a consenting affected user"}`, publicNeed.ID, publicNeed.ID, baseRevision, now.Format(time.RFC3339Nano), now.Add(time.Hour).Format(time.RFC3339Nano))
	var validation roadmapvalidations.Validation
	workflowJSON(t, server.URL, http.MethodPost, validationBase, owner, validationBody, http.StatusCreated, &validation)
	var invitation struct {
		Validation            roadmapvalidations.Validation `json:"validation"`
		ParticipantCredential struct {
			Token string `json:"token"`
		} `json:"participant_credential"`
	}
	workflowJSON(t, server.URL, http.MethodPost, validationBase+"/"+validation.ID+"/invitations", owner, fmt.Sprintf(`{"participant_id":"public-reporter","feedback_id":%q,"accessibility_needs":"keyboard-only"}`, publicNeed.ID), http.StatusCreated, &invitation)
	var findingValidation roadmapvalidations.Validation
	workflowJSON(t, server.URL, http.MethodPost, "/roadmap-validation-participant/findings", invitation.ParticipantCredential.Token, `{"finding":"The next step is clear and focus remains visible","acceptance":"accept","evidence_validity":"valid"}`, http.StatusCreated, &findingValidation)
	workflowJSON(t, server.URL, http.MethodPost, validationBase+"/"+validation.ID+"/assessments", owner, fmt.Sprintf(`{"finding_ids":[%q],"evidence_status":"valid","decision":"accept","rationale":"Participant evidence clears success and guardrail criteria"}`, findingValidation.Findings[0].ID), http.StatusCreated, &validation)
	workflowJSON(t, server.URL, http.MethodPost, feedbackBase+"/"+privateNeed.ID+"/consent-withdrawal", privateReporter, `{}`, http.StatusOK, &privateNeed)
	workflowJSON(t, server.URL, http.MethodPost, validationBase+"/"+validation.ID+"/invitations", owner, fmt.Sprintf(`{"participant_id":"private-reporter","feedback_id":%q}`, privateNeed.ID), http.StatusUnprocessableEntity, nil)

	deliveryBase := roadmapBase + "/" + roadmap.ID + "/outcomes/actionable-review/delivery"
	deliveryBody := fmt.Sprintf(`{"outcome_id":"actionable-review","base_revision":%q,"tasks":[{"title":"Design the accessible next step","owner_kind":"human","owner_id":"product-owner","acceptance_criteria":["preview matches accepted validation"],"evidence_ids":[%q],"success_measures":["keyboard guardrail passes"]},{"title":"Implement and measure the blocker panel","owner_kind":"agent","owner_id":"discovery-agent","acceptance_criteria":["review, preview, experiment, and release evidence are linked"],"evidence_ids":[%q,%q],"success_measures":["completion improves"],"depends_on":[1]}]}`, baseRevision, validation.ID, opportunity.ID, publicNeed.ID)
	var promoted struct {
		Delivery roadmapdelivery.Delivery `json:"delivery"`
	}
	workflowJSON(t, server.URL, http.MethodPost, deliveryBase, owner, deliveryBody, http.StatusCreated, &promoted)
	delivery := promoted.Delivery
	for _, link := range []string{
		`{"kind":"pull_request","resource_id":"pull-human-agent","revision":"candidate-reviewed","state":"approved"}`,
		`{"kind":"check","resource_id":"checks-current","revision":"candidate-reviewed","state":"succeeded"}`,
		`{"kind":"preview","resource_id":"preview-accessible","revision":"candidate-reviewed","state":"accepted"}`,
		`{"kind":"experiment","resource_id":"experiment-adoption","revision":"run-1","state":"completed","measure_results":{"completion improves":"failed","keyboard guardrail passes":"passed"},"evidence_ids":["aggregate:consent-bounded-run"]}`,
		`{"kind":"release","resource_id":"release-v2","revision":"candidate-reviewed","state":"published"}`,
	} {
		workflowJSON(t, server.URL, http.MethodPost, deliveryBase+"/"+delivery.ID+"/links", owner, link, http.StatusCreated, &delivery)
	}
	if delivery.State != "delivered_not_achieved" || !discoveryContains(delivery.Blockers, "failed_measure:completion improves") {
		t.Fatalf("release incorrectly stood in for achieved value: %#v", delivery)
	}
	workflowJSON(t, server.URL, http.MethodPost, deliveryBase+"/"+delivery.ID+"/revisit-requests", owner, `{"reason":"Participant evidence says the release is clearer but still does not improve completion","evidence_ids":["follow-up:released-v2"]}`, http.StatusCreated, &delivery)
	if delivery.State != "revisit_required" {
		t.Fatalf("failed measure did not force a revisit: %#v", delivery)
	}

	learningBase := "/repositories/" + string(repository.ID) + "/roadmap-deliveries/" + delivery.ID + "/learning"
	var record productlearning.Record
	workflowJSON(t, server.URL, http.MethodPost, learningBase+"/updates", owner, fmt.Sprintf(`{"kind":"measured_outcome","summary":"The reviewed release shipped but missed its completion target","rationale":"Keyboard safety passed while measured completion failed, so direction is being replanned","audience":"participants","feedback_ids":[%q],"links":[{"kind":"release","resource_id":"release-v2","label":"Validate the released outcome","public":true},{"kind":"experiment","resource_id":"private-run-details","label":"Restricted aggregate analysis","public":false}]}`, publicNeed.ID), http.StatusCreated, &record)
	workflowJSON(t, server.URL, http.MethodPost, learningBase+"/updates/"+record.Updates[0].ID+"/responses", reporter, fmt.Sprintf(`{"feedback_id":%q,"outcome":"mixed","body":"The blocker is clearer, but completion still fails on long reviews","evidence":["follow-up:released-v2"],"dissent":true}`, publicNeed.ID), http.StatusCreated, &record)
	workflowJSON(t, server.URL, http.MethodPost, learningBase+"/lessons", owner, fmt.Sprintf(`{"expected_outcomes":["completion improves","keyboard guardrail passes"],"observed_outcomes":["completion failed","keyboard guardrail passed"],"lessons":["clarity alone does not resolve long-review completion"],"dissent":["participant reports a mixed result"],"resulting_work":[{"kind":"roadmap","resource_id":%q,"label":"replan direction","public":true}],"roadmap_id":%q,"roadmap_version":2,"opportunity_disposition":"open","change_reason":"Keep the unmet opportunity open after measured failure","expected_revision":0}`, roadmap.ID, roadmap.ID), http.StatusCreated, &record)
	if len(record.Responses) != 1 || !record.Responses[0].Dissent || record.Lessons[0].OpportunityDisposition != "open" || record.OperationalAuthority {
		t.Fatalf("continuous product-learning trail is incomplete: %#v", record)
	}
}

func discoveryContains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
