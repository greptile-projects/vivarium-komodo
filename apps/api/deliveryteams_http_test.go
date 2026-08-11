package main

import (
	"errors"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deliveryteams"
	"github.com/greptile-projects/vivarium-komodo/apps/api/investigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

type deliveryChangeContexts struct{ item changesessions.Session }

func (f deliveryChangeContexts) Get(repository, parent, id string) (changesessions.Session, error) {
	if f.item.RepositoryID == repository && f.item.PullRequestID == parent && f.item.ID == id {
		return f.item, nil
	}
	return changesessions.Session{}, errors.New("missing")
}

type deliveryInvestigationContexts struct{ item investigations.Investigation }

func (f deliveryInvestigationContexts) Get(repository, id string) (investigations.Investigation, error) {
	if f.item.RepositoryID == repository && f.item.ID == id {
		return f.item, nil
	}
	return investigations.Investigation{}, errors.New("missing")
}

type deliveryDecisionContexts struct{ item decisions.Decision }

func (f deliveryDecisionContexts) Get(repository, id string) (decisions.Decision, error) {
	if f.item.RepositoryID == repository && f.item.ID == id {
		return f.item, nil
	}
	return decisions.Decision{}, errors.New("missing")
}

type deliveryWorkspaceContexts struct{ item workspaces.Workspace }

func (f deliveryWorkspaceContexts) Get(repository, id string) (workspaces.Workspace, error) {
	if f.item.RepositoryID == repository && f.item.ID == id {
		return f.item, nil
	}
	return workspaces.Workspace{}, errors.New("missing")
}

func TestDeliveryContextMustResolveAtExactRevision(t *testing.T) {
	revision := "1111111111111111111111111111111111111111"
	stores := deliveryExecutionStores{
		changes:        deliveryChangeContexts{changesessions.Session{ID: "session", RepositoryID: "repo", PullRequestID: "pull", SourceCommitID: revision}},
		investigations: deliveryInvestigationContexts{investigations.Investigation{ID: "investigation", RepositoryID: "repo", CommitID: revision}},
		decisions:      deliveryDecisionContexts{decisions.Decision{ID: "decision", RepositoryID: "repo", Alternatives: []decisions.Alternative{{Experiments: []decisions.Experiment{{ID: "experiment", Revision: revision}}}}}},
		workspaces:     deliveryWorkspaceContexts{workspaces.Workspace{ID: "workspace", RepositoryID: "repo", Revision: revision}},
	}
	contexts := []deliveryteams.ExecutionContext{
		{Kind: "change_session", ID: "session", ParentID: "pull", RepositoryID: "repo", Revision: revision},
		{Kind: "investigation", ID: "investigation", RepositoryID: "repo", Revision: revision},
		{Kind: "experiment", ID: "experiment", ParentID: "decision", RepositoryID: "repo", Revision: revision},
		{Kind: "workspace", ID: "workspace", RepositoryID: "repo", Revision: revision},
	}
	for _, context := range contexts {
		if !deliveryContextExists(context, stores) {
			t.Fatalf("context did not resolve: %#v", context)
		}
	}
	contexts[0].Revision = "2222222222222222222222222222222222222222"
	if deliveryContextExists(contexts[0], stores) {
		t.Fatal("mismatched revision resolved")
	}
	contexts[2].ParentID = "other-decision"
	if deliveryContextExists(contexts[2], stores) {
		t.Fatal("mismatched parent resolved")
	}
}
