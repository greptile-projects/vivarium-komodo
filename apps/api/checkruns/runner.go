package checkruns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

const ManifestPath = ".komodo/checks.json"

type repositoryOpener interface {
	Open(storage.ID) (*storage.Repository, error)
}

type Runner struct {
	store        *Store
	repositories repositoryOpener
}

func NewRunner(store *Store, repositories repositoryOpener) *Runner {
	return &Runner{store: store, repositories: repositories}
}

// Start discovers the manifest from the exact candidate commit and durably queues
// every declared check before executing it asynchronously.
func (r *Runner) Start(repositoryID, pullRequestID, commitID string) error {
	repository, err := r.repositories.Open(storage.ID(repositoryID))
	if err != nil {
		return err
	}
	definitions, err := readManifest(repository, storage.ObjectID(commitID))
	if err != nil {
		return err
	}
	for _, definition := range definitions {
		run, err := r.store.Create(repositoryID, pullRequestID, commitID, definition)
		if err != nil {
			return err
		}
		go r.execute(run.ID)
	}
	return nil
}

func readManifest(repository *storage.Repository, commitID storage.ObjectID) ([]Definition, error) {
	commit, err := repository.ReadCommit(commitID)
	if err != nil {
		return nil, err
	}
	entry, found, err := findEntry(repository, commit.Tree, strings.Split(ManifestPath, "/"))
	if err != nil || !found {
		return nil, err
	}
	object, err := repository.ReadObject(entry.ObjectID)
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Version int          `json:"version"`
		Checks  []Definition `json:"checks"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(object.Content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil || manifest.Version != 1 || len(manifest.Checks) == 0 || len(manifest.Checks) > 20 {
		return nil, errors.New("invalid check manifest")
	}
	names := map[string]bool{}
	for i := range manifest.Checks {
		d := &manifest.Checks[i]
		d.Name, d.Command, d.WorkingDirectory = strings.TrimSpace(d.Name), strings.TrimSpace(d.Command), strings.TrimSpace(d.WorkingDirectory)
		if d.TimeoutSeconds == 0 {
			d.TimeoutSeconds = 600
		}
		if d.Name == "" || len(d.Name) > 100 || names[d.Name] || d.Command == "" || len(d.Command) > 4000 || d.TimeoutSeconds < 1 || d.TimeoutSeconds > 1800 || !safeRelative(d.WorkingDirectory) || len(d.Environment) > 50 {
			return nil, errors.New("invalid check manifest")
		}
		names[d.Name] = true
		for key, value := range d.Environment {
			if key == "" || strings.ContainsAny(key, "=\x00") || len(key) > 100 || len(value) > 4000 {
				return nil, errors.New("invalid check manifest")
			}
		}
	}
	return manifest.Checks, nil
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
	run, err := r.store.Start(id)
	if err != nil {
		return
	}
	repository, err := r.repositories.Open(storage.ID(run.RepositoryID))
	if err != nil {
		_, _ = r.store.Complete(id, -1, "repository unavailable")
		return
	}
	dir, err := os.MkdirTemp("", "komodo-check-")
	if err != nil {
		_, _ = r.store.Complete(id, -1, "create isolated workspace")
		return
	}
	defer os.RemoveAll(dir)
	commit, err := repository.ReadCommit(storage.ObjectID(run.CommitID))
	if err == nil {
		err = materialize(repository, commit.Tree, dir)
	}
	if err != nil {
		_, _ = r.store.Complete(id, -1, "materialize exact revision")
		return
	}
	working := filepath.Join(dir, filepath.FromSlash(run.Definition.WorkingDirectory))
	if info, statErr := os.Stat(working); statErr != nil || !info.IsDir() {
		_, _ = r.store.Complete(id, -1, "working directory unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(run.Definition.TimeoutSeconds)*time.Second)
	defer cancel()
	sandboxWorking := "/workspace"
	if run.Definition.WorkingDirectory != "" {
		sandboxWorking += "/" + run.Definition.WorkingDirectory
	}
	args := []string{"--unshare-all", "--die-with-parent", "--new-session", "--clearenv", "--ro-bind", "/usr", "/usr", "--ro-bind", "/bin", "/bin", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64", "--ro-bind", "/etc", "/etc", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--bind", dir, "/workspace", "--chdir", sandboxWorking, "--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin", "--setenv", "HOME", "/tmp", "--setenv", "CI", "true", "--setenv", "KOMODO_COMMIT", run.CommitID}
	for key, value := range run.Definition.Environment {
		args = append(args, "--setenv", key, value)
	}
	args = append(args, "/bin/sh", "-c", run.Definition.Command)
	command := exec.CommandContext(ctx, "bwrap", args...)
	output, commandErr := command.CombinedOutput()
	exitCode, message := 0, ""
	if commandErr != nil {
		exitCode = -1
		var exit *exec.ExitError
		if errors.As(commandErr, &exit) {
			exitCode = exit.ExitCode()
		}
		if ctx.Err() != nil {
			message = "check timed out"
		} else {
			message = strings.TrimSpace(string(output))
			if len(message) > 2000 {
				message = message[len(message)-2000:]
			}
		}
	}
	_, _ = r.store.Complete(id, exitCode, message)
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
