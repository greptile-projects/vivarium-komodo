package storage_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestWriteAndReadGitObjects(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	var objects storage.ObjectStore = repository

	blobContent := []byte("hello from storage\n")
	blobID := writeAndAssertObject(t, objects, repository.GitDir(), storage.BlobObject, blobContent)

	treeContent := append([]byte("100644 greeting.txt\x00"), mustDecodeObjectID(t, blobID)...)
	treeID := writeAndAssertObject(t, objects, repository.GitDir(), storage.TreeObject, treeContent)

	commitContent := []byte("tree " + string(treeID) + "\n" +
		"author Ada Lovelace <ada@example.com> 0 +0000\n" +
		"committer Ada Lovelace <ada@example.com> 0 +0000\n\n" +
		"Create greeting\n")
	commitID := writeAndAssertObject(t, objects, repository.GitDir(), storage.CommitObject, commitContent)

	tagContent := []byte("object " + string(commitID) + "\n" +
		"type commit\n" +
		"tag v1.0.0\n" +
		"tagger Ada Lovelace <ada@example.com> 0 +0000\n\n" +
		"First release\n")
	writeAndAssertObject(t, objects, repository.GitDir(), storage.TagObject, tagContent)
	command := exec.Command("git", "--git-dir="+repository.GitDir(), "fsck", "--full")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git fsck failed: %v\n%s", err, output)
	}

	info, err := repository.Inspect()
	if err != nil {
		t.Fatal(err)
	}
	if info.Empty {
		t.Fatal("repository with objects reported as empty")
	}
}

func TestObjectErrorsAndIdempotentWrites(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}

	first, err := repository.WriteObject(storage.BlobObject, []byte("same content"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.WriteObject(storage.BlobObject, []byte("same content"))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("identical writes returned %s and %s", first, second)
	}
	if _, err := repository.WriteObject("note", nil); !errors.Is(err, storage.ErrInvalidObjectType) {
		t.Fatalf("WriteObject returned %v, want ErrInvalidObjectType", err)
	}
	if _, err := repository.ReadObject("not-an-object-id"); !errors.Is(err, storage.ErrInvalidObjectID) {
		t.Fatalf("ReadObject returned %v, want ErrInvalidObjectID", err)
	}
	if _, err := repository.ReadObject("0000000000000000000000000000000000000000"); !errors.Is(err, storage.ErrObjectNotFound) {
		t.Fatalf("ReadObject returned %v, want ErrObjectNotFound", err)
	}
}

func TestListObjectsMatchesGitBatchAllObjects(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	var objects storage.ObjectStore = repository

	written := map[storage.ObjectID][]byte{}
	for _, fixture := range []struct {
		typeName storage.ObjectType
		content  []byte
	}{
		{storage.BlobObject, nil},
		{storage.BlobObject, []byte("unreachable platform object\n")},
		{storage.CommitObject, []byte("tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904\n")},
	} {
		id, err := objects.WriteObject(fixture.typeName, fixture.content)
		if err != nil {
			t.Fatal(err)
		}
		written[id] = fixture.content
	}

	gitContent := []byte("object written by stock Git\n")
	command := exec.Command("git", "--git-dir="+repository.GitDir(), "hash-object", "-w", "--stdin")
	command.Stdin = bytes.NewReader(gitContent)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git hash-object failed: %v\n%s", err, output)
	}
	gitID := storage.ObjectID(strings.TrimSpace(string(output)))
	written[gitID] = gitContent

	listed, err := objects.ListObjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != len(written) {
		t.Fatalf("ListObjects returned %d objects, want %d", len(listed), len(written))
	}
	platformMetadata := make([]string, 0, len(listed))
	for index, object := range listed {
		if index > 0 && object.ID <= listed[index-1].ID {
			t.Fatalf("objects are not ordered by ID: %s then %s", listed[index-1].ID, object.ID)
		}
		if object.Size != uint64(len(object.Content)) {
			t.Fatalf("object %s reports size %d for %d bytes", object.ID, object.Size, len(object.Content))
		}
		if !bytes.Equal(object.Content, written[object.ID]) {
			t.Fatalf("object %s content changed during enumeration", object.ID)
		}
		platformMetadata = append(platformMetadata, string(object.ID)+" "+string(object.Type)+" "+strconv.FormatUint(object.Size, 10))
	}

	command = exec.Command("git", "--git-dir="+repository.GitDir(), "cat-file", "--batch-all-objects", "--batch-check=%(objectname) %(objecttype) %(objectsize)")
	output, err = command.CombinedOutput()
	if err != nil {
		t.Fatalf("git cat-file --batch-all-objects failed: %v\n%s", err, output)
	}
	gitMetadata := strings.FieldsFunc(strings.TrimSpace(string(output)), func(r rune) bool { return r == '\n' || r == '\r' })
	sort.Strings(gitMetadata)
	if strings.Join(platformMetadata, "\n") != strings.Join(gitMetadata, "\n") {
		t.Fatalf("platform objects:\n%s\nGit objects:\n%s", strings.Join(platformMetadata, "\n"), strings.Join(gitMetadata, "\n"))
	}
}

