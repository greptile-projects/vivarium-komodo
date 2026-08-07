// Package inbox owns per-user clearance state for derived attention items.
package inbox

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

type Store struct {
	root string
	mu   sync.Mutex
}

var validID = regexp.MustCompile(`^[a-f0-9]{32}$`)

func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("inbox root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, err
	}
	return &Store{root: abs}, nil
}

func (s *Store) Cleared(userID string) (map[string]bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(s.root, userID+".json"))
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	if json.Unmarshal(data, &ids) != nil {
		return nil, errors.New("invalid inbox state")
	}
	result := make(map[string]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

func (s *Store) Clear(userID, eventID string) error {
	if userID == "" || !validID.MatchString(eventID) {
		return errors.New("invalid inbox item")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.root, userID+".json")
	var ids []string
	if data, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(data, &ids) != nil {
			return errors.New("invalid inbox state")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, id := range ids {
		if id == eventID {
			return nil
		}
	}
	ids = append(ids, eventID)
	data, _ := json.Marshal(ids)
	temporary, err := os.CreateTemp(s.root, ".inbox-*")
	if err != nil {
		return err
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
		return err
	}
	return os.Rename(name, path)
}
