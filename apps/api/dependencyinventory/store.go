// Package dependencyinventory owns immutable, commit-derived package resolution evidence.
package dependencyinventory

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

var ErrNotFound = errors.New("dependency inventory not found")
var ErrInvalid = errors.New("invalid dependency inventory")
var ErrConflict = errors.New("dependency inventory already exists")

type Resolution struct {
	Identity            string   `json:"identity"`
	PackageVersionID    string   `json:"package_version_id,omitempty"`
	Version             string   `json:"version,omitempty"`
	Direct              bool     `json:"direct"`
	Dependencies        []string `json:"dependencies"`
	Status              string   `json:"status"`
	Reason              string   `json:"reason,omitempty"`
	PackageRepositoryID string   `json:"package_repository_id,omitempty"`
	SourceCommitID      string   `json:"source_commit_id,omitempty"`
	ReleaseID           string   `json:"release_id,omitempty"`
	BuildRunID          string   `json:"build_run_id,omitempty"`
	ArtifactID          string   `json:"artifact_id,omitempty"`
	ArtifactSHA256      string   `json:"artifact_sha256,omitempty"`
	License             string   `json:"license,omitempty"`
	SupportURL          string   `json:"support_url,omitempty"`
}
type Inventory struct {
	ID             string       `json:"id"`
	RepositoryID   string       `json:"repository_id"`
	CommitID       string       `json:"commit_id"`
	ReleaseID      string       `json:"release_id,omitempty"`
	BuildRunID     string       `json:"build_run_id,omitempty"`
	DeploymentID   string       `json:"deployment_id,omitempty"`
	ManifestPath   string       `json:"manifest_path"`
	LockPath       string       `json:"lock_path"`
	ManifestSHA256 string       `json:"manifest_sha256"`
	LockSHA256     string       `json:"lock_sha256"`
	Resolutions    []Resolution `json:"resolutions"`
	Status         string       `json:"status"`
	ProvenanceGaps []string     `json:"provenance_gaps"`
	CreatedByID    string       `json:"created_by_id"`
	CreatedAt      time.Time    `json:"created_at"`
}
type CreateParams struct {
	RepositoryID, CommitID, ReleaseID, BuildRunID, DeploymentID, ManifestSHA256, LockSHA256, CreatedByID string
	Resolutions                                                                                          []Resolution
	ProvenanceGaps                                                                                       []string
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
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	if e != nil {
		return nil, e
	}
	return &Store{root: a, now: time.Now}, nil
}
func (s *Store) Create(p CreateParams) (Inventory, error) {
	if p.RepositoryID == "" || p.CommitID == "" || p.CreatedByID == "" || p.ManifestSHA256 == "" || p.LockSHA256 == "" {
		return Inventory{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.listAll()
	if e != nil {
		return Inventory{}, e
	}
	for _, v := range all {
		if v.RepositoryID == p.RepositoryID && v.CommitID == p.CommitID && v.ReleaseID == p.ReleaseID && v.BuildRunID == p.BuildRunID && v.DeploymentID == p.DeploymentID {
			return Inventory{}, ErrConflict
		}
	}
	b := make([]byte, 16)
	if _, e = rand.Read(b); e != nil {
		return Inventory{}, e
	}
	status := "resolved"
	if len(p.ProvenanceGaps) > 0 {
		status = "incomplete"
	}
	v := Inventory{ID: hex.EncodeToString(b), RepositoryID: p.RepositoryID, CommitID: p.CommitID, ReleaseID: p.ReleaseID, BuildRunID: p.BuildRunID, DeploymentID: p.DeploymentID, ManifestPath: ".komodo/packages.json", LockPath: ".komodo/packages.lock.json", ManifestSHA256: p.ManifestSHA256, LockSHA256: p.LockSHA256, Resolutions: p.Resolutions, Status: status, ProvenanceGaps: p.ProvenanceGaps, CreatedByID: p.CreatedByID, CreatedAt: s.now().UTC()}
	return v, s.write(v)
}
func (s *Store) List(repositoryID string) ([]Inventory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.listAll()
	o := []Inventory{}
	for _, v := range a {
		if v.RepositoryID == repositoryID {
			o = append(o, v)
		}
	}
	return o, e
}
func (s *Store) Get(repositoryID, id string) (Inventory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.listAll()
	if e != nil {
		return Inventory{}, e
	}
	for _, v := range a {
		if v.RepositoryID == repositoryID && v.ID == id {
			return v, nil
		}
	}
	return Inventory{}, ErrNotFound
}
func (s *Store) Consumers(packageVersionID string) ([]Inventory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.listAll()
	o := []Inventory{}
	for _, v := range a {
		for _, r := range v.Resolutions {
			if r.PackageVersionID == packageVersionID {
				o = append(o, v)
				break
			}
		}
	}
	return o, e
}
func (s *Store) listAll() ([]Inventory, error) {
	es, e := os.ReadDir(s.root)
	if errors.Is(e, fs.ErrNotExist) {
		return []Inventory{}, nil
	}
	if e != nil {
		return nil, e
	}
	o := []Inventory{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		d, er := os.ReadFile(filepath.Join(s.root, x.Name()))
		var v Inventory
		if er != nil || json.Unmarshal(d, &v) != nil {
			return nil, ErrInvalid
		}
		o = append(o, v)
	}
	sort.Slice(o, func(i, j int) bool { return o[i].CreatedAt.After(o[j].CreatedAt) })
	return o, nil
}
func (s *Store) write(v Inventory) error {
	d, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	n := filepath.Join(s.root, v.ID+".json")
	t, e := os.CreateTemp(s.root, ".inventory-*.tmp")
	if e != nil {
		return e
	}
	tn := t.Name()
	defer os.Remove(tn)
	if e = t.Chmod(0600); e == nil {
		_, e = t.Write(append(d, '\n'))
	}
	if e == nil {
		e = t.Sync()
	}
	if ce := t.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(tn, n)
	}
	return e
}
func NormalizeIdentity(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