func writeAndAssertObject(t *testing.T, objects storage.ObjectStore, gitDir string, objectType storage.ObjectType, content []byte) storage.ObjectID {
	t.Helper()
	id, err := objects.WriteObject(objectType, content)
	if err != nil {
		t.Fatal(err)
	}
	object, err := objects.ReadObject(id)
	if err != nil {
		t.Fatal(err)
	}
	if object.ID != id || object.Type != objectType || !bytes.Equal(object.Content, content) {
		t.Fatalf("object changed during round trip: %+v", object)
	}

	assertGitOutput(t, gitDir, string(objectType), "cat-file", "-t", string(id))
	assertGitOutputBytes(t, gitDir, content, "cat-file", string(objectType), string(id))
	return id
}

func mustDecodeObjectID(t *testing.T, id storage.ObjectID) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(string(id))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestTraverseTreesAndCommitAncestry(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	var graph storage.GraphStore = repository

	readmeID, err := repository.WriteObject(storage.BlobObject, []byte("root\n"))
	if err != nil {
		t.Fatal(err)
	}
	scriptID, err := repository.WriteObject(storage.BlobObject, []byte("#!/bin/sh\n"))
	if err != nil {
		t.Fatal(err)
	}
	docsContent := append([]byte("100755 run.sh\x00"), mustDecodeObjectID(t, scriptID)...)
	docsID, err := repository.WriteObject(storage.TreeObject, docsContent)
	if err != nil {
		t.Fatal(err)
	}
	rootContent := append([]byte("40000 docs\x00"), mustDecodeObjectID(t, docsID)...)
	rootContent = append(rootContent, []byte("100644 README.md\x00")...)
	rootContent = append(rootContent, mustDecodeObjectID(t, readmeID)...)
	rootID, err := repository.WriteObject(storage.TreeObject, rootContent)
	if err != nil {
		t.Fatal(err)
	}

	firstID := writeCommit(t, repository, rootID, nil, 1, "Create snapshot")
	secondID := writeCommit(t, repository, rootID, []storage.ObjectID{firstID}, 2, "Continue history")
	sideID := writeCommit(t, repository, rootID, []storage.ObjectID{firstID}, 3, "Side history")
	mergeID := writeCommit(t, repository, rootID, []storage.ObjectID{secondID, sideID}, 4, "Merge histories")
	if err := repository.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: mergeID}); err != nil {
		t.Fatal(err)
	}

	root, err := graph.ReadTree(rootID)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Entries) != 2 || root.Entries[0].Name != "docs" || root.Entries[0].Mode != 0o40000 || root.Entries[0].Type != storage.TreeObject || root.Entries[1].ObjectID != readmeID {
		t.Fatalf("unexpected root tree: %+v", root)
	}
	docs, err := graph.ReadTree(root.Entries[0].ObjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs.Entries) != 1 || docs.Entries[0].Name != "run.sh" || docs.Entries[0].Mode != 0o100755 || docs.Entries[0].Type != storage.BlobObject {
		t.Fatalf("unexpected nested tree: %+v", docs)
	}
	merge, err := graph.ReadCommit(mergeID)
	if err != nil {
		t.Fatal(err)
	}
	if merge.Tree != rootID || len(merge.Parents) != 2 || merge.Parents[0] != secondID || merge.Parents[1] != sideID {
		t.Fatalf("unexpected merge commit: %+v", merge)
	}
	firstParent, err := graph.ReadCommit(merge.Parents[0])
	if err != nil || len(firstParent.Parents) != 1 || firstParent.Parents[0] != firstID {
		t.Fatalf("could not follow first-parent ancestry: %+v, %v", firstParent, err)
	}

	assertGitOutput(t, repository.GitDir(), "tree", "cat-file", "-t", string(rootID))
	assertGitOutput(t, repository.GitDir(), "3", "rev-list", "--count", "--first-parent", "main")
	assertGitOutput(t, repository.GitDir(), "Merge histories\nSide history\nContinue history\nCreate snapshot", "log", "--format=%s", "--date-order", "main")
	assertGitOutput(t, repository.GitDir(), "100755 blob "+string(scriptID)+"\trun.sh", "cat-file", "-p", string(docsID))
}

