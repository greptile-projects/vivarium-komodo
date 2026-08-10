package workspaces

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

const ManifestPath = ".komodo/workspaces.json"

type repositoryOpener interface {
	Open(storage.ID) (*storage.Repository, error)
}
type Runner struct {
	store        *Store
	repositories repositoryOpener
	mutationMu   sync.Mutex
}

func NewRunner(store *Store, repositories repositoryOpener) *Runner {
	return &Runner{store: store, repositories: repositories}
}

func (r *Runner) Definition(repositoryID, revision string) (Definition, string, error) {
	repo, err := r.repositories.Open(storage.ID(repositoryID))
	if err != nil {
		return Definition{}, "", err
	}
	raw, err := readFile(repo, storage.ObjectID(revision), ManifestPath)
	if err != nil {
		return Definition{}, "", err
	}
	var d Definition
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&d); err != nil {
		return d, "", err
	}
	if err = validate(d); err != nil {
		return d, "", err
	}
	return d, digest(raw), nil
}
func digest(raw []byte) string { sum := sha256.Sum256(raw); return fmt.Sprintf("%x", sum[:]) }
func validate(d Definition) error {
	if d.Version != 1 || len(d.Setup) == 0 || len(d.Setup) > 20 || len(d.Tools) > 50 || len(d.Dependencies) > 100 || len(d.Ports) > 20 {
		return errors.New("invalid workspace definition")
	}
	if d.Resources.CPUSeconds < 1 || d.Resources.CPUSeconds > 3600 || d.Resources.MemoryMB < 128 || d.Resources.MemoryMB > 16384 || d.Resources.DiskMB < 128 || d.Resources.DiskMB > 20480 || d.Resources.SetupTimeoutSeconds < 1 || d.Resources.SetupTimeoutSeconds > 3600 {
		return errors.New("invalid workspace resources")
	}
	for _, v := range d.Setup {
		if strings.TrimSpace(v) == "" || len(v) > 4000 {
			return errors.New("invalid setup command")
		}
	}
	for _, v := range d.Tools {
		if strings.TrimSpace(v.Name) == "" || strings.TrimSpace(v.Version) == "" || len(v.Name) > 100 || len(v.Version) > 100 {
			return errors.New("invalid tool")
		}
	}
	for _, v := range d.Dependencies {
		if strings.TrimSpace(v) == "" || len(v) > 300 {
			return errors.New("invalid dependency")
		}
	}
	seenPorts := map[int]bool{}
	for _, v := range d.Ports {
		if v.Number < 1 || v.Number > 65535 || seenPorts[v.Number] || strings.TrimSpace(v.Label) == "" || len(v.Label) > 100 {
			return errors.New("invalid preview port")
		}
		clean := filepath.Clean(v.Path)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return errors.New("invalid preview path")
		}
		seenPorts[v.Number] = true
	}
	return nil
}
func (r *Runner) Start(w Workspace) { go r.setup(w) }
func (r *Runner) setup(w Workspace) {
	repo, err := r.repositories.Open(storage.ID(w.RepositoryID))
	root := r.store.Environment(w.ID)
	if err == nil {
		err = os.Mkdir(root, 0o750)
	}
	if err == nil {
		var commit storage.Commit
		commit, err = repo.ReadCommit(storage.ObjectID(w.Revision))
		if err == nil {
			err = materialize(repo, commit.Tree, root)
		}
	}
	if err != nil {
		_, _ = r.store.Finish(w.ID, false, "materialize exact revision")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(w.Definition.Resources.SetupTimeoutSeconds)*time.Second)
	defer cancel()
	for _, commandText := range w.Definition.Setup {
		_ = r.store.Append(w.ID, Event{Type: "command", Command: commandText})
		args := []string{"--unshare-all", "--die-with-parent", "--new-session", "--clearenv", "--ro-bind", "/usr", "/usr", "--ro-bind", "/bin", "/bin", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64", "--ro-bind", "/etc", "/etc", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--bind", root, "/workspace", "--chdir", "/workspace", "--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin", "--setenv", "HOME", "/tmp", "--setenv", "KOMODO_COMMIT", w.Revision, "/bin/sh", "-c", "ulimit -t " + strconv.Itoa(w.Definition.Resources.CPUSeconds) + "; ulimit -v " + strconv.Itoa(w.Definition.Resources.MemoryMB*1024) + "; ulimit -f " + strconv.Itoa(w.Definition.Resources.DiskMB*2048) + "; " + commandText}
		cmd := exec.CommandContext(ctx, "bwrap", args...)
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		if err = cmd.Start(); err == nil {
			done := make(chan bool, 2)
			go r.capture(w.ID, "stdout", stdout, done)
			go r.capture(w.ID, "stderr", stderr, done)
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
		_ = r.store.Append(w.ID, Event{Type: "outcome", Command: commandText, ExitCode: &code})
		if err != nil {
			_, _ = r.store.Finish(w.ID, false, "setup command failed")
			return
		}
		if size, sizeErr := directorySize(root); sizeErr != nil || size > int64(w.Definition.Resources.DiskMB)<<20 {
			_, _ = r.store.Finish(w.ID, false, "workspace disk limit exceeded")
			return
		}
	}
	_, _ = r.store.Finish(w.ID, true, "exact-revision environment is ready")
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}
func (r *Runner) capture(id, stream string, pipe interface{ Read([]byte) (int, error) }, done chan<- bool) {
	defer func() { done <- true }()
	scan := bufio.NewScanner(pipe)
	buf := make([]byte, 16<<10)
	scan.Buffer(buf, 1<<20)
	for scan.Scan() {
		_ = r.store.Append(id, Event{Type: "log", Stream: stream, Message: scan.Text()})
	}
}
func readFile(repo *storage.Repository, commitID storage.ObjectID, path string) ([]byte, error) {
	commit, err := repo.ReadCommit(commitID)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(path, "/")
	treeID := commit.Tree
	for i, part := range parts {
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
			if i == len(parts)-1 {
				if entry.Type != storage.BlobObject {
					return nil, errors.New("not a file")
				}
				object, er := repo.ReadObject(entry.ObjectID)
				if er != nil {
					return nil, er
				}
				return object.Content, nil
			}
			if entry.Type != storage.TreeObject {
				return nil, errors.New("not a tree")
			}
			treeID = entry.ObjectID
			break
		}
		if !found {
			return nil, os.ErrNotExist
		}
	}
	return nil, os.ErrNotExist
}
func materialize(repo *storage.Repository, treeID storage.ObjectID, root string) error {
	tree, err := repo.ReadTree(treeID)
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
			if err = os.Mkdir(path, 0o750); err == nil {
				err = materialize(repo, entry.ObjectID, path)
			}
		case storage.BlobObject:
			if entry.Mode == 0o120000 {
				return errors.New("symlinks unsupported")
			}
			var object storage.Object
			object, err = repo.ReadObject(entry.ObjectID)
			if err == nil {
				mode := os.FileMode(0o640)
				if entry.Mode == 0o100755 {
					mode = 0o750
				}
				err = os.WriteFile(path, object.Content, mode)
			}
		default:
			err = fmt.Errorf("unsupported tree entry")
		}
		if err != nil {
			return err
		}
	}
	return nil
}
