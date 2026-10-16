package match

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/fetch"
)

func TestRunSkipsTerminalJobs(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	in := []Job{
		{Title: "A", Status: "unmatched"},
		{Title: "B", Status: "matched", Match: &Candidate{Title: "B", ID: "1", Provider: "tmdb"}},
	}
	out := Run(context.Background(), cfg, in)
	if out[0].Status != "unmatched" || out[1].Status != "matched" {
		t.Fatalf("%#v", out)
	}
}

func TestRunProgressCount(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	in := []Job{
		{Title: "A", Status: "unmatched"},
		{Title: "B", Status: "matched", Match: &Candidate{Title: "B", ID: "1", Provider: "tmdb"}},
	}
	var n int
	out := RunProgress(context.Background(), cfg, in, func(done, total int, job Job) {
		n++
		if total != len(in) {
			t.Fatalf("total %d", total)
		}
		if done < 1 || done > total {
			t.Fatalf("done %d", done)
		}
		if job.Title == "" {
			t.Fatal("empty job")
		}
	})
	if n != len(in) {
		t.Fatalf("callbacks %d want %d (out %#v)", n, len(in), out)
	}
}

func TestNgramFallbackSearch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/3/search/movie", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	})
	mux.HandleFunc("/3/search/tv", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		results := []any{}
		if q == "hero without a class who even need" {
			results = []any{map[string]any{
				"id":             236565,
				"name":           "Hero Without a Class: Who Even Needs Skills?!",
				"first_air_date": "2025-10-01",
				"overview":       "a show",
			}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	overlay := filepath.Join(t.TempDir(), "overlay.yaml")
	body := "providers:\n  tmdb:\n    api_key: k\n    base: " + srv.URL + "/3\n  tvmaze:\n    require: api_key\n"
	if err := os.WriteFile(overlay, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.New(overlay)
	if err != nil {
		t.Fatal(err)
	}
	job := Job{Title: "Hero Without a Class Who Even Need Skills", Status: "pending"}
	out := runOne(context.Background(), cfg, fetch.New(cfg), job)
	if out.Status == "unmatched" || len(out.Candidates) == 0 {
		t.Fatalf("status=%s candidates=%#v err=%s", out.Status, out.Candidates, out.Error)
	}
	if out.Candidates[0].ID != "236565" {
		t.Fatalf("hit %#v", out.Candidates[0])
	}
}
