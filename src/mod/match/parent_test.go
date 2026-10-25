package match

import (
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/fetch"
)

func showChildren() (Job, Job) {
	a := Job{
		Title:  "S01",
		Parent: "Show",
		Path:   "/media/Show/S01",
		Kind:   "movie",
		Status: "pending",
		Source: "scan",
		Files:  []JobFile{{Path: "s01e01.mkv"}, {Path: "s01e02.mkv"}},
	}
	b := Job{
		Title:  "S02",
		Parent: "Show",
		Path:   "/media/Show/S02",
		Kind:   "movie",
		Status: "pending",
		Source: "scan",
		Files:  []JobFile{{Path: "s02e01.mkv"}},
	}
	return a, b
}

func TestChildFail(t *testing.T) {
	if !ChildFail(Job{Title: "S01", Parent: "Show", Status: "unmatched"}) {
		t.Fatal("unmatched season")
	}
	if !ChildFail(Job{Title: "S01", Parent: "Show", Status: "error"}) {
		t.Fatal("error season")
	}
	if ChildFail(Job{Title: "Leftover", Status: "unmatched"}) {
		t.Fatal("title-level unmatched")
	}
	if ChildFail(Job{Title: "Show", Parent: "Show", Status: "unmatched"}) {
		t.Fatal("parent==title")
	}
	if ChildFail(Job{Title: "S01", Parent: "Show", Status: "matched"}) {
		t.Fatal("matched child")
	}
}

func TestParentJobsFirst(t *testing.T) {
	a, b := showChildren()
	got := parentJobs([]Job{a, b})
	if len(got) != 1 {
		t.Fatalf("parents %d: %#v", len(got), got)
	}
	p := got[0]
	if p.Title != "Show" || p.Path != filepath.Clean("/media/Show") {
		t.Fatalf("parent %#v", p)
	}
	if p.Status != "pending" || p.Kind != "show" || len(p.Files) != 0 {
		t.Fatalf("status/kind/files %#v", p)
	}
}

func TestParentJobsSkipExistingPath(t *testing.T) {
	if got := parentJobs([]Job{
		{Title: "Show", Parent: "Show", Path: "/media/Show", Status: "pending"},
		{Title: "S01", Parent: "Show", Path: "/media/Show/S01", Status: "pending"},
	}); len(got) != 0 {
		t.Fatalf("existing folder path: %#v", got)
	}
	if got := parentJobs([]Job{
		{Title: "Show", Parent: "Show", Path: "/media/Show", Status: "pending"},
	}); len(got) != 0 {
		t.Fatalf("parent==title: %#v", got)
	}
	if got := parentJobs([]Job{
		{Title: "X", Parent: "", Path: "/media/X", Status: "pending"},
	}); len(got) != 0 {
		t.Fatalf("empty parent: %#v", got)
	}
}

func TestParentJobsIgnoresRewrittenShowTitle(t *testing.T) {
	show := Job{
		Title:  "Anohana: The Flower We Saw That Day",
		Parent: "Anohana",
		Path:   "/library/Anohana",
		Kind:   "show",
		Status: "matched",
		Files:  []JobFile{{Path: "e01.mkv"}},
	}
	if isChild(show) {
		t.Fatal("show folder is not a child")
	}
	if got := parentJobs([]Job{show}); len(got) != 0 {
		t.Fatalf("no library-root parent: %#v", got)
	}
	punct := Job{
		Title:  "An Adventurer's Daily Grind at Age 29",
		Parent: "An Adventure s Daily Grind at Age 29",
		Path:   "/library/An Adventure's Daily Grind at Age 29",
		Kind:   "show",
		Status: "matched",
		Files:  []JobFile{{Path: "e01.mkv"}},
	}
	if isChild(punct) {
		t.Fatal("apostrophe folder is not a child")
	}
	if got := parentJobs([]Job{punct}); len(got) != 0 {
		t.Fatalf("apostrophe parent: %#v", got)
	}
	year := Job{
		Title:  "Koe no Katachi",
		Parent: "A Silent Voice",
		Path:   "/library/A Silent Voice (2016)",
		Kind:   "movie",
		Status: "matched",
		Files:  []JobFile{{Path: "movie.mkv"}},
	}
	if isChild(year) {
		t.Fatal("year folder is not a child")
	}
	if got := parentJobs([]Job{year}); len(got) != 0 {
		t.Fatalf("year parent: %#v", got)
	}
}

