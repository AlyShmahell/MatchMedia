package rank

import (
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/config"
)

func TestRankExactTitle(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	hits := Rank(cfg, Query{Title: "Cowboy Bebop"}, []Hit{
		{Title: "Other Show", Year: "1998"},
		{Title: "Cowboy Bebop", Year: "1998"},
	})
	if hits[0].Title != "Cowboy Bebop" {
		t.Fatalf("best %s", hits[0].Title)
	}
	if hits[0].Jaccard != 1 {
		t.Fatalf("jaccard %v", hits[0].Jaccard)
	}
	best, ok := Auto(cfg, Query{Title: "Cowboy Bebop"}, hits)
	if !ok || best.Title != "Cowboy Bebop" {
		t.Fatalf("auto %#v ok=%v", best, ok)
	}
}
