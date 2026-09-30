package match

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alyshmahell/matchmedia/lib/config"
)

func TestRunOneSkipsDeferredWhenAutoMatch(t *testing.T) {
	var jk atomic.Int32
	tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"score": 1, "show": map[string]any{"id": 1, "name": "Girls", "premiered": "2012", "url": "http://t"}},
		})
	}))
	t.Cleanup(tvmaze.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jk.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{map[string]any{"mal_id": 1, "title": "Girls", "year": 2012, "url": "http://j"}},
		})
	}))
	t.Cleanup(jikan.Close)
	cfg := deferPairConfig(tvmaze.URL, jikan.URL)
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{Title: "Girls"})
	if job.Status != "matched" {
		t.Fatalf("status=%s err=%s", job.Status, job.Error)
	}
	if job.Match == nil || job.Match.Provider != "tvmaze" {
		t.Fatalf("match=%+v", job.Match)
	}
	if jk.Load() != 0 {
		t.Fatalf("deferred provider hits=%d", jk.Load())
	}
}

func TestRunOneCallsDeferredWhenFastEmpty(t *testing.T) {
	var jk atomic.Int32
	tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(tvmaze.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jk.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{map[string]any{"mal_id": 1, "title": "Cowboy Bebop", "year": 1998, "url": "http://j"}},
		})
	}))
	t.Cleanup(jikan.Close)
	cfg := deferPairConfig(tvmaze.URL, jikan.URL)
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{Title: "Cowboy Bebop"})
	if jk.Load() != 1 {
		t.Fatalf("deferred provider hits=%d", jk.Load())
	}
	if job.Status != "matched" || job.Match == nil || job.Match.Provider != "jikan" {
		t.Fatalf("status=%s match=%+v err=%s", job.Status, job.Match, job.Error)
	}
}

func TestRunOneSkipsDeferredWhenManualHighScores(t *testing.T) {
	var jk atomic.Int32
	tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"score": 1, "show": map[string]any{"id": 1, "name": "Dark Matter", "premiered": "2015", "url": "http://t"}},
			map[string]any{"score": 1, "show": map[string]any{"id": 2, "name": "Dark Matter", "premiered": "2024", "url": "http://t2"}},
		})
	}))
	t.Cleanup(tvmaze.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jk.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{map[string]any{"mal_id": 1, "title": "Dark Matter", "year": 2015, "url": "http://j"}},
		})
	}))
	t.Cleanup(jikan.Close)
	cfg := deferPairConfig(tvmaze.URL, jikan.URL)
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{Title: "Dark Matter"})
	if job.Status != "manual" {
		t.Fatalf("status=%s err=%s candidates=%+v", job.Status, job.Error, job.Candidates)
	}
	if jk.Load() != 0 {
		t.Fatalf("deferred provider hits=%d", jk.Load())
	}
}

func TestRunOneCallsDeferredWhenSeveralWeakHits(t *testing.T) {
	var jk atomic.Int32
	tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"score": 1, "show": map[string]any{"id": 1, "name": "Unrelated Show", "premiered": "1999", "url": "http://t"}},
			map[string]any{"score": 1, "show": map[string]any{"id": 2, "name": "Other Thing", "premiered": "2001", "url": "http://t2"}},
		})
	}))
	t.Cleanup(tvmaze.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jk.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{map[string]any{"mal_id": 1, "title": "Cowboy Bebop", "year": 1998, "url": "http://j"}},
		})
	}))
	t.Cleanup(jikan.Close)
	cfg := deferPairConfig(tvmaze.URL, jikan.URL)
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{Title: "Cowboy Bebop"})
	if jk.Load() != 1 {
		t.Fatalf("deferred provider hits=%d", jk.Load())
	}
	if job.Status != "matched" || job.Match == nil || job.Match.Provider != "jikan" {
		t.Fatalf("status=%s match=%+v err=%s", job.Status, job.Match, job.Error)
	}
}

func TestRunOneCallsDeferredWhenFastBelowMinScore(t *testing.T) {
	var jk atomic.Int32
	tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"score": 1, "show": map[string]any{"id": 1, "name": "Unrelated Show", "premiered": "1999", "url": "http://t"}},
		})
	}))
	t.Cleanup(tvmaze.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jk.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{map[string]any{"mal_id": 1, "title": "Cowboy Bebop", "year": 1998, "url": "http://j"}},
		})
	}))
	t.Cleanup(jikan.Close)
	cfg := deferPairConfig(tvmaze.URL, jikan.URL)
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{Title: "Cowboy Bebop"})
	if jk.Load() != 1 {
		t.Fatalf("deferred provider hits=%d", jk.Load())
	}
	if job.Status != "matched" || job.Match == nil || job.Match.Provider != "jikan" {
		t.Fatalf("status=%s match=%+v candidates=%+v err=%s", job.Status, job.Match, job.Candidates, job.Error)
	}
}

