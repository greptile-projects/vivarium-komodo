// Package storage owns the on-disk boundary for Git repositories.
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrNotFound          = errors.New("repository not found")
	ErrInvalidID         = errors.New("invalid repository ID")
	ErrInvalidRepository = errors.New("invalid repository")
)

// ID is the stable, opaque identity of a repository.
type ID string

// Info describes repository state without exposing its implementation details.
type Info struct {
	ID    ID
	Bare  bool
	Empty bool
}

// RepositoryStore is the lifecycle boundary implemented by Store.
type RepositoryStore interface {
	Create() (*Repository, error)
	Open(ID) (*Repository, error)
}

// ObjectStore persists and retrieves immutable Git objects.
type ObjectStore interface {
	WriteObject(ObjectType, []byte) (ObjectID, error)
	ReadObject(ObjectID) (Object, error)
}

// Store creates and reopens repositories beneath one storage root.
type Store struct {
	root string
}

// Repository is a handle to a repository managed by a Store.
type Repository struct {
	id     ID
	gitDir string
}

// New creates a store rooted at root, or opens it if it already exists.
func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("storage root is required")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve storage root: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o750); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}

	return &Store{root: absRoot}, nil
}

// Create initializes and atomically publishes an empty bare Git repository.
func (s *Store) Create() (*Repository, error) {
	id, err := newID()
	if err != nil {
		return nil, fmt.Errorf("generate repository ID: %w", err)
	}

	staging, err := os.MkdirTemp(s.root, ".creating-")
	if err != nil {
		return nil, fmt.Errorf("stage repository: %w", err)
	}
	defer os.RemoveAll(staging)

	if err := initializeBareRepository(staging); err != nil {
		return nil, err
	}

	gitDir := filepath.Join(s.root, string(id))
	if err := os.Rename(staging, gitDir); err != nil {
		return nil, fmt.Errorf("publish repository: %w", err)
	}

	return &Repository{id: id, gitDir: gitDir}, nil
}

// Open returns a handle to an existing, valid repository.
func (s *Store) Open(id ID) (*Repository, error) {
	if !validID(id) {
		return nil, ErrInvalidID
	}

	repository := &Repository{id: id, gitDir: filepath.Join(s.root, string(id))}
	if err := repository.validate(); err != nil {
		return nil, err
	}
	return repository, nil
}

// ID returns the repository's stable identity.
func (r *Repository) ID() ID {
	return r.id
}

// GitDir returns the bare repository directory for Git storage operations.
func (r *Repository) GitDir() string {
	return r.gitDir
}

// Inspect validates the repository and reports its current high-level state.
func (r *Repository) Inspect() (Info, error) {
	if err := r.validate(); err != nil {
		return Info{}, err
	}

	empty, err := isEmpty(r.gitDir)
	if err != nil {
		return Info{}, fmt.Errorf("inspect repository: %w", err)
	}
	return Info{ID: r.id, Bare: true, Empty: empty}, nil
}

func newID() (ID, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Use UUID-shaped IDs while keeping identity generation dependency-free.
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes)
	return ID(encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]), nil
}

func validID(id ID) bool {
	value := string(id)
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil && value == strings.ToLower(value)
}

func initializeBareRepository(gitDir string) error {
	directories := []string{
		filepath.Join(gitDir, "objects", "info"),
		filepath.Join(gitDir, "objects", "pack"),
		filepath.Join(gitDir, "refs", "heads"),
		filepath.Join(gitDir, "refs", "tags"),
	}
	for _, directory := range directories {
		if err := os.MkdirAll(directory, 0o750); err != nil {
			return fmt.Errorf("initialize repository: %w", err)
		}
	}

	files := map[string]string{
		"HEAD":   "ref: refs/heads/main\n",
		"config": "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = true\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(gitDir, name), []byte(contents), 0o640); err != nil {
			return fmt.Errorf("initialize repository: %w", err)
		}
	}
	return nil
}

func (r *Repository) validate() error {
	info, err := os.Lstat(r.gitDir)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return ErrInvalidRepository
	}

	head, err := os.ReadFile(filepath.Join(r.gitDir, "HEAD"))
	if err != nil || !validHEAD(strings.TrimSpace(string(head))) {
		return ErrInvalidRepository
	}
	config, err := os.ReadFile(filepath.Join(r.gitDir, "config"))
	if err != nil || !isBareConfig(string(config)) {
		return ErrInvalidRepository
	}
	for _, directory := range []string{"objects", "refs"} {
		info, err := os.Stat(filepath.Join(r.gitDir, directory))
		if err != nil || !info.IsDir() {
			return ErrInvalidRepository
		}
	}
	return nil
}

func validHEAD(head string) bool {
	if strings.HasPrefix(head, "ref: refs/") && !strings.Contains(head, "..") {
		return true
	}
	if len(head) != 40 && len(head) != 64 {
		return false
	}
	_, err := hex.DecodeString(head)
	return err == nil
}

func isBareConfig(config string) bool {
	section := ""
	for _, rawLine := range strings.Split(config, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		if section != "core" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if found && strings.EqualFold(strings.TrimSpace(key), "bare") && strings.EqualFold(strings.TrimSpace(value), "true") {
			return true
		}
	}
	return false
}

func isEmpty(gitDir string) (bool, error) {
	for _, root := range []string{filepath.Join(gitDir, "refs"), filepath.Join(gitDir, "objects")} {
		empty := true
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				empty = false
			}
			return nil
		})
		if err != nil {
			return false, err
		}
		if !empty {
			return false, nil
		}
	}

	packedRefs, err := os.ReadFile(filepath.Join(gitDir, "packed-refs"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	for _, line := range strings.Split(string(packedRefs), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "^") {
			return false, nil
		}
	}
	return true, nil
}
