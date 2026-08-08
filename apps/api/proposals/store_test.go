package proposals

import (
	"errors"
	"testing"
)

func TestProposalAndConversationSurviveReopen(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := store.Create("repository", "author", "Explore agents", "Define the problem first.")
	if err != nil {
		t.Fatal(err)
	}
	comment, err := store.AddComment("repository", proposal.ID, "maintainer", "What outcome should we measure?")
	if err != nil {
		t.Fatal(err)
	}
	closed, err := store.Close("repository", proposal.ID, "author")
	if err != nil {
		t.Fatal(err)
	}
	if closed.State != Closed || closed.ClosedByID != "author" || closed.ClosedAt == nil {
		t.Fatalf("closed proposal = %#v", closed)
	}

	reopened, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get("repository", proposal.ID)
	if err != nil || got.AuthorID != "author" || got.Title != "Explore agents" {
		t.Fatalf("reopened proposal = %#v, %v", got, err)
	}
	comments, err := reopened.ListComments("repository", proposal.ID)
	if err != nil || len(comments) != 1 || comments[0].ID != comment.ID || comments[0].AuthorID != "maintainer" {
		t.Fatalf("reopened comments = %#v, %v", comments, err)
	}
}

func TestProposalValidation(t *testing.T) {
	store, _ := New(t.TempDir())
	if _, err := store.Create("repository", "author", "  ", ""); err != ErrInvalid {
		t.Fatalf("empty title error = %v", err)
	}
	proposal, _ := store.Create("repository", "author", "Valid", "")
	if _, err := store.AddComment("repository", proposal.ID, "author", "  "); err != ErrInvalidComment {
		t.Fatalf("empty comment error = %v", err)
	}
}

func TestProposalPlanOrdersDependenciesReadinessAndHistory(t *testing.T) {
	root := t.TempDir()
	store, _ := New(root)
	proposal, _ := store.Create("repository", "author", "Ship setup", "Agree on a reusable setup path.")
	first, err := store.CreateTask("repository", proposal.ID, "author", TaskInput{Title: "Define contract", Outcome: "The supported setup contract is documented."})
	if err != nil || !first.Ready || first.Position != 1 {
		t.Fatalf("first task = %#v, %v", first, err)
	}
	second, err := store.CreateTask("repository", proposal.ID, "maintainer", TaskInput{Title: "Implement setup", Outcome: "A newcomer can complete setup.", Position: 1, DependsOn: []string{first.ID}, DiscussionCommentIDs: []string{"comment"}})
	if err != nil || second.Ready || second.Position != 1 {
		t.Fatalf("dependent task = %#v, %v", second, err)
	}
	plan, _ := store.GetPlan("repository", proposal.ID)
	if len(plan.Tasks) != 2 || plan.Tasks[1].ID != first.ID || len(plan.History) != 2 || plan.History[1].ActorID != "maintainer" {
		t.Fatalf("plan = %#v", plan)
	}
	first, err = store.UpdateTask("repository", proposal.ID, first.ID, "reviewer", TaskInput{Title: first.Title, Outcome: first.Outcome, Position: 1, Status: TaskCompleted})
	if err != nil || first.Status != TaskCompleted {
		t.Fatalf("completed task = %#v, %v", first, err)
	}
	plan, _ = store.GetPlan("repository", proposal.ID)
	if !plan.Tasks[1].Ready || plan.History[2].Task.Status != TaskCompleted || plan.History[2].ActorID != "reviewer" {
		t.Fatalf("completed plan = %#v", plan)
	}
	reopened, _ := New(root)
	plan, err = reopened.GetPlan("repository", proposal.ID)
	if err != nil || len(plan.Tasks) != 2 || len(plan.History) != 3 {
		t.Fatalf("reopened plan = %#v, %v", plan, err)
	}
}

