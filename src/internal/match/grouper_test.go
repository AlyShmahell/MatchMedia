package match

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/alyshmahell/matchmedia/src/internal/config"
)

func writeTree(t *testing.T, root string, files []string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func titlesOf(got []Grouped) []string {
	out := make([]string, len(got))
	for i, g := range got {
		out[i] = g.Title
	}
	return out
}

func testCfg(t *testing.T) config.Config {
	t.Helper()
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	cfg, err := config.Load(filepath.Join("..", "..", "share", "config", "default.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestGroupSeasonOnlyTree(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Aldnoah Zero")
	writeTree(t, lib, []string{
		"Aldnoah Zero/Season 1/Aldnoah Zero S01E01.mkv",
		"Aldnoah Zero/Season 2/Aldnoah Zero S02E01.mkv",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 1 {
		t.Fatalf("got=%v", titlesOf(got))
	}
	if got[0].Title != "Aldnoah Zero" {
		t.Fatalf("title=%q", got[0].Title)
	}
	if got[0].Path != "Aldnoah Zero" {
		t.Fatalf("path=%q", got[0].Path)
	}
}

func TestGroupNamedSiblings(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "KonoSuba")
	writeTree(t, lib, []string{
		"KonoSuba/God's Blessing on This Wonderful World!/Season 1/ep.mkv",
		"KonoSuba/An Explosion on This Wonderful World!/Season 1/ep.mkv",
		"KonoSuba/God's Blessing on This Wonderful World!/Legend of Crimson.mkv",
	})
	got := Group(testCfg(t), lib, child)
	seen := map[string]string{}
	for _, g := range got {
		seen[g.Title] = g.Path
	}
	if _, ok := seen["God's Blessing on This Wonderful World!"]; !ok {
		t.Fatalf("missing blessing: %v", titlesOf(got))
	}
	if _, ok := seen["An Explosion on This Wonderful World!"]; !ok {
		t.Fatalf("missing explosion: %v", titlesOf(got))
	}
	if _, ok := seen["KonoSuba"]; ok {
		t.Fatalf("franchise mash: %v", titlesOf(got))
	}
	if path, ok := seen["Legend of Crimson"]; !ok {
		t.Fatalf("missing movie: %v", titlesOf(got))
	} else if path == "KonoSuba" {
		t.Fatalf("movie path collapsed to parent: %q", path)
	}
	for _, g := range got {
		if g.Parent != "KonoSuba" {
			t.Fatalf("parent=%q title=%q", g.Parent, g.Title)
		}
	}
}

func TestGroupSkipsExtras(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Solo Leveling")
	writeTree(t, lib, []string{
		"Solo Leveling/Season 1/ep.mkv",
		"Solo Leveling/Openings & Endings/NCOP01.mkv",
		"Solo Leveling/NCOP/op.mkv",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 1 || got[0].Title != "Solo Leveling" {
		t.Fatalf("got=%v", titlesOf(got))
	}
}

func TestGroupSmokeReleaseNames(t *testing.T) {
	lib := t.TempDir()
	writeTree(t, lib, []string{
		"Girls.S01.1080p.BluRay.x264-Tag/Girls.S01E01.mkv",
		"Cowboy.Bebop.1998.1080p.BluRay.x264-Tag.mkv",
	})
	cfg := testCfg(t)
	girls := Group(cfg, lib, filepath.Join(lib, "Girls.S01.1080p.BluRay.x264-Tag"))
	if len(girls) != 1 || girls[0].Title != "Girls" {
		t.Fatalf("girls=%v", girls)
	}
	bebop := Group(cfg, lib, filepath.Join(lib, "Cowboy.Bebop.1998.1080p.BluRay.x264-Tag.mkv"))
	if len(bebop) != 1 {
		t.Fatalf("bebop=%v", bebop)
	}
	if bebop[0].Title != "Cowboy Bebop" {
		t.Fatalf("title=%q", bebop[0].Title)
	}
	if bebop[0].Year != "1998" {
		t.Fatalf("year=%q", bebop[0].Year)
	}
}

func TestGroupYearSuffix(t *testing.T) {
	lib := t.TempDir()
	writeTree(t, lib, []string{
		"5 Centimeters Per Second (2007)/5 Centimeters Per Second (2007).mkv",
	})
	got := Group(testCfg(t), lib, filepath.Join(lib, "5 Centimeters Per Second (2007)"))
	if len(got) != 1 {
		t.Fatalf("got=%v", got)
	}
	if got[0].Title != "5 Centimeters Per Second" {
		t.Fatalf("title=%q", got[0].Title)
	}
	if got[0].Year != "2007" {
		t.Fatalf("year=%q", got[0].Year)
	}
}

func TestSeqRatioMatchesPython(t *testing.T) {
	if got := SeqRatio("abcd", "bcde"); got != 0.75 {
		t.Fatalf("ratio=%v", got)
	}
	if got := SeqRatio("cowboy bebop", "cowboy bebop"); got != 1 {
		t.Fatalf("ident=%v", got)
	}
	if got := SeqRatio("", ""); got != 1 {
		t.Fatalf("empty=%v", got)
	}
}

func firstWord(t *testing.T, set map[string]struct{}) string {
	t.Helper()
	var words []string
	for w := range set {
		if w != "" {
			words = append(words, w)
		}
	}
	sort.Strings(words)
	if len(words) == 0 {
		t.Fatal("empty word list")
	}
	return words[0]
}

func fileBySuffix(t *testing.T, files []JobFile, suffix string) JobFile {
	t.Helper()
	for _, f := range files {
		if strings.HasSuffix(f.Path, suffix) {
			return f
		}
	}
	t.Fatalf("missing file %s in %+v", suffix, files)
	return JobFile{}
}

func TestGroupFilesSeasonNumbers(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Show")
	writeTree(t, lib, []string{
		"Show/Season 01/Show S01E01.mkv",
		"Show/Season 02/Show S02E01.mkv",
		"Show/Season 01/note.nfo",
		"Show/Season 01/poster.jpg",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 1 || got[0].Title != "Show" {
		t.Fatalf("got=%v", titlesOf(got))
	}
	if len(got[0].Files) != 2 {
		t.Fatalf("files=%+v", got[0].Files)
	}
	for _, f := range got[0].Files {
		if strings.HasSuffix(f.Path, ".nfo") || strings.HasSuffix(f.Path, ".jpg") {
			t.Fatalf("non-video in files: %+v", f)
		}
		if f.Path == "Show" || strings.HasSuffix(f.Path, "/") {
			t.Fatalf("directory in files: %+v", f)
		}
	}
	e1 := fileBySuffix(t, got[0].Files, "Show S01E01.mkv")
	if e1.Season != "1" || e1.Episode != "1" {
		t.Fatalf("s01e01=%+v", e1)
	}
	e2 := fileBySuffix(t, got[0].Files, "Show S02E01.mkv")
	if e2.Season != "2" || e2.Episode != "1" {
		t.Fatalf("s02e01=%+v", e2)
	}
}

func TestGroupFilesSequentialEpisodes(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Show")
	writeTree(t, lib, []string{
		"Show/Season 01/a.mkv",
		"Show/Season 01/b.mkv",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 1 {
		t.Fatalf("got=%v", titlesOf(got))
	}
	a := fileBySuffix(t, got[0].Files, "/a.mkv")
	b := fileBySuffix(t, got[0].Files, "/b.mkv")
	if a.Season != "1" || a.Episode != "1" {
		t.Fatalf("a=%+v", a)
	}
	if b.Season != "1" || b.Episode != "2" {
		t.Fatalf("b=%+v", b)
	}
}

func TestGroupFilesFilenameWinsSeason(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Show")
	writeTree(t, lib, []string{
		"Show/Season 02/Show S01E05.mkv",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 1 || len(got[0].Files) != 1 {
		t.Fatalf("got=%+v", got)
	}
	f := got[0].Files[0]
	if f.Season != "1" || f.Episode != "5" {
		t.Fatalf("file=%+v", f)
	}
}

func TestGroupFilesMovieOmitsNumbers(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Show")
	writeTree(t, lib, []string{
		"Show/Title.mkv",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 1 {
		t.Fatalf("got=%v", titlesOf(got))
	}
	if len(got[0].Files) != 1 {
		t.Fatalf("files=%+v", got[0].Files)
	}
	f := got[0].Files[0]
	if f.Season != "" || f.Episode != "" {
		t.Fatalf("expected no numbers: %+v", f)
	}
	if !strings.HasSuffix(f.Path, "Title.mkv") {
		t.Fatalf("path=%q", f.Path)
	}
}

func TestGroupFilesNamedSiblingOwnership(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Show")
	writeTree(t, lib, []string{
		"Show/Season 01/e.mkv",
		"Show/Spin Off/Season 01/e.mkv",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 2 {
		t.Fatalf("got=%v", titlesOf(got))
	}
	var parent, spin *Grouped
	for i := range got {
		if got[i].Title == "Show" {
			parent = &got[i]
		}
		if got[i].Title == "Spin Off" {
			spin = &got[i]
		}
	}
	if parent == nil || spin == nil {
		t.Fatalf("got=%v", titlesOf(got))
	}
	if len(parent.Files) != 1 || strings.Contains(parent.Files[0].Path, "Spin Off") {
		t.Fatalf("parent files=%+v", parent.Files)
	}
	if len(spin.Files) != 1 || !strings.Contains(spin.Files[0].Path, "Spin Off") {
		t.Fatalf("spin files=%+v", spin.Files)
	}
}

func TestGroupKindsExtrasStayOnParent(t *testing.T) {
	cfg := testCfg(t)
	kind := firstWord(t, cfg.GroupKinds())
	extra := firstWord(t, cfg.GroupExtras())
	lib := t.TempDir()
	child := filepath.Join(lib, "Show")
	writeTree(t, lib, []string{
		"Show/Season 01/e.mkv",
		"Show/" + kind + "/k.mkv",
		"Show/" + extra + "/x.mkv",
		"Show/Spin Off/Season 01/e.mkv",
	})
	got := Group(cfg, lib, child)
	var parent, spin *Grouped
	for i := range got {
		switch got[i].Title {
		case "Show":
			parent = &got[i]
		case "Spin Off":
			spin = &got[i]
		default:
			if got[i].Title == kind || got[i].Title == extra {
				t.Fatalf("kinds/extras minted a title: %v", titlesOf(got))
			}
		}
	}
	if parent == nil {
		t.Fatalf("missing parent: %v", titlesOf(got))
	}
	if spin == nil {
		t.Fatalf("missing spin-off: %v", titlesOf(got))
	}
	k := fileBySuffix(t, parent.Files, "/k.mkv")
	if k.Season != "0" || k.Episode == "" {
		t.Fatalf("kind file=%+v", k)
	}
	for _, f := range parent.Files {
		if strings.HasSuffix(f.Path, "/x.mkv") {
			t.Fatalf("extras file kept: %+v", parent.Files)
		}
		if strings.Contains(f.Path, "Spin Off") {
			t.Fatalf("parent leaked spin-off: %+v", parent.Files)
		}
	}
	if parent.Kind != "show" || parent.Role != "title" {
		t.Fatalf("parent kind=%s role=%s", parent.Kind, parent.Role)
	}
}

func TestGroupShowNumbersLooseFile(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Show")
	writeTree(t, lib, []string{"Show/Season 01/plain.mkv"})
	got := Group(testCfg(t), lib, child)
	if len(got) != 1 || got[0].Kind != "show" || len(got[0].Files) != 1 {
		t.Fatalf("got=%+v", got)
	}
	f := got[0].Files[0]
	if f.Season != "1" || f.Episode == "" {
		t.Fatalf("file=%+v", f)
	}
}

func TestGroupOmitsTrailerFilename(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Show")
	writeTree(t, lib, []string{
		"Show/Season 01/Show S01E01.mkv",
		"Show/Season 01/Show - trailer.mkv",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 1 || len(got[0].Files) != 1 {
		t.Fatalf("got=%+v", got)
	}
	if !strings.HasSuffix(got[0].Files[0].Path, "S01E01.mkv") {
		t.Fatalf("file=%+v", got[0].Files[0])
	}
}

func TestGroupFlatMoviesAreFileJobs(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Movies")
	writeTree(t, lib, []string{"Movies/Alpha.mkv", "Movies/Beta.mkv"})
	got := Group(testCfg(t), lib, child)
	if len(got) != 2 {
		t.Fatalf("got=%v", titlesOf(got))
	}
	for _, g := range got {
		if g.Kind != "movie" || g.Role != "title" || len(g.Files) != 1 {
			t.Fatalf("job=%+v", g)
		}
		if !strings.HasSuffix(g.Path, ".mkv") {
			t.Fatalf("path=%s", g.Path)
		}
		if g.Files[0].Season != "" || g.Files[0].Episode != "" {
			t.Fatalf("file=%+v", g.Files[0])
		}
	}
}

func TestGroupVersionFolderOneMovie(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Film (2016)")
	writeTree(t, lib, []string{
		"Film (2016)/Film (2016).mkv",
		"Film (2016)/Film (2016).1080p.mkv",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 1 || got[0].Kind != "movie" || len(got[0].Files) != 2 {
		t.Fatalf("got=%+v", got)
	}
}

func TestGroupExtrasWordInTitleKept(t *testing.T) {
	lib := t.TempDir()
	show := filepath.Join(lib, "Anime", "Extras Sibling Show")
	writeTree(t, lib, []string{
		"Anime/Extras Sibling Show/Extras Sibling Show - 01.mkv",
		"Anime/Extras Sibling Show/Extras Sibling Show - 02.mkv",
		"Anime/Extras Sibling Show/Extras/Extras Sibling Show - NCOP1.mkv",
	})
	cfg := testCfg(t)
	for _, child := range []string{filepath.Join(lib, "Anime"), show} {
		got := Group(cfg, lib, child)
		var job *Grouped
		for i := range got {
			if got[i].Title == "Extras Sibling Show" {
				job = &got[i]
			}
		}
		if job == nil || job.Kind != "show" || len(job.Files) != 2 {
			t.Fatalf("child %s got=%+v", child, got)
		}
		if job.Files[0].Episode != "1" || job.Files[1].Episode != "2" {
			t.Fatalf("files=%+v", job.Files)
		}
	}
}

func TestGroupSeasonPackShow(t *testing.T) {
	lib := t.TempDir()
	show := filepath.Join(lib, "Anime", "Season Pack Show")
	season := filepath.Join(show, "Season 2")
	writeTree(t, lib, []string{
		"Anime/Season Pack Show/Season 2/[Grp]Season Pack Show Season 2_-_01_(Dual).mp4",
		"Anime/Season Pack Show/Season 2/[Grp]Season Pack Show Season 2_-_02_(Dual).mp4",
		"Anime/Season Pack Show/Season 2/[Grp]Season Pack Show Season 2_-_03_(Dual).mp4",
	})
	cfg := testCfg(t)
	for _, child := range []string{filepath.Join(lib, "Anime"), show, season} {
		got := Group(cfg, lib, child)
		if len(got) != 1 || got[0].Title != "Season Pack Show" || got[0].Kind != "show" {
			t.Fatalf("child %s got=%+v", child, got)
		}
		if !strings.HasSuffix(filepath.ToSlash(got[0].Path), "Season Pack Show") {
			t.Fatalf("path=%s", got[0].Path)
		}
		if len(got[0].Files) != 3 {
			t.Fatalf("files=%+v", got[0].Files)
		}
		for i, f := range got[0].Files {
			if f.Season != "2" || f.Episode != strconv.Itoa(i+1) {
				t.Fatalf("file=%+v", f)
			}
		}
	}
}

func TestGroupBareIndexDuplicateOmitted(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Anime", "Dual Show")
	writeTree(t, lib, []string{
		"Anime/Dual Show/Season 1/[Alpha] Dual Show - S01E01 v2.mp4",
		"Anime/Dual Show/Season 1/[Alpha] Dual Show - S01E02 v2.mp4",
		"Anime/Dual Show/Season 1/[Beta] Dual Show - 01.mp4",
		"Anime/Dual Show/Season 1/[Beta] Dual Show - Opening.mp4",
		"Anime/Dual Show/Season 2/S02E01-TAG [AAAAAAAA].mp4",
		"Anime/Dual Show/Season 2/S02E02-TAG [BBBBBBBB].mp4",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 1 || got[0].Kind != "show" || len(got[0].Files) != 4 {
		t.Fatalf("got=%+v", got)
	}
	for _, f := range got[0].Files {
		if strings.Contains(f.Path, "Opening") || strings.Contains(f.Path, "Beta") {
			t.Fatalf("kept %s", f.Path)
		}
	}
}

func TestGroupFlatDashPackIsShow(t *testing.T) {
	lib := t.TempDir()
	show := filepath.Join(lib, "Anime", "Flat Dash Show")
	writeTree(t, lib, []string{
		"Anime/Flat Dash Show/[Grp] Flat Dash Show - 01.mkv",
		"Anime/Flat Dash Show/[Grp] Flat Dash Show - 02.mkv",
		"Anime/Flat Dash Show/[Grp] Flat Dash Show - 03.mkv",
	})
	cfg := testCfg(t)
	for _, child := range []string{filepath.Join(lib, "Anime"), show} {
		got := Group(cfg, lib, child)
		if len(got) != 1 || got[0].Kind != "show" || got[0].Title != "Flat Dash Show" || len(got[0].Files) != 3 {
			t.Fatalf("child %s got=%+v", child, got)
		}
		for i, f := range got[0].Files {
			if f.Season != "1" || f.Episode != strconv.Itoa(i+1) {
				t.Fatalf("file=%+v", f)
			}
		}
	}
}

func TestGroupCourFoldersOneShow(t *testing.T) {
	lib := t.TempDir()
	anime := filepath.Join(lib, "Anime")
	show := filepath.Join(anime, "Complex Show")
	writeTree(t, lib, []string{
		"Anime/Complex Show/Cour One/Complex Show S04E01.mkv",
		"Anime/Complex Show/Cour Two/Complex Show - 01.mkv",
		"Anime/Complex Show/Cour Three/Complex Show - 01.mkv",
		"Anime/Complex Show/OVA/Complex Show OVA - 01.mkv",
	})
	cfg := testCfg(t)
	for _, child := range []string{anime, show} {
		got := Group(cfg, lib, child)
		if len(got) != 1 || got[0].Kind != "show" || got[0].Title != "Complex Show" || len(got[0].Files) != 4 {
			t.Fatalf("child %s got=%+v", child, got)
		}
		if !strings.HasSuffix(filepath.ToSlash(got[0].Path), "Complex Show") {
			t.Fatalf("path=%s", got[0].Path)
		}
		one := fileBySuffix(t, got[0].Files, "Cour One/Complex Show S04E01.mkv")
		two := fileBySuffix(t, got[0].Files, "Cour Two/Complex Show - 01.mkv")
		three := fileBySuffix(t, got[0].Files, "Cour Three/Complex Show - 01.mkv")
		ova := fileBySuffix(t, got[0].Files, "OVA/Complex Show OVA - 01.mkv")
		if one.Season == "4" || two.Season == "4" || three.Season == "4" {
			t.Fatalf("filename season leaked: %+v", got[0].Files)
		}
		if one.Episode != "1" || two.Episode != "1" || three.Episode != "1" || ova.Episode != "1" {
			t.Fatalf("episodes=%+v", got[0].Files)
		}
		if one.Season != "1" || three.Season != "2" || two.Season != "3" || ova.Season != "0" {
			t.Fatalf("seasons one=%s three=%s two=%s ova=%s", one.Season, three.Season, two.Season, ova.Season)
		}
		if child == show && !OneShowAt(lib, show, got) {
			t.Fatalf("show scan is not one title at %s (%s)", show, got[0].Path)
		}
		if child == anime && OneShowAt(lib, anime, got) {
			t.Fatalf("library scan collapsed to %s", got[0].Path)
		}
	}
}

func TestGroupFranchiseFilmsAreOwnMovies(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Halo")
	writeTree(t, lib, []string{
		"Halo/Halo 4 Forward Unto Dawn/Halo 4 Forward Unto Dawn.mkv",
		"Halo/Halo Legends/Halo Legends.mkv",
		"Halo/Halo Nightfall/Halo Nightfall.mkv",
		"Halo/Halo the Fall of Reach/Halo the Fall of Reach.mkv",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 4 {
		t.Fatalf("got=%v", titlesOf(got))
	}
	for _, g := range got {
		if g.Kind != "movie" || g.Role != "title" || len(g.Files) != 1 {
			t.Fatalf("job=%+v", g)
		}
		if g.Title == "Halo" || filepath.ToSlash(g.Path) == "Halo" {
			t.Fatalf("collapsed to parent: %+v", g)
		}
		if g.Title != filepath.Base(g.Path) {
			t.Fatalf("title=%q path=%q", g.Title, g.Path)
		}
	}
}

func TestGroupFranchiseSeriesStaySeparate(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Stargate")
	writeTree(t, lib, []string{
		"Stargate/Stargate Atlantis/Stargate Atlantis S01E01.mkv",
		"Stargate/Stargate Atlantis/Stargate Atlantis S01E02.mkv",
		"Stargate/Stargate SG1/Stargate SG1 S05E01.mkv",
		"Stargate/Stargate Universe/Stargate Universe S01E01.mkv",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 3 {
		t.Fatalf("got=%v", titlesOf(got))
	}
	for _, g := range got {
		if g.Kind != "show" || g.Title == "Stargate" || filepath.ToSlash(g.Path) == "Stargate" {
			t.Fatalf("job=%+v", g)
		}
	}
	atl := groupedByTitle(t, got, "Stargate Atlantis")
	sg1 := groupedByTitle(t, got, "Stargate SG1")
	uni := groupedByTitle(t, got, "Stargate Universe")
	if len(atl.Files) != 2 || len(sg1.Files) != 1 || len(uni.Files) != 1 {
		t.Fatalf("files atl=%d sg1=%d uni=%d", len(atl.Files), len(sg1.Files), len(uni.Files))
	}
	a1 := fileBySuffix(t, atl.Files, "S01E01.mkv")
	if a1.Season != "1" || a1.Episode != "1" {
		t.Fatalf("atlantis=%+v", a1)
	}
	s5 := fileBySuffix(t, sg1.Files, "S05E01.mkv")
	if s5.Season != "5" || s5.Episode != "1" {
		t.Fatalf("sg1=%+v", s5)
	}
}

func TestGroupSpinoffDoesNotContinueEpisodes(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Barakamon")
	files := []string{
		"Barakamon/Handa-kun/Barakamon Handa-kun E01.mkv",
		"Barakamon/Handa-kun/Barakamon Handa-kun E02.mkv",
	}
	for i := 1; i <= 3; i++ {
		files = append(files, filepath.ToSlash(filepath.Join("Barakamon", "Season 1", "Barakamon S01E"+strconv.Itoa(i)+".mkv")))
	}
	writeTree(t, lib, files)
	got := Group(testCfg(t), lib, child)
	show := groupedByTitle(t, got, "Barakamon")
	spin := groupedByTitle(t, got, "Handa-kun")
	if show.Kind != "show" || spin.Kind != "show" {
		t.Fatalf("kinds show=%s spin=%s", show.Kind, spin.Kind)
	}
	if !strings.HasSuffix(filepath.ToSlash(spin.Path), "Barakamon/Handa-kun") {
		t.Fatalf("spin path=%s", spin.Path)
	}
	if len(show.Files) != 3 || len(spin.Files) != 2 {
		t.Fatalf("show=%d spin=%d", len(show.Files), len(spin.Files))
	}
	e1 := fileBySuffix(t, spin.Files, "E01.mkv")
	e2 := fileBySuffix(t, spin.Files, "E02.mkv")
	if e1.Season != "1" || e1.Episode != "1" || e2.Episode != "2" {
		t.Fatalf("spin files=%+v", spin.Files)
	}
	for _, f := range show.Files {
		if strings.Contains(f.Path, "Handa-kun") {
			t.Fatalf("parent kept spinoff: %+v", show.Files)
		}
	}
}

func TestGroupDashSubtitleIsNextSeason(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Knights of Sidonia")
	writeTree(t, lib, []string{
		"Knights of Sidonia/Sidonia no Kishi/Sidonia no Kishi - 01.mkv",
		"Knights of Sidonia/Sidonia no Kishi/Sidonia no Kishi - 02.mkv",
		"Knights of Sidonia/Sidonia no Kishi - Daikyuu Wakusei Sen'eki/Sidonia no Kishi - Daikyuu Wakusei Sen'eki - 01.mkv",
		"Knights of Sidonia/Sidonia no Kishi - Daikyuu Wakusei Sen'eki/Sidonia no Kishi - Daikyuu Wakusei Sen'eki - 02.mkv",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 1 || got[0].Kind != "show" || got[0].Title != "Sidonia no Kishi" {
		t.Fatalf("got=%+v", got)
	}
	if len(got[0].Files) != 4 {
		t.Fatalf("files=%+v", got[0].Files)
	}
	s1 := fileBySuffix(t, got[0].Files, "Sidonia no Kishi/Sidonia no Kishi - 01.mkv")
	s2 := fileBySuffix(t, got[0].Files, "Sen'eki/Sidonia no Kishi - Daikyuu Wakusei Sen'eki - 01.mkv")
	if s1.Season != "1" || s1.Episode != "1" {
		t.Fatalf("s1=%+v", s1)
	}
	if s2.Season != "2" || s2.Episode != "1" {
		t.Fatalf("s2=%+v", s2)
	}
}

func TestGroupSharedPrefixIsSeparateShow(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Miss Kobayashi's Dragon Maid")
	writeTree(t, lib, []string{
		"Miss Kobayashi's Dragon Maid/Kobayashi-san Chi no Maid Dragon/Kobayashi-san Chi no Maid Dragon - 01.mkv",
		"Miss Kobayashi's Dragon Maid/Kobayashi-san Chi no Maid Dragon/Kobayashi-san Chi no Maid Dragon - 02.mkv",
		"Miss Kobayashi's Dragon Maid/Kobayashi-san Chi no OO Dragon/Kobayashi-san Chi no OO Dragon - 01.mkv",
		"Miss Kobayashi's Dragon Maid/Kobayashi-san Chi no OO Dragon/Kobayashi-san Chi no OO Dragon - 02.mkv",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 2 {
		t.Fatalf("got=%v", titlesOf(got))
	}
	maid := groupedByTitle(t, got, "Kobayashi-san Chi no Maid Dragon")
	oo := groupedByTitle(t, got, "Kobayashi-san Chi no OO Dragon")
	if maid.Kind != "show" || oo.Kind != "show" {
		t.Fatalf("maid=%s oo=%s", maid.Kind, oo.Kind)
	}
	if filepath.ToSlash(maid.Path) == "Miss Kobayashi's Dragon Maid" || filepath.ToSlash(oo.Path) == "Miss Kobayashi's Dragon Maid" {
		t.Fatalf("paths maid=%s oo=%s", maid.Path, oo.Path)
	}
	ep := fileBySuffix(t, oo.Files, "OO Dragon - 01.mkv")
	if ep.Season != "1" || ep.Episode != "1" {
		t.Fatalf("oo=%+v", ep)
	}
}

func TestGroupMovieContainerIsOwnFilm(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Chainsaw Man")
	writeTree(t, lib, []string{
		"Chainsaw Man/Season 1/Chainsaw Man S01E01.mkv",
		"Chainsaw Man/Movies/[Judas] Chainsaw Man The Movie.mkv",
		"Chainsaw Man/OVA/Chainsaw Man OVA - 01.mkv",
		"Chainsaw Man/movies/Made In Abyss - Dawn Of The Deep Soul.mkv",
	})
	got := Group(testCfg(t), lib, child)
	show := groupedByTitle(t, got, "Chainsaw Man")
	if show.Kind != "show" {
		t.Fatalf("show=%+v", show)
	}
	ep := fileBySuffix(t, show.Files, "S01E01.mkv")
	ova := fileBySuffix(t, show.Files, "OVA - 01.mkv")
	if ep.Season != "1" || ep.Episode != "1" || ova.Season != "0" {
		t.Fatalf("show files=%+v", show.Files)
	}
	for _, f := range show.Files {
		if strings.Contains(f.Path, "The Movie") || strings.Contains(f.Path, "Dawn Of The Deep Soul") {
			t.Fatalf("film folded onto show: %+v", show.Files)
		}
	}
	film := groupedByTitle(t, got, "Chainsaw Man The Movie")
	dawn := groupedByTitle(t, got, "Made In Abyss Dawn Of The Deep Soul")
	if film.Kind != "movie" || film.Role != "title" || len(film.Files) != 1 {
		t.Fatalf("film=%+v", film)
	}
	if dawn.Kind != "movie" || dawn.Role != "title" {
		t.Fatalf("dawn=%+v", dawn)
	}
	if film.Files[0].Season != "" || dawn.Files[0].Season != "" {
		t.Fatalf("film numbers film=%+v dawn=%+v", film.Files, dawn.Files)
	}
}

func TestGroupLooseFilmKeepsItsTitle(t *testing.T) {
	lib := t.TempDir()
	writeTree(t, lib, []string{
		"Jujutsu Kaisen/Season 1/Jujutsu Kaisen S01E01.mkv",
		"Jujutsu Kaisen/Jujutsu Kaisen 0.mkv",
		"DanMachi/Season 1/DanMachi S01E01.mkv",
		"DanMachi/Arrow of the Orion/Arrow of the Orion.mkv",
		"Black Clover/Season 1/Black Clover S01E01.mkv",
		"Black Clover/Black Clover Sword of the Wizard King (2023).mkv",
	})
	cfg := testCfg(t)
	jjk := Group(cfg, lib, filepath.Join(lib, "Jujutsu Kaisen"))
	zero := groupedByTitle(t, jjk, "Jujutsu Kaisen 0")
	if zero.Kind != "movie" || len(zero.Files) != 1 {
		t.Fatalf("zero=%+v", zero)
	}
	dan := Group(cfg, lib, filepath.Join(lib, "DanMachi"))
	arrow := groupedByTitle(t, dan, "Arrow of the Orion")
	if arrow.Kind != "movie" || !strings.HasSuffix(filepath.ToSlash(arrow.Path), "Arrow of the Orion") {
		t.Fatalf("arrow=%+v", arrow)
	}
	clover := Group(cfg, lib, filepath.Join(lib, "Black Clover"))
	sword := groupedByTitle(t, clover, "Black Clover Sword of the Wizard King")
	if sword.Kind != "movie" || sword.Year != "2023" {
		t.Fatalf("sword=%+v", sword)
	}
}

func TestGroupPackedSeasonEpisode(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "Detectorists")
	writeTree(t, lib, []string{
		"Detectorists/Season 1/Detectorists 101.mp4",
		"Detectorists/Season 1/Detectorists 102.mp4",
		"Detectorists/Season 1/Detectorists 106.mp4",
		"Detectorists/Season 1/FlashForward S01E01.mkv",
		"Detectorists/Season 1/Detectorists 201.mp4",
		"Detectorists/Season 2/Detectorists 201.mp4",
		"Detectorists/Season 2/Detectorists 207.mp4",
	})
	got := Group(testCfg(t), lib, child)
	if len(got) != 1 || got[0].Kind != "show" {
		t.Fatalf("got=%+v", got)
	}
	e101 := fileBySuffix(t, got[0].Files, "Season 1/Detectorists 101.mp4")
	e102 := fileBySuffix(t, got[0].Files, "Season 1/Detectorists 102.mp4")
	e106 := fileBySuffix(t, got[0].Files, "Season 1/Detectorists 106.mp4")
	flash := fileBySuffix(t, got[0].Files, "FlashForward S01E01.mkv")
	mis := fileBySuffix(t, got[0].Files, "Season 1/Detectorists 201.mp4")
	e201 := fileBySuffix(t, got[0].Files, "Season 2/Detectorists 201.mp4")
	e207 := fileBySuffix(t, got[0].Files, "Season 2/Detectorists 207.mp4")
	if e101.Season != "1" || e101.Episode != "1" {
		t.Fatalf("101=%+v", e101)
	}
	if e102.Episode != "2" || e106.Episode != "6" {
		t.Fatalf("102=%+v 106=%+v", e102, e106)
	}
	if flash.Season != "1" || flash.Episode != "1" {
		t.Fatalf("sxxe=%+v", flash)
	}
	if mis.Season != "1" || mis.Episode != "201" {
		t.Fatalf("mismatched hundreds=%+v", mis)
	}
	if e201.Season != "2" || e201.Episode != "1" || e207.Episode != "7" {
		t.Fatalf("201=%+v 207=%+v", e201, e207)
	}
}

func groupedByTitle(t *testing.T, got []Grouped, title string) Grouped {
	t.Helper()
	for _, g := range got {
		if g.Title == title {
			return g
		}
	}
	t.Fatalf("missing %q in %v", title, titlesOf(got))
	return Grouped{}
}
