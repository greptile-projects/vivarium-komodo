package federation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PullRequestEvent is an immutable, signed claim exchanged between instances.
// ActorSubject is deliberately remote identity, never a local principal.
type PullRequestEvent struct {
	SchemaVersion   int               `json:"schema_version"`
	ID              string            `json:"id"`
	IdempotencyKey  string            `json:"idempotency_key"`
	PullReference   string            `json:"pull_reference"`
	TargetReference string            `json:"target_reference"`
	SourceInstance  string            `json:"source_instance"`
	ActorSubject    string            `json:"actor_subject"`
	Kind            string            `json:"kind"`
	Revision        string            `json:"revision"`
	Body            string            `json:"body,omitempty"`
	State           string            `json:"state,omitempty"`
	Audience        string            `json:"audience"`
	Evidence        map[string]string `json:"evidence,omitempty"`
	OccurredAt      time.Time         `json:"occurred_at"`
	KeyID           string            `json:"key_id"`
	Signature       string            `json:"signature"`
	Verification    string            `json:"verification"`
	Current         bool              `json:"current"`
	ImportedAt      time.Time         `json:"imported_at,omitempty"`
}

func PullRequestEventBytes(v PullRequestEvent) []byte {
	v.KeyID, v.Signature, v.Verification, v.ImportedAt = "", "", "", time.Time{}
	v.Current = false
	b, _ := json.Marshal(v)
	return b
}

func validPullEvent(v PullRequestEvent) bool {
	allowed := map[string]bool{"discussion": true, "review": true, "revision": true, "check": true, "preview": true, "requested_changes": true, "closure": true}
	return v.SchemaVersion == 1 && v.IdempotencyKey != "" && len(v.IdempotencyKey) <= 200 && v.PullReference != "" && v.TargetReference != "" && v.SourceInstance != "" && v.ActorSubject != "" && allowed[v.Kind] && v.Revision != "" && len(v.Body) <= 65536 && (v.Audience == "public" || v.Audience == "participants") && len(v.Evidence) <= 32
}

// PutPullRequestEvent provides durable idempotency. Reusing a key with changed
// signed content is a conflict; exact replay returns the retained observation.
func (s *Store) PutPullRequestEvent(v PullRequestEvent) (PullRequestEvent, error) {
	if !validPullEvent(v) {
		return PullRequestEvent{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.root, "pull-request-events.json")
	items := []PullRequestEvent{}
	if b, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(b, &items) != nil {
			return PullRequestEvent{}, ErrInvalid
		}
	}
	digest := sha256.Sum256(PullRequestEventBytes(v))
	v.ID = "fpe_" + hex.EncodeToString(digest[:12])
	for _, item := range items {
		if item.SourceInstance == v.SourceInstance && item.IdempotencyKey == v.IdempotencyKey {
			if item.ID != v.ID {
				return PullRequestEvent{}, ErrConflict
			}
			return item, nil
		}
	}
	v.ImportedAt = s.now().UTC()
	items = append(items, v)
	b, _ := json.MarshalIndent(items, "", "  ")
	tmp, err := os.CreateTemp(s.root, "pull-events-*.tmp")
	if err != nil {
		return PullRequestEvent{}, err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		return PullRequestEvent{}, err
	}
	return v, nil
}

func (s *Store) PullRequestEvents(reference string) ([]PullRequestEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(filepath.Join(s.root, "pull-request-events.json"))
	if os.IsNotExist(err) {
		return []PullRequestEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := []PullRequestEvent{}
	if json.Unmarshal(b, &items) != nil {
		return nil, ErrInvalid
	}
	out := []PullRequestEvent{}
	for _, v := range items {
		if strings.TrimSpace(reference) == "" || v.PullReference == reference || v.TargetReference == reference {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OccurredAt.Before(out[j].OccurredAt) })
	return out, nil
}