func deferPairConfig(tvURL, jkURL string) config.Config {
	return config.Config{
		HTTP: config.HTTP{TimeoutMS: 5000, Retries: 1, ProviderTimeoutMS: 1000},
		Match: config.Match{
			MinScore: 0.72, MinMargin: 0.04,
			PlotStop: testPlotStop,
		},
		Providers: map[string]config.Provider{
			"tvmaze": {
				Types:  []string{"tv", ""},
				Base:   tvURL,
				URL:    "{base}/search/shows",
				Query:  map[string]string{"q": "{title}"},
				Items:  "$",
				Fields: map[string]string{"id": "show.id", "title": "show.name", "year": "show.premiered", "url": "show.url"},
			},
			"jikan": {
				Types:  []string{"anime", "movie"},
				Base:   jkURL,
				URL:    "{base}/anime",
				Query:  map[string]string{"q": "{title}"},
				Items:  "data",
				Fields: map[string]string{"id": "mal_id", "title": "title", "year": "year", "url": "url"},
				Defer:  true,
			},
		},
	}
}

func TestRunOneTypedMovieSkipsJikanFallback(t *testing.T) {
	var jk atomic.Int32
	omdb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"Search": []any{}})
	}))
	t.Cleanup(omdb.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jk.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{map[string]any{"mal_id": 1, "title": "Dune", "year": 2021, "url": "http://j"}},
		})
	}))
	t.Cleanup(jikan.Close)
	cfg := config.Config{
		HTTP:  config.HTTP{TimeoutMS: 5000, Retries: 1, ProviderTimeoutMS: 1000},
		Match: config.Match{MinScore: 0.72, MinMargin: 0.04},
		Providers: map[string]config.Provider{
			"omdb": {
				Types:  []string{"movie", "tv", ""},
				Base:   omdb.URL,
				URL:    "{base}",
				Query:  map[string]string{"s": "{title}"},
				Items:  "Search",
				Fields: map[string]string{"id": "imdbID", "title": "Title", "year": "Year", "url": "imdbID"},
			},
			"jikan": {
				Types:  []string{"anime"},
				Base:   jikan.URL,
				URL:    "{base}/anime",
				Query:  map[string]string{"q": "{title}"},
				Items:  "data",
				Fields: map[string]string{"id": "mal_id", "title": "title", "year": "year", "url": "url"},
				Defer:  true,
			},
		},
	}
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{Title: "Dune", Type: "movie"})
	if jk.Load() != 0 {
		t.Fatalf("jikan hits=%d", jk.Load())
	}
	if job.Status != "unmatched" {
		t.Fatalf("status=%s err=%s match=%+v", job.Status, job.Error, job.Match)
	}
}

func TestPreferCandidatesKeepsAnimeShaped(t *testing.T) {
	cfg := config.Config{Match: config.Match{Prefer: map[string]map[string]string{
		"anime": {"language": "Japanese", "kind": "Animation"},
	}}}
	cands := []Candidate{
		{Title: "History Erased", Attrs: map[string]string{"language": "English", "kind": "Documentary"}},
		{Title: "Epithet Erased", Attrs: map[string]string{"language": "English", "kind": "Animation"}},
		{Title: "Boku Dake ga Inai Machi", Attrs: map[string]string{"language": "Japanese", "kind": "Animation"}},
	}
	got := preferCandidates(cfg, "anime", cands)
	if len(got) != 1 || got[0].Title != "Boku Dake ga Inai Machi" {
		t.Fatalf("got=%+v", got)
	}
}

func TestPreferCandidatesKeepsAllWhenNoneMatch(t *testing.T) {
	cfg := config.Config{Match: config.Match{Prefer: map[string]map[string]string{
		"anime": {"language": "Japanese", "kind": "Animation"},
	}}}
	cands := []Candidate{
		{Title: "History Erased", Attrs: map[string]string{"language": "English", "kind": "Documentary"}},
	}
	got := preferCandidates(cfg, "anime", cands)
	if len(got) != 1 || got[0].Title != "History Erased" {
		t.Fatalf("got=%+v", got)
	}
}

