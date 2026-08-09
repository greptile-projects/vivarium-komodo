package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyinventory"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type recoveryInventories struct{ item dependencyinventory.Inventory }

func (f recoveryInventories) List(repositoryID string) ([]dependencyinventory.Inventory, error) {
	if f.item.RepositoryID == repositoryID {
		return []dependencyinventory.Inventory{f.item}, nil
	}
	return []dependencyinventory.Inventory{}, nil
}

func (f recoveryInventories) Get(repositoryID, id string) (dependencyinventory.Inventory, error) {
	if f.item.RepositoryID == repositoryID && f.item.ID == id {
		return f.item, nil
	}
	return dependencyinventory.Inventory{}, dependencyinventory.ErrNotFound
}
func (f recoveryInventories) Consumers(packageID string) ([]dependencyinventory.Inventory, error) {
	for _, r := range f.item.Resolutions {
		if r.PackageVersionID == packageID {
			return []dependencyinventory.Inventory{f.item}, nil
		}
	}
	return []dependencyinventory.Inventory{}, nil
}

func TestQuarantineStopsNewInstallsButRetainsExposure(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	publisher, _ := catalog.Create("publisher", repositories.Metadata{Name: "sdk", Visibility: repositories.Public})
	consumer, _ := catalog.Create("consumer", repositories.Metadata{Name: "app", Visibility: repositories.Public})
	store, _ := packagecatalog.New(t.TempDir())
	publish := func(version string) packagecatalog.Version {
		body := []byte(version)
		sum := sha256.Sum256(body)
		v, e := store.Publish(packagecatalog.PublishParams{OwnerID: "publisher", Name: "sdk", Version: version, RepositoryID: string(publisher.ID), ReleaseID: "release-" + version, SourceCommitID: "commit-" + version, ArtifactID: "artifact-" + version, ArtifactPath: "sdk.tgz", ArtifactMediaType: "application/gzip", ArtifactSize: int64(len(body)), ExpectedSHA256: hex.EncodeToString(sum[:]), Build: packagecatalog.BuildAttestation{RunID: "run-" + version, BuildName: "package"}, Platform: packagecatalog.Platform{OS: "linux", Arch: "amd64"}, PublisherID: "publisher", Visibility: "public"}, bytes.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	unsafe, replacement := publish("1.0.0"), publish("1.0.1")
	inventories := recoveryInventories{dependencyinventory.Inventory{ID: "inventory", RepositoryID: string(consumer.ID), CommitID: "consumer-commit", DeploymentID: "production", Resolutions: []dependencyinventory.Resolution{{Identity: unsafe.Identity, PackageVersionID: unsafe.ID, Version: unsafe.Version, Direct: true, Status: "resolved"}}}}
	credentials, _ := auth.New(t.TempDir())
	token := issueAccess(t, credentials, "publisher", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	plans, _ := proposals.New(t.TempDir())
	mux := http.NewServeMux()
	registerPackagesHTTP(mux, store, nil, nil, catalog, credentials)
	registerPackageRecoveryHTTP(mux, store, inventories, plans, catalog, credentials, nil)
	body, _ := json.Marshal(map[string]any{"state": "quarantined", "reason": "malicious postinstall", "replacement_version_id": replacement.ID})
	req := httptest.NewRequest(http.MethodPut, "/repositories/"+string(publisher.ID)+"/packages/"+unsafe.ID+"/safety", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != 200 || !bytes.Contains(response.Body.Bytes(), []byte(`"notified_repository_count":1`)) {
		t.Fatalf("quarantine = %d %s", response.Code, response.Body.String())
	}
	registry := httptest.NewRecorder()
	mux.ServeHTTP(registry, httptest.NewRequest(http.MethodGet, "/package-registry/artifacts/"+unsafe.ID, nil))
	if registry.Code != 404 {
		t.Fatalf("quarantined install = %d", registry.Code)
	}
	exposed := httptest.NewRecorder()
	mux.ServeHTTP(exposed, httptest.NewRequest(http.MethodGet, "/packages/"+unsafe.ID+"/exposure", nil))
	if exposed.Code != 200 || !bytes.Contains(exposed.Body.Bytes(), []byte(`"deployment_id":"production"`)) {
		t.Fatalf("exposure = %d %s", exposed.Code, exposed.Body.String())
	}
	historical := httptest.NewRecorder()
	mux.ServeHTTP(historical, httptest.NewRequest(http.MethodGet, "/repositories/"+string(publisher.ID)+"/packages/"+unsafe.ID+"/artifact", nil))
	if historical.Code != 200 {
		t.Fatalf("historical evidence = %d %s", historical.Code, historical.Body.String())
	}
}
