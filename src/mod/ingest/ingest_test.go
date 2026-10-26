package ingest

import (
	"strings"
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/config"
)

func TestParseCSVJSON(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := Parse(cfg, strings.NewReader("title,year\nCowboy Bebop,1998\n"), "t.csv", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Title != "Cowboy Bebop" || rows[0].Year != "1998" {
		t.Fatalf("%#v", rows)
	}
	rows, err = Parse(cfg, strings.NewReader(`[{"title":"X","type":"film"}]`), "t.json", "application/json")
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Type != "movie" {
		t.Fatalf("type %s", rows[0].Type)
	}
	jobs := Jobs(rows)
	if jobs[0].Kind != "movie" || jobs[0].Status != "pending" {
		t.Fatalf("%#v", jobs[0])
	}
}
