package agentprofiles

import "testing"

func TestCompareVersionsRequiresConsentForMaterialChanges(t *testing.T) {
	p := Profile{Versions: []Version{{Number: 1, Input: Input{Ownership: "operator-a", Models: []Model{{Provider: "p", Name: "m", Version: "1"}}, RequestedCapabilities: []string{"read"}, Pricing: Pricing{Model: "usage", Amount: 1, Unit: "task"}}}, {Number: 2, Input: Input{Ownership: "operator-a", Models: []Model{{Provider: "p", Name: "m", Version: "2"}}, RequestedCapabilities: []string{"read", "write"}, Pricing: Pricing{Model: "usage", Amount: 2, Unit: "task"}}}}}
	c, e := CompareVersions(p, 1, 2)
	if e != nil || !c.RenewedConsent || len(c.MaterialChanges) != 3 {
		t.Fatalf("material drift was not explicit: %v %+v", e, c)
	}
}
