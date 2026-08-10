package workspaces

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

var ErrUnsafeCheckpoint = errors.New("unsafe checkpoint content")

type CheckpointRequest struct {
	Summary         string
	Paths           []string
	ParentID        string
	Reproducibility Reproducibility
}

func sha(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func sensitiveCheckpointPath(path string) bool {
	lower := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	if base == ".env" || strings.HasPrefix(base, ".env.") || base == "credentials" || base == "id_rsa" || base == "id_ed25519" || strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") {
		return true
	}
	if base == ".netrc" || base == ".npmrc" || base == ".pypirc" || base == "known_hosts" {
		return true
	}
	for _, part := range strings.Split(lower, "/") {
		if part == ".git" || part == ".ssh" || part == ".aws" || part == "node_modules" {
			return true
		}
	}
	return false
}

func sensitiveCheckpointContent(data []byte) bool {
	upper := bytes.ToUpper(data)
	markers := [][]byte{[]byte("BEGIN PRIVATE KEY"), []byte("BEGIN OPENSSH PRIVATE KEY"), []byte("AWS_SECRET_ACCESS_KEY="), []byte("GITHUB_TOKEN="), []byte("AUTHORIZATION: BEARER "), []byte("PASSWORD="), []byte("SECRET="), []byte("TOKEN="), []byte("GHP_")}
	for _, marker := range markers {
		if bytes.Contains(upper, marker) {
			return true
		}
	}
	return false
}

func (r *Runner) Checkpoint(w Workspace, actor string, request CheckpointRequest) (Workspace, error) {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	request.Summary = strings.TrimSpace(request.Summary)
	if request.Summary == "" || len(request.Summary) > 1000 || len(request.Paths) == 0 || len(request.Paths) > 200 || len(request.Reproducibility.Notes) > 4000 || len(request.Reproducibility.Commands) > 50 || len(request.Reproducibility.Dependencies) > 100 {
		return Workspace{}, ErrConflict
	}
	repo, err := r.repositories.Open(storage.ID(w.RepositoryID))
	if err != nil {
		return Workspace{}, err
	}
	seen := map[string]bool{}
	changes := []CheckpointChange{}
	blobs := map[string][]byte{}
	for _, rawPath := range request.Paths {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(rawPath)))
		if clean == "." || sensitiveCheckpointPath(clean) || seen[clean] {
			return Workspace{}, ErrUnsafeCheckpoint
		}
		seen[clean] = true
		target, resolveErr := r.resolve(w.ID, clean)
		if resolveErr != nil {
			return Workspace{}, ErrUnsafeCheckpoint
		}
		var current []byte
		current, err = os.ReadFile(target)
		missing := os.IsNotExist(err)
		if err != nil && !missing {
			return Workspace{}, ErrUnsafeCheckpoint
		}
		if !missing && (len(current) > 5<<20 || sensitiveCheckpointContent(current)) {
			return Workspace{}, ErrUnsafeCheckpoint
		}
		base, baseErr := readFile(repo, storage.ObjectID(w.Revision), clean)
		baseMissing := baseErr != nil
		if missing && baseMissing {
			continue
		}
		if !missing && !baseMissing && bytes.Equal(current, base) {
			continue
		}
		change := CheckpointChange{Path: clean}
		if !baseMissing {
			change.BaseDigest = sha(base)
		}
		switch {
		case missing:
			change.Operation = "delete"
		case baseMissing:
			change.Operation = "add"
		default:
			change.Operation = "modify"
		}
		if !missing {
			change.Digest, change.BlobDigest, change.Size = sha(current), sha(current), int64(len(current))
			blobs[change.BlobDigest] = current
			change.Binary = bytes.IndexByte(current, 0) >= 0
			if !change.Binary && len(current) <= 256<<10 && len(base) <= 256<<10 {
				change.Patch = readablePatch(base, current)
			}
		}
		changes = append(changes, change)
	}
	if len(changes) == 0 {
		return Workspace{}, ErrConflict
	}
	id, err := newID()
	if err != nil {
		return Workspace{}, err
	}
	missingDeps := missingDependencies(w.Definition.Dependencies, request.Reproducibility.Dependencies)
	reasons := []string{}
	if len(missingDeps) > 0 {
		reasons = append(reasons, "declared dependencies are absent from the environment definition")
	}
	if len(request.Reproducibility.Commands) == 0 {
		reasons = append(reasons, "no reproduction or verification commands were declared")
	}
	checkpoint := Checkpoint{ID: id, WorkspaceID: w.ID, RepositoryID: w.RepositoryID, ParentID: strings.TrimSpace(request.ParentID), CreatorID: actor, BaseRevision: w.Revision, Definition: w.Definition, DefinitionDigest: w.DefinitionDigest, Summary: request.Summary, Reproducibility: request.Reproducibility, Changes: changes, Status: CheckpointStatus{Reproducible: len(reasons) == 0, Conflicts: []string{}, MissingDependencies: missingDeps, Reasons: reasons}, CreatedAt: r.store.now().UTC()}
	return r.store.AddCheckpoint(w.RepositoryID, w.ID, checkpoint, blobs)
}