func TestParentJobsKeepsReleaseFolder(t *testing.T) {
	release := Job{
		Title:  "Tsukimichi Moonlit Fantasy",
		Parent: "Tsukimichi",
		Path:   "/library/Tsukimichi/Moonlit Fantasy",
		Kind:   "show",
		Status: "pending",
		Files:  []JobFile{{Path: "e01.mkv"}},
	}
	got := parentJobs([]Job{release})
	if len(got) != 1 {
		t.Fatalf("parents %d: %#v", len(got), got)
	}
	p := got[0]
	if p.Title != "Tsukimichi" || p.Parent != "Tsukimichi" || p.Path != filepath.Clean("/library/Tsukimichi") {
		t.Fatalf("parent %#v", p)
	}
}

func TestFoldKeepsEmptyParent(t *testing.T) {
	child := Job{
		Title:  "Release",
		Parent: "Show",
		Path:   "/media/Show/Release",
		Status: "matched",
		Match:  &Candidate{ID: "2"},
		Files:  []JobFile{{Path: "a.mkv"}},
	}
	parent := parentJobs([]Job{child})
	if len(parent) != 1 {
		t.Fatalf("parents %d", len(parent))
	}
	parent[0].Status = "matched"
	parent[0].Match = &Candidate{ID: "1"}
	got := foldUnmatched(nil, []Job{parent[0], child})
	if len(got) != 2 || got[0].Title != "Show" || len(got[0].Files) != 0 || got[1].Title != "Release" {
		t.Fatalf("empty parent: %#v", got)
	}
}

func TestFoldUnmatchedIntoParent(t *testing.T) {
	s01 := Job{
		Title:  "S01",
		Parent: "Show",
		Path:   "/media/Show/S01",
		Status: "unmatched",
		Files:  []JobFile{{Path: "a.mkv"}},
	}
	s02 := Job{
		Title:  "S02",
		Parent: "Show",
		Path:   "/media/Show/S02",
		Status: "error",
		Files:  []JobFile{{Path: "b.mkv"}},
	}
	parent := Job{
		Title:  "Show",
		Parent: "Show",
		Path:   filepath.Clean("/media/Show"),
		Status: "matched",
		Kind:   "show",
		Match:  &Candidate{Title: "Show", ID: "1"},
	}
	got := foldUnmatched(nil, []Job{parent, s01, s02})
	if len(got) != 1 || got[0].Title != "Show" {
		t.Fatalf("want only parent, got %#v", got)
	}
	if !reflect.DeepEqual(got[0].Files, []JobFile{{Path: "a.mkv"}, {Path: "b.mkv"}}) {
		t.Fatalf("files %#v", got[0].Files)
	}
}

func TestFoldOnlyUnmatchedChild(t *testing.T) {
	ok := Job{
		Title:  "Spinoff",
		Parent: "Show",
		Path:   "/media/Show/Spinoff",
		Status: "matched",
		Match:  &Candidate{ID: "1"},
		Files:  []JobFile{{Path: "ok.mkv"}},
	}
	fail := Job{
		Title:  "S02",
		Parent: "Show",
		Path:   "/media/Show/S02",
		Status: "unmatched",
		Files:  []JobFile{{Path: "fail.mkv"}},
	}
	parent := Job{
		Title:  "Show",
		Parent: "Show",
		Path:   filepath.Clean("/media/Show"),
		Status: "matched",
		Match:  &Candidate{ID: "2"},
	}
	got := foldUnmatched(nil, []Job{parent, ok, fail})
	if len(got) != 2 {
		t.Fatalf("parent + matched child: %#v", got)
	}
	if got[0].Title != "Show" || !reflect.DeepEqual(got[0].Files, []JobFile{{Path: "fail.mkv"}}) {
		t.Fatalf("folded files %#v", got[0])
	}
	if got[1].Title != "Spinoff" {
		t.Fatalf("kept child %#v", got[1])
	}
}

func TestFoldSkipsFailedParent(t *testing.T) {
	fail := Job{
		Title:  "S01",
		Parent: "Show",
		Path:   "/media/Show/S01",
		Status: "unmatched",
		Files:  []JobFile{{Path: "a.mkv"}},
	}
	parent := Job{
		Title:  "Show",
		Parent: "Show",
		Path:   filepath.Clean("/media/Show"),
		Status: "unmatched",
	}
	got := foldUnmatched(nil, []Job{parent, fail})
	if len(got) != 2 {
		t.Fatalf("keep children when parent failed: %#v", got)
	}
}

