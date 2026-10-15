package rank

import (
	"regexp"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/config"
)

var nonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)

type Query struct {
	Title  string
	Year   string
	Parent string
	Type   string
}

type Hit struct {
	Provider string
	ID       string
	Title    string
	Year     string
	URL      string
	Synopsis string
	Poster   string
	Attrs    map[string]string
	NFO      string
	UniqueID string
	QueryCov float64
	Jaccard  float64
	Score    float64
}

func (h Hit) Key() string {
	return h.Provider + ":" + h.ID
}

func Rank(cfg *config.Config, q Query, hits []Hit) []Hit {
	if cfg == nil || len(hits) == 0 {
		return hits
	}
	year := strings.TrimSpace(q.Year)
	stop := cfg.PlotStop()
	titleSet := tokenSet(q.Title)
	qplot := plotQuery(q.Title, stop)
	var parentSet map[string]struct{}
	if parent := strings.TrimSpace(q.Parent); parent != "" && titleNorm(parent) != titleNorm(q.Title) {
		parentSet = contentSet(parent, stop)
	}
	out := append([]Hit(nil), hits...)
	synSets := make([]map[string]struct{}, len(out))
	for i, c := range out {
		candSet := candidateTitleSet(c)
		out[i].Jaccard = bestTitleJaccard(titleSet, c)
		out[i].QueryCov = coverage(titleSet, candSet)
		synSets[i] = tokenSet(c.Synopsis)
	}
	df := plotDF(qplot, synSets)
	titleLift := cfg.TitleLift()
	exactLift := cfg.ExactLift()
	den := 1 + titleLift + exactLift
	jobNorm := titleNorm(q.Title)
	for i := range out {
		j := out[i].Jaccard
		plot := 0.0
		if j < 1 {
			plot = plotFunc(qplot, synSets[i], df)
			parentCov := coverage(parentSet, candidateTitleSet(out[i]))
			if parentCov > plot {
				plot = parentCov
			}
		}
		exact := 0.0
		if jobNorm != "" && jobNorm == titleNorm(out[i].Title) {
			exact = 1
		}
		out[i].Score = (j + titleLift*plot + exactLift*exact) / den
	}
	sortByScore(out, q.Title, year, stop)
	return out
}

func Auto(cfg *config.Config, q Query, ranked []Hit) (Hit, bool) {
	if cfg == nil {
		return Hit{}, false
	}
	return autoMatch(cfg, q, ranked, cfg.Match.Prefer[q.Type])
}

func autoMatch(cfg *config.Config, q Query, ranked []Hit, want map[string]string) (Hit, bool) {
	if len(ranked) == 0 {
		return Hit{}, false
	}
	best := ranked[0]
	if matchStrength(best) >= cfg.Match.MinScore {
		if len(ranked) < 2 || matchStrength(best)-matchStrength(ranked[1]) >= cfg.Match.MinMargin {
			return best, true
		}
	}
	if len(ranked) > 1 && best.Score >= cfg.Match.MinScore && best.Score-ranked[1].Score >= cfg.Match.MinMargin {
		return best, true
	}
	if uniqueQueryCov(cfg, q, ranked) && primaryExtendsQuery(q.Title, best) {
		return best, true
	}
	pref := preferSubset(want, ranked)
	if len(pref) < 2 || pref[0].QueryCov < 1 {
		return Hit{}, false
	}
	if pref[0].Score-pref[1].Score < cfg.Match.MinMargin {
		return Hit{}, false
	}
	return pref[0], true
}

func matchStrength(c Hit) float64 {
	if c.Score > c.Jaccard {
		return c.Score
	}
	return c.Jaccard
}

func primaryExtendsQuery(jobTitle string, c Hit) bool {
	q := tokenSet(jobTitle)
	if len(q) == 0 {
		return false
	}
	_, covered := bestCoveringTitle(q, c)
	cset := tokenSet(covered)
	if len(cset) <= len(q) {
		return false
	}
	return coverage(q, cset) == 1
}

func uniqueQueryCov(cfg *config.Config, q Query, ranked []Hit) bool {
	if len(ranked) == 0 || ranked[0].QueryCov < 1 {
		return false
	}
	if len(plotQuery(q.Title, cfg.PlotStop())) == 0 {
		return false
	}
	seen := map[string]struct{}{}
	for _, c := range ranked {
		if c.QueryCov != 1 {
			continue
		}
		seen[coveringWorkKey(q.Title, c)] = struct{}{}
	}
	return len(seen) == 1
}

func preferSubset(want map[string]string, cands []Hit) []Hit {
	if len(want) == 0 {
		return nil
	}
	var keep []Hit
	for _, c := range cands {
		if preferMatch(c, want) {
			keep = append(keep, c)
		}
	}
	return keep
}

func preferMatch(c Hit, want map[string]string) bool {
	for k, v := range want {
		got := ""
		if c.Attrs != nil {
			got = c.Attrs[k]
		}
		if !strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(v)) {
			return false
		}
	}
	return true
}

