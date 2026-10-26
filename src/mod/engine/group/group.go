package group

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/clean"
	"github.com/alyshmahell/matchmedia/src/mod/config"
)

type Title struct {
	Name  string
	Files []string
}

type Library struct {
	ParentWeight float64
	Extensions   map[string]struct{}
	Season       *regexp.Regexp
	Clusterer    Clusterer
	clean        func(string) string
}

func New(cfg *config.Config) Library {
	if cfg == nil {
		return Library{}
	}
	return Library{
		ParentWeight: cfg.Group.ParentWeight,
		Extensions:   cfg.GroupVideoExt(),
		Season:       cfg.Season(),
		Clusterer:    Clusterer{Threshold: cfg.Group.DistanceThreshold},
		clean:        func(s string) string { return clean.Apply(cfg, s) },
	}
}

func (l Library) cmp(s string) string {
	if l.clean == nil {
		return s
	}
	if c := strings.TrimSpace(l.clean(s)); c != "" {
		return c
	}
	return s
}

type looseGroup struct {
	members []string
	idxs    []int
}

func (l Library) Group(dir string) ([]Title, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var seasonDirs, titleDirs, loose []string
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() {
			if l.Season.MatchString(name) {
				seasonDirs = append(seasonDirs, name)
			} else {
				titleDirs = append(titleDirs, name)
			}
			continue
		}
		if isVideo(l.Extensions, name) {
			loose = append(loose, name)
		}
	}

	type titleOut struct {
		titles []Title
		err    error
	}
	grouped := parallelMap(len(titleDirs), func(i int) titleOut {
		folder := titleDirs[i]
		paths, err := l.listFiles(filepath.Join(dir, folder), dir)
		if err != nil {
			return titleOut{err: err}
		}
		if len(paths) == 0 {
			return titleOut{}
		}
		labels := l.clusterPaths(paths, repeat(folder, len(paths)))
		return titleOut{titles: namedClusters(paths, labels, folder)}
	})
	var titles []Title
	for _, g := range grouped {
		if g.err != nil {
			return nil, g.err
		}
		titles = append(titles, g.titles...)
	}

	type pathOut struct {
		paths []string
		err   error
	}
	listed := parallelMap(len(seasonDirs), func(i int) pathOut {
		folder := seasonDirs[i]
		paths, err := l.listFiles(filepath.Join(dir, folder), dir)
		return pathOut{paths: paths, err: err}
	})
	var seasonPaths []string
	for _, part := range listed {
		if part.err != nil {
			return nil, part.err
		}
		seasonPaths = append(seasonPaths, part.paths...)
	}

	show := filepath.Base(dir)
	if len(seasonPaths) > 0 {
		showFiles := append([]string(nil), seasonPaths...)
		if len(loose) > 0 {
			extras, movies := l.looseAgainstSeasons(loose, seasonPaths)
			showFiles = append(showFiles, extras...)
			titles = append(titles, Title{Name: show, Files: showFiles})
			for _, movie := range movies {
				titles = append(titles, Title{Name: stem(movie[0]), Files: movie})
			}
			return titles, nil
		}
		titles = append(titles, Title{Name: show, Files: showFiles})
		return titles, nil
	}
	for _, name := range loose {
		titles = append(titles, Title{Name: stem(name), Files: []string{name}})
	}
	return titles, nil
}

func (l Library) clusterPaths(paths, parents []string) []int {
	names := make([]string, len(paths))
	for i, path := range paths {
		names[i] = l.cmp(filepath.Base(path))
	}
	cleanedParents := make([]string, len(parents))
	for i, name := range parents {
		cleanedParents[i] = l.cmp(name)
	}
	halves := parallelMap(2, func(i int) [][]float64 {
		if i == 0 {
			return NewVectorizer().FitTransform(cleanedParents)
		}
		return NewVectorizer().FitTransform(names)
	})
	scaleRows(halves[0], l.ParentWeight)
	return l.Clusterer.Label(hstack(halves[0], halves[1]))
}

func (l Library) clusterFilenames(paths []string) []int {
	stems := make([]string, len(paths))
	for i, path := range paths {
		stems[i] = l.cmp(stem(path))
	}
	return l.Clusterer.Label(NewVectorizer().FitTransform(stems))
}

