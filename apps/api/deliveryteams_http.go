package main

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deliveryteams"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
)

type deliveryTeamStore interface {
	Create(string, string, string, deliveryteams.Outcome, deliveryteams.CharterInput) (deliveryteams.Team, error)
	Get(string, string) (deliveryteams.Team, error)
	List(string) ([]deliveryteams.Team, error)
	Revise(string, string, string, int64, deliveryteams.CharterInput) (deliveryteams.Team, error)
	Invite(string, string, string, int64, deliveryteams.ParticipantInput) (deliveryteams.Team, error)
	Respond(string, string, string, string, string, int64) (deliveryteams.Team, error)
	Remove(string, string, string, string, string, int64) (deliveryteams.Team, error)
	Replace(string, string, string, string, int64, deliveryteams.ParticipantInput) (deliveryteams.Team, error)
	ProposePlan(string, string, string, string, int64, deliveryteams.PlanInput) (deliveryteams.Team, error)
	AcceptPlan(string, string, string, string, int64, int64) (deliveryteams.Team, error)
}
type deliveryOrganizationStore interface {
	Get(string) (organizations.Organization, error)
}

func registerDeliveryTeamsHTTP(mux *http.ServeMux, teams deliveryTeamStore, repos proposalRepositoryStore, credentials authStore, orgs deliveryOrganizationStore) {
	base := "/repositories/{repository}/delivery-teams"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := teams.List(string(repo.ID))
		writeDeliveryTeamResult(w, v, e, 200)
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			Name    string                     `json:"name"`
			Source  deliveryteams.Outcome      `json:"source"`
			Charter deliveryteams.CharterInput `json:"charter"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := teams.Create(string(repo.ID), in.Name, a.UserID, in.Source, in.Charter)
		if e == nil {
			w.Header().Set("Location", base+"/"+v.ID)
		}
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("GET "+base+"/{team}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := teams.Get(string(repo.ID), r.PathValue("team"))
		writeDeliveryTeamResult(w, v, e, 200)
	})
	mux.HandleFunc("PATCH "+base+"/{team}/charter", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			deliveryteams.CharterInput
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := teams.Revise(string(repo.ID), r.PathValue("team"), a.UserID, in.ExpectedVersion, in.CharterInput)
		writeDeliveryTeamResult(w, v, e, 200)
	})
	mux.HandleFunc("POST "+base+"/{team}/participants", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			deliveryteams.ParticipantInput
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		preview, e := deliveryAccess(repo, a.UserID, in.Kind, in.PrincipalID, in.RequestedActions, repos, orgs)
		if e != nil {
			writeDeliveryTeamResult(w, deliveryteams.Team{}, e, 422)
			return
		}
		in.Access = preview
		v, e := teams.Invite(string(repo.ID), r.PathValue("team"), a.UserID, in.ExpectedVersion, in.ParticipantInput)
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{team}/participants/{participant}/response", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			Response        string `json:"response"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		v, e := teams.Get(string(repo.ID), r.PathValue("team"))
		if e == nil {
			var p *deliveryteams.Participant
			for i := range v.Participants {
				if v.Participants[i].ID == r.PathValue("participant") {
					p = &v.Participants[i]
				}
			}
			if p == nil {
				e = deliveryteams.ErrNotFound
			} else if p.Kind == "human" && p.PrincipalID != a.UserID {
				e = deliveryteams.ErrForbidden
			} else if p.Kind == "agent" && !agentOperated(repo, *p, a.UserID, orgs) {
				e = deliveryteams.ErrForbidden
			}
		}
		if e == nil {
			v, e = teams.Respond(string(repo.ID), r.PathValue("team"), r.PathValue("participant"), a.UserID, in.Response, in.ExpectedVersion)
		}
		writeDeliveryTeamResult(w, v, e, 200)
	})
	mux.HandleFunc("DELETE "+base+"/{team}/participants/{participant}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			Reason          string `json:"reason"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		v, e := teams.Remove(string(repo.ID), r.PathValue("team"), r.PathValue("participant"), a.UserID, in.Reason, in.ExpectedVersion)
		writeDeliveryTeamResult(w, v, e, 200)
	})
	mux.HandleFunc("POST "+base+"/{team}/participants/{participant}/replacement", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			deliveryteams.ParticipantInput
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		preview, e := deliveryAccess(repo, a.UserID, in.Kind, in.PrincipalID, in.RequestedActions, repos, orgs)
		if e != nil {
			writeDeliveryTeamResult(w, deliveryteams.Team{}, e, 422)
			return
		}
		in.Access = preview
		v, e := teams.Replace(string(repo.ID), r.PathValue("team"), r.PathValue("participant"), a.UserID, in.ExpectedVersion, in.ParticipantInput)
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{team}/plan/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			deliveryteams.PlanInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		current, e := teams.Get(string(repo.ID), r.PathValue("team"))
		acting := ""
		if e == nil && a.UserID != current.OrganizerID {
			for _, p := range current.Participants {
				if p.State == "accepted" && ((p.Kind == "human" && p.PrincipalID == a.UserID) || (p.Kind == "agent" && agentOperated(repo, p, a.UserID, orgs))) {
					acting = p.ID
					break
				}
			}
			if acting == "" {
				e = deliveryteams.ErrForbidden
			}
		}
		var v deliveryteams.Team
		if e == nil {
			v, e = teams.ProposePlan(string(repo.ID), r.PathValue("team"), a.UserID, acting, in.ExpectedVersion, in.PlanInput)
		}
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{team}/plan/versions/{planVersion}/acceptances", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			ParticipantID   string `json:"participant_id"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		pv, e := strconv.ParseInt(r.PathValue("planVersion"), 10, 64)
		if e != nil {
			writeDeliveryTeamResult(w, deliveryteams.Team{}, deliveryteams.ErrInvalid, 422)
			return
		}
		current, e := teams.Get(string(repo.ID), r.PathValue("team"))
		var invited *deliveryteams.Participant
		if e == nil {
			for i := range current.Participants {
				if current.Participants[i].ID == in.ParticipantID {
					invited = &current.Participants[i]
					break
				}
			}
		}
		if e == nil && (invited == nil || (invited.Kind == "human" && invited.PrincipalID != a.UserID) || (invited.Kind == "agent" && !agentOperated(repo, *invited, a.UserID, orgs))) {
			e = deliveryteams.ErrForbidden
		}
		var v deliveryteams.Team
		if e == nil {
			v, e = teams.AcceptPlan(string(repo.ID), r.PathValue("team"), in.ParticipantID, a.UserID, in.ExpectedVersion, pv)
		}
		writeDeliveryTeamResult(w, v, e, 201)
	})
}