func TestGraphObjectValidation(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	blobID, err := repository.WriteObject(storage.BlobObject, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReadTree(blobID); !errors.Is(err, storage.ErrNotTree) {
		t.Fatalf("ReadTree(blob) returned %v", err)
	}
	if _, err := repository.ReadCommit(blobID); !errors.Is(err, storage.ErrNotCommit) {
		t.Fatalf("ReadCommit(blob) returned %v", err)
	}
	badTreeID, err := repository.WriteObject(storage.TreeObject, []byte("100644 missing-object-id"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReadTree(badTreeID); !errors.Is(err, storage.ErrInvalidTree) {
		t.Fatalf("ReadTree(invalid) returned %v", err)
	}
	badCommitID, err := repository.WriteObject(storage.CommitObject, []byte("parent 0000000000000000000000000000000000000000\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReadCommit(badCommitID); !errors.Is(err, storage.ErrInvalidCommit) {
		t.Fatalf("ReadCommit(invalid) returned %v", err)
	}
}

func writeCommit(t *testing.T, repository *storage.Repository, tree storage.ObjectID, parents []storage.ObjectID, timestamp int, subject string) storage.ObjectID {
	t.Helper()
	var content strings.Builder
	fmt.Fprintf(&content, "tree %s\n", tree)
	for _, parent := range parents {
		fmt.Fprintf(&content, "parent %s\n", parent)
	}
	fmt.Fprintf(&content, "author Ada Lovelace <ada@example.com> %d +0000\n", timestamp)
	fmt.Fprintf(&content, "committer Ada Lovelace <ada@example.com> %d +0000\n\n%s\n", timestamp, subject)
	id, err := repository.WriteObject(storage.CommitObject, []byte(content.String()))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestRepositoryLifecycle(t *testing.T) {
	root := t.TempDir()
	concreteStore, err := storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	var store storage.RepositoryStore = concreteStore

	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	if repository.ID() == "" {
		t.Fatal("created repository has no ID")
	}

	info, err := repository.Inspect()
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != repository.ID() || !info.Bare || !info.Empty {
		t.Fatalf("unexpected repository info: %+v", info)
	}

	reopenedStore, err := storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := reopenedStore.Open(repository.ID())
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ID() != repository.ID() || reopened.GitDir() != repository.GitDir() {
		t.Fatalf("reopened a different repository: %#v", reopened)
	}
}

func TestCreatedRepositoryIsRecognizedByGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}

	assertGitOutput(t, repository.GitDir(), "true", "rev-parse", "--is-bare-repository")
	assertGitOutput(t, repository.GitDir(), "refs/heads/main", "symbolic-ref", "HEAD")
	command := exec.Command("git", "--git-dir="+repository.GitDir(), "fsck", "--full")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git fsck failed: %v\n%s", err, output)
	}
}

func TestRepositoryStoragePassesFullGitFsck(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	var repositoryStorage storage.RepositoryStorage = repository

	writeObject := func(objectType storage.ObjectType, content []byte) storage.ObjectID {
		t.Helper()
		id, err := repositoryStorage.WriteObject(objectType, content)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	writeCommit := func(tree storage.ObjectID, parents []storage.ObjectID, timestamp int, subject string) storage.ObjectID {
		t.Helper()
		var content strings.Builder
		fmt.Fprintf(&content, "tree %s\n", tree)
		for _, parent := range parents {
			fmt.Fprintf(&content, "parent %s\n", parent)
		}
		fmt.Fprintf(&content, "author Ada Lovelace <ada@example.com> %d +0000\n", timestamp)
		fmt.Fprintf(&content, "committer Ada Lovelace <ada@example.com> %d +0000\n\n%s\n", timestamp, subject)
		return writeObject(storage.CommitObject, []byte(content.String()))
	}

	readmeID := writeObject(storage.BlobObject, []byte("# Complete storage foundation\n"))
	scriptID := writeObject(storage.BlobObject, []byte("#!/bin/sh\necho compatible\n"))
	docsTreeContent := append([]byte("100755 verify.sh\x00"), mustDecodeObjectID(t, scriptID)...)
	docsTreeID := writeObject(storage.TreeObject, docsTreeContent)
	rootTreeContent := append([]byte("100644 README.md\x00"), mustDecodeObjectID(t, readmeID)...)
	rootTreeContent = append(rootTreeContent, []byte("40000 docs\x00")...)
	rootTreeContent = append(rootTreeContent, mustDecodeObjectID(t, docsTreeID)...)
	rootTreeID := writeObject(storage.TreeObject, rootTreeContent)

	initialID := writeCommit(rootTreeID, nil, 10, "Create project")
	mainID := writeCommit(rootTreeID, []storage.ObjectID{initialID}, 20, "Advance main")
	featureID := writeCommit(rootTreeID, []storage.ObjectID{initialID}, 30, "Build feature")
	mergeID := writeCommit(rootTreeID, []storage.ObjectID{mainID, featureID}, 40, "Merge feature")
	tagContent := []byte("object " + string(mergeID) + "\n" +
		"type commit\n" +
		"tag v1.0.0\n" +
		"tagger Ada Lovelace <ada@example.com> 50 +0000\n\n" +
		"Storage foundation complete\n")
	tagID := writeObject(storage.TagObject, tagContent)
	writeObject(storage.BlobObject, []byte("intentionally unreachable but valid\n"))

	for _, reference := range []storage.Reference{
		{Name: "refs/heads/main", ObjectID: mergeID},
		{Name: "refs/heads/feature", ObjectID: featureID},
		{Name: "refs/tags/v1.0.0", ObjectID: tagID},
		{Name: "refs/tags/latest", ObjectID: mergeID},
		{Name: "refs/aliases/stable", Target: "refs/tags/v1.0.0"},
	} {
		if err := repositoryStorage.CreateReference(reference); err != nil {
			t.Fatal(err)
		}
	}

	if tree, err := repositoryStorage.ReadTree(rootTreeID); err != nil || len(tree.Entries) != 2 {
		t.Fatalf("ReadTree returned %+v, %v", tree, err)
	}
	if commit, err := repositoryStorage.ReadCommit(mergeID); err != nil || len(commit.Parents) != 2 {
		t.Fatalf("ReadCommit returned %+v, %v", commit, err)
	}
	if objects, err := repositoryStorage.ListObjects(); err != nil || len(objects) != 10 {
		t.Fatalf("ListObjects returned %d objects, %v", len(objects), err)
	}
	if references, err := repositoryStorage.ListReferences(); err != nil || len(references) != 6 {
		t.Fatalf("ListReferences returned %d references, %v", len(references), err)
	}

	command := exec.Command("git", "--git-dir="+repository.GitDir(), "fsck", "--full")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git fsck --full failed: %v\n%s", err, output)
	}
	assertGitOutput(t, repository.GitDir(), string(mergeID), "rev-parse", "main")
	assertGitOutput(t, repository.GitDir(), string(mergeID), "rev-parse", "v1.0.0^{}")
	assertGitOutput(t, repository.GitDir(), "4", "rev-list", "--count", "--all")
}

func TestManageReferencesAndDefaultBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	var references storage.ReferenceStore = repository

	if branch, err := references.DefaultBranch(); err != nil || branch != "refs/heads/main" {
		t.Fatalf("DefaultBranch returned %q, %v", branch, err)
	}
	first, err := repository.WriteObject(storage.BlobObject, []byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.WriteObject(storage.BlobObject, []byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	main := storage.Reference{Name: "refs/heads/main", ObjectID: first}
	if err := references.CreateReference(main); err != nil {
		t.Fatal(err)
	}
	if err := references.CreateReference(main); !errors.Is(err, storage.ErrReferenceExists) {
		t.Fatalf("duplicate CreateReference returned %v", err)
	}
	main.ObjectID = second
	if err := references.UpdateReference(main); err != nil {
		t.Fatal(err)
	}
	if err := references.CreateReference(storage.Reference{Name: "refs/aliases/current", Target: "refs/heads/main"}); err != nil {
		t.Fatal(err)
	}
	if err := references.SetDefaultBranch("refs/heads/trunk"); err != nil {
		t.Fatal(err)
	}

	listed, err := references.ListReferences()
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []storage.ReferenceName{"HEAD", "refs/aliases/current", "refs/heads/main"}
	if len(listed) != len(wantNames) {
		t.Fatalf("ListReferences returned %+v", listed)
	}
	for index, want := range wantNames {
		if listed[index].Name != want {
			t.Fatalf("reference %d is %q, want %q", index, listed[index].Name, want)
		}
	}
	assertGitOutput(t, repository.GitDir(), "refs/heads/trunk", "symbolic-ref", "HEAD")
	assertGitOutput(t, repository.GitDir(), string(second), "rev-parse", "refs/aliases/current")

	if err := references.DeleteReference("refs/aliases/current"); err != nil {
		t.Fatal(err)
	}
	if _, err := references.ReadReference("refs/aliases/current"); !errors.Is(err, storage.ErrReferenceNotFound) {
		t.Fatalf("deleted ReadReference returned %v", err)
	}
	if err := references.DeleteReference("HEAD"); !errors.Is(err, storage.ErrInvalidRefName) {
		t.Fatalf("DeleteReference HEAD returned %v", err)
	}
}

func TestReferenceValidationAndErrors(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	id, err := repository.WriteObject(storage.BlobObject, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []storage.ReferenceName{"main", "refs/../HEAD", "refs/heads/bad.lock", "refs/heads/a b"} {
		err := repository.CreateReference(storage.Reference{Name: name, ObjectID: id})
		if !errors.Is(err, storage.ErrInvalidRefName) {
			t.Fatalf("CreateReference(%q) returned %v", name, err)
		}
	}
	if err := repository.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: id, Target: "refs/heads/other"}); !errors.Is(err, storage.ErrInvalidReference) {
		t.Fatalf("ambiguous reference returned %v", err)
	}
	if err := repository.UpdateReference(storage.Reference{Name: "refs/heads/missing", ObjectID: id}); !errors.Is(err, storage.ErrReferenceNotFound) {
		t.Fatalf("missing UpdateReference returned %v", err)
	}
	if err := repository.SetDefaultBranch("refs/tags/v1"); !errors.Is(err, storage.ErrInvalidRefName) {
		t.Fatalf("SetDefaultBranch tag returned %v", err)
	}
}

func TestPackedReferencesRemainManageable(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	first, err := repository.WriteObject(storage.BlobObject, []byte("packed"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.WriteObject(storage.BlobObject, []byte("loose override"))
	if err != nil {
		t.Fatal(err)
	}
	name := storage.ReferenceName("refs/tags/example")
	if err := repository.CreateReference(storage.Reference{Name: name, ObjectID: first}); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "--git-dir="+repository.GitDir(), "pack-refs", "--all")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git pack-refs failed: %v\n%s", err, output)
	}
	if reference, err := repository.ReadReference(name); err != nil || reference.ObjectID != first {
		t.Fatalf("ReadReference packed ref returned %+v, %v", reference, err)
	}
	if err := repository.UpdateReference(storage.Reference{Name: name, ObjectID: second}); err != nil {
		t.Fatal(err)
	}
	assertGitOutput(t, repository.GitDir(), string(second), "rev-parse", string(name))
	if err := repository.DeleteReference(name); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReadReference(name); !errors.Is(err, storage.ErrReferenceNotFound) {
		t.Fatalf("packed reference survived deletion: %v", err)
	}
}

func TestOpenRejectsUnknownInvalidAndMalformedRepositories(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.Open("../outside"); !errors.Is(err, storage.ErrInvalidID) {
		t.Fatalf("Open returned %v, want ErrInvalidID", err)
	}
	if _, err := store.Open("00000000-0000-4000-8000-000000000000"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Open returned %v, want ErrNotFound", err)
	}

	id := storage.ID("11111111-1111-4111-8111-111111111111")
	malformedRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(malformedRoot, string(id)), 0o750); err != nil {
		t.Fatal(err)
	}
	malformedStore, err := storage.New(malformedRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := malformedStore.Open(id); !errors.Is(err, storage.ErrInvalidRepository) {
		t.Fatalf("Open returned %v, want ErrInvalidRepository", err)
	}
}

func assertGitOutput(t *testing.T, gitDir, expected string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"--git-dir=" + gitDir}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	if actual := strings.TrimSpace(string(output)); actual != expected {
		t.Fatalf("git %s returned %q, want %q", strings.Join(args, " "), actual, expected)
	}
}

func assertGitOutputBytes(t *testing.T, gitDir string, expected []byte, args ...string) {
	t.Helper()
	commandArgs := append([]string{"--git-dir=" + gitDir}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	if !bytes.Equal(output, expected) {
		t.Fatalf("git %s returned %q, want %q", strings.Join(args, " "), output, expected)
	}
}
