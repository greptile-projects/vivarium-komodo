package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilityassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitybarriers"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitycommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitypolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentdiscovery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentevaluations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprofiles"
	"github.com/greptile-projects/vivarium-komodo/apps/api/apiconsumers"
	"github.com/greptile-projects/vivarium-komodo/apps/api/apicontracts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/assurancedelivery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceevidence"
	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceprograms"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityinventories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityproofs"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityremovals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityretirements"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributionopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributorpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/datacommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dataflows"
	"github.com/greptile-projects/vivarium-komodo/apps/api/debuggingworkspaces"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deliveryteams"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyinventory"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyupdates"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designgovernance"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designproposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designsystems"
	"github.com/greptile-projects/vivarium-komodo/apps/api/docscollections"
	"github.com/greptile-projects/vivarium-komodo/apps/api/durableschemas"
	"github.com/greptile-projects/vivarium-komodo/apps/api/exploratorysessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/extensions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/federatedagents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/federation"
	"github.com/greptile-projects/vivarium-komodo/apps/api/governance"
	"github.com/greptile-projects/vivarium-komodo/apps/api/impactassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/inbox"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/independentassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/infrastructureplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/infrastructurestate"
	"github.com/greptile-projects/vivarium-komodo/apps/api/integrationqueue"
	"github.com/greptile-projects/vivarium-komodo/apps/api/interfacechecks"
	"github.com/greptile-projects/vivarium-komodo/apps/api/investigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/localeplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/localizationdelivery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/localizationverification"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/performancegoals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/performanceinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/privacyassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/privacydrift"
	"github.com/greptile-projects/vivarium-komodo/apps/api/privacyverification"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productexperiments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productfeedback"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productlearning"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productroadmaps"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectfunds"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/protectionplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/qualitygates"
	"github.com/greptile-projects/vivarium-komodo/apps/api/qualityplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/questions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryexercises"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryimprovements"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryobjectives"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryresponses"
	"github.com/greptile-projects/vivarium-komodo/apps/api/relationships"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reliabilityimprovements"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reliabilityinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reliabilitypolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/roadmapdelivery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/roadmapvalidations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runtimeinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runtimeprobes"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runtimerepairs"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runtimereplays"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securitydelivery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityexpectations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityreports"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityscenarios"
	"github.com/greptile-projects/vivarium-komodo/apps/api/serviceobjectives"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/supportquestions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/testscenarios"
	"github.com/greptile-projects/vivarium-komodo/apps/api/threatmodels"
	"github.com/greptile-projects/vivarium-komodo/apps/api/translationunits"
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
	agentProfileRoot := os.Getenv("AGENT_PROFILE_ROOT")
	if agentProfileRoot == "" {
		agentProfileRoot = "data/agent-profiles"
	}
	agentProfileStore, err := agentprofiles.New(agentProfileRoot)
	if err != nil {
		log.Fatal(err)
	}
	agentDiscoveryRoot := os.Getenv("AGENT_DISCOVERY_ROOT")
	if agentDiscoveryRoot == "" {
		agentDiscoveryRoot = "data/agent-discovery"
	}
	agentDiscoveryStore, err := agentdiscovery.New(agentDiscoveryRoot)
	if err != nil {
		log.Fatal(err)
	}
	agentEvaluationRoot := os.Getenv("AGENT_EVALUATION_ROOT")
	if agentEvaluationRoot == "" {
		agentEvaluationRoot = "data/agent-evaluations"
	}
	agentEvaluationStore, err := agentevaluations.New(agentEvaluationRoot)
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
	supportQuestionRoot := os.Getenv("SUPPORT_QUESTION_ROOT")
	if supportQuestionRoot == "" {
		supportQuestionRoot = "data/support-questions"
	}
	supportQuestionStore, err := supportquestions.New(supportQuestionRoot)
	if err != nil {
		log.Fatal(err)
	}
	supportVerificationRunner := supportquestions.NewVerificationRunner(supportQuestionStore, repositoryCatalog)
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
	dataCommitmentRoot := os.Getenv("DATA_COMMITMENT_ROOT")
	if dataCommitmentRoot == "" {
		dataCommitmentRoot = "data/data-commitments"
	}
	dataCommitmentStore, err := datacommitments.New(dataCommitmentRoot)
	if err != nil {
		log.Fatal(err)
	}
	localePlanRoot := os.Getenv("LOCALE_PLAN_ROOT")
	if localePlanRoot == "" {
		localePlanRoot = "data/locale-plans"
	}
	localePlanStore, err := localeplans.New(localePlanRoot)
	if err != nil {
		log.Fatal(err)
	}
	serviceObjectiveRoot := os.Getenv("SERVICE_OBJECTIVE_ROOT")
	if serviceObjectiveRoot == "" {
		serviceObjectiveRoot = "data/service-objectives"
	}
	serviceObjectiveStore, err := serviceobjectives.New(serviceObjectiveRoot)
	if err != nil {
		log.Fatal(err)
	}
	apiContractRoot := os.Getenv("API_CONTRACT_ROOT")
	if apiContractRoot == "" {
		apiContractRoot = "data/api-contracts"
	}
	apiContractStore, err := apicontracts.New(apiContractRoot)
	if err != nil {
		log.Fatal(err)
	}
	designSystemRoot := os.Getenv("DESIGN_SYSTEM_ROOT")
	if designSystemRoot == "" {
		designSystemRoot = "data/design-systems"
	}
	designSystemStore, err := designsystems.New(designSystemRoot)
	if err != nil {
		log.Fatal(err)
	}
	qualityPlanRoot := os.Getenv("QUALITY_PLAN_ROOT")
	if qualityPlanRoot == "" {
		qualityPlanRoot = "data/quality-plans"
	}
	qualityPlanStore, err := qualityplans.New(qualityPlanRoot)
	if err != nil {
		log.Fatal(err)
	}
	capabilityInventoryRoot := os.Getenv("CAPABILITY_INVENTORY_ROOT")
	if capabilityInventoryRoot == "" {
		capabilityInventoryRoot = "data/capability-inventories"
	}
	capabilityInventoryStore, err := capabilityinventories.New(capabilityInventoryRoot)
	if err != nil {
		log.Fatal(err)
	}
	assuranceProgramRoot := os.Getenv("ASSURANCE_PROGRAM_ROOT")
	if assuranceProgramRoot == "" {
		assuranceProgramRoot = "data/assurance-programs"
	}
	assuranceProgramStore, err := assuranceprograms.New(assuranceProgramRoot)
	if err != nil {
		log.Fatal(err)
	}
	assuranceEvidenceRoot := os.Getenv("ASSURANCE_EVIDENCE_ROOT")
	if assuranceEvidenceRoot == "" {
		assuranceEvidenceRoot = "data/assurance-evidence"
	}
	assuranceEvidenceStore, err := assuranceevidence.New(assuranceEvidenceRoot, assuranceProgramStore)
	if err != nil {
		log.Fatal(err)
	}
	assuranceAssessmentRoot := os.Getenv("ASSURANCE_ASSESSMENT_ROOT")
	if assuranceAssessmentRoot == "" {
		assuranceAssessmentRoot = "data/assurance-assessments"
	}
	assuranceAssessmentStore, err := assuranceassessments.New(assuranceAssessmentRoot, assuranceProgramStore)
	if err != nil {
		log.Fatal(err)
	}
	independentAssessmentRoot := os.Getenv("INDEPENDENT_ASSESSMENT_ROOT")
	if independentAssessmentRoot == "" {
		independentAssessmentRoot = "data/independent-assessments"
	}
	independentAssessmentStore, err := independentassessments.New(independentAssessmentRoot)
	if err != nil {
		log.Fatal(err)
	}
	capabilityRetirementRoot := os.Getenv("CAPABILITY_RETIREMENT_ROOT")
	if capabilityRetirementRoot == "" {
		capabilityRetirementRoot = "data/capability-retirements"
	}
	capabilityRetirementStore, err := capabilityretirements.New(capabilityRetirementRoot, capabilityInventoryStore)
	if err != nil {
		log.Fatal(err)
	}
	capabilityProofRoot := os.Getenv("CAPABILITY_PROOF_ROOT")
	if capabilityProofRoot == "" {
		capabilityProofRoot = "data/capability-proofs"
	}
	capabilityProofStore, err := capabilityproofs.New(capabilityProofRoot, capabilityRetirementStore)
	if err != nil {
		log.Fatal(err)
	}
	capabilityRemovalRoot := os.Getenv("CAPABILITY_REMOVAL_ROOT")
	if capabilityRemovalRoot == "" {
		capabilityRemovalRoot = "data/capability-removals"
	}
	capabilityRemovalStore, err := capabilityremovals.New(capabilityRemovalRoot, capabilityProofStore, capabilityRetirementStore)
	if err != nil {
		log.Fatal(err)
	}
	securityExpectationRoot := os.Getenv("SECURITY_EXPECTATION_ROOT")
	if securityExpectationRoot == "" {
		securityExpectationRoot = "data/security-expectations"
	}
	securityExpectationStore, err := securityexpectations.New(securityExpectationRoot)
	if err != nil {
		log.Fatal(err)
	}
	threatModelRoot := os.Getenv("THREAT_MODEL_ROOT")
	if threatModelRoot == "" {
		threatModelRoot = "data/threat-models"
	}
	threatModelStore, err := threatmodels.New(threatModelRoot)
	if err != nil {
		log.Fatal(err)
	}
	securityScenarioRoot := os.Getenv("SECURITY_SCENARIO_ROOT")
	if securityScenarioRoot == "" {
		securityScenarioRoot = "data/security-scenarios"
	}
	securityScenarioStore, err := securityscenarios.New(securityScenarioRoot)
	if err != nil {
		log.Fatal(err)
	}
	securityDeliveryRoot := os.Getenv("SECURITY_DELIVERY_ROOT")
	if securityDeliveryRoot == "" {
		securityDeliveryRoot = "data/security-delivery"
	}
	securityDeliveryStore, err := securitydelivery.New(securityDeliveryRoot)
	if err != nil {
		log.Fatal(err)
	}
	queueCoordinator.security = &securityDeliverySources{securityDeliveryStore, threatModelStore, securityScenarioStore}
	qualityGateRoot := os.Getenv("QUALITY_GATE_ROOT")
	if qualityGateRoot == "" {
		qualityGateRoot = "data/quality-gates"
	}
	qualityGateStore, err := qualitygates.New(qualityGateRoot)
	if err != nil {
		log.Fatal(err)
	}
	testScenarioRoot := os.Getenv("TEST_SCENARIO_ROOT")
	if testScenarioRoot == "" {
		testScenarioRoot = "data/test-scenarios"
	}
	testScenarioStore, err := testscenarios.New(testScenarioRoot)
	if err != nil {
		log.Fatal(err)
	}
	exploratorySessionRoot := os.Getenv("EXPLORATORY_SESSION_ROOT")
	if exploratorySessionRoot == "" {
		exploratorySessionRoot = "data/exploratory-sessions"
	}
	exploratorySessionStore, err := exploratorysessions.New(exploratorySessionRoot)
	if err != nil {
		log.Fatal(err)
	}
	designProposalRoot := os.Getenv("DESIGN_PROPOSAL_ROOT")
	if designProposalRoot == "" {
		designProposalRoot = "data/design-proposals"
	}
	designProposalStore, err := designproposals.New(designProposalRoot)
	if err != nil {
		log.Fatal(err)
	}
	interfaceCheckRoot := os.Getenv("INTERFACE_CHECK_ROOT")
	if interfaceCheckRoot == "" {
		interfaceCheckRoot = "data/interface-checks"
	}
	interfaceCheckStore, err := interfacechecks.New(interfaceCheckRoot)
	if err != nil {
		log.Fatal(err)
	}
	designGovernanceRoot := os.Getenv("DESIGN_GOVERNANCE_ROOT")
	if designGovernanceRoot == "" {
		designGovernanceRoot = "data/design-governance"
	}
	designGovernanceStore, err := designgovernance.New(designGovernanceRoot)
	if err != nil {
		log.Fatal(err)
	}
	apiConsumerRoot := os.Getenv("API_CONSUMER_ROOT")
	if apiConsumerRoot == "" {
		apiConsumerRoot = "data/api-consumers"
	}
	apiConsumerStore, err := apiconsumers.New(apiConsumerRoot, apiContractStore)
	if err != nil {
		log.Fatal(err)
	}
	durableSchemaRoot := os.Getenv("DURABLE_SCHEMA_ROOT")
	if durableSchemaRoot == "" {
		durableSchemaRoot = "data/durable-schemas"
	}
	durableSchemaStore, err := durableschemas.New(durableSchemaRoot)
	if err != nil {
		log.Fatal(err)
	}
	infrastructureStateRoot := os.Getenv("INFRASTRUCTURE_STATE_ROOT")
	if infrastructureStateRoot == "" {
		infrastructureStateRoot = "data/infrastructure-state"
	}
	infrastructureStateStore, err := infrastructurestate.New(infrastructureStateRoot)
	if err != nil {
		log.Fatal(err)
	}
	infrastructurePlanRoot := os.Getenv("INFRASTRUCTURE_PLAN_ROOT")
	if infrastructurePlanRoot == "" {
		infrastructurePlanRoot = "data/infrastructure-plans"
	}
	infrastructurePlanStore, err := infrastructureplans.New(infrastructurePlanRoot, infrastructurePlanPulls{pullRequestStore}, infrastructureStateStore)
	if err != nil {
		log.Fatal(err)
	}
	infrastructurePlanStore.ConfigureExecutionAuthority(infrastructureExecutionEnvironments{deploymentStore})
	recoveryObjectiveRoot := os.Getenv("RECOVERY_OBJECTIVE_ROOT")
	if recoveryObjectiveRoot == "" {
		recoveryObjectiveRoot = "data/recovery-objectives"
	}
	recoveryObjectiveStore, err := recoveryobjectives.New(recoveryObjectiveRoot)
	if err != nil {
		log.Fatal(err)
	}
	protectionPlanRoot := os.Getenv("PROTECTION_PLAN_ROOT")
	if protectionPlanRoot == "" {
		protectionPlanRoot = "data/protection-plans"
	}
	protectionPlanStore, err := protectionplans.New(protectionPlanRoot)
	if err != nil {
		log.Fatal(err)
	}
	recoveryExerciseRoot := os.Getenv("RECOVERY_EXERCISE_ROOT")
	if recoveryExerciseRoot == "" {
		recoveryExerciseRoot = "data/recovery-exercises"
	}
	recoveryExerciseStore, err := recoveryexercises.New(recoveryExerciseRoot, protectionPlanStore)
	if err != nil {
		log.Fatal(err)
	}
	recoveryInvestigationRoot := os.Getenv("RECOVERY_INVESTIGATION_ROOT")
	if recoveryInvestigationRoot == "" {
		recoveryInvestigationRoot = "data/recovery-investigations"
	}
	recoveryInvestigationStore, err := recoveryinvestigations.New(recoveryInvestigationRoot)
	if err != nil {
		log.Fatal(err)
	}
	recoveryImprovementRoot := os.Getenv("RECOVERY_IMPROVEMENT_ROOT")
	if recoveryImprovementRoot == "" {
		recoveryImprovementRoot = "data/recovery-improvements"
	}
	recoveryImprovementStore, err := recoveryimprovements.New(recoveryImprovementRoot)
	if err != nil {
		log.Fatal(err)
	}
	recoveryResponseRoot := os.Getenv("RECOVERY_RESPONSE_ROOT")
	if recoveryResponseRoot == "" {
		recoveryResponseRoot = "data/recovery-responses"
	}
	recoveryResponseStore, err := recoveryresponses.New(recoveryResponseRoot, protectionPlanStore)
	if err != nil {
		log.Fatal(err)
	}
	reliabilityInvestigationRoot := os.Getenv("RELIABILITY_INVESTIGATION_ROOT")
	if reliabilityInvestigationRoot == "" {
		reliabilityInvestigationRoot = "data/reliability-investigations"
	}
	reliabilityInvestigationStore, err := reliabilityinvestigations.New(reliabilityInvestigationRoot)
	if err != nil {
		log.Fatal(err)
	}
	debuggingWorkspaceRoot := os.Getenv("DEBUGGING_WORKSPACE_ROOT")
	if debuggingWorkspaceRoot == "" {
		debuggingWorkspaceRoot = "data/debugging-workspaces"
	}
	debuggingWorkspaceStore, err := debuggingworkspaces.New(debuggingWorkspaceRoot)
	if err != nil {
		log.Fatal(err)
	}
	runtimeProbeRoot := os.Getenv("RUNTIME_PROBE_ROOT")
	if runtimeProbeRoot == "" {
		runtimeProbeRoot = "data/runtime-probes"
	}
	runtimeProbeStore, err := runtimeprobes.New(runtimeProbeRoot)
	if err != nil {
		log.Fatal(err)
	}
	runtimeInvestigationRoot := os.Getenv("RUNTIME_INVESTIGATION_ROOT")
	if runtimeInvestigationRoot == "" {
		runtimeInvestigationRoot = "data/runtime-investigations"
	}
	runtimeInvestigationStore, err := runtimeinvestigations.New(runtimeInvestigationRoot)
	if err != nil {
		log.Fatal(err)
	}
	runtimeReplayRoot := os.Getenv("RUNTIME_REPLAY_ROOT")
	if runtimeReplayRoot == "" {
		runtimeReplayRoot = "data/runtime-replays"
	}
	runtimeReplayStore, err := runtimereplays.New(runtimeReplayRoot)
	if err != nil {
		log.Fatal(err)
	}
	runtimeRepairRoot := os.Getenv("RUNTIME_REPAIR_ROOT")
	if runtimeRepairRoot == "" {
		runtimeRepairRoot = "data/runtime-repairs"
	}
	runtimeRepairStore, err := runtimerepairs.New(runtimeRepairRoot)
	if err != nil {
		log.Fatal(err)
	}
	reliabilityPolicyRoot := os.Getenv("RELIABILITY_POLICY_ROOT")
	if reliabilityPolicyRoot == "" {
		reliabilityPolicyRoot = "data/reliability-policies"
	}
	reliabilityPolicyStore, err := reliabilitypolicies.New(reliabilityPolicyRoot)
	if err != nil {
		log.Fatal(err)
	}
	reliabilityImprovementRoot := os.Getenv("RELIABILITY_IMPROVEMENT_ROOT")
	if reliabilityImprovementRoot == "" {
		reliabilityImprovementRoot = "data/reliability-improvements"
	}
	reliabilityImprovementStore, err := reliabilityimprovements.New(reliabilityImprovementRoot)
	if err != nil {
		log.Fatal(err)
	}
	translationUnitRoot := os.Getenv("TRANSLATION_UNIT_ROOT")
	if translationUnitRoot == "" {
		translationUnitRoot = "data/translation-units"
	}
	translationUnitStore, err := translationunits.New(translationUnitRoot)
	if err != nil {
		log.Fatal(err)
	}
	localizationVerificationRoot := os.Getenv("LOCALIZATION_VERIFICATION_ROOT")
	if localizationVerificationRoot == "" {
		localizationVerificationRoot = "data/localization-verification"
	}
	localizationVerificationStore, err := localizationverification.New(localizationVerificationRoot)
	if err != nil {
		log.Fatal(err)
	}
	localizationDeliveryRoot := os.Getenv("LOCALIZATION_DELIVERY_ROOT")
	if localizationDeliveryRoot == "" {
		localizationDeliveryRoot = "data/localization-delivery"
	}
	localizationDeliveryStore, err := localizationdelivery.New(localizationDeliveryRoot)
	if err != nil {
		log.Fatal(err)
	}
	dataFlowRoot := os.Getenv("DATA_FLOW_ROOT")
	if dataFlowRoot == "" {
		dataFlowRoot = "data/data-flows"
	}
	dataFlowStore, err := dataflows.New(dataFlowRoot)
	if err != nil {
		log.Fatal(err)
	}
	privacyAssessmentRoot := os.Getenv("PRIVACY_ASSESSMENT_ROOT")
	if privacyAssessmentRoot == "" {
		privacyAssessmentRoot = "data/privacy-assessments"
	}
	privacyAssessmentStore, err := privacyassessments.New(privacyAssessmentRoot)
	if err != nil {
		log.Fatal(err)
	}
	privacyVerificationRoot := os.Getenv("PRIVACY_VERIFICATION_ROOT")
	if privacyVerificationRoot == "" {
		privacyVerificationRoot = "data/privacy-verifications"
	}
	privacyVerificationStore, err := privacyverification.New(privacyVerificationRoot)
	if err != nil {
		log.Fatal(err)
	}
	privacyDriftRoot := os.Getenv("PRIVACY_DRIFT_ROOT")
	if privacyDriftRoot == "" {
		privacyDriftRoot = "data/privacy-drift"
	}
	privacyDriftStore, err := privacydrift.New(privacyDriftRoot)
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
	accessibilityAssessmentRoot := os.Getenv("ACCESSIBILITY_ASSESSMENT_ROOT")
	if accessibilityAssessmentRoot == "" {
		accessibilityAssessmentRoot = "data/accessibility-assessments"
	}
	accessibilityAssessmentStore, err := accessibilityassessments.New(accessibilityAssessmentRoot)
	if err != nil {
		log.Fatal(err)
	}
	accessibilityPolicyRoot := os.Getenv("ACCESSIBILITY_POLICY_ROOT")
	if accessibilityPolicyRoot == "" {
		accessibilityPolicyRoot = "data/accessibility-policies"
	}
	accessibilityPolicyStore, err := accessibilitypolicies.New(accessibilityPolicyRoot)
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
	assuranceDeliveryRoot := os.Getenv("ASSURANCE_DELIVERY_ROOT")
	if assuranceDeliveryRoot == "" {
		assuranceDeliveryRoot = "data/assurance-delivery"
	}
	assuranceDeliveryStore, err := assurancedelivery.New(assuranceDeliveryRoot, assuranceFindingSource{independentAssessmentStore}, federationStore)
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
	registerDataCommitmentsHTTP(mux, dataCommitmentStore, repositoryCatalog, credentials)
	registerLocalePlansHTTP(mux, localePlanStore, repositoryCatalog, credentials)
	registerServiceObjectivesHTTP(mux, serviceObjectiveStore, repositoryCatalog, credentials)
	registerAPIContractsHTTP(mux, apiContractStore, repositoryCatalog, credentials)
	registerDesignSystemsHTTP(mux, designSystemStore, repositoryCatalog, credentials)
	registerQualityPlansHTTP(mux, qualityPlanStore, repositoryCatalog, credentials)
	registerCapabilityInventoriesHTTP(mux, capabilityInventoryStore, repositoryCatalog, credentials)
	registerAssuranceProgramsHTTP(mux, assuranceProgramStore, repositoryCatalog, credentials)
	registerAssuranceEvidenceHTTP(mux, assuranceEvidenceStore, repositoryCatalog, credentials)
	registerAssuranceAssessmentsHTTP(mux, assuranceAssessmentStore, repositoryCatalog, credentials)
	registerIndependentAssessmentsHTTP(mux, independentAssessmentStore, assuranceEvidenceStore, repositoryCatalog, credentials)
	registerAssuranceDeliveryHTTP(mux, assuranceDeliveryStore, independentAssessmentStore, repositoryCatalog, credentials)
	registerCapabilityRetirementsHTTP(mux, capabilityRetirementStore, repositoryCatalog, credentials)
	registerCapabilityProofsHTTP(mux, capabilityProofStore, repositoryCatalog, credentials)
	registerCapabilityRemovalsHTTP(mux, capabilityRemovalStore, repositoryCatalog, credentials)
	registerSecurityExpectationsHTTP(mux, securityExpectationStore, repositoryCatalog, credentials)
	registerThreatModelsHTTP(mux, threatModelStore, repositoryCatalog, credentials, threatModelSources{pulls: pullRequestStore, plans: proposalStore, scenarios: securityScenarioStore})
	registerSecurityScenariosHTTP(mux, securityScenarioStore, threatModelStore, repositoryCatalog, credentials, pullRequestStore, previewStore)
	registerSecurityDeliveryHTTP(mux, securityDeliveryStore, threatModelStore, securityScenarioStore, repositoryCatalog, organizationStore, credentials)
	registerQualityGatesHTTP(mux, qualityGateStore, repositoryCatalog, credentials)
	registerTestScenariosHTTP(mux, testScenarioStore, repositoryCatalog, credentials)
	registerExploratorySessionsHTTP(mux, exploratorySessionStore, repositoryCatalog, credentials, issueStore, proposalStore)
	registerDesignProposalsHTTP(mux, designProposalStore, repositoryCatalog, credentials, proposalStore, pullRequestStore)
	registerInterfaceChecksHTTP(mux, interfaceCheckStore, repositoryCatalog, credentials, interfaceCheckSources{pulls: pullRequestStore, repositories: repositoryCatalog, designs: designProposalStore})
	registerDesignGovernanceHTTP(mux, designGovernanceStore, interfaceCheckStore, repositoryCatalog, organizationStore, credentials, pullRequestStore)
	registerAPIConsumersHTTP(mux, apiConsumerStore, repositoryCatalog, credentials)
	registerDurableSchemasHTTP(mux, durableSchemaStore, repositoryCatalog, credentials, deploymentStore)
	registerInfrastructureStateHTTP(mux, infrastructureStateStore, repositoryCatalog, credentials)
	registerInfrastructurePlansHTTP(mux, infrastructurePlanStore, repositoryCatalog, credentials)
	registerRecoveryObjectivesHTTP(mux, recoveryObjectiveStore, repositoryCatalog, credentials)
	registerProtectionPlansHTTP(mux, protectionPlanStore, recoveryObjectiveStore, repositoryCatalog, credentials)
	registerRecoveryExercisesHTTP(mux, recoveryExerciseStore, repositoryCatalog, credentials)
	registerRecoveryInvestigationsHTTP(mux, recoveryInvestigationStore, recoveryExerciseStore, repositoryCatalog, credentials)
	registerRecoveryImprovementsHTTP(mux, recoveryImprovementStore, recoveryInvestigationStore, recoveryExerciseStore, proposalStore, repositoryCatalog, credentials)
	registerRecoveryResponsesHTTP(mux, recoveryResponseStore, repositoryCatalog, credentials)
	registerReliabilityPoliciesHTTP(mux, reliabilityPolicyStore, serviceObjectiveStore, repositoryCatalog, credentials)
	registerReliabilityInvestigationsHTTP(mux, reliabilityInvestigationStore, serviceObjectiveStore, repositoryCatalog, credentials)
	registerDebuggingWorkspacesHTTP(mux, debuggingWorkspaceStore, repositoryCatalog, credentials, releaseStore)
	registerRuntimeProbesHTTP(mux, runtimeProbeStore, debuggingWorkspaceStore, repositoryCatalog, credentials)
	registerRuntimeInvestigationsHTTP(mux, runtimeInvestigationStore, debuggingWorkspaceStore, runtimeProbeStore, repositoryCatalog, credentials)
	registerRuntimeReplaysHTTP(mux, runtimeReplayStore, debuggingWorkspaceStore, runtimeProbeStore, runtimeInvestigationStore, workspaceStore, previewStore, repositoryCatalog, credentials)
	registerRuntimeRepairsHTTP(mux, runtimeRepairStore, debuggingWorkspaceStore, runtimeReplayStore, runtimeInvestigationStore, proposalStore, pullRequestStore, checkRunStore, releaseStore, deploymentStore, repositoryCatalog, credentials)
	registerReliabilityImprovementsHTTP(mux, reliabilityImprovementStore, reliabilityInvestigationStore, serviceObjectiveStore, proposalStore, repositoryCatalog, credentials)
	registerTranslationUnitsHTTP(mux, translationUnitStore, repositoryCatalog, credentials, translationUnitSources{pulls: pullRequestStore, repositories: repositoryCatalog, plans: localePlanStore})
	registerLocalizationVerificationHTTP(mux, localizationVerificationStore, repositoryCatalog, credentials, localizationVerificationSources{pulls: pullRequestStore, repositories: repositoryCatalog, translations: translationUnitStore, previews: previewStore})
	registerLocalizationDeliveryHTTP(mux, localizationDeliveryStore, repositoryCatalog, credentials, localizationDeliverySources{pulls: pullRequestStore, verification: localizationVerificationStore, releases: releaseStore, docs: documentationStore, proposals: proposalStore})
	registerDataFlowsHTTP(mux, dataFlowStore, dataCommitmentStore, repositoryCatalog, credentials)
	registerPrivacyAssessmentsHTTP(mux, privacyAssessmentStore, repositoryCatalog, credentials, privacyAssessmentSources{pulls: pullRequestStore, flows: dataFlowStore, commitments: dataCommitmentStore, repositories: repositoryCatalog})
	registerPrivacyVerificationHTTP(mux, privacyVerificationStore, repositoryCatalog, credentials, dataCommitmentStore, previewStore, checkRunStore, pullRequestStore)
	registerPrivacyDriftHTTP(mux, privacyDriftStore, repositoryCatalog, credentials, dataCommitmentStore, proposalStore)
	registerAccessibilityBarriersHTTP(mux, accessibilityBarrierStore, repositoryCatalog, credentials, accessibilityBarrierSources{releases: releaseStore, docs: documentationStore, previews: previewStore, workspaces: workspaceStore, repositories: repositoryCatalog})
	registerAccessibilityAssessmentsHTTP(mux, accessibilityAssessmentStore, repositoryCatalog, credentials, accessibilityAssessmentSources{pulls: pullRequestStore, runs: checkRunStore, previews: previewStore, barriers: accessibilityBarrierStore, repositories: repositoryCatalog, commitments: accessibilityCommitmentStore, plans: proposalStore, sessions: changeSessionStore, workspaces: workspaceStore, workspaceRunner: workspaceRunner})
	registerAccessibilityPoliciesHTTP(mux, accessibilityPolicyStore, repositoryCatalog, credentials, accessibilityCommitmentStore, previewStore, accessibilityAssessmentStore, checkRunStore, pullRequestStore)
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
	registerSupportQuestionsHTTP(mux, supportQuestionStore, repositoryCatalog, credentials, supportSources{releases: releaseStore, packages: packageStore, docs: documentationStore, issues: issueStore, proposals: proposalStore, docsTasks: documentationStore}, supportVerificationRunner)
	registerContributorPathwaysHTTP(mux, contributorPathwayStore, repositoryCatalog, credentials, releaseStore, issueStore, proposalStore)
	registerDocumentationHTTP(mux, documentationStore, repositoryCatalog, credentials, releaseStore, workspaceStore, workspaceRunner, pullRequestStore)
	registerContributionOpportunitiesHTTP(mux, contributionOpportunityStore, repositoryCatalog, credentials, issueStore, proposalStore, organizationStore, contributorPathwayStore, workspaceStore, workspaceRunner, pullRequestStore, checkRunner, releaseStore)
	registerIssueRepairsHTTP(mux, issueStore, proposalStore, pullRequestStore, repositoryCatalog, credentials, issueReproductionRunner, checkRunStore)
	registerProposalTaskSessionsHTTP(mux, proposalStore, changeSessionStore, repositoryCatalog, credentials, activityStore, pullRequestStore, checkRunner)
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, repositoryCatalog, credentials, activityStore, checkRunner, checkRunStore, integrationQueueStore, previewStore, federationStore, performanceGoalStore, accessibilityPolicyStore, accessibilityAssessmentStore, privacyVerificationStore, localizationDeliveryStore, localizationVerificationStore, designGovernanceStore, interfaceCheckStore, securityDeliverySources{securityDeliveryStore, threatModelStore, securityScenarioStore})
	registerPreviewsHTTP(mux, previewStore, previewRunner, pullRequestStore, repositoryCatalog, credentials, previewSources{issues: issueStore, decisions: decisionStore, proposals: proposalStore}, previewRepairStores{plans: proposalStore, sessions: changeSessionStore, workspaces: workspaceStore, workspaceRunner: workspaceRunner})
	registerReleasesHTTP(mux, releaseStore, checkRunStore, checkRunner, pullRequestStore, repositoryCatalog, credentials, securityDeliverySources{securityDeliveryStore, threatModelStore, securityScenarioStore})
	registerPackagesHTTP(mux, packageStore, releaseStore, checkRunStore, repositoryCatalog, credentials)
	registerPackageRecoveryHTTP(mux, packageStore, dependencyInventoryStore, proposalStore, repositoryCatalog, credentials, activityStore)
	registerDependencyInventoryHTTP(mux, dependencyInventoryStore, packageStore, releaseStore, checkRunStore, deploymentStore, repositoryCatalog, credentials)
	registerDependencyUpdateHTTP(mux, dependencyUpdateStore, dependencyInventoryStore, packageStore, releaseStore, proposalStore, repositoryCatalog, credentials, activityStore)
	registerRelationshipsHTTP(mux, relationshipStore, releaseStore, deploymentStore, repositoryCatalog, proposalStore, pullRequestStore, credentials, changeSessionStore)
	registerEvolutionVerificationHTTP(mux, relationshipStore, repositoryCatalog, pullRequestStore, credentials, checkRunner, checkRunStore)
	registerEvolutionRolloutHTTP(mux, relationshipStore, repositoryCatalog, credentials, integrationQueueStore, releaseStore, deploymentStore, pullRequestStore, checkRunStore)
	registerDeploymentsHTTP(mux, deploymentStore, releaseStore, checkRunStore, repositoryCatalog, credentials, activityStore, changeSessionStore, pullRequestStore, packageSafetyEnforcer{inventories: dependencyInventoryStore, packages: packageStore}, securityDeliverySources{securityDeliveryStore, threatModelStore, securityScenarioStore})
	registerIncidentsHTTP(mux, incidentStore, deploymentStore, releaseStore, pullRequestStore, repositoryCatalog, credentials, proposalStore, activityStore, checkRunStore)
	registerWorkspacesHTTP(mux, workspaceStore, workspaceRunner, repositoryCatalog, credentials, proposalStore, pullRequestStore, incidentStore, organizationStore, checkRunner)
	registerSecurityReportsHTTP(mux, securityReportStore, repositoryCatalog, userStore, credentials, activityStore)
	registerChangeSessionsHTTP(mux, changeSessionStore, pullRequestStore, repositoryCatalog, credentials, activityStore, checkRunner)
	registerCheckRunsHTTP(mux, checkRunStore, checkRunner, pullRequestStore, repositoryCatalog, credentials, changeSessionStore, activityStore)
	registerActivitiesHTTP(mux, activityStore, repositoryCatalog, credentials)
	registerInboxHTTP(mux, activityStore, inboxStore, repositoryCatalog, proposalStore, pullRequestStore, userStore, credentials)
	registerUsersHTTP(mux, userStore, credentials)
	registerAgentProfilesHTTP(mux, agentProfileStore, credentials, userStore)
	registerAgentDiscoveryHTTP(mux, agentDiscoveryStore, agentProfileStore, repositoryCatalog, credentials)
	registerAgentEvaluationsHTTP(mux, agentEvaluationStore, agentProfileStore, repositoryCatalog, organizationStore, credentials)
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
