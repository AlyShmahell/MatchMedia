package sidecar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	st, err := Write(context.Background(), fetch.New(cfg), []match.Job{job})
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
	out, st, err := WriteProgress(context.Background(), fetch.New(cfg), "rescan", []match.Job{job}, nil)
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
	if _, st, err := WriteProgress(context.Background(), fetch.New(cfg), "nfo", []match.Job{job}, nil); err != nil || st != (Stats{}) {
		t.Fatalf("nfo %+v %v", st, err)
	}
	body, err := os.ReadFile(nfo)
	if err != nil || !strings.Contains(string(body), "<title>Keep</title>") {
		t.Fatalf("nfo mode rewrote %s", body)
	}
	if _, st, err := WriteProgress(context.Background(), fetch.New(cfg), "changes", []match.Job{job}, nil); err != nil || st != (Stats{}) {
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
	st, err := Write(context.Background(), fetch.New(cfg), []match.Job{job})
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

func TestChangesWritesEpisodeGaps(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "Show.S01E01.mkv")
	keepVideo := filepath.Join(dir, "Show.S01E02.mkv")
	if err := os.WriteFile(video, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keepVideo, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	kept := `<episodedetails><title>Stay</title></episodedetails>`
	if err := os.WriteFile(filepath.Join(dir, "Show.S01E02.nfo"), []byte(kept), 0o644); err != nil {
		t.Fatal(err)
	}
	nfo := filepath.Join(dir, "tvshow.nfo")
	body := "<tvshow><title>Keep Title</title><plot>Keep Plot</plot></tvshow>\n"
	if err := os.WriteFile(nfo, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "poster.jpg"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{
		Provider: "tmdb", ID: "7", NFO: "tvshow", Title: "Other", Synopsis: "Other plot", UniqueID: "tmdb-tv",
		Requests: []rank.Request{
			{Section: "detail", Provider: "tmdb", NFO: "tvshow", URL: "https://api.themoviedb.org/3/tv/7"},
			{Section: "detail", Provider: "omdb", NFO: "movie", URL: "https://www.omdbapi.com", Query: "i=tt0133093&plot=short"},
		},
	}
	job := match.Job{
		Path: dir, Kind: "show", Status: "matched", Ranker: "set", Match: &hit, CatalogFor: hit.Key(),
		Files: []match.JobFile{{Path: video, Season: "1", Episode: "1"}, {Path: keepVideo, Season: "1", Episode: "2"}},
		Catalog: []match.CatalogSeason{{
			Number: "1",
			Episodes: []fetch.Episode{
				{Number: "1", Title: "Pilot", Path: video},
				{Number: "2", Title: "Next", Path: keepVideo},
			},
		}},
	}
	out, st, err := WriteProgress(context.Background(), fetch.New(cfg), "changes", []match.Job{job}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.EpisodeNFO != 1 || st.Posters != 0 || st.TitleNFO != 1 {
		t.Fatalf("stats %+v", st)
	}
	got, err := os.ReadFile(nfo)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{"<title>Keep Title</title>", "<plot>Keep Plot</plot>", "https://api.themoviedb.org/3/tv/7", `query="i=tt0133093&amp;plot=short"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in %s", want, text)
		}
	}
	if strings.Contains(text, "api_key") || strings.Contains(text, "apikey") {
		t.Fatalf("key in %s", text)
	}
	if strings.Contains(text, "Other plot") {
		t.Fatalf("rewrote plot %s", text)
	}
	poster, err := os.ReadFile(filepath.Join(dir, "poster.jpg"))
	if err != nil || string(poster) != "keep" {
		t.Fatalf("poster %q %v", poster, err)
	}
	ep, err := os.ReadFile(filepath.Join(dir, "Show.S01E01.nfo"))
	if err != nil || !strings.Contains(string(ep), "<title>Pilot</title>") {
		t.Fatalf("episode %s %v", ep, err)
	}
	stay, err := os.ReadFile(filepath.Join(dir, "Show.S01E02.nfo"))
	if err != nil || string(stay) != kept {
		t.Fatalf("kept episode %s", stay)
	}
	if out[0].NFOPath != nfo {
		t.Fatalf("nfo path %q", out[0].NFOPath)
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
	st, err := Write(context.Background(), fetch.New(cfg), []match.Job{job})
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
	st, err := Write(context.Background(), fetch.New(cfg), []match.Job{job})
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

func TestWriteEpisodeThumb(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("still"))
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	video := filepath.Join(dir, "Show.S01E01.mkv")
	if err := os.WriteFile(video, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{Provider: "tvmaze", ID: "7", Title: "Show", Poster: srv.URL + "/show.jpg"}
	job := match.Job{
		Path: dir, Kind: "show", Status: "matched", Match: &hit, CatalogFor: hit.Key(),
		Catalog: []match.CatalogSeason{{
			Number: "1", Poster: srv.URL + "/season.jpg",
			Episodes: []fetch.Episode{{Number: "1", Title: "Pilot", Path: video, Poster: srv.URL + "/ep.jpg"}},
		}},
	}
	client := fetch.New(cfg)
	Prefetch(context.Background(), client, job, "")
	if hits != 2 {
		t.Fatalf("prefetch hits %d", hits)
	}
	Prefetch(context.Background(), client, job, "")
	if hits != 2 {
		t.Fatalf("second prefetch hits %d", hits)
	}
	st, err := Write(context.Background(), client, []match.Job{job})
	if err != nil {
		t.Fatal(err)
	}
	if st.Posters != 2 || hits != 2 {
		t.Fatalf("stats %+v hits %d", st, hits)
	}
	body, err := os.ReadFile(filepath.Join(dir, "Show.S01E01-thumb.jpg"))
	if err != nil || string(body) != "still" {
		t.Fatalf("thumb %q %v", body, err)
	}
}

func TestPrefetchSkipsExistingPoster(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		_, _ = w.Write([]byte("img"))
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "poster.jpg"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{Provider: "tvmaze", ID: "7", Title: "Show", Poster: srv.URL + "/show.jpg"}
	job := match.Job{Path: dir, Kind: "show", Status: "matched", Match: &hit}
	Prefetch(context.Background(), fetch.New(cfg), job, "")
	mu.Lock()
	got := hits
	mu.Unlock()
	if got != 0 {
		t.Fatalf("hits %d", got)
	}
}
