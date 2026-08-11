package issues

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

const ReproductionManifestPath = ".komodo/reproductions.json"

type ReproductionResources struct {
	CPUSeconds int `json:"cpu_seconds"`
	MemoryMB   int `json:"memory_mb"`
	DiskMB     int `json:"disk_mb"`
}
type ReproductionCommand struct {
	Name             string   `json:"name"`
	Command          string   `json:"command"`
	Directory        string   `json:"directory,omitempty"`
	TimeoutSeconds   int      `json:"timeout_seconds"`
	ExpectedExitCode int      `json:"expected_exit_code"`
	Artifacts        []string `json:"artifacts,omitempty"`
}
type ReproductionDefinition struct {
	Version       int                   `json:"version"`
	Environment   string                `json:"environment"`
	Tools         []string              `json:"tools,omitempty"`
	Setup         []string              `json:"setup,omitempty"`
	Resources     ReproductionResources `json:"resources"`
	Reproductions []ReproductionCommand `json:"reproductions"`
}
type ReproductionInput struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Content   string `json:"content"`
	SHA256    string `json:"sha256"`
}
type ReproductionEvent struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	Command   string    `json:"command,omitempty"`
	Stream    string    `json:"stream,omitempty"`
	Message   string    `json:"message,omitempty"`
	ExitCode  *int      `json:"exit_code,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type ReproductionArtifact struct {
	Path      string `json:"path"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	Content   string `json:"content"`
}
type ReproductionAttempt struct {
	ID               string                 `json:"id"`
	RepositoryID     string                 `json:"repository_id"`
	IssueID          string                 `json:"issue_id"`
	Revision         string                 `json:"revision"`
	ReleaseID        string                 `json:"release_id,omitempty"`
	ReleaseVersion   string                 `json:"release_version,omitempty"`
	CreatedByID      string                 `json:"created_by_id"`
	RerunOf          string                 `json:"rerun_of,omitempty"`
	Definition       ReproductionDefinition `json:"environment_definition"`
	DefinitionDigest string                 `json:"definition_digest"`
	Command          ReproductionCommand    `json:"reproduction_command"`
	Inputs           []ReproductionInput    `json:"sanitized_inputs"`
	State            string                 `json:"state"`
	Reproduced       bool                   `json:"reproduced"`
	ObservedResult   string                 `json:"observed_result,omitempty"`
	FailureReason    string                 `json:"failure_reason,omitempty"`
	Events           []ReproductionEvent    `json:"events"`
	Artifacts        []ReproductionArtifact `json:"artifacts"`
	CreatedAt        time.Time              `json:"created_at"`
	CompletedAt      *time.Time             `json:"completed_at,omitempty"`
}

type repositoryOpener interface {
	Open(storage.ID) (*storage.Repository, error)
}
type ReproductionRunner struct {
	store        *Store
	repositories repositoryOpener
}

func NewReproductionRunner(store *Store, repositories repositoryOpener) *ReproductionRunner {
	return &ReproductionRunner{store: store, repositories: repositories}
}

func (r *ReproductionRunner) Definition(repositoryID, revision string) (ReproductionDefinition, string, error) {
	repo, err := r.repositories.Open(storage.ID(repositoryID))
	if err != nil {
		return ReproductionDefinition{}, "", err
	}
	raw, err := readRevisionFile(repo, storage.ObjectID(revision), ReproductionManifestPath)
	if err != nil {
		return ReproductionDefinition{}, "", err
	}
	var definition ReproductionDefinition
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&definition); err != nil {
		return definition, "", err
	}
	if err = validateReproductionDefinition(definition); err != nil {
		return definition, "", err
	}
	sum := sha256.Sum256(raw)
	return definition, hex.EncodeToString(sum[:]), nil
}

func validateReproductionDefinition(d ReproductionDefinition) error {
	if d.Version != 1 || strings.TrimSpace(d.Environment) == "" || len(d.Environment) > 500 || len(d.Tools) > 50 || len(d.Setup) > 20 || len(d.Reproductions) == 0 || len(d.Reproductions) > 50 || d.Resources.CPUSeconds < 1 || d.Resources.CPUSeconds > 600 || d.Resources.MemoryMB < 128 || d.Resources.MemoryMB > 8192 || d.Resources.DiskMB < 128 || d.Resources.DiskMB > 5120 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, c := range d.Reproductions {
		clean := filepath.Clean(c.Directory)
		if strings.TrimSpace(c.Name) == "" || seen[c.Name] || strings.TrimSpace(c.Command) == "" || len(c.Command) > 4000 || c.TimeoutSeconds < 1 || c.TimeoutSeconds > 600 || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || len(c.Artifacts) > 20 {
			return ErrInvalid
		}
		seen[c.Name] = true
		for _, artifact := range c.Artifacts {
			if !safeRelative(artifact) {
				return ErrInvalid
			}
		}
	}
	for _, command := range d.Setup {
		if strings.TrimSpace(command) == "" || len(command) > 4000 {
			return ErrInvalid
		}
	}
	return nil
}

