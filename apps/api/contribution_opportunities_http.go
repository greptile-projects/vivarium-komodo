package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributionopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

type opportunityRepositories interface {
	contributorPathwayRepositories
	Fork(string, storage.ID, repositories.Metadata) (repositories.Repository, error)
}
type opportunityRunner interface {
	Definition(string, string) (workspaces.Definition, string, error)
	Start(workspaces.Workspace)
	WriteFile(workspaces.Workspace, string, string, []byte, bool, *string) (workspaces.Workspace, error)
}

func registerContributionOpportunitiesHTTP(mux *http.ServeMux, store *contributionopportunities.Store, repos opportunityRepositories, credentials authStore, issueStore *issues.Store, proposalStore *proposals.Store, organizationStore *organizations.Store, pathways contributorPathwayStore, workspaceStore workspaceStore, runner opportunityRunner) {
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, bool) {
		_, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, write)
		return actor.UserID, ok
	}
	mux.HandleFunc("GET /repositories/{repository}/contribution-opportunities", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := access(w, r, false); !ok {
			return
		}
		d, e := store.List(r.PathValue("repository"))
		if opportunityError(w, e) {
			return
		}
		now := time.Now().UTC()
		active := map[string]contributionopportunities.Claim{}
		for _, c := range d.Claims {
			if c.ReleasedAt == nil && c.ExpiresAt.After(now) {
				active[c.OpportunityID] = c
			}
		}
		writeJSON(w, 200, map[string]any{"items": d.Opportunities, "active_claims": active, "reports": d.Reports, "grants_write_access": false})
	})
	mux.HandleFunc("POST /repositories/{repository}/contribution-opportunities", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		var in contributionopportunities.Input
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		currentRevision := ""
		if opened, openErr := repos.Open(repo.ID); openErr == nil {
			if branch, branchErr := opened.DefaultBranch(); branchErr == nil {
				if ref, refErr := opened.ReadReference(branch); refErr == nil {
					currentRevision = string(ref.ObjectID)
				}
			}
		}
		title, rev, state, ready, ok := resolveOpportunitySource(string(repo.ID), currentRevision, in.Source, issueStore, proposalStore, organizationStore)
		if !ok {
			writeJSON(w, 422, map[string]string{"error": "source_not_ready_or_visible"})
			return
		}
		o, e := store.Publish(string(repo.ID), actor.UserID, title, rev, state, ready, in)
		if opportunityError(w, e) {
			return
		}
		writeJSON(w, 201, o)
	})
	mux.HandleFunc("PUT /repositories/{repository}/contribution-opportunity-profile", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var p contributionopportunities.Profile
		if !readJSON(w, r, &p, 32<<10) {
			return
		}
		p, e := store.Profile(r.PathValue("repository"), actor, p)
		if opportunityError(w, e) {
			return
		}
		writeJSON(w, 200, p)
	})
	mux.HandleFunc("GET /repositories/{repository}/contribution-opportunity-matches", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := access(w, r, true)
		if !ok {
			return
		}
		m, e := store.Matches(r.PathValue("repository"), actor)
		if opportunityError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": m, "explanation": "Scores combine interest, skill, risk, readiness, and requested assistance. Missing skills remain visible rather than silently excluding work.", "grants_write_access": false})
	})
	mux.HandleFunc("POST /repositories/{repository}/contribution-opportunities/{opportunity}/claims", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Note  string `json:"note"`
			Hours int    `json:"hours"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		c, e := store.Claim(r.PathValue("repository"), r.PathValue("opportunity"), actor, in.Note, in.Hours)
		if opportunityError(w, e) {
			return
		}
		writeJSON(w, 201, map[string]any{"claim": c, "grants_write_access": false})
	})
	mux.HandleFunc("POST /repositories/{repository}/contribution-opportunity-claims/{claim}/release", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := access(w, r, true)
		if !ok {
			return
		}
		c, e := store.Release(r.PathValue("repository"), r.PathValue("claim"), actor)
		if opportunityError(w, e) {
			return
		}
		writeJSON(w, 200, c)
	})
	mux.HandleFunc("POST /repositories/{repository}/contribution-opportunities/{opportunity}/start", func(w http.ResponseWriter, r *http.Request) {
		upstream, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Name                     string `json:"name"`
			ResponseExpectationHours int    `json:"response_expectation_hours"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		if in.ResponseExpectationHours < 0 || in.ResponseExpectationHours > 168 {
			writeJSON(w, 422, map[string]string{"error": "invalid_response_expectation"})
			return
		}
		o, err := store.Get(string(upstream.ID), r.PathValue("opportunity"))
		if opportunityError(w, err) {
			return
		}
		data, _ := store.List(string(upstream.ID))
		claimed := false
		claimID := ""
		now := time.Now().UTC()
		for _, c := range data.Claims {
			if c.OpportunityID == o.ID && c.ActorID == actor.UserID && c.ReleasedAt == nil && c.ExpiresAt.After(now) {
				claimed = true
				claimID = c.ID
			}
		}
		if !claimed || !o.Ready {
			writeJSON(w, 409, map[string]string{"error": "active_claim_required"})
			return
		}
		opened, err := repos.Open(upstream.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if _, err = opened.ReadCommit(storage.ObjectID(o.Revision)); err != nil {
			writeJSON(w, 409, map[string]string{"error": "recorded_revision_unavailable"})
			return
		}
		for _, path := range o.SampleData {
			if !safeOpportunityPath(path) || !repositoryPathExists(opened, o.Revision, path) {
				writeJSON(w, 422, map[string]string{"error": "sample_data_unavailable"})
				return
			}
		}
		name := strings.TrimSpace(in.Name)
		if name == "" {
			name = upstream.Name + "-contribution"
		}
		fork, err := repos.Fork(actor.UserID, upstream.ID, repositories.Metadata{Name: name, Description: "Contribution fork for " + o.Title, Visibility: repositories.Private})
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		definition, digest, err := runner.Definition(string(fork.ID), o.Revision)
		if err != nil {
			writeJSON(w, 422, map[string]any{"error": "non_reproducible_workspace_definition", "fork": repositoryResponse(fork)})
			return
		}
		context := workspaces.SourceContext{Type: "contribution_opportunity", ID: o.ID, UpstreamRepositoryID: string(upstream.ID), AcceptanceCriteria: o.AcceptanceCriteria, SampleData: o.SampleData, Evidence: []string{o.Source.Kind + ":" + o.Source.ResourceID}}
		if pathway, e := pathways.Get(string(upstream.ID)); e == nil && len(pathway.Versions) > 0 {
			v := pathway.Versions[len(pathway.Versions)-1]
			context.GuidanceVersion = v.Number
			context.Guidance = append(append([]string{}, v.Prerequisites...), v.SupportedSetup...)
		}
		policy, _ := workspaceStore.EffectivePolicy(string(fork.ID), fork.OrganizationID)
		workspace, err := workspaceStore.CreateWithPolicy(string(fork.ID), o.Revision, actor.UserID, context, workspaces.Access{RepositoryID: string(fork.ID), ActorID: actor.UserID, Permission: "fork:write"}, definition, digest, policy)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		runner.Start(workspace)
		responseHours := in.ResponseExpectationHours
		if responseHours == 0 {
			responseHours = 24
		}
		collaboration, err := store.StartCollaboration(string(upstream.ID), o.ID, claimID, string(fork.ID), workspace.ID, actor.UserID, o.Revision, responseHours)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 201, map[string]any{"fork": repositoryResponse(fork), "workspace": workspace, "collaboration": collaboration, "upstream_write_access": false, "agent_explanations": map[string]any{"available": true, "context_type": "workspace", "revision": o.Revision}})
	})
	mux.HandleFunc("POST /repositories/{repository}/contribution-opportunities/{opportunity}/reports", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			WorkspaceID           string `json:"workspace_id"`
			WorkspaceRepositoryID string `json:"workspace_repository_id"`
			Kind                  string `json:"kind"`
			Detail                string `json:"detail"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		fork, err := repos.Inspect(storage.ID(in.WorkspaceRepositoryID))
		if err != nil || fork.OwnerID != actor || string(fork.UpstreamID) != r.PathValue("repository") {
			writeJSON(w, 422, map[string]string{"error": "invalid_opportunity_workspace"})
			return
		}
		workspace, err := workspaceStore.Get(string(fork.ID), in.WorkspaceID)
		if err != nil || workspace.CreatorID != actor || workspace.Context.Type != "contribution_opportunity" || workspace.Context.ID != r.PathValue("opportunity") {
			writeJSON(w, 422, map[string]string{"error": "invalid_opportunity_workspace"})
			return
		}
		report, err := store.Report(r.PathValue("repository"), r.PathValue("opportunity"), actor, in.WorkspaceID, in.Kind, in.Detail)
		if opportunityError(w, err) {
			return
		}
		writeJSON(w, 201, report)
	})
	role := func(repo repositories.Repository, o contributionopportunities.Opportunity, c contributionopportunities.Collaboration, actor string) string {
		if actor == c.ContributorID {
			return "contributor"
		}
		if actor == repo.OwnerID {
			return "maintainer"
		}
		for _, id := range o.MentorIDs {
			if id == actor {
				return "mentor"
			}
		}
		if organizationStore != nil && repo.OrganizationID != "" {
			if org, err := organizationStore.Get(repo.OrganizationID); err == nil {
				for _, a := range org.Agents {
					if a.ID == actor {
						return "agent"
					}
				}
			}
		}
		return ""
	}
	context := func(w http.ResponseWriter, r *http.Request) (repositories.Repository, contributionopportunities.Opportunity, contributionopportunities.Collaboration, string, string, bool) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return repo, contributionopportunities.Opportunity{}, contributionopportunities.Collaboration{}, "", "", false
		}
		o, err := store.Get(string(repo.ID), r.PathValue("opportunity"))
		if opportunityError(w, err) {
			return repo, o, contributionopportunities.Collaboration{}, "", "", false
		}
		c, err := store.Collaboration(string(repo.ID), o.ID)
		if opportunityError(w, err) {
			return repo, o, c, "", "", false
		}
		actorRole := role(repo, o, c, actor.UserID)
		if actorRole == "" {
			writeJSON(w, 404, map[string]string{"error": "opportunity_not_found"})
			return repo, o, c, "", "", false
		}
		return repo, o, c, actor.UserID, actorRole, true
	}
	base := "/repositories/{repository}/contribution-opportunities/{opportunity}/collaboration"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		_, _, c, _, _, ok := context(w, r)
		if ok {
			writeJSON(w, 200, c)
		}
	})
	mux.HandleFunc("POST "+base+"/presence", func(w http.ResponseWriter, r *http.Request) {
		repo, _, _, actor, actorRole, ok := context(w, r)
		if !ok {
			return
		}
		var in struct {
			Surface string `json:"surface"`
		}
		if !readJSON(w, r, &in, 4<<10) {
			return
		}
		c, err := store.ObserveHelp(string(repo.ID), r.PathValue("opportunity"), actor, actorRole, in.Surface)
		if opportunityError(w, err) {
			return
		}
		writeJSON(w, 200, c)
	})
	mux.HandleFunc("POST "+base+"/events", func(w http.ResponseWriter, r *http.Request) {
		repo, _, c, actor, actorRole, ok := context(w, r)
		if !ok {
			return
		}
		var in struct {
			Kind            string   `json:"kind"`
			Message         string   `json:"message"`
			DecisionOwnerID string   `json:"decision_owner_id"`
			TargetID        string   `json:"target_id"`
			Paths           []string `json:"paths"`
			Resolved        bool     `json:"resolved"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		allowed := map[string]map[string]bool{"contributor": {"question": true, "checkpoint_request": true, "handoff": true, "resolved": true}, "mentor": {"advice": true, "answer": true, "handoff": true, "intervention": true}, "maintainer": {"advice": true, "answer": true, "handoff": true, "intervention": true, "scope_changed": true}}
		if !allowed[actorRole][in.Kind] {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		owner := strings.TrimSpace(in.DecisionOwnerID)
		if owner == "" {
			owner = c.ContributorID
		}
		event := contributionopportunities.HelpEvent{Kind: in.Kind, ActorID: actor, Role: actorRole, Message: in.Message, DecisionOwnerID: owner, TargetID: in.TargetID, Paths: in.Paths, Resolved: in.Resolved}
		updated, err := store.AddHelpEvent(string(repo.ID), r.PathValue("opportunity"), event)
		if opportunityError(w, err) {
			return
		}
		writeJSON(w, 201, updated)
	})
	mux.HandleFunc("PUT "+base+"/mentor-availability", func(w http.ResponseWriter, r *http.Request) {
		repo, _, _, actor, actorRole, ok := context(w, r)
		if !ok {
			return
		}
		if actorRole != "mentor" && actorRole != "maintainer" {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		var in struct {
			State          string     `json:"state"`
			Note           string     `json:"note"`
			AvailableUntil *time.Time `json:"available_until"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		updated, err := store.SetMentorAvailability(string(repo.ID), r.PathValue("opportunity"), actor, in.State, in.Note, in.AvailableUntil)
		if opportunityError(w, err) {
			return
		}
		writeJSON(w, 200, updated)
	})
	mux.HandleFunc("POST "+base+"/agent-controls", func(w http.ResponseWriter, r *http.Request) {
		repo, _, c, actor, actorRole, ok := context(w, r)
		if !ok {
			return
		}
		if actorRole != "contributor" && actorRole != "maintainer" {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		var in struct {
			AgentID string   `json:"agent_id"`
			Mode    string   `json:"mode"`
			Paths   []string `json:"paths"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		if role(repo, contributionopportunities.Opportunity{}, c, in.AgentID) != "agent" {
			writeJSON(w, 422, map[string]string{"error": "agent_not_approved"})
			return
		}
		updated, err := store.ControlAgent(string(repo.ID), r.PathValue("opportunity"), actor, in.AgentID, in.Mode, in.Paths, "", "grant", 0)
		if opportunityError(w, err) {
			return
		}
		writeJSON(w, 201, updated)
	})
	mux.HandleFunc("POST "+base+"/agent-controls/{control}/interventions", func(w http.ResponseWriter, r *http.Request) {
		repo, _, _, actor, actorRole, ok := context(w, r)
		if !ok {
			return
		}
		if actorRole != "contributor" && actorRole != "maintainer" {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		var in struct {
			Action  string `json:"action"`
			Version int64  `json:"version"`
		}
		if !readJSON(w, r, &in, 4<<10) {
			return
		}
		updated, err := store.ControlAgent(string(repo.ID), r.PathValue("opportunity"), actor, "", "", nil, r.PathValue("control"), in.Action, in.Version)
		if opportunityError(w, err) {
			return
		}
		writeJSON(w, 200, updated)
	})
	mux.HandleFunc("POST "+base+"/agent-actions", func(w http.ResponseWriter, r *http.Request) {
		repo, _, c, actor, actorRole, ok := context(w, r)
		if !ok {
			return
		}
		if actorRole != "agent" {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		var in struct {
			ControlID  string  `json:"control_id"`
			Kind       string  `json:"kind"`
			Summary    string  `json:"summary"`
			Path       string  `json:"path"`
			Content    string  `json:"content"`
			Deleted    bool    `json:"deleted"`
			BaseDigest *string `json:"base_digest"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		var control *contributionopportunities.AgentControl
		for i := range c.Controls {
			if c.Controls[i].ID == in.ControlID && c.Controls[i].AgentID == actor {
				control = &c.Controls[i]
			}
		}
		if control == nil || control.State != "active" {
			writeJSON(w, 409, map[string]string{"error": "agent_control_inactive"})
			return
		}
		eventKind := ""
		paths := []string{}
		switch in.Kind {
		case "explanation":
			if control.Mode != "explain" {
				break
			}
			eventKind = "agent_explanation"
		case "diagnosis":
			if control.Mode != "diagnose_setup" {
				break
			}
			eventKind = "agent_diagnosis"
		case "edit":
			if control.Mode != "edit" {
				break
			}
			allowed := false
			for _, p := range control.Paths {
				allowed = allowed || p == in.Path
			}
			if !allowed {
				break
			}
			workspace, err := workspaceStore.Get(c.WorkspaceRepositoryID, c.WorkspaceID)
			if err != nil {
				writeJSON(w, 409, map[string]string{"error": "workspace_unavailable"})
				return
			}
			if _, err = runner.WriteFile(workspace, actor, in.Path, []byte(in.Content), in.Deleted, in.BaseDigest); err != nil {
				writeWorkspaceResult(w, workspace, err)
				return
			}
			eventKind = "agent_edit"
			paths = []string{in.Path}
		}
		if eventKind == "" || strings.TrimSpace(in.Summary) == "" {
			writeJSON(w, 422, map[string]string{"error": "action_outside_control"})
			return
		}
		updated, err := store.AddHelpEvent(string(repo.ID), r.PathValue("opportunity"), contributionopportunities.HelpEvent{Kind: eventKind, ActorID: actor, Role: "agent", Message: in.Summary, DecisionOwnerID: c.ContributorID, TargetID: in.ControlID, Paths: paths})
		if opportunityError(w, err) {
			return
		}
		writeJSON(w, 201, updated)
	})
	mux.HandleFunc("POST "+base+"/transition", func(w http.ResponseWriter, r *http.Request) {
		repo, _, _, actor, actorRole, ok := context(w, r)
		if !ok {
			return
		}
		if actorRole == "agent" {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		var in struct {
			State  string `json:"state"`
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		updated, err := store.TransitionCollaboration(string(repo.ID), r.PathValue("opportunity"), actor, actorRole, in.State, in.Reason)
		if opportunityError(w, err) {
			return
		}
		writeJSON(w, 200, updated)
	})
}

func safeOpportunityPath(path string) bool {
	clean := strings.TrimSpace(path)
	return clean != "" && !strings.HasPrefix(clean, "/") && clean != ".." && !strings.HasPrefix(clean, "../") && !strings.Contains(clean, "\\")
}

func resolveOpportunitySource(repo, currentRevision string, s contributionopportunities.Source, is *issues.Store, ps *proposals.Store, os *organizations.Store) (string, string, string, bool, bool) {
	switch s.Kind {
	case "issue":
		i, e := is.Get(repo, s.ResourceID)
		if e != nil || i.Triage.Priority == "" || i.Status != "open" {
			return "", "", "", false, false
		}
		rev := i.AffectedCommitID
		if rev == "" {
			for _, x := range i.Relationships {
				if x.Kind == "code" && x.Revision != "" {
					rev = x.Revision
					break
				}
			}
		}
		return i.Title, rev, "triaged", rev != "" && len(i.Triage.AssigneeIDs) == 0, true
	case "proposal":
		p, e := ps.Get(repo, s.ResourceID)
		if e != nil {
			return "", "", "", false, false
		}
		return p.Title, currentRevision, string(p.State), p.State == "open" && currentRevision != "", true
	case "proposal_task":
		plan, e := ps.GetPlan(repo, s.ParentID)
		if e != nil {
			return "", "", "", false, false
		}
		for _, t := range plan.Tasks {
			if t.ID == s.ResourceID {
				return t.Title, t.BaseRevision, string(t.Status), t.Ready && t.Assignment == nil && t.BaseRevision != "", true
			}
		}
		return "", "", "", false, false
	case "stewardship":
		o, e := os.Get(s.OrganizationID)
		if e != nil {
			return "", "", "", false, false
		}
		for _, x := range o.StewardshipOpportunities {
			if x.ID == s.ResourceID && x.RepositoryID == repo {
				rev := ""
				if len(x.AffectedRevisions) > 0 {
					rev = x.AffectedRevisions[0]
				}
				return x.Title, rev, x.State, (x.State == "open" || x.State == "accepted") && !x.EvidenceStale && rev != "", true
			}
		}
	}
	return "", "", "", false, false
}
func opportunityError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, contributionopportunities.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_opportunity"})
	case errors.Is(e, contributionopportunities.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "opportunity_claimed_or_changed"})
	case errors.Is(e, contributionopportunities.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "opportunity_not_found"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
