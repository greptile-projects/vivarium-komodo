package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/governance"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func governancePreview(in governance.Input, repo *repositories.Repository, org *organizations.Organization) governance.Preview {
	p := governance.Preview{GeneratedAt: time.Now().UTC(), AuthorityGranted: false}
	add := func(k, r, state, detail string, block bool) {
		p.Items = append(p.Items, governance.PreviewItem{Kind: k, Resource: r, State: state, Detail: detail, Blocking: block})
		if block {
			p.Blockers = append(p.Blockers, detail)
		}
	}
	people := 1
	if repo != nil {
		people += len(repo.CollaboratorIDs)
		add("ownership", "repository:"+string(repo.ID), "retained", "The repository owner remains the operational authority; the charter grants no access.", false)
		for branch, checks := range repo.RequiredChecks {
			add("branch", "branch:"+branch, "compatible", "Existing protection requires checks: "+strings.Join(checks, ", "), false)
		}
		for branch := range repo.IntegrationQueue {
			add("branch", "branch:"+branch, "compatible", "Existing integration queue remains authoritative.", false)
		}
	}
	if org != nil {
		people = 0
		for _, m := range org.Members {
			if !m.AcceptedAt.IsZero() {
				people++
			}
		}
		add("ownership", "organization:"+org.ID, "retained", "Organization owners retain resource administration; governance roles do not imply membership or access.", false)
		for _, t := range org.Teams {
			add("team", "team:"+t.ID, "observed", "Existing team responsibilities remain separately enforced.", false)
		}
		for _, a := range org.Agents {
			add("agent", "agent:"+a.ID, "observed", "Approved agent capabilities remain separately bounded and agents receive no human vote.", false)
		}
	}
	known := map[string]bool{"ownership": true, "teams": true, "branches": true, "releases": true, "environments": true, "security": true, "agents": true}
	for _, r := range in.ProtectedResources {
		key := strings.SplitN(r, ":", 2)[0]
		if !known[key] {
			add("protected_resource", r, "unsupported", "Protected resource "+r+" cannot be mapped to an existing policy boundary.", true)
		} else {
			add("protected_resource", r, "protected", "The charter declares this resource protected; its existing owner policy still controls changes.", false)
		}
	}
	for _, role := range in.Roles {
		if role.MinimumMembers > people {
			add("eligibility", "role:"+role.Name, "impossible", "Role "+role.Name+" requires more members than the current eligible project population.", true)
		} else {
			add("eligibility", "role:"+role.Name, "possible", "Current project population can satisfy the declared minimum.", false)
		}
	}
	for _, d := range in.DecisionClasses {
		if d.Quorum > people {
			add("quorum", "decision:"+d.Name, "impossible", "Decision class "+d.Name+" has quorum above the current project population.", true)
		}
	}
	return p
}

