package impactassessments

import "testing"

func TestAssessmentDecisionsAcknowledgementsAndAgentEvidence(t *testing.T) {
	root := t.TempDir()
	store, _ := New(root)
	item, err := store.Create(Assessment{RepositoryID: "provider", Title: "Change parser", Revision: "main", CommitID: "abc", CreatorID: "alice", Sources: []Source{{Kind: "symbol", Symbol: "Parse"}}, Impacts: []Impact{{Category: "consumer", Summary: "SDK consumes parser", RepositoryID: "sdk", OwnerIDs: []string{"bob"}, Evidence: []Evidence{{RepositoryID: "sdk", CommitID: "def", Kind: "dependency", Label: "api"}}}}})
	if err != nil || len(item.Impacts) != 1 || item.Impacts[0].State != "open" {
		t.Fatalf("create = %#v, %v", item, err)
	}
	item, err = store.Update("provider", item.ID, "alice", item.Impacts[0].ID, "accepted_risk", "compatible fallback remains")
	if err != nil || item.Impacts[0].Rationale == "" {
		t.Fatalf("risk = %#v, %v", item, err)
	}
	item, err = store.Request("provider", item.ID, "alice", item.Impacts[0].ID, "bob")
	if err != nil || len(item.Participants) != 2 {
		t.Fatalf("request = %#v, %v", item, err)
	}
	item, err = store.Decide("provider", item.ID, "bob", item.Impacts[0].ID, "concern", "verify the SDK suite")
	if err != nil || item.Impacts[0].Acknowledgements[0].State != "concern" {
		t.Fatalf("decision = %#v, %v", item, err)
	}
	_, token, err := store.StartAgent("provider", item.ID, "alice")
	if err != nil || token == "" {
		t.Fatal("missing bounded agent credential")
	}
	item, err = store.AddFinding(token, "The SDK dependency needs independent verification.", "Generated clients were not visible.", []Evidence{{RepositoryID: "sdk", CommitID: "def", Kind: "dependency", Label: "api"}})
	if err != nil || len(item.Findings) != 1 || item.Findings[0].Agent != "codex" {
		t.Fatalf("finding = %#v, %v", item, err)
	}
	restored, _ := New(root)
	persisted, err := restored.Get("provider", item.ID)
	if err != nil || persisted.Impacts[0].Acknowledgements[0].Note == "" || len(persisted.Findings) != 1 {
		t.Fatalf("persisted = %#v, %v", persisted, err)
	}
	if _, err = restored.AgentContext(token); err != nil {
		t.Fatal("bounded worker credential did not survive restart")
	}
}
