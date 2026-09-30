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
	trailEpRe      = regexp.MustCompile(`(?i)(?:^|[-_\s]+)(\d{1,3})\s*$`)
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

func (g *grouper) finish(rows []groupedRow) []groupedRow {
	out := make([]groupedRow, 0, len(rows))
	for _, row := range rows {
		files := g.dropExtraFiles(row.title, row.files)
		files = dropBareDuplicates(files)
		if len(files) == 0 {
			continue
		}
		if g.showRow(row.path, files) {
			row.kind = "show"
			row.files = numberShowFiles(files)
		} else {
			row.kind = "movie"
			row.files = clearSeasonEpisode(files)
		}
		row.role = "title"
		out = append(out, row)
	}
	return out
}

func (g *grouper) dropExtraFiles(title string, files []JobFile) []JobFile {
	out := make([]JobFile, 0, len(files))
	for _, f := range files {
		if g.inExtrasDir(f.Path) || g.pre.extrasFile(filepath.Base(f.Path), title) {
			continue
		}
		out = append(out, f)
	}
	return out
}

func (g *grouper) inExtrasDir(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) < 2 {
		return false
	}
	for _, d := range parts[:len(parts)-1] {
		if g.pre.extrasName(d) {
			return true
		}
	}
	return false
}

func (g *grouper) showRow(jobRel string, files []JobFile) bool {
	if trailingPack(files) {
		return true
	}
	for _, f := range files {
		if _, _, ok := parseFilenameSE(filepath.Base(f.Path)); ok {
			return true
		}
		if g.pathIsShow(jobRel, f.Path) {
			return true
		}
	}
	return false
}

func (g *grouper) pathIsShow(jobRel, videoRel string) bool {
	rel := filepath.ToSlash(videoRel)
	jobRel = filepath.ToSlash(jobRel)
	if jobRel != "" {
		if rel == jobRel {
			return false
		}
		if strings.HasPrefix(rel, jobRel+"/") {
			rel = strings.TrimPrefix(rel, jobRel+"/")
		}
	}
	parts := strings.Split(rel, "/")
	if len(parts) < 2 {
		return false
	}
	root := filepath.Base(jobRel)
	for _, d := range parts[:len(parts)-1] {
		switch g.cls.classify(d, true, root) {
		case "kind", "extras", "season":
			return true
		}
	}
	return false
}

func numberShowFiles(files []JobFile) []JobFile {
	for i := range files {
		if files[i].Season == "" {
			files[i].Season = "1"
		}
	}
	return fillEpisodes(files)
}

func clearSeasonEpisode(files []JobFile) []JobFile {
	out := make([]JobFile, len(files))
	for i, f := range files {
		f.Season = ""
		f.Episode = ""
		out[i] = f
	}
	return out
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
	out := make([]JobFile, 0, len(items))
	for _, it := range items {
		out = append(out, JobFile{Path: it.path, Season: it.season, Episode: it.episode})
	}
	return fillEpisodes(g.assignCourSeasons(jobRel, out))
}

func (g *grouper) assignCourSeasons(jobRel string, files []JobFile) []JobFile {
	names := map[string]struct{}{}
	var order []string
	cour := make([]string, len(files))
	for i, f := range files {
		if f.Season != "" {
			continue
		}
		name, ok := g.courName(jobRel, f.Path)
		if !ok {
			continue
		}
		cour[i] = name
		if _, seen := names[name]; seen {
			continue
		}
		names[name] = struct{}{}
		order = append(order, name)
	}
	sort.Strings(order)
	rank := map[string]string{}
	for i, name := range order {
		rank[name] = strconv.Itoa(i + 1)
	}
	for i := range files {
		if cour[i] == "" {
			continue
		}
		files[i].Season = rank[cour[i]]
	}
	return files
}

func (g *grouper) courName(jobRel, videoRel string) (string, bool) {
	rel := filepath.ToSlash(videoRel)
	jobRel = filepath.ToSlash(jobRel)
	if jobRel != "" && strings.HasPrefix(rel, jobRel+"/") {
		rel = strings.TrimPrefix(rel, jobRel+"/")
	}
	parts := strings.Split(rel, "/")
	if len(parts) < 2 {
		return "", false
	}
	root := filepath.Base(jobRel)
	for _, d := range parts[:len(parts)-1] {
		switch g.cls.classify(d, true, root) {
		case "kind", "extras", "season":
			continue
		default:
			return d, true
		}
	}
	return "", false
}

func fillEpisodes(files []JobFile) []JobFile {
	used := map[string]map[string]bool{}
	for _, f := range files {
		if f.Season == "" || f.Episode == "" {
			continue
		}
		if used[f.Season] == nil {
			used[f.Season] = map[string]bool{}
		}
		used[f.Season][f.Episode] = true
	}
	next := map[string]int{}
	for i, f := range files {
		if f.Season == "" || f.Episode != "" {
			continue
		}
		if used[f.Season] == nil {
			used[f.Season] = map[string]bool{}
		}
		n := next[f.Season]
		if n < 1 {
			n = 1
		}
		for used[f.Season][strconv.Itoa(n)] {
			n++
		}
		f.Episode = strconv.Itoa(n)
		used[f.Season][f.Episode] = true
		next[f.Season] = n + 1
		files[i] = f
	}
	return files
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
	inCour := false
	for _, d := range dirs {
		switch g.cls.classify(d, true, root) {
		case "kind", "extras":
			season = "0"
		case "season":
			if n, ok := parseFolderSeason(d); ok {
				season = n
			}
		default:
			inCour = true
		}
	}
	if s, e, ok := parseFilenameSE(file); ok {
		if s != "" && !inCour {
			season = s
		}
		if e != "" {
			episode = e
		}
		return season, episode
	}
	if e, ok := trailingEpisode(file); ok {
		episode = e
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

func trailingEpisode(name string) (string, bool) {
	if _, _, ok := parseFilenameSE(name); ok {
		return "", false
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	base = bracketsRe.ReplaceAllString(base, " ")
	base = parenRe.ReplaceAllString(base, " ")
	base = strings.Trim(base, " _-")
	m := trailEpRe.FindStringSubmatch(base)
	if m == nil {
		return "", false
	}
	return canonNum(m[1]), true
}

func trailingPack(files []JobFile) bool {
	n := 0
	for _, f := range files {
		if _, ok := trailingEpisode(filepath.Base(f.Path)); ok {
			n++
		}
	}
	return n >= 2
}

func dashPackCount(entries []entry) int {
	n := 0
	for _, e := range entries {
		if e.dir {
			continue
		}
		if _, ok := trailingEpisode(e.name); ok {
			n++
		}
	}
	return n
}

func dropBareDuplicates(files []JobFile) []JobFile {
	byDir := map[string][]int{}
	for i, f := range files {
		byDir[filepath.Dir(f.Path)] = append(byDir[filepath.Dir(f.Path)], i)
	}
	drop := map[int]bool{}
	for _, idxs := range byDir {
		explicit := false
		for _, i := range idxs {
			if _, _, ok := parseFilenameSE(filepath.Base(files[i].Path)); ok {
				explicit = true
				break
			}
		}
		if !explicit {
			continue
		}
		for _, i := range idxs {
			if _, ok := trailingEpisode(filepath.Base(files[i].Path)); ok {
				drop[i] = true
			}
		}
	}
	if len(drop) == 0 {
		return files
	}
	out := make([]JobFile, 0, len(files))
	for i, f := range files {
		if drop[i] {
			continue
		}
		out = append(out, f)
	}
	return out
}
