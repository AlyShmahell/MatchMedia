package sidecar

import (
	"os"
	"path/filepath"
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
	if got.Status != "matched" || got.Title != "Title" || got.Year != "1999" {
		t.Fatalf("%#v", got)
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
