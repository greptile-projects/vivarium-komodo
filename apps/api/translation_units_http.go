package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/translationunits"
)

type localizationConfig struct {
	SchemaVersion int                    `json:"schema_version"`
	SourceLocale  string                 `json:"source_locale"`
	Locales       []string               `json:"locales"`
	Resources     []localizationResource `json:"resources"`
}
type localizationResource struct {
	ID              string              `json:"id"`
	SourcePath      string              `json:"source_path"`
	TranslationPath string              `json:"translation_path"`
	Format          string              `json:"format"`
	Context         map[string]string   `json:"context"`
	Screenshots     map[string][]string `json:"screenshots"`
	PluralRules     map[string]string   `json:"plural_rules"`
}
type translationUnitSources struct {
	pulls interface {
		Get(string, string) (pullrequests.PullRequest, error)
	}
	repositories interface {
		Open(storage.ID) (*storage.Repository, error)
	}
}

func registerTranslationUnitsHTTP(mux *http.ServeMux, s *translationunits.Store, repos proposalRepositoryStore, c authStore, src translationUnitSources) {
	base := "/repositories/{repository}/pull-requests/{pull}/translation-units"
	access := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	mux.HandleFunc("GET /repositories/{repository}/translation-extractions", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r)
		if !ok {
			return
		}
		items, e := s.List(repo)
		if translationUnitError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r)
		if !ok {
			return
		}
		x, e := s.Get(repo, r.PathValue("pull"))
		if translationUnitError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/extract", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			Revision   string `json:"revision"`
			ConfigPath string `json:"config_path"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		if in.ConfigPath == "" {
			in.ConfigPath = ".komodo/localization.json"
		}
		p, e := src.pulls.Get(repo, r.PathValue("pull"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "pull_request_not_found"})
			return
		}
		if in.Revision != p.SourceCommitID {
			writeJSON(w, 409, map[string]string{"error": "exact_pull_request_revision_required"})
			return
		}
		opened, e := src.repositories.Open(storage.ID(p.SourceRepositoryID))
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "localization_source_unavailable"})
			return
		}
		input, e := extractTranslationUnits(opened, p.SourceCommitID, p.TargetCommitID, in.ConfigPath)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_localization_extraction"})
			return
		}
		x, e := s.Create(repo, p.ID, actor, input)
		if translationUnitError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/{unit}/proposals", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			LocaleID string `json:"locale_id"`
			Text     string `json:"text"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.Propose(repo, r.PathValue("pull"), actor, r.PathValue("unit"), in.LocaleID, in.Text)
		if translationUnitError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
}

