package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type pullRequestCommit struct {
	ID        string   `json:"id"`
	ParentIDs []string `json:"parent_ids"`
	Message   string   `json:"message"`
	Author    string   `json:"author"`
	Committer string   `json:"committer"`
}

type pullRequestFileChange struct {
	Path        string `json:"path"`
	Status      string `json:"status"`
	OldObjectID string `json:"old_object_id,omitempty"`
	NewObjectID string `json:"new_object_id,omitempty"`
	OldMode     string `json:"old_mode,omitempty"`
	NewMode     string `json:"new_mode,omitempty"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
	Binary      bool   `json:"binary"`
	Patch       string `json:"patch,omitempty"`
}

func commitsBetween(repository *storage.Repository, sourceID, targetID storage.ObjectID) ([]pullRequestCommit, error) {
	return commitsBetweenRepositories(repository, repository, sourceID, targetID)
}

func commitsBetweenRepositories(sourceRepository, targetRepository *storage.Repository, sourceID, targetID storage.ObjectID) ([]pullRequestCommit, error) {
	targetReachable := map[storage.ObjectID]bool{}
	if err := walkCommits(targetRepository, targetID, targetReachable, nil); err != nil {
		return nil, err
	}
	seen := map[storage.ObjectID]bool{}
	ordered := []storage.Commit{}
	if err := walkCommits(sourceRepository, sourceID, seen, func(commit storage.Commit) {
		if !targetReachable[commit.ID] {
			ordered = append(ordered, commit)
		}
	}); err != nil {
		return nil, err
	}
	// The traversal appends parents before children, giving consumers the same
	// oldest-to-newest reading order as a normal commit log.
	result := make([]pullRequestCommit, 0, len(ordered))
	for _, commit := range ordered {
		parents := make([]string, len(commit.Parents))
		for i, parent := range commit.Parents {
			parents[i] = string(parent)
		}
		headers, message := commitDetails(commit.Content)
		result = append(result, pullRequestCommit{ID: string(commit.ID), ParentIDs: parents, Message: message, Author: headers["author"], Committer: headers["committer"]})
	}
	return result, nil
}

func walkCommits(repository *storage.Repository, id storage.ObjectID, seen map[storage.ObjectID]bool, visit func(storage.Commit)) error {
	if seen[id] {
		return nil
	}
	seen[id] = true
	commit, err := repository.ReadCommit(id)
	if err != nil {
		return err
	}
	for _, parent := range commit.Parents {
		if err := walkCommits(repository, parent, seen, visit); err != nil {
			return err
		}
	}
	if visit != nil {
		visit(commit)
	}
	return nil
}

func commitDetails(content []byte) (map[string]string, string) {
	headerBytes, messageBytes, found := bytes.Cut(content, []byte("\n\n"))
	if !found {
		return map[string]string{}, ""
	}
	headers := map[string]string{}
	for _, line := range strings.Split(string(headerBytes), "\n") {
		key, value, ok := strings.Cut(line, " ")
		if ok && (key == "author" || key == "committer") {
			headers[key] = value
		}
	}
	return headers, strings.TrimSuffix(string(messageBytes), "\n")
}

func filesBetween(repository *storage.Repository, sourceID, targetID storage.ObjectID) ([]pullRequestFileChange, error) {
	return filesBetweenRepositories(repository, repository, sourceID, targetID)
}

func filesBetweenRepositories(sourceRepository, targetRepository *storage.Repository, sourceID, targetID storage.ObjectID) ([]pullRequestFileChange, error) {
	source, err := sourceRepository.ReadCommit(sourceID)
	if err != nil {
		return nil, err
	}
	target, err := targetRepository.ReadCommit(targetID)
	if err != nil {
		return nil, err
	}
	oldFiles, err := flattenTree(targetRepository, target.Tree)
	if err != nil {
		return nil, err
	}
	newFiles, err := flattenTree(sourceRepository, source.Tree)
	if err != nil {
		return nil, err
	}
	paths := map[string]bool{}
	for path := range oldFiles {
		paths[path] = true
	}
	for path := range newFiles {
		paths[path] = true
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	changes := []pullRequestFileChange{}
	for _, path := range ordered {
		oldEntry, oldOK := oldFiles[path]
		newEntry, newOK := newFiles[path]
		if oldOK && newOK && oldEntry.ObjectID == newEntry.ObjectID && oldEntry.Mode == newEntry.Mode {
			continue
		}
		change := pullRequestFileChange{Path: path}
		if oldOK {
			change.OldObjectID, change.OldMode = string(oldEntry.ObjectID), fmt.Sprintf("%06o", oldEntry.Mode)
		}
		if newOK {
			change.NewObjectID, change.NewMode = string(newEntry.ObjectID), fmt.Sprintf("%06o", newEntry.Mode)
		}
		switch {
		case !oldOK:
			change.Status = "added"
		case !newOK:
			change.Status = "deleted"
		default:
			change.Status = "modified"
		}
		oldContent, err := fileContent(targetRepository, oldEntry, oldOK)
		if err != nil {
			return nil, err
		}
		newContent, err := fileContent(sourceRepository, newEntry, newOK)
		if err != nil {
			return nil, err
		}
		change.Binary = bytes.IndexByte(oldContent, 0) >= 0 || bytes.IndexByte(newContent, 0) >= 0 || !utf8.Valid(oldContent) || !utf8.Valid(newContent)
		if !change.Binary {
			oldLines, newLines := splitLines(oldContent), splitLines(newContent)
			change.Deletions, change.Additions = len(oldLines), len(newLines)
			change.Patch = fullFilePatch(path, oldLines, newLines, oldOK, newOK)
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func flattenTree(repository *storage.Repository, treeID storage.ObjectID) (map[string]storage.TreeEntry, error) {
	files := map[string]storage.TreeEntry{}
	var walk func(storage.ObjectID, string) error
	walk = func(id storage.ObjectID, prefix string) error {
		tree, err := repository.ReadTree(id)
		if err != nil {
			return err
		}
		for _, entry := range tree.Entries {
			path := entry.Name
			if prefix != "" {
				path = prefix + "/" + path
			}
			if entry.Type == storage.TreeObject {
				if err := walk(entry.ObjectID, path); err != nil {
					return err
				}
			} else {
				files[path] = entry
			}
		}
		return nil
	}
	return files, walk(treeID, "")
}

func fileContent(repository *storage.Repository, entry storage.TreeEntry, exists bool) ([]byte, error) {
	if !exists || entry.Type == storage.CommitObject {
		return nil, nil
	}
	object, err := repository.ReadObject(entry.ObjectID)
	if err != nil {
		return nil, err
	}
	return object.Content, nil
}

func splitLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	text := strings.TrimSuffix(string(content), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func fullFilePatch(path string, oldLines, newLines []string, oldExists, newExists bool) string {
	oldPath, newPath := "a/"+path, "b/"+path
	if !oldExists {
		oldPath = "/dev/null"
	}
	if !newExists {
		newPath = "/dev/null"
	}
	var patch strings.Builder
	fmt.Fprintf(&patch, "--- %s\n+++ %s\n@@ -1,%d +1,%d @@\n", oldPath, newPath, len(oldLines), len(newLines))
	for _, line := range oldLines {
		patch.WriteString("-" + line + "\n")
	}
	for _, line := range newLines {
		patch.WriteString("+" + line + "\n")
	}
	return patch.String()
}
