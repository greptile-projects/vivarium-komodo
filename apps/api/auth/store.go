// Package auth owns password verification and durable, revocable access grants.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidPassword = errors.New("password must contain between 12 and 72 bytes")
	ErrInvalidGrant    = errors.New("invalid access grant")
	ErrNotFound        = errors.New("access grant not found")
	ErrUnauthenticated = errors.New("credential is invalid, expired, or revoked")
)

type Kind string
type Scope string

const (
	Web Kind = "web"
	API Kind = "api"
	Git Kind = "git"

	ProfileRead     Scope = "profile:read"
	ProfileWrite    Scope = "profile:write"
	AccessManage    Scope = "access:manage"
	RepositoryRead  Scope = "repository:read"
	RepositoryWrite Scope = "repository:write"
	GitRead         Scope = "git:read"
	GitWrite        Scope = "git:write"
)

const TokenPrefix = "vkm_"

var kindPolicy = map[Kind]struct {
	maximum time.Duration
	scopes  []Scope
}{
	Web: {12 * time.Hour, []Scope{ProfileRead, ProfileWrite, AccessManage, RepositoryRead, RepositoryWrite}},
	API: {90 * 24 * time.Hour, []Scope{ProfileRead, ProfileWrite, AccessManage, RepositoryRead, RepositoryWrite}},
	Git: {30 * 24 * time.Hour, []Scope{GitRead, GitWrite}},
}

type Grant struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	Name         string     `json:"name"`
	Kind         Kind       `json:"kind"`
	Scopes       []Scope    `json:"scopes"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	Digest       string     `json:"-"`
	RepositoryID string     `json:"repository_id,omitempty"`
	Branch       string     `json:"branch,omitempty"`
}

// IssueRepositoryGit creates a worker credential whose authority is limited to
// one repository and one branch. It is intentionally unavailable through the
// general access-grant API.
func (s *Store) IssueRepositoryGit(userID, name, repositoryID, branch string, lifetime time.Duration) (IssuedGrant, error) {
	if repositoryID == "" || branch == "" || !strings.HasPrefix(branch, "refs/heads/") {
		return IssuedGrant{}, ErrInvalidGrant
	}
	return s.issue(userID, name, Git, []Scope{GitRead, GitWrite}, lifetime, repositoryID, branch)
}

type IssuedGrant struct {
	Grant
	Token string `json:"token"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("auth storage root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve auth root: %w", err)
	}
	for _, directory := range []string{filepath.Join(abs, "passwords"), filepath.Join(abs, "grants")} {
		if err := os.MkdirAll(directory, 0o750); err != nil {
			return nil, fmt.Errorf("create auth storage: %w", err)
		}
	}
	return &Store{root: abs, now: time.Now}, nil
}

func (s *Store) SetPassword(userID, password string) error {
	if len(password) < 12 || len(password) > 72 {
		return ErrInvalidPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.root, "passwords", userID+".json")
	if _, err := os.Stat(path); err == nil {
		return errors.New("password already established")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return atomicWrite(path, append(hash, '\n'), 0o600)
}

func (s *Store) VerifyPassword(userID, password string) error {
	hash, err := os.ReadFile(filepath.Join(s.root, "passwords", userID+".json"))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(strings.TrimSpace(string(hash))), []byte(password)) != nil {
		return ErrUnauthenticated
	}
	return nil
}

func (s *Store) Issue(userID, name string, kind Kind, scopes []Scope, lifetime time.Duration) (IssuedGrant, error) {
	return s.issue(userID, name, kind, scopes, lifetime, "", "")
}

func (s *Store) issue(userID, name string, kind Kind, scopes []Scope, lifetime time.Duration, repositoryID, branch string) (IssuedGrant, error) {
	policy, ok := kindPolicy[kind]
	name = strings.TrimSpace(name)
	if !ok || name == "" || len(name) > 100 || lifetime <= 0 || lifetime > policy.maximum || !validScopes(scopes, policy.scopes) {
		return IssuedGrant{}, ErrInvalidGrant
	}
	id, err := randomHex(16)
	if err != nil {
		return IssuedGrant{}, err
	}
	secret, err := randomToken()
	if err != nil {
		return IssuedGrant{}, err
	}
	now := s.now().UTC()
	grant := Grant{ID: id, UserID: userID, Name: name, Kind: kind, Scopes: append([]Scope(nil), scopes...), CreatedAt: now, ExpiresAt: now.Add(lifetime), Digest: digest(secret), RepositoryID: repositoryID, Branch: branch}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.write(grant); err != nil {
		return IssuedGrant{}, err
	}
	return IssuedGrant{Grant: grant, Token: secret}, nil
}

