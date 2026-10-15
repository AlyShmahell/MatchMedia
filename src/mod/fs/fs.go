package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

func Rel(root, path string) (string, error) {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if !Within(root, path) {
		return "", fmt.Errorf("path %q is outside %s", path, root)
	}
	if path == root {
		return "", nil
	}
	return filepath.Rel(root, path)
}

func ReadDir(dir string) ([]os.DirEntry, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	return ents, nil
}

func IsVideo(exts map[string]struct{}, name string) bool {
	_, ok := exts[strings.ToLower(filepath.Ext(name))]
	return ok
}

func Stem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func WalkVideos(dir string, exts map[string]struct{}) ([]string, error) {
	var out []string
	var walk func(string) error
	walk = func(current string) error {
		ents, err := os.ReadDir(current)
		if err != nil {
			return err
		}
		var dirs, files []string
		for _, e := range ents {
			if e.IsDir() {
				dirs = append(dirs, e.Name())
				continue
			}
			if IsVideo(exts, e.Name()) {
				files = append(files, e.Name())
			}
		}
		for _, name := range files {
			out = append(out, filepath.Join(current, name))
		}
		for _, name := range dirs {
			if err := walk(filepath.Join(current, name)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(dir); err != nil {
		return nil, err
	}
	return out, nil
}
