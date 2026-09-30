package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Entry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
}

type Listing struct {
	Path    string  `json:"path"`
	Parent  string  `json:"parent,omitempty"`
	Root    string  `json:"root"`
	Entries []Entry `json:"entries"`
}

func List(roots []string, rel string) (Listing, error) {
	roots = cleanRoots(roots)
	if len(roots) == 0 {
		return Listing{}, fmt.Errorf("browse root is empty")
	}
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." {
		return rootsListing(roots), nil
	}
	if !filepath.IsAbs(rel) {
		return Listing{}, fmt.Errorf("path %q is outside browse root", rel)
	}
	target := filepath.Clean(rel)
	root, ok := Containing(roots, target)
	if !ok {
		return Listing{}, fmt.Errorf("path %q is outside browse root", rel)
	}
	info, err := os.Stat(target)
	if err != nil {
		return Listing{}, err
	}
	if !info.IsDir() {
		return Listing{}, fmt.Errorf("not a directory: %s", target)
	}
	ents, err := os.ReadDir(target)
	if err != nil {
		return Listing{}, err
	}
	out := Listing{Path: target, Root: root, Entries: []Entry{}}
	if target == root {
		out.Parent = "."
	} else {
		out.Parent = filepath.Dir(target)
	}
	for _, e := range ents {
		p := filepath.Join(target, e.Name())
		if !Within(root, p) {
			continue
		}
		st, err := os.Stat(p)
		if err != nil || !st.IsDir() {
			continue
		}
		out.Entries = append(out.Entries, Entry{Name: e.Name(), Path: p, Dir: true})
	}
	return out, nil
}

func rootsListing(roots []string) Listing {
	out := Listing{Entries: []Entry{}}
	for _, root := range roots {
		name := filepath.Base(root)
		if name == "" || name == string(os.PathSeparator) {
			name = root
		}
		out.Entries = append(out.Entries, Entry{Name: name, Path: root, Dir: true})
	}
	return out
}

func cleanRoots(roots []string) []string {
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "" || root == "." {
			continue
		}
		out = append(out, root)
	}
	return out
}

func Containing(roots []string, path string) (string, bool) {
	path = filepath.Clean(path)
	best := ""
	for _, root := range cleanRoots(roots) {
		if !Within(root, path) {
			continue
		}
		if len(root) > len(best) {
			best = root
		}
	}
	return best, best != ""
}

func Rel(root, path string) (string, error) {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if !Within(root, path) {
		return "", fmt.Errorf("path %q is outside browse root", path)
	}
	if path == root {
		return "", nil
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	return rel, nil
}

func Within(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if root == string(os.PathSeparator) {
		return path == root || filepath.IsAbs(path)
	}
	if path == root {
		return true
	}
	prefix := root + string(os.PathSeparator)
	return strings.HasPrefix(path, prefix)
}
