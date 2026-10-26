package match

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/clean"
	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
)

var folderYearRe = regexp.MustCompile(`\s*\((\d{4})\)\s*$`)

func failedQuery(j Job) bool {
	return j.Status == "unmatched" || j.Status == "error"
}

func isChild(j Job) bool {
	p := strings.TrimSpace(j.Parent)
	if p == "" {
		return false
	}
	if base := strings.TrimSpace(filepath.Base(j.Path)); base != "" && base != "." && sameFolder(base, p) {
		return false
	}
	return rank.TitleNorm(p) != rank.TitleNorm(j.Title)
}

func sameFolder(base, parent string) bool {
	if base == parent {
		return true
	}
	name := strings.TrimSpace(folderYearRe.ReplaceAllString(clean.Apply(nil, base), ""))
	return name == parent
}

func ChildFail(j Job) bool {
	return failedQuery(j) && isChild(j)
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
		if !isChild(j) {
			continue
		}
		p := strings.TrimSpace(j.Parent)
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

func foldUnmatched(cfg *config.Config, jobs []Job) []Job {
	children := map[string][]int{}
	for i, j := range jobs {
		if !isChild(j) {
			continue
		}
		key := parentGroupKey(strings.TrimSpace(j.Parent), parentDir(j.Path))
		children[key] = append(children[key], i)
	}
	drop := make([]bool, len(jobs))
	extraFiles := make([][]JobFile, len(jobs))
	for i, j := range jobs {
		if !parentSucceeded(j) {
			continue
		}
		idxs := children[parentGroupKey(j.Title, filepath.Clean(j.Path))]
		var absorb []int
		for _, ci := range idxs {
			if failedQuery(jobs[ci]) || seasonFolder(cfg, jobs[ci]) {
				absorb = append(absorb, ci)
			}
		}
		assigned := assignReleaseSeasons(cfg, j, jobs, absorb)
		var spans []fileSpan
		for _, ci := range absorb {
			drop[ci] = true
			files := append([]JobFile(nil), jobs[ci].Files...)
			if num, ok := assigned[ci]; ok {
				files = stampSeasonFiles(cfg, num, j.Catalog, files)
			} else {
				files = clearSE(files)
			}
			from := len(extraFiles[i])
			extraFiles[i] = append(extraFiles[i], files...)
			spans = append(spans, fileSpan{from: from, to: len(extraFiles[i])})
		}
		extraFiles[i] = clearDupSeasons(j.Files, extraFiles[i], spans)
	}
	out := make([]Job, 0, len(jobs))
	for i, j := range jobs {
		if drop[i] {
			continue
		}
		if len(extraFiles[i]) > 0 {
			j.Files = append(append([]JobFile(nil), j.Files...), extraFiles[i]...)
			j = MapFilesOntoCatalog(j)
		}
		out = append(out, j)
	}
	return out
}
