package localizationverification

import "testing"

func checks(source, translation, ui string) []Check {
	out := []Check{
		{Name: "variables", Kind: "variables", LocaleID: "ar", Route: "/checkout", Status: "passed", Summary: "variables preserved", SourceDigest: source, TranslationDigest: translation, InterfaceDigest: ui},
		{Name: "plurals", Kind: "pluralization", LocaleID: "ar", Route: "/checkout", Status: "passed", Summary: "six Arabic forms exercised", SourceDigest: source, TranslationDigest: translation, InterfaceDigest: ui},
		{Name: "formats", Kind: "formatting", LocaleID: "ar", Route: "/checkout", Status: "passed", Summary: "currency and date localized", SourceDigest: source, TranslationDigest: translation, InterfaceDigest: ui},
		{Name: "terms", Kind: "terminology", LocaleID: "ar", Route: "/checkout", Status: "passed", Summary: "plan terminology used", SourceDigest: source, TranslationDigest: translation, InterfaceDigest: ui},
		{Name: "links", Kind: "links", LocaleID: "ar", Route: "/checkout", Status: "passed", Summary: "locale links resolve", SourceDigest: source, TranslationDigest: translation, InterfaceDigest: ui},
		{Name: "layout", Kind: "layout_expansion", LocaleID: "ar", Route: "/checkout", Status: "passed", Summary: "expanded labels fit", SourceDigest: source, TranslationDigest: translation, InterfaceDigest: ui},
		{Name: "rtl", Kind: "bidirectional_text", LocaleID: "ar", Route: "/checkout", Status: "failed", Summary: "mixed SKU text reverses", SourceDigest: source, TranslationDigest: translation, InterfaceDigest: ui},
		{Name: "fallback", Kind: "fallback", LocaleID: "ar", Route: "/checkout", Status: "passed", Summary: "regional fallback explicit", SourceDigest: source, TranslationDigest: translation, InterfaceDigest: ui},
		{Name: "journey", Kind: "journey", LocaleID: "ar", JourneyID: "buy", Route: "/checkout", Status: "passed", Summary: "checkout completes", SourceDigest: source, TranslationDigest: translation, InterfaceDigest: ui},
	}
	for i := range out {
		out[i].UnitIDs = []string{"sku-label"}
		out[i].InterfacePaths = []string{"ui/checkout.tsx"}
	}
	out[0].UnitIDs = []string{"unrelated-label"}
	return out
}

func TestRevisionExactLocalizedReviewAndSelectiveInvalidation(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a, e := s.Put("repo", "pull", "candidate", ".komodo/localization-checks.json", "config", "developer", checks("source-a", "translation-a", "ui-a"))
	if e != nil || len(a.Checks) != 9 {
		t.Fatalf("checks not retained: %+v %v", a, e)
	}
	a, e = s.AddPreview("repo", "pull", "translator", "runtime-preview", "ar", "candidate", "https://preview", []string{"/checkout"}, []string{"regional"})
	if e != nil {
		t.Fatal(e)
	}
	pid := a.Previews[0].ID
	a, e = s.AddFinding("repo", "pull", "translator", pid, "/checkout", "directionality", "blocking", "SKU and Arabic product name render in the wrong visual order", []string{"sku-label"}, []string{"ui/checkout.tsx"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Decide("repo", "pull", "translator", pid, "/checkout", "approve", "looks fine"); e != ErrForbidden {
		t.Fatalf("uninvited translator decided: %v", e)
	}
	a, e = s.Decide("repo", "pull", "regional", pid, "/checkout", "reject", "the purchase journey is unusable in RTL")
	if e != nil {
		t.Fatal(e)
	}
	if a.Findings[0].Stale || a.Decisions[0].Stale {
		t.Fatal("new evidence unexpectedly stale")
	}
	// An unrelated unit changes while this finding's source, translation, and interface inputs do not.
	next := checks("source-a", "translation-a", "ui-a")
	next[0].TranslationDigest = "unrelated-translation-b"
	a, e = s.Put("repo", "pull", "candidate-two", ".komodo/localization-checks.json", "config-two", "developer", next)
	if e != nil {
		t.Fatal(e)
	}
	if a.Findings[0].Stale {
		t.Fatal("unaffected unit-grounded finding became stale")
	}
	if !a.Decisions[0].Stale {
		t.Fatal("route-wide decision ignored an affected translation")
	}
	if a.Checks[0].SourceDigest != "source-a" || a.Checks[0].InterfaceDigest != "ui-a" {
		t.Fatal("unaffected input identities were lost")
	}
}
