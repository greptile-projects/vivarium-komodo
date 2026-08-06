package storage

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrInvalidTree   = errors.New("invalid tree")
	ErrInvalidCommit = errors.New("invalid commit")
	ErrNotTree       = errors.New("object is not a tree")
	ErrNotCommit     = errors.New("object is not a commit")
)

// Tree is one repository directory snapshot.
type Tree struct {
	ID      ObjectID
	Entries []TreeEntry
}

// TreeEntry names an object contained directly in a tree. Mode is the Git
// file mode represented as an octal number; Type is the type implied by it.
type TreeEntry struct {
	Name     string
	Mode     uint32
	Type     ObjectType
	ObjectID ObjectID
}

// Commit identifies a snapshot and its immediate ancestors. Content retains
// the exact commit payload for callers that also need attribution or message
// headers not interpreted by the graph boundary.
type Commit struct {
	ID      ObjectID
	Tree    ObjectID
	Parents []ObjectID
	Content []byte
}

// ReadTree parses a tree object's binary entries in their stored order.
// Subtrees can be traversed by passing entries of Type TreeObject back to
// ReadTree.
func (r *Repository) ReadTree(id ObjectID) (Tree, error) {
	object, err := r.ReadObject(id)
	if err != nil {
		return Tree{}, err
	}
	if object.Type != TreeObject {
		return Tree{}, ErrNotTree
	}

	entries := make([]TreeEntry, 0)
	content := object.Content
	for len(content) > 0 {
		space := bytes.IndexByte(content, ' ')
		if space <= 0 {
			return Tree{}, ErrInvalidTree
		}
		nul := bytes.IndexByte(content[space+1:], 0)
		if nul < 0 {
			return Tree{}, ErrInvalidTree
		}
		nul += space + 1
		if nul == space+1 || len(content) < nul+1+20 {
			return Tree{}, ErrInvalidTree
		}

		mode, parseErr := strconv.ParseUint(string(content[:space]), 8, 32)
		entryType, typeErr := treeEntryType(uint32(mode))
		name := string(content[space+1 : nul])
		if parseErr != nil || typeErr != nil || strings.Contains(name, "/") {
			return Tree{}, ErrInvalidTree
		}
		entryID := ObjectID(fmt.Sprintf("%x", content[nul+1:nul+21]))
		entries = append(entries, TreeEntry{Name: name, Mode: uint32(mode), Type: entryType, ObjectID: entryID})
		content = content[nul+21:]
	}
	return Tree{ID: id, Entries: entries}, nil
}

// ReadCommit parses the snapshot and ordered parent links from a commit.
// Each parent can be followed by passing its ID back to ReadCommit.
func (r *Repository) ReadCommit(id ObjectID) (Commit, error) {
	object, err := r.ReadObject(id)
	if err != nil {
		return Commit{}, err
	}
	if object.Type != CommitObject {
		return Commit{}, ErrNotCommit
	}

	headers := object.Content
	if end := bytes.Index(headers, []byte("\n\n")); end >= 0 {
		headers = headers[:end]
	}
	lines := bytes.Split(headers, []byte{'\n'})
	if len(lines) == 0 || !bytes.HasPrefix(lines[0], []byte("tree ")) {
		return Commit{}, ErrInvalidCommit
	}
	tree := ObjectID(string(bytes.TrimPrefix(lines[0], []byte("tree "))))
	if !validObjectID(tree) {
		return Commit{}, ErrInvalidCommit
	}
	parents := make([]ObjectID, 0)
	for _, line := range lines[1:] {
		if !bytes.HasPrefix(line, []byte("parent ")) {
			continue
		}
		parent := ObjectID(string(bytes.TrimPrefix(line, []byte("parent "))))
		if !validObjectID(parent) {
			return Commit{}, ErrInvalidCommit
		}
		parents = append(parents, parent)
	}
	content := append([]byte(nil), object.Content...)
	return Commit{ID: id, Tree: tree, Parents: parents, Content: content}, nil
}

func treeEntryType(mode uint32) (ObjectType, error) {
	switch mode {
	case 0o40000:
		return TreeObject, nil
	case 0o160000:
		return CommitObject, nil
	case 0o100644, 0o100755, 0o120000:
		return BlobObject, nil
	default:
		return "", ErrInvalidTree
	}
}