func extractTranslationUnits(repo *storage.Repository, candidate, target, configPath string) (translationunits.Input, error) {
	configBlob, content, ok := localizationBlob(repo, candidate, configPath)
	if !ok {
		return translationunits.Input{}, translationunits.ErrInvalid
	}
	var cfg localizationConfig
	if json.Unmarshal(content, &cfg) != nil || cfg.SchemaVersion != 1 || cfg.SourceLocale == "" || len(cfg.Locales) == 0 || len(cfg.Resources) == 0 {
		return translationunits.Input{}, translationunits.ErrInvalid
	}
	localeSet := map[string]bool{}
	for _, l := range cfg.Locales {
		if l == "" || l == cfg.SourceLocale || localeSet[l] {
			return translationunits.Input{}, translationunits.ErrInvalid
		}
		localeSet[l] = true
	}
	units := []translationunits.Unit{}
	resourceIDs := map[string]bool{}
	for _, r := range cfg.Resources {
		if r.ID == "" || r.SourcePath == "" || r.TranslationPath == "" || r.Format != "json" || resourceIDs[r.ID] {
			return translationunits.Input{}, translationunits.ErrInvalid
		}
		resourceIDs[r.ID] = true
		cb, cc, cok := localizationBlob(repo, candidate, r.SourcePath)
		tb, tc, tok := localizationBlob(repo, target, r.SourcePath)
		_ = tb
		if !cok && !tok {
			return translationunits.Input{}, translationunits.ErrInvalid
		}
		cm := map[string]string{}
		tm := map[string]string{}
		if cok && !flattenMessages(cc, cm, " ") {
			return translationunits.Input{}, translationunits.ErrInvalid
		}
		if tok && !flattenMessages(tc, tm, " ") {
			return translationunits.Input{}, translationunits.ErrInvalid
		}
		keys := map[string]bool{}
		for k := range cm {
			keys[k] = true
		}
		for k := range tm {
			keys[k] = true
		}
		ordered := make([]string, 0, len(keys))
		for k := range keys {
			ordered = append(ordered, k)
		}
		sort.Strings(ordered)
		translations := map[string]map[string]string{}
		priorTranslations := map[string]map[string]string{}
		translationBlobs := map[string]string{}
		for _, l := range cfg.Locales {
			path := strings.ReplaceAll(r.TranslationPath, "{locale}", l)
			if b, c, exists := localizationBlob(repo, candidate, path); exists {
				m := map[string]string{}
				if !flattenMessages(c, m, " ") {
					return translationunits.Input{}, translationunits.ErrInvalid
				}
				translations[l] = m
				translationBlobs[l] = string(b)
			}
			if _, c, exists := localizationBlob(repo, target, path); exists {
				m := map[string]string{}
				if flattenMessages(c, m, " ") {
					priorTranslations[l] = m
				}
			}
		}
		for _, k := range ordered {
			msg, hasCandidate := cm[k]
			prior, hasTarget := tm[k]
			change := "reused"
			if !hasTarget {
				change = "added"
			} else if !hasCandidate {
				change = "removed"
				msg = prior
			} else if prior != msg {
				change = "changed"
			}
			blob := string(cb)
			body := cc
			if !hasCandidate {
				blob = string(tb)
				body = tc
			}
			u := translationunits.Unit{ID: stableTranslationUnitID(r.ID, k), ResourceID: r.ID, Key: k, Message: msg, PriorMessage: prior, Context: r.Context[k], ScreenshotURLs: r.Screenshots[k], Variables: messageVariables(msg), PluralRule: r.PluralRules[k], Location: translationunits.Location{Path: r.SourcePath, Line: messageLine(body, k), BlobID: blob}, Change: change, Translations: map[string]translationunits.TranslationState{}}
			for _, l := range cfg.Locales {
				v, present := translations[l][k]
				status := "untranslated"
				source := msg
				if change == "removed" {
					status = "removed"
				} else if present {
					status = "translated"
					if change == "changed" && priorTranslations[l][k] == v {
						status = "superseded"
						source = prior
					}
				}
				u.Translations[l] = translationunits.TranslationState{Text: v, Status: status, SourceMessage: source}
				_ = translationBlobs[l]
			}
			units = append(units, u)
		}
	}
	return translationunits.Input{Revision: candidate, TargetRevision: target, SourceLocale: cfg.SourceLocale, Locales: cfg.Locales, ConfigPath: configPath, ConfigBlobID: string(configBlob), Units: units}, nil
}
func localizationBlob(r *storage.Repository, revision, path string) (storage.ObjectID, []byte, bool) {
	oid, ok := assessmentBlob(r, storage.ObjectID(revision), path)
	if !ok {
		return "", nil, false
	}
	o, e := r.ReadObject(oid)
	if e != nil || o.Type != storage.BlobObject {
		return "", nil, false
	}
	return oid, o.Content, true
}
func flattenMessages(b []byte, out map[string]string, _ string) bool {
	var v any
	if json.Unmarshal(b, &v) != nil {
		return false
	}
	var walk func(any, string) bool
	walk = func(x any, p string) bool {
		switch y := x.(type) {
		case string:
			if p == "" {
				return false
			}
			out[p] = y
			return true
		case map[string]any:
			for k, v := range y {
				n := k
				if p != "" {
					n = p + "." + k
				}
				if !walk(v, n) {
					return false
				}
			}
			return true
		default:
			return false
		}
	}
	return walk(v, "")
}
func stableTranslationUnitID(resource, key string) string {
	d := sha256.Sum256([]byte(resource + "\x00" + key))
	return hex.EncodeToString(d[:12])
}

var variablePattern = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)`)

func messageVariables(s string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range variablePattern.FindAllStringSubmatch(s, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out
}
func messageLine(b []byte, key string) int {
	leaf := key
	if i := strings.LastIndex(key, "."); i >= 0 {
		leaf = key[i+1:]
	}
	needle := fmt.Sprintf("%q", leaf)
	for i, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	return 1
}
func translationUnitError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, translationunits.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "translation_extraction_not_found"})
	case errors.Is(e, translationunits.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_translation_work"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