func TestFoldKeepsParentWhenAllMatched(t *testing.T) {
	s01 := Job{
		Title:  "S01",
		Parent: "Show",
		Path:   "/media/Show/S01",
		Status: "matched",
		Match:  &Candidate{ID: "1"},
	}
	s02 := Job{
		Title:  "S02",
		Parent: "Show",
		Path:   "/media/Show/S02",
		Status: "matched",
		Match:  &Candidate{ID: "2"},
	}
	parent := Job{
		Title:  "Show",
		Parent: "Show",
		Path:   filepath.Clean("/media/Show"),
		Status: "matched",
		Match:  &Candidate{ID: "3"},
	}
	got := foldUnmatched(nil, []Job{parent, s01, s02})
	if len(got) != 1 || got[0].Path != filepath.Clean("/media/Show") {
		t.Fatalf("season folders join the parent: %#v", got)
	}
}

func TestFoldManualParent(t *testing.T) {
	fail := Job{
		Title:  "S01",
		Parent: "Show",
		Path:   "/media/Show/S01",
		Status: "unmatched",
		Files:  []JobFile{{Path: "a.mkv"}},
	}
	parent := Job{
		Title:  "Show",
		Parent: "Show",
		Path:   filepath.Clean("/media/Show"),
		Status: "manual",
	}
	got := foldUnmatched(nil, []Job{parent, fail})
	if len(got) != 1 || got[0].Status != "manual" || len(got[0].Files) != 1 {
		t.Fatalf("manual parent folds unmatched: %#v", got)
	}
}

func TestFoldReleaseFoldersOntoSeasons(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow(catalogSeasons(12, 12))
	s01 := releaseJob("Show S01", numberedFiles("/media/Show/Show S01", "Show - ", 1, 12), "unmatched")
	s02 := releaseJob("Show S02", numberedFiles("/media/Show/Show S02", "Show - ", 1, 12), "unmatched")
	got := foldUnmatched(cfg, []Job{parent, s01, s02})
	if len(got) != 1 {
		t.Fatalf("jobs %d", len(got))
	}
	assertSeason(t, got[0], "1", 12)
	assertSeason(t, got[0], "2", 12)
}

func TestFoldSeasonFolderKeepsFilenameEpisodes(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow(catalogSeasons(12, 12))
	files := make([]JobFile, 12)
	for i := 0; i < 12; i++ {
		files[i] = JobFile{Path: "/media/Show/Show (Season 2)/Show - S01E" + two(i+1) + ".mkv"}
	}
	child := releaseJob("Show (Season 2)", files, "unmatched")
	got := foldUnmatched(cfg, []Job{parent, child})
	if len(got) != 1 {
		t.Fatalf("jobs %d", len(got))
	}
	assertEpisode(t, got[0], "2", "1")
	assertEpisode(t, got[0], "2", "12")
	for _, f := range got[0].Files {
		if f.Season == "1" {
			t.Fatalf("filename season kept: %#v", f)
		}
	}
}

func TestFoldMixedFilenameSeasonsStayUnnumbered(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow(catalogSeasons(12, 12))
	files := []JobFile{
		{Path: "/media/Show/Show (Season 2)/Show - S01E01.mkv"},
		{Path: "/media/Show/Show (Season 2)/Show - S02E01.mkv"},
	}
	// Two files are inside the ballpark of a 12-episode season (slack 2)? abs(10) > 2.
	// Use a 2-episode season so the token still fits, then mixed names refuse numbering.
	parent.Catalog = []CatalogSeason{seasonWith("2", "Season 2", 2)}
	child := releaseJob("Show (Season 2)", files, "unmatched")
	got := foldUnmatched(cfg, []Job{parent, child})
	if len(got) != 1 || len(got[0].Files) != 2 {
		t.Fatalf("fold %#v", got)
	}
	for _, f := range got[0].Files {
		if f.Season != "" || f.Episode != "" {
			t.Fatalf("numbered mixed seasons: %#v", f)
		}
	}
}

func TestFoldCumulativeEpisodeNumbers(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow(catalogSeasons(12, 12))
	child := releaseJob("Show S02", numberedFiles("/media/Show/Show S02", "Show - ", 13, 24), "unmatched")
	got := foldUnmatched(cfg, []Job{parent, child})
	assertEpisode(t, got[0], "2", "1")
	assertEpisode(t, got[0], "2", "12")
	for _, f := range got[0].Files {
		if f.Episode == "13" {
			t.Fatalf("unshifted episode: %#v", f)
		}
	}
}

