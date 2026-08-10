package main

import (
	"bytes"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/relationships"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// Code intelligence is deliberately derived from one immutable commit on every
// request. This keeps the first implementation useful without creating an index
// whose revision or authorization can drift from repository state.
type codeIntelligenceStore interface {
	proposalRepositoryStore
	Open(storage.ID) (*storage.Repository, error)
}

type codeRelationshipStore interface {
	Dependencies() ([]relationships.Dependency, error)
	Interfaces() ([]relationships.Interface, error)
}

type sourceLocation struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

type codeSymbol struct {
	Name       string           `json:"name"`
	Kind       string           `json:"kind"`
	Language   string           `json:"language"`
	Definition sourceLocation   `json:"definition"`
	References []sourceLocation `json:"references"`
	Callers    []sourceLocation `json:"callers"`
	Tests      []sourceLocation `json:"tests"`
	Owner      *commitResponse  `json:"owner,omitempty"`
}

type codeDependency struct {
	ID                   string `json:"id"`
	RepositoryID         string `json:"repository_id"`
	ProviderRepositoryID string `json:"provider_repository_id"`
	InterfaceName        string `json:"interface_name"`
	Constraint           string `json:"constraint"`
	CommitID             string `json:"commit_id"`
	State                string `json:"state"`
	EvidencePath         string `json:"evidence_path,omitempty"`
}

type indexedSource struct {
	path     string
	language string
	lines    []string
	objectID storage.ObjectID
}

var definitionPatterns = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"function", regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?(?:func|function|fn|def)\s+(?:\([^)]*\)\s*)?([A-Za-z_$][\w$]*)`)},
	{"type", regexp.MustCompile(`^\s*(?:export\s+)?(?:type|interface|class|struct|enum)\s+([A-Za-z_$][\w$]*)`)},
	{"function", regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*=>`)},
}

func registerCodeIntelligenceHTTP(mux *http.ServeMux, store codeIntelligenceStore, credentials authStore, relationshipStore codeRelationshipStore) {
	mux.HandleFunc("GET /repositories/{repository}/code-intelligence", getCodeIntelligence(store, credentials, relationshipStore))
}

