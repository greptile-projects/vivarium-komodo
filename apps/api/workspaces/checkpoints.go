package workspaces

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

var ErrUnsafeCheckpoint = errors.New("unsafe checkpoint content")

type CheckpointRequest struct {
	Summary         string
	Paths           []string
	ParentID        string
	Reproducibility Reproducibility
}

type PublishRequest struct {
	Branch, Message string
	// ExpectedBranchTip permits a reconciled tree to advance the original
	// contribution branch without pretending it was based on the target tree.
	ExpectedBranchTip string
	ParentCommitIDs   []string
	Mode              string
}

// Publish writes only the immutable bytes named by a checkpoint. The mutable
// workspace is deliberately never walked here.
func (r *Runner) Publish(w Workspace, actor, checkpointID string, request PublishRequest) (storage.ObjectID, []string, error) {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	var checkpoint *Checkpoint
	for i := range w.Checkpoints {
		if w.Checkpoints[i].ID == checkpointID {
			checkpoint = &w.Checkpoints[i]
			break
		}
	}
	branch := strings.TrimSpace(request.Branch)
	message := strings.TrimSpace(request.Message)
	if checkpoint == nil {
		return "", nil, ErrNotFound
	}
	if checkpoint.Publication != nil || branch == "" || message == "" || len(message) > 10000 || strings.HasPrefix(branch, "refs/") {
		return "", nil, ErrConflict
	}
	repo, err := r.repositories.Open(storage.ID(w.RepositoryID))
	if err != nil {
		return "", nil, err
	}
	base, err := repo.ReadCommit(storage.ObjectID(checkpoint.BaseRevision))
	if err != nil {
		return "", nil, err
	}
	files := map[string]treeFile{}
	if err = flattenTree(repo, base.Tree, "", files); err != nil {
		return "", nil, err
	}
	for _, change := range checkpoint.Changes {
		if change.Operation == "delete" {
			delete(files, change.Path)
			continue
		}
		blobDigest := change.BlobDigest
		if blobDigest == "" {
			blobDigest = change.Digest
		}
		data, e := r.store.Blob(blobDigest)
		if e != nil || sha(data) != change.Digest {
			return "", nil, ErrUnsafeCheckpoint
		}
		blob, e := repo.WriteObject(storage.BlobObject, data)
		if e != nil {
			return "", nil, e
		}
		mode := uint32(0100644)
		if old, ok := files[change.Path]; ok {
			mode = old.mode
		}
		files[change.Path] = treeFile{mode: mode, id: blob}
	}
	tree, err := writeTree(repo, files)
	if err != nil {
		return "", nil, err
	}
	stamp := strconv.FormatInt(time.Now().UTC().Unix(), 10) + " +0000"
	trailers := "\nWorkspace-ID: " + w.ID + "\nCheckpoint-ID: " + checkpoint.ID
	if w.Context.Type != "repository" {
		trailers += "\nWorkspace-Context: " + w.Context.Type + "/" + w.Context.ID
	}
	parents := append([]string(nil), request.ParentCommitIDs...)
	if len(parents) == 0 {
		parents = []string{checkpoint.BaseRevision}
	}
	parentLines := ""
	seenParents := map[string]bool{}
	for _, parent := range parents {
		if parent != "" && !seenParents[parent] {
			parentLines += "parent " + parent + "\n"
			seenParents[parent] = true
		}
	}
	if checkpoint.Verification != nil {
		trailers += "\nVerification-Candidate: " + checkpoint.Verification.Digest
		trailers += "\nResolution-Source: " + checkpoint.Verification.Inputs.Source
		trailers += "\nResolution-Target: " + checkpoint.Verification.Inputs.Target
		for _, decision := range checkpoint.Verification.Decisions {
			if len(decision.StaleInputKeys) == 0 && decision.Decision == "approved" {
				trailers += "\nResolution-Approval: " + decision.ID
			}
		}
	}
	for _, resolution := range w.Resolutions {
		trailers += "\nResolution-Entry: " + resolution.ID
	}
	for _, command := range checkpoint.Reproducibility.Commands {
		trailers += "\nResolution-Command: " + strings.ReplaceAll(command, "\n", " ")
	}
	content := fmt.Sprintf("tree %s\n%sauthor %s <%s@users.local> %s\ncommitter %s <%s@users.local> %s\n\n%s%s\n", tree, parentLines, actor, actor, stamp, actor, actor, stamp, message, trailers)
	commit, err := repo.WriteObject(storage.CommitObject, []byte(content))
	if err != nil {
		return "", nil, err
	}
	refName := storage.ReferenceName("refs/heads/" + branch)
	ref, readErr := repo.ReadReference(refName)
	if readErr == nil {
		expected := checkpoint.BaseRevision
		if request.ExpectedBranchTip != "" {
			expected = request.ExpectedBranchTip
		}
		if ref.ObjectID != storage.ObjectID(expected) {
			return "", nil, ErrConflict
		}
		err = repo.CompareAndSwapReference(refName, ref.ObjectID, commit)
	} else if errors.Is(readErr, storage.ErrReferenceNotFound) {
		err = repo.CreateReference(storage.Reference{Name: refName, ObjectID: commit})
	} else {
		err = readErr
	}
	if err != nil {
		return "", nil, ErrConflict
	}
	contributors := map[string]bool{checkpoint.CreatorID: true, actor: true}
	paths := map[string]bool{}
	for _, c := range checkpoint.Changes {
		paths[c.Path] = true
	}
	for _, c := range w.Changes {
		if paths[c.Path] {
			contributors[c.ActorID] = true
		}
	}
	ids := []string{}
	for id := range contributors {
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	publication := Publication{CommitID: string(commit), Branch: branch, Mode: request.Mode, PublisherID: actor, ContributorIDs: ids, Commands: append([]string(nil), checkpoint.Reproducibility.Commands...), PublishedAt: time.Now().UTC()}
	if checkpoint.Verification != nil {
		publication.SourceCommitID, publication.TargetCommitID, publication.VerificationDigest = checkpoint.Verification.Inputs.Source, checkpoint.Verification.Inputs.Target, checkpoint.Verification.Digest
		for _, decision := range checkpoint.Verification.Decisions {
			if len(decision.StaleInputKeys) == 0 && decision.Decision == "approved" {
				publication.ApprovalIDs = append(publication.ApprovalIDs, decision.ID)
			}
		}
	}
	for _, resolution := range w.Resolutions {
		publication.ResolutionIDs = append(publication.ResolutionIDs, resolution.ID)
	}
	_, err = r.store.RecordPublication(w.RepositoryID, w.ID, checkpoint.ID, publication)
	if err != nil {
		return "", nil, err
	}
	return commit, ids, nil
}

type treeFile struct {
	mode uint32
	id   storage.ObjectID
}

func flattenTree(repo *storage.Repository, id storage.ObjectID, prefix string, files map[string]treeFile) error {
	tree, err := repo.ReadTree(id)
	if err != nil {
		return err
	}
	for _, entry := range tree.Entries {
		path := entry.Name
		if prefix != "" {
			path = prefix + "/" + path
		}
		if entry.Type == storage.TreeObject {
			if err := flattenTree(repo, entry.ObjectID, path, files); err != nil {
				return err
			}
		} else {
			files[path] = treeFile{entry.Mode, entry.ObjectID}
		}
	}
	return nil
}

type treeNode struct {
	files map[string]treeFile
	dirs  map[string]*treeNode
}

func writeTree(repo *storage.Repository, files map[string]treeFile) (storage.ObjectID, error) {
	root := &treeNode{map[string]treeFile{}, map[string]*treeNode{}}
	for path, file := range files {
		parts := strings.Split(path, "/")
		node := root
		for _, part := range parts[:len(parts)-1] {
			if node.dirs[part] == nil {
				node.dirs[part] = &treeNode{map[string]treeFile{}, map[string]*treeNode{}}
			}
			node = node.dirs[part]
		}
		node.files[parts[len(parts)-1]] = file
	}
	var build func(*treeNode) (storage.ObjectID, error)
	build = func(node *treeNode) (storage.ObjectID, error) {
		names := []string{}
		for n := range node.files {
			names = append(names, n)
		}
		for n := range node.dirs {
			names = append(names, n)
		}
		sort.Slice(names, func(i, j int) bool {
			left, right := names[i], names[j]
			if node.dirs[left] != nil {
				left += "/"
			}
			if node.dirs[right] != nil {
				right += "/"
			}
			return left < right
		})
		var data bytes.Buffer
		for _, n := range names {
			file, ok := node.files[n]
			mode := file.mode
			id := file.id
			if !ok {
				var e error
				id, e = build(node.dirs[n])
				if e != nil {
					return "", e
				}
				mode = 040000
			}
			fmt.Fprintf(&data, "%o %s%c", mode, n, byte(0))
			raw, e := hex.DecodeString(string(id))
			if e != nil {
				return "", e
			}
			data.Write(raw)
		}
		return repo.WriteObject(storage.TreeObject, data.Bytes())
	}
	return build(root)
}

func sha(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// PolicyDigest is the stable identity used by reconciliation verification.
func PolicyDigest(policy Policy) string {
	encoded, _ := json.Marshal(policy)
	return sha(encoded)
}

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
	if w.Context.Conflict != nil {
		checkpoint.Verification = buildVerificationCandidate(w, checkpoint)
	}
	return r.store.AddCheckpoint(w.RepositoryID, w.ID, checkpoint, blobs)
}

func buildVerificationCandidate(w Workspace, checkpoint Checkpoint) *VerificationCandidate {
	encoded, _ := json.Marshal(struct {
		Base    string
		Changes []CheckpointChange
	}{checkpoint.BaseRevision, checkpoint.Changes})
	policy, _ := json.Marshal(w.Policy)
	inputs := VerificationInputs{Candidate: sha(encoded), Source: w.Context.Conflict.Source.CommitID, Target: w.Context.Conflict.Target.CommitID, Dependency: checkpoint.DefinitionDigest, Policy: sha(policy)}
	criteria := []VerificationCriterion{}
	add := func(kind, description, origin string, affected []string) {
		key := sha([]byte(kind + "\x00" + description + "\x00" + origin))[:16]
		criteria = append(criteria, VerificationCriterion{ID: key, Kind: kind, Description: description, Origin: origin, AffectedInputs: affected, OwnerIDs: append([]string(nil), w.Context.Conflict.OwnerIDs...)})
	}
	for _, command := range checkpoint.Definition.Commands {
		kind := "required_check"
		name := strings.ToLower(command.Name)
		if strings.Contains(name, "contract") {
			kind = "contract_scenario"
		}
		if strings.Contains(name, "schema") {
			kind = "schema_scenario"
		}
		if strings.Contains(name, "preview") {
			kind = "preview_acceptance"
		}
		if strings.Contains(name, "conflict") {
			kind = "conflict_test"
		}
		add(kind, command.Name+": "+command.Command, "target:.komodo/workspaces.json", []string{"candidate", "target", "dependency", "policy"})
	}
	for _, command := range checkpoint.Reproducibility.Commands {
		add("reproduction", command, "checkpoint:reproducibility", []string{"candidate", "source", "target", "dependency"})
	}
	for _, resolution := range w.Resolutions {
		for _, impact := range resolution.Impacts {
			if impact.Disposition == "preserved" || impact.Disposition == "changed" {
				add("acceptance", impact.Outcome, "resolution:"+resolution.ID, []string{"candidate", "source", "target"})
			}
		}
	}
	if len(criteria) == 0 {
		add("conflict_scenario", "Resolved paths remain compatible with both frozen contributions", "conflict-analysis:"+w.Context.Conflict.PullRequestID, []string{"candidate", "source", "target"})
	}
	digestBody, _ := json.Marshal(struct {
		Inputs   VerificationInputs
		Criteria []VerificationCriterion
	}{inputs, criteria})
	return &VerificationCandidate{Digest: sha(digestBody), Inputs: inputs, Criteria: criteria, Attempts: []VerificationAttempt{}, Decisions: []VerificationDecision{}, Status: "pending", Blockers: []string{"required verification evidence is incomplete"}}
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
