package group

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/config"
)

func TestGroupLibrary(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	lib := New(cfg)
	root := t.TempDir()

	show := filepath.Join(root, "Example Show")
	touch(t, filepath.Join(show, "Season 1", "Example Show S01E01.mkv"))
	touch(t, filepath.Join(show, "Season 1", "Example Show S01E02.mkv"))
	touch(t, filepath.Join(show, "My Movie", "My Movie Title.mkv"))
	touch(t, filepath.Join(show, "Example Show S01E03.mkv"))
	touch(t, filepath.Join(show, "Totally Unrelated Film.mkv"))
	touch(t, filepath.Join(show, "notes.txt"))

	got, err := lib.Group(show)
	if err != nil {
		t.Fatal(err)
	}
	want := []Title{
		{Name: "My Movie", Files: []string{filepath.Join("My Movie", "My Movie Title.mkv")}},
		{Name: "Example Show", Files: []string{
			filepath.Join("Season 1", "Example Show S01E01.mkv"),
			filepath.Join("Season 1", "Example Show S01E02.mkv"),
			"Example Show S01E03.mkv",
		}},
		{Name: "Totally Unrelated Film", Files: []string{"Totally Unrelated Film.mkv"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestSeasonPattern(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	lib := New(cfg)
	for _, name := range []string{"Season 1", "OVAs", "Specials", "Extras", "s01"} {
		if !lib.Season.MatchString(name) {
			t.Fatalf("%q should match season_pattern", name)
		}
	}
	for _, name := range []string{"Show", "Behind The Scenes", "s123"} {
		if lib.Season.MatchString(name) {
			t.Fatalf("%q should not match season_pattern", name)
		}
	}
}

func TestNamedClusters(t *testing.T) {
	got := namedClusters(
		[]string{"a/one.mkv", "a/two.mkv", "b/solo.mkv"},
		[]int{1, 1, 0},
		"Parent",
	)
	want := []Title{
		{Name: "Parent", Files: []string{"a/one.mkv", "a/two.mkv"}},
		{Name: "solo", Files: []string{"b/solo.mkv"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestClusterThreshold(t *testing.T) {
	same := Clusterer{Threshold: 0.49}.Label([][]float64{{1, 0}, {1, 0}, {0, 1}})
	if same[0] != same[1] || same[0] == same[2] {
		t.Fatalf("%v", same)
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}
