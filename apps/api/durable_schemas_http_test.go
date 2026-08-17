package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/durableschemas"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestDurableSchemaAndMigrationPublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "state", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "consumer")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	consumer := issueAccess(t, credentials, "consumer", auth.API, auth.RepositoryRead)
	store, _ := durableschemas.New(t.TempDir())
	mux := http.NewServeMux()
	registerDurableSchemasHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/durable-schemas"
	body := `{"name":"jobs","store_kind":"queue","description":"background work envelope","source_revision":"0123456789012345678901234567890123456789","definition_path":"schemas/jobs.json","format":"json_schema","fields":[{"name":"user_id","type":"string","required":true,"classification":"personal","description":"account receiving the result"}],"owner_ids":["owner"],"compatibility":"consumers tolerate additive optional fields","retention":"delete after 7 days","privacy_commitments":["user ids remain in the EU"],"links":[{"kind":"service","resource_id":"worker","label":"job worker"},{"kind":"environment","resource_id":"production","label":"production queue"}],"change_reason":"publish reviewed schema"}`
	var schema durableschemas.Schema
	workflowJSON(t, server.URL, http.MethodPost, base, owner, body, 201, &schema)
	if schema.CurrentVersion != 1 || schema.Versions[0].AuthorID != "owner" || len(schema.Gaps) != 0 {
		t.Fatalf("unexpected schema: %+v", schema)
	}
	revised := body[:1] + `"expected_version":1,` + body[1:]
	revised = strings.Replace(revised, "additive optional fields", "additive fields; removed fields require migration", 1)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+schema.ID+"/versions", owner, revised, 201, &schema)
	migration := `{"title":"Re-key queued jobs","source_kind":"pull_request","source_id":"pull-42","schema_id":"` + schema.ID + `","from_version":1,"to_version":2,"operations":[{"kind":"read","object":"jobs.user_id","description":"read old identifier","reversible":true},{"kind":"write","object":"jobs.subject_id","description":"write replacement identifier","reversible":true},{"kind":"backfill","object":"pending jobs","description":"copy identifiers","reversible":true},{"kind":"destructive","object":"jobs.user_id","description":"remove old field after drain","destructive":true,"reversible":false}],"affected_consumers":["worker","admin replay"],"rollback_limits":["cannot restore user_id after old messages expire"],"steps":[{"id":"dual-write","description":"write both fields","operation_kinds":["write"],"owner_id":"owner"},{"id":"backfill","description":"copy pending messages","depends_on":["dual-write"],"operation_kinds":["read","backfill"],"owner_id":"owner"},{"id":"remove","description":"remove old field","depends_on":["backfill"],"operation_kinds":["destructive"],"owner_id":"owner"}],"success_measures":["zero old-field reads for 24h"],"required_approver_ids":["consumer"],"summary":"make the irreversible queue change visible before deployment"}`
	var plan durableschemas.Migration
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations", owner, migration, 201, &plan)
	if len(plan.Blockers) != 1 || plan.Blockers[0] != "approval_required:consumer" {
		t.Fatalf("approval not derived: %+v", plan)
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/schema-migrations/"+plan.ID+"/approvals", consumer, `{"owner_id":"consumer","decision":"approved","rationale":"dual-read window protects our consumer"}`, 201, &plan)
	if len(plan.Blockers) != 0 || len(plan.Events) != 2 {
		t.Fatalf("approval history missing: %+v", plan)
	}
}
