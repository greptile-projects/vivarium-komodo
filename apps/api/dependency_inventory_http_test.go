package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyinventory"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestDependencyInventoryConnectsCommitAndPublicConsumers(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	publisher, _ := catalog.Create("publisher", repositories.Metadata{Name: "library", Visibility: repositories.Public})
	consumer, _ := catalog.Create("owner", repositories.Metadata{Name: "application", Visibility: repositories.Public})
	packages, _ := packagecatalog.New(t.TempDir())
	body := []byte("artifact")
	digest := sha256.Sum256(body)
	version, err := packages.Publish(packagecatalog.PublishParams{OwnerID: "publisher", Name: "sdk", Version: "1.2.3", RepositoryID: string(publisher.ID), ReleaseID: "release", SourceCommitID: "source", ArtifactID: "artifact", ArtifactPath: "sdk.tgz", ArtifactMediaType: "application/gzip", ArtifactSize: int64(len(body)), ExpectedSHA256: hex.EncodeToString(digest[:]), Build: packagecatalog.BuildAttestation{RunID: "build", BuildName: "package", Command: "build", CompletedAt: time.Now()}, Platform: packagecatalog.Platform{OS: "linux", Arch: "amd64"}, PublisherID: "publisher", Visibility: "public", License: "Apache-2.0", SupportURL: "https://example.test/support"}, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{"version":1,"direct_dependencies":["@publisher/sdk"]}`)
	lock, _ := json.Marshal(map[string]any{"version": 1, "packages": []any{map[string]any{"identity": version.Identity, "version": version.Version, "package_version_id": version.ID, "dependencies": []string{}}}})
	opened, _ := catalog.Open(consumer.ID)
	manifestBlob := writeObject(t, opened, storage.BlobObject, manifest)
	lockBlob := writeObject(t, opened, storage.BlobObject, lock)
	komodoTree := writeObject(t, opened, storage.TreeObject, append(treeEntry("100644", "packages.json", manifestBlob), treeEntry("100644", "packages.lock.json", lockBlob)...))
	root := writeObject(t, opened, storage.TreeObject, treeEntry("40000", ".komodo", komodoTree))
	commit := writeCommit(t, opened, root, nil, "lock dependencies")
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	inventories, _ := dependencyinventory.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerDependencyInventoryHTTP(mux, inventories, packages, nil, nil, nil, catalog, credentials)
	payload, _ := json.Marshal(map[string]string{"commit_id": string(commit)})
	req := httptest.NewRequest(http.MethodPost, "/repositories/"+string(consumer.ID)+"/dependency-inventories", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != 201 {
		t.Fatalf("create = %d %s", response.Code, response.Body.String())
	}
	var inventory dependencyinventory.Inventory
	_ = json.Unmarshal(response.Body.Bytes(), &inventory)
	if inventory.Status != "resolved" || len(inventory.Resolutions) != 1 || !inventory.Resolutions[0].Direct || inventory.CreatedByID != "owner" {
		t.Fatalf("inventory = %#v", inventory)
	}
	public := httptest.NewRecorder()
	mux.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/packages/"+version.ID+"/consumers", nil))
	if public.Code != 200 || !bytes.Contains(public.Body.Bytes(), []byte(consumer.ID)) {
		t.Fatalf("consumers = %d %s", public.Code, public.Body.String())
	}
}
