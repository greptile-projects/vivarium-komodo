package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deliveryteams"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyinventory"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyupdates"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/impactassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/inbox"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/integrationqueue"
	"github.com/greptile-projects/vivarium-komodo/apps/api/investigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/questions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/relationships"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	registerGitHTTP(mux, repositoryCatalog, credentials)
	registerRepositoriesHTTP(mux, repositoryCatalog, credentials)
	registerOrganizationsHTTP(mux, organizationStore, repositoryCatalog, userStore, packageStore, releaseStore, pullRequestStore, incidentStore, proposalStore, relationshipStore, securityReportStore, credentials, activityStore)
	registerRepositoryBrowserHTTP(mux, repositoryCatalog, credentials)
	registerCodeIntelligenceHTTP(mux, repositoryCatalog, credentials, relationshipStore)
	registerQuestionsHTTP(mux, questionStore, repositoryCatalog, credentials, relationshipStore, checkRunStore)
	registerInvestigationsHTTP(mux, investigationStore, repositoryCatalog, credentials, workspaceStore, questionStore)
	registerImpactAssessmentsHTTP(mux, impactStore, repositoryCatalog, credentials, relationshipStore, investigationStore, releaseStore, deploymentStore, packageStore)
	registerDecisionsHTTP(mux, decisionStore, repositoryCatalog, credentials, workspaceStore, workspaceRunner, proposalStore)
	registerDeliveryTeamsHTTP(mux, deliveryTeamStore, repositoryCatalog, credentials, organizationStore)
	registerReasoningWorkHTTP(mux, investigationStore, impactStore, proposalStore, repositoryCatalog, credentials)
	registerCollaboratorsHTTP(mux, repositoryCatalog, userStore, credentials, activityStore)
	registerProposalsHTTP(mux, proposalStore, repositoryCatalog, credentials, activityStore)
	registerProposalTaskSessionsHTTP(mux, proposalStore, changeSessionStore, repositoryCatalog, credentials, activityStore, pullRequestStore, checkRunner)
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, repositoryCatalog, credentials, activityStore, checkRunner, checkRunStore, integrationQueueStore)
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
