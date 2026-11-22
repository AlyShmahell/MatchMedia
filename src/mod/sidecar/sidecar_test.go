package sidecar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/match"
)

func TestTitleNFOAndComplete(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Title.mkv")
	if err := os.WriteFile(video, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	job := match.Job{Path: video, Kind: "movie", Files: []match.JobFile{{Path: video}}}
	if TitleNFO(job) {
		t.Fatal("no nfo yet")
	}
	nfo := filepath.Join(dir, "Title.nfo")
	if err := os.WriteFile(nfo, []byte(`<movie><title>Title</title><year>1999</year></movie>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !TitleNFO(job) {
		t.Fatal("want title nfo")
	}
	if Complete(job, false) {
		t.Fatal("complete needs poster")
	}
	if err := os.WriteFile(filepath.Join(dir, "Title-poster.jpg"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !Complete(job, false) {
		t.Fatal("want complete")
	}
	got := FromSidecar(job)
	if got.Status != "matched" || got.Ranker != "sidecar" || got.Title != "Title" || got.Year != "1999" {
		t.Fatalf("%#v", got)
	}
	if got.Match == nil || got.Match.Score != 0 || got.Match.Jaccard != 0 {
		t.Fatalf("placeholder score %#v", got.Match)
	}
	poster := filepath.Join(dir, "Title-poster.jpg")
	if got.NFOPath != nfo || got.PosterPath != poster {
		t.Fatalf("paths nfo=%q poster=%q", got.NFOPath, got.PosterPath)
	}
	if got.Match == nil || got.Match.Poster != "" {
		t.Fatalf("poster url %#v", got.Match)
	}
	if len(got.Files) != 1 || got.Files[0].PosterPath == "" {
		t.Fatalf("thumb %#v", got.Files)
	}
}

func TestCompleteRequiresSeasonSidecar(t *testing.T) {
	dir := t.TempDir()
	show := filepath.Join(dir, "Show")
	ep := filepath.Join(show, "Season 1", "Show.S01E01.mkv")
	if err := os.MkdirAll(filepath.Dir(ep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ep, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "tvshow.nfo"), []byte("<tvshow><title>Show</title></tvshow>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "poster.jpg"), []byte("p"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(ep, filepath.Ext(ep))+".nfo", []byte("<episodedetails><title>Ep</title></episodedetails>"), 0o644); err != nil {
		t.Fatal(err)
	}
	job := match.Job{Path: show, Kind: "show", Files: []match.JobFile{{Path: ep}}}
	if !Complete(job, false) {
		t.Fatal("title sidecar")
	}
	if Complete(job, true) {
		t.Fatal("missing season files")
	}
	if err := os.WriteFile(filepath.Join(show, "Season 1", "season.nfo"), []byte("<season><title>Season 1</title></season>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if Complete(job, true) {
		t.Fatal("missing season poster")
	}
	if err := os.WriteFile(filepath.Join(show, "Season 1", "poster.jpg"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Complete(job, true) {
		t.Fatal("season sidecar")
	}
}
