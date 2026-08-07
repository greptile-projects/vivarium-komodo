package main

import (
	"bytes"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type repositoryBrowserStore interface {
	proposalRepositoryStore
	Open(storage.ID) (*storage.Repository, error)
}

func registerRepositoryBrowserHTTP(mux *http.ServeMux, store repositoryBrowserStore, credentials authStore) {
	mux.HandleFunc("GET /repositories/{repository}/branches", listRepositoryBranches(store, credentials))
	mux.HandleFunc("GET /repositories/{repository}/commits", listRepositoryCommits(store, credentials))
	mux.HandleFunc("GET /repositories/{repository}/commits/{commit}", getRepositoryCommit(store, credentials))
	mux.HandleFunc("GET /repositories/{repository}/tree", getRepositoryTree(store, credentials))
	mux.HandleFunc("GET /repositories/{repository}/blob", getRepositoryBlob(store, credentials))
}

type branchResponse struct {
	Name      string `json:"name"`
	CommitID  string `json:"commit_id"`
	IsDefault bool   `json:"is_default"`
}

type commitResponse struct {
	ID         string   `json:"id"`
	TreeID     string   `json:"tree_id"`
	ParentIDs  []string `json:"parent_ids"`
	Author     string   `json:"author"`
	Email      string   `json:"email"`
	AuthoredAt string   `json:"authored_at,omitempty"`
	Message    string   `json:"message"`
}

func openBrowsableRepository(w http.ResponseWriter, r *http.Request, store repositoryBrowserStore, credentials authStore) (*storage.Repository, bool) {
	item, _, ok := proposalRepositoryAccess(w, r, store, credentials, auth.RepositoryRead, false)
	if !ok {
		return nil, false
	}
	opened, err := store.Open(item.ID)
	if err != nil {
		writeRepositoryError(w, err)
		return nil, false
	}
	return opened, true
}

func listRepositoryBranches(store repositoryBrowserStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opened, ok := openBrowsableRepository(w, r, store, credentials)
		if !ok {
			return
		}
		defaultRef, err := opened.DefaultBranch()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		refs, err := opened.ListReferences()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		branches := make([]branchResponse, 0)
		for _, ref := range refs {
			name, found := strings.CutPrefix(string(ref.Name), "refs/heads/")
			if found && ref.ObjectID != "" {
				branches = append(branches, branchResponse{Name: name, CommitID: string(ref.ObjectID), IsDefault: ref.Name == defaultRef})
			}
		}
		sort.Slice(branches, func(i, j int) bool { return branches[i].Name < branches[j].Name })
		writeJSON(w, 200, map[string]any{"items": branches, "default_branch": strings.TrimPrefix(string(defaultRef), "refs/heads/")})
	}
}

func resolveRevision(repository *storage.Repository, revision string) (storage.ObjectID, string, error) {
	if revision == "" {
		defaultRef, err := repository.DefaultBranch()
		if err != nil {
			return "", "", err
		}
		revision = strings.TrimPrefix(string(defaultRef), "refs/heads/")
	}
	ref, err := repository.ReadReference(storage.ReferenceName("refs/heads/" + revision))
	if err == nil && ref.ObjectID != "" {
		return ref.ObjectID, revision, nil
	}
	id := storage.ObjectID(revision)
	if _, objectErr := repository.ReadCommit(id); objectErr == nil {
		return id, revision, nil
	}
	return "", "", storage.ErrObjectNotFound
}

func listRepositoryCommits(store repositoryBrowserStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opened, ok := openBrowsableRepository(w, r, store, credentials)
		if !ok {
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		tip, revision, err := resolveRevision(opened, r.URL.Query().Get("ref"))
		if err != nil {
			if errors.Is(err, storage.ErrObjectNotFound) {
				writeJSON(w, 404, map[string]string{"error": "revision_not_found"})
			} else {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
			}
			return
		}
		commits := make([]commitResponse, 0)
		seen := map[storage.ObjectID]bool{}
		queue := []storage.ObjectID{tip}
		for len(queue) > 0 && len(commits) < 1000 {
			id := queue[0]
			queue = queue[1:]
			if seen[id] {
				continue
			}
			seen[id] = true
			commit, readErr := opened.ReadCommit(id)
			if readErr != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			commits = append(commits, repositoryCommitResponse(commit))
			queue = append(queue, commit.Parents...)
		}
		total := len(commits)
		writeJSON(w, 200, map[string]any{"items": paginate(commits, page, perPage), "page": page, "per_page": perPage, "total_count": total, "revision": revision, "commit_id": tip})
	}
}

func getRepositoryCommit(store repositoryBrowserStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opened, ok := openBrowsableRepository(w, r, store, credentials)
		if !ok {
			return
		}
		commit, err := opened.ReadCommit(storage.ObjectID(r.PathValue("commit")))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "commit_not_found"})
			return
		}
		writeJSON(w, 200, repositoryCommitResponse(commit))
	}
}

