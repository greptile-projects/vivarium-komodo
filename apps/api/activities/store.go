// Package activities owns the append-only account of meaningful repository collaboration.
package activities

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

type UserResolver interface {
	FindByHandle(string) (users.User, error)
}

type Resource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type Event struct {
	ID           string            `json:"id"`
	RepositoryID string            `json:"repository_id"`
	ActorID      string            `json:"actor_id"`
	Type         string            `json:"type"`
	Resource     Resource          `json:"resource"`
	TargetUserID string            `json:"target_user_id,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

type Input struct {
	RepositoryID string
	ActorID      string
	Type         string
	Resource     Resource
	TargetUserID string
	Metadata     map[string]string
	MentionText  string
}

type Store struct {
	root  string
	users UserResolver
	mu    sync.Mutex
	now   func() time.Time
}

var mentionPattern = regexp.MustCompile(`(?i)(?:^|[^a-z0-9-])@([a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?)`)

func New(root string, resolver UserResolver) (*Store, error) {
	if root == "" || resolver == nil {
		return nil, errors.New("activity root and user resolver are required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, err
	}
	return &Store{root: abs, users: resolver, now: time.Now}, nil
}

// Record appends the primary state change and a distinct event for every
// resolvable user mention. Mention events retain stable user IDs even if the
// mentioned collaborator later changes their handle.
func (s *Store) Record(input Input) (Event, error) {
	if input.RepositoryID == "" || input.ActorID == "" || input.Type == "" || input.Resource.Type == "" || input.Resource.ID == "" {
		return Event{}, errors.New("invalid activity")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	event, err := s.append(input, input.Type, input.TargetUserID, input.Metadata)
	if err != nil {
		return Event{}, err
	}
	seen := map[string]bool{}
	for _, match := range mentionPattern.FindAllStringSubmatch(input.MentionText, -1) {
		handle := strings.ToLower(match[1])
		user, err := s.users.FindByHandle(handle)
		if err != nil || user.ID == "" || string(user.ID) == input.ActorID || seen[string(user.ID)] {
			continue
		}
		seen[string(user.ID)] = true
		metadata := map[string]string{"handle": handle, "source_event_id": event.ID}
		if _, err := s.append(input, "mention.created", string(user.ID), metadata); err != nil {
			return Event{}, err
		}
	}
	return event, nil
}

func (s *Store) append(input Input, eventType, targetUserID string, metadata map[string]string) (Event, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return Event{}, err
	}
	event := Event{ID: hex.EncodeToString(idBytes), RepositoryID: input.RepositoryID, ActorID: input.ActorID, Type: eventType, Resource: input.Resource, TargetUserID: targetUserID, Metadata: metadata, CreatedAt: s.now().UTC()}
	dir := filepath.Join(s.root, input.RepositoryID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return Event{}, err
	}
	data, err := json.Marshal(event)
	if err != nil {
		return Event{}, err
	}
	temporary, err := os.CreateTemp(dir, ".activity-*")
	if err != nil {
		return Event{}, err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o640); err == nil {
		_, err = temporary.Write(data)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return Event{}, err
	}
	if err := os.Rename(name, filepath.Join(dir, event.ID+".json")); err != nil {
		return Event{}, err
	}
	return event, nil
}

func (s *Store) List(repositoryID string) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, repositoryID))
	if errors.Is(err, fs.ErrNotExist) {
		return []Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.root, repositoryID, entry.Name()))
		if err != nil {
			return nil, err
		}
		var event Event
		if json.Unmarshal(data, &event) != nil || event.RepositoryID != repositoryID || event.ID == "" || event.ActorID == "" || event.Type == "" || event.Resource.ID == "" || event.CreatedAt.IsZero() {
			return nil, errors.New("invalid stored activity")
		}
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].CreatedAt.Equal(events[j].CreatedAt) {
			return events[i].ID > events[j].ID
		}
		return events[i].CreatedAt.After(events[j].CreatedAt)
	})
	return events, nil
}
