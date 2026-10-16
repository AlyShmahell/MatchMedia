package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
)

func TestSearchMovieAndTV(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/3/search/movie", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{map[string]any{"id": 1, "title": "Film", "release_date": "1999-01-01", "overview": "a movie", "poster_path": "/p.jpg"}},
		})
	})
	mux.HandleFunc("/3/search/tv", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{map[string]any{"id": 2, "name": "Show", "first_air_date": "1998-01-01", "overview": "a show", "poster_path": "/s.jpg"}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	overlay := filepath.Join(t.TempDir(), "overlay.yaml")
	body := "providers:\n  tmdb:\n    api_key: k\n    base: " + srv.URL + "/3\n"
	if err := os.WriteFile(overlay, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.New(overlay)
	if err != nil {
		t.Fatal(err)
	}
	c := New(cfg)
	hits, errs, ok := c.Search(context.Background(), rank.Query{Title: "X"})
	if len(errs) > 0 || ok == 0 {
		t.Fatalf("ok=%d errs=%v hits=%d", ok, errs, len(hits))
	}
	var movie, tv bool
	for _, h := range hits {
		if h.Provider != "tmdb" {
			t.Fatalf("provider %s", h.Provider)
		}
		switch h.NFO {
		case "movie":
			movie = true
			if h.UniqueID != "tmdb-movie" || !strings.Contains(h.URL, "/movie/") {
				t.Fatalf("%#v", h)
			}
		case "tvshow":
			tv = true
			if h.UniqueID != "tmdb-tv" || !strings.Contains(h.URL, "/tv/") {
				t.Fatalf("%#v", h)
			}
		}
	}
	if !movie || !tv {
		t.Fatalf("hits %#v", hits)
	}
}

func TestSearchConcurrentOnce(t *testing.T) {
	var n atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		time.Sleep(50 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{map[string]any{"id": 1, "title": "Film"}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	overlay := filepath.Join(t.TempDir(), "overlay.yaml")
	body := "providers:\n  stub:\n    base: " + srv.URL + "\n    url: \"{base}/search\"\n    items: results\n    fields: { id: id, title: title }\n"
	if err := os.WriteFile(overlay, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.New(overlay)
	if err != nil {
		t.Fatal(err)
	}
	c := New(cfg)
	q := rank.Query{Title: "A Silent Voice", Year: "2016"}
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		got      [][]rank.Hit
		firstErr error
	)
	wg.Add(2)
	for range 2 {
		go func() {
			defer wg.Done()
			hits, errs, ok := c.Search(context.Background(), q)
			mu.Lock()
			defer mu.Unlock()
			if firstErr != nil {
				return
			}
			if len(errs) > 0 || ok == 0 || len(hits) != 1 {
				firstErr = fmt.Errorf("ok=%d errs=%v hits=%d", ok, errs, len(hits))
				return
			}
			got = append(got, hits)
		}()
	}
	wg.Wait()
	if firstErr != nil {
		t.Fatal(firstErr)
	}
	if n.Load() != 1 {
		t.Fatalf("requests %d", n.Load())
	}
	if len(got) != 2 {
		t.Fatalf("results %d", len(got))
	}
	got[0][0].Jaccard = 1
	got[0][0].Score = 9
	if got[1][0].Jaccard != 0 || got[1][0].Score != 0 {
		t.Fatalf("shared hits %#v", got[1][0])
	}
}

func TestUseWaves(t *testing.T) {
	var primaryN, deferN, fallbackN atomic.Int32
	hit := func(id int, title string) map[string]any {
		return map[string]any{"results": []any{map[string]any{"id": id, "title": title}}}
	}
	empty := map[string]any{"results": []any{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/primary", func(w http.ResponseWriter, r *http.Request) {
		primaryN.Add(1)
		q := r.URL.Query().Get("q")
		if q == "down" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if q != "X" {
			_ = json.NewEncoder(w).Encode(empty)
			return
		}
		_ = json.NewEncoder(w).Encode(hit(1, "P"))
	})
	mux.HandleFunc("/defer", func(w http.ResponseWriter, r *http.Request) {
		deferN.Add(1)
		q := r.URL.Query().Get("q")
		if q == "down" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if q == "empty" {
			_ = json.NewEncoder(w).Encode(empty)
			return
		}
		_ = json.NewEncoder(w).Encode(hit(2, "D"))
	})
	mux.HandleFunc("/fallback", func(w http.ResponseWriter, r *http.Request) {
		fallbackN.Add(1)
		_ = json.NewEncoder(w).Encode(hit(3, "F"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	write := func(t *testing.T, extra string) *config.Config {
		t.Helper()
		overlay := filepath.Join(t.TempDir(), "overlay.yaml")
		body := "providers:\n  tvmaze:\n    require: api_key\n  alpha:\n    use: primary\n    base: " + srv.URL + "\n    url: \"{base}/primary\"\n    query: { q: \"{title}\" }\n    items: results\n    fields: { id: id, title: title }\n  beta:\n    use: defer\n    base: " + srv.URL + "\n    url: \"{base}/defer\"\n    query: { q: \"{title}\" }\n    items: results\n    fields: { id: id, title: title }\n  gamma:\n    use: fallback\n    base: " + srv.URL + "\n    url: \"{base}/fallback\"\n    items: results\n    fields: { id: id, title: title }\n" + extra
		if err := os.WriteFile(overlay, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := config.New(overlay)
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}

	t.Run("primaryHitsStillDeferSkipFallback", func(t *testing.T) {
		primaryN.Store(0)
		deferN.Store(0)
		fallbackN.Store(0)
		c := New(write(t, ""))
		hits, errs, ok := c.Search(context.Background(), rank.Query{Title: "X"})
		if len(errs) > 0 || ok == 0 {
			t.Fatalf("ok=%d errs=%v", ok, errs)
		}
		if primaryN.Load() != 1 || deferN.Load() != 1 || fallbackN.Load() != 0 {
			t.Fatalf("counts p=%d d=%d f=%d", primaryN.Load(), deferN.Load(), fallbackN.Load())
		}
		if len(hits) != 2 {
			t.Fatalf("hits %d", len(hits))
		}
	})

	t.Run("emptyPrimaryAndDeferSkipFallback", func(t *testing.T) {
		primaryN.Store(0)
		deferN.Store(0)
		fallbackN.Store(0)
		c := New(write(t, ""))
		hits, errs, ok := c.Search(context.Background(), rank.Query{Title: "empty"})
		if len(errs) > 0 || ok == 0 {
			t.Fatalf("ok=%d errs=%v", ok, errs)
		}
		if primaryN.Load() != 1 || deferN.Load() != 1 || fallbackN.Load() != 0 {
			t.Fatalf("counts p=%d d=%d f=%d", primaryN.Load(), deferN.Load(), fallbackN.Load())
		}
		if len(hits) != 0 {
			t.Fatalf("hits %#v", hits)
		}
	})

	t.Run("primaryErrorCallsFallback", func(t *testing.T) {
		primaryN.Store(0)
		deferN.Store(0)
		fallbackN.Store(0)
		c := New(write(t, "http:\n  retries: 1\n"))
		hits, errs, ok := c.Search(context.Background(), rank.Query{Title: "down"})
		if ok == 0 {
			t.Fatalf("ok=%d errs=%v", ok, errs)
		}
		if primaryN.Load() != 1 || deferN.Load() != 1 || fallbackN.Load() != 1 {
			t.Fatalf("counts p=%d d=%d f=%d", primaryN.Load(), deferN.Load(), fallbackN.Load())
		}
		if len(hits) != 1 || hits[0].Title != "F" {
			t.Fatalf("hits %#v", hits)
		}
	})

	t.Run("emptyPrimaryDeferHitsSkipFallback", func(t *testing.T) {
		primaryN.Store(0)
		deferN.Store(0)
		fallbackN.Store(0)
		c := New(write(t, ""))
		hits, errs, ok := c.Search(context.Background(), rank.Query{Title: "deferhit"})
		if len(errs) > 0 || ok == 0 {
			t.Fatalf("ok=%d errs=%v hits=%d", ok, errs, len(hits))
		}
		if primaryN.Load() != 1 || deferN.Load() != 1 || fallbackN.Load() != 0 {
			t.Fatalf("counts p=%d d=%d f=%d", primaryN.Load(), deferN.Load(), fallbackN.Load())
		}
		if len(hits) != 1 || hits[0].Title != "D" {
			t.Fatalf("hits %#v", hits)
		}
	})
}

func TestCooldownTripsOnAttempts(t *testing.T) {
	var n atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	overlay := filepath.Join(t.TempDir(), "overlay.yaml")
	body := "http:\n  retries: 2\n  cooldown:\n    fails: 2\n    min_ms: 60000\n    max_ms: 120000\nproviders:\n  tvmaze:\n    require: api_key\n  stub:\n    use: primary\n    base: " + srv.URL + "\n    url: \"{base}/search\"\n    items: results\n    fields: { id: id, title: title }\n"
	if err := os.WriteFile(overlay, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.New(overlay)
	if err != nil {
		t.Fatal(err)
	}
	c := New(cfg)
	q := rank.Query{Title: "X"}
	_, _, _ = c.Search(context.Background(), q)
	first := n.Load()
	if first != 2 {
		t.Fatalf("attempts %d want 2", first)
	}
	hits, errs, ok := c.Search(context.Background(), q)
	if n.Load() != first {
		t.Fatalf("retried after cooldown: %d", n.Load())
	}
	if ok != 0 || len(hits) != 0 || len(errs) == 0 {
		t.Fatalf("ok=%d hits=%d errs=%v", ok, len(hits), errs)
	}
}