func repositoryCommitResponse(commit storage.Commit) commitResponse {
	result := commitResponse{ID: string(commit.ID), TreeID: string(commit.Tree), ParentIDs: make([]string, len(commit.Parents))}
	for i, parent := range commit.Parents {
		result.ParentIDs[i] = string(parent)
	}
	headers, message, _ := bytes.Cut(commit.Content, []byte("\n\n"))
	result.Message = strings.TrimSpace(string(message))
	for _, line := range strings.Split(string(headers), "\n") {
		if !strings.HasPrefix(line, "author ") {
			continue
		}
		identity := strings.TrimPrefix(line, "author ")
		if end := strings.LastIndex(identity, ">"); end >= 0 {
			if start := strings.LastIndex(identity[:end], " <"); start >= 0 {
				result.Author, result.Email = identity[:start], identity[start+2:end]
				fields := strings.Fields(identity[end+1:])
				if len(fields) >= 1 {
					if seconds, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
						result.AuthoredAt = time.Unix(seconds, 0).UTC().Format(time.RFC3339)
					}
				}
			}
		}
	}
	return result
}

type treeEntryResponse struct {
	Name     string             `json:"name"`
	Path     string             `json:"path"`
	Type     storage.ObjectType `json:"type"`
	Mode     uint32             `json:"mode"`
	ObjectID string             `json:"object_id"`
	Size     uint64             `json:"size,omitempty"`
}

func treeAtPath(repository *storage.Repository, commitID storage.ObjectID, path string) (storage.Tree, error) {
	commit, err := repository.ReadCommit(commitID)
	if err != nil {
		return storage.Tree{}, err
	}
	tree, err := repository.ReadTree(commit.Tree)
	if err != nil {
		return storage.Tree{}, err
	}
	if path == "" {
		return tree, nil
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return storage.Tree{}, storage.ErrNotTree
		}
		found := false
		for _, entry := range tree.Entries {
			if entry.Name == segment && entry.Type == storage.TreeObject {
				tree, err = repository.ReadTree(entry.ObjectID)
				found = true
				break
			}
		}
		if err != nil || !found {
			return storage.Tree{}, storage.ErrNotTree
		}
	}
	return tree, nil
}

func getRepositoryTree(store repositoryBrowserStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opened, ok := openBrowsableRepository(w, r, store, credentials)
		if !ok {
			return
		}
		commitID, revision, err := resolveRevision(opened, r.URL.Query().Get("ref"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "revision_not_found"})
			return
		}
		path := strings.Trim(r.URL.Query().Get("path"), "/")
		tree, err := treeAtPath(opened, commitID, path)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "path_not_found"})
			return
		}
		entries := make([]treeEntryResponse, 0, len(tree.Entries))
		for _, entry := range tree.Entries {
			entryPath := entry.Name
			if path != "" {
				entryPath = path + "/" + entry.Name
			}
			item := treeEntryResponse{Name: entry.Name, Path: entryPath, Type: entry.Type, Mode: entry.Mode, ObjectID: string(entry.ObjectID)}
			if entry.Type == storage.BlobObject {
				if object, readErr := opened.ReadObject(entry.ObjectID); readErr == nil {
					item.Size = object.Size
				}
			}
			entries = append(entries, item)
		}
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].Type == entries[j].Type {
				return entries[i].Name < entries[j].Name
			}
			return entries[i].Type == storage.TreeObject
		})
		writeJSON(w, 200, map[string]any{"revision": revision, "commit_id": commitID, "path": path, "tree_id": tree.ID, "entries": entries})
	}
}

func getRepositoryBlob(store repositoryBrowserStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opened, ok := openBrowsableRepository(w, r, store, credentials)
		if !ok {
			return
		}
		commitID, revision, err := resolveRevision(opened, r.URL.Query().Get("ref"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "revision_not_found"})
			return
		}
		path := strings.Trim(r.URL.Query().Get("path"), "/")
		directory, name := "", path
		if index := strings.LastIndex(path, "/"); index >= 0 {
			directory, name = path[:index], path[index+1:]
		}
		tree, err := treeAtPath(opened, commitID, directory)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "path_not_found"})
			return
		}
		var objectID storage.ObjectID
		for _, entry := range tree.Entries {
			if entry.Name == name && entry.Type == storage.BlobObject {
				objectID = entry.ObjectID
				break
			}
		}
		if objectID == "" {
			writeJSON(w, 404, map[string]string{"error": "path_not_found"})
			return
		}
		object, err := opened.ReadObject(objectID)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "path_not_found"})
			return
		}
		const maxRenderedBlob = 1 << 20
		binary := bytes.IndexByte(object.Content, 0) >= 0 || !utf8.Valid(object.Content)
		content := ""
		truncated := object.Size > maxRenderedBlob
		if !binary {
			if truncated {
				content = string(object.Content[:maxRenderedBlob])
			} else {
				content = string(object.Content)
			}
		}
		writeJSON(w, 200, map[string]any{"revision": revision, "commit_id": commitID, "path": path, "object_id": object.ID, "size": object.Size, "binary": binary, "truncated": truncated, "content": content})
	}
}
