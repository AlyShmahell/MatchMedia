package scan

import (
	"fmt"
	"path/filepath"

	matchfs "github.com/alyshmahell/matchmedia/lib/fs"
)

const (
	ModeRescan  = "rescan"
	ModeChanges = "changes"
)

func NormalizeMode(mode string) (string, error) {
	switch mode {
	case "", ModeRescan:
		return ModeRescan, nil
	case ModeChanges:
		return ModeChanges, nil
	default:
		return "", fmt.Errorf("unknown mode")
	}
}

func RequireEpisodeNFO(mode string, flag *bool) bool {
	if mode != ModeChanges {
		return true
	}
	if flag == nil {
		return true
	}
	return *flag
}

func ResolveTarget(root, path string) (string, error) {
	root = filepath.Clean(root)
	if path == "" {
		path = root
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be absolute")
	}
	path = filepath.Clean(path)
	if !matchfs.Within(root, path) {
		return "", fmt.Errorf("path outside browse_root")
	}
	return path, nil
}
