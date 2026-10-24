package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
)

func TestPublicQueryStripsKey(t *testing.T) {
	tmdb := publicQuery(map[string]string{"api_key": "{api_key}"}, map[string]string{"api_key": "secret"})
	if tmdb != "" {
		t.Fatalf("tmdb query %q", tmdb)
	}
	omdb := publicQuery(map[string]string{
		"apikey": "{api_key}",
		"i":      "{id}",
		"plot":   "short",
	}, map[string]string{"api_key": "secret", "id": "tt0133093"})
	if omdb != "i=tt0133093&plot=short" {
		t.Fatalf("omdb query %q", omdb)
	}
}

func TestDetailRecordsRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	cfg := overlayCfg(t, srv.URL)
	c := New(cfg)

	movie := rank.Hit{Provider: "tmdb", ID: "129", NFO: "movie", Title: "Title"}
	c.Detail(context.Background(), rank.Query{Title: "Title"}, &movie)
	if len(movie.Requests) != 1 || movie.Requests[0].Section != "detail" || movie.Requests[0].Query != "" {
		t.Fatalf("tmdb %#v", movie.Requests)
	}
	if movie.Requests[0].URL != srv.URL+"/movie/129" {
		t.Fatalf("tmdb url %q", movie.Requests[0].URL)
	}

	omdb := rank.Hit{Provider: "omdb", ID: "tt0133093", NFO: "movie", Title: "Title"}
	c.Detail(context.Background(), rank.Query{Title: "Title"}, &omdb)
	if len(omdb.Requests) != 1 || omdb.Requests[0].Section != "detail" {
		t.Fatalf("omdb %#v", omdb.Requests)
	}
	if omdb.Requests[0].Query != "i=tt0133093&plot=short" || omdb.Requests[0].URL != srv.URL {
		t.Fatalf("omdb url %q query %q", omdb.Requests[0].URL, omdb.Requests[0].Query)
	}
}

func overlayCfg(t *testing.T, base string) *config.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "overlay.yaml")
	body := "providers:\n  tmdb:\n    api_key: k\n    base: " + base + "\n  omdb:\n    api_key: k\n    base: " + base + "\n  tvmaze:\n    require: api_key\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.New(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
