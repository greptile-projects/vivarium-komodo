package workspaces

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrUnsafePath = errors.New("unsafe workspace path")

type File struct {
	Path      string `json:"path"`
	Directory bool   `json:"directory"`
	Size      int64  `json:"size,omitempty"`
	Content   string `json:"content,omitempty"`
	Binary    bool   `json:"binary,omitempty"`
	Digest    string `json:"digest,omitempty"`
}
type Match struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}
type CommandResult struct {
	Command     string    `json:"command"`
	Directory   string    `json:"directory"`
	ExitCode    int       `json:"exit_code"`
	Stdout      string    `json:"stdout"`
	Stderr      string    `json:"stderr"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

func (r *Runner) resolve(id, path string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "." {
		clean = ""
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.ContainsRune(clean, '\x00') {
		return "", ErrUnsafePath
	}
	root := r.store.Environment(id)
	target := filepath.Join(root, clean)
	current := root
	for _, component := range strings.Split(clean, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			break
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", ErrUnsafePath
		}
	}
	return target, nil
}

func (r *Runner) Files(w Workspace, path string) ([]File, error) {
	target, err := r.resolve(w.ID, path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		data, e := os.ReadFile(target)
		if e != nil {
			return nil, e
		}
		if len(data) > 1<<20 {
			data = data[:1<<20]
		}
		digest := sha256.Sum256(data)
		return []File{{Path: filepath.ToSlash(strings.TrimPrefix(target, r.store.Environment(w.ID)+string(filepath.Separator))), Size: info.Size(), Content: string(data), Binary: bytes.IndexByte(data, 0) >= 0, Digest: hex.EncodeToString(digest[:])}}, nil
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	out := make([]File, 0, len(entries))
	for _, entry := range entries {
		i, e := entry.Info()
		if e != nil {
			continue
		}
		rel, _ := filepath.Rel(r.store.Environment(w.ID), filepath.Join(target, entry.Name()))
		out = append(out, File{Path: filepath.ToSlash(rel), Directory: entry.IsDir(), Size: i.Size()})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Directory && !out[j].Directory || out[i].Directory == out[j].Directory && out[i].Path < out[j].Path
	})
	return out, nil
}

func (r *Runner) WriteFile(w Workspace, actor, path string, content []byte, deleted bool, baseDigest *string) (Workspace, error) {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	target, err := r.resolve(w.ID, path)
	if err != nil || path == "" {
		return Workspace{}, ErrUnsafePath
	}
	if baseDigest != nil {
		current, readErr := os.ReadFile(target)
		currentDigest := ""
		if readErr == nil {
			sum := sha256.Sum256(current)
			currentDigest = hex.EncodeToString(sum[:])
		} else if !os.IsNotExist(readErr) {
			return Workspace{}, readErr
		}
		if currentDigest != *baseDigest {
			return Workspace{}, ErrConflict
		}
	}
	if deleted {
		err = os.Remove(target)
	} else {
		if len(content) > 1<<20 {
			return Workspace{}, errors.New("file too large")
		}
		err = os.MkdirAll(filepath.Dir(target), 0750)
		if err == nil {
			err = os.WriteFile(target, content, 0640)
		}
	}
	if err != nil && !(deleted && os.IsNotExist(err)) {
		return Workspace{}, err
	}
	digest := ""
	if !deleted {
		sum := sha256.Sum256(content)
		digest = hex.EncodeToString(sum[:])
	}
	return r.store.RecordChange(w.RepositoryID, w.ID, actor, filepath.ToSlash(filepath.Clean(path)), digest, deleted)
}

func (r *Runner) Search(w Workspace, query string) ([]Match, error) {
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 200 {
		return nil, errors.New("invalid query")
	}
	out := []Match{}
	err := filepath.WalkDir(r.store.Environment(w.ID), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if len(out) >= 200 {
			return nil
		}
		data, e := os.ReadFile(path)
		if e != nil || len(data) > 1<<20 || bytes.IndexByte(data, 0) >= 0 {
			return nil
		}
		rel, _ := filepath.Rel(r.store.Environment(w.ID), path)
		for index, line := range strings.Split(string(data), "\n") {
			if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				out = append(out, Match{Path: filepath.ToSlash(rel), Line: index + 1, Text: line})
				if len(out) >= 200 {
					break
				}
			}
		}
		return nil
	})
	return out, err
}

func (r *Runner) Command(w Workspace, actor, command, directory string, timeoutSeconds int) (CommandResult, error) {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	command = strings.TrimSpace(command)
	if command == "" || len(command) > 4000 {
		return CommandResult{}, errors.New("invalid command")
	}
	if timeoutSeconds < 1 || timeoutSeconds > 300 {
		timeoutSeconds = 60
	}
	cwd, err := r.resolve(w.ID, directory)
	if err != nil {
		return CommandResult{}, err
	}
	if info, e := os.Stat(cwd); e != nil || !info.IsDir() {
		return CommandResult{}, ErrUnsafePath
	}
	rel, _ := filepath.Rel(r.store.Environment(w.ID), cwd)
	sandboxCWD := "/workspace"
	if rel != "." {
		sandboxCWD += "/" + filepath.ToSlash(rel)
	}
	start := time.Now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	args := []string{"--unshare-all", "--die-with-parent", "--new-session", "--clearenv", "--ro-bind", "/usr", "/usr", "--ro-bind", "/bin", "/bin", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64", "--ro-bind", "/etc", "/etc", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--bind", r.store.Environment(w.ID), "/workspace", "--chdir", sandboxCWD, "--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin", "--setenv", "HOME", "/tmp", "--setenv", "KOMODO_COMMIT", w.Revision, "/bin/sh", "-c", "ulimit -t " + strconv.Itoa(w.Definition.Resources.CPUSeconds) + "; ulimit -v " + strconv.Itoa(w.Definition.Resources.MemoryMB*1024) + "; ulimit -f " + strconv.Itoa(w.Definition.Resources.DiskMB*2048) + "; " + command}
	cmd := exec.CommandContext(ctx, "bwrap", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{buffer: &stdout, remaining: 1 << 20}
	cmd.Stderr = &limitedWriter{buffer: &stderr, remaining: 1 << 20}
	runErr := cmd.Run()
	code := 0
	if runErr != nil {
		code = -1
		var exit *exec.ExitError
		if errors.As(runErr, &exit) {
			code = exit.ExitCode()
		}
	}
	result := CommandResult{Command: command, Directory: filepath.ToSlash(rel), ExitCode: code, Stdout: stdout.String(), Stderr: stderr.String(), StartedAt: start, CompletedAt: time.Now().UTC()}
	// The caller receives its output, but shared durable activity deliberately
	// records only execution metadata. Raw terminal input and output can contain
	// secrets and never become collaboration history implicitly.
	_, recordErr := r.store.RecordActivity(w.RepositoryID, w.ID, Event{Type: "command", Kind: "execution", Surface: "terminal", ExitCode: &code, Message: "private command completed", ActorID: actor})
	if recordErr != nil {
		return result, recordErr
	}
	if size, e := directorySize(r.store.Environment(w.ID)); e != nil || size > int64(w.Definition.Resources.DiskMB)<<20 {
		return result, errors.New("workspace disk limit exceeded")
	}
	return result, nil
}

type limitedWriter struct {
	buffer    *bytes.Buffer
	remaining int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	original := len(p)
	if len(p) > w.remaining {
		p = p[:w.remaining]
	}
	_, _ = w.buffer.Write(p)
	w.remaining -= len(p)
	return original, nil
}
