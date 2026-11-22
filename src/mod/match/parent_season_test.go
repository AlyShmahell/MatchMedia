package match

import (
	"path/filepath"
	"testing"
)

func TestSeasonFolder(t *testing.T) {
	cfg := foldCfg(t)
	show := filepath.Clean("/media/Show")
	nested := filepath.Join(show, "Season 12", "Disc 1", "ep.mkv")
	if got := SeasonFolder(cfg, show, nested, "12"); got != filepath.Join(show, "Season 12") {
		t.Fatalf("nested %s", got)
	}
	if got := SeasonFolder(cfg, show, nested, "3"); got != "" {
		t.Fatalf("other number %s", got)
	}
	named := filepath.Join(show, "Season 12", "Show.S03E01.mkv")
	if got := SeasonFolder(cfg, show, named, "3"); got != "" {
		t.Fatalf("filename token %s", got)
	}
	longer := filepath.Join(show, "Archive Season 03 Extra", "ep.mkv")
	if got := SeasonFolder(cfg, show, longer, "3"); got != filepath.Dir(longer) {
		t.Fatalf("extra words %s", got)
	}
	if got := SeasonFolder(cfg, show, longer, "03"); got != filepath.Dir(longer) {
		t.Fatalf("padded number %s", got)
	}
	if got := SeasonFolder(cfg, show, filepath.Join(show, "ep.mkv"), "3"); got != "" {
		t.Fatalf("flat %s", got)
	}
	if got := SeasonFolder(cfg, show, "/other/Season 12/ep.mkv", "12"); got != "" {
		t.Fatalf("outside %s", got)
	}
	if got := SeasonFolder(cfg, show, filepath.Join(show, "Archive", "ep.mkv"), "8"); got != "" {
		t.Fatalf("no token %s", got)
	}
	first := filepath.Join(show, "Season 03 S12", "ep.mkv")
	if got := SeasonFolder(cfg, show, first, "3"); got != filepath.Dir(first) {
		t.Fatalf("first token %s", got)
	}
	if got := SeasonFolder(cfg, show, first, "12"); got != "" {
		t.Fatalf("later token %s", got)
	}
	for _, name := range []string{"Specials", "Movies", "OVAs"} {
		p := filepath.Join(show, name, "ep.mkv")
		if got := SeasonFolder(cfg, show, p, "0"); got != filepath.Dir(p) {
			t.Fatalf("%s %s", name, got)
		}
		if got := SeasonFolder(cfg, show, p, "4"); got != "" {
			t.Fatalf("%s as other season %s", name, got)
		}
	}
}
