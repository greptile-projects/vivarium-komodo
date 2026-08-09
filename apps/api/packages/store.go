// Package packages owns immutable, attested package versions.
package packages

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound        = errors.New("package version not found")
	ErrInvalid         = errors.New("invalid package version")
	ErrVersionConflict = errors.New("package version already exists")
	ErrSafetyConflict  = errors.New("package safety transition conflict")
)

var (
	namePattern    = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,98}[a-z0-9])?$`)
	versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	platformValue  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
	dependencyName = regexp.MustCompile(`^@[a-z0-9][a-z0-9._-]{0,98}/[a-z0-9][a-z0-9._-]{0,99}$`)
)

type Platform struct {
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Runtime string `json:"runtime,omitempty"`
}

type BuildAttestation struct {
	RunID       string    `json:"run_id"`
	BuildName   string    `json:"build_name"`
	Command     string    `json:"command"`
	CompletedAt time.Time `json:"completed_at"`
}

type Version struct {
	ID                  string            `json:"id"`
	Identity            string            `json:"identity"`
	Name                string            `json:"name"`
	Version             string            `json:"version"`
	RepositoryID        string            `json:"repository_id"`
	ReleaseID           string            `json:"release_id"`
	SourceCommitID      string            `json:"source_commit_id"`
	ArtifactID          string            `json:"artifact_id"`
	ArtifactPath        string            `json:"artifact_path"`
	ArtifactMediaType   string            `json:"artifact_media_type"`
	ArtifactSize        int64             `json:"artifact_size"`
	SHA256              string            `json:"sha256"`
	Build               BuildAttestation  `json:"build_attestation"`
	Platform            Platform          `json:"platform"`
	Dependencies        map[string]string `json:"dependencies"`
	Documentation       string            `json:"documentation,omitempty"`
	DocumentationSHA256 string            `json:"documentation_sha256,omitempty"`
	PublisherID         string            `json:"publisher_id"`
	Visibility          string            `json:"visibility"`
	Lifecycle           string            `json:"lifecycle"`
	Safety              *SafetyNotice     `json:"safety,omitempty"`
	SafetyHistory       []SafetyNotice    `json:"safety_history,omitempty"`
	License             string            `json:"license,omitempty"`
	SupportURL          string            `json:"support_url,omitempty"`
	PublishedAt         time.Time         `json:"published_at"`
}

// SafetyNotice is mutable distribution policy layered over immutable package
// evidence. History is append-only so a publisher cannot erase an earlier
// warning by changing its current recommendation.
type SafetyNotice struct {
	State                string    `json:"state"`
	Reason               string    `json:"reason"`
	ReplacementVersionID string    `json:"replacement_version_id,omitempty"`
	ActorID              string    `json:"actor_id"`
	CreatedAt            time.Time `json:"created_at"`
}

type PublishParams struct {
	OwnerID, Name, Version, RepositoryID, ReleaseID, SourceCommitID string
	ArtifactID, ArtifactPath, ArtifactMediaType, ExpectedSHA256     string
	ArtifactSize                                                    int64
	Build                                                           BuildAttestation
	Platform                                                        Platform
	Dependencies                                                    map[string]string
	Documentation                                                   string
	PublisherID, Visibility                                         string
	License, SupportURL                                             string
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("package root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(abs, "versions"), 0o750); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(abs, "artifacts"), 0o750); err != nil {
		return nil, err
	}
	return &Store{root: abs, now: time.Now}, nil
}

// Publish copies and verifies bytes before the version record becomes visible.
func (s *Store) Publish(p PublishParams, source io.Reader) (Version, error) {
	p.Name, p.Version = strings.ToLower(strings.TrimSpace(p.Name)), strings.TrimSpace(p.Version)
	p.Visibility = strings.ToLower(strings.TrimSpace(p.Visibility))
	p.Documentation = strings.TrimSpace(p.Documentation)
	p.License, p.SupportURL = strings.TrimSpace(p.License), strings.TrimSpace(p.SupportURL)
	if p.OwnerID == "" || p.RepositoryID == "" || p.ReleaseID == "" || p.SourceCommitID == "" || p.PublisherID == "" || p.ArtifactID == "" || !namePattern.MatchString(p.Name) || !versionPattern.MatchString(p.Version) || (p.Visibility != "public" && p.Visibility != "private") || !validPlatform(p.Platform) || !validDependencies(p.Dependencies) || len(p.Documentation) > 64<<10 || len(p.License) > 100 || !validSupportURL(p.SupportURL) || len(p.ExpectedSHA256) != 64 || source == nil {
		return Version{}, ErrInvalid
	}
	identity := "@" + strings.ToLower(p.OwnerID) + "/" + p.Name
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.listAll()
	if err != nil {
		return Version{}, err
	}
	for _, item := range items {
		if strings.EqualFold(item.Identity, identity) && strings.EqualFold(item.Version, p.Version) {
			return Version{}, ErrVersionConflict
		}
	}
	tmp, err := os.CreateTemp(filepath.Join(s.root, "artifacts"), ".upload-*.tmp")
	if err != nil {
		return Version{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(source, 1<<30+1))
	if syncErr := tmp.Sync(); copyErr == nil {
		copyErr = syncErr
	}
	if closeErr := tmp.Close(); copyErr == nil {
		copyErr = closeErr
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if copyErr != nil {
		return Version{}, copyErr
	}
	if size > 1<<30 || size != p.ArtifactSize || digest != strings.ToLower(p.ExpectedSHA256) {
		return Version{}, ErrInvalid
	}
	id, err := newID()
	if err != nil {
		return Version{}, err
	}
	artifactName := id + ".blob"
	if err := os.Rename(tmpName, filepath.Join(s.root, "artifacts", artifactName)); err != nil {
		return Version{}, err
	}
	documentationDigest := ""
	if p.Documentation != "" {
		sum := sha256.Sum256([]byte(p.Documentation))
		documentationDigest = hex.EncodeToString(sum[:])
	}
	item := Version{ID: id, Identity: identity, Name: p.Name, Version: p.Version, RepositoryID: p.RepositoryID, ReleaseID: p.ReleaseID, SourceCommitID: p.SourceCommitID, ArtifactID: p.ArtifactID, ArtifactPath: p.ArtifactPath, ArtifactMediaType: p.ArtifactMediaType, ArtifactSize: size, SHA256: digest, Build: p.Build, Platform: p.Platform, Dependencies: cloneMap(p.Dependencies), Documentation: p.Documentation, DocumentationSHA256: documentationDigest, PublisherID: p.PublisherID, Visibility: p.Visibility, Lifecycle: "active", License: p.License, SupportURL: p.SupportURL, PublishedAt: s.now().UTC()}
	if err := s.write(item); err != nil {
		_ = os.Remove(filepath.Join(s.root, "artifacts", artifactName))
		return Version{}, err
	}
	return item, nil
}

// GetByID resolves a globally unique version without weakening HTTP policy.
// Callers remain responsible for applying repository and package visibility.
func (s *Store) GetByID(id string) (Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(id)
}

// SetSafety changes only distribution policy; build, artifact, release, and
// checksum evidence in the version record remains byte-for-byte represented.
func (s *Store) SetSafety(repositoryID, id, state, reason, replacementID, actorID string) (Version, error) {
	state, reason, replacementID = strings.ToLower(strings.TrimSpace(state)), strings.TrimSpace(reason), strings.TrimSpace(replacementID)
	if repositoryID == "" || id == "" || actorID == "" || (state != "deprecated" && state != "quarantined") || reason == "" || len(reason) > 4096 || replacementID == id {
		return Version{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err != nil || item.RepositoryID != repositoryID {
		return Version{}, ErrNotFound
	}
	if replacementID != "" {
		replacement, replacementErr := s.read(replacementID)
		if replacementErr != nil || replacement.Identity != item.Identity || replacement.Lifecycle != "active" {
			return Version{}, ErrInvalid
		}
	}
	if item.Safety != nil && item.Safety.State == state && item.Safety.Reason == reason && item.Safety.ReplacementVersionID == replacementID {
		return Version{}, ErrSafetyConflict
	}
	notice := SafetyNotice{State: state, Reason: reason, ReplacementVersionID: replacementID, ActorID: actorID, CreatedAt: s.now().UTC()}
	item.Lifecycle, item.Safety = state, &notice
	item.SafetyHistory = append(item.SafetyHistory, notice)
	return item, s.write(item)
}

// Search returns catalog records matching identity, version, or documentation.
// Visibility filtering belongs to the caller because it depends on its actor.
func (s *Store) Search(query string) ([]Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.listAll()
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return items, nil
	}
	matched := make([]Version, 0)
	for _, item := range items {
		haystack := strings.ToLower(item.Identity + " " + item.Version + " " + item.Documentation)
		if strings.Contains(haystack, query) {
			matched = append(matched, item)
		}
	}
	return matched, nil
}

func (s *Store) Get(repositoryID, id string) (Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err == nil && item.RepositoryID != repositoryID {
		err = ErrNotFound
	}
	return item, err
}

func (s *Store) List(repositoryID string) ([]Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.listAll()
	if err != nil {
		return nil, err
	}
	items := []Version{}
	for _, item := range all {
		if item.RepositoryID == repositoryID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) OpenArtifact(repositoryID, id string) (Version, *os.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(id)
	if err != nil || item.RepositoryID != repositoryID {
		return Version{}, nil, ErrNotFound
	}
	file, err := os.Open(filepath.Join(s.root, "artifacts", item.ID+".blob"))
	return item, file, err
}

func (s *Store) listAll() ([]Version, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "versions"))
	if err != nil {
		return nil, err
	}
	items := []Version{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		item, err := s.read(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].PublishedAt.After(items[j].PublishedAt) })
	return items, nil
}

func (s *Store) read(id string) (Version, error) {
	data, err := os.ReadFile(filepath.Join(s.root, "versions", id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Version{}, ErrNotFound
	}
	if err != nil {
		return Version{}, err
	}
	var item Version
	if json.Unmarshal(data, &item) != nil || item.ID != id {
		return Version{}, ErrNotFound
	}
	return item, nil
}

func (s *Store) write(item Version) error {
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Join(s.root, "versions"), ".version-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(append(data, '\n'))
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(s.root, "versions", item.ID+".json"))
}

func validPlatform(p Platform) bool {
	return platformValue.MatchString(p.OS) && platformValue.MatchString(p.Arch) && (p.Runtime == "" || platformValue.MatchString(p.Runtime))
}
func validDependencies(values map[string]string) bool {
	if len(values) > 100 {
		return false
	}
	for name, constraint := range values {
		if !dependencyName.MatchString(name) || strings.TrimSpace(constraint) == "" || len(constraint) > 200 {
			return false
		}
	}
	return true
}
func validSupportURL(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 500 {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}
func cloneMap(values map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range values {
		out[k] = strings.TrimSpace(v)
	}
	return out
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", raw[:8], raw[8:12], raw[12:16], raw[16:20], raw[20:]), nil
}