func missingDependencies(available, declared []string) []string {
	have := map[string]bool{}
	for _, value := range available {
		have[strings.ToLower(strings.TrimSpace(value))] = true
	}
	missing := []string{}
	for _, value := range declared {
		value = strings.TrimSpace(value)
		if value != "" && !have[strings.ToLower(value)] {
			missing = append(missing, value)
		}
	}
	sort.Strings(missing)
	return missing
}

func readablePatch(before, after []byte) string {
	if bytes.Equal(before, after) {
		return ""
	}
	oldLines, newLines := strings.Split(string(before), "\n"), strings.Split(string(after), "\n")
	var out strings.Builder
	out.WriteString("--- base\n+++ checkpoint\n")
	max := len(oldLines)
	if len(newLines) > max {
		max = len(newLines)
	}
	for i := 0; i < max && out.Len() < 64<<10; i++ {
		if i < len(oldLines) && i < len(newLines) && oldLines[i] == newLines[i] {
			continue
		}
		if i < len(oldLines) {
			fmt.Fprintf(&out, "-%s\n", oldLines[i])
		}
		if i < len(newLines) {
			fmt.Fprintf(&out, "+%s\n", newLines[i])
		}
	}
	return out.String()
}

func (r *Runner) Restore(w Workspace, actor, checkpointID string) (Workspace, CheckpointStatus, error) {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	var checkpoint *Checkpoint
	for i := range w.Checkpoints {
		if w.Checkpoints[i].ID == checkpointID {
			checkpoint = &w.Checkpoints[i]
			break
		}
	}
	if checkpoint == nil {
		return Workspace{}, CheckpointStatus{}, ErrNotFound
	}
	status := r.checkpointStatus(w, *checkpoint)
	if len(status.Conflicts) > 0 {
		return Workspace{}, status, ErrConflict
	}
	verified := map[string][]byte{}
	for _, change := range checkpoint.Changes {
		if change.Operation == "delete" {
			continue
		}
		blobDigest := change.BlobDigest
		if blobDigest == "" {
			blobDigest = change.Digest
		}
		data, err := r.store.Blob(blobDigest)
		if err != nil || sha(data) != change.Digest {
			return Workspace{}, status, ErrUnsafeCheckpoint
		}
		verified[change.Path] = data
	}
	for _, change := range checkpoint.Changes {
		target, _ := r.resolve(w.ID, change.Path)
		if change.Operation == "delete" {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return Workspace{}, status, err
			}
			continue
		}
		data := verified[change.Path]
		err := os.MkdirAll(filepath.Dir(target), 0750)
		if err == nil {
			err = os.WriteFile(target, data, 0640)
		}
		if err != nil {
			return Workspace{}, status, err
		}
	}
	updated, err := r.store.RecordActivity(w.RepositoryID, w.ID, Event{Type: "checkpoint_restored", Kind: "authorship", ActorID: actor, TargetID: checkpoint.ID, Message: "restored checkpoint changes"})
	return updated, status, err
}

func (r *Runner) InspectCheckpoint(w Workspace, checkpointID string) (Checkpoint, error) {
	for _, checkpoint := range w.Checkpoints {
		if checkpoint.ID == checkpointID {
			checkpoint.Status = r.checkpointStatus(w, checkpoint)
			return checkpoint, nil
		}
	}
	return Checkpoint{}, ErrNotFound
}

func (r *Runner) checkpointStatus(w Workspace, checkpoint Checkpoint) CheckpointStatus {
	status := checkpoint.Status
	status.Conflicts = []string{}
	status.Diverged = false
	if w.Revision != checkpoint.BaseRevision {
		status.Diverged = true
		status.Conflicts = append(status.Conflicts, "workspace base revision differs from checkpoint")
	}
	if w.DefinitionDigest != checkpoint.DefinitionDigest {
		status.Diverged = true
		status.Conflicts = append(status.Conflicts, "environment definition differs from checkpoint")
	}
	for _, change := range checkpoint.Changes {
		target, _ := r.resolve(w.ID, change.Path)
		current, err := os.ReadFile(target)
		currentDigest := ""
		if err == nil {
			currentDigest = sha(current)
		}
		if currentDigest != change.BaseDigest && currentDigest != change.Digest {
			status.Diverged = true
			status.Conflicts = append(status.Conflicts, change.Path)
		}
	}
	return status
}
