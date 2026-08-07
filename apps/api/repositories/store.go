// Package repositories owns the application repository catalog and connects
// durable repository identity to its owning user.
package repositories

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

var ErrNotFound = errors.New("owned repository not found")

type Repository struct {
	ID        storage.ID `json:"id"`
	OwnerID   string     `json:"owner_id"`
	Empty     bool       `json:"empty"`
	CreatedAt time.Time  `json:"created_at"`
}

type repositoryStorage interface {
	Create() (*storage.Repository, error)
	Open(storage.ID) (*storage.Repository, error)
	Delete(storage.ID) error
}

type Store struct {
	root    string
	storage repositoryStorage
	mu      sync.Mutex
	now     func() time.Time
}

func New(root string, repositoryStorage repositoryStorage) (*Store, error) {
	if root == "" || repositoryStorage == nil {
		return nil, errors.New("catalog root and repository storage are required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve repository catalog root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("create repository catalog: %w", err)
	}
	return &Store{root: abs, storage: repositoryStorage, now: time.Now}, nil
}

func (s *Store) Create(ownerID string) (Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	repository, err := s.storage.Create()
	if err != nil {
		return Repository{}, err
	}
	item := Repository{ID: repository.ID(), OwnerID: ownerID, Empty: true, CreatedAt: s.now().UTC()}
	if err := s.write(item); err != nil {
		_ = s.storage.Delete(repository.ID())
		return Repository{}, err
	}
	return item, nil
}

func (s *Store) Get(ownerID string, id storage.ID) (Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	opened, err := s.storage.Open(id)
	if errors.Is(err, storage.ErrInvalidID) || errors.Is(err, storage.ErrNotFound) {
		return Repository{}, ErrNotFound
	}
	if err != nil {
		return Repository{}, err
	}
	item, err := s.read(id)
	if err != nil || item.OwnerID != ownerID {
		return Repository{}, ErrNotFound
	}
	info, err := opened.Inspect()
	if err != nil {
		return Repository{}, err
	}
	item.Empty = info.Empty
	return item, nil
}

func (s *Store) List(ownerID string) ([]Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	items := []Repository{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		item, err := s.read(storage.ID(entry.Name()[:len(entry.Name())-5]))
		if err != nil {
			return nil, err
		}
		if item.OwnerID != ownerID {
			continue
		}
		opened, err := s.storage.Open(item.ID)
		if err != nil {
			return nil, err
		}
		info, err := opened.Inspect()
		if err != nil {
			return nil, err
		}
		item.Empty = info.Empty
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *Store) Delete(ownerID string, id storage.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.storage.Open(id); errors.Is(err, storage.ErrInvalidID) || errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	item, err := s.read(id)
	if err != nil || item.OwnerID != ownerID {
		return ErrNotFound
	}
	if err := s.storage.Delete(id); err != nil {
		return err
	}
	if err := os.Remove(s.path(id)); err != nil {
		return fmt.Errorf("remove repository catalog entry: %w", err)
	}
	return nil
}

func (s *Store) path(id storage.ID) string { return filepath.Join(s.root, string(id)+".json") }

func (s *Store) read(id storage.ID) (Repository, error) {
	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, fs.ErrNotExist) {
		return Repository{}, ErrNotFound
	}
	if err != nil {
		return Repository{}, err
	}
	var item Repository
	if json.Unmarshal(data, &item) != nil || item.ID != id || item.OwnerID == "" || item.CreatedAt.IsZero() {
		return Repository{}, errors.New("invalid repository catalog entry")
	}
	return item, nil
}

func (s *Store) write(item Repository) error {
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(s.root, ".repository-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o640); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.path(item.ID))
}
