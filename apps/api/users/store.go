// Package users owns durable human identity and profile storage.
package users

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound       = errors.New("user not found")
	ErrInvalidID      = errors.New("invalid user ID")
	ErrInvalidProfile = errors.New("invalid user profile")
	ErrHandleTaken    = errors.New("handle is already taken")
)

var handlePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?$`)

// ID is a user's stable identity. Profile fields may change; this value does not.
type ID string

// User is the durable identity and minimal public profile of a collaborator.
type User struct {
	ID          ID        `json:"id"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Profile contains the user-controlled identity fields.
type Profile struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
}

// Store persists users beneath one directory.
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("user storage root is required")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve user storage root: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o750); err != nil {
		return nil, fmt.Errorf("create user storage root: %w", err)
	}
	return &Store{root: absRoot, now: time.Now}, nil
}

func (s *Store) Create(profile Profile) (User, error) {
	profile, err := validateProfile(profile)
	if err != nil {
		return User{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureHandleAvailable(profile.Handle, ""); err != nil {
		return User{}, err
	}
	id, err := newID()
	if err != nil {
		return User{}, fmt.Errorf("generate user ID: %w", err)
	}
	now := s.now().UTC()
	user := User{ID: id, Handle: profile.Handle, DisplayName: profile.DisplayName, CreatedAt: now, UpdatedAt: now}
	if err := s.write(user, true); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) Get(id ID) (User, error) {
	if !validID(id) {
		return User{}, ErrInvalidID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(id)
}

// FindByHandle resolves the case-normalized sign-in name to its stable actor.
func (s *Store) FindByHandle(handle string) (User, error) {
	handle = strings.ToLower(strings.TrimSpace(handle))
	if !handlePattern.MatchString(handle) {
		return User{}, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return User{}, fmt.Errorf("list users: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		user, err := s.read(ID(strings.TrimSuffix(entry.Name(), ".json")))
		if err != nil {
			return User{}, err
		}
		if user.Handle == handle {
			return user, nil
		}
	}
	return User{}, ErrNotFound
}

func (s *Store) Update(id ID, profile Profile) (User, error) {
	if !validID(id) {
		return User{}, ErrInvalidID
	}
	profile, err := validateProfile(profile)
	if err != nil {
		return User{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user, err := s.read(id)
	if err != nil {
		return User{}, err
	}
	if err := s.ensureHandleAvailable(profile.Handle, id); err != nil {
		return User{}, err
	}
	user.Handle = profile.Handle
	user.DisplayName = profile.DisplayName
	user.UpdatedAt = s.now().UTC()
	if err := s.write(user, false); err != nil {
		return User{}, err
	}
	return user, nil
}

func validateProfile(profile Profile) (Profile, error) {
	profile.Handle = strings.ToLower(strings.TrimSpace(profile.Handle))
	profile.DisplayName = strings.TrimSpace(profile.DisplayName)
	if !handlePattern.MatchString(profile.Handle) || len(profile.DisplayName) == 0 || len(profile.DisplayName) > 100 {
		return Profile{}, ErrInvalidProfile
	}
	return profile, nil
}

func (s *Store) ensureHandleAvailable(handle string, except ID) error {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		user, err := s.read(ID(strings.TrimSuffix(entry.Name(), ".json")))
		if err != nil {
			return err
		}
		if user.ID != except && user.Handle == handle {
			return ErrHandleTaken
		}
	}
	return nil
}

func (s *Store) read(id ID) (User, error) {
	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, fs.ErrNotExist) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("read user: %w", err)
	}
	var user User
	if err := json.Unmarshal(data, &user); err != nil || user.ID != id {
		return User{}, errors.New("invalid stored user")
	}
	return user, nil
}

func (s *Store) write(user User, exclusive bool) error {
	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("encode user: %w", err)
	}
	temporary, err := os.CreateTemp(s.root, ".user-*")
	if err != nil {
		return fmt.Errorf("stage user: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o640); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		return fmt.Errorf("write user: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync user: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close user: %w", err)
	}
	if exclusive {
		if _, err := os.Stat(s.path(user.ID)); !errors.Is(err, fs.ErrNotExist) {
			return errors.New("user ID collision")
		}
	}
	if err := os.Rename(temporaryName, s.path(user.ID)); err != nil {
		return fmt.Errorf("publish user: %w", err)
	}
	return nil
}

func (s *Store) path(id ID) string { return filepath.Join(s.root, string(id)+".json") }

func newID() (ID, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	e := hex.EncodeToString(b)
	return ID(e[:8] + "-" + e[8:12] + "-" + e[12:16] + "-" + e[16:20] + "-" + e[20:]), nil
}

func validID(id ID) bool {
	v := string(id)
	if len(v) != 36 || v[8] != '-' || v[13] != '-' || v[18] != '-' || v[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(v, "-", ""))
	return err == nil && v == strings.ToLower(v)
}