func TestProposalPlanRejectsUnknownAndCyclicDependencies(t *testing.T) {
	store, _ := New(t.TempDir())
	proposal, _ := store.Create("repository", "author", "Plan", "")
	first, _ := store.CreateTask("repository", proposal.ID, "author", TaskInput{Title: "One", Outcome: "One is done."})
	second, _ := store.CreateTask("repository", proposal.ID, "author", TaskInput{Title: "Two", Outcome: "Two is done.", DependsOn: []string{first.ID}})
	_, err := store.UpdateTask("repository", proposal.ID, first.ID, "author", TaskInput{Title: first.Title, Outcome: first.Outcome, DependsOn: []string{second.ID}})
	if !errors.Is(err, ErrInvalidDependency) {
		t.Fatalf("cyclic dependency error = %v", err)
	}
	_, err = store.CreateTask("repository", proposal.ID, "author", TaskInput{Title: "Three", Outcome: "Three is done.", DependsOn: []string{"missing"}})
	if !errors.Is(err, ErrInvalidDependency) {
		t.Fatalf("unknown dependency error = %v", err)
	}
}

func TestProposalTaskAssignmentIsExclusiveScopedAndAttributable(t *testing.T) {
	store, _ := New(t.TempDir())
	proposal, _ := store.Create("repository", "author", "Plan", "")
	task, _ := store.CreateTask("repository", proposal.ID, "author", TaskInput{Title: "Implement", Outcome: "The change is reviewable."})
	assigned, err := store.AssignTask("repository", proposal.ID, task.ID, "planner", "", AssignmentInput{Kind: AgentAssignee, AssigneeID: "codex", Mandate: "Implement only this outcome.", RepositoryID: "repository", BaseRevision: "abc"})
	if err != nil || assigned.Assignment == nil || assigned.Assignment.CredentialIssued || len(assigned.Assignment.Permissions) != 2 {
		t.Fatalf("assignment = %#v, %v", assigned, err)
	}
	if _, err := store.AssignTask("repository", proposal.ID, task.ID, "racer", "", AssignmentInput{Kind: HumanAssignee, AssigneeID: "human", Mandate: "Take over.", RepositoryID: "repository", BaseRevision: "def"}); !errors.Is(err, ErrTaskAssigned) {
		t.Fatalf("concurrent claim error = %v", err)
	}
	reassigned, err := store.AssignTask("repository", proposal.ID, task.ID, "maintainer", assigned.Assignment.ID, AssignmentInput{Kind: HumanAssignee, AssigneeID: "human", Mandate: "Deliver the same outcome.", RepositoryID: "repository", BaseRevision: "def"})
	if err != nil || reassigned.Assignment.AssigneeID != "human" || reassigned.Assignment.AssignedByID != "maintainer" {
		t.Fatalf("reassignment = %#v, %v", reassigned, err)
	}
	if _, err := store.RevokeTaskAssignment("repository", proposal.ID, task.ID, "stale", assigned.Assignment.ID); !errors.Is(err, ErrAssignmentConflict) {
		t.Fatalf("stale revocation error = %v", err)
	}
	revoked, err := store.RevokeTaskAssignment("repository", proposal.ID, task.ID, "maintainer", reassigned.Assignment.ID)
	if err != nil || revoked.Assignment != nil {
		t.Fatalf("revoked = %#v, %v", revoked, err)
	}
	plan, _ := store.GetPlan("repository", proposal.ID)
	if len(plan.History) != 4 || plan.History[1].Action != "task.assigned" || plan.History[2].Action != "task.reassigned" || plan.History[3].Action != "task.assignment_revoked" {
		t.Fatalf("history = %#v", plan.History)
	}
}

func TestProposalTaskAssignmentRequiresReadyTask(t *testing.T) {
	store, _ := New(t.TempDir())
	proposal, _ := store.Create("repository", "author", "Plan", "")
	first, _ := store.CreateTask("repository", proposal.ID, "author", TaskInput{Title: "First", Outcome: "First done."})
	blocked, _ := store.CreateTask("repository", proposal.ID, "author", TaskInput{Title: "Blocked", Outcome: "Blocked done.", DependsOn: []string{first.ID}})
	_, err := store.AssignTask("repository", proposal.ID, blocked.ID, "author", "", AssignmentInput{Kind: HumanAssignee, AssigneeID: "author", Mandate: "Work", RepositoryID: "repository", BaseRevision: "abc"})
	if !errors.Is(err, ErrTaskNotReady) {
		t.Fatalf("blocked assignment error = %v", err)
	}
}

