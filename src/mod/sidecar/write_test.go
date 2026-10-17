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

func TestWriteStampsPathsAndFacts(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "Title.mkv")
	if err := os.WriteFile(video, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{
		Provider: "tmdb", ID: "129", Title: "Title", Year: "2001", UniqueID: "tmdb-movie",
		Synopsis: "A plot",
		Attrs:    map[string]string{"genres": "Drama, Mystery", "rating": "8.2", "runtime": "90", "language": "ja"},
	}
	job := match.Job{
		Path: video, Kind: "movie", Status: "matched", Match: &hit,
		Files: []match.JobFile{{Path: video}},
	}
	out, st, err := WriteProgress(context.Background(), cfg, "rescan", []match.Job{job}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.TitleNFO != 1 || len(out) != 1 || out[0].NFOPath != filepath.Join(dir, "Title.nfo") {
		t.Fatalf("stats %+v job %#v", st, out)
	}
	if out[0].Files[0].PosterPath != filepath.Join(dir, "Title-thumb.jpg") {
		t.Fatalf("thumb %q", out[0].Files[0].PosterPath)
	}
	body, err := os.ReadFile(out[0].NFOPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	for _, want := range []string{"<genre>Drama</genre>", "<genre>Mystery</genre>", "<rating>8.2</rating>", "<runtime>90</runtime>", "<language>ja</language>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	back := FromSidecar(out[0])
	if back.Match == nil || back.Match.Attrs["rating"] != "8.2" || back.Match.Attrs["genres"] != "Drama, Mystery" {
		t.Fatalf("attrs %#v", back.Match)
	}
}

func TestWriteModeSkipsNFOAndCompleteChanges(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "Title.mkv")
	if err := os.WriteFile(video, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	nfo := filepath.Join(dir, "Title.nfo")
	if err := os.WriteFile(nfo, []byte(`<movie><title>Keep</title><year>1999</year></movie>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Title-poster.jpg"), []byte("art"), 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{Provider: "tmdb", ID: "1", Title: "Other", UniqueID: "tmdb-movie"}
	job := FromSidecar(match.Job{Path: video, Kind: "movie", Files: []match.JobFile{{Path: video}}})
	job.Match = &hit
	job.Status = "matched"
	if _, st, err := WriteProgress(context.Background(), cfg, "nfo", []match.Job{job}, nil); err != nil || st != (Stats{}) {
		t.Fatalf("nfo %+v %v", st, err)
	}
	body, err := os.ReadFile(nfo)
	if err != nil || !strings.Contains(string(body), "<title>Keep</title>") {
		t.Fatalf("nfo mode rewrote %s", body)
	}
	if _, st, err := WriteProgress(context.Background(), cfg, "changes", []match.Job{job}, nil); err != nil || st != (Stats{}) {
		t.Fatalf("changes %+v %v", st, err)
	}
	body, err = os.ReadFile(nfo)
	if err != nil || !strings.Contains(string(body), "<title>Keep</title>") {
		t.Fatalf("changes rewrote %s", body)
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