func TestAutoMatchSoloMinScore(t *testing.T) {
	anime := map[string]string{"language": "Japanese", "kind": "Animation"}
	cfg := config.Config{Match: config.Match{
		MinScore: 0.72, SoloMinScore: 0.01, MinMargin: 0.04,
		Prefer: map[string]map[string]string{"anime": anime},
	}}
	want := preferWant(cfg, "anime")
	if _, ok := autoMatch(cfg, Job{}, []Candidate{{Score: 0.05}}, nil); ok {
		t.Fatal("weak solo without prefer should not match")
	}
	if _, ok := autoMatch(cfg, Job{}, []Candidate{{Score: 0.05, Attrs: anime}}, want); ok {
		t.Fatal("prefer + weak solo should not match")
	}
	if _, ok := autoMatch(cfg, Job{}, []Candidate{{Score: 0, Attrs: anime}}, want); ok {
		t.Fatal("prefer-filtered solo with score 0 should not match")
	}
	if _, ok := autoMatch(cfg, Job{}, []Candidate{{Jaccard: 1}}, nil); !ok {
		t.Fatal("solo jaccard 1.0 should match")
	}
	if _, ok := autoMatch(cfg, Job{}, []Candidate{{Score: 1, Jaccard: 0}}, nil); !ok {
		t.Fatal("solo score 1.0 should match")
	}
	if pick, ok := autoMatch(cfg, Job{}, []Candidate{{Title: "The 100", Score: 1}, {Title: "Other", Score: 0}}, nil); !ok || pick.Title != "The 100" {
		t.Fatal("score 1.0 among 0.0 should match the winner")
	}
	if pick, ok := autoMatch(cfg, Job{}, []Candidate{
		{Title: "The 100", Score: 1, Jaccard: 0.80},
		{Title: "Girlfriends", Score: 0.31, Jaccard: 0.79},
	}, nil); !ok || pick.Title != "The 100" {
		t.Fatal("score gap should match when jaccard is close")
	}
	if _, ok := autoMatch(cfg, Job{}, []Candidate{{Jaccard: 0.72, Score: 0.50}}, nil); !ok {
		t.Fatal("jaccard 0.72 should match on min_score path")
	}
	if _, ok := autoMatch(cfg, Job{}, []Candidate{{Score: 0.72}}, nil); !ok {
		t.Fatal("solo score 0.72 should match")
	}
	if _, ok := autoMatch(cfg, Job{}, []Candidate{{Jaccard: 0.80, Score: 0.80}, {Jaccard: 0.79, Score: 0.79}}, nil); ok {
		t.Fatal("two close high scores should stay manual")
	}
	if _, ok := autoMatch(cfg, Job{}, []Candidate{{Score: 0.50, Jaccard: 0.50}, {Score: 0.10, Jaccard: 0.10}}, nil); ok {
		t.Fatal("two candidates still need min_score")
	}
	if pick, ok := autoMatch(cfg, Job{}, []Candidate{
		{Title: "Frieren: Beyond Journey's End", Score: 0.33, QueryCov: 1, Attrs: anime},
		{Title: "Sousou no Frieren", Score: 0.25, QueryCov: 1, Attrs: anime},
	}, want); !ok || pick.Title != "Frieren: Beyond Journey's End" {
		t.Fatal("prefer + full query coverage + margin should match")
	}
}

