package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenanceassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancegraphs"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancepolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func registerProvenanceAssessmentsHTTP(mux *http.ServeMux, s *provenanceassessments.Store, graphs *provenancegraphs.Store, policies *provenancepolicies.Store, pulls pullRequestStore, repos pullRequestRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/provenance-assessments"
	mux.HandleFunc("POST "+base, createProvenanceAssessment(s, graphs, policies, pulls, repos, credentials))
	mux.HandleFunc("GET "+base, listProvenanceAssessments(s, policies, graphs, repos, credentials))
	mux.HandleFunc("GET "+base+"/{assessment}", getProvenanceAssessment(s, policies, graphs, repos, credentials))
	mutate := func(suffix string, decision bool) {
		mux.HandleFunc("POST "+base+"/{assessment}/"+suffix, func(w http.ResponseWriter, r *http.Request) {
			repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
			if !ok {
				return
			}
			v, e := s.Get(r.PathValue("assessment"))
			if e != nil || v.RepositoryID != string(repo.ID) {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return
			}
			var body struct {
				ExpectedRevision int64                            `json:"expected_revision"`
				ActorKind        string                           `json:"actor_kind"`
				Annotation       provenanceassessments.Annotation `json:"annotation"`
				Decision         provenanceassessments.Decision   `json:"decision"`
			}
			if !readJSON(w, r, &body, 64<<10) {
				return
			}
			if decision {
				if actor.UserID != repo.OwnerID {
					writeJSON(w, 403, map[string]string{"error": "provenance_owner_required"})
					return
				}
				body.Decision.OwnerID = actor.UserID
				v, e = s.Decide(v.ID, actor.UserID, body.ExpectedRevision, body.Decision)
			} else {
				if body.ActorKind == "" {
					body.ActorKind = "human"
				}
				v, e = s.Annotate(v.ID, actor.UserID, body.ActorKind, body.ExpectedRevision, body.Annotation)
			}
			writeProvenanceMutation(w, e, v, policies, graphs)
		})
	}
	mutate("annotations", false)
	mutate("decisions", true)
	mux.HandleFunc("POST "+base+"/{assessment}/findings/{finding}/repairs", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "provenance_owner_required"})
			return
		}
		v, e := s.Get(r.PathValue("assessment"))
		if e != nil || v.RepositoryID != string(repo.ID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in provenanceassessments.Repair
		var body struct {
			ExpectedRevision int64                        `json:"expected_revision"`
			Repair           provenanceassessments.Repair `json:"repair"`
		}
		if !readJSON(w, r, &body, 256<<10) {
			return
		}
		in = body.Repair
		in.FindingID = r.PathValue("finding")
		in.Obligations = repairObligations(v, in.FindingID, graphs)
		updated, made, e := s.CreateRepair(v.ID, actor.UserID, body.ExpectedRevision, in)
		if provenanceRepairError(w, e) {
			return
		}
		writeJSON(w, 201, map[string]any{"assessment": provenanceassessments.Derive(updated, currentProvenanceKeys(updated, policies, graphs), time.Now().UTC()), "repair": made, "authority_notice": "The finding and repair grant no code, evidence, agent, review, merge, disclosure, distribution, or release authority."})
	})
	mux.HandleFunc("POST "+base+"/{assessment}/repairs/{repair}/progress", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		v, e := s.Get(r.PathValue("assessment"))
		if e != nil || v.RepositoryID != string(repo.ID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var body struct {
			ExpectedRevision int64  `json:"expected_revision"`
			Status           string `json:"status"`
			Summary          string `json:"summary"`
		}
		if !readJSON(w, r, &body, 128<<10) {
			return
		}
		v, e = s.ProgressRepair(v.ID, r.PathValue("repair"), actor.UserID, body.Status, body.Summary, body.ExpectedRevision)
		if provenanceRepairError(w, e) {
			return
		}
		view := provenanceassessments.Derive(v, currentProvenanceKeys(v, policies, graphs), time.Now().UTC())
		writeJSON(w, 201, provenanceassessments.Project(view, actor.UserID == repo.OwnerID))
	})
	mux.HandleFunc("POST "+base+"/{assessment}/repairs/{repair}/delivery", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		v, e := s.Get(r.PathValue("assessment"))
		if e != nil || v.RepositoryID != string(repo.ID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var body struct {
			ExpectedRevision int64                                `json:"expected_revision"`
			Delivery         provenanceassessments.RepairDelivery `json:"delivery"`
		}
		if !readJSON(w, r, &body, 256<<10) {
			return
		}
		pull, e := pulls.Get(string(repo.ID), body.Delivery.PullRequestID)
		if e != nil || pull.SourceCommitID != body.Delivery.Revision {
			writeJSON(w, 422, map[string]string{"error": "revision_exact_pull_request_required"})
			return
		}
		v, e = s.DeliverRepair(v.ID, r.PathValue("repair"), actor.UserID, body.ExpectedRevision, body.Delivery)
		if provenanceRepairError(w, e) {
			return
		}
		view := provenanceassessments.Derive(v, currentProvenanceKeys(v, policies, graphs), time.Now().UTC())
		writeJSON(w, 201, provenanceassessments.Project(view, actor.UserID == repo.OwnerID))
	})
}

