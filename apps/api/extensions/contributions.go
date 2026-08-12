package extensions

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"time"
)

const (
	MaxHourlyOperations  = 120
	MaxContributionBytes = 1 << 20
	MaxArtifactBytes     = 5 << 20
)

type Resource struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Revision string `json:"revision"`
}
type Annotation struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Message   string `json:"message"`
	Level     string `json:"level"`
}
type Artifact struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	URL       string `json:"url"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}
type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}
type ContributionInput struct {
	IdempotencyKey string       `json:"idempotency_key"`
	Resource       Resource     `json:"resource"`
	Kind           string       `json:"kind"`
	State          string       `json:"state,omitempty"`
	Title          string       `json:"title"`
	Body           string       `json:"body,omitempty"`
	Annotations    []Annotation `json:"annotations,omitempty"`
	Artifacts      []Artifact   `json:"artifacts,omitempty"`
	Links          []Link       `json:"links,omitempty"`
}
type Contribution struct {
	ID             string       `json:"id"`
	ExtensionID    string       `json:"extension_id"`
	InstallationID string       `json:"installation_id"`
	ActorType      string       `json:"actor_type"`
	IdempotencyKey string       `json:"idempotency_key"`
	Resource       Resource     `json:"resource"`
	Kind           string       `json:"kind"`
	State          string       `json:"state,omitempty"`
	Title          string       `json:"title"`
	Body           string       `json:"body,omitempty"`
	Annotations    []Annotation `json:"annotations,omitempty"`
	Artifacts      []Artifact   `json:"artifacts,omitempty"`
	Links          []Link       `json:"links,omitempty"`
	Trusted        bool         `json:"trusted"`
	PolicyEffect   string       `json:"policy_effect"`
	CreatedAt      time.Time    `json:"created_at"`
}
type ActionInputField struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
}
type ActionEffect struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
}
type ActionInput struct {
	IdempotencyKey string             `json:"idempotency_key"`
	Resource       Resource           `json:"resource"`
	Name           string             `json:"name"`
	Label          string             `json:"label"`
	Description    string             `json:"description"`
	Inputs         []ActionInputField `json:"inputs"`
	Effects        []ActionEffect     `json:"effects"`
}
type Invocation struct {
	ID        string            `json:"id"`
	ActionID  string            `json:"action_id"`
	ActorID   string            `json:"actor_id"`
	Resource  Resource          `json:"resource"`
	Inputs    map[string]string `json:"inputs"`
	Effects   []ActionEffect    `json:"effects"`
	Status    string            `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
}
type Action struct {
	ID             string             `json:"id"`
	ExtensionID    string             `json:"extension_id"`
	InstallationID string             `json:"installation_id"`
	IdempotencyKey string             `json:"idempotency_key"`
	Resource       Resource           `json:"resource"`
	Name           string             `json:"name"`
	Label          string             `json:"label"`
	Description    string             `json:"description"`
	Inputs         []ActionInputField `json:"inputs"`
	Effects        []ActionEffect     `json:"effects"`
	Invocations    []Invocation       `json:"invocations,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
}
type Usage struct {
	WindowStartedAt time.Time `json:"window_started_at"`
	Operations      int       `json:"operations"`
	Bytes           int64     `json:"bytes"`
}

func digestToken(v string) string { d := sha256.Sum256([]byte(v)); return hex.EncodeToString(d[:]) }
func (s *Store) IssueCredential(repo, installation, actor string) (Installation, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Installation{}, "", e
	}
	for n := range d.Installations {
		i := &d.Installations[n]
		if i.ID != installation || i.RepositoryID != repo {
			continue
		}
		if i.Status != "active" || actor == "" {
			return Installation{}, "", ErrForbidden
		}
		token := "vke_" + id("token")
		if d.Credentials == nil {
			d.Credentials = map[string]string{}
		}
		d.Credentials[i.ID] = digestToken(token)
		i.Version++
		i.Events = append(i.Events, InstallationEvent{Sequence: int64(len(i.Events) + 1), Type: "credential_rotated", ActorID: actor, CreatedAt: s.now().UTC()})
		return *i, token, s.save(d)
	}
	return Installation{}, "", ErrNotFound
}
func (s *Store) Authenticate(token string) (Installation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Installation{}, e
	}
	got := digestToken(token)
	for _, i := range d.Installations {
		want := d.Credentials[i.ID]
		if i.Status == "active" && want != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1 {
			return i, nil
		}
	}
	return Installation{}, ErrForbidden
}
func hasPermission(i Installation, p string) bool {
	for _, x := range i.Permissions {
		if x == p {
			return true
		}
	}
	return false
}
func validResource(r Resource) bool {
	return (r.Type == "repository" || r.Type == "pull_request" || r.Type == "issue" || r.Type == "release" || r.Type == "deployment" || r.Type == "incident" || r.Type == "task") && r.ID != "" && r.Revision != ""
}
func supportsResource(i Installation, r Resource) bool {
	want := map[string]string{"pull_request": "pull_requests", "issue": "issues", "release": "releases", "deployment": "deployments", "incident": "incidents", "task": "tasks", "repository": "repository"}[r.Type]
	for _, got := range i.ResourceTypes {
		if got == want {
			return true
		}
	}
	return false
}
func allowedKind(k string) bool {
	return k == "status" || k == "check" || k == "annotation" || k == "artifact" || k == "link" || k == "comment"
}
func resetUsage(i *Installation, now time.Time, cost int64) error {
	if i.Usage.WindowStartedAt.IsZero() || now.Sub(i.Usage.WindowStartedAt) >= time.Hour {
		i.Usage = Usage{WindowStartedAt: now}
	}
	if i.Usage.Operations >= MaxHourlyOperations || i.Usage.Bytes+cost > MaxArtifactBytes {
		return ErrForbidden
	}
	i.Usage.Operations++
	i.Usage.Bytes += cost
	return nil
}
func (s *Store) Publish(token string, in ContributionInput, currentRevision string) (Contribution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Contribution{}, e
	}
	got := digestToken(token)
	for n := range d.Installations {
		i := &d.Installations[n]
		if i.Status != "active" || subtle.ConstantTimeCompare([]byte(got), []byte(d.Credentials[i.ID])) != 1 {
			continue
		}
		for _, x := range i.Contributions {
			if x.IdempotencyKey == in.IdempotencyKey {
				return x, nil
			}
		}
		cost := int64(len(in.Title) + len(in.Body))
		for _, a := range in.Artifacts {
			cost += a.Size
		}
		if in.IdempotencyKey == "" || !validResource(in.Resource) || !supportsResource(*i, in.Resource) || in.Resource.Revision != currentRevision || !allowedKind(in.Kind) || len(in.Title) > 200 || len(in.Body) > MaxContributionBytes || cost > MaxArtifactBytes || ((in.Kind == "check" || in.Kind == "annotation") && !hasPermission(*i, "checks:write")) {
			return Contribution{}, ErrInvalid
		}
		now := s.now().UTC()
		if resetUsage(i, now, cost) != nil {
			return Contribution{}, ErrForbidden
		}
		c := Contribution{ID: id("extout"), ExtensionID: i.ExtensionID, InstallationID: i.ID, ActorType: "extension", IdempotencyKey: in.IdempotencyKey, Resource: in.Resource, Kind: in.Kind, State: in.State, Title: strings.TrimSpace(in.Title), Body: in.Body, Annotations: in.Annotations, Artifacts: in.Artifacts, Links: in.Links, Trusted: false, PolicyEffect: "advisory_only", CreatedAt: now}
		i.Contributions = append(i.Contributions, c)
		return c, s.save(d)
	}
	return Contribution{}, ErrForbidden
}
func (s *Store) DeclareAction(token string, in ActionInput, currentRevision string) (Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Action{}, e
	}
	got := digestToken(token)
	for n := range d.Installations {
		i := &d.Installations[n]
		if i.Status != "active" || subtle.ConstantTimeCompare([]byte(got), []byte(d.Credentials[i.ID])) != 1 {
			continue
		}
		for _, x := range i.Actions {
			if x.IdempotencyKey == in.IdempotencyKey {
				return x, nil
			}
		}
		if in.IdempotencyKey == "" || !validResource(in.Resource) || !supportsResource(*i, in.Resource) || in.Resource.Revision != currentRevision || in.Name == "" || in.Label == "" || len(in.Inputs) > 20 || len(in.Effects) == 0 {
			return Action{}, ErrInvalid
		}
		for _, effect := range in.Effects {
			if effect.Kind != "extension_request" && effect.Kind != "comment" && effect.Kind != "check" {
				return Action{}, ErrInvalid
			}
		}
		a := Action{ID: id("extact"), ExtensionID: i.ExtensionID, InstallationID: i.ID, IdempotencyKey: in.IdempotencyKey, Resource: in.Resource, Name: in.Name, Label: in.Label, Description: in.Description, Inputs: in.Inputs, Effects: in.Effects, CreatedAt: s.now().UTC()}
		i.Actions = append(i.Actions, a)
		return a, s.save(d)
	}
	return Action{}, ErrForbidden
}
func (s *Store) Invoke(repo, installation, action, actor, currentRevision string, inputs map[string]string) (Invocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Invocation{}, e
	}
	for n := range d.Installations {
		i := &d.Installations[n]
		if i.ID != installation || i.RepositoryID != repo || i.Status != "active" {
			continue
		}
		for a := range i.Actions {
			x := &i.Actions[a]
			if x.ID != action {
				continue
			}
			if x.Resource.Revision != currentRevision {
				return Invocation{}, ErrInvalid
			}
			for _, field := range x.Inputs {
				v, ok := inputs[field.Name]
				if field.Required && !ok {
					return Invocation{}, ErrInvalid
				}
				if len(v) > 2000 {
					return Invocation{}, ErrInvalid
				}
			}
			v := Invocation{ID: id("extinv"), ActionID: x.ID, ActorID: actor, Resource: x.Resource, Inputs: inputs, Effects: x.Effects, Status: "requested", CreatedAt: s.now().UTC()}
			x.Invocations = append(x.Invocations, v)
			return v, s.save(d)
		}
	}
	return Invocation{}, ErrNotFound
}
