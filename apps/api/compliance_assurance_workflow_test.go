package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/assurancedelivery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceevidence"
	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceprograms"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/independentassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type assuranceWorkflowSigner struct{}

func (assuranceWorkflowSigner) Sign(payload []byte) (string, string, error) {
	sum := sha256.Sum256(payload)
	return "test-ed25519-key", hex.EncodeToString(sum[:]), nil
}

// TestComplianceAssuranceWorkflow is the black-box boundary for the complete
// obligation-to-verifiable-assurance loop. It crosses the public Assurance and
// independent-assessor APIs and uses stock Git identities for the assessed and
// repaired release revisions.
func TestComplianceAssuranceWorkflow(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "payments", Visibility: repositories.Public})
	for _, actor := range []string{"control-owner", "agent", "developer"} {
		_, _ = repos.AddCollaborator("owner", repo.ID, actor)
	}
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	controlOwner := issueAccess(t, credentials, "control-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "agent", auth.API, auth.RepositoryRead)
	developer := issueAccess(t, credentials, "developer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)

	programs, _ := assuranceprograms.New(t.TempDir())
	evidence, _ := assuranceevidence.New(t.TempDir(), programs)
	impacts, _ := assuranceassessments.New(t.TempDir(), programs)
	independent, _ := independentassessments.New(t.TempDir())
	delivery, _ := assurancedelivery.New(t.TempDir(), assuranceFindingSource{independent}, assuranceWorkflowSigner{})
	mux := http.NewServeMux()
	registerAssuranceProgramsHTTP(mux, programs, repos, credentials)
	registerAssuranceEvidenceHTTP(mux, evidence, repos, credentials)
	registerAssuranceAssessmentsHTTP(mux, impacts, repos, credentials)
	registerIndependentAssessmentsHTTP(mux, independent, evidence, repos, credentials)
	registerAssuranceDeliveryHTTP(mux, delivery, independent, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	root := "/repositories/" + string(repo.ID)

	opened, _ := repos.Open(repo.ID)
	commit := func(parent storage.ObjectID, body, message, author string) storage.ObjectID {
		blob, _ := opened.WriteObject(storage.BlobObject, []byte(body))
		tree, _ := opened.WriteObject(storage.TreeObject, treeEntry("100644", "retention.txt", blob))
		parentLine := ""
		if parent != "" {
			parentLine = "parent " + string(parent) + "\n"
		}
		value, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\n"+parentLine+"author "+author+" <"+author+"@example.test> 1 +0000\ncommitter "+author+" <"+author+"@example.test> 1 +0000\n\n"+message+"\n"))
		return value
	}
	base := commit("", "release records expire after seven days\n", "release candidate", "developer")
	repair := commit(base, "release records are retained for one year and deletion is audited\n", "agent-authored retention repair", "agent")
	release := commit(repair, "release v2.4.0: retention control enabled\n", "reviewed release", "developer")

	now := time.Now().UTC()
	periodStart, periodEnd := now.Add(-24*time.Hour), now.Add(time.Hour)
	definition := assuranceprograms.Input{Name: "Payments service assurance", Description: "Contract and regulatory commitments for the released payments service", Scope: "payments production release v2.4.0", ChangeReason: "map applicable retention obligation", OwnerIDs: []string{"owner"}, Requirements: []assuranceprograms.Requirement{{ID: "records", SourceKind: "regulatory", SourceReference: "payments-records/12", SourceVersion: "2026", Title: "Release record retention", Text: "retain release and deployment proof", Applicability: "payments production", Interpretation: "retain immutable delivery evidence for one year", AuthorID: "owner"}}, Controls: []assuranceprograms.Control{{ID: "retention", Objective: "Retain delivery proof", Claim: "Every production release has review, verification, release, and deployment evidence", ReviewPeriod: "each release", RequirementIDs: []string{"records"}, OwnerIDs: []string{"control-owner"}, Targets: []assuranceprograms.Target{{Kind: "release", Reference: "payments-v2.4.0", Revision: string(release)}, {Kind: "repository", Reference: string(repo.ID), Revision: string(release)}, {Kind: "procedure", Reference: "retention-policy", Revision: "2026-08"}}, EvidenceCriteria: []assuranceprograms.EvidenceCriterion{{ID: "delivery", Kind: "release", Description: "ordinary review through deployment", Frequency: "each release", SourceReference: "delivery:payments-v2.4.0"}}}}, Exceptions: []assuranceprograms.Exception{{ID: "short-retention", RequirementID: "records", ControlID: "retention", Rationale: "retain only seven days", OwnerID: "control-owner", ApprovalReference: "decision:pending", ExpiresAt: now.Add(7 * 24 * time.Hour)}}}
	var program assuranceprograms.Program
	workflowValue(t, server.URL, http.MethodPost, root+"/assurance-programs", owner, definition, http.StatusCreated, &program)
	if program.ClaimStatus != "gaps_explicit" || len(program.Gaps) != 1 || program.Gaps[0].Kind != "expiring_exception" || program.Versions[0].Requirements[0].SourceReference != "payments-records/12" {
		t.Fatalf("obligation mapping or rejected-exception warning was lost: %#v", program)
	}

	evidenceBase := root + "/assurance-programs/" + program.ID + "/evidence"
	var query assuranceevidence.Query
	workflowValue(t, server.URL, http.MethodPost, evidenceBase+"/queries", controlOwner, assuranceevidence.QueryInput{ControlVersion: 1, ControlID: "retention", Name: "reviewed release delivery", Kind: "release", Source: "releases/payments-v2.4.0", Schedule: "each release", FreshnessHours: 48, Audience: "repository", Transformations: []string{"metadata-only", "remove actor email"}}, http.StatusCreated, &query)
	var missing assuranceevidence.Package
	workflowValue(t, server.URL, http.MethodPost, evidenceBase+"/packages", controlOwner, assuranceevidence.CollectInput{ControlVersion: 1, ControlID: "retention", PeriodStart: periodStart, PeriodEnd: periodEnd}, http.StatusCreated, &missing)
	if missing.Fresh || missing.Coverage[query.ID] != "gap" {
		t.Fatalf("missing evidence became assurance: %#v", missing)
	}
	// Restricted content is rejected instead of being copied into assurance.
	unsafe := assuranceevidence.Record{QueryID: query.ID, SourceRecordID: "deployment-secret", SourceRevision: string(base), ObservedAt: now, SourceDigest: digest64("secret"), SourceAttestation: "delivery-attestation", Audience: "repository", Accessible: true, ContainsCredentials: true, Result: "passed"}
	workflowValue(t, server.URL, http.MethodPost, evidenceBase+"/packages", controlOwner, assuranceevidence.CollectInput{ControlVersion: 1, ControlID: "retention", PeriodStart: periodStart, PeriodEnd: periodEnd, Records: []assuranceevidence.Record{unsafe}}, http.StatusUnprocessableEntity, nil)

	impactInput := assuranceassessments.Input{CandidateKind: "release_candidate", CandidateID: "payments-v2.4.0", CandidateRevision: string(base), ProgramID: program.ID, ProgramVersion: 1, Summary: "agent assesses the candidate against the mapped obligation", Inputs: []assuranceassessments.BoundInput{{Key: "candidate", Revision: string(base)}, {Key: "program", Revision: "1"}, {Key: "retention-policy", Revision: "2026-08"}}, Impacts: []assuranceassessments.Impact{{ControlID: "retention", RequirementIDs: []string{"records"}, Rationale: "candidate shortens the required retention period", ChangedEvidence: []string{missing.ID}, RequiredOwnerIDs: []string{"control-owner"}, InputKeys: []string{"candidate", "program", "retention-policy"}, RequiredForReadiness: true, Actions: []assuranceassessments.Action{{ID: "exception", Kind: "exception", Description: "reject the seven-day exception", OwnerIDs: []string{"control-owner"}, Required: true}, {ID: "delivery-proof", Kind: "evidence", Description: "collect reviewed deployed revision", OwnerIDs: []string{"control-owner"}, Required: true}}}}}
	var impact assuranceassessments.Assessment
	workflowValue(t, server.URL, http.MethodPost, root+"/assurance-assessments", developer, impactInput, http.StatusCreated, &impact)
	workflowValue(t, server.URL, http.MethodPost, root+"/assurance-assessments/"+impact.ID+"/annotations", agent, assuranceassessments.AnnotationInput{Kind: "challenge", Body: "the seven-day exception contradicts the mapped one-year obligation", ControlIDs: []string{"retention"}, Citations: []assuranceassessments.Citation{{Reference: "retention.txt", Revision: string(base), Audience: "repository"}}}, http.StatusCreated, &impact)
	workflowValue(t, server.URL, http.MethodPost, root+"/assurance-assessments/"+impact.ID+"/decisions", controlOwner, map[string]any{"control_id": "retention", "decision": "request_changes", "rationale": "exception rejected; repair and delivery proof required"}, http.StatusCreated, &impact)
	workflowValue(t, server.URL, http.MethodPost, root+"/assurance-assessments/"+impact.ID+"/revisions", developer, map[string]any{"expected_revision": string(base), "candidate_revision": string(repair), "inputs": []assuranceassessments.BoundInput{{Key: "candidate", Revision: string(repair)}, {Key: "program", Revision: "1"}, {Key: "retention-policy", Revision: "2026-08"}}}, http.StatusCreated, &impact)
	if !impact.Decisions[0].Stale || impact.Ready {
		t.Fatalf("candidate drift retained the old control decision: %#v", impact)
	}

	record := assuranceevidence.Record{QueryID: query.ID, SourceRecordID: "delivery:v2.4.0", SourceRevision: string(release), ObservedAt: now, SourceDigest: digest64(string(release)), SourceAttestation: "review-check-release-deployment", Audience: "repository", Accessible: true, Result: "reviewed, verified, released, deployed"}
	var proof assuranceevidence.Package
	workflowValue(t, server.URL, http.MethodPost, evidenceBase+"/packages", controlOwner, assuranceevidence.CollectInput{ControlVersion: 1, ControlID: "retention", PeriodStart: periodStart, PeriodEnd: periodEnd, Records: []assuranceevidence.Record{record}}, http.StatusCreated, &proof)

	assessmentInput := independentassessments.OpenInput{Title: "Payments v2.4 independent assessment", Purpose: "challenge release-specific retention assurance", Scope: independentassessments.Scope{ProgramID: program.ID, ProgramVersion: 1, ControlIDs: []string{"retention"}, Systems: []string{"payments"}, Releases: []string{"payments-v2.4.0@" + string(release)}, PeriodStart: periodStart, PeriodEnd: periodEnd, EvidencePackageIDs: []string{proof.ID, "denied-package"}}, StartsAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	var independentAssessment independentassessments.Assessment
	workflowValue(t, server.URL, http.MethodPost, root+"/independent-assessments", owner, assessmentInput, http.StatusCreated, &independentAssessment)
	var invitation struct {
		Assessment independentassessments.Assessment `json:"assessment"`
		Credential independentassessments.Credential `json:"credential"`
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/independent-assessments/"+independentAssessment.ID+"/invitations", owner, independentassessments.InvitationInput{AssessorID: "auditor", AssessorName: "Independent Auditor", Organization: "Audit Cooperative", Kind: "external", ConflictDisclosure: "no conflict", ExpiresAt: now.Add(30 * time.Minute)}, http.StatusCreated, &invitation)
	var context independentassessments.Context
	workflowValue(t, server.URL, http.MethodGet, "/independent-assessor/context", invitation.Credential.Token, nil, http.StatusOK, &context)
	if len(context.Evidence) != 1 || len(context.UnavailableEvidenceIDs) != 1 || context.UnavailableEvidenceIDs[0] != "denied-package" {
		t.Fatalf("assessor evidence projection hid denied access: %#v", context)
	}
	var challenged independentassessments.Assessment
	workflowValue(t, server.URL, http.MethodPost, "/independent-assessor/events", invitation.Credential.Token, independentassessments.EventInput{Kind: "finding", Subject: "retention implementation gap", Body: "base revision retained records for seven days", ControlID: "retention", EvidencePackageIDs: []string{proof.ID}}, http.StatusCreated, &challenged)
	findingID := challenged.Events[len(challenged.Events)-1].ID
	workflowValue(t, server.URL, http.MethodPost, root+"/independent-assessments/"+challenged.ID+"/events", owner, independentassessments.EventInput{Kind: "finding_response", Subject: "finding contested pending delivery", Body: "the repaired Git revision must pass ordinary delivery before acceptance", ControlID: "retention", EvidencePackageIDs: []string{proof.ID}, ParentID: findingID}, http.StatusCreated, &challenged)
	workflowValue(t, server.URL, http.MethodPost, "/independent-assessor/events", invitation.Credential.Token, independentassessments.EventInput{Kind: "disagreement", Subject: "finding remains open", Body: "deployment evidence must name the exact released revision", ControlID: "retention", EvidencePackageIDs: []string{proof.ID}, ParentID: findingID}, http.StatusCreated, &challenged)

	var remediation assurancedelivery.Remediation
	workflowValue(t, server.URL, http.MethodPost, root+"/independent-assessments/"+challenged.ID+"/findings/"+findingID+"/remediations", owner, assurancedelivery.RemediationInput{AffectedRevision: string(release), Deadline: now.Add(7 * 24 * time.Hour), EvidencePackageIDs: []string{proof.ID}, Work: []assurancedelivery.WorkInput{{Kind: "pull_request", Title: "Agent-authored retention repair", OwnerKind: "agent", OwnerID: "agent", ResourceID: "pull:retention-repair@" + string(repair), AcceptanceCriteria: []string{"reviewed"}}, {Kind: "operational_work", Title: "Release and deploy retention control", OwnerKind: "human", OwnerID: "developer", ResourceID: "deployment:payments-v2.4.0@" + string(release), AcceptanceCriteria: []string{"verified", "released", "deployed"}}}}, http.StatusCreated, &remediation)
	workflowValue(t, server.URL, http.MethodPost, root+"/assurance-remediations/"+remediation.ID+"/work/"+remediation.Work[0].ID+"/progress", agent, assurancedelivery.ProgressInput{Status: "completed", Summary: "ordinary pull review approved the agent-authored repair", ResourceID: "pull:retention-repair", Revision: string(repair), EvidencePackageIDs: []string{proof.ID}}, http.StatusCreated, &remediation)
	workflowValue(t, server.URL, http.MethodPost, root+"/assurance-remediations/"+remediation.ID+"/work/"+remediation.Work[1].ID+"/progress", developer, assurancedelivery.ProgressInput{Status: "completed", Summary: "checks passed; v2.4.0 released and deployed", ResourceID: "deployment:payments-v2.4.0", Revision: string(release), EvidencePackageIDs: []string{proof.ID}}, http.StatusCreated, &remediation)
	workflowValue(t, server.URL, http.MethodPost, root+"/assurance-remediations/"+remediation.ID+"/verifications", owner, assurancedelivery.VerificationInput{AffectedRevision: string(release), EvidenceDigest: proof.PackageHash, EvidencePackageIDs: []string{proof.ID}, Criteria: map[string]bool{"reviewed": true, "verified": true, "released": true, "deployed": true}, Summary: "exact released revision satisfies retention control"}, http.StatusCreated, &remediation)
	workflowValue(t, server.URL, http.MethodPost, "/independent-assessor/remediations/"+remediation.ID+"/dispositions", invitation.Credential.Token, map[string]string{"decision": "accept", "rationale": "independent evidence verifies the delivered correction"}, http.StatusCreated, &remediation)
	if remediation.Status != "closed" || remediation.AuthorityGranted {
		t.Fatalf("accepted finding did not close without granting authority: %#v", remediation)
	}

	var statement assurancedelivery.Statement
	statementInput := assurancedelivery.StatementInput{ProgramID: program.ID, ProgramVersion: 1, ReleaseID: "payments-v2.4.0", ReleaseRevision: string(release), Scope: "payments production", PeriodStart: periodStart, PeriodEnd: periodEnd, ExpiresAt: now.Add(30 * 24 * time.Hour), ControlIDs: []string{"retention"}, ExceptionReferences: []string{}, EvidencePackageIDs: []string{proof.ID}, RemediationIDs: []string{remediation.ID}, Audience: "public", EvidenceDigest: proof.PackageHash}
	workflowValue(t, server.URL, http.MethodPost, root+"/assurance-statements", owner, statementInput, http.StatusCreated, &statement)
	if statement.Status != "current" || statement.Signature == "" || statement.ReleaseRevision != string(release) {
		t.Fatalf("release-specific statement is not independently verifiable: %#v", statement)
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/assurance-remediations/"+remediation.ID+"/drift", owner, map[string]string{"revision": string(commit(release, "retention telemetry disabled\n", "post-release drift", "developer")), "reason": "deployment drift disabled retention telemetry"}, http.StatusCreated, &remediation)
	workflowValue(t, server.URL, http.MethodGet, root+"/assurance-statements/"+statement.ID, "", nil, http.StatusOK, &statement)
	if statement.Status != "changed" || statement.SignedPayload == "" {
		t.Fatalf("post-publication drift did not preserve and invalidate the claim: %#v", statement)
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/assurance-statements/"+statement.ID+"/revocation", owner, map[string]string{"reason": "release assurance changed after deployment drift"}, http.StatusCreated, &statement)
	if statement.Status != "revoked" {
		t.Fatalf("changed assurance statement was not revoked: %#v", statement)
	}
	workflowValue(t, server.URL, http.MethodDelete, root+"/independent-assessments/"+challenged.ID+"/invitations/"+invitation.Assessment.Invitations[0].ID, owner, nil, http.StatusOK, &challenged)
	workflowValue(t, server.URL, http.MethodGet, "/independent-assessor/context", invitation.Credential.Token, nil, http.StatusUnauthorized, nil)
}

func digest64(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