func TestDependentReadinessFollowsMergedContributionAndPersists(t *testing.T) {
	root := t.TempDir()
	store, _ := New(root)
	proposal, _ := store.Create("repository", "author", "Plan", "")
	first, _ := store.CreateTask("repository", proposal.ID, "author", TaskInput{Title: "First", Outcome: "First lands."})
	second, _ := store.CreateTask("repository", proposal.ID, "author", TaskInput{Title: "Second", Outcome: "Second lands.", DependsOn: []string{first.ID}})
	first, _ = store.PublishTaskContribution("repository", proposal.ID, first.ID, "author", TaskContribution{PullRequestID: "pull", SourceCommitID: "source", TargetCommitID: "target", Status: ContributionReview})
	if first.Status != TaskReview {
		t.Fatalf("review contribution = %#v", first)
	}
	_, _ = store.UpdateTaskContribution("repository", proposal.ID, first.ID, "pull", "maintainer", ContributionMerged)
	plan, _ := store.GetPlan("repository", proposal.ID)
	if !taskByID(plan.Tasks, second.ID).Ready || len(taskByID(plan.Tasks, second.ID).BlockedBy) != 0 {
		t.Fatalf("ready dependent = %#v", plan.Tasks)
	}
	_, _ = store.UpdateTaskContribution("repository", proposal.ID, first.ID, "pull", "maintainer", ContributionClosed)
	plan, _ = store.GetPlan("repository", proposal.ID)
	dependent := taskByID(plan.Tasks, second.ID)
	if dependent.Ready || len(dependent.BlockedBy) != 1 || dependent.BlockedBy[0] != first.ID {
		t.Fatalf("blocked dependent = %#v", dependent)
	}
	reopened, _ := New(root)
	plan, _ = reopened.GetPlan("repository", proposal.ID)
	if taskByID(plan.Tasks, second.ID).Ready {
		t.Fatal("reopened dependent unexpectedly ready")
	}
}

func TestAssignmentRebaseRejectsActiveWorkAndRetainsHistory(t *testing.T) {
	store, _ := New(t.TempDir())
	proposal, _ := store.Create("repository", "author", "Plan", "")
	task, _ := store.CreateTask("repository", proposal.ID, "author", TaskInput{Title: "Implement", Outcome: "Work lands."})
	task, _ = store.AssignTask("repository", proposal.ID, task.ID, "planner", "", AssignmentInput{Kind: AgentAssignee, AssigneeID: "codex", Mandate: "Implement", RepositoryID: "repository", BaseRevision: "old"})
	rebased, err := store.RebaseTaskAssignment("repository", proposal.ID, task.ID, "planner", task.Assignment.ID, "new")
	if err != nil || rebased.Assignment.BaseRevision != "new" {
		t.Fatalf("rebased = %#v, %v", rebased, err)
	}
	started, _ := store.StartAssignedTask("repository", proposal.ID, task.ID, "planner", task.Assignment.ID, "session", "codex/task")
	if _, err := store.RebaseTaskAssignment("repository", proposal.ID, task.ID, "planner", task.Assignment.ID, "later"); !errors.Is(err, ErrActiveTaskConflict) {
		t.Fatalf("active rebase error = %v", err)
	}
	if _, err := store.UpdateTask("repository", proposal.ID, task.ID, "planner", TaskInput{Title: started.Title, Outcome: "Different", Status: started.Status}); !errors.Is(err, ErrActiveTaskConflict) {
		t.Fatalf("active plan edit error = %v", err)
	}
	plan, _ := store.GetPlan("repository", proposal.ID)
	if plan.History[2].Action != "task.base_rebased" {
		t.Fatalf("history = %#v", plan.History)
	}
}
