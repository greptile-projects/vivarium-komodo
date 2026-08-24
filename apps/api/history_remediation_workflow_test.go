package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/historyremediations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestExposureToContainedRepositoryHistory is the black-box boundary for a
// distributed project recovering from a committed credential. It uses the same
// public HTTP surface as view=security and stock Git, while the retained record
// coordinates independently controlled copies without acquiring their authority.
func TestExposureToContainedRepositoryHistory(t *testing.T) {
	requireGit(t)
	gitStore, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	history, _ := historyremediations.New(t.TempDir())
	repository, _ := repos.Create("maintainer", repositories.Metadata{Name: "distributed-service", Visibility: repositories.Private})
	for _, actor := range []string{"agent", "fork-owner"} {
		if _, err := repos.AddCollaborator("maintainer", repository.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	tokens := map[string]string{}
	for _, actor := range []string{"maintainer", "agent", "fork-owner"} {
		tokens[actor] = issueAccess(t, credentials, actor, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	}
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)

	mux := http.NewServeMux()
	registerGitHTTP(mux, repos, credentials, history)
	registerHistoryRemediationsHTTP(mux, history, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	remoteURL, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
	remoteURL.User = url.UserPassword("git", maintainerGit)
	remote := remoteURL.String()

	work := gitClone(t, remote)
	gitOutput(t, work, "config", "user.name", "Maintainer")
	gitOutput(t, work, "config", "user.email", "maintainer@example.test")
	if err := os.WriteFile(filepath.Join(work, "service.txt"), []byte("service=v1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, work, "add", "service.txt")
	gitOutput(t, work, "commit", "-m", "Establish service")
	base := gitOutput(t, work, "rev-parse", "HEAD")
	// The test never records this value in remediation input, logs, or evidence.
	if err := os.WriteFile(filepath.Join(work, "credential.txt"), []byte("synthetic-sensitive-value\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, work, "add", "credential.txt")
	gitOutput(t, work, "commit", "-m", "Configure deployment (signed upstream)")
	exposed := gitOutput(t, work, "rev-parse", "HEAD")
	objectID := gitOutput(t, work, "rev-parse", "HEAD:credential.txt")
	gitOutput(t, work, "push", "-u", "origin", "main")
	gitOutput(t, work, "push", "origin", "main:refs/heads/release/1.x")
	staleClone := gitClone(t, remote)
	gitOutput(t, staleClone, "config", "user.name", "Stale Collaborator")
	gitOutput(t, staleClone, "config", "user.email", "stale@example.test")

	// Build a payload-free replacement lineage with stock Git. Its tree contains
	// ordinary content only and deliberately breaks the old signed commit ID.
	gitOutput(t, work, "switch", "--detach", base)
	if err := os.WriteFile(filepath.Join(work, "MIGRATION.md"), []byte("Fetch replacement refs before contributing.\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, work, "add", "MIGRATION.md")
	gitOutput(t, work, "commit", "-m", "Publish sanitized history guidance")
	replacement := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "origin", replacement+":refs/heads/remediation-candidate")

	root := "/repositories/" + string(repository.ID) + "/history-remediations"
	request := func(method, path, actor string, value any, want int, out any) {
		t.Helper()
		body := ""
		if value != nil {
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			body = string(encoded)
		}
		workflowJSON(t, server.URL, method, path, tokens[actor], body, want, out)
	}

	// A scanner's false match remains attributable in a separate restricted
	// record instead of expanding the confirmed rewrite scope.
	falseInput := historyremediations.Input{Title: "Dismiss generated fixture match", Source: historyremediations.Source{Kind: "security_finding", ID: "finding:false-match"}, ContentDescription: "Digest-only scanner match; payload omitted.", Reason: "Retain the correction without rewriting safe history.", Audience: "owners_only", ResponseOwnerIDs: []string{"maintainer"}, Objects: []historyremediations.Object{{ID: "safe-fixture", RepositoryID: string(repository.ID), Kind: "blob", ObjectID: strings.Repeat("a", 40), Match: "false_match", Reason: "Maintainer verified the synthetic fixture is unrelated.", AttributedTo: "maintainer"}}, Scope: []historyremediations.Scope{{Kind: "repository", Reference: string(repository.ID)}}, Evidence: []historyremediations.Evidence{{ID: "scan:false", Kind: "scanner_digest", Reference: "scan:42", Digest: "sha256:false", Summary: "Payload-free comparison disproved the match.", Status: "available", RecordedBy: "agent"}}, Approvals: []historyremediations.Approval{{Kind: "repository_owner", OwnerID: "maintainer", Required: true, Status: "approved"}}}
	var falseRecord historyremediations.Remediation
	request(http.MethodPost, root, "maintainer", falseInput, http.StatusCreated, &falseRecord)
	if len(falseRecord.Blockers) != 1 || falseRecord.Blockers[0].Kind != "false_match" {
		t.Fatalf("false match correction lost: %#v", falseRecord.Blockers)
	}

	in := historyremediations.Input{Title: "Remove committed deployment credential", Source: historyremediations.Source{Kind: "security_finding", ID: "finding:credential"}, ContentDescription: "Confirmed credential-bearing blob; bytes intentionally omitted.", Reason: "Rotate the credential and remove its reachable lineage.", Audience: "named_participants", ResponseOwnerIDs: []string{"maintainer"}, ParticipantIDs: []string{"agent"}, Objects: []historyremediations.Object{{ID: "credential-blob", RepositoryID: string(repository.ID), Kind: "blob", ObjectID: objectID, Digest: "sha256:payload-withheld", Match: "confirmed", AttributedTo: "scanner"}}, Scope: []historyremediations.Scope{{Kind: "repository", Reference: string(repository.ID)}, {Kind: "ref", Reference: "refs/heads/main", Revision: exposed}, {Kind: "ref", Reference: "refs/heads/release/1.x", Revision: exposed}, {Kind: "package", Reference: "package:service@1.0"}, {Kind: "artifact", Reference: "artifact:release-1.0"}, {Kind: "environment", Reference: "deployment:production"}}, Evidence: []historyremediations.Evidence{{ID: "scan:confirmed", Kind: "object_digest", Reference: "scanner:restricted-result", Revision: exposed, Digest: "sha256:scan-result", Summary: "Exact object ID is reachable; no payload was copied.", Status: "available", RecordedBy: "scanner"}}, Approvals: []historyremediations.Approval{{Kind: "repository_owner", OwnerID: "maintainer", Required: true, Status: "approved"}, {Kind: "security_owner", OwnerID: "maintainer", Required: true, Status: "approved"}}}
	var remediation historyremediations.Remediation
	request(http.MethodPost, root, "maintainer", in, http.StatusCreated, &remediation)
	path := root + "/" + remediation.ID

	for _, finding := range []historyremediations.ReachabilityInput{
		{CopyKind: "branch", Reference: "refs/heads/main", Revision: exposed, ObjectIDs: []string{objectID}, DerivedExposures: []historyremediations.DerivedExposure{{Kind: "credential", Reference: "credential:deployment", State: "active"}}, Status: "confirmed", Summary: "Agent found the object ID from ref reachability without reading it.", Citations: []historyremediations.Citation{{Kind: "git-rev-list", Reference: "analysis:main", Revision: exposed, Digest: "sha256:main", Access: "restricted"}}},
		{CopyKind: "pull_request", Reference: "pull:17", Revision: exposed, ObjectIDs: []string{objectID}, Status: "confirmed", Summary: "Open pull lineage reaches the object.", Citations: []historyremediations.Citation{{Kind: "pull-revision", Reference: "pull:17", Revision: exposed, Digest: "sha256:pull", Access: "restricted"}}},
		{CopyKind: "fork", Reference: "fork:independent/service", Revision: exposed, ObjectIDs: []string{objectID}, Status: "independently_controlled", ControlledBy: "fork-owner", Summary: "Fork owner controls its advertised copy.", Citations: []historyremediations.Citation{{Kind: "federated-ref", Reference: "peer:fork", Revision: exposed, Digest: "sha256:fork", Access: "restricted"}}},
		{CopyKind: "federated_contribution", Reference: "peer:unavailable/pull/9", Revision: exposed, ObjectIDs: []string{objectID}, Status: "unverifiable", Summary: "Federated peer is unavailable.", Uncertainty: "Non-advertised refs cannot be checked while the peer is offline.", Citations: []historyremediations.Citation{{Kind: "peer-observation", Reference: "peer:unavailable", Digest: "sha256:offline", Access: "inaccessible"}}},
		{CopyKind: "package", Reference: "package:service@1.0", Revision: "1.0", ObjectIDs: []string{objectID}, Status: "confirmed", Summary: "Published source package retains the object.", Citations: []historyremediations.Citation{{Kind: "package-manifest", Reference: "package:service@1.0", Digest: "sha256:package", Access: "restricted"}}},
		{CopyKind: "release_artifact", Reference: "artifact:release-1.0", Revision: "1.0", ObjectIDs: []string{objectID}, Status: "confirmed", Summary: "Release source artifact retains the object.", Citations: []historyremediations.Citation{{Kind: "artifact-index", Reference: "artifact:release-1.0", Digest: "sha256:artifact", Access: "restricted"}}},
		{CopyKind: "deployment", Reference: "deployment:production", Revision: exposed, ObjectIDs: []string{objectID}, DerivedExposures: []historyremediations.DerivedExposure{{Kind: "credential", Reference: "credential:deployment", State: "active"}}, Status: "confirmed", Summary: "Deployment used the affected revision.", Citations: []historyremediations.Citation{{Kind: "deployment-receipt", Reference: "deployment:production", Revision: exposed, Digest: "sha256:deployment", Access: "restricted"}}},
	} {
		request(http.MethodPost, path+"/reachability", "agent", finding, http.StatusCreated, &remediation)
	}
	if remediation.ReachabilitySummary.DerivedExposureCount != 2 {
		t.Fatalf("agent analysis lost derived exposure: %#v", remediation.ReachabilitySummary)
	}

	rule := historyremediations.RewriteRuleInput{Kind: "remove_object", ObjectIDs: []string{objectID}, PreserveAuthorship: true, PreserveTimestamps: true, SignaturePolicy: "resign", Rationale: "Remove the named object and re-sign the affected release lineage."}
	request(http.MethodPost, path+"/rewrite-rules", "maintainer", rule, http.StatusCreated, &remediation)
	candidate := historyremediations.RewriteCandidateInput{RuleIDs: []string{remediation.RewriteRules[0].ID}, Refs: []historyremediations.RefReplacement{{Reference: "refs/heads/main", OldRevision: exposed, NewRevision: replacement}, {Reference: "refs/heads/release/1.x", OldRevision: exposed, NewRevision: replacement}}, CommitMap: []historyremediations.CommitMapping{{OldCommit: exposed, NewCommit: replacement, AuthorshipPreserved: true, SignatureStatus: "resigned"}}, UnaffectedDigest: "sha256:unaffected-trees", CandidateDigest: "sha256:sanitized-candidate", ChangedObjectIDs: []string{objectID, exposed, replacement}, StorageBeforeBytes: 2048, StorageAfterBytes: 1024, RollbackUntil: time.Now().Add(24 * time.Hour), RollbackLimits: []string{"Independent clones and protected backups remain owner controlled."}, CollaboratorActions: []string{"Fetch, verify the attestation, and rebase unpublished work."}, LinkImpacts: []historyremediations.LinkImpact{{Kind: "signed_commit", Reference: exposed, Status: "broken", Action: "Use the attested re-signed replacement."}}, UnrewritableResources: []string{"fork:independent/service", "peer:unavailable", "backup:protected"}}
	request(http.MethodPost, path+"/rewrite-candidates", "maintainer", candidate, http.StatusCreated, &remediation)

	checks := func(failed bool) []historyremediations.RehearsalCheck {
		out := []historyremediations.RehearsalCheck{}
		for _, domain := range []string{"integrity", "build", "check", "release", "dependency", "clone", "fetch"} {
			status := "passed"
			if failed && domain == "fetch" {
				status = "failed"
			}
			out = append(out, historyremediations.RehearsalCheck{Domain: domain, Status: status, Reference: "rehearsal:" + domain, Digest: "sha256:" + domain, Summary: "Payload-free bounded " + domain + " result."})
		}
		return out
	}
	rehearsal := historyremediations.RehearsalInput{CandidateID: remediation.Candidates[0].ID, Environment: "networkless-synthetic", BudgetMinutes: 20, BudgetCost: 10, Checks: checks(true), ObservedMinutes: 4, ObservedCost: 2}
	request(http.MethodPost, path+"/rewrite-rehearsals", "maintainer", rehearsal, http.StatusCreated, &remediation)
	failedRehearsal := remediation.Rehearsals[0]
	rehearsal.Checks = checks(false)
	request(http.MethodPost, path+"/rewrite-rehearsals", "maintainer", rehearsal, http.StatusCreated, &remediation)
	if failedRehearsal.Status != "failed" || remediation.Rehearsals[1].Status != "passed" {
		t.Fatalf("rehearsal recovery lost: %#v", remediation.Rehearsals)
	}

	pauses := []historyremediations.Pause{}
	for _, kind := range []string{"push", "queue", "session", "workflow", "release"} {
		pauses = append(pauses, historyremediations.Pause{Kind: kind, Reference: kind + ":affected", Status: "paused", Guidance: "Fetch replacement refs and rebase; the old lineage is quarantined."})
	}
	publication := historyremediations.PublicationInput{CandidateID: remediation.Candidates[0].ID, ExpectedUpdatedAt: remediation.UpdatedAt, Attestation: historyremediations.Attestation{Digest: "sha256:sanitized-candidate", SignerID: "independent-reviewer", Signature: "ed25519:attestation"}, QuarantinedObjectIDs: []string{objectID, exposed}, CredentialActions: []historyremediations.CredentialAction{{Reference: "credential:deployment", Action: "rotate", Receipt: "rotation:receipt-42"}}, Pauses: pauses, MigrationTargets: []historyremediations.MigrationTarget{{ID: "fork", Kind: "fork", Reference: "fork:independent/service", OwnerID: "fork-owner", Audience: "owner", Authority: "independent_owner", Instructions: "Rewrite locally and report only a payload-free receipt.", Mapping: "redacted", Status: "pending"}, {ID: "offline-peer", Kind: "federated_copy", Reference: "peer:unavailable", OwnerID: "fork-owner", Audience: "owner", Authority: "independent_owner", Instructions: "Retry contact without treating outage as containment.", Mapping: "unavailable", Status: "pending"}, {ID: "pull", Kind: "open_pull_request", Reference: "pull:17", OwnerID: "maintainer", Audience: "participants", Authority: "coordinator", Instructions: "Migrate discussion to a replacement revision.", Mapping: "full", Status: "pending"}}}

	// A concurrent release-line movement makes the two-ref transaction fail; main
	// remains untouched, proving publication cannot leave a partial rewrite.
	repo, _ := repos.Open(repository.ID)
	if err := repo.UpdateReference(storage.Reference{Name: "refs/heads/release/1.x", ObjectID: storage.ObjectID(base)}); err != nil {
		t.Fatal(err)
	}
	request(http.MethodPost, path+"/publications", "maintainer", publication, http.StatusInternalServerError, nil)
	if got := gitOutput(t, repo.GitDir(), "rev-parse", "refs/heads/main"); got != exposed {
		t.Fatalf("partial publication moved main to %s", got)
	}
	if err := repo.UpdateReference(storage.Reference{Name: "refs/heads/release/1.x", ObjectID: storage.ObjectID(exposed)}); err != nil {
		t.Fatal(err)
	}
	request(http.MethodPost, path+"/publications", "maintainer", publication, http.StatusCreated, &remediation)
	if got := gitOutput(t, repo.GitDir(), "rev-parse", "refs/heads/main"); got != replacement {
		t.Fatalf("sanitized main=%s", got)
	}

	if err := os.WriteFile(filepath.Join(staleClone, "STALE.md"), []byte("old-lineage work\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, staleClone, "add", "STALE.md")
	gitOutput(t, staleClone, "commit", "-m", "Work from stale clone")
	command := exec.Command("git", "-C", staleClone, "push", "--force", "origin", "main")
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "history rewrite migration in progress") {
		t.Fatalf("stale push was not safely guided: err=%v output=%s", err, output)
	}

	request(http.MethodPost, path+"/migration-targets/fork", "fork-owner", historyremediations.MigrationDecisionInput{Status: "migrated", Receipt: "Fork owner independently published replacement refs."}, http.StatusOK, &remediation)
	request(http.MethodPost, path+"/collaboration-migrations", "maintainer", historyremediations.CollaborationMigrationInput{Kind: "pull_request", Reference: "pull:17", Action: "migrate", ReplacementRevision: replacement, DiscussionReference: "discussion:pull:17", Attribution: []string{"author", "reviewer"}, Receipt: "Discussion and authorship retained on replacement pull."}, http.StatusCreated, &remediation)

	domains := []string{"repository_reachability", "object_access", "fork_federation", "package_artifact", "credential_rotation", "deployment", "cache", "protected_recovery_copy"}
	round := func(residual bool) historyremediations.ContainmentRoundInput {
		cs := []historyremediations.ContainmentCheck{}
		for _, domain := range domains {
			status := "passed"
			reference := domain + ":verified"
			if residual && domain == "object_access" {
				status, reference = "reintroduced", "object:reintroduced-by-stale-cache"
			}
			if residual && domain == "protected_recovery_copy" {
				status, reference = "legal_hold", "backup:protected"
			}
			if residual && domain == "fork_federation" {
				status, reference = "unreachable", "peer:unavailable"
			}
			cs = append(cs, historyremediations.ContainmentCheck{ID: domain, Domain: domain, Reference: reference, Revision: replacement, Status: status, Digest: "sha256:" + domain, Summary: "Current payload-free containment result.", OwnerID: "maintainer", ExpiresAt: time.Now().Add(time.Hour)})
		}
		return historyremediations.ContainmentRoundInput{Policy: historyremediations.CompletionPolicy{RequiredDomains: domains, MaximumAgeHours: 24}, Checks: cs}
	}
	request(http.MethodPost, path+"/containment-rounds", "maintainer", round(true), http.StatusCreated, &remediation)
	if remediation.ContainmentRounds[0].Status != "residuals" {
		t.Fatal("reintroduced object, protected backup, and unavailable peer were hidden")
	}
	request(http.MethodPost, path+"/containment-rounds", "maintainer", round(false), http.StatusCreated, &remediation)
	passing := remediation.ContainmentRounds[1]
	request(http.MethodPost, path+"/recovery-decisions", "maintainer", historyremediations.RecoveryInput{Flow: "push", Reference: "push:affected", RoundID: passing.ID, CheckIDs: []string{"repository_reachability", "object_access", "fork_federation", "credential_rotation"}, Decision: "resume"}, http.StatusCreated, &remediation)

	// Ordinary contribution resumes only after current containment evidence. A
	// fresh clone can now push a normal fast-forward on the sanitized lineage.
	fresh := gitClone(t, remote)
	gitOutput(t, fresh, "config", "user.name", "Collaborator")
	gitOutput(t, fresh, "config", "user.email", "collaborator@example.test")
	if err := os.WriteFile(filepath.Join(fresh, "CONTRIBUTING.md"), []byte("Ordinary contributions restored.\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, fresh, "add", "CONTRIBUTING.md")
	gitOutput(t, fresh, "commit", "-m", "Restore ordinary contribution")
	gitOutput(t, fresh, "push", "origin", "main")

	request(http.MethodGet, path, "maintainer", nil, http.StatusOK, &remediation)
	if len(remediation.Publications) != 1 || remediation.Publications[0].CredentialActions[0].Action != "rotate" || len(remediation.RecoveryDecisions) != 1 || remediation.Publications[0].MigrationTargets[0].Status != "migrated" {
		t.Fatalf("restricted exposure-to-containment trail is incomplete: %#v", remediation)
	}
}
