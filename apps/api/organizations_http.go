package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
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
}
type organizationCredentialStore interface {
	authStore
	IssueRepositoryGit(string, string, string, string, time.Duration) (auth.IssuedGrant, error)
	RevokeIDs([]string) error
}

func registerOrganizationsHTTP(mux *http.ServeMux, orgs *organizations.Store, repos organizationRepositories, people organizationUsers, packages organizationPackages, releaseStore organizationReleases, pulls organizationPulls, incidentStore organizationIncidents, credentials organizationCredentialStore) {
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
