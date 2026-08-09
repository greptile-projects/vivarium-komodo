package checkruns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

const ManifestPath = ".komodo/checks.json"
const ReleaseManifestPath = ".komodo/releases.json"
const EvolutionManifestPath = ".komodo/evolution-checks.json"

type repositoryOpener interface {
	Open(storage.ID) (*storage.Repository, error)
}

type Runner struct {
	store        *Store
	repositories repositoryOpener
	mu           sync.Mutex
	cancels      map[string]context.CancelFunc
	onComplete   func(Run)
}

// SetCompletionHook connects terminal check evidence to workflow coordination.
// The hook runs after the terminal state is durable and must return quickly.
func (r *Runner) SetCompletionHook(hook func(Run)) { r.onComplete = hook }

func NewRunner(store *Store, repositories repositoryOpener) *Runner {
	return &Runner{store: store, repositories: repositories, cancels: map[string]context.CancelFunc{}}
}

// Rerun creates a distinct attempt from the exact definition and revision of
// an existing run. The initiating collaborator remains durable attribution.
func (r *Runner) Rerun(repositoryID, pullRequestID, runID, actorID string) (Run, error) {
	previous, err := r.store.Get(repositoryID, pullRequestID, runID)
	if err != nil {
		return Run{}, err
	}
	if previous.State == Queued || previous.State == Running {
		return Run{}, ErrInvalidTransition
	}
	run, err := r.store.createAttempt(repositoryID, previous.SourceRepositoryID, pullRequestID, previous.CommitID, previous.Definition, actorID, previous.ID)
	if err == nil {
		go r.execute(run.ID)
	}
	return run, err
}

func (r *Runner) Cancel(repositoryID, pullRequestID, runID, actorID string) (Run, error) {
	if _, err := r.store.Get(repositoryID, pullRequestID, runID); err != nil {
		return Run{}, err
	}
	run, err := r.store.Cancel(runID, actorID)
	if err != nil {
		return Run{}, err
	}
	r.mu.Lock()
	cancel := r.cancels[runID]
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return run, nil
}

// Start discovers the manifest from the exact candidate commit and durably queues
// every declared check before executing it asynchronously.
func (r *Runner) Start(repositoryID, sourceRepositoryID, pullRequestID, commitID string) error {
	repository, err := r.repositories.Open(storage.ID(sourceRepositoryID))
	if err != nil {
		return err
	}
	definitions, err := readManifest(repository, storage.ObjectID(commitID))
	if err != nil {
		return err
	}
	for _, definition := range definitions {
		run, err := r.store.CreateForSource(repositoryID, sourceRepositoryID, pullRequestID, commitID, definition)
		if err != nil {
			return err
		}
		go r.execute(run.ID)
	}
	return nil
}

// StartRelease queues the repository-defined release build steps captured at
// the release's exact source revision. The release ID is used as the durable
// execution namespace, so its evidence cannot be confused with PR checks.
func (r *Runner) StartRelease(repositoryID, releaseID, commitID, actorID string) ([]Run, error) {
	repository, err := r.repositories.Open(storage.ID(repositoryID))
	if err != nil {
		return nil, err
	}
	definitions, err := readReleaseManifest(repository, storage.ObjectID(commitID))
	if err != nil {
		return nil, err
	}
	runs := make([]Run, 0, len(definitions))
	for _, definition := range definitions {
		run, err := r.store.createAttempt(repositoryID, repositoryID, "release:"+releaseID, commitID, definition, actorID, "")
		if err != nil {
			return runs, err
		}
		runs = append(runs, run)
	}
	go r.executeRelease(runs)
	return runs, nil
}

// StartEvolution runs provider-defined checks against an exact provider and
// consumer revision matrix. The namespace keeps this evidence separate from
// ordinary pull-request and release checks.
func (r *Runner) StartEvolution(repositoryID, planID, attemptID, actorID string, revisions []Revision) ([]Run, error) {
	if len(revisions) < 2 || len(revisions) > 25 {
		return nil, errors.New("invalid evolution revision matrix")
	}
	seen := map[string]bool{}
	for _, revision := range revisions {
		if revision.RepositoryID == "" || revision.CommitID == "" || seen[revision.RepositoryID] {
			return nil, errors.New("invalid evolution revision matrix")
		}
		seen[revision.RepositoryID] = true
	}
	provider, err := r.repositories.Open(storage.ID(repositoryID))
	if err != nil {
		return nil, err
	}
	definitions, err := readEvolutionManifest(provider, storage.ObjectID(revisions[0].CommitID))
	if err != nil {
		return nil, err
	}
	runs := make([]Run, 0, len(definitions))
	for _, definition := range definitions {
		run, createErr := r.store.createAttempt(repositoryID, repositoryID, "evolution:"+planID+":"+attemptID, revisions[0].CommitID, definition, actorID, "")
		if createErr != nil {
			return runs, createErr
		}
		run.Revisions = append([]Revision{}, revisions...)
		if writeErr := r.store.write(run); writeErr != nil {
			return runs, writeErr
		}
		runs = append(runs, run)
		go r.execute(run.ID)
	}
	return runs, nil
}

