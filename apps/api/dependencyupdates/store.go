// Package dependencyupdates owns consumer-controlled package update policy and evidence.
package dependencyupdates

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

var ErrNotFound = errors.New("dependency update not found")
var ErrInvalid = errors.New("invalid dependency update")
var ErrConflict = errors.New("dependency update already exists")

type Policy struct {
	RepositoryID string    `json:"repository_id"`
	Identity     string    `json:"identity"`
	TargetBranch string    `json:"target_branch"`
	Allowed      string    `json:"allowed"`
	Enabled      bool      `json:"enabled"`
	UpdatedByID  string    `json:"updated_by_id"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Evidence struct {
	CurrentPackageVersionID   string   `json:"current_package_version_id"`
	CurrentVersion            string   `json:"current_version"`
	CandidatePackageVersionID string   `json:"candidate_package_version_id"`
	CandidateVersion          string   `json:"candidate_version"`
	PublisherRepositoryID     string   `json:"publisher_repository_id"`
	ReleaseID                 string   `json:"release_id"`
	ReleaseNotes              string   `json:"release_notes"`
	SourceCommitID            string   `json:"source_commit_id"`
	BuildRunID                string   `json:"build_run_id"`
	ArtifactSHA256            string   `json:"artifact_sha256"`
	Compatibility             string   `json:"compatibility"`
	AffectedPaths             []string `json:"affected_dependency_paths"`
}

type Update struct {
	ID           string          `json:"id"`
	RepositoryID string          `json:"repository_id"`
	InventoryID  string          `json:"inventory_id"`
	BaseCommitID string          `json:"base_commit_id"`
	TargetBranch string          `json:"target_branch"`
	Identity     string          `json:"identity"`
	ProposalID   string          `json:"proposal_id"`
	TaskID       string          `json:"task_id"`
	Manifest     json.RawMessage `json:"manifest"`
	Lock         json.RawMessage `json:"lock"`
	Evidence     Evidence        `json:"evidence"`
	CreatedByID  string          `json:"created_by_id"`
	CreatedAt    time.Time       `json:"created_at"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, ErrInvalid
	}
	abs, err := filepath.Abs(root)
	if err == nil {
		err = os.MkdirAll(filepath.Join(abs, "updates"), 0750)
	}
	if err != nil {
		return nil, err
	}
	return &Store{root: abs, now: time.Now}, nil
}

func (s *Store) PutPolicy(p Policy) (Policy, error) {
	p.Identity = strings.ToLower(strings.TrimSpace(p.Identity))
	p.TargetBranch = strings.TrimSpace(p.TargetBranch)
	p.Allowed = strings.TrimSpace(p.Allowed)
	if p.RepositoryID == "" || p.Identity == "" || p.TargetBranch == "" || p.UpdatedByID == "" || (p.Allowed != "patch" && p.Allowed != "minor" && p.Allowed != "major") {
		return Policy{}, ErrInvalid
	}
	p.UpdatedAt = s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, "policies", p.RepositoryID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return Policy{}, err
	}
	return p, atomicWrite(filepath.Join(dir, hex.EncodeToString([]byte(p.Identity))+".json"), p)
}

func (s *Store) ListPolicies(repositoryID string) ([]Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Policy
	entries, err := os.ReadDir(filepath.Join(s.root, "policies", repositoryID))
	if errors.Is(err, fs.ErrNotExist) {
		return []Policy{}, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		var p Policy
		if e.IsDir() {
			continue
		}
		b, er := os.ReadFile(filepath.Join(s.root, "policies", repositoryID, e.Name()))
		if er != nil || json.Unmarshal(b, &p) != nil {
			return nil, ErrInvalid
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Identity < out[j].Identity })
	return out, nil
}

func (s *Store) Create(u Update) (Update, error) {
	if u.RepositoryID == "" || u.InventoryID == "" || u.BaseCommitID == "" || u.Identity == "" || u.ProposalID == "" || u.TaskID == "" || len(u.Manifest) == 0 || len(u.Lock) == 0 {
		return Update{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.list(u.RepositoryID)
	if err != nil {
		return Update{}, err
	}
	for _, v := range all {
		if v.BaseCommitID == u.BaseCommitID && v.Identity == u.Identity && v.Evidence.CandidatePackageVersionID == u.Evidence.CandidatePackageVersionID {
			return Update{}, ErrConflict
		}
	}
	b := make([]byte, 16)
	if _, err = rand.Read(b); err != nil {
		return Update{}, err
	}
	u.ID = hex.EncodeToString(b)
	u.CreatedAt = s.now().UTC()
	return u, atomicWrite(filepath.Join(s.root, "updates", u.ID+".json"), u)
}
func (s *Store) Exists(repositoryID, baseCommitID, identity, candidateID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.list(repositoryID)
	if err != nil {
		return false, err
	}
	for _, v := range all {
		if v.BaseCommitID == baseCommitID && v.Identity == identity && v.Evidence.CandidatePackageVersionID == candidateID {
			return true, nil
		}
	}
	return false, nil
}
func (s *Store) List(repositoryID string) ([]Update, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list(repositoryID)
}
func (s *Store) list(repositoryID string) ([]Update, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "updates"))
	if err != nil {
		return nil, err
	}
	out := []Update{}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		var u Update
		b, er := os.ReadFile(filepath.Join(s.root, "updates", e.Name()))
		if er != nil || json.Unmarshal(b, &u) != nil {
			return nil, ErrInvalid
		}
		if u.RepositoryID == repositoryID {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func atomicWrite(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".writing-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(append(b, '\n'))
	}
	if err == nil {
		err = tmp.Sync()
	}
	if e := tmp.Close(); err == nil {
		err = e
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	return err
}
