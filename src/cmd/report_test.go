package main

import (
	"strings"
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/match"
)

func TestRenderReport(t *testing.T) {
	chosen := match.Candidate{
		Provider: "tmdb",
		ID:       "13",
		Title:    "Cowboy Bebop",
		Year:     "1998",
		Score:    0.9876,
		Jaccard:  1,
		URL:      "https://example.test/13",
	}
	other := match.Candidate{
		Provider: "tmdb",
		ID:       "99",
		Title:    "Other | Show",
		Year:     "1999",
		Score:    0.4,
		Jaccard:  0.25,
		URL:      "https://example.test/99",
	}
	jobs := []match.Job{
		{
			Title:      "Cowboy Bebop",
			Year:       "1998",
			Kind:       "show",
			Status:     "matched",
			Path:       "/media/Cowboy Bebop",
			Files:      []match.JobFile{{Path: "a.mkv"}, {Path: "b.mkv"}},
			Match:      &chosen,
			Candidates: []match.Candidate{chosen, other},
		},
		{
			Title:  "Manual `Pick`",
			Year:   "2001",
			Kind:   "movie",
			Status: "manual",
			Path:   "/media/Manual",
			Files:  []match.JobFile{{Path: "c.mkv"}},
			Candidates: []match.Candidate{
				{Provider: "tmdb", ID: "2", Title: "Maybe", Year: "2001", Score: 0.55, Jaccard: 0.5, URL: "https://example.test/2"},
				{Provider: "tmdb", ID: "3", Title: "Also", Year: "2000", Score: 0.44, Jaccard: 0.4, URL: "https://example.test/3"},
			},
		},
		{
			Title:  "No Hits",
			Kind:   "movie",
			Status: "unmatched",
			Path:   "/media/No Hits",
			Files:  []match.JobFile{{Path: "d.mkv"}},
		},
		{
			Title:  "Dead Mount Death Play",
			Year:   "2023",
			Kind:   "show",
			Status: "matched",
			Ranker: "replay",
			Path:   "/media/Dead Mount Death Play",
			Files:  []match.JobFile{{Path: "a.mkv"}},
			Match:  &match.Candidate{Title: "Dead Mount Death Play", Score: 0},
		},
		{
			Title:  "Violet Evergarden",
			Year:   "2018",
			Kind:   "show",
			Status: "matched",
			Ranker: "sidecar",
			Path:   "/media/Violet Evergarden",
			Files:  []match.JobFile{{Path: "a.mkv"}},
			Match:  &match.Candidate{Provider: "tvmaze", ID: "1", Title: "Violet Evergarden"},
			Candidates: []match.Candidate{{
				Provider: "tvmaze", ID: "1", Title: "Violet Evergarden", Year: "2018",
			}},
		},
	}

	got := renderReport(jobs)
	if !strings.Contains(got, "jobs: 5 · ✅ matched: 3 · ✋ manual: 1 · ❌ unmatched: 1 · 💥 error: 0") {
		t.Fatalf("counts:\n%s", got)
	}
	if !strings.Contains(got, "| Title | Year | Kind | Files | Ngrams | Status | Match | Score | Link |") {
		t.Fatal("summary header")
	}
	if !strings.Contains(got, "| Cowboy Bebop | 1998 | show | 2 | 0 | ✅ matched | Cowboy Bebop | 0.988 | https://example.test/13 |") {
		t.Fatalf("matched summary row:\n%s", got)
	}
	if !strings.Contains(got, "| Manual \\`Pick\\` | 2001 | movie | 1 | 0 | ✋ manual |  |  |  |") {
		t.Fatalf("manual summary row:\n%s", got)
	}
	if !strings.Contains(got, "| No Hits |  | movie | 1 | 0 | ❌ unmatched |  |  |  |") {
		t.Fatalf("unmatched summary row:\n%s", got)
	}
	if !strings.Contains(got, "| Dead Mount Death Play | 2023 | show | 1 | 0 | ✅ matched | Dead Mount Death Play |  |  |") {
		t.Fatalf("replay summary row:\n%s", got)
	}
	if !strings.Contains(got, "| Violet Evergarden | 2018 | show | 1 | 0 | ✅ matched | Violet Evergarden |  |  |") {
		t.Fatalf("sidecar summary row:\n%s", got)
	}
	if !strings.Contains(got, "| 1 | yes | Violet Evergarden | 2018 |  |  | tvmaze |  |") {
		t.Fatalf("sidecar candidate row:\n%s", got)
	}
	if !strings.Contains(got, "status: ✅ matched") {
		t.Fatalf("matched status line:\n%s", got)
	}
	if !strings.Contains(got, "status: ✋ manual") {
		t.Fatalf("manual status line:\n%s", got)
	}

	if !strings.Contains(got, "## Cowboy Bebop") {
		t.Fatal("matched section")
	}
	if !strings.Contains(got, "| # | Chosen | Title | Year | Score | Jaccard | Provider | Link |") {
		t.Fatal("candidate header")
	}
	if !strings.Contains(got, "| 1 | yes | Cowboy Bebop | 1998 | 0.988 | 1.000 | tmdb | https://example.test/13 |") {
		t.Fatalf("chosen row:\n%s", got)
	}
	if !strings.Contains(got, "| 2 |  | Other \\| Show | 1999 | 0.400 | 0.250 | tmdb | https://example.test/99 |") {
		t.Fatalf("ranked other row:\n%s", got)
	}

	if !strings.Contains(got, "## Manual `Pick`") {
		t.Fatal("manual heading keeps backticks")
	}
	if !strings.Contains(got, "| 1 |  | Maybe | 2001 | 0.550 | 0.500 | tmdb | https://example.test/2 |") {
		t.Fatalf("manual first candidate:\n%s", got)
	}
	if !strings.Contains(got, "| 2 |  | Also | 2000 | 0.440 | 0.400 | tmdb | https://example.test/3 |") {
		t.Fatalf("manual second candidate:\n%s", got)
	}

	idx := strings.Index(got, "## No Hits")
	if idx < 0 {
		t.Fatal("unmatched section")
	}
	if !strings.Contains(got[idx:], "no candidates") {
		t.Fatalf("unmatched candidates:\n%s", got[idx:])
	}
}
