package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributionopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
)

func registerContributionOpportunitiesHTTP(mux *http.ServeMux, store *contributionopportunities.Store, repos contributorPathwayRepositories, credentials authStore, issueStore *issues.Store, proposalStore *proposals.Store, organizationStore *organizations.Store) {
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
		writeJSON(w, 200, map[string]any{"items": d.Opportunities, "active_claims": active, "grants_write_access": false})
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
