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
	hits, errs, ok := c.Search(context.Background(), rank.Query{Title: "X"}, false)
	if ok == 0 && len(errs) > 0 {
		t.Fatalf("errs %v (tmdb is defer, need deferred pass)", errs)
	}
	hits, errs, ok = c.Search(context.Background(), rank.Query{Title: "X"}, true)
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
			hits, errs, ok := c.Search(context.Background(), q, false)
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
