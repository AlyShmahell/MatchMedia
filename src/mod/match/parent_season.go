package match

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/clean"
	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/fetch"
)

var (
	seasonTokenRe = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(?:eason)?[\s._-]*0*(\d{1,2})(?:[^a-z0-9]|$)`)
	mediaKindRe   = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(movies?|ovas?|specials?)(?:[^a-z0-9]|$)`)
	fileSERe      = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{1,2})e(\d{1,3})`)
	endNumRe      = regexp.MustCompile(`(?i)(?:^|\s)-\s*(\d{1,3})\s*$`)
	midNumRe      = regexp.MustCompile(`(?i)(?:^|\s)(\d{1,3})\s+-\s+`)
)

type fileSpan struct {
	from, to int
}

type parsedEP struct {
	season  string
	episode string
	fromSE  bool
}

func assignReleaseSeasons(cfg *config.Config, parent Job, jobs []Job, failed []int) map[int]string {
	seasons := seasonsWithEpisodes(parent.Catalog)
	if len(seasons) == 0 || len(failed) == 0 {
		return nil
	}
	type pick struct {
		idx    int
		season string
	}
	var picks []pick
	for _, ci := range failed {
		base := clean.Apply(cfg, filepath.Base(jobs[ci].Path))
		nfiles := len(jobs[ci].Files)
		if nfiles == 0 {
			continue
		}
		if num, ok := seasonToken(base); ok {
			if s, ok := tokenFit(seasons, num, nfiles); ok {
				picks = append(picks, pick{ci, s})
			}
			continue
		}
		if mediaKindRe.MatchString(base) {
			if s, ok := exactSpecial(seasons, nfiles); ok {
				picks = append(picks, pick{ci, s})
			}
			continue
		}
		if s, ok := exactUnique(seasons, nfiles); ok {
			picks = append(picks, pick{ci, s})
		}
	}
	count := map[string]int{}
	for _, p := range picks {
		count[p.season]++
	}
	out := map[int]string{}
	for _, p := range picks {
		if count[p.season] == 1 {
			out[p.idx] = p.season
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func seasonsWithEpisodes(catalog []CatalogSeason) []CatalogSeason {
	var out []CatalogSeason
	for _, s := range catalog {
		if len(s.Episodes) > 0 {
			out = append(out, s)
		}
	}
	return out
}

func seasonFolder(cfg *config.Config, j Job) bool {
	_, ok := seasonToken(clean.Apply(cfg, filepath.Base(j.Path)))
	return ok
}

func seasonToken(name string) (string, bool) {
	m := seasonTokenRe.FindStringSubmatch(name)
	if len(m) != 2 {
		return "", false
	}
	return normNum(m[1]), true
}

func tokenFit(seasons []CatalogSeason, num string, nfiles int) (string, bool) {
	var hit *CatalogSeason
	for i := range seasons {
		if isSpecial(seasons[i]) {
			continue
		}
		if fetch.CatalogNumKey(seasons[i].Number) != num {
			continue
		}
		if hit != nil {
			return "", false
		}
		hit = &seasons[i]
	}
	if hit == nil || !nearCount(nfiles, len(hit.Episodes)) {
		return "", false
	}
	return fetch.CatalogNumKey(hit.Number), true
}

func exactSpecial(seasons []CatalogSeason, nfiles int) (string, bool) {
	s, ok := exactUnique(seasons, nfiles)
	if !ok {
		return "", false
	}
	for i := range seasons {
		if fetch.CatalogNumKey(seasons[i].Number) == s && isSpecial(seasons[i]) {
			return s, true
		}
	}
	return "", false
}

func exactUnique(seasons []CatalogSeason, nfiles int) (string, bool) {
	var hit *CatalogSeason
	for i := range seasons {
		if len(seasons[i].Episodes) != nfiles {
			continue
		}
		if hit != nil {
			return "", false
		}
		hit = &seasons[i]
	}
	if hit == nil {
		return "", false
	}
	return fetch.CatalogNumKey(hit.Number), true
}

func isSpecial(s CatalogSeason) bool {
	if fetch.CatalogNumKey(s.Number) == "0" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(s.Title), "specials")
}

func nearCount(files, episodes int) bool {
	if episodes <= 0 || files <= 0 {
		return false
	}
	slack := episodes / 5
	if slack < 1 {
		slack = 1
	}
	d := files - episodes
	if d < 0 {
		d = -d
	}
	return d <= slack
}

func stampSeasonFiles(cfg *config.Config, seasonNum string, catalog []CatalogSeason, files []JobFile) []JobFile {
	out := append([]JobFile(nil), files...)
	season := findSeason(catalog, seasonNum)
	n := len(season.Episodes)
	if n == 0 {
		return clearSE(out)
	}
	parsed := make([]parsedEP, len(out))
	for i, f := range out {
		parsed[i] = parseEpisode(clean.Apply(cfg, trimExt(filepath.Base(f.Path))))
	}
	if mixedFilenameSeasons(parsed) {
		return clearSE(out)
	}
	if sameSESeason(parsed) && episodesFit(parsed, n) {
		return applyEpisodeNums(out, seasonNum, episodeVals(parsed))
	}
	if allIndexed(parsed) && allInRange(parsed, n) && uniqueEpisodes(parsed) {
		return applyEpisodeNums(out, seasonNum, episodeVals(parsed))
	}
	if anyInRange(parsed, n) {
		for i := range out {
			out[i].Season = ""
			out[i].Episode = ""
			v, ok := episodeVal(parsed[i])
			if ok && v >= 1 && v <= n {
				out[i].Season = seasonNum
				out[i].Episode = strconv.Itoa(v)
			}
		}
		return out
	}
	if allIndexed(parsed) && allAbove(parsed, n) {
		if shifted, ok := shiftEpisodes(catalog, seasonNum, parsed, n); ok {
			return applyEpisodeNums(out, seasonNum, shifted)
		}
	}
	if len(out) == n && noneNumbered(parsed) {
		return zipEpisodes(out, seasonNum, season.Episodes)
	}
	return clearSE(out)
}

func findSeason(catalog []CatalogSeason, num string) CatalogSeason {
	for _, s := range catalog {
		if fetch.CatalogNumKey(s.Number) == num {
			return s
		}
	}
	return CatalogSeason{}
}

func parseEpisode(name string) parsedEP {
	if m := fileSERe.FindStringSubmatch(name); len(m) == 3 {
		return parsedEP{season: normNum(m[1]), episode: normNum(m[2]), fromSE: true}
	}
	if m := endNumRe.FindStringSubmatch(name); len(m) == 2 {
		return parsedEP{episode: normNum(m[1])}
	}
	if m := midNumRe.FindStringSubmatch(name); len(m) == 2 {
		return parsedEP{episode: normNum(m[1])}
	}
	return parsedEP{}
}

func mixedFilenameSeasons(parsed []parsedEP) bool {
	seen := map[string]struct{}{}
	count := 0
	for _, p := range parsed {
		if !p.fromSE {
			return false
		}
		count++
		seen[p.season] = struct{}{}
	}
	return count > 0 && len(seen) > 1
}

func sameSESeason(parsed []parsedEP) bool {
	if len(parsed) == 0 {
		return false
	}
	season := ""
	for _, p := range parsed {
		if !p.fromSE {
			return false
		}
		if season == "" {
			season = p.season
		}
		if p.season != season {
			return false
		}
	}
	return true
}

func episodesFit(parsed []parsedEP, n int) bool {
	return allIndexed(parsed) && allInRange(parsed, n) && uniqueEpisodes(parsed)
}

func allIndexed(parsed []parsedEP) bool {
	if len(parsed) == 0 {
		return false
	}
	for _, p := range parsed {
		if _, ok := episodeVal(p); !ok {
			return false
		}
	}
	return true
}

func allInRange(parsed []parsedEP, n int) bool {
	for _, p := range parsed {
		v, ok := episodeVal(p)
		if !ok || v < 1 || v > n {
			return false
		}
	}
	return true
}

func anyInRange(parsed []parsedEP, n int) bool {
	for _, p := range parsed {
		v, ok := episodeVal(p)
		if ok && v >= 1 && v <= n {
			return true
		}
	}
	return false
}

func allAbove(parsed []parsedEP, n int) bool {
	for _, p := range parsed {
		v, ok := episodeVal(p)
		if !ok || v <= n {
			return false
		}
	}
	return true
}

func uniqueEpisodes(parsed []parsedEP) bool {
	seen := map[int]struct{}{}
	for _, p := range parsed {
		v, ok := episodeVal(p)
		if !ok {
			return false
		}
		if _, dup := seen[v]; dup {
			return false
		}
		seen[v] = struct{}{}
	}
	return true
}

func noneNumbered(parsed []parsedEP) bool {
	for _, p := range parsed {
		if _, ok := episodeVal(p); ok {
			return false
		}
	}
	return true
}

func episodeVals(parsed []parsedEP) []int {
	out := make([]int, len(parsed))
	for i, p := range parsed {
		v, _ := episodeVal(p)
		out[i] = v
	}
	return out
}

func episodeVal(p parsedEP) (int, bool) {
	if p.episode == "" {
		return 0, false
	}
	n, err := strconv.Atoi(p.episode)
	if err != nil {
		return 0, false
	}
	return n, true
}

func shiftEpisodes(catalog []CatalogSeason, seasonNum string, parsed []parsedEP, n int) ([]int, bool) {
	off := seasonOffset(catalog, seasonNum)
	out := make([]int, len(parsed))
	seen := map[int]struct{}{}
	for i, p := range parsed {
		v, ok := episodeVal(p)
		if !ok {
			return nil, false
		}
		v -= off
		if v < 1 || v > n {
			return nil, false
		}
		if _, dup := seen[v]; dup {
			return nil, false
		}
		seen[v] = struct{}{}
		out[i] = v
	}
	return out, true
}

func seasonOffset(catalog []CatalogSeason, seasonNum string) int {
	target, err := strconv.Atoi(seasonNum)
	if err != nil {
		return 0
	}
	sum := 0
	for _, s := range catalog {
		n, err := strconv.Atoi(strings.TrimSpace(s.Number))
		if err != nil || n < 1 || n >= target {
			continue
		}
		sum += len(s.Episodes)
	}
	return sum
}

func applyEpisodeNums(files []JobFile, seasonNum string, nums []int) []JobFile {
	out := append([]JobFile(nil), files...)
	for i := range out {
		out[i].Season = seasonNum
		out[i].Episode = strconv.Itoa(nums[i])
	}
	return out
}

func zipEpisodes(files []JobFile, seasonNum string, episodes []fetch.Episode) []JobFile {
	out := append([]JobFile(nil), files...)
	order := make([]int, len(out))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return filepath.Base(out[order[a]].Path) < filepath.Base(out[order[b]].Path)
	})
	eps := append([]fetch.Episode(nil), episodes...)
	sort.SliceStable(eps, func(a, b int) bool {
		na, ea := strconv.Atoi(strings.TrimSpace(eps[a].Number))
		nb, eb := strconv.Atoi(strings.TrimSpace(eps[b].Number))
		if ea == nil && eb == nil {
			return na < nb
		}
		return eps[a].Number < eps[b].Number
	})
	for i, idx := range order {
		out[idx].Season = seasonNum
		out[idx].Episode = fetch.CatalogNumKey(eps[i].Number)
	}
	return out
}

func clearSE(files []JobFile) []JobFile {
	out := append([]JobFile(nil), files...)
	for i := range out {
		out[i].Season = ""
		out[i].Episode = ""
	}
	return out
}

func clearDupSeasons(base, extra []JobFile, spans []fileSpan) []JobFile {
	counts := map[string]int{}
	add := func(f JobFile) {
		if f.Season == "" || f.Episode == "" {
			return
		}
		counts[f.Season+"\t"+f.Episode]++
	}
	for _, f := range base {
		add(f)
	}
	for _, f := range extra {
		add(f)
	}
	bad := map[int]bool{}
	for si, sp := range spans {
		for k := sp.from; k < sp.to && k < len(extra); k++ {
			f := extra[k]
			if f.Season == "" || f.Episode == "" {
				continue
			}
			if counts[f.Season+"\t"+f.Episode] > 1 {
				bad[si] = true
			}
		}
	}
	for si, sp := range spans {
		if !bad[si] {
			continue
		}
		for k := sp.from; k < sp.to && k < len(extra); k++ {
			extra[k].Season = ""
			extra[k].Episode = ""
		}
	}
	return extra
}

func normNum(s string) string {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return strings.TrimSpace(s)
	}
	return strconv.Itoa(n)
}

func trimExt(name string) string {
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}
