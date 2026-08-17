package supportquestions

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
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

type VerificationResources struct {
	CPUSeconds int `json:"cpu_seconds"`
	MemoryMB   int `json:"memory_mb"`
	DiskMB     int `json:"disk_mb"`
}
type VerificationEnvironment struct {
	Name        string                `json:"name"`
	ImageDigest string                `json:"image_digest"`
	Tools       []string              `json:"tools,omitempty"`
	Resources   VerificationResources `json:"resources"`
}
type VerificationInput struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Content   string `json:"content,omitempty"`
	SHA256    string `json:"sha256"`
}
type VerificationArtifact struct {
	Path      string `json:"path"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	Content   string `json:"content"`
}
type VerificationEvent struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	Command   string    `json:"command,omitempty"`
	Stream    string    `json:"stream,omitempty"`
	Message   string    `json:"message,omitempty"`
	ExitCode  *int      `json:"exit_code,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type VerificationAttempt struct {
	ID                 string                  `json:"id"`
	RepositoryID       string                  `json:"repository_id"`
	QuestionID         string                  `json:"question_id"`
	AnswerID           string                  `json:"answer_id"`
	AnswerRevisionID   string                  `json:"answer_revision_id"`
	SourceRevision     string                  `json:"source_revision"`
	SoftwareVersion    string                  `json:"software_version"`
	Instructions       []string                `json:"instructions"`
	InstructionsDigest string                  `json:"instructions_digest"`
	Environment        VerificationEnvironment `json:"environment"`
	Dependencies       map[string]string       `json:"dependencies"`
	Inputs             []VerificationInput     `json:"sanitized_inputs"`
	ArtifactPaths      []string                `json:"artifact_paths,omitempty"`
	CreatedByID        string                  `json:"created_by_id"`
	RerunOf            string                  `json:"rerun_of,omitempty"`
	State              string                  `json:"state"`
	Result             string                  `json:"result,omitempty"`
	FailureReason      string                  `json:"failure_reason,omitempty"`
	CostUnits          float64                 `json:"cost_units"`
	Events             []VerificationEvent     `json:"events"`
	Artifacts          []VerificationArtifact  `json:"artifacts"`
	CreatedAt          time.Time               `json:"created_at"`
	CompletedAt        *time.Time              `json:"completed_at,omitempty"`
}
type VerificationInputRequest struct {
	AnswerID         string                  `json:"answer_id"`
	AnswerRevisionID string                  `json:"answer_revision_id"`
	SourceRevision   string                  `json:"source_revision"`
	SoftwareVersion  string                  `json:"software_version"`
	Environment      VerificationEnvironment `json:"environment"`
	Dependencies     map[string]string       `json:"dependencies"`
	Inputs           []VerificationInput     `json:"sanitized_inputs"`
	ArtifactPaths    []string                `json:"artifact_paths"`
	CostUnits        float64                 `json:"cost_units"`
}