func getCodeIntelligence(store codeIntelligenceStore, credentials authStore, relationshipStore codeRelationshipStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := proposalRepositoryAccess(w, r, store, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		opened, err := store.Open(item.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		commitID, revision, err := resolveRevision(opened, r.URL.Query().Get("ref"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "revision_not_found"})
			return
		}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		selected := strings.TrimSpace(r.URL.Query().Get("symbol"))
		if len(query) > 200 || len(selected) > 200 {
			writeJSON(w, 422, map[string]string{"error": "invalid_query"})
			return
		}

		sources, skipped, limited, err := collectSources(opened, commitID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		symbols := buildSymbols(opened, commitID, sources, query, selected)
		dependencies, hidden, err := readableCodeDependencies(store, relationshipStore, item, actor.UserID, string(commitID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		status := "complete"
		reasons := []string{}
		if limited {
			status = "incomplete"
			reasons = append(reasons, "analysis_limit_reached")
		}
		if skipped > 0 {
			status = "incomplete"
			reasons = append(reasons, "binary_or_unsupported_files_skipped")
		}
		if hidden > 0 {
			status = "incomplete"
			reasons = append(reasons, "dependencies_hidden_by_permissions")
		}
		writeJSON(w, 200, map[string]any{
			"repository_id": item.ID, "revision": revision, "commit_id": commitID,
			"analysis": map[string]any{"status": status, "stale": false, "analyzed_at": time.Now().UTC(), "files_analyzed": len(sources), "files_skipped": skipped, "reasons": reasons},
			"symbols":  symbols, "dependencies": dependencies,
		})
	}
}

func languageForPath(path string) string {
	switch {
	case strings.HasSuffix(path, ".go"):
		return "go"
	case strings.HasSuffix(path, ".ts"), strings.HasSuffix(path, ".tsx"):
		return "typescript"
	case strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".jsx"):
		return "javascript"
	case strings.HasSuffix(path, ".py"):
		return "python"
	case strings.HasSuffix(path, ".rs"):
		return "rust"
	default:
		return ""
	}
}

func collectSources(repo *storage.Repository, commitID storage.ObjectID) ([]indexedSource, int, bool, error) {
	commit, err := repo.ReadCommit(commitID)
	if err != nil {
		return nil, 0, false, err
	}
	const maxFiles, maxBytes = 2000, 20 << 20
	var sources []indexedSource
	skipped, total := 0, 0
	limited := false
	var walk func(storage.ObjectID, string) error
	walk = func(treeID storage.ObjectID, prefix string) error {
		tree, er := repo.ReadTree(treeID)
		if er != nil {
			return er
		}
		for _, entry := range tree.Entries {
			path := entry.Name
			if prefix != "" {
				path = prefix + "/" + entry.Name
			}
			if entry.Type == storage.TreeObject {
				if er = walk(entry.ObjectID, path); er != nil {
					return er
				}
				continue
			}
			if entry.Type != storage.BlobObject {
				continue
			}
			language := languageForPath(path)
			if language == "" {
				skipped++
				continue
			}
			object, readErr := repo.ReadObject(entry.ObjectID)
			if readErr != nil {
				return readErr
			}
			if len(sources) >= maxFiles || total+len(object.Content) > maxBytes {
				limited = true
				continue
			}
			if bytes.IndexByte(object.Content, 0) >= 0 || !utf8.Valid(object.Content) {
				skipped++
				continue
			}
			total += len(object.Content)
			sources = append(sources, indexedSource{path: path, language: language, lines: strings.Split(string(object.Content), "\n"), objectID: entry.ObjectID})
		}
		return nil
	}
	err = walk(commit.Tree, "")
	sort.Slice(sources, func(i, j int) bool { return sources[i].path < sources[j].path })
	return sources, skipped, limited, err
}

func buildSymbols(repo *storage.Repository, commitID storage.ObjectID, sources []indexedSource, query, selected string) []codeSymbol {
	var out []codeSymbol
	for _, source := range sources {
		for lineIndex, line := range source.lines {
			for _, pattern := range definitionPatterns {
				match := pattern.re.FindStringSubmatch(line)
				if len(match) != 2 {
					continue
				}
				name := match[1]
				if selected != "" && name != selected {
					continue
				}
				if query != "" && !strings.Contains(strings.ToLower(name+" "+source.path), strings.ToLower(query)) {
					continue
				}
				symbol := codeSymbol{Name: name, Kind: pattern.kind, Language: source.language, Definition: sourceLocation{Path: source.path, Line: lineIndex + 1}}
				// Collection queries return lightweight definition hits. Expensive
				// relationship and history expansion is reserved for an explicitly
				// selected symbol, keeping bounded scans responsive on large trees.
				if selected == "" {
					out = append(out, symbol)
					if len(out) >= 250 {
						return out
					}
					break
				}
				word := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
				call := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*\(`)
				for _, candidate := range sources {
					for i, candidateLine := range candidate.lines {
						if candidate.path == source.path && i == lineIndex {
							continue
						}
						location := sourceLocation{Path: candidate.path, Line: i + 1}
						if word.MatchString(candidateLine) {
							symbol.References = append(symbol.References, location)
							if isTestPath(candidate.path) {
								symbol.Tests = append(symbol.Tests, location)
							}
						}
						if pattern.kind == "function" && call.MatchString(candidateLine) {
							symbol.Callers = append(symbol.Callers, location)
						}
					}
				}
				if owner := lastPathCommit(repo, commitID, source.path, source.objectID); owner != nil {
					symbol.Owner = owner
				}
				out = append(out, symbol)
				break
			}
			if len(out) >= 250 {
				return out
			}
		}
	}
	return out
}

func isTestPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "/test") || strings.Contains(lower, "_test.") || strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.")
}

func objectAtPath(repo *storage.Repository, commitID storage.ObjectID, path string) storage.ObjectID {
	dir, name := "", path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		dir, name = path[:i], path[i+1:]
	}
	tree, err := treeAtPath(repo, commitID, dir)
	if err != nil {
		return ""
	}
	for _, entry := range tree.Entries {
		if entry.Name == name && entry.Type == storage.BlobObject {
			return entry.ObjectID
		}
	}
	return ""
}

func lastPathCommit(repo *storage.Repository, tip storage.ObjectID, path string, current storage.ObjectID) *commitResponse {
	id := tip
	for steps := 0; steps < 500 && id != ""; steps++ {
		commit, err := repo.ReadCommit(id)
		if err != nil {
			return nil
		}
		if len(commit.Parents) == 0 || objectAtPath(repo, commit.Parents[0], path) != current {
			response := repositoryCommitResponse(commit)
			return &response
		}
		id = commit.Parents[0]
	}
	return nil
}

func readableCodeDependencies(store codeIntelligenceStore, rel codeRelationshipStore, repository repositories.Repository, actorID, commitID string) ([]codeDependency, int, error) {
	if rel == nil {
		return []codeDependency{}, 0, nil
	}
	items, err := rel.Dependencies()
	if err != nil {
		return nil, 0, err
	}
	interfaces, err := rel.Interfaces()
	if err != nil {
		return nil, 0, err
	}
	var out []codeDependency
	hidden := 0
	for _, item := range items {
		if item.RepositoryID != string(repository.ID) || item.CommitID != commitID {
			continue
		}
		provider, inspectErr := store.Inspect(storage.ID(item.ProviderRepositoryID))
		readable := inspectErr == nil && (provider.Visibility == repositories.Public || provider.OwnerID == actorID)
		if inspectErr == nil && !readable && actorID != "" {
			readable, _ = store.IsCollaborator(provider.ID, actorID)
		}
		if !readable {
			hidden++
			continue
		}
		state, evidence := "unresolved", ""
		for _, published := range interfaces {
			if published.RepositoryID == item.ProviderRepositoryID && strings.EqualFold(published.Name, item.InterfaceName) {
				state = "resolved"
				evidence = published.SchemaPath
				break
			}
		}
		out = append(out, codeDependency{ID: item.ID, RepositoryID: item.RepositoryID, ProviderRepositoryID: item.ProviderRepositoryID, InterfaceName: item.InterfaceName, Constraint: item.Constraint, CommitID: item.CommitID, State: state, EvidencePath: evidence})
	}
	return out, hidden, nil
}