func sanitizeInputs(inputs []ReproductionInput) ([]ReproductionInput, error) {
	if len(inputs) > 10 {
		return nil, ErrInvalid
	}
	total := 0
	out := make([]ReproductionInput, len(inputs))
	seen := map[string]bool{}
	secretNames := []string{"token", "secret", "password", "credential", "private_key", ".env", "id_rsa"}
	secretValues := []string{"-----begin private key", "ghp_", "github_pat_", "sk-", "authorization: bearer", "aws_secret_access_key"}
	for i, input := range inputs {
		name := filepath.ToSlash(filepath.Clean(strings.TrimSpace(input.Name)))
		lowerName := strings.ToLower(name)
		if !safeRelative(name) || seen[name] || (input.MediaType != "text/plain" && input.MediaType != "application/json" && input.MediaType != "application/octet-stream") {
			return nil, ErrInvalid
		}
		for _, marker := range secretNames {
			if strings.Contains(lowerName, marker) {
				return nil, ErrInvalid
			}
		}
		decoded, err := base64.StdEncoding.DecodeString(input.Content)
		if err != nil || len(decoded) > 1<<20 {
			return nil, ErrInvalid
		}
		if containsSecret(decoded, secretValues) {
			return nil, ErrInvalid
		}
		total += len(decoded)
		if total > 5<<20 {
			return nil, ErrInvalid
		}
		sum := sha256.Sum256(decoded)
		input.Name, input.SHA256 = name, hex.EncodeToString(sum[:])
		out[i] = input
		seen[name] = true
	}
	return out, nil
}

func safeRelative(path string) bool {
	clean := filepath.Clean(path)
	return path != "" && clean != "." && !filepath.IsAbs(clean) && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator)) && !strings.ContainsRune(path, '\x00')
}

