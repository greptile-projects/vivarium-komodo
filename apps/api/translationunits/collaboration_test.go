package translationunits

import "testing"

func collaborativeInput() Input {
	return Input{Revision: "candidate", TargetRevision: "target", SourceLocale: "en", Locales: []string{"fr"}, ConfigBlobID: "config", PlanID: "plan", PlanVersion: 2, ProductContext: "Billing confirmation", Terminology: map[string][]Terminology{"fr": {{Concept: "workspace", Preferred: "espace de travail", Avoid: []string{"bureau"}}}}, ReviewerIDs: map[string][]string{"fr": {"regional-reviewer"}}, Protected: true, Embargoed: true, PermittedActorIDs: []string{"translator", "agent", "regional-reviewer"}, Units: []Unit{{ID: "unit", ResourceID: "app", Key: "confirm", Message: "Confirm workspace", Change: "changed", Translations: map[string]TranslationState{"fr": {Status: "untranslated"}}}}}
}

func TestCollaborativeTranslationRetainsHumanJudgmentAndConcurrency(t *testing.T) {
	s, _ := New(t.TempDir())
	x, e := s.Create("repo", "pull", "owner", collaborativeInput())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Authorized("repo", "pull", "reader"); e != ErrForbidden {
		t.Fatalf("protected work leaked: %v", e)
	}
	x, e = s.Claim("repo", "pull", "translator", "fr", "claim", "", "starting locale", 0)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Claim("repo", "pull", "regional-reviewer", "fr", "claim", "", "collision", 0); e != ErrConflict {
		t.Fatalf("expected optimistic conflict, got %v", e)
	}
	x, e = s.Discuss("repo", "pull", "translator", "unit", "fr", "Use the product term, but confirm the tone.")
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.Suggest("repo", "pull", "translator", "agent", "unit", "fr", "Confirmer l’espace de travail", "Medium: imperative tone depends on the confirmation flow.", []Evidence{{Kind: "source", Reference: "candidate:app.confirm", Summary: "Exact source message and product context"}, {Kind: "terminology", Reference: "plan@2:workspace", Summary: "Preferred term espace de travail"}})
	if e != nil {
		t.Fatal(e)
	}
	q := x.Suggestions[0]
	if q.Revision != "candidate" || q.RequestedByID != "translator" || len(q.Evidence) != 2 {
		t.Fatalf("opaque suggestion: %+v", q)
	}
	if _, e = s.DecideSuggestion("repo", "pull", "agent", q.ID, "approve", "", "looks good"); e != ErrForbidden {
		t.Fatalf("agent became its own authority: %v", e)
	}
	x, e = s.DecideSuggestion("repo", "pull", "translator", q.ID, "edit", "Confirmez l’espace de travail", "Human chose formal register")
	if e != nil {
		t.Fatal(e)
	}
	p := x.Proposals[len(x.Proposals)-1]
	if p.ActorID != "translator" || p.Origin != "agent_suggestion_edit" {
		t.Fatalf("human authorship missing: %+v", p)
	}
	if _, e = s.ReviewProposal("repo", "pull", "translator", p.ID, "approve", "self review"); e != ErrForbidden {
		t.Fatalf("review requirement bypassed: %v", e)
	}
	x, e = s.ReviewProposal("repo", "pull", "regional-reviewer", p.ID, "approve", "Terminology and register are appropriate")
	if e != nil || len(x.Reviews) != 1 {
		t.Fatalf("review missing: %+v %v", x.Reviews, e)
	}
	x, e = s.Claim("repo", "pull", "translator", "fr", "handoff", "regional-reviewer", "ready for regional review", 1)
	if e != nil || x.Claims[len(x.Claims)-1].ActorID != "regional-reviewer" {
		t.Fatalf("handoff missing: %+v %v", x.Claims, e)
	}
}
