package previews

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type opener interface {
	Open(storage.ID) (*storage.Repository, error)
}
type Runner struct {
	store        *Store
	repositories opener
}

func NewRunner(s *Store, r opener) *Runner { return &Runner{store: s, repositories: r} }
func (r *Runner) Definition(repo, revision string) (Definition, string, error) {
	h, e := r.repositories.Open(storage.ID(repo))
	if e != nil {
		return Definition{}, "", e
	}
	raw, e := readFile(h, storage.ObjectID(revision), ManifestPath)
	if e != nil {
		return Definition{}, "", e
	}
	var d Definition
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	e = dec.Decode(&d)
	if e == nil {
		e = validate(d)
	}
	sum := sha256.Sum256(raw)
	return d, fmt.Sprintf("%x", sum[:]), e
}
func validate(d Definition) error {
	r := d.Resources
	if d.Version != 1 || len(d.Build) > 20 || strings.TrimSpace(d.Start) == "" || len(d.Start) > 4000 || d.Port < 1 || d.Port > 65535 || len(d.Configuration) > 30 || r.CPUSeconds < 1 || r.CPUSeconds > 3600 || r.MemoryMB < 128 || r.MemoryMB > 16384 || r.DiskMB < 128 || r.DiskMB > 20480 || r.BuildTimeoutSeconds < 1 || r.BuildTimeoutSeconds > 3600 || r.LifetimeMinutes < 1 || r.LifetimeMinutes > 1440 {
		return errors.New("invalid preview definition")
	}
	seen := map[string]bool{}
	for _, c := range d.Build {
		if strings.TrimSpace(c) == "" || len(c) > 4000 {
			return errors.New("invalid build command")
		}
	}
	for _, k := range d.Configuration {
		if k == "" || len(k) > 100 || seen[k] || strings.Trim(k, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") != "" {
			return errors.New("invalid configuration name")
		}
		seen[k] = true
	}
	return nil
}
func (r *Runner) Start(p Preview) { go r.run(p) }
func (r *Runner) run(p Preview) {
	repo, e := r.repositories.Open(storage.ID(p.SourceRepositoryID))
	root := r.store.Environment(p.ID)
	if e == nil {
		e = os.MkdirAll(root, 0750)
	}
	if e == nil {
		var c storage.Commit
		c, e = repo.ReadCommit(storage.ObjectID(p.Revision))
		if e == nil {
			e = materialize(repo, c.Tree, root)
		}
	}
	if e != nil {
		_, _ = r.store.Transition(p.ID, "failed", "", "materialize exact revision", 0)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.Definition.Resources.BuildTimeoutSeconds)*time.Second)
	defer cancel()
	for _, command := range p.Definition.Build {
		if e = r.command(ctx, p, root, command, false, 0); e != nil {
			_, _ = r.store.Transition(p.ID, "failed", "", "build command failed", 0)
			return
		}
	}
	listener, er := netListen()
	if er != nil {
		_, _ = r.store.Transition(p.ID, "failed", "", "allocate preview port", 0)
		return
	}
	port := listener
	url := "/api/repositories/" + p.RepositoryID + "/pull-requests/" + p.PullRequestID + "/previews/" + p.ID + "/proxy/"
	ctx, stop := context.WithDeadline(context.Background(), p.ExpiresAt)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- r.command(ctx, p, root, p.Definition.Start, true, port) }()
	startup := time.NewTimer(15 * time.Second)
	defer startup.Stop()
	for {
		connection, dialErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if dialErr == nil {
			_ = connection.Close()
			_, _ = r.store.Transition(p.ID, "ready", url, "", port)
			break
		}
		select {
		case e = <-done:
			_, _ = r.store.Transition(p.ID, "failed", "", "preview process exited during startup", port)
			return
		case <-startup.C:
			stop()
			_, _ = r.store.Transition(p.ID, "failed", "", "preview did not become reachable", port)
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
	if e = <-done; e != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			_, _ = r.store.Transition(p.ID, "expired", url, "preview lifetime elapsed", port)
		} else {
			_, _ = r.store.Transition(p.ID, "failed", url, "preview process exited", port)
		}
	} else {
		_, _ = r.store.Transition(p.ID, "stopped", url, "preview process exited", port)
	}
}
func netListen() (int, error) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return 0, e
	}
	p := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return p, nil
}
func (r *Runner) command(ctx context.Context, p Preview, root, command string, serve bool, port int) error {
	_ = r.store.Append(p.ID, Event{Type: "command", Command: command})
	args := []string{"--unshare-user", "--unshare-pid", "--unshare-ipc", "--unshare-uts", "--die-with-parent", "--new-session", "--clearenv", "--ro-bind", "/usr", "/usr", "--ro-bind", "/bin", "/bin", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64", "--ro-bind", "/etc", "/etc", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--bind", root, "/workspace", "--chdir", "/workspace", "--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin", "--setenv", "HOME", "/tmp", "--setenv", "KOMODO_COMMIT", p.Revision, "--setenv", "PORT", strconv.Itoa(port), "--setenv", "KOMODO_DECLARED_PORT", strconv.Itoa(p.Definition.Port)}
	for k, v := range p.Configuration {
		args = append(args, "--setenv", k, v)
	}
	args = append(args, "/bin/sh", "-c", "ulimit -t "+strconv.Itoa(p.Definition.Resources.CPUSeconds)+"; ulimit -v "+strconv.Itoa(p.Definition.Resources.MemoryMB*1024)+"; ulimit -f "+strconv.Itoa(p.Definition.Resources.DiskMB*2048)+"; "+command)
	cmd := exec.CommandContext(ctx, "bwrap", args...)
	out, _ := cmd.StdoutPipe()
	errOut, _ := cmd.StderrPipe()
	if e := cmd.Start(); e != nil {
		return e
	}
	go r.capture(p.ID, "stdout", out)
	go r.capture(p.ID, "stderr", errOut)
	e := cmd.Wait()
	code := 0
	if e != nil {
		code = -1
		var x *exec.ExitError
		if errors.As(e, &x) {
			code = x.ExitCode()
		}
	}
	_ = r.store.Append(p.ID, Event{Type: "outcome", Command: command, ExitCode: &code})
	return e
}
func (r *Runner) capture(id, stream string, v io.Reader) {
	s := bufio.NewScanner(v)
	s.Buffer(make([]byte, 16<<10), 1<<20)
	for s.Scan() {
		_ = r.store.Append(id, Event{Type: "log", Stream: stream, Message: s.Text()})
	}
}
func readFile(repo *storage.Repository, commit storage.ObjectID, path string) ([]byte, error) {
	c, e := repo.ReadCommit(commit)
	if e != nil {
		return nil, e
	}
	tree := c.Tree
	parts := strings.Split(path, "/")
	for i, part := range parts {
		t, e := repo.ReadTree(tree)
		if e != nil {
			return nil, e
		}
		found := false
		for _, x := range t.Entries {
			if x.Name != part {
				continue
			}
			found = true
			if i == len(parts)-1 {
				o, e := repo.ReadObject(x.ObjectID)
				if e != nil || x.Type != storage.BlobObject {
					return nil, errors.New("not a file")
				}
				return o.Content, nil
			}
			tree = x.ObjectID
		}
		if !found {
			return nil, os.ErrNotExist
		}
	}
	return nil, os.ErrNotExist
}
func materialize(repo *storage.Repository, id storage.ObjectID, root string) error {
	t, e := repo.ReadTree(id)
	if e != nil {
		return e
	}
	for _, x := range t.Entries {
		p := filepath.Join(root, x.Name)
		if x.Type == storage.TreeObject {
			if e = os.Mkdir(p, 0750); e == nil {
				e = materialize(repo, x.ObjectID, p)
			}
		} else {
			o, er := repo.ReadObject(x.ObjectID)
			e = er
			if e == nil {
				mode := os.FileMode(0640)
				if x.Mode == 0o100755 {
					mode = 0750
				}
				e = os.WriteFile(p, o.Content, mode)
			}
		}
		if e != nil {
			return e
		}
	}
	return nil
}