func readEvolutionManifest(repository *storage.Repository, commitID storage.ObjectID) ([]Definition, error) {
	definitions, err := readDefinitions(repository, commitID, EvolutionManifestPath, "evolution_checks")
	if err != nil {
		return nil, err
	}
	for i := range definitions {
		definitions[i].Kind = strings.ToLower(strings.TrimSpace(definitions[i].Kind))
		if definitions[i].Kind != "contract" && definitions[i].Kind != "integration" {
			return nil, errors.New("invalid evolution check kind")
		}
	}
	return definitions, nil
}

func (r *Runner) ValidateRelease(repositoryID, commitID string) error {
	repository, err := r.repositories.Open(storage.ID(repositoryID))
	if err != nil {
		return err
	}
	_, err = readReleaseManifest(repository, storage.ObjectID(commitID))
	return err
}

func (r *Runner) executeRelease(runs []Run) {
	states := map[string]State{}
	for _, run := range runs {
		blocked := false
		for _, dependency := range run.Definition.Dependencies {
			if states[dependency] != Succeeded {
				blocked = true
				break
			}
		}
		if blocked {
			started, err := r.store.Start(run.ID)
			if err == nil {
				_, _ = r.store.Complete(started.ID, -1, false, "dependency failed")
			}
			states[run.Definition.Name] = Failed
			continue
		}
		r.execute(run.ID)
		completed, err := r.store.Get(run.RepositoryID, run.PullRequestID, run.ID)
		if err != nil {
			states[run.Definition.Name] = Failed
		} else {
			states[run.Definition.Name] = completed.State
		}
	}
}

func readManifest(repository *storage.Repository, commitID storage.ObjectID) ([]Definition, error) {
	return readDefinitions(repository, commitID, ManifestPath, "checks")
}

func readReleaseManifest(repository *storage.Repository, commitID storage.ObjectID) ([]Definition, error) {
	definitions, err := readDefinitions(repository, commitID, ReleaseManifestPath, "builds")
	if err != nil {
		return nil, err
	}
	if len(definitions) == 0 {
		return nil, errors.New("release manifest is required")
	}
	known := map[string]bool{}
	for i := range definitions {
		seen := map[string]bool{}
		for _, dependency := range definitions[i].Dependencies {
			if !known[dependency] || seen[dependency] {
				return nil, errors.New("invalid release manifest dependencies")
			}
			seen[dependency] = true
		}
		known[definitions[i].Name] = true
	}
	return definitions, nil
}

