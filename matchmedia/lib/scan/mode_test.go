package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeMode(t *testing.T) {
	got, err := NormalizeMode("")
	if err != nil || got != ModeRescan {
		t.Fatalf("empty: %q %v", got, err)
	}
	got, err = NormalizeMode(ModeRescan)
	if err != nil || got != ModeRescan {
		t.Fatalf("rescan: %q %v", got, err)
	}
	got, err = NormalizeMode(ModeChanges)
	if err != nil || got != ModeChanges {
		t.Fatalf("changes: %q %v", got, err)
	}
	if _, err = NormalizeMode("delta"); err == nil {
		t.Fatal("expected unknown mode")
	}
}

func TestRequireEpisodeNFO(t *testing.T) {
	if !RequireEpisodeNFO(ModeChanges, nil) {
		t.Fatal("changes omit defaults true")
	}
	yes, no := true, false
	if !RequireEpisodeNFO(ModeChanges, &yes) {
		t.Fatal("changes true")
	}
	if RequireEpisodeNFO(ModeChanges, &no) {
		t.Fatal("changes false")
	}
	if !RequireEpisodeNFO(ModeRescan, &no) {
		t.Fatal("rescan ignores flag")
	}
}

func TestResolveTarget(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveTarget(root, "")
	if err != nil || got != filepath.Clean(root) {
		t.Fatalf("empty path: %q %v", got, err)
	}
	inside := filepath.Join(root, "shows")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveTarget(root, inside)
	if err != nil || got != inside {
		t.Fatalf("inside: %q %v", got, err)
	}
	if _, err = ResolveTarget(root, "shows"); err == nil {
		t.Fatal("expected relative path error")
	}
	outside := filepath.Join(root, "..", "outside")
	if _, err = ResolveTarget(root, outside); err == nil {
		t.Fatal("expected path outside browse_root")
	}
}
