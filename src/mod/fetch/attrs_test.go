package fetch

import (
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
)

func TestAttrStringJoinsLists(t *testing.T) {
	genres := []any{
		map[string]any{"id": 18.0, "name": "Drama"},
		map[string]any{"id": 35.0, "name": "Comedy"},
	}
	if got := attrString(genres); got != "Drama, Comedy" {
		t.Fatalf("genres %q", got)
	}
	countries := []any{"US", "JP"}
	if got := attrString(countries); got != "US, JP" {
		t.Fatalf("countries %q", got)
	}
	if got := attrString(map[string]any{"average": 8.5}); got != "8.5" {
		t.Fatalf("rating %q", got)
	}
}

func TestExtraAttrsSkipsZeroRuntime(t *testing.T) {
	item := map[string]any{
		"genres":       []any{map[string]any{"name": "Adventure"}},
		"runtime":      0.0,
		"vote_average": 7.25,
	}
	got := extraAttrs(map[string]string{
		"genres":   "genres",
		"runtime":  "runtime",
		"rating":   "vote_average",
		"synopsis": "overview",
	}, item)
	if got["genres"] != "Adventure" || got["rating"] != "7.25" {
		t.Fatalf("%#v", got)
	}
	if _, ok := got["runtime"]; ok {
		t.Fatalf("runtime %#v", got)
	}
	if _, ok := got["synopsis"]; ok {
		t.Fatal("synopsis is a core field")
	}
}

func TestMergeDetailCopiesAttrs(t *testing.T) {
	cand := &rank.Hit{Title: "Show", Attrs: map[string]string{"language": "ja"}}
	ep := &config.Episode{Fields: map[string]string{
		"synopsis":  "overview",
		"genres":    "genres",
		"runtime":   "runtime",
		"premiered": "first_air_date",
	}}
	item := map[string]any{
		"overview":       "A plot.",
		"genres":         []any{map[string]any{"name": "Fantasy"}},
		"runtime":        24.0,
		"first_air_date": "2024-07-07",
	}
	mergeDetail(cand, ep, item, 100)
	if cand.Synopsis != "A plot." {
		t.Fatalf("synopsis %q", cand.Synopsis)
	}
	if cand.Attrs["genres"] != "Fantasy" || cand.Attrs["runtime"] != "24" || cand.Attrs["premiered"] != "2024-07-07" {
		t.Fatalf("%#v", cand.Attrs)
	}
	if cand.Year != "2024" {
		t.Fatalf("year from premiered %q", cand.Year)
	}
	if cand.Attrs["language"] != "ja" {
		t.Fatalf("kept language %#v", cand.Attrs)
	}
	dst := &rank.Hit{}
	applyDetail(dst, *cand)
	if dst.Attrs["genres"] != "Fantasy" || dst.Synopsis != "A plot." {
		t.Fatalf("apply %#v %q", dst.Attrs, dst.Synopsis)
	}
}

func TestHitFromYearFromPremiered(t *testing.T) {
	spec := config.Provider{
		Year: "prefix4",
		Fields: map[string]string{
			"id": "id", "title": "title", "year": "release_date",
			"premiered": "release_date", "rating": "vote_average",
		},
	}
	item := map[string]any{
		"id": 1.0, "title": "Film", "release_date": "2016-07-16", "vote_average": 8.1,
	}
	h, ok := hitFrom("tmdb", spec, item, 100)
	if !ok {
		t.Fatal("not ok")
	}
	if h.Year != "2016" || h.Attrs["premiered"] != "2016-07-16" {
		t.Fatalf("%#v", h)
	}
}

func TestNormalizePremieredAttrsFoldsAliases(t *testing.T) {
	got := normalizePremieredAttrs(map[string]string{
		"release_date": "2014-07-06",
		"language":     "ja",
	})
	if got["premiered"] != "2014-07-06" || got["language"] != "ja" {
		t.Fatalf("%#v", got)
	}
	if _, ok := got["release_date"]; ok {
		t.Fatalf("alias leaked %#v", got)
	}
}

func TestMergeDetailYearFromReleasedAlias(t *testing.T) {
	cand := &rank.Hit{Title: "Film"}
	ep := &config.Episode{Fields: map[string]string{
		"synopsis": "Plot",
		"premiered": "Released",
	}}
	item := map[string]any{
		"Plot":     "Hello",
		"Released": "16 Jul 2016",
	}
	mergeDetail(cand, ep, item, 100)
	// OMDb Released is not ISO; still stored under premiered. Year only from ISO-ish prefix4.
	if cand.Attrs["premiered"] != "16 Jul 2016" {
		t.Fatalf("%#v", cand.Attrs)
	}
}

func TestEpisodeFromRuntimeAndAirdate(t *testing.T) {
	list := &config.CatalogList{Fields: map[string]string{
		"id":       "id",
		"number":   "episode_number",
		"title":    "name",
		"synopsis": "overview",
		"runtime":  "runtime",
		"airdate":  "air_date",
	}}
	item := map[string]any{
		"id":             9.0,
		"episode_number": 1.0,
		"name":           "Pilot",
		"overview":       "<p>Hello</p>",
		"runtime":        24.0,
		"air_date":       "2024-07-07",
	}
	ep, ok := episodeFrom(config.Provider{}, list, item, 100)
	if !ok {
		t.Fatal("not ok")
	}
	if ep.Synopsis != "Hello" || ep.Runtime != "24" || ep.Aired != "2024-07-07" {
		t.Fatalf("%#v", ep)
	}
	item["runtime"] = 0.0
	item["air_date"] = ""
	ep, ok = episodeFrom(config.Provider{}, list, item, 100)
	if !ok || ep.Runtime != "" || ep.Aired != "" {
		t.Fatalf("%#v %v", ep, ok)
	}
}