func deliveryRepositoryWriteAccess(w http.ResponseWriter, r *http.Request, repos proposalRepositoryStore, credentials authStore) (repositories.Repository, auth.Grant, bool) {
	repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
	if !ok {
		return repo, actor, false
	}
	if actor.UserID == repo.OwnerID {
		return repo, actor, true
	}
	member, err := repos.IsCollaborator(repo.ID, actor.UserID)
	if err != nil || !member {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return repositories.Repository{}, auth.Grant{}, false
	}
	return repo, actor, true
}

func agentOperated(repo repositories.Repository, p deliveryteams.Participant, user string, orgs deliveryOrganizationStore) bool {
	if repo.OrganizationID == "" {
		return false
	}
	o, e := orgs.Get(repo.OrganizationID)
	if e != nil {
		return false
	}
	for _, a := range o.Agents {
		if a.ID == p.PrincipalID {
			for _, x := range a.OperatorIDs {
				if x == user {
					return true
				}
			}
		}
	}
	return false
}
func deliveryAccess(repo repositories.Repository, organizer, kind, principal string, requested []string, repos proposalRepositoryStore, orgs deliveryOrganizationStore) (deliveryteams.AccessPreview, error) {
	allowed := map[string]bool{"contents:read": true, "discussion:write": true, "candidate_branch:write": true}
	sources := []string{"repository participation"}
	if repo.OwnerID == organizer {
		allowed["metadata:write"] = true
		allowed["default_branch:write"] = true
		allowed["merge"] = true
		sources = []string{"repository ownership"}
	}
	participant := map[string]bool{}
	expiry := (*time.Time)(nil)
	if kind == "human" {
		member := principal == repo.OwnerID
		if !member {
			member, _ = repos.IsCollaborator(repo.ID, principal)
		}
		if !member {
			return deliveryteams.AccessPreview{}, deliveryteams.ErrForbidden
		}
		participant["contents:read"] = true
		participant["discussion:write"] = true
		participant["candidate_branch:write"] = true
		if principal == repo.OwnerID {
			participant["metadata:write"] = true
			participant["default_branch:write"] = true
			participant["merge"] = true
		}
		sources = []string{"existing repository role"}
	} else if kind == "agent" {
		if repo.OrganizationID == "" {
			return deliveryteams.AccessPreview{}, deliveryteams.ErrForbidden
		}
		o, e := orgs.Get(repo.OrganizationID)
		if e != nil {
			return deliveryteams.AccessPreview{}, deliveryteams.ErrForbidden
		}
		approved := false
		for _, a := range o.Agents {
			if a.ID == principal {
				approved = true
			}
		}
		if !approved {
			return deliveryteams.AccessPreview{}, deliveryteams.ErrForbidden
		}
		now := time.Now()
		for _, g := range o.RoleGrants {
			if g.PrincipalKind != "agent" || g.PrincipalID != principal || g.RevokedAt != nil || !now.Before(g.ExpiresAt) {
				continue
			}
			covers := false
			for _, x := range g.Resources {
				if x.RepositoryID == string(repo.ID) || (x.Kind == "repository" && x.ID == string(repo.ID)) {
					covers = true
				}
			}
			if !covers {
				continue
			}
			participant["contents:read"] = true
			if g.Role == "contributor" || g.Role == "maintainer" || g.Role == "operator" {
				participant["discussion:write"] = true
				participant["candidate_branch:write"] = true
			}
			if g.Role == "maintainer" || g.Role == "operator" {
				participant["metadata:write"] = true
			}
			sources = append(sources, "organization grant "+g.ID)
			if expiry == nil || g.ExpiresAt.Before(*expiry) {
				x := g.ExpiresAt
				expiry = &x
			}
		}
	} else {
		return deliveryteams.AccessPreview{}, deliveryteams.ErrInvalid
	}
	actions := []string{}
	missing := []string{}
	for _, x := range requested {
		if allowed[x] && participant[x] {
			actions = append(actions, x)
		} else {
			missing = append(missing, x)
		}
	}
	sort.Strings(actions)
	sort.Strings(missing)
	return deliveryteams.AccessPreview{Actions: actions, Sources: sources, ExpiresAt: expiry, Missing: missing, GrantsAuthority: false}, nil
}
func writeDeliveryTeamResult(w http.ResponseWriter, v any, e error, status int) {
	if e == nil {
		writeJSON(w, status, v)
		return
	}
	switch {
	case errors.Is(e, deliveryteams.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "delivery_team_not_found"})
	case errors.Is(e, deliveryteams.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "authority_not_held"})
	case errors.Is(e, deliveryteams.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "delivery_team_changed"})
	case errors.Is(e, deliveryteams.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_delivery_team"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}