func TestAutoMatchUniqueQueryCov(t *testing.T) {
	cfg := config.Config{Match: config.Match{MinScore: 0.72, MinMargin: 0.04, PlotStop: testPlotStop}}
	if pick, ok := autoMatch(cfg, Job{Title: "Apothecary Diaries"}, []Candidate{
		{Title: "The Apothecary Diaries", Score: 0.48, Jaccard: 0.67, QueryCov: 1},
	}, nil); !ok || pick.Title != "The Apothecary Diaries" {
		t.Fatal("solo unique queryCov should match")
	}
	if pick, ok := autoMatch(cfg, Job{Title: "Anohana"}, []Candidate{
		{Title: "Anohana: The Flower We Saw That Day", Score: 0.10, QueryCov: 1},
		{Title: "Konohana Kitan", Score: 0, QueryCov: 0},
	}, nil); !ok || pick.Title != "Anohana: The Flower We Saw That Day" {
		t.Fatal("unique nickname queryCov should match")
	}
	if pick, ok := autoMatch(cfg, Job{Title: "Berserk The Golden Arc"}, []Candidate{
		{Title: "Berserk: The Golden Age Arc - Memorial Edition", Score: 0.41, QueryCov: 1},
	}, nil); !ok || pick.Title != "Berserk: The Golden Age Arc - Memorial Edition" {
		t.Fatal("solo memorial edition should unique-match")
	}
	if pick, ok := autoMatch(cfg, Job{Title: "Banished from the Hero's Party"}, []Candidate{
		{Title: "Banished from the Hero's Party, I Decided to Live a Quiet Life in the Countryside", Score: 0.31, QueryCov: 1},
		{Title: "Scooped Up by an S-Rank Adventurer!", Score: 0.30, QueryCov: 0},
	}, nil); !ok || !strings.HasPrefix(pick.Title, "Banished from the Hero's Party") {
		t.Fatal("unique official queryCov should beat scooped")
	}
	if _, ok := autoMatch(cfg, Job{Title: "the"}, []Candidate{
		{Title: "The 100", Score: 0.10, QueryCov: 1},
	}, nil); ok {
		t.Fatal("stopword-only job should not unique-match")
	}
	if _, ok := autoMatch(cfg, Job{Title: ""}, []Candidate{
		{Title: "The 100", Score: 0.10, QueryCov: 1},
	}, nil); ok {
		t.Fatal("empty job should not unique-match")
	}
	if _, ok := autoMatch(cfg, Job{Title: "Apothecary Diaries"}, []Candidate{
		{Title: "The Apothecary Diaries", Score: 0.48, QueryCov: 1},
		{Title: "The Apothecary Diaries Season 2", Score: 0.40, QueryCov: 1},
	}, nil); ok {
		t.Fatal("two full queryCov hits should stay manual")
	}
	if pick, ok := autoMatch(cfg, Job{Title: "Banished from the Hero's Party"}, []Candidate{
		{Title: "Banished from the Hero's Party, I Decided to Live a Quiet Life in the Countryside", Year: "2021", Score: 0.31, QueryCov: 1},
		{Title: "Shin no Nakama ja Nai to Yuusha no Party wo Oidasareta node", Year: "2021", Score: 0.31, QueryCov: 1, Attrs: map[string]string{
			"title_en": "Banished from the Hero's Party, I Decided to Live a Quiet Life in the Countryside",
		}},
		{Title: "Scooped Up by an S-Rank Adventurer!", Year: "2023", Score: 0.30, QueryCov: 0},
	}, nil); !ok || !strings.HasPrefix(pick.Title, "Banished from the Hero's Party") {
		t.Fatal("same covering title and year should unique-match")
	}
	if _, ok := autoMatch(cfg, Job{Title: "Banished from the Hero's Party"}, []Candidate{
		{Title: "Banished from the Hero's Party, I Decided to Live a Quiet Life in the Countryside", Year: "2021", Score: 0.31, QueryCov: 1},
		{Title: "Banished from the Hero's Party, I Decided to Live a Quiet Life in the Countryside Season 2", Year: "2024", Score: 0.28, QueryCov: 1},
	}, nil); ok {
		t.Fatal("season 2 is a second work")
	}
	if _, ok := autoMatch(cfg, Job{Title: "The Office"}, []Candidate{
		{Title: "The Office", Year: "2005", Score: 0.89, QueryCov: 1},
		{Title: "The Office", Year: "2001", Score: 0.89, QueryCov: 1},
	}, nil); ok {
		t.Fatal("same title different years should stay manual")
	}
}

func TestPreferCandidatesUntypedKeepsAll(t *testing.T) {
	cfg := config.Config{Match: config.Match{Prefer: map[string]map[string]string{
		"anime": {"language": "Japanese", "kind": "Animation"},
	}}}
	cands := []Candidate{
		{Title: "Arifureta Kiseki", Attrs: map[string]string{"language": "Japanese", "kind": "Scripted"}},
		{Title: "Arifureta: From Commonplace to World's Strongest", Attrs: map[string]string{"language": "Japanese", "kind": "Animation"}},
	}
	got := preferCandidates(cfg, "", cands)
	if len(got) != 2 {
		t.Fatalf("got=%+v", got)
	}
}

func hasCandidateTitle(cands []Candidate, title string) bool {
	for _, c := range cands {
		if c.Title == title {
			return true
		}
	}
	return false
}

