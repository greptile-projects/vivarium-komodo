package responsealerts

import (
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/responsepolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responserotations"
)

func TestCorrelationRoutingAndDeliveryFailureRemainDistinct(t *testing.T) {
	now := time.Now().UTC()
	p := responsepolicies.Policy{ID: "policy", CurrentVersion: 2, Versions: []responsepolicies.Version{{Number: 2, Input: responsepolicies.Input{Coverage: []responsepolicies.Coverage{{ID: "api-critical", ResourceKind: "service", ResourceID: "api", SignalClass: "reliability", Severity: "critical", TeamID: "operators", Target: responsepolicies.Target{AcknowledgeMinutes: 5}}}}}}}
	r := responserotations.Rotation{ID: "rotation", Revision: 3, Input: responserotations.Input{PolicyID: "policy", PolicyVersion: 2, TeamID: "operators"}, CurrentShift: &responserotations.ShiftView{ResponderID: "alice"}}
	s, _ := New(t.TempDir())
	in := Input{Signal: Signal{SignalClass: "reliability", Severity: "critical", ResourceKind: "service", ResourceID: "api", Revision: "deploy-7", ObservedAt: now, CorrelationKey: "api:errors:us-east", Summary: "error budget burn", AffectedResources: []string{"service:api", "region:us-east"}, AffectedUserCount: 240, Evidence: []Evidence{{Kind: "service_level", Reference: "slo:api", Revision: "window-9", Accessible: true}}, Uncertainty: "sample excludes one region"}, RateLimitPerHour: 2}
	a, err := s.Create("repo", "monitor", in, p, []responserotations.Rotation{r})
	if err != nil || a.Status != "delivering" || a.RoutingAttempts[0].RecipientID != "alice" || a.PolicyVersion != 2 || a.ResponseDeadline == nil {
		t.Fatalf("policy/rotation routing missing: %+v %v", a, err)
	}
	dup, _ := s.Create("repo", "monitor", in, p, []responserotations.Rotation{r})
	if dup.ID != a.ID || dup.DuplicateCount != 1 || len(dup.RoutingAttempts) != 1 {
		t.Fatalf("correlated signal produced duplicate attention: %+v", dup)
	}
	failed, _ := s.RecordAttempt("repo", a.ID, "delivery-worker", AttemptInput{ExpectedRevision: dup.Revision, RecipientID: "alice", Channel: "web_push", Status: "failed", Reason: "endpoint unavailable", PolicyVersion: 2})
	if failed.Status != "delivery_failed" || len(failed.Gaps) == 0 {
		t.Fatalf("delivery failure was hidden: %+v", failed)
	}
	if failed.Status == "acknowledged" {
		t.Fatal("delivery state became response acknowledgement")
	}
}

func TestSuppressionMaintenanceStalenessAndPolicyChangeAreAudited(t *testing.T) {
	now := time.Now().UTC()
	p := responsepolicies.Policy{ID: "p", CurrentVersion: 4, Versions: []responsepolicies.Version{{Number: 4, Input: responsepolicies.Input{Coverage: []responsepolicies.Coverage{{ID: "security", ResourceKind: "repository", ResourceID: "repo", SignalClass: "security", Severity: "high", TeamID: "security", Target: responsepolicies.Target{AcknowledgeMinutes: 10}}}}}}}
	s, _ := New(t.TempDir())
	base := Signal{SignalClass: "security", Severity: "high", ResourceKind: "repository", ResourceID: "repo", Revision: "scan-1", ObservedAt: now, CorrelationKey: "known-scan", Summary: "scanner finding", Evidence: []Evidence{{Kind: "scan", Reference: "private:scan", Revision: "1", Accessible: false}}}
	a, _ := s.Create("repo", "scanner", Input{Signal: base, SuppressionKeys: []string{"known-scan"}}, p, nil)
	if a.Status != "suppressed" || len(a.Events) != 1 || len(a.Gaps) != 1 {
		t.Fatalf("suppression/evidence not visible: %+v", a)
	}
	base.CorrelationKey = "maintenance"
	base.Revision = "scan-2"
	b, _ := s.Create("repo", "scanner", Input{Signal: base, MaintenanceWindows: []Window{{StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), Reason: "declared migration"}}}, p, nil)
	if b.Status != "maintenance" {
		t.Fatalf("maintenance not deterministic: %+v", b)
	}
	base.CorrelationKey = "old"
	base.Revision = "scan-3"
	base.ObservedAt = now.Add(-25 * time.Hour)
	c, _ := s.Create("repo", "scanner", Input{Signal: base}, p, nil)
	if c.Status != "stale" {
		t.Fatalf("stale signal routed: %+v", c)
	}
}

