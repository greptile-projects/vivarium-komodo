package main

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reasoning"
	"github.com/greptile-projects/vivarium-komodo/apps/api/relationships"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityreports"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

type organizationRepositories interface {
	Create(string, repositories.Metadata) (repositories.Repository, error)
	Inspect(storage.ID) (repositories.Repository, error)
	TransferOwner(storage.ID, string, string, string, string, string) (repositories.Repository, error)
	ListOrganization(string) ([]repositories.Repository, error)
	AddCollaborator(string, storage.ID, string) (repositories.Repository, error)
	RemoveCollaborator(string, storage.ID, string) error
}
type organizationUsers interface {
	FindByHandle(string) (users.User, error)
}
type organizationPackages interface {
	List(string) ([]packagecatalog.Version, error)
}
type organizationReleases interface {
	List(string) ([]releases.Release, error)
}
type organizationPulls interface {
	List(string) ([]pullrequests.PullRequest, error)
}
type organizationIncidents interface {
	List(string) ([]incidents.Incident, error)
	Get(string, string) (incidents.Incident, error)
}
type organizationCredentialStore interface {
	authStore
	IssueRepositoryGit(string, string, string, string, time.Duration) (auth.IssuedGrant, error)
	RevokeIDs([]string) error
}
type organizationProposals interface {
	Get(string, string) (proposals.Proposal, error)
	Create(string, string, string, string) (proposals.Proposal, error)
	CreateTask(string, string, string, proposals.TaskInput) (proposals.Task, error)
}
type organizationEvolutions interface {
	Evolution(string) (relationships.EvolutionPlan, error)
}
type organizationSecurityReports interface {
	Get(string, string, func(string) bool) (securityreports.Report, error)
	ListVisible(string, func(string) bool) ([]securityreports.Report, error)
}
type organizationActivities interface {
	Record(activities.Input) (activities.Event, error)
}