func animeScanConfig(tvURL, jkURL string) config.Config {
	cfg := deferPairConfig(tvURL, jkURL)
	cfg.Match.Prefer = map[string]map[string]string{
		"anime": {"language": "Japanese", "kind": "Animation"},
	}
	tv := cfg.Providers["tvmaze"]
	tv.Types = []string{"tv", "anime", ""}
	tv.Fields["kind"] = "show.type"
	tv.Fields["language"] = "show.language"
	tv.Fields["synopsis"] = "show.summary"
	cfg.Providers["tvmaze"] = tv
	jk := cfg.Providers["jikan"]
	jk.Fields["title_en"] = "title_english"
	jk.Attrs = map[string]string{"language": "Japanese", "kind": "Animation"}
	cfg.Providers["jikan"] = jk
	return cfg
}

func TestRunOneArifuretaPrefersMainSeries(t *testing.T) {
	var jk atomic.Int32
	tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"show": map[string]any{"id": 1, "name": "Arifureta Kiseki", "premiered": "2008", "url": "http://t", "type": "Scripted", "language": "Japanese"}},
			map[string]any{"show": map[string]any{"id": 2, "name": "Arifureta: From Commonplace to World's Strongest", "premiered": "2019", "url": "http://t2", "type": "Animation", "language": "Japanese"}},
		})
	}))
	t.Cleanup(tvmaze.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jk.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{map[string]any{"mal_id": 1, "title": "Arifureta: From Commonplace to World's Strongest", "year": 2019, "url": "http://j"}},
		})
	}))
	t.Cleanup(jikan.Close)
	cfg := animeScanConfig(tvmaze.URL, jikan.URL)
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{Title: "Arifureta"})
	if job.Status != "manual" || job.Match != nil {
		t.Fatalf("status=%s match=%+v candidates=%+v", job.Status, job.Match, job.Candidates)
	}
	if !hasCandidateTitle(job.Candidates, "Arifureta Kiseki") {
		t.Fatalf("missing kiseki: %+v", job.Candidates)
	}
	if !hasCandidateTitle(job.Candidates, "Arifureta: From Commonplace to World's Strongest") {
		t.Fatalf("missing main series: %+v", job.Candidates)
	}
}

func TestRunOneErasedCallsJikan(t *testing.T) {
	var jk atomic.Int32
	tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"show": map[string]any{"id": 1, "name": "Crashed", "premiered": "2017", "url": "http://t", "type": "Variety", "language": "English"}},
			map[string]any{"show": map[string]any{"id": 2, "name": "Epithet Erased", "premiered": "2021", "url": "http://t2", "type": "Animation", "language": "English"}},
		})
	}))
	t.Cleanup(tvmaze.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jk.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{map[string]any{"mal_id": 31098, "title": "Boku Dake ga Inai Machi", "title_english": "Erased", "year": 2016, "url": "http://j"}},
		})
	}))
	t.Cleanup(jikan.Close)
	cfg := animeScanConfig(tvmaze.URL, jikan.URL)
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{Title: "Erased"})
	if jk.Load() != 1 {
		t.Fatalf("jikan hits=%d", jk.Load())
	}
	if job.Status != "matched" || job.Match == nil || job.Match.Title != "Boku Dake ga Inai Machi" {
		t.Fatalf("status=%s match=%+v candidates=%+v", job.Status, job.Match, job.Candidates)
	}
}

func TestRunOneFrierenBeatsFrieden(t *testing.T) {
	tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"show": map[string]any{"id": 1, "name": "Frieden", "premiered": "2020", "url": "http://t", "type": "Scripted", "language": "German"}},
			map[string]any{"show": map[string]any{"id": 2, "name": "Frieren: Beyond Journey's End", "premiered": "2023", "url": "http://t2", "type": "Animation", "language": "Japanese"}},
			map[string]any{"show": map[string]any{"id": 3, "name": "Sousou no Frieren", "premiered": "2023", "url": "http://t3", "type": "Animation", "language": "Japanese"}},
		})
	}))
	t.Cleanup(tvmaze.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(jikan.Close)
	cfg := animeScanConfig(tvmaze.URL, jikan.URL)
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{Title: "Frieren"})
	if job.Status != "manual" || job.Match != nil {
		t.Fatalf("status=%s match=%+v candidates=%+v", job.Status, job.Match, job.Candidates)
	}
	if len(job.Candidates) == 0 || job.Candidates[0].Title == "Frieden" {
		t.Fatalf("frieden first: %+v", job.Candidates)
	}
	if !hasCandidateTitle(job.Candidates, "Frieden") {
		t.Fatalf("missing frieden: %+v", job.Candidates)
	}
	if !hasCandidateTitle(job.Candidates, "Frieren: Beyond Journey's End") && !hasCandidateTitle(job.Candidates, "Sousou no Frieren") {
		t.Fatalf("missing frieren: %+v", job.Candidates)
	}
}

