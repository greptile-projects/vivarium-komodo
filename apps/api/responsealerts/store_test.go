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
