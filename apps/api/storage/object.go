package storage

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var (
	ErrObjectNotFound    = errors.New("object not found")
	ErrInvalidObjectID   = errors.New("invalid object ID")
	ErrInvalidObjectType = errors.New("invalid object type")
	ErrInvalidObject     = errors.New("invalid object")
)

// ObjectID is the SHA-1 identity of a Git object.
type ObjectID string

// ObjectType identifies one of Git's four repository object types.
type ObjectType string

const (
	BlobObject   ObjectType = "blob"
	TreeObject   ObjectType = "tree"
	CommitObject ObjectType = "commit"
	TagObject    ObjectType = "tag"
)

// Object is an immutable Git object with its identity, type, and exact content.
type Object struct {
	ID      ObjectID
	Type    ObjectType
	Size    uint64
	Content []byte
}

// WriteObject stores content in Git's loose-object format and returns its
// content-derived identity. Repeated writes of the same object are idempotent.
func (r *Repository) WriteObject(objectType ObjectType, content []byte) (ObjectID, error) {
	if !validObjectType(objectType) {
		return "", ErrInvalidObjectType
	}
	if err := r.validate(); err != nil {
		return "", err
	}

	canonical := make([]byte, 0, len(content)+32)
	canonical = fmt.Appendf(canonical, "%s %d%c", objectType, len(content), byte(0))
	canonical = append(canonical, content...)
	digest := sha1.Sum(canonical)
	id := ObjectID(hex.EncodeToString(digest[:]))
	path := r.objectPath(id)

	if _, err := os.Stat(path); err == nil {
		if _, err := r.ReadObject(id); err != nil {
			return "", fmt.Errorf("verify existing object %s: %w", id, err)
		}
		return id, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("inspect object %s: %w", id, err)
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return "", fmt.Errorf("create object directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".writing-")
	if err != nil {
		return "", fmt.Errorf("stage object: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	writer := zlib.NewWriter(temporary)
	if _, err = writer.Write(canonical); err == nil {
		err = writer.Close()
	}
	if err == nil {
		err = temporary.Chmod(0o440)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", fmt.Errorf("write object: %w", err)
	}

	// A hard link publishes without overwriting an object another writer won.
	if err := os.Link(temporaryPath, path); err != nil {
		if _, statErr := os.Stat(path); statErr != nil {
			return "", fmt.Errorf("publish object: %w", err)
		}
	}
	if _, err := r.ReadObject(id); err != nil {
		return "", fmt.Errorf("verify published object %s: %w", id, err)
	}
	return id, nil
}

// ReadObject retrieves an object and verifies that its canonical bytes hash to
// the requested identity.
func (r *Repository) ReadObject(id ObjectID) (Object, error) {
	if !validObjectID(id) {
		return Object{}, ErrInvalidObjectID
	}
	if err := r.validate(); err != nil {
		return Object{}, err
	}

	file, err := os.Open(r.objectPath(id))
	if errors.Is(err, fs.ErrNotExist) {
		return Object{}, ErrObjectNotFound
	}
	if err != nil {
		return Object{}, fmt.Errorf("open object: %w", err)
	}
	defer file.Close()

	reader, err := zlib.NewReader(file)
	if err != nil {
		return Object{}, ErrInvalidObject
	}
	canonical, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		return Object{}, ErrInvalidObject
	}

	nul := bytes.IndexByte(canonical, 0)
	if nul < 0 {
		return Object{}, ErrInvalidObject
	}
	header := string(canonical[:nul])
	typeName, sizeText, found := strings.Cut(header, " ")
	objectType := ObjectType(typeName)
	size, sizeErr := strconv.ParseUint(sizeText, 10, 64)
	content := canonical[nul+1:]
	if !found || !validObjectType(objectType) || sizeErr != nil || size != uint64(len(content)) {
		return Object{}, ErrInvalidObject
	}
	digest := sha1.Sum(canonical)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), string(id)) {
		return Object{}, ErrInvalidObject
	}

	return Object{ID: id, Type: objectType, Size: size, Content: content}, nil
}

// ListObjects returns every loose object in the repository, ordered by object
// ID. Each object is fully read and verified before it is returned.
func (r *Repository) ListObjects() ([]Object, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}

	objectsDirectory := filepath.Join(r.gitDir, "objects")
	directories, err := os.ReadDir(objectsDirectory)
	if err != nil {
		return nil, fmt.Errorf("list object directories: %w", err)
	}

	var ids []ObjectID
	for _, directory := range directories {
		if !directory.IsDir() || !validHexComponent(directory.Name(), 2) {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(objectsDirectory, directory.Name()))
		if err != nil {
			return nil, fmt.Errorf("list object directory %s: %w", directory.Name(), err)
		}
		for _, entry := range entries {
			if entry.Type().IsRegular() && validHexComponent(entry.Name(), 38) {
				ids = append(ids, ObjectID(directory.Name()+entry.Name()))
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	objects := make([]Object, 0, len(ids))
	for _, id := range ids {
		object, err := r.ReadObject(id)
		if err != nil {
			return nil, fmt.Errorf("read listed object %s: %w", id, err)
		}
		objects = append(objects, object)
	}
	return objects, nil
}

func (r *Repository) objectPath(id ObjectID) string {
	value := string(id)
	return filepath.Join(r.gitDir, "objects", value[:2], value[2:])
}

func validObjectType(objectType ObjectType) bool {
	switch objectType {
	case BlobObject, TreeObject, CommitObject, TagObject:
		return true
	default:
		return false
	}
}

func validObjectID(id ObjectID) bool {
	value := string(id)
	if len(value) != sha1.Size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validHexComponent(value string, length int) bool {
	if len(value) != length || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