func TestRunOneTypedFrierenAutoMatches(t *testing.T) {
	tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"show": map[string]any{"id": 1, "name": "Frieden", "premiered": "2020", "url": "http://t", "type": "Scripted", "language": "German"}},
			map[string]any{"show": map[string]any{"id": 2, "name": "Frieren: Beyond Journey's End", "premiered": "2023", "url": "http://t2", "type": "Animation", "language": "Japanese"}},
			map[string]any{"show": map[string]any{"id": 3, "name": "Sousou no Frieren", "premiered": "2023", "url": "http://t3", "type": "Animation", "language": "Japanese"}},
		})
	}))
	t.Cleanup(tvmaze.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(jikan.Close)
	cfg := animeScanConfig(tvmaze.URL, jikan.URL)
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{Title: "Frieren", Type: "anime"})
	if job.Status != "matched" || job.Match == nil || job.Match.Title == "Frieden" {
		t.Fatalf("status=%s match=%+v candidates=%+v", job.Status, job.Match, job.Candidates)
	}
	if !hasCandidateTitle(job.Candidates, "Frieden") {
		t.Fatalf("missing frieden: %+v", job.Candidates)
	}
}

func TestRunOneScarletBondCallsJikan(t *testing.T) {
	var jk atomic.Int32
	tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"show": map[string]any{"id": 1, "name": "Beauty's Bone, Scarlet Sleeves", "premiered": "2026", "url": "http://t", "type": "Scripted", "language": "Chinese"}},
		})
	}))
	t.Cleanup(tvmaze.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jk.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{map[string]any{
				"mal_id":        10218,
				"title":         "Tensei shitara Slime Datta Ken Movie: Guren no Kizuna-hen",
				"title_english": "That Time I Got Reincarnated as a Slime the Movie: Scarlet Bond",
				"year":          2022,
				"url":           "http://j",
			}},
		})
	}))
	t.Cleanup(jikan.Close)
	cfg := animeScanConfig(tvmaze.URL, jikan.URL)
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{
		Title:  "Scarlet Bond",
		Parent: "That Time I Got Reincarnated as a Slime",
	})
	if jk.Load() != 1 {
		t.Fatalf("jikan hits=%d status=%s match=%+v", jk.Load(), job.Status, job.Match)
	}
	if job.Status != "matched" || job.Match == nil || job.Match.Provider != "jikan" {
		t.Fatalf("status=%s match=%+v candidates=%+v", job.Status, job.Match, job.Candidates)
	}
	if len(job.Candidates) == 0 || job.Candidates[0].Provider != "jikan" {
		t.Fatalf("jikan first: %+v", job.Candidates)
	}
	if !hasCandidateTitle(job.Candidates, "Beauty's Bone, Scarlet Sleeves") {
		t.Fatalf("missing tvmaze: %+v", job.Candidates)
	}
}

func TestRunOneSinbadPrefersMagi(t *testing.T) {
	tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"show": map[string]any{"id": 1, "name": "The Adventures of Sinbad", "premiered": "1996", "url": "http://t", "type": "Scripted", "language": "English"}},
			map[string]any{"show": map[string]any{"id": 2, "name": "Magi: Sinbad no Bouken", "premiered": "2016", "url": "http://t2", "type": "Animation", "language": "Japanese"}},
		})
	}))
	t.Cleanup(tvmaze.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(jikan.Close)
	cfg := animeScanConfig(tvmaze.URL, jikan.URL)
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{
		Title:  "adventure of sinbad",
		Parent: "Magi The Labyrinth of Magic",
	})
	if job.Status != "manual" || job.Match != nil {
		t.Fatalf("status=%s match=%+v candidates=%+v", job.Status, job.Match, job.Candidates)
	}
	if !hasCandidateTitle(job.Candidates, "Magi: Sinbad no Bouken") {
		t.Fatalf("missing magi: %+v", job.Candidates)
	}
	if !hasCandidateTitle(job.Candidates, "The Adventures of Sinbad") {
		t.Fatalf("missing adventures: %+v", job.Candidates)
	}
}