func instructionDigest(v []string) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func secretLike(data []byte) bool {
	lower := strings.ToLower(string(data))
	for _, m := range []string{"-----begin private key", "ghp_", "github_pat_", "sk-", "authorization: bearer", "aws_secret_access_key", "password="} {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
func safePath(v string) bool {
	c := filepath.Clean(v)
	return v != "" && c != "." && !filepath.IsAbs(c) && c != ".." && !strings.HasPrefix(c, ".."+string(filepath.Separator)) && !strings.ContainsRune(v, '\x00')
}

func validateVerification(q Question, answer Answer, revision AnswerRevision, in VerificationInputRequest) ([]VerificationInput, error) {
	if in.AnswerID != answer.ID || in.AnswerRevisionID != revision.ID || strings.TrimSpace(in.SourceRevision) == "" || strings.TrimSpace(in.SoftwareVersion) == "" || strings.TrimSpace(in.Environment.Name) == "" || strings.TrimSpace(in.Environment.ImageDigest) == "" || in.Environment.Name != q.Environment || in.Environment.Resources.CPUSeconds < 1 || in.Environment.Resources.CPUSeconds > 600 || in.Environment.Resources.MemoryMB < 128 || in.Environment.Resources.MemoryMB > 8192 || in.Environment.Resources.DiskMB < 128 || in.Environment.Resources.DiskMB > 5120 || len(in.Inputs) > 10 || len(in.ArtifactPaths) > 20 || in.CostUnits < 0 {
		return nil, ErrInvalid
	}
	applicable := false
	for _, v := range revision.ApplicableVersions {
		if v == in.SoftwareVersion {
			applicable = true
		}
	}
	if !applicable {
		return nil, ErrInvalid
	}
	for k, v := range in.Dependencies {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" || len(k) > 200 || len(v) > 200 {
			return nil, ErrInvalid
		}
	}
	for _, p := range in.ArtifactPaths {
		if !safePath(p) {
			return nil, ErrInvalid
		}
	}
	total, seen := 0, map[string]bool{}
	out := make([]VerificationInput, len(in.Inputs))
	for i, item := range in.Inputs {
		item.Name = filepath.ToSlash(filepath.Clean(strings.TrimSpace(item.Name)))
		lower := strings.ToLower(item.Name)
		if !safePath(item.Name) || seen[item.Name] || !map[string]bool{"text/plain": true, "application/json": true, "application/octet-stream": true}[item.MediaType] {
			return nil, ErrInvalid
		}
		for _, m := range []string{"token", "secret", "password", "credential", "private_key", ".env", "id_rsa", "userdata", "user-data"} {
			if strings.Contains(lower, m) {
				return nil, ErrInvalid
			}
		}
		raw, e := base64.StdEncoding.DecodeString(item.Content)
		if e != nil || len(raw) > 1<<20 || secretLike(raw) {
			return nil, ErrInvalid
		}
		total += len(raw)
		if total > 5<<20 {
			return nil, ErrInvalid
		}
		sum := sha256.Sum256(raw)
		item.SHA256 = hex.EncodeToString(sum[:])
		out[i], seen[item.Name] = item, true
	}
	return out, nil
}

func (s *Store) CreateVerification(q Question, actor, rerunOf string, in VerificationInputRequest) (VerificationAttempt, error) {
	var answer *Answer
	var revision *AnswerRevision
	for i := range q.Answers {
		if q.Answers[i].ID == in.AnswerID {
			answer = &q.Answers[i]
			for j := range answer.Revisions {
				if answer.Revisions[j].ID == in.AnswerRevisionID {
					revision = &answer.Revisions[j]
				}
			}
		}
	}
	if actor == "" || answer == nil || revision == nil {
		return VerificationAttempt{}, ErrInvalid
	}
	inputs, e := validateVerification(q, *answer, *revision, in)
	if e != nil {
		return VerificationAttempt{}, e
	}
	id, e := newID()
	if e != nil {
		return VerificationAttempt{}, e
	}
	now := s.now().UTC()
	a := VerificationAttempt{ID: id, RepositoryID: q.RepositoryID, QuestionID: q.ID, AnswerID: answer.ID, AnswerRevisionID: revision.ID, SourceRevision: in.SourceRevision, SoftwareVersion: in.SoftwareVersion, Instructions: append([]string{}, revision.Instructions...), InstructionsDigest: instructionDigest(revision.Instructions), Environment: in.Environment, Dependencies: in.Dependencies, Inputs: inputs, ArtifactPaths: in.ArtifactPaths, CreatedByID: actor, RerunOf: rerunOf, State: "queued", CostUnits: in.CostUnits, Events: []VerificationEvent{}, Artifacts: []VerificationArtifact{}, CreatedAt: now}
	s.mu.Lock()
	defer s.mu.Unlock()
	return a, s.writeVerification(a)
}
func (s *Store) GetVerification(repo, q, id string) (VerificationAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readVerification(repo, q, id)
}
func (s *Store) ListVerifications(repo, q string) ([]VerificationAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo, q, "verifications"))
	if errors.Is(e, fs.ErrNotExist) {
		return []VerificationAttempt{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []VerificationAttempt{}
	for _, x := range es {
		if x.IsDir() || filepath.Ext(x.Name()) != ".json" {
			continue
		}
		a, er := s.readVerification(repo, q, strings.TrimSuffix(x.Name(), ".json"))
		if er != nil {
			return nil, er
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) readVerification(repo, q, id string) (VerificationAttempt, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, q, "verifications", id+".json"))
	if e != nil {
		return VerificationAttempt{}, ErrNotFound
	}
	var a VerificationAttempt
	if json.Unmarshal(b, &a) != nil || a.ID != id || a.RepositoryID != repo || a.QuestionID != q {
		return VerificationAttempt{}, ErrNotFound
	}
	return a, nil
}
func (s *Store) writeVerification(a VerificationAttempt) error {
	dir := filepath.Join(s.root, a.RepositoryID, a.QuestionID, "verifications")
	if e := os.MkdirAll(dir, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(a, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(dir, ".attempt-*")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Chmod(0640)
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, filepath.Join(dir, a.ID+".json"))
}
func (s *Store) mutateVerification(a VerificationAttempt, fn func(*VerificationAttempt)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.readVerification(a.RepositoryID, a.QuestionID, a.ID)
	if e != nil {
		return e
	}
	fn(&v)
	return s.writeVerification(v)
}

type verificationRepositoryOpener interface {
	Open(storage.ID) (*storage.Repository, error)
}
type VerificationRunner struct {
	store        *Store
	repositories verificationRepositoryOpener
}

func NewVerificationRunner(s *Store, r verificationRepositoryOpener) *VerificationRunner {
	return &VerificationRunner{store: s, repositories: r}
}
func (r *VerificationRunner) Start(a VerificationAttempt) { go r.execute(a) }
func (r *VerificationRunner) execute(a VerificationAttempt) {
	repo, e := r.repositories.Open(storage.ID(a.RepositoryID))
	root, me := os.MkdirTemp("", "komodo-support-verification-*")
	if e == nil {
		e = me
	}
	if root != "" {
		defer os.RemoveAll(root)
	}
	if e == nil {
		var c storage.Commit
		c, e = repo.ReadCommit(storage.ObjectID(a.SourceRevision))
		if e == nil {
			e = materialize(repo, c.Tree, root)
		}
	}
	if e == nil {
		for _, in := range a.Inputs {
			var raw []byte
			raw, e = base64.StdEncoding.DecodeString(in.Content)
			if e != nil {
				break
			}
			p := filepath.Join(root, ".komodo-inputs", filepath.FromSlash(in.Name))
			if e = os.MkdirAll(filepath.Dir(p), 0750); e == nil {
				e = os.WriteFile(p, raw, 0640)
			}
			if e != nil {
				break
			}
		}
	}
	if e != nil {
		r.complete(a, "failed", "environment setup failed", e.Error(), nil)
		return
	}
	for _, cmd := range a.Instructions {
		code, er := r.run(a, root, cmd)
		if er != nil {
			r.complete(a, "failed", "instruction failed", "exit code "+strconv.Itoa(code), nil)
			return
		}
	}
	arts, e := verificationArtifacts(root, a.ArtifactPaths)
	if e != nil {
		r.complete(a, "failed", "declared artifact unavailable", e.Error(), nil)
		return
	}
	r.complete(a, "passed", "", "all instructions completed successfully", arts)
}
func (r *VerificationRunner) run(a VerificationAttempt, root, command string) (int, error) {
	_ = r.event(a, VerificationEvent{Type: "command", Command: command})
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.Environment.Resources.CPUSeconds)*time.Second)
	defer cancel()
	args := []string{"--unshare-all", "--die-with-parent", "--new-session", "--clearenv", "--ro-bind", "/usr", "/usr", "--ro-bind", "/bin", "/bin", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64", "--ro-bind", "/etc", "/etc", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--bind", root, "/workspace", "--chdir", "/workspace", "--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin", "--setenv", "HOME", "/tmp", "--setenv", "KOMODO_COMMIT", a.SourceRevision, "--setenv", "KOMODO_INPUT_DIR", "/workspace/.komodo-inputs", "/bin/sh", "-c", "ulimit -t " + strconv.Itoa(a.Environment.Resources.CPUSeconds) + "; ulimit -v " + strconv.Itoa(a.Environment.Resources.MemoryMB*1024) + "; ulimit -f " + strconv.Itoa(a.Environment.Resources.DiskMB*2048) + "; " + command}
	c := exec.CommandContext(ctx, "bwrap", args...)
	stdout, _ := c.StdoutPipe()
	stderr, _ := c.StderrPipe()
	e := c.Start()
	if e == nil {
		done := make(chan bool, 2)
		go r.logs(a, stdout, "stdout", done)
		go r.logs(a, stderr, "stderr", done)
		e = c.Wait()
		<-done
		<-done
	}
	code := 0
	if e != nil {
		code = -1
		var x *exec.ExitError
		if errors.As(e, &x) {
			code = x.ExitCode()
		}
	}
	_ = r.event(a, VerificationEvent{Type: "outcome", Command: command, ExitCode: &code})
	return code, e
}
func (r *VerificationRunner) logs(a VerificationAttempt, rd io.Reader, stream string, done chan<- bool) {
	defer func() { done <- true }()
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 16<<10), 1<<20)
	for sc.Scan() {
		msg := sc.Text()
		if secretLike([]byte(msg)) {
			msg = "[redacted credential-like output]"
		}
		_ = r.event(a, VerificationEvent{Type: "log", Stream: stream, Message: msg})
	}
}
func (r *VerificationRunner) event(a VerificationAttempt, e VerificationEvent) error {
	return r.store.mutateVerification(a, func(v *VerificationAttempt) {
		if len(v.Events) >= 2000 {
			return
		}
		e.Sequence = int64(len(v.Events) + 1)
		e.CreatedAt = time.Now().UTC()
		v.Events = append(v.Events, e)
		v.State = "running"
	})
}
func (r *VerificationRunner) complete(a VerificationAttempt, state, reason, result string, arts []VerificationArtifact) {
	now := time.Now().UTC()
	_ = r.store.mutateVerification(a, func(v *VerificationAttempt) {
		v.State = state
		v.FailureReason = reason
		v.Result = result
		v.Artifacts = arts
		v.CompletedAt = &now
	})
}
func materialize(repo *storage.Repository, treeID storage.ObjectID, root string) error {
	tree, e := repo.ReadTree(treeID)
	if e != nil {
		return e
	}
	for _, x := range tree.Entries {
		if x.Name == "." || x.Name == ".." || strings.ContainsAny(x.Name, "/\x00") {
			return ErrInvalid
		}
		p := filepath.Join(root, x.Name)
		switch x.Type {
		case storage.TreeObject:
			if e = os.Mkdir(p, 0750); e == nil {
				e = materialize(repo, x.ObjectID, p)
			}
		case storage.BlobObject:
			if x.Mode == 0120000 {
				return ErrInvalid
			}
			var o storage.Object
			o, e = repo.ReadObject(x.ObjectID)
			if e == nil {
				mode := os.FileMode(0640)
				if x.Mode == 0100755 {
					mode = 0750
				}
				e = os.WriteFile(p, o.Content, mode)
			}
		default:
			e = ErrInvalid
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func verificationArtifacts(root string, paths []string) ([]VerificationArtifact, error) {
	out := []VerificationArtifact{}
	total := int64(0)
	for _, name := range paths {
		p := filepath.Join(root, filepath.FromSlash(name))
		info, e := os.Lstat(p)
		if e != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
			return nil, ErrInvalid
		}
		b, e := os.ReadFile(p)
		if e != nil || secretLike(b) {
			return nil, ErrInvalid
		}
		total += int64(len(b))
		if total > 5<<20 {
			return nil, ErrInvalid
		}
		sum := sha256.Sum256(b)
		out = append(out, VerificationArtifact{Path: name, MediaType: "application/octet-stream", Size: int64(len(b)), SHA256: hex.EncodeToString(sum[:]), Content: base64.StdEncoding.EncodeToString(b)})
	}
	return out, nil
}
