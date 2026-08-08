package proposals

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidTask        = errors.New("invalid proposal task")
	ErrInvalidDependency  = errors.New("invalid proposal task dependency")
	ErrTaskNotReady       = errors.New("proposal task is not ready")
	ErrTaskAssigned       = errors.New("proposal task is already assigned")
	ErrAssignmentConflict = errors.New("proposal task assignment changed")
)

type TaskStatus string

const (
	TaskPlanned    TaskStatus = "planned"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskCanceled   TaskStatus = "canceled"
)

type Task struct {
	ID                   string          `json:"id"`
	ProposalID           string          `json:"proposal_id"`
	Title                string          `json:"title"`
	Outcome              string          `json:"outcome"`
	Position             int             `json:"position"`
	Status               TaskStatus      `json:"status"`
	DependsOn            []string        `json:"depends_on"`
	DiscussionCommentIDs []string        `json:"discussion_comment_ids"`
	CreatedByID          string          `json:"created_by_id"`
	UpdatedByID          string          `json:"updated_by_id"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
	Ready                bool            `json:"ready"`
	Assignment           *TaskAssignment `json:"assignment,omitempty"`
}

type AssigneeKind string

const (
	HumanAssignee AssigneeKind = "human"
	AgentAssignee AssigneeKind = "agent"
)

// TaskAssignment is the pre-work responsibility contract. It describes the
// exact context and authority a worker will receive; credentials and execution
// are deliberately created only when work starts.
type TaskAssignment struct {
	ID               string       `json:"id"`
	Kind             AssigneeKind `json:"kind"`
	AssigneeID       string       `json:"assignee_id"`
	Mandate          string       `json:"mandate"`
	RepositoryID     string       `json:"repository_id"`
	BaseRevision     string       `json:"base_revision"`
	Permissions      []string     `json:"permissions"`
	CredentialIssued bool         `json:"credential_issued"`
	AssignedByID     string       `json:"assigned_by_id"`
	AssignedAt       time.Time    `json:"assigned_at"`
	SessionID        string       `json:"session_id,omitempty"`
	WorkingBranch    string       `json:"working_branch,omitempty"`
	StartedAt        *time.Time   `json:"started_at,omitempty"`
}

func (s *Store) StartAssignedTask(repositoryID, proposalID, taskID, actorID, expectedAssignmentID, sessionID, workingBranch string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks, err := s.readTasks(repositoryID, proposalID)
	if err != nil {
		return Task{}, err
	}
	index := taskIndex(tasks, taskID)
	if index < 0 {
		return Task{}, ErrNotFound
	}
	current := tasks[index]
	if current.Assignment == nil || current.Assignment.ID != expectedAssignmentID {
		return Task{}, ErrAssignmentConflict
	}
	if current.Assignment.Kind != AgentAssignee || current.Assignment.SessionID != "" || sessionID == "" || workingBranch == "" {
		return Task{}, ErrTaskAssigned
	}
	now := s.now().UTC()
	current.Assignment.CredentialIssued = true
	current.Assignment.SessionID = sessionID
	current.Assignment.WorkingBranch = workingBranch
	current.Assignment.StartedAt = &now
	current.Status, current.Ready = TaskInProgress, false
	current.UpdatedByID, current.UpdatedAt = actorID, now
	tasks[index] = current
	if err := s.writeTasks(repositoryID, proposalID, tasks); err != nil {
		return Task{}, err
	}
	if err := s.appendPlanEvent(repositoryID, proposalID, actorID, "task.started", current); err != nil {
		return Task{}, err
	}
	return current, nil
}

type AssignmentInput struct {
	Kind         AssigneeKind
	AssigneeID   string
	Mandate      string
	RepositoryID string
	BaseRevision string
}

type TaskInput struct {
	Title                string
	Outcome              string
	Position             int
	Status               TaskStatus
	DependsOn            []string
	DiscussionCommentIDs []string
}

type PlanEvent struct {
	ID         string    `json:"id"`
	ProposalID string    `json:"proposal_id"`
	TaskID     string    `json:"task_id"`
	ActorID    string    `json:"actor_id"`
	Action     string    `json:"action"`
	Task       Task      `json:"task"`
	CreatedAt  time.Time `json:"created_at"`
}

type Plan struct {
	ProposalID string      `json:"proposal_id"`
	Tasks      []Task      `json:"tasks"`
	History    []PlanEvent `json:"history"`
}

func (s *Store) GetPlan(repositoryID, proposalID string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.readProposal(repositoryID, proposalID); err != nil {
		return Plan{}, err
	}
	tasks, err := s.readTasks(repositoryID, proposalID)
	if err != nil {
		return Plan{}, err
	}
	events, err := s.readPlanEvents(repositoryID, proposalID)
	if err != nil {
		return Plan{}, err
	}
	deriveReadiness(tasks)
	return Plan{ProposalID: proposalID, Tasks: tasks, History: events}, nil
}

func (s *Store) CreateTask(repositoryID, proposalID, actorID string, input TaskInput) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.readProposal(repositoryID, proposalID); err != nil {
		return Task{}, err
	}
	tasks, err := s.readTasks(repositoryID, proposalID)
	if err != nil {
		return Task{}, err
	}
	input = normalizeTaskInput(input)
	if input.Position == 0 {
		input.Position = len(tasks) + 1
	}
	if err := validateTaskInput(input, len(tasks)+1); err != nil {
		return Task{}, err
	}
	if err := validateDependencies("", input.DependsOn, tasks); err != nil {
		return Task{}, err
	}
	id, err := newID()
	if err != nil {
		return Task{}, err
	}
	now := s.now().UTC()
	task := Task{ID: id, ProposalID: proposalID, Title: input.Title, Outcome: input.Outcome, Status: input.Status, DependsOn: input.DependsOn, DiscussionCommentIDs: input.DiscussionCommentIDs, CreatedByID: actorID, UpdatedByID: actorID, CreatedAt: now, UpdatedAt: now}
	tasks = insertTask(tasks, task, input.Position)
	deriveReadiness(tasks)
	created := taskByID(tasks, id)
	if err := s.writeTasks(repositoryID, proposalID, tasks); err != nil {
		return Task{}, err
	}
	if err := s.appendPlanEvent(repositoryID, proposalID, actorID, "task.created", created); err != nil {
		return Task{}, err
	}
	return created, nil
}

func (s *Store) UpdateTask(repositoryID, proposalID, taskID, actorID string, input TaskInput) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.readProposal(repositoryID, proposalID); err != nil {
		return Task{}, err
	}
	tasks, err := s.readTasks(repositoryID, proposalID)
	if err != nil {
		return Task{}, err
	}
	index := -1
	for i := range tasks {
		if tasks[i].ID == taskID {
			index = i
			break
		}
	}
	if index < 0 {
		return Task{}, ErrNotFound
	}
	input = normalizeTaskInput(input)
	if input.Position == 0 {
		input.Position = tasks[index].Position
	}
	if err := validateTaskInput(input, len(tasks)); err != nil {
		return Task{}, err
	}
	if err := validateDependencies(taskID, input.DependsOn, tasks); err != nil {
		return Task{}, err
	}
	current := tasks[index]
	current.Title, current.Outcome, current.Status = input.Title, input.Outcome, input.Status
	current.DependsOn, current.DiscussionCommentIDs = input.DependsOn, input.DiscussionCommentIDs
	current.UpdatedByID, current.UpdatedAt = actorID, s.now().UTC()
	tasks = append(tasks[:index], tasks[index+1:]...)
	tasks = insertTask(tasks, current, input.Position)
	if hasDependencyCycle(tasks) {
		return Task{}, ErrInvalidDependency
	}
	deriveReadiness(tasks)
	if err := s.writeTasks(repositoryID, proposalID, tasks); err != nil {
		return Task{}, err
	}
	updated := taskByID(tasks, taskID)
	if err := s.appendPlanEvent(repositoryID, proposalID, actorID, "task.updated", updated); err != nil {
		return Task{}, err
	}
	return taskByID(tasks, taskID), nil
}

func (s *Store) AssignTask(repositoryID, proposalID, taskID, actorID, expectedAssignmentID string, input AssignmentInput) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks, err := s.readTasks(repositoryID, proposalID)
	if err != nil {
		return Task{}, err
	}
	index := taskIndex(tasks, taskID)
	if index < 0 {
		return Task{}, ErrNotFound
	}
	deriveReadiness(tasks)
	current := tasks[index]
	if !current.Ready {
		return Task{}, ErrTaskNotReady
	}
	if current.Assignment != nil && expectedAssignmentID == "" {
		return Task{}, ErrTaskAssigned
	}
	if current.Assignment != nil && current.Assignment.ID != expectedAssignmentID {
		return Task{}, ErrAssignmentConflict
	}
	if current.Assignment == nil && expectedAssignmentID != "" {
		return Task{}, ErrAssignmentConflict
	}
	input.AssigneeID, input.Mandate, input.RepositoryID, input.BaseRevision = strings.TrimSpace(input.AssigneeID), strings.TrimSpace(input.Mandate), strings.TrimSpace(input.RepositoryID), strings.TrimSpace(input.BaseRevision)
	if (input.Kind != HumanAssignee && input.Kind != AgentAssignee) || input.AssigneeID == "" || len(input.AssigneeID) > 100 || input.Mandate == "" || len(input.Mandate) > 4096 || input.RepositoryID == "" || input.BaseRevision == "" {
		return Task{}, ErrInvalidTask
	}
	id, err := newID()
	if err != nil {
		return Task{}, err
	}
	now := s.now().UTC()
	current.Assignment = &TaskAssignment{ID: id, Kind: input.Kind, AssigneeID: input.AssigneeID, Mandate: input.Mandate, RepositoryID: input.RepositoryID, BaseRevision: input.BaseRevision, Permissions: []string{"contents:read", "candidate_branch:write"}, CredentialIssued: false, AssignedByID: actorID, AssignedAt: now}
	current.UpdatedByID, current.UpdatedAt = actorID, now
	tasks[index] = current
	if err := s.writeTasks(repositoryID, proposalID, tasks); err != nil {
		return Task{}, err
	}
	action := "task.assigned"
	if expectedAssignmentID != "" {
		action = "task.reassigned"
	}
	if err := s.appendPlanEvent(repositoryID, proposalID, actorID, action, current); err != nil {
		return Task{}, err
	}
	return current, nil
}

func (s *Store) RevokeTaskAssignment(repositoryID, proposalID, taskID, actorID, expectedAssignmentID string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks, err := s.readTasks(repositoryID, proposalID)
	if err != nil {
		return Task{}, err
	}
	index := taskIndex(tasks, taskID)
	if index < 0 {
		return Task{}, ErrNotFound
	}
	current := tasks[index]
	if current.Assignment == nil || expectedAssignmentID == "" || current.Assignment.ID != expectedAssignmentID {
		return Task{}, ErrAssignmentConflict
	}
	if current.Assignment.SessionID != "" {
		return Task{}, ErrTaskAssigned
	}
	revokedSnapshot := current
	current.Assignment = nil
	current.UpdatedByID, current.UpdatedAt = actorID, s.now().UTC()
	tasks[index] = current
	if err := s.writeTasks(repositoryID, proposalID, tasks); err != nil {
		return Task{}, err
	}
	revokedSnapshot.UpdatedByID, revokedSnapshot.UpdatedAt = actorID, current.UpdatedAt
	if err := s.appendPlanEvent(repositoryID, proposalID, actorID, "task.assignment_revoked", revokedSnapshot); err != nil {
		return Task{}, err
	}
	return current, nil
}

func taskIndex(tasks []Task, id string) int {
	for i := range tasks {
		if tasks[i].ID == id {
			return i
		}
	}
	return -1
}

func normalizeTaskInput(input TaskInput) TaskInput {
	input.Title, input.Outcome = strings.TrimSpace(input.Title), strings.TrimSpace(input.Outcome)
	if input.Status == "" {
		input.Status = TaskPlanned
	}
	input.DependsOn = uniqueStrings(input.DependsOn)
	input.DiscussionCommentIDs = uniqueStrings(input.DiscussionCommentIDs)
	return input
}

func validateTaskInput(input TaskInput, maximumPosition int) error {
	if input.Title == "" || len(input.Title) > 200 || input.Outcome == "" || len(input.Outcome) > 4096 || input.Position < 1 || input.Position > maximumPosition || !validTaskStatus(input.Status) {
		return ErrInvalidTask
	}
	return nil
}

func validTaskStatus(status TaskStatus) bool {
	return status == TaskPlanned || status == TaskInProgress || status == TaskCompleted || status == TaskCanceled
}

func validateDependencies(taskID string, dependencies []string, tasks []Task) error {
	known := map[string]bool{}
	for _, task := range tasks {
		known[task.ID] = true
	}
	for _, dependency := range dependencies {
		if dependency == taskID || !known[dependency] {
			return ErrInvalidDependency
		}
	}
	return nil
}

func hasDependencyCycle(tasks []Task) bool {
	dependencies := map[string][]string{}
	for _, task := range tasks {
		dependencies[task.ID] = task.DependsOn
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return true
		}
		if visited[id] {
			return false
		}
		visiting[id] = true
		for _, dependency := range dependencies[id] {
			if visit(dependency) {
				return true
			}
		}
		visiting[id] = false
		visited[id] = true
		return false
	}
	for id := range dependencies {
		if visit(id) {
			return true
		}
	}
	return false
}

func insertTask(tasks []Task, task Task, position int) []Task {
	index := position - 1
	if index > len(tasks) {
		index = len(tasks)
	}
	tasks = append(tasks, Task{})
	copy(tasks[index+1:], tasks[index:])
	tasks[index] = task
	for i := range tasks {
		tasks[i].Position = i + 1
		tasks[i].Ready = false
	}
	return tasks
}

func deriveReadiness(tasks []Task) {
	completed := map[string]bool{}
	for _, task := range tasks {
		completed[task.ID] = task.Status == TaskCompleted
	}
	for i := range tasks {
		ready := tasks[i].Status == TaskPlanned
		for _, dependency := range tasks[i].DependsOn {
			ready = ready && completed[dependency]
		}
		tasks[i].Ready = ready
	}
}

func taskByID(tasks []Task, id string) Task {
	for _, task := range tasks {
		if task.ID == id {
			return task
		}
	}
	return Task{}
}
func uniqueStrings(values []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func (s *Store) planDir(repositoryID, proposalID string) string {
	return filepath.Join(s.root, repositoryID, proposalID, "plan")
}
func (s *Store) readTasks(repositoryID, proposalID string) ([]Task, error) {
	var tasks []Task
	data, err := os.ReadFile(filepath.Join(s.planDir(repositoryID, proposalID), "tasks.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return []Task{}, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, errors.New("invalid stored proposal plan")
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Position < tasks[j].Position })
	for i, task := range tasks {
		if task.ID == "" || task.ProposalID != proposalID || task.Position != i+1 || !validTaskStatus(task.Status) {
			return nil, errors.New("invalid stored proposal plan")
		}
	}
	return tasks, nil
}
func (s *Store) writeTasks(repositoryID, proposalID string, tasks []Task) error {
	return s.writeJSON(filepath.Join(s.planDir(repositoryID, proposalID), "tasks.json"), tasks, false)
}
func (s *Store) appendPlanEvent(repositoryID, proposalID, actorID, action string, task Task) error {
	id, err := newID()
	if err != nil {
		return err
	}
	event := PlanEvent{ID: id, ProposalID: proposalID, TaskID: task.ID, ActorID: actorID, Action: action, Task: task, CreatedAt: s.now().UTC()}
	return s.writeJSON(filepath.Join(s.planDir(repositoryID, proposalID), "history", fmt.Sprintf("%s-%s.json", event.CreatedAt.Format("20060102T150405.000000000"), id)), event, true)
}
func (s *Store) readPlanEvents(repositoryID, proposalID string) ([]PlanEvent, error) {
	dir := filepath.Join(s.planDir(repositoryID, proposalID), "history")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []PlanEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	events := []PlanEvent{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var event PlanEvent
		if json.Unmarshal(data, &event) != nil || event.ProposalID != proposalID || event.ActorID == "" {
			return nil, errors.New("invalid stored proposal plan history")
		}
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].CreatedAt.Equal(events[j].CreatedAt) {
			return events[i].ID < events[j].ID
		}
		return events[i].CreatedAt.Before(events[j].CreatedAt)
	})
	return events, nil
}