func readDefinitions(repository *storage.Repository, commitID storage.ObjectID, manifestPath, field string) ([]Definition, error) {
	commit, err := repository.ReadCommit(commitID)
	if err != nil {
		return nil, err
	}
	entry, found, err := findEntry(repository, commit.Tree, strings.Split(manifestPath, "/"))
	if err != nil || !found {
		return nil, err
	}
	object, err := repository.ReadObject(entry.ObjectID)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Version         int          `json:"version"`
		Checks          []Definition `json:"checks"`
		Builds          []Definition `json:"builds"`
		EvolutionChecks []Definition `json:"evolution_checks"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(object.Content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil || raw.Version != 1 {
		return nil, errors.New("invalid execution manifest")
	}
	definitions := raw.Checks
	if field == "builds" {
		definitions = raw.Builds
	} else if field == "evolution_checks" {
		definitions = raw.EvolutionChecks
	}
	if len(definitions) == 0 || len(definitions) > 20 || (field == "checks" && (len(raw.Builds) != 0 || len(raw.EvolutionChecks) != 0)) || (field == "builds" && (len(raw.Checks) != 0 || len(raw.EvolutionChecks) != 0)) || (field == "evolution_checks" && (len(raw.Checks) != 0 || len(raw.Builds) != 0)) {
		return nil, errors.New("invalid check manifest")
	}
	names := map[string]bool{}
	for i := range definitions {
		d := &definitions[i]
		d.Name, d.Command, d.WorkingDirectory = strings.TrimSpace(d.Name), strings.TrimSpace(d.Command), strings.TrimSpace(d.WorkingDirectory)
		if d.TimeoutSeconds == 0 {
			d.TimeoutSeconds = 600
		}
		if d.Name == "" || len(d.Name) > 100 || names[d.Name] || d.Command == "" || len(d.Command) > 4000 || d.TimeoutSeconds < 1 || d.TimeoutSeconds > 1800 || !safeRelative(d.WorkingDirectory) || len(d.Environment) > 50 {
			return nil, errors.New("invalid check manifest")
		}
		if len(d.Artifacts) > 20 {
			return nil, errors.New("invalid check manifest")
		}
		if len(d.Dependencies) > 20 {
			return nil, errors.New("invalid execution manifest")
		}
		if field != "builds" && len(d.Dependencies) != 0 {
			return nil, errors.New("check dependencies are unsupported")
		}
		for j := range d.Dependencies {
			d.Dependencies[j] = strings.TrimSpace(d.Dependencies[j])
			if d.Dependencies[j] == "" {
				return nil, errors.New("invalid execution manifest")
			}
		}
		seenArtifacts := map[string]bool{}
		for j, path := range d.Artifacts {
			path = strings.TrimSpace(path)
			if path == "" || len(path) > 500 || !safeRelative(path) || seenArtifacts[path] {
				return nil, errors.New("invalid check manifest")
			}
			d.Artifacts[j], seenArtifacts[path] = path, true
		}
		names[d.Name] = true
		for key, value := range d.Environment {
			if key == "" || strings.ContainsAny(key, "=\x00") || len(key) > 100 || len(value) > 4000 {
				return nil, errors.New("invalid check manifest")
			}
		}
	}
	return definitions, nil
}

func findEntry(repository *storage.Repository, treeID storage.ObjectID, parts []string) (storage.TreeEntry, bool, error) {
	tree, err := repository.ReadTree(treeID)
	if err != nil {
		return storage.TreeEntry{}, false, err
	}
	for _, entry := range tree.Entries {
		if entry.Name != parts[0] {
			continue
		}
		if len(parts) == 1 {
			return entry, entry.Type == storage.BlobObject, nil
		}
		if entry.Type != storage.TreeObject {
			return storage.TreeEntry{}, false, nil
		}
		return findEntry(repository, entry.ObjectID, parts[1:])
	}
	return storage.TreeEntry{}, false, nil
}

func (r *Runner) execute(id string) {
	rootContext, stop := context.WithCancel(context.Background())
	r.mu.Lock()
	r.cancels[id] = stop
	r.mu.Unlock()
	defer func() {
		stop()
		r.mu.Lock()
		delete(r.cancels, id)
		r.mu.Unlock()
	}()
	run, err := r.store.Start(id)
	if err != nil {
		return
	}
	dir, err := os.MkdirTemp("", "komodo-check-")
	if err != nil {
		_, _ = r.store.Complete(id, -1, false, "create isolated workspace")
		return
	}
	defer os.RemoveAll(dir)
	workingRoot := dir
	if len(run.Revisions) == 0 {
		repository, openErr := r.repositories.Open(storage.ID(run.SourceRepositoryID))
		err = openErr
		if err == nil {
			var commit storage.Commit
			commit, err = repository.ReadCommit(storage.ObjectID(run.CommitID))
			if err == nil {
				err = materialize(repository, commit.Tree, dir)
			}
		}
	} else {
		root := filepath.Join(dir, "repositories")
		err = os.Mkdir(root, 0o750)
		for _, revision := range run.Revisions {
			if err != nil {
				break
			}
			repository, openErr := r.repositories.Open(storage.ID(revision.RepositoryID))
			if openErr != nil {
				err = openErr
				break
			}
			commit, readErr := repository.ReadCommit(storage.ObjectID(revision.CommitID))
			if readErr != nil {
				err = readErr
				break
			}
			target := filepath.Join(root, revision.RepositoryID)
			if err = os.Mkdir(target, 0o750); err == nil {
				err = materialize(repository, commit.Tree, target)
			}
		}
		workingRoot = filepath.Join(root, run.Revisions[0].RepositoryID)
	}
	if err != nil {
		_, _ = r.store.Complete(id, -1, false, "materialize exact revision")
		return
	}
	working := filepath.Join(workingRoot, filepath.FromSlash(run.Definition.WorkingDirectory))
	if info, statErr := os.Stat(working); statErr != nil || !info.IsDir() {
		_, _ = r.store.Complete(id, -1, false, "working directory unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(rootContext, time.Duration(run.Definition.TimeoutSeconds)*time.Second)
	defer cancel()
	sandboxWorking := "/workspace"
	if len(run.Revisions) > 0 {
		sandboxWorking += "/repositories/" + run.Revisions[0].RepositoryID
	}
	if run.Definition.WorkingDirectory != "" {
		sandboxWorking += "/" + run.Definition.WorkingDirectory
	}
	args := []string{"--unshare-all", "--die-with-parent", "--new-session", "--clearenv", "--ro-bind", "/usr", "/usr", "--ro-bind", "/bin", "/bin", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64", "--ro-bind", "/etc", "/etc", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--bind", dir, "/workspace", "--chdir", sandboxWorking, "--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin", "--setenv", "HOME", "/tmp", "--setenv", "CI", "true", "--setenv", "KOMODO_COMMIT", run.CommitID}
	for key, value := range run.Definition.Environment {
		args = append(args, "--setenv", key, value)
	}
	args = append(args, "/bin/sh", "-c", run.Definition.Command)
	command := exec.CommandContext(ctx, "bwrap", args...)
	stdout, stdoutErr := command.StdoutPipe()
	stderr, stderrErr := command.StderrPipe()
	if stdoutErr != nil || stderrErr != nil {
		_, _ = r.store.Complete(id, -1, false, "capture command output")
		return
	}
	commandErr := command.Start()
	if commandErr == nil {
		done := make(chan struct{}, 2)
		go r.capture(id, "stdout", stdout, done)
		go r.capture(id, "stderr", stderr, done)
		commandErr = command.Wait()
		<-done
		<-done
	}
	exitCode, message := 0, ""
	if commandErr != nil {
		exitCode = -1
		var exit *exec.ExitError
		if errors.As(commandErr, &exit) {
			exitCode = exit.ExitCode()
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			message = "check timed out"
		} else if errors.Is(ctx.Err(), context.Canceled) {
			message = "check canceled"
		} else {
			message = "command failed"
		}
	}
	for _, artifactPath := range run.Definition.Artifacts {
		path := filepath.Join(workingRoot, filepath.FromSlash(artifactPath))
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > 25<<20 {
			continue
		}
		content, readErr := os.ReadFile(path)
		if readErr == nil {
			mediaType := mime.TypeByExtension(filepath.Ext(path))
			if mediaType == "" {
				mediaType = "application/octet-stream"
			}
			_, _ = r.store.AddArtifact(id, artifactPath, mediaType, content)
		}
	}
	completed, completeErr := r.store.Complete(id, exitCode, errors.Is(ctx.Err(), context.DeadlineExceeded), message)
	if completeErr == nil && r.onComplete != nil {
		r.onComplete(completed)
	}
}

func (r *Runner) capture(id, stream string, reader io.Reader, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
	buffer := make([]byte, 16<<10)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			_ = r.store.AppendLog(id, stream, string(buffer[:n]))
		}
		if err != nil {
			return
		}
	}
}

func materialize(repository *storage.Repository, treeID storage.ObjectID, root string) error {
	tree, err := repository.ReadTree(treeID)
	if err != nil {
		return err
	}
	for _, entry := range tree.Entries {
		if entry.Name == "." || entry.Name == ".." || strings.ContainsAny(entry.Name, "/\x00") {
			return errors.New("unsafe tree entry")
		}
		path := filepath.Join(root, entry.Name)
		switch entry.Type {
		case storage.TreeObject:
			if err := os.Mkdir(path, 0o750); err != nil {
				return err
			}
			if err := materialize(repository, entry.ObjectID, path); err != nil {
				return err
			}
		case storage.BlobObject:
			if entry.Mode == 0o120000 {
				return errors.New("symlinks are not materialized")
			}
			object, err := repository.ReadObject(entry.ObjectID)
			if err != nil {
				return err
			}
			mode := os.FileMode(0o640)
			if entry.Mode == 0o100755 {
				mode = 0o750
			}
			if err := os.WriteFile(path, object.Content, mode); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported tree entry")
		}
	}
	return nil
}

func safeRelative(path string) bool {
	if path == "" {
		return true
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	return clean != "." && clean != ".." && !filepath.IsAbs(clean) && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}
