package extensions

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
)

const DeliverySchemaVersion = 1

type Delivery struct {
	ID               string            `json:"id"`
	InstallationID   string            `json:"installation_id"`
	EventID          string            `json:"event_id"`
	EventType        string            `json:"event_type"`
	SchemaVersion    int               `json:"schema_version"`
	RepositoryID     string            `json:"repository_id"`
	OrderingID       int64             `json:"ordering_id"`
	Payload          json.RawMessage   `json:"payload"`
	PayloadSHA256    string            `json:"payload_sha256"`
	Status           string            `json:"status"`
	Attempts         []DeliveryAttempt `json:"attempts"`
	CreatedAt        time.Time         `json:"created_at"`
	NextAttemptAt    *time.Time        `json:"next_attempt_at,omitempty"`
	DeadLetterReason string            `json:"dead_letter_reason,omitempty"`
}

type DeliveryAttempt struct {
	Sequence    int       `json:"sequence"`
	StatusCode  int       `json:"status_code,omitempty"`
	Outcome     string    `json:"outcome"`
	Error       string    `json:"error,omitempty"`
	AttemptedAt time.Time `json:"attempted_at"`
}
type Envelope struct {
	SchemaVersion int                 `json:"schema_version"`
	DeliveryID    string              `json:"delivery_id"`
	EventID       string              `json:"event_id"`
	EventType     string              `json:"event_type"`
	RepositoryID  string              `json:"repository_id"`
	OrderingID    int64               `json:"ordering_id"`
	OccurredAt    time.Time           `json:"occurred_at"`
	ActorID       string              `json:"actor_id"`
	Resource      activities.Resource `json:"resource"`
	Changes       map[string]string   `json:"changes,omitempty"`
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func resourceGrant(t string) string {
	switch t {
	case "pull_request":
		return "pull_requests"
	case "check_run", "check":
		return "checks"
	case "release":
		return "releases"
	case "deployment":
		return "deployments"
	case "incident":
		return "incidents"
	case "issue":
		return "issues"
	case "proposal", "task":
		return "tasks"
	default:
		return "repository"
	}
}

// Reconcile creates at most one immutable delivery per source event and installation.
// Events outside the current grant are deliberately indistinguishable from absent events.
func (s *Store) Reconcile(repo, installation string, source []activities.Event) ([]Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return nil, err
	}
	for n := range d.Installations {
		i := &d.Installations[n]
		if i.ID != installation || i.RepositoryID != repo {
			continue
		}
		if i.Status == "active" {
			sort.Slice(source, func(a, b int) bool {
				if source[a].CreatedAt.Equal(source[b].CreatedAt) {
					return source[a].ID < source[b].ID
				}
				return source[a].CreatedAt.Before(source[b].CreatedAt)
			})
			seen := map[string]bool{}
			for _, v := range i.Deliveries {
				seen[v.EventID] = true
			}
			for _, e := range source {
				if e.RepositoryID != repo || seen[e.ID] || !contains(i.EventTypes, e.Type) || !contains(i.ResourceTypes, resourceGrant(e.Resource.Type)) {
					continue
				}
				sequence := int64(len(i.Deliveries) + 1)
				deliveryID := id("dlv")
				env := Envelope{SchemaVersion: DeliverySchemaVersion, DeliveryID: deliveryID, EventID: e.ID, EventType: e.Type, RepositoryID: repo, OrderingID: sequence, OccurredAt: e.CreatedAt, ActorID: e.ActorID, Resource: e.Resource, Changes: redact(e.Metadata)}
				body, _ := json.Marshal(env)
				sum := sha256.Sum256(body)
				i.Deliveries = append(i.Deliveries, Delivery{ID: deliveryID, InstallationID: i.ID, EventID: e.ID, EventType: e.Type, SchemaVersion: DeliverySchemaVersion, RepositoryID: repo, OrderingID: sequence, Payload: body, PayloadSHA256: hex.EncodeToString(sum[:]), Status: "pending", CreatedAt: s.now().UTC()})
			}
		}
		return append([]Delivery(nil), i.Deliveries...), s.save(d)
	}
	return nil, ErrNotFound
}

func redact(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		low := strings.ToLower(k)
		if strings.Contains(low, "token") || strings.Contains(low, "secret") || strings.Contains(low, "password") || strings.Contains(low, "credential") {
			out[k] = "[REDACTED]"
		} else if len(v) > 1000 {
			out[k] = v[:1000]
		} else {
			out[k] = v
		}
	}
	return out
}
func Sign(key string, timestamp time.Time, body []byte) string {
	mac := hmac.New(sha256.New, []byte(key))
	fmt.Fprintf(mac, "%d.", timestamp.Unix())
	mac.Write(body)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

func (s *Store) DeliveryContext(repo, installation string) (Installation, Extension, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Installation{}, Extension{}, "", e
	}
	for _, i := range d.Installations {
		if i.ID == installation && i.RepositoryID == repo {
			for _, x := range d.Extensions {
				if x.ID == i.ExtensionID {
					return i, x, d.SigningKeys[i.ID], nil
				}
			}
			return Installation{}, Extension{}, "", ErrNotFound
		}
	}
	return Installation{}, Extension{}, "", ErrNotFound
}

func (s *Store) RecordAttempt(repo, installation, delivery, outcome string, code int, message string, retry bool) (Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Delivery{}, e
	}
	for n := range d.Installations {
		i := &d.Installations[n]
		if i.ID != installation || i.RepositoryID != repo {
			continue
		}
		for j := range i.Deliveries {
			v := &i.Deliveries[j]
			if v.ID != delivery {
				continue
			}
			if i.Status != "active" {
				return Delivery{}, ErrForbidden
			}
			if v.Status == "delivered" && !retry {
				return *v, nil
			}
			now := s.now().UTC()
			v.Attempts = append(v.Attempts, DeliveryAttempt{Sequence: len(v.Attempts) + 1, StatusCode: code, Outcome: outcome, Error: message, AttemptedAt: now})
			if outcome == "delivered" {
				v.Status = "delivered"
				v.NextAttemptAt = nil
			} else if len(v.Attempts) >= 5 {
				v.Status = "dead_letter"
				v.DeadLetterReason = message
				v.NextAttemptAt = nil
			} else {
				v.Status = "retrying"
				next := now.Add(time.Duration(1<<min(len(v.Attempts), 6)) * time.Minute)
				v.NextAttemptAt = &next
			}
			return *v, s.save(d)
		}
	}
	return Delivery{}, ErrNotFound
}
