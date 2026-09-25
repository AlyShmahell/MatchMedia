package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithinFilesystemRoot(t *testing.T) {
	if !Within("/", "/mnt/microsd/media/anime") {
		t.Fatal("absolute path under /")
	}
	if !Within("/", "/") {
		t.Fatal("root is inside itself")
	}
	if Within("/", "relative") {
		t.Fatal("relative path is not inside filesystem root")
	}
}

func TestWithinPrefix(t *testing.T) {
	if !Within("/media", "/media") {
		t.Fatal("equal")
	}
	if !Within("/media", "/media/tv") {
		t.Fatal("child")
	}
	if Within("/media", "/mnt") {
		t.Fatal("sibling")
	}
	if Within("/media", "/media2") {
		t.Fatal("prefix-not-separator")
	}
}

func TestListRoots(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	if err := os.Mkdir(filepath.Join(a, "shows"), 0o755); err != nil {
		t.Fatal(err)
	}
	top, err := List([]string{a, b}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(top.Entries) != 2 || top.Parent != "" {
		t.Fatalf("top=%+v", top)
	}
	got, err := List([]string{a, b}, a)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != a || got.Parent != "." || len(got.Entries) != 1 {
		t.Fatalf("a=%+v", got)
	}
	if _, err = List([]string{a, b}, t.TempDir()); err == nil {
		t.Fatal("expected path outside roots")
	}
}
