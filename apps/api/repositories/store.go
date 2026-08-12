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
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

var ErrNotFound = errors.New("owned repository not found")
var (
	ErrInvalidRepository = errors.New("invalid repository metadata")
	ErrNameTaken         = errors.New("repository name is already taken")
	ErrNotFork           = errors.New("repository is not a fork")
	ErrForkConflict      = errors.New("fork branch has diverged from upstream")
	ErrUpstreamBranch    = errors.New("upstream branch not found")
)

var namePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,98}[a-z0-9])?$`)

type Visibility string

const (
	Private Visibility = "private"
	Public  Visibility = "public"
)

type Repository struct {
	ID                storage.ID                        `json:"id"`
	OwnerID           string                            `json:"owner_id"`
	OrganizationID    string                            `json:"organization_id,omitempty"`
	Name              string                            `json:"name"`
	Description       string                            `json:"description"`
	Visibility        Visibility                        `json:"visibility"`
	Empty             bool                              `json:"empty"`
	CreatedAt         time.Time                         `json:"created_at"`
	UpdatedAt         time.Time                         `json:"updated_at"`
	CollaboratorIDs   []string                          `json:"collaborator_ids,omitempty"`
	RequiredChecks    map[string][]string               `json:"required_checks,omitempty"`
	PreviewAcceptance []PreviewAcceptanceRequirement    `json:"preview_acceptance_requirements,omitempty"`
	IntegrationQueue  map[string]IntegrationQueuePolicy `json:"integration_queue,omitempty"`
	UpstreamID        storage.ID                        `json:"upstream_repository_id,omitempty"`
}

type PreviewAcceptanceScenario struct {
	ID            string   `json:"id"`
	Description   string   `json:"description"`
	RequiredRoles []string `json:"required_roles"`
}
type PreviewAcceptanceRequirement struct {
	ID             string                      `json:"id"`
	TargetBranches []string                    `json:"target_branches"`
	Paths          []string                    `json:"paths,omitempty"`
	RiskClasses    []string                    `json:"risk_classes,omitempty"`
	Scenarios      []PreviewAcceptanceScenario `json:"scenarios"`
}

func (s *Store) SetPreviewAcceptance(ownerID string, id storage.ID, requirements []PreviewAcceptanceRequirement) (Repository, error) {
	if len(requirements) > 20 {
		return Repository{}, ErrInvalidRepository
	}
	seen := map[string]bool{}
	roles := map[string]bool{"owner": true, "repository_participant": true, "test": true, "feedback": true}
	for i := range requirements {
		r := &requirements[i]
		r.ID = strings.TrimSpace(r.ID)
		if r.ID == "" || len(r.ID) > 100 || seen[r.ID] || len(r.TargetBranches) == 0 || len(r.Scenarios) == 0 || len(r.Scenarios) > 20 || len(r.Paths) > 50 || len(r.RiskClasses) > 20 {
			return Repository{}, ErrInvalidRepository
		}
		seen[r.ID] = true
		values := append(append(append([]string{}, r.TargetBranches...), r.Paths...), r.RiskClasses...)
		for _, v := range values {
			if strings.TrimSpace(v) == "" || len(v) > 300 {
				return Repository{}, ErrInvalidRepository
			}
		}
		scenarios := map[string]bool{}
		for j := range r.Scenarios {
			sc := &r.Scenarios[j]
			sc.ID = strings.TrimSpace(sc.ID)
			sc.Description = strings.TrimSpace(sc.Description)
			if sc.ID == "" || sc.Description == "" || len(sc.Description) > 1000 || scenarios[sc.ID] || len(sc.RequiredRoles) == 0 {
				return Repository{}, ErrInvalidRepository
			}
			scenarios[sc.ID] = true
			for _, role := range sc.RequiredRoles {
				if !roles[role] {
					return Repository{}, ErrInvalidRepository
				}
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err != nil || item.OwnerID != ownerID {
		return Repository{}, ErrNotFound
	}
	item.PreviewAcceptance = requirements
	item.UpdatedAt = s.now().UTC()
	if err = s.write(item); err != nil {
		return Repository{}, err
	}
	return item, nil
}

// TransferOwner changes only the catalog's ownership pointer. The storage ID,
// Git objects, refs, timestamps, and every repository-linked resource remain
// untouched. Acceptance is enforced by the organization boundary before this
// method is called.
func (s *Store) TransferOwner(id storage.ID, fromKind, fromID, toKind, toID, administratorID string) (Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err != nil {
		return Repository{}, ErrNotFound
	}
	if (fromKind == "user" && (item.OrganizationID != "" || item.OwnerID != fromID)) || (fromKind == "organization" && item.OrganizationID != fromID) {
		return Repository{}, ErrNotFound
	}
	if toKind == "organization" {
		item.OrganizationID = toID
		item.OwnerID = administratorID
	} else if toKind == "user" {
		item.OrganizationID = ""
		item.OwnerID = toID
	} else {
		return Repository{}, ErrInvalidRepository
	}
	if err := s.ensureNameAvailable(item.OwnerID, item.Name, item.ID); err != nil {
		return Repository{}, err
	}
	item.UpdatedAt = s.now().UTC()
	if err := s.write(item); err != nil {
		return Repository{}, err
	}
	return item, nil
}

func (s *Store) ListOrganization(id string) ([]Repository, error) {
	return s.list(func(item Repository) bool { return item.OrganizationID == id })
}

type IntegrationQueuePolicy struct {
	Enabled         bool   `json:"enabled"`
	Concurrency     int    `json:"concurrency"`
	FailureBehavior string `json:"failure_behavior"`
}

func (s *Store) SetIntegrationQueue(ownerID string, id storage.ID, branch string, policy IntegrationQueuePolicy) (Repository, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") || strings.Contains(branch, "..") || strings.ContainsAny(branch, " ~^:?*[\\") || policy.Concurrency < 1 || policy.Concurrency > 10 || (policy.FailureBehavior != "pause" && policy.FailureBehavior != "remove") {
		return Repository{}, ErrInvalidRepository
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err != nil || item.OwnerID != ownerID {
		return Repository{}, ErrNotFound
	}
	if item.IntegrationQueue == nil {
		item.IntegrationQueue = map[string]IntegrationQueuePolicy{}
	}
	if !policy.Enabled {
		delete(item.IntegrationQueue, branch)
	} else {
		item.IntegrationQueue[branch] = policy
	}
	item.UpdatedAt = s.now().UTC()
	if err := s.write(item); err != nil {
		return Repository{}, err
	}
	return item, nil
}

func (s *Store) SetRequiredChecks(ownerID string, id storage.ID, branch string, checks []string) (Repository, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") || strings.Contains(branch, "..") || strings.ContainsAny(branch, " ~^:?*[\\") || len(checks) > 20 {
		return Repository{}, ErrInvalidRepository
	}
	seen := map[string]bool{}
	for i := range checks {
		checks[i] = strings.TrimSpace(checks[i])
		if checks[i] == "" || len(checks[i]) > 100 || seen[checks[i]] {
			return Repository{}, ErrInvalidRepository
		}
		seen[checks[i]] = true
	}
	sort.Strings(checks)
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err != nil || item.OwnerID != ownerID {
		return Repository{}, ErrNotFound
	}
	if item.RequiredChecks == nil {
		item.RequiredChecks = map[string][]string{}
	}
	if len(checks) == 0 {
		delete(item.RequiredChecks, branch)
	} else {
		item.RequiredChecks[branch] = append([]string(nil), checks...)
	}
	item.UpdatedAt = s.now().UTC()
	if err := s.write(item); err != nil {
		return Repository{}, err
	}
	return item, nil
}

func (s *Store) AddCollaborator(ownerID string, id storage.ID, userID string) (Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err != nil || item.OwnerID != ownerID {
		return Repository{}, ErrNotFound
	}
	if userID == "" || userID == ownerID {
		return Repository{}, ErrInvalidRepository
	}
	for _, existing := range item.CollaboratorIDs {
		if existing == userID {
			return item, nil
		}
	}
	item.CollaboratorIDs = append(item.CollaboratorIDs, userID)
	sort.Strings(item.CollaboratorIDs)
	item.UpdatedAt = s.now().UTC()
	if err := s.write(item); err != nil {
		return Repository{}, err
	}
	return item, nil
}

func (s *Store) RemoveCollaborator(ownerID string, id storage.ID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err != nil || item.OwnerID != ownerID {
		return ErrNotFound
	}
	for i, existing := range item.CollaboratorIDs {
		if existing == userID {
			item.CollaboratorIDs = append(item.CollaboratorIDs[:i], item.CollaboratorIDs[i+1:]...)
			item.UpdatedAt = s.now().UTC()
			return s.write(item)
		}
	}
	return ErrNotFound
}

func (s *Store) IsCollaborator(id storage.ID, userID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err != nil {
		return false, err
	}
	for _, existing := range item.CollaboratorIDs {
		if existing == userID {
			return true, nil
		}
	}
	return false, nil
}

type Metadata struct {
	Name        string
	Description string
	Visibility  Visibility
}

type repositoryStorage interface {
	Create() (*storage.Repository, error)
	Fork(storage.ID) (*storage.Repository, error)
	LinkObjects(storage.ID, storage.ID) error
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

func (s *Store) Create(ownerID string, metadata Metadata) (Repository, error) {
	metadata, err := validateMetadata(metadata)
	if err != nil {
		return Repository{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureNameAvailable(ownerID, metadata.Name, ""); err != nil {
		return Repository{}, err
	}
	repository, err := s.storage.Create()
	if err != nil {
		return Repository{}, err
	}
	now := s.now().UTC()
	item := Repository{ID: repository.ID(), OwnerID: ownerID, Name: metadata.Name, Description: metadata.Description, Visibility: metadata.Visibility, Empty: true, CreatedAt: now, UpdatedAt: now}
	if err := s.write(item); err != nil {
		_ = s.storage.Delete(repository.ID())
		return Repository{}, err
	}
	return item, nil
}

// Fork creates an independently owned repository while retaining the source
// repository as durable lineage metadata.
func (s *Store) Fork(ownerID string, upstreamID storage.ID, metadata Metadata) (Repository, error) {
	metadata, err := validateMetadata(metadata)
	if err != nil {
		return Repository{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.read(upstreamID); err != nil {
		return Repository{}, ErrNotFound
	}
	if err := s.ensureNameAvailable(ownerID, metadata.Name, ""); err != nil {
		return Repository{}, err
	}
	repository, err := s.storage.Fork(upstreamID)
	if err != nil {
		return Repository{}, err
	}
	now := s.now().UTC()
	item := Repository{ID: repository.ID(), OwnerID: ownerID, Name: metadata.Name, Description: metadata.Description, Visibility: metadata.Visibility, UpstreamID: upstreamID, CreatedAt: now, UpdatedAt: now}
	info, err := repository.Inspect()
	if err != nil {
		_ = s.storage.Delete(repository.ID())
		return Repository{}, err
	}
	item.Empty = info.Empty
	if err := s.write(item); err != nil {
		_ = s.storage.Delete(repository.ID())
		return Repository{}, err
	}
	return item, nil
}

type SyncResult struct {
	Repository Repository
	Branch     string
	Before     storage.ObjectID
	After      storage.ObjectID
	Updated    bool
}

// SyncForkBranch fast-forwards one fork branch to the identically named
// upstream branch. Diverged fork work is never overwritten.
func (s *Store) SyncForkBranch(ownerID string, id storage.ID, branch string) (SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err != nil || item.OwnerID != ownerID {
		return SyncResult{}, ErrNotFound
	}
	if item.UpstreamID == "" {
		return SyncResult{}, ErrNotFork
	}
	name := storage.ReferenceName("refs/heads/" + branch)
	upstream, err := s.storage.Open(item.UpstreamID)
	if err != nil {
		return SyncResult{}, ErrNotFound
	}
	fork, err := s.storage.Open(item.ID)
	if err != nil {
		return SyncResult{}, err
	}
	upstreamRef, err := upstream.ReadReference(name)
	if errors.Is(err, storage.ErrReferenceNotFound) || errors.Is(err, storage.ErrInvalidRefName) {
		return SyncResult{}, ErrUpstreamBranch
	}
	if err != nil || upstreamRef.ObjectID == "" {
		return SyncResult{}, err
	}
	if err := s.storage.LinkObjects(item.UpstreamID, item.ID); err != nil {
		return SyncResult{}, err
	}
	result := SyncResult{Repository: item, Branch: branch, After: upstreamRef.ObjectID}
	forkRef, err := fork.ReadReference(name)
	if errors.Is(err, storage.ErrReferenceNotFound) {
		if err := fork.CreateReference(storage.Reference{Name: name, ObjectID: upstreamRef.ObjectID}); err != nil {
			return SyncResult{}, err
		}
		result.Updated = true
	} else if err != nil {
		return SyncResult{}, err
	} else {
		result.Before = forkRef.ObjectID
		if forkRef.ObjectID == upstreamRef.ObjectID {
			return result, nil
		}
		ancestor, err := isAncestor(fork, forkRef.ObjectID, upstreamRef.ObjectID)
		if err != nil {
			return SyncResult{}, err
		}
		if !ancestor {
			return SyncResult{}, ErrForkConflict
		}
		if err := fork.UpdateReference(storage.Reference{Name: name, ObjectID: upstreamRef.ObjectID}); err != nil {
			return SyncResult{}, err
		}
		result.Updated = true
	}
	item.UpdatedAt = s.now().UTC()
	if err := s.write(item); err != nil {
		return SyncResult{}, err
	}
	result.Repository = item
	return result, nil
}

func isAncestor(repository storage.RepositoryStorage, ancestor, descendant storage.ObjectID) (bool, error) {
	pending, seen := []storage.ObjectID{descendant}, map[storage.ObjectID]bool{}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if current == ancestor {
			return true, nil
		}
		if seen[current] {
			continue
		}
		seen[current] = true
		commit, err := repository.ReadCommit(current)
		if err != nil {
			return false, err
		}
		pending = append(pending, commit.Parents...)
	}
	return false, nil
}

func validateMetadata(metadata Metadata) (Metadata, error) {
	metadata.Name = strings.ToLower(strings.TrimSpace(metadata.Name))
	metadata.Description = strings.TrimSpace(metadata.Description)
	if !namePattern.MatchString(metadata.Name) || len(metadata.Description) > 280 || (metadata.Visibility != Private && metadata.Visibility != Public) {
		return Metadata{}, ErrInvalidRepository
	}
	return metadata, nil
}

func (s *Store) ensureNameAvailable(ownerID, name string, except storage.ID) error {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		item, err := s.read(storage.ID(strings.TrimSuffix(entry.Name(), ".json")))
		if err != nil {
			return err
		}
		if item.OwnerID == ownerID && item.ID != except && item.Name == name {
			return ErrNameTaken
		}
	}
	return nil
}

// Inspect returns repository metadata without applying an actor policy. It is
// the catalog boundary used by transports to make one authorization decision.
func (s *Store) Inspect(id storage.ID) (Repository, error) {
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
	if err != nil {
		return Repository{}, err
	}
	info, err := opened.Inspect()
	if err != nil {
		return Repository{}, err
	}
	item.Empty = info.Empty
	return item, nil
}

func (s *Store) Open(id storage.ID) (*storage.Repository, error) {
	if _, err := s.Inspect(id); err != nil {
		return nil, err
	}
	return s.storage.Open(id)
}

func (s *Store) LinkObjects(source, target storage.ID) error {
	if _, err := s.Inspect(source); err != nil {
		return err
	}
	if _, err := s.Inspect(target); err != nil {
		return err
	}
	return s.storage.LinkObjects(source, target)
}

func (s *Store) SetVisibility(ownerID string, id storage.ID, visibility Visibility) (Repository, error) {
	if visibility != Private && visibility != Public {
		return Repository{}, errors.New("invalid repository visibility")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err != nil || item.OwnerID != ownerID {
		return Repository{}, ErrNotFound
	}
	item.Visibility = visibility
	item.UpdatedAt = s.now().UTC()
	if err := s.write(item); err != nil {
		return Repository{}, err
	}
	return item, nil
}

func (s *Store) Update(ownerID string, id storage.ID, metadata Metadata) (Repository, error) {
	metadata, err := validateMetadata(metadata)
	if err != nil {
		return Repository{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err != nil || item.OwnerID != ownerID {
		return Repository{}, ErrNotFound
	}
	if err := s.ensureNameAvailable(ownerID, metadata.Name, id); err != nil {
		return Repository{}, err
	}
	item.Name, item.Description, item.Visibility, item.UpdatedAt = metadata.Name, metadata.Description, metadata.Visibility, s.now().UTC()
	if err := s.write(item); err != nil {
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
	return s.list(func(item Repository) bool { return item.OwnerID == ownerID })
}

// ListAccessible returns repositories the actor owns or has been invited to.
// Public repositories are deliberately excluded: this is the actor's durable
// workspace, not a global discovery index.
func (s *Store) ListAccessible(userID string) ([]Repository, error) {
	return s.list(func(item Repository) bool {
		if item.OwnerID == userID {
			return true
		}
		for _, collaboratorID := range item.CollaboratorIDs {
			if collaboratorID == userID {
				return true
			}
		}
		return false
	})
}

// ListPublic returns the discovery index without granting membership.
func (s *Store) ListPublic(query string) ([]Repository, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	return s.list(func(item Repository) bool {
		return item.Visibility == Public && (query == "" || strings.Contains(strings.ToLower(item.Name+" "+item.Description), query))
	})
}

func (s *Store) list(include func(Repository) bool) ([]Repository, error) {
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
		if !include(item) {
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
	if json.Unmarshal(data, &item) != nil {
		return Repository{}, errors.New("invalid repository catalog entry")
	}
	// Catalog entries created before visibility was introduced are private by
	// default, preserving their previous owner-only behavior.
	if item.Visibility == "" {
		item.Visibility = Private
	}
	if item.Name == "" {
		item.Name = string(item.ID)
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	}
	if item.ID != id || item.OwnerID == "" || (item.Visibility != Private && item.Visibility != Public) || item.CreatedAt.IsZero() {
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
