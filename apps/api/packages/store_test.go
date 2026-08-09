package packages

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"
)

func TestPublishIsImmutableAndCopiesVerifiedBytes(t *testing.T) {
	store, _ := New(t.TempDir())
	content := []byte("trusted package")
	digest := sha256.Sum256(content)
	p := PublishParams{OwnerID: "owner", Name: "sdk", Version: "1.2.3", RepositoryID: "repo", ReleaseID: "release", SourceCommitID: "commit", ArtifactID: "artifact", ArtifactPath: "dist/sdk.tgz", ArtifactMediaType: "application/gzip", ArtifactSize: int64(len(content)), ExpectedSHA256: hex.EncodeToString(digest[:]), Build: BuildAttestation{RunID: "run", BuildName: "package", Command: "build"}, Platform: Platform{OS: "linux", Arch: "amd64", Runtime: "go1.24"}, Dependencies: map[string]string{"@owner/core": "^1.0.0"}, PublisherID: "owner", Visibility: "public"}
	item, err := store.Publish(p, bytes.NewReader(content))
	if err != nil || item.Identity != "@owner/sdk" || item.Lifecycle != "active" || item.SHA256 != p.ExpectedSHA256 {
		t.Fatalf("publish = %#v, %v", item, err)
	}
	_, file, err := store.OpenArtifact("repo", item.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	got, _ := io.ReadAll(file)
	if !bytes.Equal(got, content) {
		t.Fatalf("content = %q", got)
	}
	if _, err = store.Publish(p, bytes.NewReader(content)); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("conflict = %v", err)
	}
}

func TestFailedUploadIsNotVisible(t *testing.T) {
	store, _ := New(t.TempDir())
	p := PublishParams{OwnerID: "owner", Name: "sdk", Version: "1.0.0", RepositoryID: "repo", ReleaseID: "release", SourceCommitID: "commit", ArtifactID: "artifact", ArtifactSize: 4, ExpectedSHA256: string(make([]byte, 64)), Build: BuildAttestation{RunID: "run", BuildName: "build"}, Platform: Platform{OS: "linux", Arch: "amd64"}, PublisherID: "owner", Visibility: "private"}
	if _, err := store.Publish(p, bytes.NewBufferString("bad")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("publish = %v", err)
	}
	items, _ := store.List("repo")
	if len(items) != 0 {
		t.Fatalf("partial items = %#v", items)
	}
}

func TestSafetyNoticePreservesEvidenceAndHistory(t *testing.T) {
	store, _ := New(t.TempDir())
	publish := func(version string) Version {
		content := []byte("package-" + version)
		digest := sha256.Sum256(content)
		item, err := store.Publish(PublishParams{OwnerID: "owner", Name: "sdk", Version: version, RepositoryID: "repo", ReleaseID: "release-" + version, SourceCommitID: "commit-" + version, ArtifactID: "artifact-" + version, ArtifactPath: "sdk.tgz", ArtifactSize: int64(len(content)), ExpectedSHA256: hex.EncodeToString(digest[:]), Build: BuildAttestation{RunID: "run-" + version, BuildName: "package"}, Platform: Platform{OS: "linux", Arch: "amd64"}, PublisherID: "owner", Visibility: "public"}, bytes.NewReader(content))
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	unsafe, replacement := publish("1.0.0"), publish("1.0.1")
	deprecated, err := store.SetSafety("repo", unsafe.ID, "deprecated", "upgrade promptly", replacement.ID, "owner")
	if err != nil || deprecated.SHA256 != unsafe.SHA256 || deprecated.SourceCommitID != unsafe.SourceCommitID || len(deprecated.SafetyHistory) != 1 {
		t.Fatalf("deprecated = %#v, %v", deprecated, err)
	}
	quarantined, err := store.SetSafety("repo", unsafe.ID, "quarantined", "confirmed compromise", replacement.ID, "owner")
	if err != nil || quarantined.Lifecycle != "quarantined" || len(quarantined.SafetyHistory) != 2 || quarantined.SafetyHistory[0].Reason != "upgrade promptly" {
		t.Fatalf("quarantined = %#v, %v", quarantined, err)
	}
	_, file, err := store.OpenArtifact("repo", unsafe.ID)
	if err != nil {
		t.Fatalf("historical artifact unavailable: %v", err)
	}
	file.Close()
}
