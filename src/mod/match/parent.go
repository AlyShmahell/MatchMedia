package match

import (
	"path/filepath"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
)

func failedQuery(j Job) bool {
	return j.Status == "unmatched" || j.Status == "error"
}

func parentSucceeded(j Job) bool {
	return j.Status == "matched" || j.Status == "manual"
}

func parentDir(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Clean(filepath.Dir(path))
}

func parentGroupKey(parent, dir string) string {
	return rank.TitleNorm(parent) + "\t" + dir
}

func parentJobs(jobs []Job) []Job {
	havePath := map[string]struct{}{}
	for _, j := range jobs {
		if p := filepath.Clean(j.Path); p != "" && p != "." {
			havePath[p] = struct{}{}
		}
	}
	type group struct {
		parent string
		path   string
		source string
		nfiles int
		show   bool
		se     bool
	}
	groups := map[string]*group{}
	var order []string
	for _, j := range jobs {
		p := strings.TrimSpace(j.Parent)
		if p == "" || rank.TitleNorm(p) == rank.TitleNorm(j.Title) {
			continue
		}
		dir := parentDir(j.Path)
		if dir == "" || dir == "." {
			continue
		}
		if _, ok := havePath[dir]; ok {
			continue
		}
		key := parentGroupKey(p, dir)
		g, ok := groups[key]
		if !ok {
			g = &group{parent: p, path: dir, source: j.Source}
			groups[key] = g
			order = append(order, key)
		}
		g.nfiles += len(j.Files)
		if j.Kind == "show" {
			g.show = true
		}
		for _, f := range j.Files {
			if strings.TrimSpace(f.Season) != "" || strings.TrimSpace(f.Episode) != "" {
				g.se = true
			}
		}
	}
	out := make([]Job, 0, len(order))
	for _, key := range order {
		g := groups[key]
		kind := "movie"
		if g.show || g.se || g.nfiles > 1 {
			kind = "show"
		}
		out = append(out, Job{
			Source: g.source,
			Title:  g.parent,
			Parent: g.parent,
			Path:   g.path,
			Kind:   kind,
			Role:   "title",
			Status: "pending",
		})
	}
	return out
}

func foldUnmatched(jobs []Job) []Job {
	children := map[string][]int{}
	for i, j := range jobs {
		p := strings.TrimSpace(j.Parent)
		if p == "" || rank.TitleNorm(p) == rank.TitleNorm(j.Title) {
			continue
		}
		key := parentGroupKey(p, parentDir(j.Path))
		children[key] = append(children[key], i)
	}
	drop := make([]bool, len(jobs))
	extraFiles := make([][]JobFile, len(jobs))
	for i, j := range jobs {
		if !parentSucceeded(j) {
			continue
		}
		idxs := children[parentGroupKey(j.Title, filepath.Clean(j.Path))]
		for _, ci := range idxs {
			if !failedQuery(jobs[ci]) {
				continue
			}
			drop[ci] = true
			extraFiles[i] = append(extraFiles[i], jobs[ci].Files...)
		}
	}
	out := make([]Job, 0, len(jobs))
	for i, j := range jobs {
		if drop[i] {
			continue
		}
		if len(extraFiles[i]) > 0 {
			j.Files = append(append([]JobFile(nil), j.Files...), extraFiles[i]...)
		}
		out = append(out, j)
	}
	return out
}
