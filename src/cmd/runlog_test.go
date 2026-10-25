package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/match"
)

func TestRunLogFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "matchmedia.log")
	rl, err := openRunLog(path)
	if err != nil {
		t.Fatal(err)
	}
	rl.header("MatchMedia")
	rl.lines("path: /lib", "mode: rescan")
	rl.header("scan")
	rl.lines(itemCount(2))
	rl.header("match")
	rl.lines(queueLines([]match.Job{
		{Title: "Show", Path: "/lib/Show", Status: "pending"},
		{Title: "S01", Path: "/lib/Show/S01", Status: "matched"},
	})...)
	rl.errorJob(match.Job{Title: "Skip", Status: "matched"})
	rl.errorJob(match.Job{Title: "Tokyo Ghoul", Path: "/lib/Tokyo Ghoul", Status: "error", Error: "tvmaze: status 429"})
	rl.errorJob(match.Job{Title: "A Journey", Path: "/lib/A Journey", Status: "error", Error: "tvmaze: status 429"})
	rl.header("report")
	rl.lines("wrote match.md")
	if err := rl.Close(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"MatchMedia",
		helpSep,
		"path: /lib",
		"mode: rescan",
		"",
		"scan",
		helpSep,
		"2 items",
		"",
		"match",
		helpSep,
		"2 items",
		"Show\t/lib/Show\tpending",
		"S01\t/lib/Show/S01\tmatched",
		"",
		"Tokyo Ghoul",
		"path: /lib/Tokyo Ghoul",
		"error: tvmaze: status 429",
		"",
		"A Journey",
		"path: /lib/A Journey",
		"error: tvmaze: status 429",
		"",
		"report",
		helpSep,
		"wrote match.md",
		"",
	}, "\n")
	if string(body) != want {
		t.Fatalf("log:\n%s\nwant:\n%s", body, want)
	}
}
