package main

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type conflictRevision struct {
	RepositoryID string `json:"repository_id"`
	Branch       string `json:"branch"`
	CommitID     string `json:"commit_id"`
	LiveCommitID string `json:"live_commit_id,omitempty"`
	Stale        bool   `json:"stale"`
}
type conflictIntent struct {
	PullRequestID      string                         `json:"pull_request_id,omitempty"`
	ProposalID         string                         `json:"proposal_id,omitempty"`
	TaskID             string                         `json:"task_id,omitempty"`
	OwnerID            string                         `json:"owner_id,omitempty"`
	Title              string                         `json:"title,omitempty"`
	Description        string                         `json:"description,omitempty"`
	DiscussionURL      string                         `json:"discussion_url,omitempty"`
	AcceptanceCriteria []pullrequests.CriterionStatus `json:"acceptance_criteria"`
}
type conflictSide struct {
	Revision conflictRevision    `json:"revision"`
	Commits  []pullRequestCommit `json:"commits"`
	Intent   conflictIntent      `json:"intent"`
}
type conflictItem struct {
	Kind         string                 `json:"kind"`
	Path         string                 `json:"path,omitempty"`
	Symbol       string                 `json:"symbol,omitempty"`
	Detail       string                 `json:"detail"`
	SourceChange *pullRequestFileChange `json:"source_change,omitempty"`
	TargetChange *pullRequestFileChange `json:"target_change,omitempty"`
}
type conflictCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	RunID    string `json:"run_id,omitempty"`
	CommitID string `json:"commit_id"`
	Detail   string `json:"detail"`
}
type conflictAnalysis struct {
	BaseCommitID    string             `json:"base_commit_id"`
	Source          conflictSide       `json:"source"`
	Target          conflictSide       `json:"target"`
	Conflicts       []conflictItem     `json:"conflicts"`
	AffectedChecks  []conflictCheck    `json:"affected_checks"`
	Complete        bool               `json:"complete"`
	Stale           bool               `json:"stale"`
	Blockers        []readinessBlocker `json:"blockers"`
	MutatesBranches bool               `json:"mutates_branches"`
}