func (l Library) looseAgainstSeasons(loose, seasonPaths []string) ([]string, [][]string) {
	seasonStems := make([]string, len(seasonPaths))
	for i, path := range seasonPaths {
		seasonStems[i] = l.cmp(stem(path))
	}
	looseStems := make([]string, len(loose))
	for i, path := range loose {
		looseStems[i] = l.cmp(stem(path))
	}
	fitted := NewVectorizer()
	seasonVecs := fitted.FitTransform(seasonStems)
	looseVecs := fitted.Transform(looseStems)
	centroid := meanVectors(seasonVecs)
	labels := l.clusterFilenames(loose)

	groups := make(map[int]*looseGroup)
	var seen []int
	for i, path := range loose {
		label := labels[i]
		g, ok := groups[label]
		if !ok {
			g = &looseGroup{}
			groups[label] = g
			seen = append(seen, label)
		}
		g.members = append(g.members, path)
		g.idxs = append(g.idxs, i)
	}
	ordered := make([]looseGroup, len(seen))
	distances := make([]float64, len(seen))
	for i, label := range seen {
		g := groups[label]
		ordered[i] = *g
		distances[i] = cosineDistance(meanIndexed(looseVecs, g.idxs), centroid)
	}
	return splitFarthest(ordered, distances)
}

func (l Library) listFiles(dir, relativeTo string) ([]string, error) {
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
			if isVideo(l.Extensions, e.Name()) {
				files = append(files, e.Name())
			}
		}
		for _, name := range files {
			rel, err := filepath.Rel(relativeTo, filepath.Join(current, name))
			if err != nil {
				return err
			}
			out = append(out, rel)
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

func namedClusters(paths []string, labels []int, parentName string) []Title {
	groups := make(map[int][]string)
	var seen []int
	for i, path := range paths {
		label := labels[i]
		if _, ok := groups[label]; !ok {
			seen = append(seen, label)
		}
		groups[label] = append(groups[label], path)
	}
	slices.SortStableFunc(seen, func(a, b int) int {
		return len(groups[b]) - len(groups[a])
	})
	titles := make([]Title, len(seen))
	for i, label := range seen {
		members := groups[label]
		name := stem(members[0])
		if i == 0 {
			name = parentName
		}
		titles[i] = Title{Name: name, Files: members}
	}
	return titles
}

func splitFarthest(groups []looseGroup, distances []float64) (extras []string, movies [][]string) {
	farthest := math.Inf(-1)
	for _, d := range distances {
		if d > farthest {
			farthest = d
		}
	}
	for i, g := range groups {
		if isClose(distances[i], farthest) {
			movies = append(movies, g.members)
			continue
		}
		extras = append(extras, g.members...)
	}
	return extras, movies
}

func isClose(a, b float64) bool {
	const (
		rtol = 1e-05
		atol = 1e-08
	)
	return math.Abs(a-b) <= atol+rtol*math.Abs(b)
}

func isVideo(exts map[string]struct{}, name string) bool {
	_, ok := exts[strings.ToLower(filepath.Ext(name))]
	return ok
}

func stem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func scaleRows(rows [][]float64, weight float64) {
	for _, row := range rows {
		for i := range row {
			row[i] *= weight
		}
	}
}

func hstack(a, b [][]float64) [][]float64 {
	out := make([][]float64, len(a))
	for i := range a {
		row := make([]float64, 0, len(a[i])+len(b[i]))
		row = append(row, a[i]...)
		row = append(row, b[i]...)
		out[i] = row
	}
	return out
}

func meanVectors(vecs [][]float64) []float64 {
	if len(vecs) == 0 {
		return nil
	}
	out := make([]float64, len(vecs[0]))
	for _, vec := range vecs {
		for i, x := range vec {
			out[i] += x
		}
	}
	inv := 1 / float64(len(vecs))
	for i := range out {
		out[i] *= inv
	}
	return out
}

func meanIndexed(vecs [][]float64, idxs []int) []float64 {
	out := make([]float64, len(vecs[0]))
	for _, i := range idxs {
		for j, x := range vecs[i] {
			out[j] += x
		}
	}
	inv := 1 / float64(len(idxs))
	for j := range out {
		out[j] *= inv
	}
	return out
}
