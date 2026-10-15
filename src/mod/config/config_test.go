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
	if len(eps) != 2 {
		t.Fatalf("want movie and tv endpoints, got %d", len(eps))
	}
	var movie, tv bool
	for _, ep := range eps {
		if ep.Name != "tmdb" {
			t.Fatalf("provider %s", ep.Name)
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
	}
	if !movie || !tv {
		t.Fatal("need movie and tv")
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
		if ep.Spec.APIKey != "testkey" {
			t.Fatalf("endpoint key %q", ep.Spec.APIKey)
		}
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
