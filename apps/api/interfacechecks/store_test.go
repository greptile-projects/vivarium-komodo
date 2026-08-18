package interfacechecks

import "testing"

func sampleRun() Run {
	return Run{
		RepositoryID: "repo", PullRequestID: "pull", Revision: "candidate",
		SpecificationKind: "implementation_contract", SpecificationID: "contract", SpecificationVersion: 2,
		ConfigPath: ".komodo/interface-checks.json", ConfigBlobID: "config-a", CreatedByID: "runner",
		Cases: []Case{{
			Name: "checkout-mobile-ar", Journey: "checkout", Surface: "/checkout",
			Context:        Context{Viewport: "390x844", Theme: "dark", ContentLength: "expanded", Locale: "ar", InteractionState: "validation-error", AssistiveTechnology: "screen-reader"},
			RequirementIDs: []string{"confirm-order"}, Inputs: []Input{{Path: "ui/checkout.tsx", BlobID: "blob-a"}},
			Status: "passed", Summary: "journey completes", Coverage: []string{"visual", "behavioral", "keyboard"},
			Artifacts:   []Artifact{{Kind: "recording", Name: "journey.webm", Digest: "1234567890abcdef", MediaType: "video/webm", Size: 42}},
			Differences: []Difference{{ID: "diff-1", Kind: "visual", Summary: "spacing differs", RequirementIDs: []string{"confirm-order"}}},
		}},
	}
}

func TestScopedStalenessClassificationAndApproval(t *testing.T) {
	s, _ := New(t.TempDir())
	v, e := s.Create(sampleRun())
	if e != nil {
		t.Fatal(e)
	}
	if v.Passed {
		t.Fatal("unclassified difference passed")
	}
	v, e = s.Classify("repo", "pull", v.ID, "checkout-mobile-ar", "diff-1", "designer", "intentional", "approved token update")
	if e != nil {
		t.Fatal(e)
	}
	v, e = s.Approve("repo", "pull", v.ID, "checkout-mobile-ar", "owner", "approved", "matches accepted behavior", []string{"diff-1"})
	if e != nil || !v.Passed {
		t.Fatalf("approved evidence not passing: %#v %v", v, e)
	}
	DeriveCurrent(&v, "candidate", "config-a", map[string]string{"ui/checkout.tsx": "blob-b"})
	if v.Cases[0].Current || v.Approvals[0].Current || len(v.AffectedRequirements) != 1 || v.Passed {
		t.Fatalf("affected evidence remained current: %#v", v)
	}
}

func TestUnrelatedBlobDoesNotInvalidate(t *testing.T) {
	s, _ := New(t.TempDir())
	v, e := s.Create(sampleRun())
	if e != nil {
		t.Fatal(e)
	}
	DeriveCurrent(&v, "candidate", "config-a", map[string]string{"ui/checkout.tsx": "blob-a", "README.md": "new"})
	if !v.Cases[0].Current {
		t.Fatal("unrelated input invalidated case")
	}
	DeriveCurrent(&v, "new-candidate", "config-a", map[string]string{"ui/checkout.tsx": "blob-a"})
	if !v.Cases[0].Current {
		t.Fatal("unrelated pull revision invalidated reusable evidence")
	}
	DeriveCurrent(&v, "candidate", "config-b", map[string]string{"ui/checkout.tsx": "blob-a"})
	if v.Current || v.Cases[0].Current {
		t.Fatal("changed repository definition remained current")
	}
}