func TestAssignedResponderWorkspaceIsSteerableAndAgentIsReadOnly(t *testing.T) {
	now := time.Now().UTC()
	p := responsepolicies.Policy{ID: "p", CurrentVersion: 1, Versions: []responsepolicies.Version{{Number: 1, Input: responsepolicies.Input{Coverage: []responsepolicies.Coverage{{ID: "c", ResourceKind: "service", ResourceID: "api", SignalClass: "reliability", Severity: "critical", TeamID: "ops", Target: responsepolicies.Target{AcknowledgeMinutes: 5}}}}}}}
	r := responserotations.Rotation{ID: "r", Revision: 2, Input: responserotations.Input{PolicyID: "p", PolicyVersion: 1, TeamID: "ops"}, CurrentShift: &responserotations.ShiftView{ResponderID: "alice"}}
	s, _ := New(t.TempDir())
	a, _ := s.Create("repo", "monitor", Input{Signal: Signal{SignalClass: "reliability", Severity: "critical", ResourceKind: "service", ResourceID: "api", Revision: "release-7", ObservedAt: now, CorrelationKey: "api:error", Summary: "errors", Evidence: []Evidence{{Kind: "deployment", Reference: "deploy-7", Revision: "event-9", Accessible: true}}}}, p, []responserotations.Rotation{r})
	if _, err := s.OpenWorkspace("repo", a.ID, "mallory", WorkspaceInput{ExpectedRevision: a.Revision}); err != ErrInvalid {
		t.Fatalf("unassigned responder opened workspace: %v", err)
	}
	a, err := s.OpenWorkspace("repo", a.ID, "alice", WorkspaceInput{ExpectedRevision: a.Revision, Context: []ContextReference{{Kind: "release", ResourceID: "release-7", Revision: "commit-7", Permitted: true, Audience: "participants"}, {Kind: "runbook", ResourceID: "rb-api", Revision: "v3", Permitted: true, Audience: "participants"}}})
	if err != nil || a.Status != "acknowledged" || a.Workspace.AssignedResponderID != "alice" {
		t.Fatalf("workspace not acknowledged: %+v %v", a, err)
	}
	a, _ = s.Act("repo", a.ID, "alice", WorkspaceActionInput{ExpectedRevision: a.Revision, Kind: "classify", Classification: "availability", Detail: "confirmed user-facing availability loss"})
	a, _ = s.Act("repo", a.ID, "alice", WorkspaceActionInput{ExpectedRevision: a.Revision, Kind: "invite", AssigneeID: "owner", Detail: "invite service owner"})
	a, err = s.RunDiagnostic("repo", a.ID, "owner", DiagnosticInput{ExpectedRevision: a.Revision, Name: "read replica lag", CommandReference: "rb-api#replica-lag", ContextReferences: []string{"rb-api"}, ApprovedByID: "alice", SanitizedOutput: "lag=42s"})
	if err != nil || len(a.Workspace.Diagnostics) != 1 {
		t.Fatalf("approved diagnostic missing: %+v %v", a, err)
	}
	started, token, err := s.StartAgent("repo", a.ID, "alice", AgentInput{ExpectedRevision: a.Revision, Agent: "triage-agent@v2", Mandate: "compare release and runbook", ContextReferences: []string{"release-7", "rb-api"}})
	if err != nil || token == "" || started.Workspace.AgentInvestigations[0].CredentialDigest != "" {
		t.Fatalf("bounded credential missing or leaked: %+v %v", started, err)
	}
	if _, _, err = s.AddAgentRecord(token, "finding", "release changed retry behavior", []string{"release-7"}); err != nil {
		t.Fatalf("agent could not publish cited finding: %v", err)
	}
	if _, _, err = s.AddAgentRecord(token, "action", "restart production", []string{"release-7"}); err != ErrInvalid {
		t.Fatalf("agent gained action authority: %v", err)
	}
	latest, _ := s.Get("repo", a.ID)
	if latest.Workspace.Classification != "availability" || len(latest.Workspace.Participants) != 2 || len(latest.Workspace.AgentInvestigations[0].Records) != 1 {
		t.Fatalf("collaboration was not retained: %+v", latest.Workspace)
	}
}