func registerOrganizationsHTTP(mux *http.ServeMux, orgs *organizations.Store, repos organizationRepositories, people organizationUsers, packages organizationPackages, releaseStore organizationReleases, pulls organizationPulls, incidentStore organizationIncidents, proposalStore organizationProposals, evolutionStore organizationEvolutions, securityStore organizationSecurityReports, credentials organizationCredentialStore, activity organizationActivities) {
	mux.HandleFunc("GET /organizations/{organization}/stewardship-opportunities", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		if !orgs.IsMember(id, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		o, err := orgs.Get(id)
		if organizationError(w, err) {
			return
		}
		items := append([]organizations.StewardshipOpportunity(nil), o.StewardshipOpportunities...)
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Rank != items[j].Rank {
				if items[i].Rank == 0 {
					return false
				}
				if items[j].Rank == 0 {
					return true
				}
				return items[i].Rank < items[j].Rank
			}
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		})
		writeJSON(w, 200, map[string]any{"items": items, "work_policies": o.StewardshipWorkPolicies})
	})
	mux.HandleFunc("POST /organizations/{organization}/stewardship-opportunities/evaluations", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in organizations.StewardshipOpportunity
		if !readJSON(w, r, &in, 131072) {
			return
		}
		if !organizationRepository(w, r.PathValue("organization"), in.RepositoryID, repos) {
			return
		}
		_, made, err := orgs.EvaluateStewardship(r.PathValue("organization"), actor.UserID, in)
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/stewardship-opportunities/{opportunity}/comments", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 8192) {
			return
		}
		_, made, err := orgs.DiscussStewardship(r.PathValue("organization"), r.PathValue("opportunity"), actor.UserID, in.Body)
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/stewardship-opportunities/{opportunity}/decisions", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in organizations.StewardshipDecision
		if !readJSON(w, r, &in, 8192) {
			return
		}
		_, made, err := orgs.DecideStewardship(r.PathValue("organization"), r.PathValue("opportunity"), actor.UserID, in)
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 200, made)
	})
	mux.HandleFunc("PUT /organizations/{organization}/stewardship-mandates/{mandate}/versions/{version}/work-policy", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		version, err := parsePositiveInt64(r.PathValue("version"))
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_version"})
			return
		}
		var in struct {
			ExpectedVersion int64                                `json:"expected_version"`
			Rules           []organizations.StewardshipClassRule `json:"rules"`
		}
		if !readJSON(w, r, &in, 32768) {
			return
		}
		_, made, err := orgs.PutStewardshipWorkPolicy(r.PathValue("organization"), actor.UserID, r.PathValue("mandate"), version, in.ExpectedVersion, in.Rules)
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 200, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/stewardship-opportunities/{opportunity}/work-decisions", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			Mode                    string `json:"mode"`
			Risk                    string `json:"risk"`
			Hours                   int    `json:"hours"`
			ExpectedDecisionVersion int64  `json:"expected_decision_version"`
			ExpectedPolicyVersion   int64  `json:"expected_policy_version"`
		}
		if !readJSON(w, r, &in, 8192) {
			return
		}
		auto := in.Mode == "auto_start"
		if !auto && in.Mode != "approval" {
			writeJSON(w, 422, map[string]string{"error": "invalid_mode"})
			return
		}
		o, err := orgs.Get(r.PathValue("organization"))
		if organizationError(w, err) {
			return
		}
		var opportunity *organizations.StewardshipOpportunity
		for i := range o.StewardshipOpportunities {
			if o.StewardshipOpportunities[i].ID == r.PathValue("opportunity") {
				opportunity = &o.StewardshipOpportunities[i]
			}
		}
		if opportunity == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		blockers := stewardshipSafetyBlockers(actor.UserID, *opportunity, incidentStore, securityStore)
		_, made, err := orgs.DecideStewardshipWork(o.ID, opportunity.ID, actor.UserID, in.Risk, in.Hours, in.ExpectedDecisionVersion, in.ExpectedPolicyVersion, auto, blockers)
		if organizationError(w, err) {
			return
		}
		recordStewardshipActivity(activity, made, actor.UserID, "stewardship.work_"+made.WorkDecisions[len(made.WorkDecisions)-1].State)
		writeJSON(w, 200, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/stewardship-opportunities/{opportunity}/promotion", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		type taskInput struct {
			Title              string   `json:"title"`
			Outcome            string   `json:"outcome"`
			OwnerKind          string   `json:"owner_kind"`
			OwnerID            string   `json:"owner_id"`
			Risk               string   `json:"risk"`
			CompletionCriteria []string `json:"completion_criteria"`
			VerificationPlan   []string `json:"verification_plan"`
			DependsOn          []int    `json:"depends_on"`
		}
		var in struct {
			Title        string      `json:"title"`
			Body         string      `json:"body"`
			BaseRevision string      `json:"base_revision"`
			Tasks        []taskInput `json:"tasks"`
		}
		if !readJSON(w, r, &in, 131072) {
			return
		}
		o, err := orgs.Get(r.PathValue("organization"))
		if organizationError(w, err) {
			return
		}
		var opportunity *organizations.StewardshipOpportunity
		for i := range o.StewardshipOpportunities {
			if o.StewardshipOpportunities[i].ID == r.PathValue("opportunity") {
				opportunity = &o.StewardshipOpportunities[i]
			}
		}
		if opportunity == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		baseCurrent := false
		for _, revision := range opportunity.AffectedRevisions {
			baseCurrent = baseCurrent || revision == strings.TrimSpace(in.BaseRevision)
		}
		if opportunity.State != "accepted" || !baseCurrent || len(in.Tasks) == 0 || len(stewardshipSafetyBlockers(actor.UserID, *opportunity, incidentStore, securityStore)) > 0 {
			writeJSON(w, 409, map[string]string{"error": "promotion_blocked_or_base_changed"})
			return
		}
		proposal, err := proposalStore.Create(opportunity.RepositoryID, actor.UserID, in.Title, in.Body)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_promotion"})
			return
		}
		tasks := []proposals.Task{}
		evidence := make([]reasoning.Evidence, 0, len(opportunity.Citations))
		for _, citation := range opportunity.Citations {
			evidence = append(evidence, reasoning.Evidence{RepositoryID: citation.RepositoryID, CommitID: citation.Revision, Kind: citation.Kind, ResourceID: citation.ResourceID, Label: citation.Summary})
		}
		acknowledgements := make([]reasoning.Acknowledgement, 0, len(opportunity.AffectedOwnerIDs))
		for _, ownerID := range opportunity.AffectedOwnerIDs {
			acknowledgements = append(acknowledgements, reasoning.Acknowledgement{OwnerID: ownerID, State: "pending"})
		}
		for index, input := range in.Tasks {
			deps := []string{}
			valid := true
			for _, position := range input.DependsOn {
				if position < 1 || position > len(tasks) {
					valid = false
					break
				}
				deps = append(deps, tasks[position-1].ID)
			}
			if !valid {
				writeJSON(w, 422, map[string]string{"error": "invalid_task_dependency"})
				return
			}
			context := &reasoning.Context{Kind: "stewardship_opportunity", OrganizationID: o.ID, OpportunityID: opportunity.ID, MandateID: opportunity.MandateID, MandateVersion: opportunity.MandateVersion, RepositoryID: opportunity.RepositoryID, CommitID: in.BaseRevision, Claim: opportunity.Summary, Risk: input.Risk, State: "accepted", Rationale: opportunity.InScopeReason, Verification: input.VerificationPlan, Evidence: evidence, Acknowledgements: acknowledgements}
			made, createErr := proposalStore.CreateTask(opportunity.RepositoryID, proposal.ID, actor.UserID, proposals.TaskInput{Title: input.Title, Outcome: input.Outcome, Position: index + 1, Status: proposals.TaskPlanned, DependsOn: deps, OwnerKind: input.OwnerKind, OwnerID: input.OwnerID, CompletionCriteria: input.CompletionCriteria, Risk: input.Risk, VerificationPlan: input.VerificationPlan, BaseRevision: in.BaseRevision, ReasoningContext: context})
			if createErr != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_task"})
				return
			}
			tasks = append(tasks, made)
		}
		ids := make([]string, len(tasks))
		for i := range tasks {
			ids[i] = tasks[i].ID
		}
		_, made, err := orgs.LinkStewardshipWork(o.ID, opportunity.ID, actor.UserID, proposal.ID, in.BaseRevision, ids)
		if organizationError(w, err) {
			return
		}
		recordStewardshipActivity(activity, made, actor.UserID, "stewardship.opportunity_promoted")
		writeJSON(w, 201, map[string]any{"opportunity": made, "proposal": proposal, "tasks": tasks})
	})
	mux.HandleFunc("GET /organizations/{organization}/stewardship-mandates", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		if !orgs.IsMember(id, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		o, err := orgs.Get(id)
		if organizationError(w, err) {
			return
		}
		items := append([]organizations.StewardshipMandate(nil), o.StewardshipMandates...)
		for i := range items {
			items[i].State = organizations.StewardshipState(items[i], time.Now().UTC())
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /organizations/{organization}/stewardship-mandates", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in organizations.StewardshipMandate
		if !readJSON(w, r, &in, 65536) {
			return
		}
		for _, scope := range in.Scopes {
			if !organizationRepository(w, r.PathValue("organization"), scope.RepositoryID, repos) {
				return
			}
		}
		_, made, err := orgs.DraftStewardship(r.PathValue("organization"), actor.UserID, "", in)
		if organizationError(w, err) {
			return
		}
		w.Header().Set("Location", "/organizations/"+r.PathValue("organization")+"/stewardship-mandates/"+made.ID+"/versions/1")
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/stewardship-mandates/{mandate}/versions", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in organizations.StewardshipMandate
		if !readJSON(w, r, &in, 65536) {
			return
		}
		for _, scope := range in.Scopes {
			if !organizationRepository(w, r.PathValue("organization"), scope.RepositoryID, repos) {
				return
			}
		}
		_, made, err := orgs.DraftStewardship(r.PathValue("organization"), actor.UserID, r.PathValue("mandate"), in)
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/stewardship-mandates/{mandate}/versions/{version}/accept", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		version, err := parsePositiveInt64(r.PathValue("version"))
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_organization"})
			return
		}
		_, made, err := orgs.AcceptStewardship(r.PathValue("organization"), r.PathValue("mandate"), version, actor.UserID)
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 200, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/stewardship-mandates/{mandate}/versions/{version}/{action}", func(w http.ResponseWriter, r *http.Request) {
		action := r.PathValue("action")
		if action != "pause" && action != "resume" && action != "revoke" {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		version, err := parsePositiveInt64(r.PathValue("version"))
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_organization"})
			return
		}
		_, made, err := orgs.TransitionStewardship(r.PathValue("organization"), r.PathValue("mandate"), version, actor.UserID, action)
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 200, made)
	})
	mux.HandleFunc("GET /organizations/{organization}/stewardship-mandates/{mandate}/versions/{version}/preview", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		if !orgs.IsMember(id, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		version, err := parsePositiveInt64(r.PathValue("version"))
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_organization"})
			return
		}
		o, err := orgs.Get(id)
		if organizationError(w, err) {
			return
		}
		var mandate *organizations.StewardshipMandate
		for i := range o.StewardshipMandates {
			if o.StewardshipMandates[i].ID == r.PathValue("mandate") && o.StewardshipMandates[i].Version == version {
				mandate = &o.StewardshipMandates[i]
			}
		}
		if mandate == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		scopes := map[string]any{}
		now := time.Now().UTC()
		for _, scope := range mandate.Scopes {
			rules, e := orgs.EffectivePolicy(id, scope.RepositoryID, nil)
			if organizationError(w, e) {
				return
			}
			grants := []organizations.RoleGrant{}
			for _, grant := range o.RoleGrants {
				if grant.PrincipalKind != "agent" || grant.PrincipalID != mandate.AgentID || grant.RevokedAt != nil || !grant.ExpiresAt.After(now) {
					continue
				}
				for _, resource := range grant.Resources {
					if resource.Kind == "repository" && resource.ID == scope.RepositoryID {
						grants = append(grants, grant)
						break
					}
				}
			}
			scopes[scope.RepositoryID] = map[string]any{"branches": scope.Branches, "effective_policy": rules, "existing_agent_grants": grants}
		}
		writeJSON(w, 200, map[string]any{"mandate_id": mandate.ID, "version": mandate.Version, "state": organizations.StewardshipState(*mandate, now), "scopes": scopes, "allowed_actions": mandate.AllowedActions, "authority_created_by_mandate": false, "mandate_write_authority": false, "mandate_merge_authority": false, "note": "The mandate records responsibility only. Existing grants remain independently bounded and revocable."})
	})
	mux.HandleFunc("POST /organizations/{organization}/policies", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in organizations.PolicyVersion
		if !readJSON(w, r, &in, 65536) {
			return
		}
		if !organizationPolicyTargets(w, r.PathValue("organization"), in.Targets, repos) {
			return
		}
		_, made, err := orgs.DraftPolicy(r.PathValue("organization"), actor.UserID, "", in)
		if organizationError(w, err) {
			return
		}
		w.Header().Set("Location", "/organizations/"+r.PathValue("organization")+"/policies/"+made.ID+"/versions/1")
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/policies/{policy}/versions", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in organizations.PolicyVersion
		if !readJSON(w, r, &in, 65536) {
			return
		}
		if !organizationPolicyTargets(w, r.PathValue("organization"), in.Targets, repos) {
			return
		}
		_, made, err := orgs.DraftPolicy(r.PathValue("organization"), actor.UserID, r.PathValue("policy"), in)
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("GET /organizations/{organization}/policies", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		if !orgs.IsMember(r.PathValue("organization"), actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		o, err := orgs.Get(r.PathValue("organization"))
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": o.Policies, "exceptions": o.PolicyExceptions})
	})
	mux.HandleFunc("POST /organizations/{organization}/policies/{policy}/versions/{version}/activate", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		version, err := parsePositiveInt64(r.PathValue("version"))
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_organization"})
			return
		}
		_, active, err := orgs.ActivatePolicy(r.PathValue("organization"), actor.UserID, r.PathValue("policy"), version)
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 200, active)
	})
	mux.HandleFunc("POST /organizations/{organization}/policies/{policy}/versions/{version}/preview", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		if !orgs.IsMember(id, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		version, err := parsePositiveInt64(r.PathValue("version"))
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_organization"})
			return
		}
		var in struct {
			RepositoryIDs []string `json:"repository_ids"`
		}
		if !readJSON(w, r, &in, 16384) {
			return
		}
		o, err := orgs.Get(id)
		if organizationError(w, err) {
			return
		}
		var draft *organizations.PolicyVersion
		for i := range o.Policies {
			if o.Policies[i].ID == r.PathValue("policy") && o.Policies[i].Version == version {
				copy := o.Policies[i]
				draft = &copy
			}
		}
		if draft == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		impacts := map[string][]organizations.EffectiveRule{}
		for _, repoID := range in.RepositoryIDs {
			if !organizationRepository(w, id, repoID, repos) {
				return
			}
			rules, e := orgs.EffectivePolicy(id, repoID, draft)
			if organizationError(w, e) {
				return
			}
			impacts[repoID] = rules
		}
		writeJSON(w, 200, map[string]any{"policy_id": draft.ID, "version": draft.Version, "state": draft.State, "impacts": impacts, "activation_changes_new_decisions_only": true})
	})
	mux.HandleFunc("GET /organizations/{organization}/repositories/{repository}/effective-policy", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		id, repoID := r.PathValue("organization"), r.PathValue("repository")
		if !orgs.IsMember(id, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if !organizationRepository(w, id, repoID, repos) {
			return
		}
		rules, err := orgs.EffectivePolicy(id, repoID, nil)
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 200, map[string]any{"repository_id": repoID, "rules": rules})
	})
	mux.HandleFunc("POST /organizations/{organization}/policy-exceptions", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		var in organizations.PolicyException
		if !readJSON(w, r, &in, 8192) {
			return
		}
		if !organizationRepository(w, id, in.RepositoryID, repos) {
			return
		}
		if !organizationPolicyMaintainer(orgs, id, actor.UserID, in.RepositoryID) {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		_, made, err := orgs.RequestPolicyException(id, actor.UserID, in)
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/policy-exceptions/{exception}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			Decision string `json:"decision"`
		}
		if !readJSON(w, r, &in, 1024) {
			return
		}
		_, resolved, err := orgs.ResolvePolicyException(r.PathValue("organization"), r.PathValue("exception"), actor.UserID, in.Decision)
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 200, resolved)
	})
	mux.HandleFunc("GET /organizations/{organization}/directory", func(w http.ResponseWriter, r *http.Request) {
		actor, authenticated, ok := authenticateOptionalRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		o, e := orgs.Get(r.PathValue("organization"))
		if organizationError(w, e) {
			return
		}
		writeJSON(w, 200, organizations.DirectoryFor(o, authenticated && orgs.IsMember(o.ID, actor.UserID)))
	})
	mux.HandleFunc("POST /organizations/{organization}/teams", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in organizations.Team
		if !readJSON(w, r, &in, 8192) {
			return
		}
		_, made, e := orgs.CreateTeam(r.PathValue("organization"), actor.UserID, in)
		if organizationError(w, e) {
			return
		}
		w.Header().Set("Location", "/organizations/"+r.PathValue("organization")+"/teams/"+made.ID)
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/teams/{team}/members", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			UserID          string `json:"user_id"`
			Role            string `json:"role"`
			ExpectedVersion int64  `json:"expected_version"`
		}
		if !readJSON(w, r, &in, 2048) {
			return
		}
		o, e := orgs.InviteTeamMember(r.PathValue("organization"), r.PathValue("team"), actor.UserID, in.UserID, in.Role, in.ExpectedVersion)
		if organizationError(w, e) {
			return
		}
		writeJSON(w, 201, o)
	})
	mux.HandleFunc("POST /organizations/{organization}/teams/{team}/members/accept", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
		}
		if !readJSON(w, r, &in, 1024) {
			return
		}
		o, e := orgs.AcceptTeam(r.PathValue("organization"), r.PathValue("team"), actor.UserID, in.ExpectedVersion)
		if organizationError(w, e) {
			return
		}
		writeJSON(w, 200, o)
	})
	mux.HandleFunc("DELETE /organizations/{organization}/teams/{team}/members/{user}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
		}
		if !readJSON(w, r, &in, 1024) {
			return
		}
		before, _ := orgs.EffectiveAccess(r.PathValue("organization"), r.PathValue("user"))
		o, e := orgs.RemoveTeamMember(r.PathValue("organization"), r.PathValue("team"), actor.UserID, r.PathValue("user"), in.ExpectedVersion)
		if organizationError(w, e) {
			return
		}
		after, _ := orgs.EffectiveAccess(r.PathValue("organization"), r.PathValue("user"))
		if e = revokeLostCredentials(credentials, r.PathValue("user"), before, after); e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, o)
	})
	mux.HandleFunc("POST /organizations/{organization}/teams/{team}/responsibilities", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			RepositoryID    string `json:"repository_id"`
			Area            string `json:"area"`
			Description     string `json:"description"`
			Visibility      string `json:"visibility"`
			ExpectedVersion int64  `json:"expected_version"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		repo, e := repos.Inspect(storage.ID(in.RepositoryID))
		if e != nil || repo.OrganizationID != r.PathValue("organization") {
			writeJSON(w, 422, map[string]string{"error": "invalid_organization"})
			return
		}
		_, made, e := orgs.AddResponsibility(r.PathValue("organization"), r.PathValue("team"), actor.UserID, organizations.Responsibility{RepositoryID: in.RepositoryID, Area: in.Area, Description: in.Description, Visibility: in.Visibility}, in.ExpectedVersion)
		if organizationError(w, e) {
			return
		}
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/agents", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in organizations.Agent
		if !readJSON(w, r, &in, 8192) {
			return
		}
		_, made, e := orgs.RegisterAgent(r.PathValue("organization"), actor.UserID, in)
		if organizationError(w, e) {
			return
		}
		w.Header().Set("Location", "/organizations/"+r.PathValue("organization")+"/agents/"+made.ID)
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("GET /organizations/{organization}/access/effective", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		if !orgs.IsMember(r.PathValue("organization"), actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		items, e := orgs.EffectiveAccess(r.PathValue("organization"), actor.UserID)
		if organizationError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"user_id": actor.UserID, "items": items})
	})
	mux.HandleFunc("POST /organizations/{organization}/access-grants", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in organizations.RoleGrant
		if !readJSON(w, r, &in, 16384) {
			return
		}
		if !organizationResources(w, r.PathValue("organization"), in.Resources, repos) {
			return
		}
		_, made, e := orgs.GrantRole(r.PathValue("organization"), actor.UserID, in)
		if organizationError(w, e) {
			return
		}
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/access-requests", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in organizations.AccessRequest
		if !readJSON(w, r, &in, 16384) {
			return
		}
		if !organizationResources(w, r.PathValue("organization"), in.Resources, repos) {
			return
		}
		_, made, e := orgs.RequestAccess(r.PathValue("organization"), actor.UserID, in)
		if organizationError(w, e) {
			return
		}
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/access-requests/{request}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			Decision string `json:"decision"`
		}
		if !readJSON(w, r, &in, 1024) {
			return
		}
		_, request, grant, e := orgs.ResolveAccessRequest(r.PathValue("organization"), r.PathValue("request"), actor.UserID, in.Decision)
		if organizationError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"request": request, "grant": grant})
	})
	mux.HandleFunc("DELETE /organizations/{organization}/access-grants/{grant}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		_, revoked, e := orgs.RevokeRole(r.PathValue("organization"), r.PathValue("grant"), actor.UserID)
		if organizationError(w, e) {
			return
		}
		if e = credentials.RevokeIDs(revoked.CredentialIDs); e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, revoked)
	})
	mux.HandleFunc("POST /organizations/{organization}/access-grants/{grant}/credentials", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			RepositoryID   string `json:"repository_id"`
			Branch         string `json:"branch"`
			ExpiresInHours int    `json:"expires_in_hours"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		grants, e := orgs.EffectiveAccess(r.PathValue("organization"), actor.UserID)
		if organizationError(w, e) {
			return
		}
		var selected *organizations.RoleGrant
		for i := range grants {
			if grants[i].ID == r.PathValue("grant") {
				selected = &grants[i]
				break
			}
		}
		allowed := selected != nil && selected.Role != "viewer" && in.ExpiresInHours > 0 && in.ExpiresInHours <= 24 && time.Now().Add(time.Duration(in.ExpiresInHours)*time.Hour).Before(selected.ExpiresAt.Add(time.Second)) && strings.HasPrefix(in.Branch, "refs/heads/") && !contains(selected.Exceptions, "candidate_branch:write")
		if allowed {
			allowed = false
			for _, resource := range selected.Resources {
				if resource.Kind == "repository" && resource.ID == in.RepositoryID {
					allowed = true
				}
			}
		}
		if !allowed {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		issued, e := credentials.IssueRepositoryGit(actor.UserID, "Organization role "+selected.ID, in.RepositoryID, in.Branch, time.Duration(in.ExpiresInHours)*time.Hour)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_credential"})
			return
		}
		if _, _, e = orgs.AttachCredential(r.PathValue("organization"), selected.ID, actor.UserID, issued.ID); e != nil {
			_ = credentials.RevokeIDs([]string{issued.ID})
			organizationError(w, e)
			return
		}
		writeJSON(w, 201, map[string]any{"id": issued.ID, "token": issued.Token, "username": "agent", "repository_id": in.RepositoryID, "branch": in.Branch, "expires_at": issued.ExpiresAt})
	})
	mux.HandleFunc("POST /organizations", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			Slug        string `json:"slug"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		o, e := orgs.Create(actor.UserID, in.Slug, in.Name, in.Description)
		if organizationError(w, e) {
			return
		}
		w.Header().Set("Location", "/organizations/"+o.ID)
		writeJSON(w, 201, o)
	})
	mux.HandleFunc("GET /organizations", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		items, e := orgs.ListFor(actor.UserID)
		if organizationError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "page": 1, "per_page": len(items), "total_count": len(items)})
	})
	mux.HandleFunc("GET /organizations/{organization}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		if !orgs.IsMember(id, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		o, e := orgs.Get(id)
		if organizationError(w, e) {
			return
		}
		writeJSON(w, 200, o)
	})
	mux.HandleFunc("POST /organizations/{organization}/members", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			Handle string `json:"handle"`
		}
		if !readJSON(w, r, &in, 2048) {
			return
		}
		u, e := people.FindByHandle(in.Handle)
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "user_not_found"})
			return
		}
		o, e := orgs.Invite(r.PathValue("organization"), actor.UserID, string(u.ID))
		if organizationError(w, e) {
			return
		}
		writeJSON(w, 201, o)
	})
	mux.HandleFunc("POST /organizations/{organization}/members/accept", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		o, e := orgs.Accept(r.PathValue("organization"), actor.UserID)
		if organizationError(w, e) {
			return
		}
		for _, repo := range mustOrgRepos(repos, o.ID) {
			_, _ = repos.AddCollaborator(repo.OwnerID, repo.ID, actor.UserID)
		}
		writeJSON(w, 200, o)
	})
	mux.HandleFunc("DELETE /organizations/{organization}/members/{user}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		user := r.PathValue("user")
		before, _ := orgs.EffectiveAccess(id, user)
		o, e := orgs.Remove(id, actor.UserID, user)
		if organizationError(w, e) {
			return
		}
		for _, repo := range mustOrgRepos(repos, id) {
			_ = repos.RemoveCollaborator(repo.OwnerID, repo.ID, user)
		}
		if e = revokeLostCredentials(credentials, user, before, nil); e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, o)
	})
	mux.HandleFunc("POST /organizations/{organization}/repositories", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		if !orgs.IsOwner(id, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			Name        string                  `json:"name"`
			Description string                  `json:"description"`
			Visibility  repositories.Visibility `json:"visibility"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		if in.Visibility == "" {
			in.Visibility = repositories.Private
		}
		repo, e := repos.Create(actor.UserID, repositories.Metadata{Name: in.Name, Description: in.Description, Visibility: in.Visibility})
		if e != nil {
			writeRepositoryError(w, e)
			return
		}
		repo, e = repos.TransferOwner(repo.ID, "user", actor.UserID, "organization", id, actor.UserID)
		if e != nil {
			writeRepositoryError(w, e)
			return
		}
		o, _ := orgs.Get(id)
		for _, member := range o.Members {
			if !member.AcceptedAt.IsZero() && member.UserID != repo.OwnerID {
				_, _ = repos.AddCollaborator(repo.OwnerID, repo.ID, member.UserID)
			}
		}
		writeJSON(w, 201, repositoryResponse(repo))
	})
	mux.HandleFunc("POST /organizations/{organization}/repository-transfers", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		var in struct {
			RepositoryID string `json:"repository_id"`
			ToKind       string `json:"to_kind"`
			ToID         string `json:"to_id"`
		}
		if !readJSON(w, r, &in, 2048) {
			return
		}
		repo, e := repos.Inspect(storage.ID(in.RepositoryID))
		if e != nil {
			writeRepositoryError(w, e)
			return
		}
		fromKind, fromID := "user", repo.OwnerID
		if repo.OrganizationID != "" {
			fromKind, fromID = "organization", repo.OrganizationID
		}
		if in.ToKind == "" {
			in.ToKind = "organization"
		}
		if in.ToID == "" {
			in.ToID = id
		}
		_, t, e := orgs.RequestTransfer(id, actor.UserID, organizations.Transfer{RepositoryID: in.RepositoryID, FromKind: fromKind, FromID: fromID, ToKind: in.ToKind, ToID: in.ToID})
		if organizationError(w, e) {
			return
		}
		writeJSON(w, 201, t)
	})
	mux.HandleFunc("POST /organizations/{organization}/repository-transfers/{transfer}/accept", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		_, t, e := orgs.ResolveTransfer(id, r.PathValue("transfer"), actor.UserID, "accepted")
		if organizationError(w, e) {
			return
		}
		repo, e := repos.TransferOwner(storage.ID(t.RepositoryID), t.FromKind, t.FromID, t.ToKind, t.ToID, actor.UserID)
		if e != nil {
			writeRepositoryError(w, e)
			return
		}
		if t.ToKind == "organization" {
			o, _ := orgs.Get(t.ToID)
			for _, m := range o.Members {
				if !m.AcceptedAt.IsZero() && m.UserID != repo.OwnerID {
					_, _ = repos.AddCollaborator(repo.OwnerID, repo.ID, m.UserID)
				}
			}
		}
		writeJSON(w, 200, repositoryResponse(repo))
	})
	mux.HandleFunc("GET /organizations/{organization}/portfolio", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		if !orgs.IsMember(id, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		rs, e := repos.ListOrganization(id)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		pkg := []packagecatalog.Version{}
		rel := []releases.Release{}
		active := []pullrequests.PullRequest{}
		incs := []incidents.Incident{}
		for _, repo := range rs {
			p, _ := packages.List(string(repo.ID))
			pkg = append(pkg, p...)
			x, _ := releaseStore.List(string(repo.ID))
			rel = append(rel, x...)
			pr, _ := pulls.List(string(repo.ID))
			for _, v := range pr {
				if v.Status == pullrequests.Open {
					active = append(active, v)
				}
			}
			ii, _ := incidentStore.List(string(repo.ID))
			for _, v := range ii {
				if v.Status != "resolved" {
					incs = append(incs, v)
				}
			}
		}
		writeJSON(w, 200, map[string]any{"repositories": rs, "packages": pkg, "active_work": active, "releases": rel, "incidents": incs})
	})
	mux.HandleFunc("GET /organizations/{organization}/initiatives", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		if !orgs.IsMember(id, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		items, err := orgs.InitiativeView(id, func(repoID string) bool {
			repo, e := repos.Inspect(storage.ID(repoID))
			return e == nil && repo.OrganizationID == id
		})
		if organizationError(w, err) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /organizations/{organization}/initiatives", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		var in organizations.Initiative
		if !readJSON(w, r, &in, 65536) || !organizationResources(w, id, in.Sources, repos) || !organizationInitiativeSources(w, in.Sources, actor.UserID, orgs.IsOwner(id, actor.UserID), proposalStore, evolutionStore, incidentStore, securityStore) {
			return
		}
		_, made, err := orgs.CreateInitiative(id, actor.UserID, in)
		if organizationError(w, err) {
			return
		}
		w.Header().Set("Location", "/organizations/"+id+"/initiatives/"+made.ID)
		writeJSON(w, 201, made)
	})
	mux.HandleFunc("POST /organizations/{organization}/initiatives/{initiative}/items", func(w http.ResponseWriter, r *http.Request) {
		putInitiativeItem(w, r, orgs, repos, credentials, "")
	})
	mux.HandleFunc("PUT /organizations/{organization}/initiatives/{initiative}/items/{item}", func(w http.ResponseWriter, r *http.Request) {
		putInitiativeItem(w, r, orgs, repos, credentials, r.PathValue("item"))
	})
}
func organizationInitiativeSources(w http.ResponseWriter, sources []organizations.ResourceRef, actor string, maintainer bool, proposals organizationProposals, evolutions organizationEvolutions, incidents organizationIncidents, security organizationSecurityReports) bool {
	for _, source := range sources {
		valid := false
		switch source.Kind {
		case "proposal":
			_, err := proposals.Get(source.RepositoryID, source.ID)
			valid = err == nil
		case "evolution":
			plan, err := evolutions.Evolution(source.ID)
			valid = err == nil && plan.RepositoryID == source.RepositoryID
		case "incident":
			_, err := incidents.Get(source.RepositoryID, source.ID)
			valid = err == nil
		case "security":
			report, err := security.Get(source.ID, actor, func(repositoryID string) bool { return maintainer && repositoryID == source.RepositoryID })
			valid = err == nil
			if valid {
				valid = false
				for _, affected := range report.Affected {
					if affected.RepositoryID == source.RepositoryID {
						valid = true
					}
				}
			}
		}
		if !valid {
			writeJSON(w, 422, map[string]string{"error": "invalid_initiative_source"})
			return false
		}
	}
	return true
}
func putInitiativeItem(w http.ResponseWriter, r *http.Request, orgs *organizations.Store, repos organizationRepositories, credentials organizationCredentialStore, itemID string) {
	actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
	if !ok {
		return
	}
	id := r.PathValue("organization")
	var in organizations.InitiativeItem
	if !readJSON(w, r, &in, 65536) {
		return
	}
	refs := []organizations.ResourceRef{in.Source}
	refs = append(refs, in.Contributions...)
	if !organizationResources(w, id, refs, repos) || !organizationRepository(w, id, in.RepositoryID, repos) {
		return
	}
	_, saved, err := orgs.PutInitiativeItem(id, r.PathValue("initiative"), itemID, actor.UserID, in)
	if organizationError(w, err) {
		return
	}
	status := 200
	if itemID == "" {
		status = 201
	}
	writeJSON(w, status, saved)
}
func mustOrgRepos(r organizationRepositories, id string) []repositories.Repository {
	x, _ := r.ListOrganization(id)
	return x
}
func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
func revokeLostCredentials(credentials organizationCredentialStore, user string, before, after []organizations.RoleGrant) error {
	retained := map[string]bool{}
	for _, grant := range after {
		retained[grant.ID] = true
	}
	ids := []string{}
	for _, grant := range before {
		if !retained[grant.ID] {
			for _, id := range grant.CredentialIDs {
				if grant.CredentialUsers[id] == user {
					ids = append(ids, id)
				}
			}
		}
	}
	return credentials.RevokeIDs(ids)
}
func organizationResources(w http.ResponseWriter, organization string, resources []organizations.ResourceRef, repos organizationRepositories) bool {
	for _, resource := range resources {
		repositoryID := resource.RepositoryID
		if resource.Kind == "repository" {
			repositoryID = resource.ID
		}
		repo, err := repos.Inspect(storage.ID(repositoryID))
		if err != nil || repo.OrganizationID != organization {
			writeJSON(w, 422, map[string]string{"error": "invalid_resource"})
			return false
		}
	}
	return true
}
func parsePositiveInt64(value string) (int64, error) {
	n, e := strconv.ParseInt(value, 10, 64)
	if e != nil || n < 1 {
		return 0, errors.New("invalid number")
	}
	return n, nil
}
func organizationRepository(w http.ResponseWriter, organization, repository string, repos organizationRepositories) bool {
	repo, err := repos.Inspect(storage.ID(repository))
	if err != nil || repo.OrganizationID != organization {
		writeJSON(w, 422, map[string]string{"error": "invalid_resource"})
		return false
	}
	return true
}
func organizationPolicyTargets(w http.ResponseWriter, organization string, targets []organizations.PolicyTarget, repos organizationRepositories) bool {
	for _, target := range targets {
		if target.Kind == "repository" && !organizationRepository(w, organization, target.ID, repos) {
			return false
		}
	}
	return true
}
func organizationPolicyMaintainer(orgs *organizations.Store, organization, user, repository string) bool {
	if orgs.IsOwner(organization, user) {
		return true
	}
	grants, err := orgs.EffectiveAccess(organization, user)
	if err != nil {
		return false
	}
	for _, grant := range grants {
		if grant.Role != "maintainer" && grant.Role != "operator" {
			continue
		}
		for _, resource := range grant.Resources {
			if resource.Kind == "repository" && resource.ID == repository {
				return true
			}
		}
	}
	return false
}
func organizationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, organizations.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(e, organizations.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
	case errors.Is(e, organizations.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "conflict"})
	case errors.Is(e, organizations.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_organization"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}

func stewardshipSafetyBlockers(actor string, opportunity organizations.StewardshipOpportunity, incidentStore organizationIncidents, securityStore organizationSecurityReports) []string {
	blockers := []string{}
	if incidents, err := incidentStore.List(opportunity.RepositoryID); err == nil {
		for _, incident := range incidents {
			if incident.Status != "resolved" {
				blockers = append(blockers, "active_incident")
				break
			}
		}
	}
	if reports, err := securityStore.ListVisible(actor, func(repository string) bool { return repository == opportunity.RepositoryID }); err == nil {
		for _, report := range reports {
			if report.EmbargoState == "lifted" {
				continue
			}
			for _, affected := range report.Affected {
				if affected.RepositoryID == opportunity.RepositoryID {
					blockers = append(blockers, "embargoed_evidence")
					break
				}
			}
		}
	}
	return blockers
}

func recordStewardshipActivity(activity organizationActivities, opportunity organizations.StewardshipOpportunity, actor, eventType string) {
	metadata := map[string]string{"organization_opportunity_id": opportunity.ID, "state": opportunity.State}
	if opportunity.Work != nil {
		metadata["proposal_id"], metadata["base_revision"] = opportunity.Work.ProposalID, opportunity.Work.BaseRevision
	}
	if len(opportunity.WorkDecisions) > 0 {
		metadata["decision_version"] = strconv.FormatInt(opportunity.WorkDecisions[len(opportunity.WorkDecisions)-1].Version, 10)
	}
	for _, owner := range opportunity.AffectedOwnerIDs {
		_, _ = activity.Record(activities.Input{RepositoryID: opportunity.RepositoryID, ActorID: actor, Type: eventType, Resource: activities.Resource{Type: "stewardship_opportunity", ID: opportunity.ID}, TargetUserID: owner, Metadata: metadata})
	}
}
