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
	other := t.TempDir()
	roots := []string{root, other}
	got, lib, err := ResolveTarget(roots, "")
	if err != nil || got != filepath.Clean(root) || lib != filepath.Clean(root) {
		t.Fatalf("empty path: %q %q %v", got, lib, err)
	}
	inside := filepath.Join(root, "shows")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	got, lib, err = ResolveTarget(roots, inside)
	if err != nil || got != inside || lib != filepath.Clean(root) {
		t.Fatalf("inside: %q %q %v", got, lib, err)
	}
	if _, _, err = ResolveTarget(roots, "shows"); err == nil {
		t.Fatal("expected relative path error")
	}
	outside := filepath.Join(root, "..", "outside")
	if _, _, err = ResolveTarget(roots, outside); err == nil {
		t.Fatal("expected path outside browse roots")
	}
	inOther := filepath.Join(other, "movies")
	if err := os.Mkdir(inOther, 0o755); err != nil {
		t.Fatal(err)
	}
	got, lib, err = ResolveTarget(roots, inOther)
	if err != nil || got != inOther || lib != filepath.Clean(other) {
		t.Fatalf("other root: %q %q %v", got, lib, err)
	}
}
