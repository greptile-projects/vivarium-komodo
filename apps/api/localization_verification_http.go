package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/localizationverification"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/translationunits"
)

type localizationVerificationSources struct {
	pulls interface {
		Get(string, string) (pullrequests.PullRequest, error)
	}
	repositories interface {
		Open(storage.ID) (*storage.Repository, error)
	}
	translations interface {
		Authorized(string, string, string) (translationunits.Extraction, error)
	}
	previews interface {
		Get(string, string, string) (previews.Preview, error)
	}
}
type localizationChecksConfig struct {
	SchemaVersion int                           `json:"schema_version"`
	Checks        []localizationCheckDefinition `json:"checks"`
}
type localizationCheckDefinition struct {
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	LocaleID       string   `json:"locale_id"`
	JourneyID      string   `json:"journey_id"`
	Route          string   `json:"route"`
	Command        string   `json:"command"`
	InterfacePaths []string `json:"interface_paths"`
	UnitIDs        []string `json:"unit_ids"`
}

func registerLocalizationVerificationHTTP(mux *http.ServeMux, s *localizationverification.Store, repos proposalRepositoryStore, c authStore, src localizationVerificationSources) {
	base := "/repositories/{repository}/pull-requests/{pull}/localization-verification"
	access := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	mux.HandleFunc("GET /repositories/{repository}/localization-verifications", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r)
		if !ok {
			return
		}
		items, e := s.List(repo)
		if localizationVerificationError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		if _, e := src.translations.Authorized(repo, r.PathValue("pull"), actor); localizationVerificationError(w, e) {
			return
		}
		a, e := s.Get(repo, r.PathValue("pull"))
		if localizationVerificationError(w, e) {
			return
		}
		writeJSON(w, 200, a)
	})
	mux.HandleFunc("POST "+base+"/runs", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			Revision   string `json:"revision"`
			ConfigPath string `json:"config_path"`
			Results    map[string]struct {
				Status  string `json:"status"`
				Summary string `json:"summary"`
			} `json:"results"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		if in.ConfigPath == "" {
			in.ConfigPath = ".komodo/localization-checks.json"
		}
		p, e := src.pulls.Get(repo, r.PathValue("pull"))
		if e != nil || p.SourceCommitID != in.Revision {
			writeJSON(w, 409, map[string]string{"error": "exact_pull_request_revision_required"})
			return
		}
		x, e := src.translations.Authorized(repo, p.ID, actor)
		if e != nil || x.Revision != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "current_translation_extraction_required"})
			return
		}
		opened, e := src.repositories.Open(storage.ID(p.SourceRepositoryID))
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "localization_source_unavailable"})
			return
		}
		blob, body, ok := localizationBlob(opened, in.Revision, in.ConfigPath)
		if !ok {
			writeJSON(w, 422, map[string]string{"error": "localization_checks_invalid"})
			return
		}
		var cfg localizationChecksConfig
		if json.Unmarshal(body, &cfg) != nil || cfg.SchemaVersion != 1 || len(cfg.Checks) == 0 {
			writeJSON(w, 422, map[string]string{"error": "localization_checks_invalid"})
			return
		}
		checks := []localizationverification.Check{}
		kinds := map[string]bool{}
		for _, d := range cfg.Checks {
			result, exists := in.Results[d.Name]
			if !exists || result.Summary == "" {
				writeJSON(w, 422, map[string]string{"error": "localization_check_result_missing"})
				return
			}
			sd, td, ui, valid := localizationInputDigests(opened, in.Revision, x, d)
			if !valid {
				writeJSON(w, 422, map[string]string{"error": "localization_check_input_invalid"})
				return
			}
			checks = append(checks, localizationverification.Check{Name: d.Name, Kind: d.Kind, LocaleID: d.LocaleID, JourneyID: d.JourneyID, Route: d.Route, Status: result.Status, Summary: result.Summary, SourceDigest: sd, TranslationDigest: td, InterfaceDigest: ui, Command: d.Command, UnitIDs: d.UnitIDs, InterfacePaths: d.InterfacePaths})
			kinds[d.Kind] = true
		}
		for _, required := range []string{"variables", "pluralization", "formatting", "terminology", "links", "layout_expansion", "bidirectional_text", "fallback", "journey"} {
			if !kinds[required] {
				writeJSON(w, 422, map[string]string{"error": "localization_check_class_missing"})
				return
			}
		}
		a, e := s.Put(repo, p.ID, in.Revision, in.ConfigPath, string(blob), actor, checks)
		if localizationVerificationError(w, e) {
			return
		}
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("POST "+base+"/previews", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			PreviewID   string   `json:"preview_id"`
			LocaleID    string   `json:"locale_id"`
			Revision    string   `json:"revision"`
			Routes      []string `json:"routes"`
			ReviewerIDs []string `json:"reviewer_ids"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		p, e := src.previews.Get(repo, r.PathValue("pull"), in.PreviewID)
		if e != nil || p.Revision != in.Revision || p.State != "ready" || p.URL == "" {
			writeJSON(w, 422, map[string]string{"error": "ready_exact_revision_preview_required"})
			return
		}
		x, e := src.translations.Authorized(repo, r.PathValue("pull"), actor)
		if e != nil {
			localizationVerificationError(w, e)
			return
		}
		allowed := map[string]bool{}
		validLocale := false
		for _, locale := range x.Locales {
			validLocale = validLocale || locale == in.LocaleID
		}
		if !validLocale {
			writeJSON(w, 422, map[string]string{"error": "localization_locale_invalid"})
			return
		}
		for _, id := range x.ReviewerIDs[in.LocaleID] {
			allowed[id] = true
		}
		for _, id := range in.ReviewerIDs {
			if !allowed[id] {
				writeJSON(w, 403, map[string]string{"error": "reviewer_not_invited_by_locale_plan"})
				return
			}
		}
		a, e := s.AddPreview(repo, r.PathValue("pull"), actor, in.PreviewID, in.LocaleID, in.Revision, p.URL, localizationverification.Clean(in.Routes), localizationverification.Clean(in.ReviewerIDs))
		if localizationVerificationError(w, e) {
			return
		}
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("POST "+base+"/previews/{preview}/findings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			Route          string   `json:"route"`
			Kind           string   `json:"kind"`
			Severity       string   `json:"severity"`
			Body           string   `json:"body"`
			UnitIDs        []string `json:"unit_ids"`
			InterfacePaths []string `json:"interface_paths"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		if _, e := src.translations.Authorized(repo, r.PathValue("pull"), actor); localizationVerificationError(w, e) {
			return
		}
		a, e := s.AddFinding(repo, r.PathValue("pull"), actor, r.PathValue("preview"), in.Route, in.Kind, in.Severity, in.Body, in.UnitIDs, in.InterfacePaths)
		if localizationVerificationError(w, e) {
			return
		}
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("POST "+base+"/previews/{preview}/decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			Route     string `json:"route"`
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		if _, e := src.translations.Authorized(repo, r.PathValue("pull"), actor); localizationVerificationError(w, e) {
			return
		}
		a, e := s.Decide(repo, r.PathValue("pull"), actor, r.PathValue("preview"), in.Route, in.Decision, in.Rationale)
		if localizationVerificationError(w, e) {
			return
		}
		writeJSON(w, 201, a)
	})
}

func localizationInputDigests(repo *storage.Repository, revision string, x translationunits.Extraction, d localizationCheckDefinition) (string, string, string, bool) {
	unitSet := map[string]bool{}
	for _, id := range d.UnitIDs {
		unitSet[id] = true
	}
	sources, translations := []string{}, []string{}
	for _, u := range x.Units {
		if len(unitSet) > 0 && !unitSet[u.ID] {
			continue
		}
		state, ok := u.Translations[d.LocaleID]
		if !ok {
			return "", "", "", false
		}
		sources = append(sources, u.ID+":"+u.Message+":"+u.Location.BlobID)
		translations = append(translations, u.ID+":"+state.Text+":"+state.Status)
	}
	if len(sources) == 0 {
		return "", "", "", false
	}
	interfaces := []string{}
	for _, path := range d.InterfacePaths {
		blob, _, ok := localizationBlob(repo, revision, path)
		if !ok {
			return "", "", "", false
		}
		interfaces = append(interfaces, path+":"+string(blob))
	}
	sort.Strings(sources)
	sort.Strings(translations)
	sort.Strings(interfaces)
	return hashLocalization(sources), hashLocalization(translations), hashLocalization(interfaces), true
}
func hashLocalization(xs []string) string {
	h := sha256.Sum256([]byte(strings.Join(xs, "\x00")))
	return hex.EncodeToString(h[:])
}
func localizationVerificationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	status, code := 422, "localization_verification_invalid"
	switch e {
	case localizationverification.ErrNotFound:
		status, code = 404, "localization_verification_not_found"
	case localizationverification.ErrForbidden, translationunits.ErrForbidden:
		status, code = 403, "localization_verification_forbidden"
	}
	writeJSON(w, status, map[string]string{"error": code})
	return true
}
