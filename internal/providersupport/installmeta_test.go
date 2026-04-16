package providersupport

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstallMeta_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := InstallMeta{
		Method:      InstallMethodImage,
		Source:      "ghcr.io/foo/bar@sha256:abc",
		InstalledAt: time.Now().UTC().Truncate(time.Second),
		Version:     "0.2.0",
	}
	if err := WriteInstallMeta(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadInstallMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != want.Method || got.Source != want.Source ||
		got.Version != want.Version || !got.InstalledAt.Equal(want.InstalledAt) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestReadInstallMeta_AbsentReturnsGitDefault(t *testing.T) {
	dir := t.TempDir()
	got, err := ReadInstallMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Backward compat: existing installs (no .mgtt-install.json) are git-installed.
	if got.Method != InstallMethodGit {
		t.Fatalf("expected default method 'git' for legacy installs, got %q", got.Method)
	}
}

func TestReadInstallMeta_CorruptFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".mgtt-install.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadInstallMeta(dir)
	if err == nil {
		t.Fatal("expected error on corrupt metadata, got nil")
	}
}