func (s *Store) CreateReproduction(issue Issue, revision, releaseID, releaseVersion, actor, rerunOf string, definition ReproductionDefinition, digest string, command ReproductionCommand, inputs []ReproductionInput) (ReproductionAttempt, error) {
	inputs, err := sanitizeInputs(inputs)
	if err != nil {
		return ReproductionAttempt{}, err
	}
	id, err := newID()
	if err != nil {
		return ReproductionAttempt{}, err
	}
	now := s.now().UTC()
	a := ReproductionAttempt{ID: id, RepositoryID: issue.RepositoryID, IssueID: issue.ID, Revision: revision, ReleaseID: releaseID, ReleaseVersion: releaseVersion, CreatedByID: actor, RerunOf: rerunOf, Definition: definition, DefinitionDigest: digest, Command: command, Inputs: inputs, State: "queued", Events: []ReproductionEvent{}, Artifacts: []ReproductionArtifact{}, CreatedAt: now}
	s.mu.Lock()
	defer s.mu.Unlock()
	return a, s.writeReproduction(a)
}
func (s *Store) GetReproduction(repo, issueID, id string) (ReproductionAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readReproduction(repo, issueID, id)
}
func (s *Store) ListReproductions(repo, issueID string) ([]ReproductionAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, repo, issueID, "reproductions"))
	if errors.Is(err, fs.ErrNotExist) {
		return []ReproductionAttempt{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []ReproductionAttempt{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		a, er := s.readReproduction(repo, issueID, strings.TrimSuffix(entry.Name(), ".json"))
		if er != nil {
			return nil, er
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) mutateReproduction(repo, issueID, id string, fn func(*ReproductionAttempt)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.readReproduction(repo, issueID, id)
	if err != nil {
		return err
	}
	fn(&a)
	return s.writeReproduction(a)
}
func (s *Store) readReproduction(repo, issueID, id string) (ReproductionAttempt, error) {
	data, err := os.ReadFile(filepath.Join(s.root, repo, issueID, "reproductions", id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return ReproductionAttempt{}, ErrNotFound
	}
	if err != nil {
		return ReproductionAttempt{}, err
	}
	var a ReproductionAttempt
	if json.Unmarshal(data, &a) != nil || a.ID != id || a.IssueID != issueID || a.RepositoryID != repo {
		return ReproductionAttempt{}, ErrNotFound
	}
	return a, nil
}
func (s *Store) writeReproduction(a ReproductionAttempt) error {
	dir := filepath.Join(s.root, a.RepositoryID, a.IssueID, "reproductions")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".attempt-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0640)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, a.ID+".json"))
}

func (r *ReproductionRunner) Start(a ReproductionAttempt) { go r.execute(a) }
func (r *ReproductionRunner) execute(a ReproductionAttempt) {
	repo, err := r.repositories.Open(storage.ID(a.RepositoryID))
	root, mkErr := os.MkdirTemp("", "komodo-reproduction-*")
	if err == nil {
		err = mkErr
	}
	if root != "" {
		defer os.RemoveAll(root)
	}
	if err == nil {
		var commit storage.Commit
		commit, err = repo.ReadCommit(storage.ObjectID(a.Revision))
		if err == nil {
			err = materializeRevision(repo, commit.Tree, root)
		}
	}
	if err == nil {
		for _, input := range a.Inputs {
			var data []byte
			data, err = base64.StdEncoding.DecodeString(input.Content)
			if err != nil {
				break
			}
			path := filepath.Join(root, ".komodo-inputs", filepath.FromSlash(input.Name))
			if err = os.MkdirAll(filepath.Dir(path), 0750); err == nil {
				err = os.WriteFile(path, data, 0640)
			}
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		r.complete(a, "failed", false, "environment setup failed", err.Error(), nil)
		return
	}
	commands := append([]string{}, a.Definition.Setup...)
	commands = append(commands, a.Command.Command)
	for index, command := range commands {
		code, runErr := r.run(a, root, command, index == len(commands)-1)
		if runErr != nil {
			reason := "setup command failed"
			if index == len(commands)-1 {
				reason = "reproduction command failed"
			}
			r.complete(a, "failed", false, reason, fmt.Sprintf("exit code %d", code), nil)
			return
		}
	}
	artifacts, artifactErr := collectArtifacts(root, a.Command.Artifacts)
	if artifactErr != nil {
		r.complete(a, "failed", false, "declared artifact unavailable", artifactErr.Error(), nil)
		return
	}
	reproduced := true
	r.complete(a, "completed", reproduced, "", fmt.Sprintf("command exited with expected code %d", a.Command.ExpectedExitCode), artifacts)
}
func (r *ReproductionRunner) run(a ReproductionAttempt, root, command string, isReproduction bool) (int, error) {
	_ = r.append(a, ReproductionEvent{Type: "command", Command: command})
	timeout := a.Command.TimeoutSeconds
	if !isReproduction {
		timeout = min(timeout, 120)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	dir := "/workspace"
	if isReproduction && a.Command.Directory != "" {
		dir += "/" + filepath.ToSlash(filepath.Clean(a.Command.Directory))
	}
	args := []string{"--unshare-all", "--die-with-parent", "--new-session", "--clearenv", "--ro-bind", "/usr", "/usr", "--ro-bind", "/bin", "/bin", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64", "--ro-bind", "/etc", "/etc", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--bind", root, "/workspace", "--chdir", dir, "--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin", "--setenv", "HOME", "/tmp", "--setenv", "KOMODO_COMMIT", a.Revision, "--setenv", "KOMODO_INPUT_DIR", "/workspace/.komodo-inputs", "/bin/sh", "-c", "ulimit -t " + strconv.Itoa(a.Definition.Resources.CPUSeconds) + "; ulimit -v " + strconv.Itoa(a.Definition.Resources.MemoryMB*1024) + "; ulimit -f " + strconv.Itoa(a.Definition.Resources.DiskMB*2048) + "; " + command}
	cmd := exec.CommandContext(ctx, "bwrap", args...)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	err := cmd.Start()
	if err == nil {
		done := make(chan bool, 2)
		go r.capture(a, stdout, "stdout", done)
		go r.capture(a, stderr, "stderr", done)
		err = cmd.Wait()
		<-done
		<-done
	}
	code := 0
	if err != nil {
		code = -1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		}
	}
	_ = r.append(a, ReproductionEvent{Type: "outcome", Command: command, ExitCode: &code})
	if isReproduction && code == a.Command.ExpectedExitCode {
		return code, nil
	}
	if isReproduction && err == nil {
		return code, errors.New("unexpected reproduction exit code")
	}
	return code, err
}
func (r *ReproductionRunner) capture(a ReproductionAttempt, reader io.Reader, stream string, done chan<- bool) {
	defer func() { done <- true }()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 16<<10), 1<<20)
	for scanner.Scan() {
		message := scanner.Text()
		if containsSecret([]byte(message), []string{"-----begin private key", "ghp_", "github_pat_", "sk-", "authorization: bearer", "aws_secret_access_key"}) {
			message = "[redacted credential-like output]"
		}
		_ = r.append(a, ReproductionEvent{Type: "log", Stream: stream, Message: message})
	}
}
func (r *ReproductionRunner) append(a ReproductionAttempt, event ReproductionEvent) error {
	return r.store.mutateReproduction(a.RepositoryID, a.IssueID, a.ID, func(current *ReproductionAttempt) {
		if len(current.Events) >= 2000 {
			return
		}
		event.Sequence = int64(len(current.Events) + 1)
		event.CreatedAt = time.Now().UTC()
		current.Events = append(current.Events, event)
		current.State = "running"
	})
}
func (r *ReproductionRunner) complete(a ReproductionAttempt, state string, reproduced bool, reason, result string, artifacts []ReproductionArtifact) {
	now := time.Now().UTC()
	_ = r.store.mutateReproduction(a.RepositoryID, a.IssueID, a.ID, func(current *ReproductionAttempt) {
		current.State = state
		current.Reproduced = reproduced
		current.FailureReason = reason
		current.ObservedResult = result
		current.Artifacts = artifacts
		current.CompletedAt = &now
	})
}
func collectArtifacts(root string, paths []string) ([]ReproductionArtifact, error) {
	out := []ReproductionArtifact{}
	total := int64(0)
	for _, name := range paths {
		path := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
			return nil, ErrInvalid
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		total += int64(len(data))
		if total > 5<<20 {
			return nil, ErrInvalid
		}
		if containsSecret(data, []string{"-----begin private key", "ghp_", "github_pat_", "sk-", "authorization: bearer", "aws_secret_access_key"}) {
			return nil, ErrInvalid
		}
		sum := sha256.Sum256(data)
		out = append(out, ReproductionArtifact{Path: name, MediaType: "application/octet-stream", Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), Content: base64.StdEncoding.EncodeToString(data)})
	}
	return out, nil
}
func readRevisionFile(repo *storage.Repository, commitID storage.ObjectID, path string) ([]byte, error) {
	commit, err := repo.ReadCommit(commitID)
	if err != nil {
		return nil, err
	}
	treeID := commit.Tree
	for i, part := range strings.Split(path, "/") {
		tree, er := repo.ReadTree(treeID)
		if er != nil {
			return nil, er
		}
		found := false
		for _, entry := range tree.Entries {
			if entry.Name != part {
				continue
			}
			found = true
			if i == len(strings.Split(path, "/"))-1 {
				if entry.Type != storage.BlobObject {
					return nil, ErrInvalid
				}
				object, er := repo.ReadObject(entry.ObjectID)
				if er != nil {
					return nil, er
				}
				return object.Content, nil
			}
			if entry.Type != storage.TreeObject {
				return nil, ErrInvalid
			}
			treeID = entry.ObjectID
			break
		}
		if !found {
			return nil, fs.ErrNotExist
		}
	}
	return nil, fs.ErrNotExist
}
func materializeRevision(repo *storage.Repository, treeID storage.ObjectID, root string) error {
	tree, err := repo.ReadTree(treeID)
	if err != nil {
		return err
	}
	for _, entry := range tree.Entries {
		if entry.Name == "." || entry.Name == ".." || strings.ContainsAny(entry.Name, "/\x00") {
			return ErrInvalid
		}
		path := filepath.Join(root, entry.Name)
		switch entry.Type {
		case storage.TreeObject:
			if err = os.Mkdir(path, 0750); err == nil {
				err = materializeRevision(repo, entry.ObjectID, path)
			}
		case storage.BlobObject:
			if entry.Mode == 0120000 {
				return ErrInvalid
			}
			var object storage.Object
			object, err = repo.ReadObject(entry.ObjectID)
			if err == nil {
				mode := os.FileMode(0640)
				if entry.Mode == 0100755 {
					mode = 0750
				}
				err = os.WriteFile(path, object.Content, mode)
			}
		default:
			err = ErrInvalid
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func containsSecret(data []byte, markers []string) bool {
	lower := strings.ToLower(string(data))
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