func TestRunOneThe100KeepsScriptedHit(t *testing.T) {
	const girlfriends = "The 100 Girlfriends Who Really, Really, Really, Really, Really Love You"
	tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"show": map[string]any{"id": 6, "name": "The 100", "premiered": "2014", "url": "http://t", "type": "Scripted", "language": "English", "summary": "The 100 survivors land on Earth."}},
			map[string]any{"show": map[string]any{"id": 70514, "name": girlfriends, "premiered": "2023", "url": "http://t2", "type": "Animation", "language": "Japanese", "summary": "A boy is confessed to by 100 girlfriends."}},
		})
	}))
	t.Cleanup(tvmaze.Close)
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(jikan.Close)
	cfg := animeScanConfig(tvmaze.URL, jikan.URL)
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{Title: "The 100"})
	if job.Status != "matched" || job.Match == nil || job.Match.Title != "The 100" || job.Match.ID != "6" {
		t.Fatalf("status=%s match=%+v candidates=%+v", job.Status, job.Match, job.Candidates)
	}
	if !hasCandidateTitle(job.Candidates, "The 100") || !hasCandidateTitle(job.Candidates, girlfriends) {
		t.Fatalf("candidates=%+v", job.Candidates)
	}
	if job.Candidates[0].Title != "The 100" || job.Candidates[0].Jaccard != 1 || job.Candidates[0].Score != exactScore() {
		t.Fatalf("first=%+v", job.Candidates[0])
	}
	for _, c := range job.Candidates {
		if c.Title == girlfriends && c.Score >= 1 {
			t.Fatalf("girlfriends score=%v", c.Score)
		}
	}
}

func TestRunOneUniqueQueryCov(t *testing.T) {
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(jikan.Close)

	run := func(title string, shows []any) Job {
		tvmaze := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(shows)
		}))
		t.Cleanup(tvmaze.Close)
		return runOne(context.Background(), animeScanConfig(tvmaze.URL, jikan.URL), newHTTP(animeScanConfig(tvmaze.URL, jikan.URL)), Job{Title: title})
	}

	apothecary := run("Apothecary Diaries", []any{
		map[string]any{"show": map[string]any{"id": 1, "name": "The Apothecary Diaries", "url": "http://a"}},
	})
	if apothecary.Status != "matched" || apothecary.Match == nil || apothecary.Match.Title != "The Apothecary Diaries" {
		t.Fatalf("apothecary: status=%s match=%+v", apothecary.Status, apothecary.Match)
	}

	anohana := run("Anohana", []any{
		map[string]any{"show": map[string]any{"id": 1, "name": "Anohana: The Flower We Saw That Day", "url": "http://a"}},
		map[string]any{"show": map[string]any{"id": 2, "name": "Konohana Kitan", "url": "http://k"}},
	})
	if anohana.Status != "matched" || anohana.Match == nil || anohana.Match.Title != "Anohana: The Flower We Saw That Day" {
		t.Fatalf("anohana: status=%s match=%+v", anohana.Status, anohana.Match)
	}
	if !hasCandidateTitle(anohana.Candidates, "Konohana Kitan") {
		t.Fatalf("anohana should keep both: %+v", anohana.Candidates)
	}

	berserk := run("Berserk The Golden Arc", []any{
		map[string]any{"show": map[string]any{"id": 1, "name": "Berserk: The Golden Age Arc - Memorial Edition", "url": "http://b"}},
	})
	if berserk.Status != "matched" || berserk.Match == nil || berserk.Match.Title != "Berserk: The Golden Age Arc - Memorial Edition" {
		t.Fatalf("berserk: status=%s match=%+v", berserk.Status, berserk.Match)
	}

	banished := run("Banished from the Hero's Party", []any{
		map[string]any{"show": map[string]any{"id": 1, "name": "Banished from the Hero's Party, I Decided to Live a Quiet Life in the Countryside", "url": "http://b"}},
		map[string]any{"show": map[string]any{"id": 2, "name": "Scooped Up by an S-Rank Adventurer!", "url": "http://s"}},
	})
	if banished.Status != "matched" || banished.Match == nil || !strings.HasPrefix(banished.Match.Title, "Banished from the Hero's Party") {
		t.Fatalf("banished: status=%s match=%+v", banished.Status, banished.Match)
	}
	if !hasCandidateTitle(banished.Candidates, "Scooped Up by an S-Rank Adventurer!") {
		t.Fatalf("banished should keep both: %+v", banished.Candidates)
	}

	two := run("Apothecary Diaries", []any{
		map[string]any{"show": map[string]any{"id": 1, "name": "The Apothecary Diaries", "url": "http://a"}},
		map[string]any{"show": map[string]any{"id": 2, "name": "The Apothecary Diaries Season 2", "url": "http://a2"}},
	})
	if two.Status != "manual" {
		t.Fatalf("two queryCov=1 should stay manual: status=%s match=%+v", two.Status, two.Match)
	}
}

