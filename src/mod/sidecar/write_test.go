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
				Year:   "2012",
				Aired:  "2012-04-05",
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
	if !strings.Contains(string(ep), "<aired>2012-04-05</aired>") || !strings.Contains(string(ep), "<year>2012</year>") {
		t.Fatalf("episode aired/year %s", ep)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if e.IsDir() && strings.HasPrefix(e.Name(), "Season") {
			t.Fatalf("catalog season dir %s", e.Name())
		}
		if e.Name() == "season.nfo" {
			t.Fatal("season nfo in show dir")
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
	if _, err := os.Stat(filepath.Join(dir, "season.nfo")); !os.IsNotExist(err) {
		t.Fatalf("season nfo in show dir: %v", err)
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

func TestWriteSeasonFolders(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	t.Cleanup(srv.Close)
	show := t.TempDir()
	ep12 := filepath.Join(show, "Season 12", "Disc 1", "ep.mkv")
	ep3 := filepath.Join(show, "Archive Season 03 Extra", "ep.mkv")
	ep7 := filepath.Join(show, "S07", "ep.mkv")
	for _, p := range []string{ep12, ep3, ep7} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hit := rank.Hit{Provider: "tmdb", ID: "9", Title: "Show", Poster: srv.URL + "/show.jpg", UniqueID: "tmdb-tv"}
	job := match.Job{
		Path: show, Kind: "show", Status: "matched", Match: &hit, CatalogFor: hit.Key(),
		Catalog: []match.CatalogSeason{
			{
				Number: "12", Title: "Show", Synopsis: "Cold", Year: "2019", Poster: srv.URL + "/s12.jpg",
				Episodes: []fetch.Episode{{Number: "1", Title: "One", Path: ep12}},
			},
			{
				Number: "03", Title: "  ", Poster: srv.URL + "/s3.jpg",
				Episodes: []fetch.Episode{{Number: "1", Title: "Two", Path: ep3, Poster: srv.URL + "/thumb.jpg"}},
			},
			{
				Number: "7", Title: "The Long Night", Poster: srv.URL + "/s7.jpg",
				Episodes: []fetch.Episode{{Number: "1", Title: "Three", Path: ep7}},
			},
		},
	}
	out, st, err := WriteProgress(context.Background(), fetch.New(cfg), "", []match.Job{job}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.TitleNFO != 4 || st.EpisodeNFO != 3 || st.Posters != 5 {
		t.Fatalf("stats %+v", st)
	}
	if out[0].NFOPath != filepath.Join(show, "tvshow.nfo") || out[0].PosterPath != filepath.Join(show, "poster.jpg") {
		t.Fatalf("show paths nfo %q poster %q", out[0].NFOPath, out[0].PosterPath)
	}
	if body := readFile(t, filepath.Join(show, "poster.jpg")); body != "/show.jpg" {
		t.Fatalf("show poster %q", body)
	}
	if _, err := os.Stat(filepath.Join(show, "season.nfo")); !os.IsNotExist(err) {
		t.Fatalf("season nfo in show dir: %v", err)
	}
	s12 := readFile(t, filepath.Join(show, "Season 12", "season.nfo"))
	for _, want := range []string{"<title>Season 12</title>", "<seasonnumber>12</seasonnumber>", "<plot>Cold</plot>", "<year>2019</year>"} {
		if !strings.Contains(s12, want) {
			t.Fatalf("missing %s in %s", want, s12)
		}
	}
	if strings.Contains(s12, "<title>Show</title>") {
		t.Fatalf("show name kept %s", s12)
	}
	if body := readFile(t, filepath.Join(show, "Season 12", "poster.jpg")); body != "/s12.jpg" {
		t.Fatalf("season 12 poster %q", body)
	}
	if _, err := os.Stat(filepath.Join(show, "Season 12", "Disc 1", "poster.jpg")); !os.IsNotExist(err) {
		t.Fatalf("poster in nested dir: %v", err)
	}
	s3 := readFile(t, filepath.Join(show, "Archive Season 03 Extra", "season.nfo"))
	for _, want := range []string{"<title>Season 3</title>", "<seasonnumber>3</seasonnumber>"} {
		if !strings.Contains(s3, want) {
			t.Fatalf("missing %s in %s", want, s3)
		}
	}
	if strings.Contains(s3, "<plot>") {
		t.Fatalf("empty plot written %s", s3)
	}
	if body := readFile(t, filepath.Join(show, "Archive Season 03 Extra", "poster.jpg")); body != "/s3.jpg" {
		t.Fatalf("season 3 poster %q", body)
	}
	if body := readFile(t, filepath.Join(show, "Archive Season 03 Extra", "ep-thumb.jpg")); body != "/thumb.jpg" {
		t.Fatalf("thumb %q", body)
	}
	s7 := readFile(t, filepath.Join(show, "S07", "season.nfo"))
	for _, want := range []string{"<title>The Long Night</title>", "<seasonnumber>7</seasonnumber>"} {
		if !strings.Contains(s7, want) {
			t.Fatalf("missing %s in %s", want, s7)
		}
	}
}

func TestWriteSeasonSpecials(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("art"))
	}))
	t.Cleanup(srv.Close)
	show := t.TempDir()
	ep := filepath.Join(show, "Movies", "ep.mkv")
	if err := os.MkdirAll(filepath.Dir(ep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ep, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{Provider: "tmdb", ID: "9", Title: "Show", UniqueID: "tmdb-tv"}
	job := match.Job{
		Path: show, Kind: "show", Status: "matched", Match: &hit, CatalogFor: hit.Key(),
		Catalog: []match.CatalogSeason{{
			Number: "0", Poster: srv.URL + "/s0.jpg",
			Episodes: []fetch.Episode{{Number: "1", Title: "Extra", Path: ep}},
		}},
	}
	if _, err := Write(context.Background(), fetch.New(cfg), []match.Job{job}); err != nil {
		t.Fatal(err)
	}
	body := readFile(t, filepath.Join(show, "Movies", "season.nfo"))
	for _, want := range []string{"<title>Specials</title>", "<seasonnumber>0</seasonnumber>"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
	if _, err := os.Stat(filepath.Join(show, "Movies", "poster.jpg")); err != nil {
		t.Fatal(err)
	}
}

func TestWriteSkipsUnclaimedSeasonFolder(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	show := t.TempDir()
	shared := filepath.Join(show, "Movies S04", "ep.mkv")
	other := filepath.Join(show, "Season 4", "ep.mkv")
	plain := filepath.Join(show, "Archive", "ep.mkv")
	for _, p := range []string{shared, other, plain} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hit := rank.Hit{Provider: "tmdb", ID: "9", Title: "Show", UniqueID: "tmdb-tv"}
	job := match.Job{
		Path: show, Kind: "show", Status: "matched", Match: &hit, CatalogFor: hit.Key(),
		Catalog: []match.CatalogSeason{
			{Number: "4", Episodes: []fetch.Episode{{Number: "1", Title: "A", Path: shared}}},
			{Number: "0", Episodes: []fetch.Episode{{Number: "1", Title: "B", Path: shared}}},
			{Number: "9", Episodes: []fetch.Episode{{Number: "1", Title: "C", Path: other}}},
			{Number: "8", Episodes: []fetch.Episode{{Number: "1", Title: "D", Path: plain}}},
		},
	}
	st, err := Write(context.Background(), fetch.New(cfg), []match.Job{job})
	if err != nil {
		t.Fatal(err)
	}
	if st.TitleNFO != 1 {
		t.Fatalf("stats %+v", st)
	}
	if err := filepath.WalkDir(show, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == "season.nfo" {
			t.Fatalf("wrote %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestChangesKeepsSeasonFiles(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("new"))
	}))
	t.Cleanup(srv.Close)
	show := t.TempDir()
	ep12 := filepath.Join(show, "Season 12", "ep.mkv")
	ep3 := filepath.Join(show, "Archive Season 03 Extra", "ep.mkv")
	ep7 := filepath.Join(show, "S07", "ep.mkv")
	for _, p := range []string{ep12, ep3, ep7} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(show, "tvshow.nfo"), []byte("<tvshow><title>Keep Title</title></tvshow>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "poster.jpg"), []byte("keep-show"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "Season 12", "ep.nfo"), []byte("keep-ep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "Season 12", "ep-thumb.jpg"), []byte("keep-thumb"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "Season 12", "season.nfo"), []byte("keep-nfo-12"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "Season 12", "poster.jpg"), []byte("keep-poster-12"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "Archive Season 03 Extra", "season.nfo"), []byte("keep-nfo-3"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "S07", "ep.nfo"), []byte("keep-ep-7"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "S07", "poster.jpg"), []byte("keep-poster-7"), 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{Provider: "tmdb", ID: "9", Title: "Show", Poster: srv.URL + "/show.jpg", UniqueID: "tmdb-tv"}
	job := match.Job{
		Path: show, Kind: "show", Status: "matched", Ranker: "replay", Match: &hit, CatalogFor: hit.Key(),
		Files: []match.JobFile{
			{Path: ep12, Season: "12", Episode: "1"},
			{Path: ep3, Season: "3", Episode: "1"},
			{Path: ep7, Season: "7", Episode: "1"},
		},
		Catalog: []match.CatalogSeason{
			{
				Number: "12", Title: "Show", Poster: srv.URL + "/s12.jpg",
				Episodes: []fetch.Episode{{Number: "1", Title: "One", Path: ep12, Poster: srv.URL + "/t12.jpg"}},
			},
			{
				Number: "3", Title: "Show", Poster: srv.URL + "/s3.jpg",
				Episodes: []fetch.Episode{{Number: "1", Title: "Two", Path: ep3}},
			},
			{
				Number: "7", Title: "The Long Night", Poster: srv.URL + "/s7.jpg",
				Episodes: []fetch.Episode{{Number: "1", Title: "Three", Path: ep7}},
			},
		},
	}
	out, st, err := WriteProgress(context.Background(), fetch.New(cfg), "changes", []match.Job{job}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.TitleNFO != 1 || st.EpisodeNFO != 1 || st.Posters != 1 {
		t.Fatalf("stats %+v", st)
	}
	if out[0].NFOPath != filepath.Join(show, "tvshow.nfo") {
		t.Fatalf("nfo path %q", out[0].NFOPath)
	}
	if body := readFile(t, filepath.Join(show, "tvshow.nfo")); !strings.Contains(body, "<title>Keep Title</title>") {
		t.Fatalf("show nfo %s", body)
	}
	if body := readFile(t, filepath.Join(show, "poster.jpg")); body != "keep-show" {
		t.Fatalf("show poster %q", body)
	}
	if body := readFile(t, filepath.Join(show, "Season 12", "season.nfo")); body != "keep-nfo-12" {
		t.Fatalf("season 12 nfo %q", body)
	}
	if body := readFile(t, filepath.Join(show, "Season 12", "poster.jpg")); body != "keep-poster-12" {
		t.Fatalf("season 12 poster %q", body)
	}
	if body := readFile(t, filepath.Join(show, "Season 12", "ep.nfo")); body != "keep-ep" {
		t.Fatalf("episode nfo %q", body)
	}
	if body := readFile(t, filepath.Join(show, "Season 12", "ep-thumb.jpg")); body != "keep-thumb" {
		t.Fatalf("thumb %q", body)
	}
	if body := readFile(t, filepath.Join(show, "Archive Season 03 Extra", "season.nfo")); body != "keep-nfo-3" {
		t.Fatalf("season 3 nfo %q", body)
	}
	if body := readFile(t, filepath.Join(show, "Archive Season 03 Extra", "poster.jpg")); body != "new" {
		t.Fatalf("season 3 poster %q", body)
	}
	s7 := readFile(t, filepath.Join(show, "S07", "season.nfo"))
	if !strings.Contains(s7, "<title>The Long Night</title>") || !strings.Contains(s7, "<seasonnumber>7</seasonnumber>") {
		t.Fatalf("season 7 nfo %s", s7)
	}
	if body := readFile(t, filepath.Join(show, "S07", "poster.jpg")); body != "keep-poster-7" {
		t.Fatalf("season 7 poster %q", body)
	}
	ep := readFile(t, filepath.Join(show, "Archive Season 03 Extra", "ep.nfo"))
	if !strings.Contains(ep, "<title>Two</title>") {
		t.Fatalf("episode %s", ep)
	}
}

func TestChangesWritesMissingReleaseSeason(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("art"))
	}))
	t.Cleanup(srv.Close)
	show := t.TempDir()
	season := "Show.SEASON.01.S01"
	ep := filepath.Join(show, season, "ep.mkv")
	if err := os.MkdirAll(filepath.Dir(ep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ep, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, season, "ep.nfo"), []byte("keep-ep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "tvshow.nfo"), []byte("<tvshow><title>Keep Title</title></tvshow>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "poster.jpg"), []byte("keep-show"), 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{Provider: "tmdb", ID: "9", Title: "Show", Poster: srv.URL + "/show.jpg", UniqueID: "tmdb-tv"}
	job := match.Job{
		Path: show, Kind: "show", Status: "matched", Ranker: "replay", Match: &hit, CatalogFor: hit.Key(),
		Files: []match.JobFile{{Path: ep, Season: "1", Episode: "1"}},
		Catalog: []match.CatalogSeason{{
			Number: "1", Title: "Show", Poster: srv.URL + "/s1.jpg",
			Episodes: []fetch.Episode{{Number: "1", Title: "One", Path: ep}},
		}},
	}
	_, st, err := WriteProgress(context.Background(), fetch.New(cfg), "changes", []match.Job{job}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.TitleNFO != 1 || st.EpisodeNFO != 0 || st.Posters != 1 {
		t.Fatalf("stats %+v", st)
	}
	if body := readFile(t, filepath.Join(show, "tvshow.nfo")); !strings.Contains(body, "<title>Keep Title</title>") {
		t.Fatalf("show nfo %s", body)
	}
	if body := readFile(t, filepath.Join(show, "poster.jpg")); body != "keep-show" {
		t.Fatalf("show poster %q", body)
	}
	if body := readFile(t, filepath.Join(show, season, "ep.nfo")); body != "keep-ep" {
		t.Fatalf("episode nfo %q", body)
	}
	nfo := readFile(t, filepath.Join(show, season, "season.nfo"))
	for _, want := range []string{"<title>Season 1</title>", "<seasonnumber>1</seasonnumber>"} {
		if !strings.Contains(nfo, want) {
			t.Fatalf("missing %s in %s", want, nfo)
		}
	}
	if body := readFile(t, filepath.Join(show, season, "poster.jpg")); body != "art" {
		t.Fatalf("season poster %q", body)
	}
}

func TestRescanReplacesSeasonPoster(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("new"))
	}))
	t.Cleanup(srv.Close)
	show := t.TempDir()
	ep := filepath.Join(show, "Season 12", "ep.mkv")
	if err := os.MkdirAll(filepath.Dir(ep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ep, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "Season 12", "season.nfo"), []byte("old-nfo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "Season 12", "poster.jpg"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	hit := rank.Hit{Provider: "tmdb", ID: "9", Title: "Show", Poster: srv.URL + "/show.jpg", UniqueID: "tmdb-tv"}
	job := match.Job{
		Path: show, Kind: "show", Status: "matched", Match: &hit, CatalogFor: hit.Key(),
		Catalog: []match.CatalogSeason{{
			Number: "12", Title: "The Long Night", Poster: srv.URL + "/s12.jpg",
			Episodes: []fetch.Episode{{Number: "1", Title: "One", Path: ep}},
		}},
	}
	if _, _, err := WriteProgress(context.Background(), fetch.New(cfg), "rescan", []match.Job{job}, nil); err != nil {
		t.Fatal(err)
	}
	body := readFile(t, filepath.Join(show, "Season 12", "season.nfo"))
	if !strings.Contains(body, "<title>The Long Night</title>") || strings.Contains(body, "old-nfo") {
		t.Fatalf("season nfo %s", body)
	}
	if got := readFile(t, filepath.Join(show, "Season 12", "poster.jpg")); got != "new" {
		t.Fatalf("poster %q", got)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