func TestFoldInRangeBlocksShift(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow(catalogSeasons(12, 12))
	files := numberedFiles("/media/Show/Show S02", "Show - ", 13, 23)
	files = append(files, JobFile{Path: "/media/Show/Show S02/Show - 01.mkv"})
	child := releaseJob("Show S02", files, "unmatched")
	got := foldUnmatched(cfg, []Job{parent, child})
	foundShifted := false
	foundOne := false
	for _, f := range got[0].Files {
		if f.Episode == "1" && filepath.Base(f.Path) == "Show - 01.mkv" {
			foundOne = true
		}
		if f.Episode == "1" && filepath.Base(f.Path) == "Show - 13.mkv" {
			foundShifted = true
		}
	}
	if !foundOne || foundShifted {
		t.Fatalf("shift blocked: %#v", got[0].Files)
	}
}

func TestFoldIgnoresQualityTags(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow(catalogSeasons(12, 12))
	child := releaseJob("Show S02 1080p Dual Audio WEBRip DD+ x265-ZZ", numberedFiles("/media/Show/Show S02 1080p Dual Audio WEBRip DD+ x265-ZZ", "Show - ", 1, 12), "unmatched")
	child.Files[0].Path = "/media/Show/pack/Show - 01 [1080p][HEVC x265 10bit].mkv"
	got := foldUnmatched(cfg, []Job{parent, child})
	assertEpisode(t, got[0], "2", "1")
	for _, f := range got[0].Files {
		if strings.Contains(f.Path, "10bit") && f.Episode != "1" {
			t.Fatalf("tag became episode: %#v", f)
		}
	}
}

func TestFoldExactCountWithoutToken(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow([]CatalogSeason{seasonWith("1", "Season 1", 12)})
	child := releaseJob("Release", numberedFiles("/media/Show/Release", "Show - ", 1, 12), "unmatched")
	got := foldUnmatched(cfg, []Job{parent, child})
	assertSeason(t, got[0], "1", 12)
}

func TestFoldAmbiguousCountStaysUnnumbered(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow(catalogSeasons(12, 12))
	child := releaseJob("Release", numberedFiles("/media/Show/Release", "Show - ", 1, 12), "unmatched")
	got := foldUnmatched(cfg, []Job{parent, child})
	for _, f := range got[0].Files {
		if f.Season != "" {
			t.Fatalf("ambiguous count numbered: %#v", f)
		}
	}
}

func TestFoldNearCountWithoutTokenStaysUnnumbered(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow([]CatalogSeason{seasonWith("1", "Season 1", 12)})
	child := releaseJob("Release", numberedFiles("/media/Show/Release", "Show - ", 1, 11), "unmatched")
	got := foldUnmatched(cfg, []Job{parent, child})
	for _, f := range got[0].Files {
		if f.Season != "" {
			t.Fatalf("near count numbered: %#v", f)
		}
	}
}

func TestFoldMoviesOnlyOntoSpecials(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow([]CatalogSeason{
		seasonWith("1", "Season 1", 12),
		seasonWith("0", "Specials", 4),
	})
	movies := releaseJob("Show Movies", numberedFiles("/media/Show/Show Movies", "Part - ", 1, 4), "unmatched")
	s02 := releaseJob("Show S02", numberedFiles("/media/Show/Show S02", "Show - ", 1, 4), "unmatched")
	got := foldUnmatched(cfg, []Job{parent, movies, s02})
	if len(got) != 1 {
		t.Fatalf("jobs %#v", got)
	}
	assertSeason(t, got[0], "0", 4)
	for _, f := range got[0].Files {
		if strings.Contains(f.Path, "Show S02") && f.Season != "" {
			t.Fatalf("s02 on specials: %#v", f)
		}
	}
}

func TestFoldKeepsMatchedAndManualChildren(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow(catalogSeasons(12, 12))
	matched := releaseJob("Spinoff", numberedFiles("/media/Show/Spinoff", "Spinoff - ", 1, 12), "matched")
	matched.Match = &Candidate{ID: "9"}
	manual := releaseJob("Gaiden", numberedFiles("/media/Show/Gaiden", "Gaiden - ", 1, 12), "manual")
	got := foldUnmatched(cfg, []Job{parent, matched, manual})
	if len(got) != 3 {
		t.Fatalf("kept children: %#v", got)
	}
}

