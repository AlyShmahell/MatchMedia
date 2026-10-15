package sidecar

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
	"github.com/alyshmahell/matchmedia/src/mod/fetch"
	"github.com/alyshmahell/matchmedia/src/mod/match"
)

func TestWriteMovieFile(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "Title.mkv")
	if err := os.WriteFile(video, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{Provider: "tmdb", ID: "129", Title: "Title", Year: "2001", UniqueID: "tmdb-movie"}
	job := match.Job{
		Path:   video,
		Kind:   "movie",
		Status: "matched",
		Match:  &hit,
		Files:  []match.JobFile{{Path: video}},
	}
	st, err := Write(context.Background(), cfg, []match.Job{job})
	if err != nil {
		t.Fatal(err)
	}
	if st.TitleNFO != 1 || st.EpisodeNFO != 0 || st.Posters != 0 {
		t.Fatalf("stats %+v", st)
	}
	body, err := os.ReadFile(filepath.Join(dir, "Title.nfo"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "<movie>") || !strings.Contains(got, "<title>Title</title>") {
		t.Fatalf("nfo %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "movie.nfo")); err == nil {
		t.Fatal("file job should not write movie.nfo")
	}
}

func TestWriteShowDirEpisodeNFO(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "Show.S01E01.mkv")
	if err := os.WriteFile(video, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{Provider: "tmdb", ID: "139", Title: "Show", Year: "2012", UniqueID: "tmdb-tv"}
	job := match.Job{
		Path:       dir,
		Kind:       "show",
		Status:     "matched",
		Match:      &hit,
		CatalogFor: hit.Key(),
		Files:      []match.JobFile{{Path: video, Season: "1", Episode: "1"}},
		Catalog: []match.CatalogSeason{{
			Number: "1",
			Episodes: []fetch.Episode{{
				Number: "1",
				Title:  "Pilot",
				Path:   video,
			}},
		}},
	}
	st, err := Write(context.Background(), cfg, []match.Job{job})
	if err != nil {
		t.Fatal(err)
	}
	if st.TitleNFO != 1 || st.EpisodeNFO != 1 || st.Posters != 0 {
		t.Fatalf("stats %+v", st)
	}
	body, err := os.ReadFile(filepath.Join(dir, "tvshow.nfo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "<tvshow>") {
		t.Fatalf("tvshow.nfo %s", body)
	}
	ep, err := os.ReadFile(filepath.Join(dir, "Show.S01E01.nfo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ep), "<episodedetails>") || !strings.Contains(string(ep), "<title>Pilot</title>") {
		t.Fatalf("episode nfo %s", ep)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if e.IsDir() && strings.HasPrefix(e.Name(), "Season") {
			t.Fatalf("catalog season dir %s", e.Name())
		}
	}
}

func TestWriteSkipsEmptyPath(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "Title.mkv")
	if err := os.WriteFile(video, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{Provider: "tmdb", ID: "1", Title: "Title", UniqueID: "tmdb-movie"}
	job := match.Job{
		Kind:   "movie",
		Status: "matched",
		Match:  &hit,
		Files:  []match.JobFile{{Path: video}},
	}
	st, err := Write(context.Background(), cfg, []match.Job{job})
	if err != nil {
		t.Fatal(err)
	}
	if st != (Stats{}) {
		t.Fatalf("stats %+v", st)
	}
	if _, err := os.Stat(filepath.Join(dir, "Title.nfo")); !os.IsNotExist(err) {
		t.Fatal("wrote nfo without job.Path")
	}
}

func TestWriteKeepsExistingPoster(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "Title.mkv")
	if err := os.WriteFile(video, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	poster := filepath.Join(dir, "Title-poster.jpg")
	if err := os.WriteFile(poster, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{
		Provider: "tmdb",
		ID:       "129",
		Title:    "Title",
		Poster:   "http://127.0.0.1:1/missing.jpg",
		UniqueID: "tmdb-movie",
	}
	job := match.Job{
		Path:   video,
		Kind:   "movie",
		Status: "matched",
		Match:  &hit,
		Files:  []match.JobFile{{Path: video}},
	}
	st, err := Write(context.Background(), cfg, []match.Job{job})
	if err != nil {
		t.Fatal(err)
	}
	if st.TitleNFO != 1 || st.Posters != 0 {
		t.Fatalf("stats %+v", st)
	}
	body, err := os.ReadFile(poster)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "keep" {
		t.Fatalf("poster %q", body)
	}
}