func (s *Store) Authenticate(token string, required Scope) (Grant, error) {
	if !strings.HasPrefix(token, TokenPrefix) {
		return Grant{}, ErrUnauthenticated
	}
	want := digest(token)
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, "grants"))
	if err != nil {
		return Grant{}, err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		grant, err := s.read(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return Grant{}, err
		}
		if subtle.ConstantTimeCompare([]byte(grant.Digest), []byte(want)) != 1 {
			continue
		}
		if grant.RevokedAt != nil || !s.now().Before(grant.ExpiresAt) || !hasScope(grant.Scopes, required) {
			return Grant{}, ErrUnauthenticated
		}
		now := s.now().UTC()
		grant.LastUsedAt = &now
		if err := s.write(grant); err != nil {
			return Grant{}, err
		}
		return grant, nil
	}
	return Grant{}, ErrUnauthenticated
}

func (s *Store) List(userID string) ([]Grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, "grants"))
	if err != nil {
		return nil, err
	}
	grants := []Grant{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		grant, err := s.read(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		if grant.UserID == userID {
			grants = append(grants, grant)
		}
	}
	sort.Slice(grants, func(i, j int) bool { return grants[i].CreatedAt.Before(grants[j].CreatedAt) })
	return grants, nil
}

func (s *Store) Revoke(userID, id string) (Grant, error) {
	if len(id) != 32 {
		return Grant{}, ErrNotFound
	}
	if decoded, err := hex.DecodeString(id); err != nil || hex.EncodeToString(decoded) != id {
		return Grant{}, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, err := s.read(id)
	if err != nil || grant.UserID != userID {
		return Grant{}, ErrNotFound
	}
	if grant.RevokedAt == nil {
		now := s.now().UTC()
		grant.RevokedAt = &now
		if err := s.write(grant); err != nil {
			return Grant{}, err
		}
	}
	return grant, nil
}

func (s *Store) RevokeRepositoryGit(repositoryID, branch, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, "grants"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		grant, err := s.read(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return err
		}
		if grant.RepositoryID != repositoryID || grant.Branch != branch || grant.Name != name || grant.RevokedAt != nil {
			continue
		}
		now := s.now().UTC()
		grant.RevokedAt = &now
		if err := s.write(grant); err != nil {
			return err
		}
	}
	return nil
}

type storedGrant struct {
	Grant
	Digest string `json:"digest"`
}

func (s *Store) read(id string) (Grant, error) {
	data, err := os.ReadFile(filepath.Join(s.root, "grants", id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Grant{}, ErrNotFound
	}
	if err != nil {
		return Grant{}, err
	}
	var stored storedGrant
	if json.Unmarshal(data, &stored) != nil || stored.ID != id || stored.Digest == "" {
		return Grant{}, errors.New("invalid stored grant")
	}
	stored.Grant.Digest = stored.Digest
	return stored.Grant, nil
}
func (s *Store) write(grant Grant) error {
	data, err := json.Marshal(storedGrant{Grant: grant, Digest: grant.Digest})
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(s.root, "grants", grant.ID+".json"), append(data, '\n'), 0o600)
}

func validScopes(scopes, allowed []Scope) bool {
	if len(scopes) == 0 {
		return false
	}
	seen := map[Scope]bool{}
	for _, scope := range scopes {
		if seen[scope] || !hasScope(allowed, scope) {
			return false
		}
		seen[scope] = true
	}
	return true
}
func hasScope(scopes []Scope, wanted Scope) bool {
	for _, scope := range scopes {
		if scope == wanted {
			return true
		}
	}
	return false
}
func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func randomHex(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return TokenPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}
func atomicWrite(path string, data []byte, mode fs.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".auth-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
