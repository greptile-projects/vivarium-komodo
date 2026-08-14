package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/translationunits"
)

func TestExtractionClassifiesRevisionExactTranslationWorkAndRetainsProposals(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repo, _ := git.Create()
	config := []byte(`{"schema_version":1,"source_locale":"en-US","locales":["fr-FR","ar"],"resources":[{"id":"app","source_path":"en.json","translation_path":"{locale}.json","format":"json","context":{"welcome":"Dashboard heading"},"screenshots":{"welcome":["https://example.test/dashboard.png"]},"plural_rules":{"count":"one, other"}}]}`)
	target := localizationCommit(t, repo, "target", config, map[string]string{
		"en.json":    `{"welcome":"Welcome","count":"{count} items","old":"Old","same":"Same"}`,
		"fr-FR.json": `{"welcome":"Bienvenue","count":"{count} éléments","old":"Ancien","same":"Même"}`,
	})
	candidate := localizationCommit(t, repo, "candidate", config, map[string]string{
		"en.json":    `{"welcome":"Welcome back, {name}","count":"{count} items","new":"New","same":"Same"}`,
		"fr-FR.json": `{"welcome":"Bienvenue","count":"{count} éléments","same":"Même"}`,
	})
	in, err := extractTranslationUnits(repo, string(candidate), string(target), ".komodo/localization.json")
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]translationunits.Unit{}
	for _, u := range in.Units {
		byKey[u.Key] = u
	}
	if byKey["welcome"].Change != "changed" || byKey["welcome"].Translations["fr-FR"].Status != "superseded" || byKey["welcome"].Context == "" || len(byKey["welcome"].ScreenshotURLs) != 1 || len(byKey["welcome"].Variables) != 1 {
		t.Fatalf("changed source lost translator context: %+v", byKey["welcome"])
	}
	if byKey["new"].Change != "added" || byKey["new"].Translations["ar"].Status != "untranslated" || byKey["old"].Change != "removed" || byKey["same"].Change != "reused" || byKey["count"].PluralRule != "one, other" {
		t.Fatalf("change classification incomplete: %+v", byKey)
	}
	store, _ := translationunits.New(t.TempDir())
	x, err := store.Create("repo", "pull", "developer", in)
	if err != nil {
		t.Fatal(err)
	}
	x, err = store.Propose("repo", "pull", "translator", byKey["welcome"].ID, "fr-FR", "Bon retour, {name}")
	if err != nil {
		t.Fatal(err)
	}
	if x.Units[0].ID == "" || len(x.Proposals) != 1 || x.Proposals[0].ActorID != "translator" {
		t.Fatalf("proposal attribution missing: %+v", x)
	}
	for i := range in.Units {
		if in.Units[i].ID == byKey["welcome"].ID {
			in.Units[i].Message += "!"
		}
	}
	newer, err := store.Create("repo", "pull", "developer", in)
	if err != nil {
		t.Fatal(err)
	}
	if len(newer.Proposals) != 1 || !newer.Proposals[0].Superseded {
		t.Fatalf("source evolution erased or reused history: %+v", newer.Proposals)
	}
}

func TestRepositoryReaderCanProposeTranslationWithoutWriteAccess(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "localized", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "translator")
	opened, _ := catalog.Open(repo.ID)
	config := []byte(`{"schema_version":1,"source_locale":"en","locales":["fr"],"resources":[{"id":"app","source_path":"en.json","translation_path":"{locale}.json","format":"json"}]}`)
	target := localizationCommit(t, opened, "target", config, map[string]string{"en.json": `{"hello":"Hello"}`})
	candidate := localizationCommit(t, opened, "candidate", config, map[string]string{"en.json": `{"hello":"Hello there"}`})
	pulls, _ := pullrequests.New(t.TempDir())
	pull, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repo.ID), AuthorID: "owner", Title: "Change greeting", SourceBranch: "change", TargetBranch: "main", SourceCommitID: string(candidate), TargetCommitID: string(target)})
	credentials, _ := auth.New(t.TempDir())
	token := issueAccess(t, credentials, "translator", auth.API, auth.RepositoryRead)
	store, _ := translationunits.New(t.TempDir())
	mux := http.NewServeMux()
	registerTranslationUnitsHTTP(mux, store, catalog, credentials, translationUnitSources{pulls: pulls, repositories: catalog})
	server := httptest.NewServer(mux)
	defer server.Close()
	var extraction translationunits.Extraction
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/pull-requests/"+pull.ID+"/translation-units/extract", token, `{"revision":"`+string(candidate)+`"}`, 201, &extraction)
	if len(extraction.Units) != 1 {
		t.Fatalf("missing extracted unit: %+v", extraction)
	}
	body, _ := json.Marshal(map[string]string{"locale_id": "fr", "text": "Bonjour"})
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/pull-requests/"+pull.ID+"/translation-units/"+extraction.Units[0].ID+"/proposals", token, string(body), 201, &extraction)
	if len(extraction.Proposals) != 1 || extraction.Proposals[0].ActorID != "translator" {
		t.Fatalf("reader proposal not attributed: %+v", extraction.Proposals)
	}
}

func localizationCommit(t *testing.T, repo *storage.Repository, message string, config []byte, files map[string]string) storage.ObjectID {
	t.Helper()
	configID, _ := repo.WriteObject(storage.BlobObject, config)
	komodo, _ := repo.WriteObject(storage.TreeObject, dataFlowTreeEntry("100644", "localization.json", configID))
	entries := dataFlowTreeEntry("40000", ".komodo", komodo)
	for _, name := range []string{"en.json", "fr-FR.json", "ar.json"} {
		if content, ok := files[name]; ok {
			id, _ := repo.WriteObject(storage.BlobObject, []byte(content))
			entries = append(entries, dataFlowTreeEntry("100644", name, id)...)
		}
	}
	root, _ := repo.WriteObject(storage.TreeObject, entries)
	commit, err := repo.WriteObject(storage.CommitObject, []byte("tree "+string(root)+"\nauthor Translator <t@example.test> 1 +0000\ncommitter Translator <t@example.test> 1 +0000\n\n"+message+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	return commit
}
