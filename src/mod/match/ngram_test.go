package match

import (
	"strings"
	"testing"
)

func TestWordNgrams(t *testing.T) {
	hero := wordNgrams("Hero Without a Class Who Even Need Skills")
	if len(hero) == 0 || hero[0] != "hero without a class who even need" {
		t.Fatalf("hero first %#v", hero)
	}
	if wordNgrams("A Silent Voice") != nil {
		t.Fatalf("3 tokens: %#v", wordNgrams("A Silent Voice"))
	}
	got := wordNgrams("An Adventure's Daily Grind at Age 29")
	if len(got) == 0 {
		t.Fatal("adventure ngrams empty")
	}
	for _, p := range got {
		for _, tok := range strings.Fields(p) {
			if tok == "s" {
				t.Fatalf("kept 1-char token in %q", p)
			}
		}
	}
	want := "daily grind at age 29"
	found := false
	for _, p := range got {
		if p == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing %q in %#v", want, got)
	}
}
