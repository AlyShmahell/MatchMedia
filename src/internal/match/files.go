package match

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	fileSxxERe     = regexp.MustCompile(`(?i)s(\d{1,2})e(\d{1,3})`)
	fileEpisodeRe  = regexp.MustCompile(`(?i)\bepisode\s*(\d+)`)
	folderSeasonRe = regexp.MustCompile(`(?i)season\s*(\d+)`)
	folderSxxNumRe = regexp.MustCompile(`(?i)^s(\d{1,2})$`)
)

func (g *grouper) attachFiles(rows []groupedRow) []groupedRow {
	if len(rows) == 0 {
		return rows
	}
	for i := range rows {
		rows[i].files = g.numberFiles(rows[i].path, g.ownedVideos(rows[i], rows))
	}
	return rows
}

func (g *grouper) ownedVideos(row groupedRow, rows []groupedRow) []string {
	abs := g.absFromRel(row.path)
	if abs == "" {
		return nil
	}
	var out []string
	for _, v := range g.videosFor(abs) {
		rel := g.libRel(v)
		skip := false
		for _, other := range rows {
			if other.path == "" || other.path == row.path {
				continue
			}
			if pathOwns(rel, other.path) && len(other.path) > len(row.path) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, v)
		}
	}
	return out
}

func pathOwns(fileRel, jobRel string) bool {
	fileRel = strings.TrimSuffix(filepath.ToSlash(fileRel), "/")
	jobRel = strings.TrimSuffix(filepath.ToSlash(jobRel), "/")
	if jobRel == "" {
		return false
	}
	return fileRel == jobRel || strings.HasPrefix(fileRel, jobRel+"/")
}

func (g *grouper) absFromRel(rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return ""
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(g.library, filepath.FromSlash(rel))
}

func (g *grouper) numberFiles(jobRel string, videos []string) []JobFile {
	type item struct {
		path, season, episode string
	}
	items := make([]item, 0, len(videos))
	for _, v := range videos {
		rel := g.libRel(v)
		s, e := g.inferSE(jobRel, rel)
		items = append(items, item{path: rel, season: s, episode: e})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].path < items[j].path })
	used := map[string]map[string]bool{}
	for _, it := range items {
		if it.season == "" || it.episode == "" {
			continue
		}
		if used[it.season] == nil {
			used[it.season] = map[string]bool{}
		}
		used[it.season][it.episode] = true
	}
	next := map[string]int{}
	out := make([]JobFile, 0, len(items))
	for _, it := range items {
		s, e := it.season, it.episode
		if s != "" && e == "" {
			if used[s] == nil {
				used[s] = map[string]bool{}
			}
			n := next[s]
			if n < 1 {
				n = 1
			}
			for used[s][strconv.Itoa(n)] {
				n++
			}
			e = strconv.Itoa(n)
			used[s][e] = true
			next[s] = n + 1
		}
		out = append(out, JobFile{Path: it.path, Season: s, Episode: e})
	}
	return out
}

func (g *grouper) inferSE(jobRel, videoRel string) (season, episode string) {
	rel := filepath.ToSlash(videoRel)
	jobRel = filepath.ToSlash(jobRel)
	if jobRel != "" {
		if rel == jobRel {
			rel = filepath.Base(rel)
		} else if strings.HasPrefix(rel, jobRel+"/") {
			rel = strings.TrimPrefix(rel, jobRel+"/")
		}
	}
	if rel == "" || rel == "." {
		rel = filepath.Base(videoRel)
	}
	parts := strings.Split(rel, "/")
	file := parts[len(parts)-1]
	dirs := parts[:len(parts)-1]
	root := filepath.Base(jobRel)
	for _, d := range dirs {
		switch g.cls.classify(d, true, root) {
		case "kind", "extras":
			season = "0"
		case "season":
			if n, ok := parseFolderSeason(d); ok {
				season = n
			}
		}
	}
	if s, e, ok := parseFilenameSE(file); ok {
		if s != "" {
			season = s
		}
		if e != "" {
			episode = e
		}
	}
	return season, episode
}

func parseFolderSeason(name string) (string, bool) {
	name = strings.TrimSpace(strings.TrimRight(name, "/"))
	if m := folderSeasonRe.FindStringSubmatch(name); len(m) == 2 {
		return canonNum(m[1]), true
	}
	if m := folderSxxNumRe.FindStringSubmatch(name); len(m) == 2 {
		return canonNum(m[1]), true
	}
	return "", false
}

func parseFilenameSE(name string) (season, episode string, ok bool) {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if m := fileSxxERe.FindStringSubmatch(base); len(m) == 3 {
		return canonNum(m[1]), canonNum(m[2]), true
	}
	if m := fileEpisodeRe.FindStringSubmatch(base); len(m) == 2 {
		return "", canonNum(m[1]), true
	}
	return "", "", false
}

func canonNum(s string) string {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return strings.TrimSpace(s)
	}
	return strconv.Itoa(n)
}
