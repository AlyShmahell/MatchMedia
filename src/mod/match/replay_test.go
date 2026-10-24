package match

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
	"github.com/alyshmahell/matchmedia/src/mod/fetch"
)

func TestReplaySkipsSearch(t *testing.T) {
	var mu sync.Mutex
	var searches int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/tv":
			mu.Lock()
			searches++
			mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
		case "/tv/7":
			_, _ = w.Write([]byte(`{"seasons":[{"id":11,"name":"Season 1","season_number":1}]}`))
		case "/tv/7/season/1":
			_, _ = w.Write([]byte(`{"episodes":[{"id":101,"name":"Pilot","episode_number":1}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	job := Job{
		Title: "Show", Kind: "show", Status: "pending", Path: t.TempDir(),
		Requests: []rank.Request{
			{Section: "catalog/seasons", Provider: "tmdb", NFO: "tvshow", URL: srv.URL + "/tv/7"},
			{Section: "catalog/episodes", Provider: "tmdb", NFO: "tvshow", Season: "1", URL: srv.URL + "/tv/7/season/1"},
		},
		Files: []JobFile{{Path: "Show.S01E01.mkv", Season: "1", Episode: "1"}},
	}
	out := Collect(Run(context.Background(), fetch.New(replayCfg(t, srv.URL)), []Job{job}))
	mu.Lock()
	n := searches
	mu.Unlock()
	if n != 0 {
		t.Fatalf("searches %d", n)
	}
	if len(out) != 1 || out[0].Status != "matched" || out[0].Ranker != "replay" {
		t.Fatalf("%#v", out)
	}
	if len(out[0].Catalog) != 1 || len(out[0].Catalog[0].Episodes) != 1 || out[0].Catalog[0].Episodes[0].Title != "Pilot" {
		t.Fatalf("catalog %#v", out[0].Catalog)
	}
}

func TestReplayFailureSearchesOnce(t *testing.T) {
	var mu sync.Mutex
	var searches int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search/tv" {
			mu.Lock()
			searches++
			mu.Unlock()
			_, _ = w.Write([]byte(`{"results":[{"id":7,"name":"Show","first_air_date":"2016-01-01"}]}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	job := Job{
		Title: "Show", Kind: "show", Status: "pending",
		Requests: []rank.Request{
			{Section: "catalog/seasons", Provider: "tmdb", NFO: "tvshow", URL: srv.URL + "/tv/7"},
		},
	}
	out := Collect(Run(context.Background(), fetch.New(replayCfg(t, srv.URL)), []Job{job}))
	mu.Lock()
	n := searches
	mu.Unlock()
	if n != 1 {
		t.Fatalf("searches %d", n)
	}
	if len(out) != 1 || out[0].Status != "matched" || out[0].Ranker == "replay" {
		t.Fatalf("%#v", out)
	}
}

func replayCfg(t *testing.T, base string) *config.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "overlay.yaml")
	body := "providers:\n  tmdb:\n    api_key: k\n    base: " + base + "\n  omdb:\n    require: api_key\n  tvmaze:\n    require: api_key\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.New(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