func governanceError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, governance.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(e, governance.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_charter"})
	case errors.Is(e, governance.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "charter_conflict"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}

func registerGovernanceHTTP(mux *http.ServeMux, s *governance.Store, repos ownedRepositoryStore, orgs *organizations.Store, credentials authStore) {
	register := func(scopeType, base string) {
		access := func(w http.ResponseWriter, r *http.Request, write bool) (string, auth.Grant, *repositories.Repository, *organizations.Organization, bool) {
			scope := auth.RepositoryRead
			if write {
				scope = auth.RepositoryWrite
			}
			a, ok := authenticateRequest(w, r, credentials, scope)
			if !ok {
				return "", a, nil, nil, false
			}
			id := r.PathValue(scopeType)
			if scopeType == "repository" {
				v, e := repos.Inspect(storage.ID(id))
				if e != nil {
					writeRepositoryError(w, e)
					return "", a, nil, nil, false
				}
				if write && v.OwnerID != a.UserID {
					writeJSON(w, 403, map[string]string{"error": "owner_required"})
					return "", a, nil, nil, false
				}
				if !write && v.Visibility != repositories.Public && v.OwnerID != a.UserID {
					member, _ := repos.IsCollaborator(v.ID, a.UserID)
					if !member {
						writeJSON(w, 404, map[string]string{"error": "not_found"})
						return "", a, nil, nil, false
					}
				}
				return id, a, &v, nil, true
			}
			v, e := orgs.Get(id)
			if e != nil || (!orgs.IsMember(id, a.UserID) && !orgs.IsOwner(id, a.UserID)) {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return "", a, nil, nil, false
			}
			if write && !orgs.IsOwner(id, a.UserID) {
				writeJSON(w, 403, map[string]string{"error": "organization_owner_required"})
				return "", a, nil, nil, false
			}
			return id, a, nil, &v, true
		}
		mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
			id, _, _, _, ok := access(w, r, false)
			if !ok {
				return
			}
			v, e := s.Get(scopeType, id)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 200, v)
		})
		mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
			id, a, repo, org, ok := access(w, r, true)
			if !ok {
				return
			}
			var in struct {
				ExpectedVersion int64 `json:"expected_version"`
				governance.Input
			}
			if !readJSON(w, r, &in, 128<<10) {
				return
			}
			v, e := s.Publish(scopeType, id, a.UserID, in.ExpectedVersion, in.Input, governancePreview(in.Input, repo, org))
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 201, v)
		})
		mux.HandleFunc("POST "+base+"/approvals", func(w http.ResponseWriter, r *http.Request) {
			id, a, _, _, ok := access(w, r, true)
			if !ok {
				return
			}
			var in struct {
				Version int64  `json:"version"`
				Note    string `json:"note"`
			}
			if !readJSON(w, r, &in, 8<<10) {
				return
			}
			v, e := s.Approve(scopeType, id, a.UserID, in.Note, in.Version)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 201, v)
		})
		mux.HandleFunc("POST "+base+"/activation", func(w http.ResponseWriter, r *http.Request) {
			id, a, repo, org, ok := access(w, r, true)
			if !ok {
				return
			}
			var in struct {
				Version int64 `json:"version"`
			}
			if !readJSON(w, r, &in, 8<<10) {
				return
			}
			v, e := s.Get(scopeType, id)
			if e != nil {
				governanceError(w, e)
				return
			}
			v, e = s.Activate(scopeType, id, a.UserID, in.Version, governancePreview(v.Current.Input, repo, org))
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 200, v)
		})
		mux.HandleFunc("POST "+base+"/exceptions", func(w http.ResponseWriter, r *http.Request) {
			id, a, _, _, ok := access(w, r, true)
			if !ok {
				return
			}
			var in struct {
				Version int64 `json:"version"`
				governance.Exception
			}
			if !readJSON(w, r, &in, 8<<10) {
				return
			}
			v, e := s.Except(scopeType, id, a.UserID, in.Version, in.Exception)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 201, v)
		})
		mux.HandleFunc("POST "+base+"/standings", func(w http.ResponseWriter, r *http.Request) {
			id, a, _, _, ok := access(w, r, true)
			if !ok {
				return
			}
			var in struct {
				Version int64 `json:"version"`
				governance.StandingInput
			}
			if !readJSON(w, r, &in, 32<<10) {
				return
			}
			v, e := s.Invite(scopeType, id, a.UserID, in.Version, in.StandingInput)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 201, v)
		})
		mux.HandleFunc("POST "+base+"/standings/{standing}/{action}", func(w http.ResponseWriter, r *http.Request) {
			id, a, _, _, ok := access(w, r, false)
			if !ok {
				return
			}
			action := r.PathValue("action")
			ownerAction := action == "suspend" || action == "expire" || action == "revoke_identity" || action == "revoke_federation_trust"
			if ownerAction {
				_, a, _, _, ok = access(w, r, true)
				if !ok {
					return
				}
			}
			var in struct {
				Reason string `json:"reason"`
			}
			if !readJSON(w, r, &in, 8<<10) {
				return
			}
			v, e := s.Transition(scopeType, id, r.PathValue("standing"), a.UserID, action, in.Reason)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 200, v)
		})
		mux.HandleFunc("GET "+base+"/health", func(w http.ResponseWriter, r *http.Request) {
			id, _, _, _, ok := access(w, r, false)
			if !ok {
				return
			}
			v, e := s.Health(scopeType, id)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 200, v)
		})
		mux.HandleFunc("POST "+base+"/stewardship", func(w http.ResponseWriter, r *http.Request) {
			id, a, _, _, ok := access(w, r, true)
			if !ok {
				return
			}
			var in governance.StewardshipInput
			if !readJSON(w, r, &in, 64<<10) {
				return
			}
			v, e := s.OpenStewardship(scopeType, id, a.UserID, in)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 201, v)
		})
		mux.HandleFunc("POST "+base+"/stewardship/{case}/{action}", func(w http.ResponseWriter, r *http.Request) {
			id, a, _, _, ok := access(w, r, true)
			if !ok {
				return
			}
			var in struct {
				Reason   string `json:"reason"`
				Resource string `json:"resource"`
			}
			if !readJSON(w, r, &in, 16<<10) {
				return
			}
			v, e := s.TransitionStewardship(scopeType, id, r.PathValue("case"), a.UserID, r.PathValue("action"), in.Reason, in.Resource)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 200, v)
		})
		mux.HandleFunc("GET "+base+"/proposals", func(w http.ResponseWriter, r *http.Request) {
			id, _, _, _, ok := access(w, r, false)
			if !ok {
				return
			}
			v, e := s.ListProposals(scopeType, id)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 200, map[string]any{"items": v})
		})
		mux.HandleFunc("POST "+base+"/proposals", func(w http.ResponseWriter, r *http.Request) {
			id, a, _, _, ok := access(w, r, false)
			if !ok {
				return
			}
			var in governance.ProposalInput
			if !readJSON(w, r, &in, 128<<10) {
				return
			}
			v, e := s.OpenProposal(scopeType, id, a.UserID, in)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 201, v)
		})
		mux.HandleFunc("GET "+base+"/proposals/{proposal}", func(w http.ResponseWriter, r *http.Request) {
			id, _, _, _, ok := access(w, r, false)
			if !ok {
				return
			}
			v, e := s.GetProposal(scopeType, id, r.PathValue("proposal"))
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 200, v)
		})
		mux.HandleFunc("POST "+base+"/proposals/{proposal}/discussion", func(w http.ResponseWriter, r *http.Request) {
			id, a, _, _, ok := access(w, r, false)
			if !ok {
				return
			}
			var in struct {
				ActorKind string                        `json:"actor_kind"`
				Body      string                        `json:"body"`
				Citations []governance.ProposalEvidence `json:"citations"`
			}
			if !readJSON(w, r, &in, 64<<10) {
				return
			}
			if in.ActorKind == "" {
				in.ActorKind = "human"
			}
			v, e := s.Discuss(scopeType, id, r.PathValue("proposal"), a.UserID, in.ActorKind, in.Body, in.Citations)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 201, v)
		})
		mux.HandleFunc("POST "+base+"/proposals/{proposal}/ballots", func(w http.ResponseWriter, r *http.Request) {
			id, a, _, _, ok := access(w, r, false)
			if !ok {
				return
			}
			var in struct {
				Choice  string `json:"choice"`
				Reason  string `json:"reason"`
				Abstain bool   `json:"abstain"`
			}
			if !readJSON(w, r, &in, 16<<10) {
				return
			}
			v, e := s.Cast(scopeType, id, r.PathValue("proposal"), a.UserID, in.Choice, in.Reason, in.Abstain)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 201, v)
		})
		mux.HandleFunc("POST "+base+"/proposals/{proposal}/tally", func(w http.ResponseWriter, r *http.Request) {
			id, _, _, _, ok := access(w, r, true)
			if !ok {
				return
			}
			var in struct {
				CloseEarly bool `json:"close_early"`
			}
			if !readJSON(w, r, &in, 8<<10) {
				return
			}
			v, e := s.Finalize(scopeType, id, r.PathValue("proposal"), in.CloseEarly)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 200, v)
		})
		mux.HandleFunc("POST "+base+"/proposals/{proposal}/implementation", func(w http.ResponseWriter, r *http.Request) {
			id, a, _, _, ok := access(w, r, true)
			if !ok {
				return
			}
			var in governance.ImplementationInput
			if !readJSON(w, r, &in, 64<<10) {
				return
			}
			v, e := s.RecordImplementation(scopeType, id, r.PathValue("proposal"), a.UserID, in)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 201, v)
		})
		mux.HandleFunc("POST "+base+"/proposals/{proposal}/contests", func(w http.ResponseWriter, r *http.Request) {
			id, a, _, _, ok := access(w, r, false)
			if !ok {
				return
			}
			var in struct {
				Reason   string                        `json:"reason"`
				Evidence []governance.ProposalEvidence `json:"evidence"`
			}
			if !readJSON(w, r, &in, 32<<10) {
				return
			}
			v, e := s.Contest(scopeType, id, r.PathValue("proposal"), a.UserID, in.Reason, in.Evidence)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 201, v)
		})
		mux.HandleFunc("POST "+base+"/proposals/{proposal}/contests/{contest}/resolution", func(w http.ResponseWriter, r *http.Request) {
			id, a, _, _, ok := access(w, r, true)
			if !ok {
				return
			}
			var in struct {
				Resolution string `json:"resolution"`
			}
			if !readJSON(w, r, &in, 16<<10) {
				return
			}
			v, e := s.ResolveContest(scopeType, id, r.PathValue("proposal"), r.PathValue("contest"), a.UserID, in.Resolution)
			if e != nil {
				governanceError(w, e)
				return
			}
			writeJSON(w, 200, v)
		})
	}
	register("repository", "/repositories/{repository}/governance-charter")
	register("organization", "/organizations/{organization}/governance-charter")
}