var declaredSymbol = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:func|function|class|type|interface|const|var|let|def|struct)\s+([A-Za-z_][A-Za-z0-9_]*)`)

func getPullRequestConflicts(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, checks readinessCheckStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		pull, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		sourceRepo, err := repositories.Open(storage.ID(pull.SourceRepositoryID))
		if err != nil {
			writeJSON(w, 200, incompleteConflict(pull, "source_repository_unavailable", "The source repository is unavailable."))
			return
		}
		targetRepo, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		analysis, err := analyzePullConflict(r.Context(), pull, store, sourceRepo, targetRepo, checks)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "conflict_analysis_failed"})
			return
		}
		writeJSON(w, 200, analysis)
	}
}

func incompleteConflict(p pullrequests.PullRequest, code, message string) conflictAnalysis {
	return conflictAnalysis{Source: conflictSide{Revision: conflictRevision{RepositoryID: p.SourceRepositoryID, Branch: p.SourceBranch, CommitID: p.SourceCommitID}, Intent: intentFor(p)}, Target: conflictSide{Revision: conflictRevision{RepositoryID: p.RepositoryID, Branch: p.TargetBranch, CommitID: p.TargetCommitID}}, Conflicts: []conflictItem{}, AffectedChecks: []conflictCheck{}, Blockers: []readinessBlocker{{Code: code, Message: message}}, MutatesBranches: false}
}

func analyzePullConflict(ctx context.Context, pull pullrequests.PullRequest, store pullRequestStore, sourceRepo, targetRepo *storage.Repository, checks readinessCheckStore) (conflictAnalysis, error) {
	result := incompleteConflict(pull, "", "")
	result.Blockers = []readinessBlocker{}
	result.Complete = true
	result.Source.Revision = inspectConflictRevision(sourceRepo, pull.SourceRepositoryID, pull.SourceBranch, pull.SourceCommitID)
	result.Target.Revision = inspectConflictRevision(targetRepo, pull.RepositoryID, pull.TargetBranch, pull.TargetCommitID)
	result.Stale = result.Source.Revision.Stale || result.Target.Revision.Stale
	if result.Stale {
		result.Blockers = append(result.Blockers, readinessBlocker{Code: "revision_changed", Message: "A selected branch no longer points to the analyzed revision."})
	}
	base, err := mergeBase(ctx, targetRepo, sourceRepo, pull.TargetCommitID, pull.SourceCommitID)
	if err != nil {
		result.Complete = false
		result.Blockers = append(result.Blockers, readinessBlocker{Code: "merge_base_unavailable", Message: "The common base could not be determined."})
		return result, nil
	}
	result.BaseCommitID = base
	result.Source.Commits, _ = commitsBetweenRepositories(sourceRepo, targetRepo, storage.ObjectID(pull.SourceCommitID), storage.ObjectID(base))
	result.Target.Commits, _ = commitsBetween(targetRepo, storage.ObjectID(pull.TargetCommitID), storage.ObjectID(base))
	sourceFiles, err := filesBetweenRepositories(sourceRepo, targetRepo, storage.ObjectID(pull.SourceCommitID), storage.ObjectID(base))
	if err != nil {
		return result, err
	}
	targetFiles, err := filesBetween(targetRepo, storage.ObjectID(pull.TargetCommitID), storage.ObjectID(base))
	if err != nil {
		return result, err
	}
	textual, _ := textualConflictPaths(ctx, targetRepo, sourceRepo, pull.TargetCommitID, pull.SourceCommitID)
	targetByPath := map[string]pullRequestFileChange{}
	for _, f := range targetFiles {
		targetByPath[f.Path] = f
	}
	for _, sf := range sourceFiles {
		if tf, found := targetByPath[sf.Path]; found {
			kind := "semantic"
			detail := "Both revisions independently change this file; combined behavior needs verification."
			if textual[sf.Path] {
				kind = "textual"
				detail = "Git reports overlapping text that cannot be merged automatically."
			} else if structuralPath(sf.Path) {
				kind = "structural"
				detail = "Both revisions change the same schema or interface definition."
			}
			s, t := sf, tf
			result.Conflicts = append(result.Conflicts, conflictItem{Kind: kind, Path: sf.Path, Detail: detail, SourceChange: &s, TargetChange: &t})
		}
	}
	// A shared declared symbol across different files is useful semantic evidence even when Git merges cleanly.
	sourceSymbols := symbolsByName(sourceRepo, sourceFiles)
	targetSymbols := symbolsByName(targetRepo, targetFiles)
	for name, sp := range sourceSymbols {
		if tp, found := targetSymbols[name]; found && sp != tp {
			result.Conflicts = append(result.Conflicts, conflictItem{Kind: "semantic", Symbol: name, Detail: "Both revisions independently change a declaration with this name in different files (" + sp + " and " + tp + ")."})
		}
	}
	sort.Slice(result.Conflicts, func(i, j int) bool {
		return result.Conflicts[i].Kind+result.Conflicts[i].Path+result.Conflicts[i].Symbol < result.Conflicts[j].Kind+result.Conflicts[j].Path+result.Conflicts[j].Symbol
	})
	all, _ := store.List(pull.RepositoryID)
	for _, p := range all {
		if p.ID != pull.ID && p.SourceCommitID == pull.TargetCommitID {
			result.Target.Intent = intentFor(p)
			break
		}
	}
	if checks != nil {
		runs, _ := checks.List(pull.RepositoryID, pull.ID)
		for _, run := range runs {
			if run.CommitID == pull.SourceCommitID {
				status := string(run.State)
				detail := "This exact-revision check may be affected by the combined changes."
				if run.State == checkruns.Failed {
					detail = "This exact-revision check failed and is independent semantic incompatibility evidence."
				}
				result.AffectedChecks = append(result.AffectedChecks, conflictCheck{Name: run.Definition.Name, Status: status, RunID: run.ID, CommitID: run.CommitID, Detail: detail})
			}
		}
	}
	if len(result.Conflicts) == 0 {
		result.Blockers = append(result.Blockers, readinessBlocker{Code: "no_detected_conflict", Message: "No textual, structural, or heuristic semantic incompatibility was detected for these exact revisions."})
	}
	return result, nil
}

func intentFor(p pullrequests.PullRequest) conflictIntent {
	criteria := []pullrequests.CriterionStatus{}
	if p.DeliveryEvidence != nil {
		criteria = append(criteria, p.DeliveryEvidence.CompletionCriteria...)
	}
	if p.ContributionContext != nil {
		criteria = append(criteria, p.ContributionContext.AcceptanceCriteria...)
	}
	return conflictIntent{PullRequestID: p.ID, ProposalID: p.ProposalID, TaskID: p.TaskID, OwnerID: p.AuthorID, Title: p.Title, Description: p.Body, DiscussionURL: "/repositories/" + p.RepositoryID + "?view=pulls&pull=" + p.ID + "&section=discussion", AcceptanceCriteria: criteria}
}
func inspectConflictRevision(repo *storage.Repository, repositoryID, branch, selected string) conflictRevision {
	r := conflictRevision{RepositoryID: repositoryID, Branch: branch, CommitID: selected}
	if ref, err := repo.ReadReference(storage.ReferenceName("refs/heads/" + branch)); err == nil {
		r.LiveCommitID = string(ref.ObjectID)
		r.Stale = r.LiveCommitID != selected
	} else {
		r.Stale = true
	}
	return r
}
func mergeBase(ctx context.Context, target, source *storage.Repository, a, b string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "--git-dir="+target.GitDir(), "merge-base", a, b)
	cmd.Env = append(os.Environ(), "GIT_ALTERNATE_OBJECT_DIRECTORIES="+filepath.Join(source.GitDir(), "objects"))
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
func textualConflictPaths(ctx context.Context, target, source *storage.Repository, a, b string) (map[string]bool, error) {
	dir, err := os.MkdirTemp("", "conflict-evidence-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	_ = os.MkdirAll(filepath.Join(dir, "pack"), 0750)
	cmd := exec.CommandContext(ctx, "git", "--git-dir="+target.GitDir(), "merge-tree", "--write-tree", "--name-only", a, b)
	cmd.Env = append(os.Environ(), "GIT_OBJECT_DIRECTORY="+dir, "GIT_ALTERNATE_OBJECT_DIRECTORIES="+filepath.Join(target.GitDir(), "objects")+string(os.PathListSeparator)+filepath.Join(source.GitDir(), "objects"))
	out, runErr := cmd.CombinedOutput()
	paths := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, " ") && strings.Contains(line, ".") {
			paths[line] = true
		}
	}
	return paths, runErr
}
func structuralPath(path string) bool {
	p := strings.ToLower(path)
	return strings.Contains(p, "schema") || strings.Contains(p, "openapi") || strings.Contains(p, "interface") || strings.HasSuffix(p, ".proto") || strings.HasSuffix(p, ".graphql")
}
func symbolsByName(repo *storage.Repository, files []pullRequestFileChange) map[string]string {
	out := map[string]string{}
	for _, f := range files {
		if f.NewObjectID == "" || f.Binary {
			continue
		}
		obj, err := repo.ReadObject(storage.ObjectID(f.NewObjectID))
		if err != nil {
			continue
		}
		for _, m := range declaredSymbol.FindAllSubmatch(obj.Content, -1) {
			out[string(m[1])] = f.Path
		}
	}
	return out
}
