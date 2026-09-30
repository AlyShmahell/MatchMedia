package scan

import (
	"fmt"
	"path/filepath"
	"strings"

	matchfs "github.com/alyshmahell/matchmedia/src/internal/fs"
)

const (
	ModeRescan  = "rescan"
	ModeChanges = "changes"
	ModeNFO     = "nfo"
)

func NormalizeMode(mode string) (string, error) {
	switch mode {
	case "", ModeRescan:
		return ModeRescan, nil
	case ModeChanges:
		return ModeChanges, nil
	case ModeNFO:
		return ModeNFO, nil
	default:
		return "", fmt.Errorf("unknown mode")
	}
}

func RequireEpisodeNFO(mode string, flag *bool) bool {
	switch mode {
	case ModeNFO:
		if flag == nil {
			return false
		}
		return *flag
	case ModeChanges:
		if flag == nil {
			return true
		}
		return *flag
	default:
		return false
	}
}

func ResolveTarget(roots []string, path string) (string, string, error) {
	roots = matchfsClean(roots)
	if len(roots) == 0 {
		return "", "", fmt.Errorf("browse root is empty")
	}
	if strings.TrimSpace(path) == "" {
		return roots[0], roots[0], nil
	}
	if !filepath.IsAbs(path) {
		return "", "", fmt.Errorf("path must be absolute")
	}
	path = filepath.Clean(path)
	root, ok := matchfs.Containing(roots, path)
	if !ok {
		return "", "", fmt.Errorf("path outside browse roots")
	}
	return path, root, nil
}

func matchfsClean(roots []string) []string {
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
