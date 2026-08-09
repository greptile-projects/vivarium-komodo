// Package relationships owns versioned interface publications and consumer declarations.
package relationships

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("relationship not found")
	ErrInvalid  = errors.New("invalid relationship")
	ErrConflict = errors.New("relationship already exists")
)

type Interface struct {
	ID            string    `json:"id"`
	RepositoryID  string    `json:"repository_id"`
	Name          string    `json:"name"`
	Version       string    `json:"version"`
	CommitID      string    `json:"commit_id"`
	ReleaseID     string    `json:"release_id"`
	SchemaPath    string    `json:"schema_path,omitempty"`
	PublishedByID string    `json:"published_by_id"`
	PublishedAt   time.Time `json:"published_at"`
}

type Dependency struct {
	ID                   string    `json:"id"`
	RepositoryID         string    `json:"repository_id"`
	CommitID             string    `json:"commit_id"`
	ReleaseID            string    `json:"release_id,omitempty"`
	ProviderRepositoryID string    `json:"provider_repository_id"`
	InterfaceName        string    `json:"interface_name"`
	Constraint           string    `json:"constraint"`
	DeclaredByID         string    `json:"declared_by_id"`
	DeclaredAt           time.Time `json:"declared_at"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrInvalid
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(abs, 0750); err != nil {
		return nil, err
	}
	return &Store{root: abs, now: time.Now}, nil
}

func (s *Store) Publish(v Interface) (Interface, error) {
	v.Name = strings.TrimSpace(v.Name)
	v.Version = strings.TrimSpace(v.Version)
	v.SchemaPath = strings.TrimSpace(v.SchemaPath)
	if v.RepositoryID == "" || v.Name == "" || len(v.Name) > 100 || !validVersion(v.Version) || v.CommitID == "" || v.ReleaseID == "" || v.PublishedByID == "" || len(v.SchemaPath) > 500 {
		return Interface{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.listInterfaces()
	if err != nil {
		return Interface{}, err
	}
	for _, item := range items {
		if item.RepositoryID == v.RepositoryID && strings.EqualFold(item.Name, v.Name) && item.Version == v.Version {
			return Interface{}, ErrConflict
		}
	}
	v.ID, err = newID()
	if err != nil {
		return Interface{}, err
	}
	v.PublishedAt = s.now().UTC()
	return v, s.write("interfaces", v.ID, v)
}

func (s *Store) Declare(v Dependency) (Dependency, error) {
	v.InterfaceName = strings.TrimSpace(v.InterfaceName)
	v.Constraint = strings.TrimSpace(v.Constraint)
	if v.RepositoryID == "" || v.CommitID == "" || v.ProviderRepositoryID == "" || v.InterfaceName == "" || len(v.InterfaceName) > 100 || !validConstraint(v.Constraint) || v.DeclaredByID == "" {
		return Dependency{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v.ID, _ = newID()
	if v.ID == "" {
		return Dependency{}, errors.New("generate id")
	}
	v.DeclaredAt = s.now().UTC()
	return v, s.write("dependencies", v.ID, v)
}

func (s *Store) Interfaces() ([]Interface, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listInterfaces()
}
func (s *Store) Dependencies() ([]Dependency, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listDependencies()
}

func (s *Store) listInterfaces() ([]Interface, error) {
	var out []Interface
	err := s.list("interfaces", func(b []byte) error {
		var v Interface
		if json.Unmarshal(b, &v) != nil {
			return ErrNotFound
		}
		out = append(out, v)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].PublishedAt.After(out[j].PublishedAt) })
	return out, err
}
func (s *Store) listDependencies() ([]Dependency, error) {
	var out []Dependency
	err := s.list("dependencies", func(b []byte) error {
		var v Dependency
		if json.Unmarshal(b, &v) != nil {
			return ErrNotFound
		}
		out = append(out, v)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].DeclaredAt.After(out[j].DeclaredAt) })
	return out, err
}
func (s *Store) list(kind string, add func([]byte) error) error {
	entries, err := os.ReadDir(filepath.Join(s.root, kind))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, er := os.ReadFile(filepath.Join(s.root, kind, e.Name()))
		if er != nil {
			return er
		}
		if er = add(b); er != nil {
			return er
		}
	}
	return nil
}
func (s *Store) write(kind, id string, v any) error {
	dir := filepath.Join(s.root, kind)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".relationship-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	_ = tmp.Chmod(0600)
	if _, err = tmp.Write(append(b, '\n')); err == nil {
		err = tmp.Sync()
	}
	if e := tmp.Close(); err == nil {
		err = e
	}
	if err == nil {
		err = os.Rename(name, filepath.Join(dir, id+".json"))
	}
	return err
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func validVersion(v string) bool {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
func validConstraint(v string) bool {
	v = strings.TrimSpace(v)
	if v == "*" {
		return true
	}
	if strings.HasPrefix(v, "^") || strings.HasPrefix(v, "~") || strings.HasPrefix(v, ">=") {
		v = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(v, "^"), "~"), ">=")
	}
	return validVersion(v)
}

// Satisfies supports the intentionally small public constraint vocabulary: exact, ^, ~, >= and *.
func Satisfies(version, constraint string) bool {
	if constraint == "*" {
		return true
	}
	v, ok := numbers(version)
	if !ok {
		return false
	}
	prefix := ""
	for _, p := range []string{">=", "^", "~"} {
		if strings.HasPrefix(constraint, p) {
			prefix = p
			constraint = strings.TrimPrefix(constraint, p)
			break
		}
	}
	c, ok := numbers(constraint)
	if !ok {
		return false
	}
	cmp := compare(v, c)
	switch prefix {
	case ">=":
		return cmp >= 0
	case "^":
		return cmp >= 0 && v[0] == c[0]
	case "~":
		return cmp >= 0 && v[0] == c[0] && v[1] == c[1]
	default:
		return cmp == 0
	}
}
func numbers(v string) ([3]int, bool) {
	var n [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return n, false
	}
	for i, p := range parts {
		for _, r := range p {
			n[i] = n[i]*10 + int(r-'0')
		}
	}
	return n, true
}
func compare(a, b [3]int) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
