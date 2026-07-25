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
	"regexp"
	"strings"
)

const (
	idBytes       = 16
	defaultBranch = "main"
)

var (
	// ErrNotFound is returned when a repository ID is not present in the store.
	ErrNotFound = errors.New("repository not found")
	// ErrInvalidRepository is returned when a path does not contain the bare
	// repository structure managed by this package.
	ErrInvalidRepository = errors.New("invalid repository")

	validID = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// Store manages bare Git repositories below a single filesystem root.
type Store struct {
	root string
}

// Repository is an opened repository. Its fields are deliberately private so
// callers cannot manufacture a repository without Store's validation.
type Repository struct {
	id   string
	path string
}

// Info describes the stable properties of an opened repository.
type Info struct {
	ID            string
	Path          string
	Bare          bool
	DefaultBranch string
}

// New returns a repository store rooted at root.
func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("storage root is required")
	}

	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve storage root: %w", err)
	}
	return &Store{root: absolute}, nil
}

// Create initializes and opens a new empty bare Git repository.
func (s *Store) Create() (*Repository, error) {
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}

	for {
		id, err := newID()
		if err != nil {
			return nil, err
		}

		path := s.repositoryPath(id)
		err = createBareRepository(path)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("create repository: %w", err)
		}
		return &Repository{id: id, path: path}, nil
	}
}

// Open reopens an existing repository by its stable ID.
func (s *Store) Open(id string) (*Repository, error) {
	if !validID.MatchString(id) {
		return nil, ErrNotFound
	}

	repository := &Repository{id: id, path: s.repositoryPath(id)}
	if _, err := validate(repository.path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return repository, nil
}

// ID returns the repository's stable storage identifier.
func (r *Repository) ID() string {
	return r.id
}

// Inspect returns repository metadata after checking that its on-disk
// structure is still valid.
func (r *Repository) Inspect() (Info, error) {
	branch, err := validate(r.path)
	if err != nil {
		return Info{}, err
	}
	return Info{
		ID:            r.id,
		Path:          r.path,
		Bare:          true,
		DefaultBranch: branch,
	}, nil
}

func (s *Store) repositoryPath(id string) string {
	return filepath.Join(s.root, id+".git")
}

func newID() (string, error) {
	value := make([]byte, idBytes)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate repository ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func createBareRepository(path string) (err error) {
	parent := filepath.Dir(path)
	if _, err := os.Stat(path); err == nil {
		return fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	temporary, err := os.MkdirTemp(parent, ".creating-")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(temporary)
	}()

	directories := []string{
		"branches",
		"hooks",
		"info",
		"objects/info",
		"objects/pack",
		"refs/heads",
		"refs/tags",
	}
	for _, directory := range directories {
		if err = os.MkdirAll(filepath.Join(temporary, directory), 0o750); err != nil {
			return err
		}
	}

	files := map[string]string{
		"HEAD":         "ref: refs/heads/" + defaultBranch + "\n",
		"config":       "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = true\n",
		"description":  "Unnamed repository; edit this file 'description' to name the repository.\n",
		"info/exclude": "# git ls-files --others --exclude-from=.git/info/exclude\n",
	}
	for name, contents := range files {
		if err = os.WriteFile(filepath.Join(temporary, name), []byte(contents), 0o640); err != nil {
			return err
		}
	}
	if err = os.Rename(temporary, path); err != nil {
		if _, statErr := os.Stat(path); statErr == nil {
			return fs.ErrExist
		}
		return err
	}
	return err
}

func validate(path string) (string, error) {
	if info, err := os.Stat(path); err != nil {
		return "", err
	} else if !info.IsDir() {
		return "", invalid(path, nil)
	}

	requiredDirectories := []string{"objects", "objects/info", "objects/pack", "refs", "refs/heads", "refs/tags"}
	for _, name := range requiredDirectories {
		info, err := os.Stat(filepath.Join(path, name))
		if err != nil {
			return "", invalid(path, err)
		}
		if !info.IsDir() {
			return "", invalid(path, nil)
		}
	}

	head, err := os.ReadFile(filepath.Join(path, "HEAD"))
	if err != nil {
		return "", invalid(path, err)
	}
	const headPrefix = "ref: refs/heads/"
	headValue := strings.TrimSpace(string(head))
	if !strings.HasPrefix(headValue, headPrefix) || len(headValue) == len(headPrefix) {
		return "", invalid(path, nil)
	}
	branch := strings.TrimPrefix(headValue, headPrefix)

	config, err := os.ReadFile(filepath.Join(path, "config"))
	if err != nil {
		return "", invalid(path, err)
	}
	configValue := string(config)
	if !strings.Contains(configValue, "repositoryformatversion = 0") ||
		!strings.Contains(configValue, "bare = true") {
		return "", invalid(path, nil)
	}
	return branch, nil
}

func invalid(path string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%w: %s", ErrInvalidRepository, path)
	}
	return fmt.Errorf("%w: %s: %v", ErrInvalidRepository, path, cause)
}
