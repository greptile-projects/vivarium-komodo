package workspaces

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPolicyExpiryStopRetainsEvidence(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := DefaultPolicy()
	p.AgentExecution = false
	p.Sharing = "private"
	p.MemoryMB = 512
	if _, err = s.SetPolicy("repository", "repo", p); err != nil {
		t.Fatal(err)
	}
	w, err := s.Create("repo", "012345", "creator", SourceContext{Type: "repository"}, Access{}, Definition{Resources: ResourceLimits{CPUSeconds: 900, MemoryMB: 4096, DiskMB: 8192}}, "definition")
	if err != nil {
		t.Fatal(err)
	}
	if w.Policy.MemoryMB != 512 || w.Definition.Resources.MemoryMB != 512 {
		t.Fatalf("policy not captured/capped: %#v", w)
	}
	if _, err = s.Finish(w.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Grant("repo", w.ID, "creator", "peer", "human", "edit", []string{"files"}); err != ErrConflict {
		t.Fatalf("private sharing grant error = %v", err)
	}
	s.now = func() time.Time { return time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC) }
	if _, err = s.AnnounceExpiry("repo", w.ID, "owner", s.now().Add(23*time.Hour)); err != ErrConflict {
		t.Fatalf("short notice = %v", err)
	}
	w, err = s.AnnounceExpiry("repo", w.ID, "owner", s.now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	w.Checkpoints = []Checkpoint{{ID: "retained", Publication: &Publication{CommitID: "published"}}}
	if err = s.write(w); err != nil {
		t.Fatal(err)
	}
	w, err = s.Stop("repo", w.ID, "owner", "obsolete definition", true)
	if err != nil {
		t.Fatal(err)
	}
	if w.State != Expired || len(w.Checkpoints) != 1 || w.Checkpoints[0].Publication.CommitID != "published" {
		t.Fatalf("evidence lost: %#v", w)
	}
}

func TestOrganizationPolicyInheritanceAndAutomaticLifecycle(t *testing.T) {
	s, _ := New(t.TempDir())
	p := DefaultPolicy()
	p.MemoryMB = 768
	p.IdleMinutes = 5
	p.RetentionDays = 1
	p.ExpiryNoticeHours = 2
	if _, err := s.SetPolicy("organization", "org", p); err != nil {
		t.Fatal(err)
	}
	effective, _ := s.EffectivePolicy("repo", "org")
	if effective.MemoryMB != 768 {
		t.Fatalf("organization policy not inherited: %#v", effective)
	}
	w, _ := s.CreateWithPolicy("repo", "revision", "creator", SourceContext{Type: "repository"}, Access{}, Definition{Resources: ResourceLimits{CPUSeconds: 10, MemoryMB: 1024, DiskMB: 1024}}, "definition", effective)
	w, _ = s.Finish(w.ID, true, "")
	s.now = func() time.Time { return w.UpdatedAt.Add(6 * time.Minute) }
	got, err := s.Get("repo", w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != Suspended {
		t.Fatalf("idle workspace state = %s", got.State)
	}
}

func TestExportIncludesOnlySafeChangedFiles(t *testing.T) {
	s, _ := New(t.TempDir())
	r := &Runner{store: s}
	w, _ := s.Create("repo", "revision", "creator", SourceContext{Type: "repository"}, Access{}, Definition{Resources: ResourceLimits{CPUSeconds: 1, MemoryMB: 128, DiskMB: 128}}, "definition")
	w, _ = s.Finish(w.ID, true, "")
	root := s.Environment(w.ID)
	if err := os.MkdirAll(root, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "safe.txt"), []byte("work"), 0640); err != nil {
		t.Fatal(err)
	}
	w, _ = s.RecordChange("repo", w.ID, "creator", "safe.txt", "digest", false)
	data, err := r.Export(w)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 1 || zr.File[0].Name != "safe.txt" {
		t.Fatalf("unexpected export: %#v", zr.File)
	}
	f, _ := zr.File[0].Open()
	got, _ := io.ReadAll(f)
	if string(got) != "work" {
		t.Fatalf("got %q", got)
	}
}
