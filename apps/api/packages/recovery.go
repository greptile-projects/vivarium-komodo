package packages

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
	"time"
)

type ConsumerPolicy struct {
	RepositoryID    string    `json:"repository_id"`
	BlockDeprecated bool      `json:"block_deprecated"`
	UpdatedByID     string    `json:"updated_by_id"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Exception struct {
	ID               string    `json:"id"`
	RepositoryID     string    `json:"repository_id"`
	PackageVersionID string    `json:"package_version_id"`
	Reason           string    `json:"reason"`
	ActorID          string    `json:"actor_id"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
}

type Repair struct {
	ID                   string    `json:"id"`
	RepositoryID         string    `json:"repository_id"`
	InventoryID          string    `json:"inventory_id"`
	PackageVersionID     string    `json:"package_version_id"`
	ReplacementVersionID string    `json:"replacement_version_id,omitempty"`
	ProposalID           string    `json:"proposal_id"`
	TaskID               string    `json:"task_id"`
	Priority             string    `json:"priority"`
	OwnerType            string    `json:"owner_type"`
	OwnerID              string    `json:"owner_id,omitempty"`
	CreatedByID          string    `json:"created_by_id"`
	CreatedAt            time.Time `json:"created_at"`
}

func (s *Store) PutConsumerPolicy(repositoryID, actorID string, blockDeprecated bool) (ConsumerPolicy, error) {
	if repositoryID == "" || actorID == "" {
		return ConsumerPolicy{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := ConsumerPolicy{RepositoryID: repositoryID, BlockDeprecated: blockDeprecated, UpdatedByID: actorID, UpdatedAt: s.now().UTC()}
	d := filepath.Join(s.root, "consumer-policies")
	if err := os.MkdirAll(d, 0750); err != nil {
		return ConsumerPolicy{}, err
	}
	return p, atomicJSON(filepath.Join(d, repositoryID+".json"), p)
}

func (s *Store) GetConsumerPolicy(repositoryID string) (ConsumerPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var p ConsumerPolicy
	b, err := os.ReadFile(filepath.Join(s.root, "consumer-policies", repositoryID+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return ConsumerPolicy{RepositoryID: repositoryID, BlockDeprecated: true}, nil
	}
	if err != nil || json.Unmarshal(b, &p) != nil || p.RepositoryID != repositoryID {
		return ConsumerPolicy{}, ErrInvalid
	}
	return p, nil
}

func (s *Store) CreateException(repositoryID, packageID, reason, actorID string, expiresAt time.Time) (Exception, error) {
	reason = strings.TrimSpace(reason)
	if repositoryID == "" || packageID == "" || actorID == "" || reason == "" || !expiresAt.After(s.now()) {
		return Exception{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := randomID()
	if err != nil {
		return Exception{}, err
	}
	v := Exception{ID: id, RepositoryID: repositoryID, PackageVersionID: packageID, Reason: reason, ActorID: actorID, ExpiresAt: expiresAt.UTC(), CreatedAt: s.now().UTC()}
	d := filepath.Join(s.root, "exceptions")
	if err = os.MkdirAll(d, 0750); err != nil {
		return Exception{}, err
	}
	return v, atomicJSON(filepath.Join(d, id+".json"), v)
}

func (s *Store) HasActiveException(repositoryID, packageID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, _ := os.ReadDir(filepath.Join(s.root, "exceptions"))
	now := s.now()
	for _, e := range entries {
		var v Exception
		b, err := os.ReadFile(filepath.Join(s.root, "exceptions", e.Name()))
		if err == nil && json.Unmarshal(b, &v) == nil && v.RepositoryID == repositoryID && v.PackageVersionID == packageID && v.ExpiresAt.After(now) {
			return true
		}
	}
	return false
}

func (s *Store) CreateRepair(v Repair) (Repair, error) {
	if v.RepositoryID == "" || v.InventoryID == "" || v.PackageVersionID == "" || v.ProposalID == "" || v.TaskID == "" || v.CreatedByID == "" || (v.OwnerType != "human" && v.OwnerType != "agent") {
		return Repair{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, _ := s.listRepairs(v.RepositoryID)
	for _, x := range all {
		if x.InventoryID == v.InventoryID && x.PackageVersionID == v.PackageVersionID {
			return Repair{}, ErrSafetyConflict
		}
	}
	var err error
	v.ID, err = randomID()
	if err != nil {
		return Repair{}, err
	}
	v.CreatedAt = s.now().UTC()
	d := filepath.Join(s.root, "repairs")
	if err = os.MkdirAll(d, 0750); err != nil {
		return Repair{}, err
	}
	return v, atomicJSON(filepath.Join(d, v.ID+".json"), v)
}
func (s *Store) ListRepairs(repositoryID string) ([]Repair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listRepairs(repositoryID)
}
func (s *Store) listRepairs(repositoryID string) ([]Repair, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "repairs"))
	if errors.Is(err, fs.ErrNotExist) {
		return []Repair{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Repair{}
	for _, e := range entries {
		var v Repair
		b, x := os.ReadFile(filepath.Join(s.root, "repairs", e.Name()))
		if x != nil || json.Unmarshal(b, &v) != nil {
			return nil, ErrInvalid
		}
		if v.RepositoryID == repositoryID {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func atomicJSON(name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	t, err := os.CreateTemp(filepath.Dir(name), ".writing-*")
	if err != nil {
		return err
	}
	n := t.Name()
	defer os.Remove(n)
	if err = t.Chmod(0600); err == nil {
		_, err = t.Write(append(b, '\n'))
	}
	if e := t.Close(); err == nil {
		err = e
	}
	if err == nil {
		err = os.Rename(n, name)
	}
	return err
}
