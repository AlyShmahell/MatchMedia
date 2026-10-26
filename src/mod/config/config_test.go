package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewShippedTMDB(t *testing.T) {
	cfg, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Group.DistanceThreshold != 0.49 {
		t.Fatalf("threshold %v", cfg.Group.DistanceThreshold)
	}
	eps := cfg.Endpoints()
	if len(eps) != 4 {
		t.Fatalf("want tmdb movie/tv, omdb, tvmaze, got %d", len(eps))
	}
	var movie, tv, omdb, tvmaze bool
	for _, ep := range eps {
		switch ep.Name {
		case "tmdb":
			if ep.Spec.Use != UsePrimary {
				t.Fatalf("tmdb use %s", ep.Spec.Use)
			}
			switch ep.Spec.NFO {
			case "movie":
				movie = true
				if ep.Spec.UniqueID != "tmdb-movie" {
					t.Fatalf("movie uniqueid %s", ep.Spec.UniqueID)
				}
			case "tvshow":
				tv = true
				if ep.Spec.Catalog == nil {
					t.Fatal("tv catalog missing")
				}
			}
		case "omdb":
			omdb = true
			if ep.Spec.Use != UseDefer {
				t.Fatalf("omdb use %s", ep.Spec.Use)
			}
		case "tvmaze":
			tvmaze = true
			if ep.Spec.Use != UseFallback {
				t.Fatalf("tvmaze use %s", ep.Spec.Use)
			}
		default:
			t.Fatalf("provider %s", ep.Name)
		}
	}
	if !movie || !tv || !omdb || !tvmaze {
		t.Fatal("need tmdb movie/tv, omdb, tvmaze")
	}
}

func TestInvalidUse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overlay.yaml")
	if err := os.WriteFile(path, []byte("providers:\n  tmdb:\n    use: later\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path); err == nil {
		t.Fatal("want use error")
	}
}

func TestOverlayWins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overlay.yaml")
	if err := os.WriteFile(path, []byte("match:\n  min_score: 0.9\nproviders:\n  tmdb:\n    api_key: testkey\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Match.MinScore != 0.9 {
		t.Fatalf("min_score %v", cfg.Match.MinScore)
	}
	if cfg.Providers["tmdb"].APIKey != "testkey" {
		t.Fatalf("api_key %q", cfg.Providers["tmdb"].APIKey)
	}
	for _, ep := range cfg.Endpoints() {
		if ep.Name != "tmdb" {
			continue
		}
		if ep.Spec.APIKey != "testkey" {
			t.Fatalf("endpoint key %q", ep.Spec.APIKey)
		}
	}
}

func TestSearchSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overlay.yaml")
	over := []byte("providers:\n  local:\n    url: http://example/search\n    items: results\n    fields: { id: id, title: title }\n    min_interval_ms: 40\n    titles:\n      url: http://example/titles\n      min_interval_ms: 90\n  tvmaze:\n    search:\n      min_interval_ms: 600\n")
	if err := os.WriteFile(path, over, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	movie, ok := cfg.Endpoint("tmdb", "movie")
	if !ok || movie.Search == nil || movie.Search.URL != "{base}/search/movie" || movie.Search.Items != "results" || movie.Fields["title"] != "title" {
		t.Fatalf("tmdb movie search %#v", movie.Search)
	}
	tvmaze, ok := cfg.Endpoint("tvmaze", "tvshow")
	if !ok || tvmaze.MinIntervalMS != 0 || tvmaze.Search == nil || tvmaze.Search.MinIntervalMS != 600 {
		t.Fatalf("tvmaze pace provider %d search %#v", tvmaze.MinIntervalMS, tvmaze.Search)
	}
	if tvmaze.Search.Fields["network"] != "show.network.name" || tvmaze.Fields["title"] != "show.name" {
		t.Fatalf("tvmaze fields %#v", tvmaze.Search.Fields)
	}
	if tvmaze.Titles == nil || tvmaze.Titles.MinIntervalMS != 0 {
		t.Fatal("tvmaze titles paced")
	}
	local, ok := cfg.Endpoint("local", "")
	if !ok || local.Search == nil || local.Search.URL != "http://example/search" || local.MinIntervalMS != 40 {
		t.Fatalf("local %#v", local.Search)
	}
	if local.Titles == nil || local.Titles.MinIntervalMS != 90 {
		t.Fatalf("local titles %#v", local.Titles)
	}
}

func TestEndpointLookup(t *testing.T) {
	cfg, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Endpoint("tmdb", "movie"); !ok {
		t.Fatal("movie endpoint")
	}
	if _, ok := cfg.Endpoint("tmdb", "tvshow"); !ok {
		t.Fatal("tv endpoint")
	}
}
