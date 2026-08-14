package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitybarriers"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitycommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributionopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributorpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deliveryteams"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyinventory"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyupdates"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/docscollections"
	"github.com/greptile-projects/vivarium-komodo/apps/api/extensions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/federatedagents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/federation"
	"github.com/greptile-projects/vivarium-komodo/apps/api/governance"
	"github.com/greptile-projects/vivarium-komodo/apps/api/impactassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/inbox"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/integrationqueue"
	"github.com/greptile-projects/vivarium-komodo/apps/api/investigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/performancegoals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/performanceinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productexperiments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productfeedback"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productlearning"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productroadmaps"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectfunds"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/questions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/relationships"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/roadmapdelivery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/roadmapvalidations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityreports"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

func main() {
	repositoryRoot := os.Getenv("REPOSITORY_ROOT")
	if repositoryRoot == "" {
		repositoryRoot = "repositories"
	}
	repositoryStorage, err := storage.New(repositoryRoot)
	if err != nil {
		log.Fatal(err)
	}
	repositoryCatalogRoot := os.Getenv("REPOSITORY_CATALOG_ROOT")
	if repositoryCatalogRoot == "" {
		repositoryCatalogRoot = "data/repositories"
	}
	repositoryCatalog, err := repositories.New(repositoryCatalogRoot, repositoryStorage)
	if err != nil {
		log.Fatal(err)
	}
	userRoot := os.Getenv("USER_ROOT")
	if userRoot == "" {
		userRoot = "data/users"
	}
	userStore, err := users.New(userRoot)
	if err != nil {
		log.Fatal(err)
	}
	authRoot := os.Getenv("AUTH_ROOT")
	if authRoot == "" {
		authRoot = "data/auth"
	}
	credentials, err := auth.New(authRoot)
	if err != nil {
		log.Fatal(err)
	}
	proposalRoot := os.Getenv("PROPOSAL_ROOT")
	if proposalRoot == "" {
		proposalRoot = "data/proposals"
	}
	proposalStore, err := proposals.New(proposalRoot)
	if err != nil {
		log.Fatal(err)
	}
	pullRequestRoot := os.Getenv("PULL_REQUEST_ROOT")
	if pullRequestRoot == "" {
		pullRequestRoot = "data/pull-requests"
	}
	pullRequestStore, err := pullrequests.New(pullRequestRoot)
	if err != nil {
		log.Fatal(err)
	}
	releaseRoot := os.Getenv("RELEASE_ROOT")
	if releaseRoot == "" {
		releaseRoot = "data/releases"
	}
	releaseStore, err := releases.New(releaseRoot)
	if err != nil {
		log.Fatal(err)
	}
	packageRoot := os.Getenv("PACKAGE_ROOT")
	if packageRoot == "" {
		packageRoot = "data/packages"
	}
	packageStore, err := packagecatalog.New(packageRoot)
	if err != nil {
		log.Fatal(err)
	}
	dependencyInventoryRoot := os.Getenv("DEPENDENCY_INVENTORY_ROOT")
	if dependencyInventoryRoot == "" {
		dependencyInventoryRoot = "data/dependency-inventories"
	}
	dependencyInventoryStore, err := dependencyinventory.New(dependencyInventoryRoot)
	if err != nil {
		log.Fatal(err)
	}
	dependencyUpdateRoot := os.Getenv("DEPENDENCY_UPDATE_ROOT")
	if dependencyUpdateRoot == "" {
		dependencyUpdateRoot = "data/dependency-updates"
	}
	dependencyUpdateStore, err := dependencyupdates.New(dependencyUpdateRoot)
	if err != nil {
		log.Fatal(err)
	}
	relationshipRoot := os.Getenv("RELATIONSHIP_ROOT")
	if relationshipRoot == "" {
		relationshipRoot = "data/relationships"
	}
	relationshipStore, err := relationships.New(relationshipRoot)
	if err != nil {
		log.Fatal(err)
	}
	deploymentRoot := os.Getenv("DEPLOYMENT_ROOT")
	if deploymentRoot == "" {
		deploymentRoot = "data/deployments"
	}
	deploymentStore, err := deployments.New(deploymentRoot)
	if err != nil {
		log.Fatal(err)
	}
	incidentRoot := os.Getenv("INCIDENT_ROOT")
	if incidentRoot == "" {
		incidentRoot = "data/incidents"
	}
	incidentStore, err := incidents.New(incidentRoot)
	if err != nil {
		log.Fatal(err)
	}
	securityReportRoot := os.Getenv("SECURITY_REPORT_ROOT")
	if securityReportRoot == "" {
		securityReportRoot = "data/security-reports"
	}
	securityReportStore, err := securityreports.New(securityReportRoot)
	if err != nil {
		log.Fatal(err)
	}
	issueRoot := os.Getenv("ISSUE_ROOT")
	if issueRoot == "" {
		issueRoot = "data/issues"
	}
	issueStore, err := issues.New(issueRoot)
	if err != nil {
		log.Fatal(err)
	}
	issueReproductionRunner := issues.NewReproductionRunner(issueStore, repositoryCatalog)
	changeSessionRoot := os.Getenv("CHANGE_SESSION_ROOT")
	if changeSessionRoot == "" {
		changeSessionRoot = "data/change-sessions"
	}
	changeSessionStore, err := changesessions.New(changeSessionRoot)
	if err != nil {
		log.Fatal(err)
	}
	checkRunRoot := os.Getenv("CHECK_RUN_ROOT")
	if checkRunRoot == "" {
		checkRunRoot = "data/check-runs"
	}
	checkRunStore, err := checkruns.New(checkRunRoot)
	if err != nil {
		log.Fatal(err)
	}
	checkRunner := checkruns.NewRunner(checkRunStore, repositoryCatalog)
	integrationQueueRoot := os.Getenv("INTEGRATION_QUEUE_ROOT")
	if integrationQueueRoot == "" {
		integrationQueueRoot = "data/integration-queue"
	}
	integrationQueueStore, err := integrationqueue.New(integrationQueueRoot)
	if err != nil {
		log.Fatal(err)
	}
	activityRoot := os.Getenv("ACTIVITY_ROOT")
	if activityRoot == "" {
		activityRoot = "data/activities"
	}
	activityStore, err := activities.New(activityRoot, userStore)
	if err != nil {
		log.Fatal(err)
	}
	queueCoordinator := &integrationQueueCoordinator{queue: integrationQueueStore, pulls: pullRequestStore, repositories: repositoryCatalog, checks: checkRunStore, starter: checkRunner, activity: activityStore, proposals: proposalStore}
	checkRunner.SetCompletionHook(func(checkruns.Run) { go queueCoordinator.reconcileAll(context.Background()) })
	go queueCoordinator.run(context.Background())
	inboxRoot := os.Getenv("INBOX_ROOT")
	if inboxRoot == "" {
		inboxRoot = "data/inbox"
	}
	inboxStore, err := inbox.New(inboxRoot)
	if err != nil {
		log.Fatal(err)
	}
	organizationRoot := os.Getenv("ORGANIZATION_ROOT")
	if organizationRoot == "" {
		organizationRoot = "data/organizations"
	}
	organizationStore, err := organizations.New(organizationRoot)
	if err != nil {
		log.Fatal(err)
	}
	governanceRoot := os.Getenv("GOVERNANCE_ROOT")
	if governanceRoot == "" {
		governanceRoot = "data/governance"
	}
	governanceStore, err := governance.New(governanceRoot)
	if err != nil {
		log.Fatal(err)
	}
	workspaceRoot := os.Getenv("WORKSPACE_ROOT")
	if workspaceRoot == "" {
		workspaceRoot = "data/workspaces"
	}
	workspaceStore, err := workspaces.New(workspaceRoot)
	if err != nil {
		log.Fatal(err)
	}
	workspaceRunner := workspaces.NewRunner(workspaceStore, repositoryCatalog)
	questionRoot := os.Getenv("QUESTION_ROOT")
	if questionRoot == "" {
		questionRoot = "data/questions"
	}
	questionStore, err := questions.New(questionRoot)
	if err != nil {
		log.Fatal(err)
	}
	investigationRoot := os.Getenv("INVESTIGATION_ROOT")
	if investigationRoot == "" {
		investigationRoot = "data/investigations"
	}
	investigationStore, err := investigations.New(investigationRoot)
	if err != nil {
		log.Fatal(err)
	}
	impactRoot := os.Getenv("IMPACT_ASSESSMENT_ROOT")
	if impactRoot == "" {
		impactRoot = "data/impact-assessments"
	}
	impactStore, err := impactassessments.New(impactRoot)
	if err != nil {
		log.Fatal(err)
	}
	decisionRoot := os.Getenv("DECISION_ROOT")
	if decisionRoot == "" {
		decisionRoot = "data/decisions"
	}
	decisionStore, err := decisions.New(decisionRoot)
	if err != nil {
		log.Fatal(err)
	}
	deliveryTeamRoot := os.Getenv("DELIVERY_TEAM_ROOT")
	if deliveryTeamRoot == "" {
		deliveryTeamRoot = "data/delivery-teams"
	}
	deliveryTeamStore, err := deliveryteams.New(deliveryTeamRoot)
	if err != nil {
		log.Fatal(err)
	}
	performanceGoalRoot := os.Getenv("PERFORMANCE_GOAL_ROOT")
	if performanceGoalRoot == "" {
		performanceGoalRoot = "data/performance-goals"
	}
	performanceGoalStore, err := performancegoals.New(performanceGoalRoot)
	if err != nil {
		log.Fatal(err)
	}
	accessibilityCommitmentRoot := os.Getenv("ACCESSIBILITY_COMMITMENT_ROOT")
	if accessibilityCommitmentRoot == "" {
		accessibilityCommitmentRoot = "data/accessibility-commitments"
	}
	accessibilityCommitmentStore, err := accessibilitycommitments.New(accessibilityCommitmentRoot)
	if err != nil {
		log.Fatal(err)
	}
	accessibilityBarrierRoot := os.Getenv("ACCESSIBILITY_BARRIER_ROOT")
	if accessibilityBarrierRoot == "" {
		accessibilityBarrierRoot = "data/accessibility-barriers"
	}
	accessibilityBarrierStore, err := accessibilitybarriers.New(accessibilityBarrierRoot)
	if err != nil {
		log.Fatal(err)
	}
	projectFundRoot := os.Getenv("PROJECT_FUND_ROOT")
	if projectFundRoot == "" {
		projectFundRoot = "data/project-funds"
	}
	projectFundStore, err := projectfunds.New(projectFundRoot)
	if err != nil {
		log.Fatal(err)
	}
	performanceInvestigationRoot := os.Getenv("PERFORMANCE_INVESTIGATION_ROOT")
	if performanceInvestigationRoot == "" {
		performanceInvestigationRoot = "data/performance-investigations"
	}
	performanceInvestigationStore, err := performanceinvestigations.New(performanceInvestigationRoot)
	if err != nil {
		log.Fatal(err)
	}
	productExperimentRoot := os.Getenv("PRODUCT_EXPERIMENT_ROOT")
	if productExperimentRoot == "" {
		productExperimentRoot = "data/product-experiments"
	}
	productExperimentStore, err := productexperiments.New(productExperimentRoot)
	if err != nil {
		log.Fatal(err)
	}
	productFeedbackRoot := os.Getenv("PRODUCT_FEEDBACK_ROOT")
	if productFeedbackRoot == "" {
		productFeedbackRoot = "data/product-feedback"
	}
	productFeedbackStore, err := productfeedback.New(productFeedbackRoot)
	if err != nil {
		log.Fatal(err)
	}
	productOpportunityRoot := os.Getenv("PRODUCT_OPPORTUNITY_ROOT")
	if productOpportunityRoot == "" {
		productOpportunityRoot = "data/product-opportunities"
	}
	productOpportunityStore, err := productopportunities.New(productOpportunityRoot)
	if err != nil {
		log.Fatal(err)
	}
	productRoadmapRoot := os.Getenv("PRODUCT_ROADMAP_ROOT")
	if productRoadmapRoot == "" {
		productRoadmapRoot = "data/product-roadmaps"
	}
	productRoadmapStore, err := productroadmaps.New(productRoadmapRoot)
	if err != nil {
		log.Fatal(err)
	}
	roadmapValidationRoot := os.Getenv("ROADMAP_VALIDATION_ROOT")
	if roadmapValidationRoot == "" {
		roadmapValidationRoot = "data/roadmap-validations"
	}
	roadmapValidationStore, err := roadmapvalidations.New(roadmapValidationRoot)
	if err != nil {
		log.Fatal(err)
	}
	roadmapDeliveryRoot := os.Getenv("ROADMAP_DELIVERY_ROOT")
	if roadmapDeliveryRoot == "" {
		roadmapDeliveryRoot = "data/roadmap-deliveries"
	}
	roadmapDeliveryStore, err := roadmapdelivery.New(roadmapDeliveryRoot)
	if err != nil {
		log.Fatal(err)
	}
	productLearningRoot := os.Getenv("PRODUCT_LEARNING_ROOT")
	if productLearningRoot == "" {
		productLearningRoot = "data/product-learning"
	}
	productLearningStore, err := productlearning.New(productLearningRoot)
	if err != nil {
		log.Fatal(err)
	}
	contributorPathwayRoot := os.Getenv("CONTRIBUTOR_PATHWAY_ROOT")
	if contributorPathwayRoot == "" {
		contributorPathwayRoot = "data/contributor-pathways"
	}
	contributorPathwayStore, err := contributorpathways.New(contributorPathwayRoot)
	if err != nil {
		log.Fatal(err)
	}
	documentationRoot := os.Getenv("DOCUMENTATION_ROOT")
	if documentationRoot == "" {
		documentationRoot = "data/documentation"
	}
	documentationStore, err := docscollections.New(documentationRoot)
	if err != nil {
		log.Fatal(err)
	}
	contributionOpportunityRoot := os.Getenv("CONTRIBUTION_OPPORTUNITY_ROOT")
	if contributionOpportunityRoot == "" {
		contributionOpportunityRoot = "data/contribution-opportunities"
	}
	contributionOpportunityStore, err := contributionopportunities.New(contributionOpportunityRoot)
	if err != nil {
		log.Fatal(err)
	}
	previewRoot := os.Getenv("PREVIEW_ROOT")
	if previewRoot == "" {
		previewRoot = "data/previews"
	}
	previewStore, err := previews.New(previewRoot)
	if err != nil {
		log.Fatal(err)
	}
	previewRunner := previews.NewRunner(previewStore, repositoryCatalog)
	extensionRoot := os.Getenv("EXTENSION_ROOT")
	if extensionRoot == "" {
		extensionRoot = "data/extensions"
	}
	extensionStore, err := extensions.New(extensionRoot)
	if err != nil {
		log.Fatal(err)
	}
	federationRoot := os.Getenv("FEDERATION_ROOT")
	if federationRoot == "" {
		federationRoot = "data/federation"
	}
	instanceOrigin := os.Getenv("INSTANCE_ORIGIN")
	if instanceOrigin == "" {
		instanceOrigin = "https://localhost:8080"
	}
	federationStore, err := federation.New(federationRoot, federation.Config{Instance: instanceOrigin, Operators: []federation.Operator{{Name: "Komodo operator", Contact: "mailto:operator@localhost"}}, Capabilities: []string{"identity.discovery", "actor.lookup", "key.rotation", "repository.discovery", "repository.contributions", "pull_request.exchange", "repository.contribution_receipts"}, Endpoints: federation.Endpoints{Discovery: instanceOrigin + "/.well-known/komodo-federation", Actors: instanceOrigin + "/federation/actors/{kind}/{id}", Repositories: instanceOrigin + "/federation/repositories/{id}", RepositoryObjects: instanceOrigin + "/federation/repositories/{id}/objects", Contributions: instanceOrigin + "/federation/contributions", PullRequestEvents: instanceOrigin + "/federation/pull-request-events", ContributionReceipts: instanceOrigin + "/federation/contribution-receipts"}})
	if err != nil {
		log.Fatal(err)
	}
	federatedAgentRoot := os.Getenv("FEDERATED_AGENT_SESSION_ROOT")
	if federatedAgentRoot == "" {
		federatedAgentRoot = "data/federated-agent-sessions"
	}
	federatedAgentStore, err := federatedagents.New(federatedAgentRoot)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	registerGitHTTP(mux, repositoryCatalog, credentials)
	registerRepositoriesHTTP(mux, repositoryCatalog, credentials)
	registerOrganizationsHTTP(mux, organizationStore, repositoryCatalog, userStore, packageStore, releaseStore, pullRequestStore, incidentStore, proposalStore, relationshipStore, securityReportStore, credentials, activityStore)
	registerGovernanceHTTP(mux, governanceStore, repositoryCatalog, organizationStore, credentials)
	registerRepositoryBrowserHTTP(mux, repositoryCatalog, credentials)
	registerCodeIntelligenceHTTP(mux, repositoryCatalog, credentials, relationshipStore)
	registerQuestionsHTTP(mux, questionStore, repositoryCatalog, credentials, relationshipStore, checkRunStore)
	registerInvestigationsHTTP(mux, investigationStore, repositoryCatalog, credentials, workspaceStore, questionStore)
	registerImpactAssessmentsHTTP(mux, impactStore, repositoryCatalog, credentials, relationshipStore, investigationStore, releaseStore, deploymentStore, packageStore)
	registerDecisionsHTTP(mux, decisionStore, repositoryCatalog, credentials, workspaceStore, workspaceRunner, proposalStore)
	registerDeliveryTeamsHTTP(mux, deliveryTeamStore, repositoryCatalog, credentials, organizationStore, deliveryExecutionStores{changes: changeSessionStore, investigations: investigationStore, decisions: decisionStore, workspaces: workspaceStore}, pullRequestStore, checkRunner)
	registerPerformanceGoalsHTTP(mux, performanceGoalStore, repositoryCatalog, releaseStore, credentials, pullRequestStore)
	registerAccessibilityCommitmentsHTTP(mux, accessibilityCommitmentStore, repositoryCatalog, credentials)
	registerAccessibilityBarriersHTTP(mux, accessibilityBarrierStore, repositoryCatalog, credentials, accessibilityBarrierSources{releases: releaseStore, docs: documentationStore, previews: previewStore, workspaces: workspaceStore, repositories: repositoryCatalog})
	registerProjectFundsHTTP(mux, projectFundStore, repositoryCatalog, credentials)
	registerPerformanceInvestigationsHTTP(mux, performanceInvestigationStore, performanceGoalStore, repositoryCatalog, credentials, proposalStore)
	registerProductExperimentsHTTP(mux, productExperimentStore, repositoryCatalog, credentials, pullRequestStore, releaseStore, deploymentStore)
	registerProductFeedbackHTTP(mux, productFeedbackStore, repositoryCatalog, credentials, feedbackSources{releases: releaseStore, docs: documentationStore, previews: previewStore, issues: issueStore, experiments: productExperimentStore, organizations: organizationStore})
	registerProductOpportunitiesHTTP(mux, productOpportunityStore, repositoryCatalog, credentials, opportunitySources{feedback: productFeedbackStore, issues: issueStore, previews: previewStore, experiments: productExperimentStore})
	registerProductRoadmapsHTTP(mux, productRoadmapStore, productOpportunityStore, repositoryCatalog, credentials)
	registerRoadmapValidationsHTTP(mux, roadmapValidationStore, productRoadmapStore, productFeedbackStore, repositoryCatalog, credentials)
	registerRoadmapDeliveryHTTP(mux, roadmapDeliveryStore, productRoadmapStore, proposalStore, repositoryCatalog, credentials)
	registerProductLearningHTTP(mux, productLearningStore, roadmapDeliveryStore, productFeedbackStore, productOpportunityStore, productRoadmapStore, repositoryCatalog, credentials)
	registerReasoningWorkHTTP(mux, investigationStore, impactStore, proposalStore, repositoryCatalog, credentials)
	registerCollaboratorsHTTP(mux, repositoryCatalog, userStore, credentials, activityStore)
	registerProposalsHTTP(mux, proposalStore, repositoryCatalog, credentials, activityStore)
	registerIssuesHTTP(mux, issueStore, releaseStore, repositoryCatalog, credentials, issueReproductionRunner)
	registerContributorPathwaysHTTP(mux, contributorPathwayStore, repositoryCatalog, credentials, releaseStore, issueStore, proposalStore)
	registerDocumentationHTTP(mux, documentationStore, repositoryCatalog, credentials, releaseStore, workspaceStore, workspaceRunner, pullRequestStore)
	registerContributionOpportunitiesHTTP(mux, contributionOpportunityStore, repositoryCatalog, credentials, issueStore, proposalStore, organizationStore, contributorPathwayStore, workspaceStore, workspaceRunner, pullRequestStore, checkRunner, releaseStore)
	registerIssueRepairsHTTP(mux, issueStore, proposalStore, pullRequestStore, repositoryCatalog, credentials, issueReproductionRunner, checkRunStore)
	registerProposalTaskSessionsHTTP(mux, proposalStore, changeSessionStore, repositoryCatalog, credentials, activityStore, pullRequestStore, checkRunner)
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, repositoryCatalog, credentials, activityStore, checkRunner, checkRunStore, integrationQueueStore, previewStore, federationStore, performanceGoalStore)
	registerPreviewsHTTP(mux, previewStore, previewRunner, pullRequestStore, repositoryCatalog, credentials, previewSources{issues: issueStore, decisions: decisionStore, proposals: proposalStore}, previewRepairStores{plans: proposalStore, sessions: changeSessionStore, workspaces: workspaceStore, workspaceRunner: workspaceRunner})
	registerReleasesHTTP(mux, releaseStore, checkRunStore, checkRunner, pullRequestStore, repositoryCatalog, credentials)
	registerPackagesHTTP(mux, packageStore, releaseStore, checkRunStore, repositoryCatalog, credentials)
	registerPackageRecoveryHTTP(mux, packageStore, dependencyInventoryStore, proposalStore, repositoryCatalog, credentials, activityStore)
	registerDependencyInventoryHTTP(mux, dependencyInventoryStore, packageStore, releaseStore, checkRunStore, deploymentStore, repositoryCatalog, credentials)
	registerDependencyUpdateHTTP(mux, dependencyUpdateStore, dependencyInventoryStore, packageStore, releaseStore, proposalStore, repositoryCatalog, credentials, activityStore)
	registerRelationshipsHTTP(mux, relationshipStore, releaseStore, deploymentStore, repositoryCatalog, proposalStore, pullRequestStore, credentials, changeSessionStore)
	registerEvolutionVerificationHTTP(mux, relationshipStore, repositoryCatalog, pullRequestStore, credentials, checkRunner, checkRunStore)
	registerEvolutionRolloutHTTP(mux, relationshipStore, repositoryCatalog, credentials, integrationQueueStore, releaseStore, deploymentStore, pullRequestStore, checkRunStore)
	registerDeploymentsHTTP(mux, deploymentStore, releaseStore, checkRunStore, repositoryCatalog, credentials, activityStore, changeSessionStore, pullRequestStore, packageSafetyEnforcer{inventories: dependencyInventoryStore, packages: packageStore})
	registerIncidentsHTTP(mux, incidentStore, deploymentStore, releaseStore, pullRequestStore, repositoryCatalog, credentials, proposalStore, activityStore, checkRunStore)
	registerWorkspacesHTTP(mux, workspaceStore, workspaceRunner, repositoryCatalog, credentials, proposalStore, pullRequestStore, incidentStore, organizationStore, checkRunner)
	registerSecurityReportsHTTP(mux, securityReportStore, repositoryCatalog, userStore, credentials, activityStore)
	registerChangeSessionsHTTP(mux, changeSessionStore, pullRequestStore, repositoryCatalog, credentials, activityStore, checkRunner)
	registerCheckRunsHTTP(mux, checkRunStore, checkRunner, pullRequestStore, repositoryCatalog, credentials, changeSessionStore, activityStore)
	registerActivitiesHTTP(mux, activityStore, repositoryCatalog, credentials)
	registerInboxHTTP(mux, activityStore, inboxStore, repositoryCatalog, proposalStore, pullRequestStore, userStore, credentials)
	registerUsersHTTP(mux, userStore, credentials)
	registerExtensionsHTTP(mux, extensionStore, repositoryCatalog, organizationStore, credentials, activityStore, pullRequestStore)
	registerFederationHTTP(mux, federationStore, credentials)
	registerFederatedRepositoriesHTTP(mux, federationStore, repositoryCatalog, pullRequestStore, releaseStore, contributorPathwayStore, issueStore, contributionOpportunityStore, activityStore, credentials)
	registerFederatedAgentSessionsHTTP(mux, federatedAgentStore, federationStore, repositoryCatalog, credentials)
	registerAuthHTTP(mux, credentials, userStore)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
