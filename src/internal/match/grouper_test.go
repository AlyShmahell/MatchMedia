package match

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/alyshmahell/matchmedia/lib/config"
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
	if k.Season != "0" {
		t.Fatalf("kind file=%+v", k)
	}
	x := fileBySuffix(t, parent.Files, "/x.mkv")
	if x.Season != "0" {
		t.Fatalf("extras file=%+v", x)
	}
	for _, f := range parent.Files {
		if strings.Contains(f.Path, "Spin Off") {
			t.Fatalf("parent leaked spin-off: %+v", parent.Files)
		}
	}
}