func plotQuery(title string, stop map[string]struct{}) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, t := range strings.Fields(titleNorm(title)) {
		if len(t) < 3 {
			continue
		}
		if _, ok := stop[t]; ok {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func plotDF(q []string, syns []map[string]struct{}) map[string]int {
	df := make(map[string]int, len(q))
	for _, t := range q {
		n := 0
		for _, syn := range syns {
			if _, ok := syn[t]; ok {
				n++
			}
		}
		df[t] = n
	}
	return df
}

func plotFunc(q []string, syn map[string]struct{}, df map[string]int) float64 {
	if len(q) == 0 || len(syn) == 0 {
		return 0
	}
	var num, den float64
	for _, t := range q {
		u := 1.0
		if df[t] >= 2 {
			u = 0.25
		}
		w := float64(len(t)) * u
		den += w
		if _, ok := syn[t]; ok {
			num += w
		}
	}
	if den == 0 {
		return 0
	}
	return num / den
}

func contentSet(s string, stop map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for _, t := range plotQuery(s, stop) {
		out[t] = struct{}{}
	}
	return out
}

func bestTitleJaccard(q map[string]struct{}, c Hit) float64 {
	j, _ := bestCoveringTitle(q, c)
	return j
}

func bestCoveringTitle(q map[string]struct{}, c Hit) (float64, string) {
	bestJ := jaccard(q, tokenSet(c.Title))
	best := c.Title
	for k, v := range c.Attrs {
		if !strings.HasPrefix(strings.ToLower(k), "title") {
			continue
		}
		if alt := jaccard(q, tokenSet(v)); alt > bestJ {
			bestJ = alt
			best = v
		}
	}
	return bestJ, best
}

func coveringWorkKey(jobTitle string, c Hit) string {
	_, title := bestCoveringTitle(tokenSet(jobTitle), c)
	return titleNorm(title) + "\t" + strings.TrimSpace(c.Year)
}

func candidateTitleSet(c Hit) map[string]struct{} {
	out := tokenSet(c.Title)
	for k, v := range c.Attrs {
		if strings.HasPrefix(strings.ToLower(k), "title") {
			out = unionSets(out, tokenSet(v))
		}
	}
	return out
}

func sharesContent(a, b map[string]struct{}) bool {
	for t := range a {
		if _, ok := b[t]; ok {
			return true
		}
	}
	return false
}

func contentOverlap(jobTitle string, c Hit, stop map[string]struct{}) bool {
	return sharesContent(contentSet(jobTitle, stop), contentSetFromTokens(candidateTitleSet(c), stop))
}

func contentSetFromTokens(toks map[string]struct{}, stop map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for t := range toks {
		if len(t) < 3 {
			continue
		}
		if _, ok := stop[t]; ok {
			continue
		}
		out[t] = struct{}{}
	}
	return out
}

func titleTokenPrefix(jobTitle, candTitle string) bool {
	q := strings.Fields(titleNorm(jobTitle))
	c := strings.Fields(titleNorm(candTitle))
	if len(q) == 0 || len(q) > len(c) {
		return false
	}
	for i := range q {
		if q[i] != c[i] {
			return false
		}
	}
	return true
}

func TitleNorm(s string) string {
	return titleNorm(s)
}

func TokenSet(s string) map[string]struct{} {
	return tokenSet(s)
}

func Coverage(q, c map[string]struct{}) float64 {
	return coverage(q, c)
}

func SharesContent(a, b map[string]struct{}) bool {
	return sharesContent(a, b)
}

func ContentSet(s string, stop map[string]struct{}) map[string]struct{} {
	return contentSet(s, stop)
}

func titleNorm(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, ";", " "))
	return strings.TrimSpace(nonAlnumRe.ReplaceAllString(s, " "))
}

func tokenSet(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, t := range strings.Fields(titleNorm(s)) {
		if t != "" {
			out[t] = struct{}{}
		}
	}
	return out
}

func unionSets(a, b map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(a)+len(b))
	for t := range a {
		out[t] = struct{}{}
	}
	for t := range b {
		out[t] = struct{}{}
	}
	return out
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	uni := len(a)
	for t := range b {
		if _, ok := a[t]; ok {
			inter++
		} else {
			uni++
		}
	}
	if uni == 0 {
		return 1
	}
	return float64(inter) / float64(uni)
}

func coverage(q, c map[string]struct{}) float64 {
	if len(q) == 0 {
		return 0
	}
	n := 0
	for t := range q {
		if _, ok := c[t]; ok {
			n++
		}
	}
	return float64(n) / float64(len(q))
}

func sortByScore(cands []Hit, jobTitle, year string, stop map[string]struct{}) {
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0 && betterRank(cands[j], cands[j-1], jobTitle, year, stop); j-- {
			cands[j], cands[j-1] = cands[j-1], cands[j]
		}
	}
}

func betterRank(a, b Hit, jobTitle, year string, stop map[string]struct{}) bool {
	ao, bo := contentOverlap(jobTitle, a, stop), contentOverlap(jobTitle, b, stop)
	if ao != bo {
		return ao
	}
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	ap, bp := titleTokenPrefix(jobTitle, a.Title), titleTokenPrefix(jobTitle, b.Title)
	if ap != bp {
		return ap
	}
	if a.Jaccard != b.Jaccard {
		return a.Jaccard > b.Jaccard
	}
	if year != "" && (a.Year == year) != (b.Year == year) {
		return a.Year == year
	}
	return false
}