func fastSlowSearch(base string) config.Provider {
	return config.Provider{
		Types:  []string{"tv", ""},
		Base:   base,
		URL:    "{base}/search/shows",
		Query:  map[string]string{"q": "{title}"},
		Items:  "$",
		Fields: map[string]string{"id": "show.id", "title": "show.name", "year": "show.premiered", "url": "show.url"},
	}
}

func fastSlowCfg(fastURL, slowURL string, deferSlow bool) config.Config {
	slow := fastSlowSearch(slowURL)
	slow.Defer = deferSlow
	return config.Config{
		HTTP: config.HTTP{TimeoutMS: 5000, Retries: 1, ProviderTimeoutMS: 200, Backoff: config.ExpRange{MinExp: 0, MaxExp: 2}},
		Match: config.Match{
			MinScore: 0.72, MinMargin: 0.04,
			CooldownFails: 1,
			Cooldown:      config.ExpRange{MinExp: 10, MaxExp: 12},
			PlotStop:      testPlotStop,
		},
		Providers: map[string]config.Provider{
			"fast": fastSlowSearch(fastURL),
			"slow": slow,
		},
	}
}

func TestRunOneRanksWhenDeferredCooling(t *testing.T) {
	var slowN atomic.Int32
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"score": 1, "show": map[string]any{"id": 1, "name": "Other Name", "premiered": "1999", "url": "http://t"}},
		})
	}))
	t.Cleanup(fast.Close)
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slowN.Add(1)
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	t.Cleanup(slow.Close)
	cfg := fastSlowCfg(fast.URL, slow.URL, true)
	cool := NewCircuit()
	ctx := WithCircuit(context.Background(), cool)
	httpc := newHTTP(cfg)
	job := Job{Title: "Title"}
	first := runOne(ctx, cfg, httpc, job)
	if first.Status == "error" {
		t.Fatalf("first status=%s err=%s", first.Status, first.Error)
	}
	if first.Error != "" {
		t.Fatalf("first error=%q", first.Error)
	}
	hits := slowN.Load()
	if hits < 1 {
		t.Fatalf("slow hits=%d", hits)
	}
	second := runOne(ctx, cfg, httpc, job)
	if second.Status == "error" {
		t.Fatalf("second status=%s err=%s", second.Status, second.Error)
	}
	if second.Error != "" {
		t.Fatalf("second error=%q", second.Error)
	}
	if slowN.Load() != hits {
		t.Fatalf("slow called during cooldown: before=%d after=%d", hits, slowN.Load())
	}
}

func TestRunOneAllCooldownIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	t.Cleanup(srv.Close)
	cfg := fastSlowCfg(srv.URL, srv.URL, false)
	cool := NewCircuit()
	ctx := WithCircuit(context.Background(), cool)
	httpc := newHTTP(cfg)
	job := Job{Title: "Title"}
	first := runOne(ctx, cfg, httpc, job)
	if first.Status != "error" {
		t.Fatalf("first status=%s err=%s", first.Status, first.Error)
	}
	second := runOne(ctx, cfg, httpc, job)
	if second.Status != "error" {
		t.Fatalf("second status=%s err=%s", second.Status, second.Error)
	}
	if !strings.Contains(second.Error, "fast: cooldown") || !strings.Contains(second.Error, "slow: cooldown") {
		t.Fatalf("error=%q", second.Error)
	}
}

func TestRunOneAllHardFailIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	t.Cleanup(srv.Close)
	cfg := fastSlowCfg(srv.URL, srv.URL, false)
	cfg.Match.CooldownFails = 99
	job := runOne(context.Background(), cfg, newHTTP(cfg), Job{Title: "Title"})
	if job.Status != "error" {
		t.Fatalf("status=%s err=%s", job.Status, job.Error)
	}
	if !strings.Contains(job.Error, "fast:") || !strings.Contains(job.Error, "slow:") {
		t.Fatalf("error=%q", job.Error)
	}
	if strings.Contains(job.Error, "cooldown") {
		t.Fatalf("unexpected cooldown: %q", job.Error)
	}
}
