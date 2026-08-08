// Package deployments owns governed release environments and their durable promotion history.
package deployments

import (
	"crypto/aes"
	"crypto/cipher"
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

var (
	ErrNotFound   = errors.New("deployment resource not found")
	ErrInvalid    = errors.New("invalid deployment resource")
	ErrConflict   = errors.New("deployment concurrency limit reached")
	ErrTransition = errors.New("invalid deployment transition")
)

type Environment struct {
	ID                string            `json:"id"`
	RepositoryID      string            `json:"repository_id"`
	Name              string            `json:"name"`
	Position          int               `json:"position"`
	Command           string            `json:"command"`
	Configuration     map[string]string `json:"configuration"`
	SecretNames       []string          `json:"secret_names"`
	RequiredApprovals int               `json:"required_approvals"`
	Concurrency       int               `json:"concurrency"`
	CreatedByID       string            `json:"created_by_id"`
	UpdatedByID       string            `json:"updated_by_id"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}
type EnvironmentInput struct {
	Name              string            `json:"name"`
	Position          int               `json:"position"`
	Command           string            `json:"command"`
	Configuration     map[string]string `json:"configuration"`
	Secrets           map[string]string `json:"secrets"`
	RequiredApprovals int               `json:"required_approvals"`
	Concurrency       int               `json:"concurrency"`
}
type HealthSignal struct {
	Name           string `json:"name"`
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}
type RolloutStage struct {
	Name    string         `json:"name"`
	Command string         `json:"command,omitempty"`
	Health  []HealthSignal `json:"health"`
}
type ManifestEnvironment struct {
	Name   string         `json:"name"`
	Stages []RolloutStage `json:"stages"`
}
type Manifest struct {
	Version      int                   `json:"version"`
	Environments []ManifestEnvironment `json:"environments"`
}
type Approval struct {
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	State     string    `json:"state,omitempty"`
	ActorID   string    `json:"actor_id,omitempty"`
	Stream    string    `json:"stream,omitempty"`
	Message   string    `json:"message,omitempty"`
	Stage     string    `json:"stage,omitempty"`
	Signal    string    `json:"signal,omitempty"`
	Outcome   string    `json:"outcome,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Deployment struct {
	ID             string     `json:"id"`
	RepositoryID   string     `json:"repository_id"`
	EnvironmentID  string     `json:"environment_id"`
	ReleaseID      string     `json:"release_id"`
	BuildRunID     string     `json:"build_run_id"`
	ArtifactID     string     `json:"artifact_id"`
	ArtifactPath   string     `json:"artifact_path"`
	ArtifactSHA256 string     `json:"artifact_sha256"`
	SourceCommitID string     `json:"source_commit_id"`
	State          string     `json:"state"`
	InitiatedByID  string     `json:"initiated_by_id"`
	Approvals      []Approval `json:"approvals"`
	Events         []Event    `json:"events"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CurrentStage   string     `json:"current_stage,omitempty"`
	DecisionByID   string     `json:"decision_by_id,omitempty"`
	DecisionReason string     `json:"decision_reason,omitempty"`
}
type CreateDeployment struct{ RepositoryID, EnvironmentID, ReleaseID, BuildRunID, ArtifactID, ArtifactPath, ArtifactSHA256, SourceCommitID, ActorID string }

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
	key  []byte
}

func New(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(abs, 0750); err != nil {
		return nil, err
	}
	keyPath := filepath.Join(abs, ".credentials.key")
	key, err := os.ReadFile(keyPath)
	if errors.Is(err, fs.ErrNotExist) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err == nil {
			err = os.WriteFile(keyPath, key, 0600)
		}
	}
	if err != nil || len(key) != 32 {
		return nil, errors.New("deployment credential key unavailable")
	}
	return &Store{root: abs, now: time.Now, key: key}, nil
}
func (s *Store) PutEnvironment(repositoryID, id, actor string, in EnvironmentInput) (Environment, error) {
	in.Name, in.Command = strings.TrimSpace(in.Name), strings.TrimSpace(in.Command)
	if repositoryID == "" || actor == "" || in.Name == "" || in.Command == "" || in.Position < 1 || in.RequiredApprovals < 0 || in.RequiredApprovals > 20 || in.Concurrency < 1 || in.Concurrency > 20 {
		return Environment{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	existing, err := s.readEnvironment(repositoryID, id)
	if id != "" && errors.Is(err, ErrNotFound) {
		return Environment{}, ErrNotFound
	}
	if id == "" {
		id, _ = newID()
		existing = Environment{ID: id, RepositoryID: repositoryID, CreatedByID: actor, CreatedAt: now}
	} else if err != nil {
		return Environment{}, err
	}
	secretNames := append([]string{}, existing.SecretNames...)
	secrets, _ := s.readSecrets(repositoryID, existing.ID)
	if secrets == nil {
		secrets = map[string]string{}
	}
	for k, v := range in.Secrets {
		k = strings.TrimSpace(k)
		if k == "" || v == "" {
			return Environment{}, ErrInvalid
		}
		secrets[k] = v
	}
	secretNames = secretNames[:0]
	for k := range secrets {
		secretNames = append(secretNames, k)
	}
	sort.Strings(secretNames)
	existing.Name, existing.Position, existing.Command, existing.Configuration, existing.SecretNames, existing.RequiredApprovals, existing.Concurrency, existing.UpdatedByID, existing.UpdatedAt = in.Name, in.Position, in.Command, nonNilMap(in.Configuration), secretNames, in.RequiredApprovals, in.Concurrency, actor, now
	if err = s.writeJSON(filepath.Join(s.root, repositoryID, "environments", existing.ID+".json"), existing); err != nil {
		return Environment{}, err
	}
	if err = s.writeSecrets(repositoryID, existing.ID, secrets); err != nil {
		return Environment{}, err
	}
	return existing, nil
}
func (s *Store) ListEnvironments(repositoryID string) ([]Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, repositoryID, "environments"))
	if errors.Is(err, fs.ErrNotExist) {
		return []Environment{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Environment{}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		v, er := s.readEnvironment(repositoryID, strings.TrimSuffix(e.Name(), ".json"))
		if er != nil {
			return nil, er
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Position == out[j].Position {
			return out[i].Name < out[j].Name
		}
		return out[i].Position < out[j].Position
	})
	return out, nil
}
func (s *Store) GetEnvironment(repositoryID, id string) (Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readEnvironment(repositoryID, id)
}
func (s *Store) Secrets(repositoryID, id string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readSecrets(repositoryID, id)
}
func (s *Store) Create(p CreateDeployment) (Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	env, err := s.readEnvironment(p.RepositoryID, p.EnvironmentID)
	if err != nil {
		return Deployment{}, err
	}
	all, _ := s.listDeployments(p.RepositoryID)
	active := 0
	for _, d := range all {
		if d.EnvironmentID == env.ID && (d.State == "pending" || d.State == "queued" || d.State == "running" || d.State == "paused") {
			active++
		}
	}
	if active >= env.Concurrency {
		return Deployment{}, ErrConflict
	}
	id, _ := newID()
	now := s.now().UTC()
	state := "queued"
	if env.RequiredApprovals > 0 {
		state = "pending"
	}
	d := Deployment{ID: id, RepositoryID: p.RepositoryID, EnvironmentID: p.EnvironmentID, ReleaseID: p.ReleaseID, BuildRunID: p.BuildRunID, ArtifactID: p.ArtifactID, ArtifactPath: p.ArtifactPath, ArtifactSHA256: p.ArtifactSHA256, SourceCommitID: p.SourceCommitID, State: state, InitiatedByID: p.ActorID, Approvals: []Approval{}, CreatedAt: now}
	d.append(Event{Type: "initiated", State: state, ActorID: p.ActorID, CreatedAt: now})
	return d, s.writeDeployment(d)
}
func (s *Store) Approve(repositoryID, id, actor string) (Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.readDeployment(repositoryID, id)
	if err != nil {
		return d, err
	}
	if d.State != "pending" {
		return d, ErrTransition
	}
	for _, a := range d.Approvals {
		if a.ActorID == actor {
			return d, ErrTransition
		}
	}
	now := s.now().UTC()
	d.Approvals = append(d.Approvals, Approval{actor, now})
	d.append(Event{Type: "approved", ActorID: actor, CreatedAt: now})
	env, _ := s.readEnvironment(repositoryID, d.EnvironmentID)
	if len(d.Approvals) >= env.RequiredApprovals {
		d.State = "queued"
		d.append(Event{Type: "status", State: "queued", ActorID: actor, CreatedAt: now})
	}
	return d, s.writeDeployment(d)
}
func (s *Store) Start(repositoryID, id string) (Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.readDeployment(repositoryID, id)
	if e != nil || d.State != "queued" {
		if e == nil {
			e = ErrTransition
		}
		return d, e
	}
	now := s.now().UTC()
	d.State = "running"
	d.StartedAt = &now
	d.append(Event{Type: "status", State: "running", CreatedAt: now})
	return d, s.writeDeployment(d)
}
func (s *Store) Log(repositoryID, id, stream, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.readDeployment(repositoryID, id)
	if e != nil {
		return e
	}
	if d.State != "running" && d.State != "paused" {
		return ErrTransition
	}
	d.append(Event{Type: "log", Stream: stream, Message: message, CreatedAt: s.now().UTC()})
	return s.writeDeployment(d)
}
func (s *Store) Stage(repositoryID, id, stage, eventType, signal, outcome, message string) (Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.readDeployment(repositoryID, id)
	if err != nil || (d.State != "running" && d.State != "paused") {
		if err == nil {
			err = ErrTransition
		}
		return d, err
	}
	now := s.now().UTC()
	d.CurrentStage = stage
	d.append(Event{Type: eventType, State: d.State, Stage: stage, Signal: signal, Outcome: outcome, Message: message, CreatedAt: now})
	return d, s.writeDeployment(d)
}
func (s *Store) Control(repositoryID, id, actor, action, reason string) (Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.readDeployment(repositoryID, id)
	if err != nil {
		return d, err
	}
	reason = strings.TrimSpace(reason)
	now := s.now().UTC()
	switch action {
	case "pause":
		if d.State != "running" {
			return d, ErrTransition
		}
		d.State = "paused"
	case "resume":
		if d.State != "paused" {
			return d, ErrTransition
		}
		d.State = "running"
	case "cancel":
		if d.State != "pending" && d.State != "queued" && d.State != "running" && d.State != "paused" {
			return d, ErrTransition
		}
		d.State = "canceled"
	case "fail":
		if d.State != "running" && d.State != "paused" {
			return d, ErrTransition
		}
		if reason == "" {
			return d, ErrInvalid
		}
		d.State = "failed"
	default:
		return d, ErrInvalid
	}
	d.DecisionByID, d.DecisionReason = actor, reason
	if d.State == "failed" || d.State == "canceled" {
		d.CompletedAt = &now
	}
	d.append(Event{Type: "rollout." + action, State: d.State, ActorID: actor, Message: reason, Stage: d.CurrentStage, CreatedAt: now})
	return d, s.writeDeployment(d)
}
func (s *Store) Complete(repositoryID, id string, success bool, message string) (Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.readDeployment(repositoryID, id)
	if e != nil || (d.State != "running" && d.State != "paused") {
		if e == nil {
			e = ErrTransition
		}
		return d, e
	}
	now := s.now().UTC()
	d.State = "failed"
	if success {
		d.State = "succeeded"
	}
	d.CompletedAt = &now
	d.append(Event{Type: "status", State: d.State, Message: message, CreatedAt: now})
	return d, s.writeDeployment(d)
}
func (s *Store) GetDeployment(repositoryID, id string) (Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readDeployment(repositoryID, id)
}
func (s *Store) ListDeployments(repositoryID string) ([]Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listDeployments(repositoryID)
}
func (s *Store) listDeployments(repositoryID string) ([]Deployment, error) {
	entries, e := os.ReadDir(filepath.Join(s.root, repositoryID, "deployments"))
	if errors.Is(e, fs.ErrNotExist) {
		return []Deployment{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Deployment{}
	for _, v := range entries {
		if filepath.Ext(v.Name()) == ".json" {
			d, er := s.readDeployment(repositoryID, strings.TrimSuffix(v.Name(), ".json"))
			if er != nil {
				return nil, er
			}
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (d *Deployment) append(e Event) {
	e.Sequence = int64(len(d.Events) + 1)
	d.Events = append(d.Events, e)
}
func (s *Store) readEnvironment(r, id string) (Environment, error) {
	var v Environment
	if readJSON(filepath.Join(s.root, r, "environments", id+".json"), &v) != nil || v.RepositoryID != r || v.ID != id {
		return v, ErrNotFound
	}
	return v, nil
}
func (s *Store) readDeployment(r, id string) (Deployment, error) {
	var v Deployment
	if readJSON(filepath.Join(s.root, r, "deployments", id+".json"), &v) != nil || v.RepositoryID != r || v.ID != id {
		return v, ErrNotFound
	}
	return v, nil
}
func (s *Store) writeDeployment(v Deployment) error {
	return s.writeJSON(filepath.Join(s.root, v.RepositoryID, "deployments", v.ID+".json"), v)
}
func readJSON(p string, v any) error {
	b, e := os.ReadFile(p)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func (s *Store) writeJSON(p string, v any) error {
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(filepath.Dir(p), ".tmp-")
	if e != nil {
		return e
	}
	n := tmp.Name()
	defer os.Remove(n)
	tmp.Chmod(0600)
	_, e = tmp.Write(append(b, '\n'))
	if e == nil {
		e = tmp.Close()
	} else {
		tmp.Close()
	}
	if e != nil {
		return e
	}
	return os.Rename(n, p)
}
func (s *Store) writeSecrets(r, id string, v map[string]string) error {
	plain, _ := json.Marshal(v)
	block, _ := aes.NewCipher(s.key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	sealed := gcm.Seal(nonce, nonce, plain, nil)
	return s.writeJSON(filepath.Join(s.root, r, "credentials", id+".json"), map[string]string{"ciphertext": hex.EncodeToString(sealed)})
}
func (s *Store) readSecrets(r, id string) (map[string]string, error) {
	var box map[string]string
	if e := readJSON(filepath.Join(s.root, r, "credentials", id+".json"), &box); e != nil {
		return nil, e
	}
	raw, e := hex.DecodeString(box["ciphertext"])
	if e != nil {
		return nil, e
	}
	block, _ := aes.NewCipher(s.key)
	gcm, _ := cipher.NewGCM(block)
	if len(raw) < gcm.NonceSize() {
		return nil, ErrInvalid
	}
	plain, e := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if e != nil {
		return nil, e
	}
	out := map[string]string{}
	e = json.Unmarshal(plain, &out)
	return out, e
}
func nonNilMap(v map[string]string) map[string]string {
	if v == nil {
		return map[string]string{}
	}
	return v
}
func ParseManifest(data []byte, environmentName string) ([]RolloutStage, error) {
	var manifest Manifest
	if json.Unmarshal(data, &manifest) != nil || manifest.Version != 1 || len(manifest.Environments) == 0 {
		return nil, ErrInvalid
	}
	for _, environment := range manifest.Environments {
		if strings.EqualFold(strings.TrimSpace(environment.Name), strings.TrimSpace(environmentName)) {
			if len(environment.Stages) == 0 {
				return nil, ErrInvalid
			}
			seenStages := map[string]bool{}
			for i := range environment.Stages {
				stage := &environment.Stages[i]
				stage.Name = strings.TrimSpace(stage.Name)
				if stage.Name == "" || seenStages[stage.Name] || len(stage.Health) == 0 {
					return nil, ErrInvalid
				}
				seenStages[stage.Name] = true
				seenSignals := map[string]bool{}
				for j := range stage.Health {
					signal := &stage.Health[j]
					signal.Name, signal.Command = strings.TrimSpace(signal.Name), strings.TrimSpace(signal.Command)
					if signal.Name == "" || signal.Command == "" || seenSignals[signal.Name] || signal.TimeoutSeconds < 0 || signal.TimeoutSeconds > 600 {
						return nil, ErrInvalid
					}
					if signal.TimeoutSeconds == 0 {
						signal.TimeoutSeconds = 60
					}
					seenSignals[signal.Name] = true
				}
			}
			return environment.Stages, nil
		}
	}
	return nil, ErrNotFound
}
func newID() (string, error) {
	b := make([]byte, 16)
	_, e := rand.Read(b)
	return hex.EncodeToString(b), e
}