func TestFoldMatchedSeasonFolder(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow(catalogSeasons(12, 12))
	matched := releaseJob("Show S01", numberedFiles("/media/Show/Show S01", "Show - ", 1, 12), "matched")
	matched.Match = &Candidate{ID: "9"}
	loose := releaseJob("Season 9", []JobFile{
		{Path: "/media/Show/Season 9/a.mkv"},
		{Path: "/media/Show/Season 9/b.mkv"},
	}, "manual")
	spinoff := releaseJob("Spinoff", numberedFiles("/media/Show/Spinoff", "Spinoff - ", 1, 3), "matched")
	spinoff.Match = &Candidate{ID: "8"}
	got := foldUnmatched(cfg, []Job{parent, matched, loose, spinoff})
	if len(got) != 2 {
		t.Fatalf("jobs %#v", got)
	}
	if got[0].Title != "Show" || got[1].Title != "Spinoff" || len(got[0].Files) != 14 {
		t.Fatalf("kept %#v", got)
	}
	assertSeason(t, got[0], "1", 12)
	for _, f := range got[0].Files {
		if strings.Contains(f.Path, "Season 9") && (f.Season != "" || f.Episode != "") {
			t.Fatalf("unfitted season numbered: %#v", f)
		}
	}
}

func TestFoldDuplicateEpisodeClearsNumbering(t *testing.T) {
	cfg := foldCfg(t)
	parent := matchedShow([]CatalogSeason{seasonWith("1", "Season 1", 2)})
	parent.Files = []JobFile{{Path: "/media/Show/already.mkv", Season: "1", Episode: "1"}}
	child := releaseJob("Show S01", []JobFile{
		{Path: "/media/Show/Show S01/Show - 01.mkv"},
		{Path: "/media/Show/Show S01/Show - 02.mkv"},
	}, "unmatched")
	got := foldUnmatched(cfg, []Job{parent, child})
	if len(got) != 1 || len(got[0].Files) != 3 {
		t.Fatalf("files %#v", got)
	}
	if got[0].Files[0].Season != "1" || got[0].Files[0].Episode != "1" {
		t.Fatalf("parent file %#v", got[0].Files[0])
	}
	for _, f := range got[0].Files[1:] {
		if f.Season != "" || f.Episode != "" {
			t.Fatalf("duplicate kept numbering: %#v", f)
		}
	}
}

func foldCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func matchedShow(catalog []CatalogSeason) Job {
	return Job{
		Title:   "Show",
		Parent:  "Show",
		Path:    filepath.Clean("/media/Show"),
		Status:  "matched",
		Kind:    "show",
		Match:   &Candidate{ID: "1"},
		Catalog: catalog,
	}
}

func releaseJob(name string, files []JobFile, status string) Job {
	return Job{
		Title:  name,
		Parent: "Show",
		Path:   filepath.Join("/media/Show", name),
		Status: status,
		Files:  files,
	}
}

func catalogSeasons(a, b int) []CatalogSeason {
	return []CatalogSeason{seasonWith("1", "Season 1", a), seasonWith("2", "Season 2", b)}
}

func seasonWith(num, title string, n int) CatalogSeason {
	s := CatalogSeason{Number: num, Title: title}
	for i := 1; i <= n; i++ {
		s.Episodes = append(s.Episodes, fetch.Episode{Number: strconv.Itoa(i)})
	}
	return s
}

func numberedFiles(dir, prefix string, from, to int) []JobFile {
	out := make([]JobFile, 0, to-from+1)
	for n := from; n <= to; n++ {
		out = append(out, JobFile{Path: filepath.Join(dir, prefix+two(n)+".mkv")})
	}
	return out
}

func two(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func assertSeason(t *testing.T, job Job, season string, n int) {
	t.Helper()
	count := 0
	for _, f := range job.Files {
		if f.Season == season {
			count++
		}
	}
	if count != n {
		t.Fatalf("season %s count %d files %#v", season, count, job.Files)
	}
}

func assertEpisode(t *testing.T, job Job, season, episode string) {
	t.Helper()
	for _, f := range job.Files {
		if f.Season == season && f.Episode == episode {
			return
		}
	}
	t.Fatalf("missing %s episode %s in %#v", season, episode, job.Files)
}