func repairObligations(v provenanceassessments.Assessment, finding string, graphs *provenancegraphs.Store) []string {
	subject := ""
	for _, f := range v.Findings {
		if f.ID == finding {
			subject = f.Subject
		}
	}
	items, _ := graphs.List(v.RepositoryID)
	out := []string{}
	for _, g := range items {
		if g.ID == v.GraphID {
			for _, n := range g.Nodes {
				if n.ID == subject {
					out = append(out, n.Obligations...)
				}
			}
			break
		}
	}
	return out
}
func provenanceRepairError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	status, code := 422, "invalid_provenance_repair"
	if errors.Is(err, provenanceassessments.ErrNotFound) {
		status, code = 404, "provenance_repair_not_found"
	}
	if errors.Is(err, provenanceassessments.ErrConflict) {
		status, code = 409, "provenance_repair_conflict"
	}
	writeJSON(w, status, map[string]string{"error": code})
	return true
}

func createProvenanceAssessment(s *provenanceassessments.Store, graphs *provenancegraphs.Store, policies *provenancepolicies.Store, pulls pullRequestStore, repos pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			CandidateKind       string   `json:"candidate_kind"`
			CandidateID         string   `json:"candidate_id"`
			Revision            string   `json:"revision"`
			GraphID             string   `json:"graph_id"`
			PolicyID            string   `json:"policy_id"`
			PolicyVersion       int64    `json:"policy_version"`
			DistributionTargets []string `json:"distribution_targets"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		opened, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if _, e = opened.ReadCommit(storage.ObjectID(in.Revision)); e != nil || !visibleCommit(opened, storage.ObjectID(in.Revision)) {
			writeJSON(w, 422, map[string]string{"error": "candidate_revision_not_visible"})
			return
		}
		if in.CandidateKind == "pull_request" {
			p, e := pulls.Get(string(repo.ID), in.CandidateID)
			if e != nil || p.SourceCommitID != in.Revision {
				writeJSON(w, 422, map[string]string{"error": "candidate_revision_mismatch"})
				return
			}
		}
		gs, e := graphs.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		var g *provenancegraphs.Graph
		for i := range gs {
			if gs[i].ID == in.GraphID && gs[i].Revision == in.Revision {
				g = &gs[i]
				break
			}
		}
		if g == nil {
			writeJSON(w, 422, map[string]string{"error": "current_provenance_graph_required"})
			return
		}
		p, e := policies.Get("repository", string(repo.ID), in.PolicyID)
		if e != nil || in.PolicyVersion != p.CurrentVersion {
			writeJSON(w, 422, map[string]string{"error": "current_provenance_policy_required"})
			return
		}
		pv := p.Versions[len(p.Versions)-1]
		findings, valid := compareProvenance(*g, pv, in.DistributionTargets)
		if !valid {
			writeJSON(w, 422, map[string]string{"error": "invalid_distribution_target"})
			return
		}
		keys := provenanceInputKeys(*g, p)
		v, e := s.Create(provenanceassessments.Assessment{RepositoryID: string(repo.ID), CandidateKind: in.CandidateKind, CandidateID: in.CandidateID, Revision: in.Revision, DistributionTargets: in.DistributionTargets, GraphID: g.ID, PolicyID: p.ID, PolicyVersion: p.CurrentVersion, InputKeys: keys, Findings: findings, CreatedByID: actor.UserID})
		if errors.Is(e, provenanceassessments.ErrConflict) {
			writeJSON(w, 409, map[string]string{"error": "assessment_exists"})
			return
		}
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_provenance_assessment"})
			return
		}
		writeJSON(w, 201, provenanceassessments.Derive(v, keys, time.Now().UTC()))
	}
}

func provenanceInputKeys(g provenancegraphs.Graph, p provenancepolicies.Policy) []provenanceassessments.InputKey {
	out := []provenanceassessments.InputKey{{Kind: "candidate", Reference: g.RepositoryID, Revision: g.Revision}, {Kind: "graph", Reference: g.ID, Revision: g.DeclarationSHA256}, {Kind: "policy", Reference: p.ID, Revision: strconv.FormatInt(p.CurrentVersion, 10)}}
	for _, n := range g.Nodes {
		if n.Kind == "dependency" || n.Kind == "tool" {
			rev := "unknown"
			for _, c := range n.Citations {
				if c.Revision != "" {
					rev = c.Revision
					break
				}
			}
			out = append(out, provenanceassessments.InputKey{Kind: n.Kind, Reference: n.ID, Revision: rev})
		}
	}
	return out
}
func currentProvenanceKeys(v provenanceassessments.Assessment, policies *provenancepolicies.Store, graphs *provenancegraphs.Store) []provenanceassessments.InputKey {
	out := []provenanceassessments.InputKey{}
	for _, key := range v.InputKeys {
		if key.Kind != "graph" {
			out = append(out, key)
		}
	}
	if items, e := graphs.List(v.RepositoryID); e == nil {
		for _, graph := range items {
			if graph.Revision == v.Revision {
				out = append(out, provenanceassessments.InputKey{Kind: "graph", Reference: graph.ID, Revision: graph.DeclarationSHA256})
				break
			}
		}
	}
	p, e := policies.Get("repository", v.RepositoryID, v.PolicyID)
	if e == nil {
		for i := range out {
			if out[i].Kind == "policy" {
				out[i].Revision = strconv.FormatInt(p.CurrentVersion, 10)
			}
		}
	}
	return out
}

func compareProvenance(g provenancegraphs.Graph, p provenancepolicies.Version, targets []string) ([]provenanceassessments.Finding, bool) {
	out := []provenanceassessments.Finding{}
	n := 0
	add := func(kind, subject, detail string, blocking bool) {
		n++
		out = append(out, provenanceassessments.Finding{ID: "finding-" + strconv.Itoa(n), Kind: kind, Subject: subject, Detail: detail, Blocking: blocking, DistributionTargets: append([]string{}, targets...)})
	}
	contexts := map[string]provenancepolicies.DistributionContext{}
	for _, c := range p.DistributionContexts {
		contexts[c.ID] = c
	}
	for _, t := range targets {
		if _, ok := contexts[t]; !ok {
			return nil, false
		}
	}
	for _, gap := range g.Gaps {
		kind := "unattributed_material"
		if gap.Kind != "missing_origin" && gap.Kind != "stale_citation" {
			kind = "provenance_gap"
		}
		add(kind, gap.Subject, gap.Detail, true)
	}
	rules := map[string]provenancepolicies.MaterialRule{}
	for _, r := range p.Rules {
		rules[r.Kind] = r
	}
	for _, node := range g.Nodes {
		kind := node.Kind
		if kind == "file" || kind == "fragment" {
			kind = "source"
		}
		if kind == "tool" {
			kind = "build_input"
		}
		if node.Transformation == "generated" {
			kind = "generated_code"
		}
		rule, ok := rules[kind]
		if !ok {
			continue
		}
		if node.License == "" {
			add("unidentified_license", node.ID, "material has no identified license", true)
		} else if !containsFold(rule.Licenses, node.License) {
			add("incompatible_license", node.ID, "license "+node.License+" is not permitted for "+kind, true)
		}
		claims := strings.ToLower(strings.Join(node.Claims, "\n"))
		for _, claim := range node.Claims {
			parts := strings.SplitN(claim, "=", 2)
			if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "origin") && !containsFold(rule.Origins, strings.TrimSpace(parts[1])) {
				add("unpermitted_origin", node.ID, "origin "+strings.TrimSpace(parts[1])+" is not permitted for "+kind, true)
			}
		}
		for _, x := range rule.Attribution {
			if !strings.Contains(claims, strings.ToLower(x)) {
				add("required_attribution", node.ID, x, true)
			}
		}
		for _, x := range rule.Attestations {
			if !strings.Contains(claims, strings.ToLower(x)) {
				add("contributor_attestation", node.ID, x, true)
			}
		}
		if kind == "generated_code" && node.Transformation == "" {
			add("generated_output_concern", node.ID, "generated material does not identify its transformation", true)
		}
		for _, o := range node.Obligations {
			lo := strings.ToLower(o)
			if strings.Contains(lo, "notice") {
				add("required_notice", node.ID, o, true)
			}
			if strings.Contains(lo, "source") || strings.Contains(lo, "offer") {
				add("source_offer_obligation", node.ID, o, true)
			}
		}
	}
	for _, edge := range g.Edges {
		if edge.Kind == "generated" {
			foundTool := false
			for _, node := range g.Nodes {
				if node.ID == edge.From && (node.Kind == "tool" || node.Kind == "build_step") {
					foundTool = true
				}
			}
			if !foundTool {
				add("generated_output_concern", edge.To, "generated output does not identify a generator or build step", true)
			}
		}
	}
	for _, t := range targets {
		c := contexts[t]
		if c.NoticeRequired {
			add("required_notice", t, "distribution context requires a notice bundle", true)
		}
		for _, owner := range c.OwnerIDs {
			add("owner_acknowledgement", t, "distribution owner "+owner+" must acknowledge the exact candidate", true)
		}
	}
	return out, true
}
func containsFold(xs []string, x string) bool {
	for _, v := range xs {
		if strings.EqualFold(v, x) {
			return true
		}
	}
	return false
}
func listProvenanceAssessments(s *provenanceassessments.Store, p *provenancepolicies.Store, g *provenancegraphs.Store, repos pullRequestRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := s.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		out := []provenanceassessments.View{}
		for _, v := range xs {
			view := provenanceassessments.Derive(v, currentProvenanceKeys(v, p, g), time.Now().UTC())
			out = append(out, provenanceassessments.Project(view, actor.UserID == repo.OwnerID))
		}
		writeJSON(w, 200, map[string]any{"items": out, "total_count": len(out)})
	}
}
func getProvenanceAssessment(s *provenanceassessments.Store, p *provenancepolicies.Store, g *provenancegraphs.Store, repos pullRequestRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := s.Get(r.PathValue("assessment"))
		if e != nil || v.RepositoryID != string(repo.ID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		view := provenanceassessments.Derive(v, currentProvenanceKeys(v, p, g), time.Now().UTC())
		writeJSON(w, 200, provenanceassessments.Project(view, actor.UserID == repo.OwnerID))
	}
}
func writeProvenanceMutation(w http.ResponseWriter, e error, v provenanceassessments.Assessment, p *provenancepolicies.Store, g *provenancegraphs.Store) {
	switch {
	case errors.Is(e, provenanceassessments.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "assessment_revision_conflict"})
	case errors.Is(e, provenanceassessments.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_provenance_record"})
	case e != nil:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	default:
		writeJSON(w, 201, provenanceassessments.Derive(v, currentProvenanceKeys(v, p, g), time.Now().UTC()))
	}
}
