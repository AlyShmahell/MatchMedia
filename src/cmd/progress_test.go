package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/match"
)

func TestTallyFoldedJobs(t *testing.T) {
	check := func(t *testing.T, jobs []match.Job, wantMatched, wantUnmatched int) {
		t.Helper()
		matched, manual, unmatched, errored := tally(jobs)
		if matched != wantMatched || manual != 0 || unmatched != wantUnmatched || errored != 0 {
			t.Fatalf("tally matched=%d manual=%d unmatched=%d error=%d", matched, manual, unmatched, errored)
		}
		got := renderReport(jobs)
		want := fmt.Sprintf("jobs: %d · ✅ matched: %d · ✋ manual: %d · ❌ unmatched: %d · 💥 error: %d",
			len(jobs), matched, manual, unmatched, errored)
		if !strings.Contains(got, want) {
			t.Fatalf("report counts %q missing in:\n%s", want, got)
		}
	}
	parent := match.Job{
		Title:  "Show",
		Status: "matched",
		Kind:   "show",
		Match:  &match.Candidate{Title: "Show", ID: "1"},
		Files:  []match.JobFile{{Path: "s01.mkv"}},
	}
	t.Run("dropped child", func(t *testing.T) {
		check(t, []match.Job{parent}, 1, 0)
	})
	t.Run("leftover unmatched", func(t *testing.T) {
		check(t, []match.Job{parent, {
			Title:  "Leftover",
			Status: "unmatched",
			Kind:   "movie",
			Files:  []match.JobFile{{Path: "x.mkv"}},
		}}, 1, 1)
	})
}

func TestLiveCountsFrom(t *testing.T) {
	var matched, sidecar, replay, manual, unmatched, errored int
	liveCountsFrom(match.Job{Title: "S01", Parent: "Show", Status: "unmatched"}, &matched, &sidecar, &replay, &manual, &unmatched, &errored)
	liveCountsFrom(match.Job{Title: "S02", Parent: "Show", Status: "error"}, &matched, &sidecar, &replay, &manual, &unmatched, &errored)
	if unmatched != 0 || errored != 0 {
		t.Fatalf("child fail counted unmatched=%d error=%d", unmatched, errored)
	}
	liveCountsFrom(match.Job{Title: "S01", Parent: "Show", Status: "matched"}, &matched, &sidecar, &replay, &manual, &unmatched, &errored)
	if matched != 1 {
		t.Fatalf("matched child %d", matched)
	}
	liveCountsFrom(match.Job{Title: "Saved", Status: "matched", Ranker: "sidecar"}, &matched, &sidecar, &replay, &manual, &unmatched, &errored)
	liveCountsFrom(match.Job{Title: "Again", Status: "matched", Ranker: "replay"}, &matched, &sidecar, &replay, &manual, &unmatched, &errored)
	if matched != 1 || sidecar != 1 || replay != 1 {
		t.Fatalf("matched=%d sidecar=%d replay=%d", matched, sidecar, replay)
	}
	liveCountsFrom(match.Job{Title: "Leftover", Status: "unmatched"}, &matched, &sidecar, &replay, &manual, &unmatched, &errored)
	if unmatched != 1 {
		t.Fatalf("title unmatched %d", unmatched)
	}
}

func TestRenderProgressPlain(t *testing.T) {
	got := render(snapshot{
		Phase:     "match",
		Done:      12,
		Total:     40,
		Matched:   9,
		Sidecar:   4,
		Replay:    3,
		Manual:    2,
		Unmatched: 1,
		Error:     0,
		Folded:    6,
		Title:     "Cowboy Bebop",
	}, false, 160)
	if strings.Contains(got, "\x1b") {
		t.Fatalf("ansi in plain: %q", got)
	}
	for _, want := range []string{"match", "12/40", "matched:9", "manual:2", "unmatched:1", "error:0", "sidecar:4", "replay:3", "folded:6", "Cowboy Bebop"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestRenderProgressColor(t *testing.T) {
	got := render(snapshot{Phase: "match", Done: 1, Total: 2, Matched: 1, Title: "X"}, true, 120)
	if !strings.Contains(got, ansiCyan) || !strings.Contains(got, ansiGreen) || !strings.Contains(got, ansiYellow) || !strings.Contains(got, ansiRed) {
		t.Fatalf("missing colors: %q", got)
	}
}

func TestRenderProgressTruncates(t *testing.T) {
	got := render(snapshot{Phase: "scan", Done: 1, Total: 1, Title: strings.Repeat("A", 200)}, false, 40)
	if strings.Contains(got, "\x1b") {
		t.Fatal("ansi")
	}
	if strings.Count(got, "A") > 40 {
		t.Fatalf("too long %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("want ellipsis %q", got)
	}
}

func TestUseColorNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if useColor(true) {
		t.Fatal("NO_COLOR must disable color")
	}
	t.Setenv("NO_COLOR", "")
	if !useColor(true) {
		t.Fatal("tty without NO_COLOR uses color")
	}
	if useColor(false) {
		t.Fatal("non-tty")
	}
}

func TestRenderSaveCounts(t *testing.T) {
	got := render(snapshot{Phase: "save", Done: 3, Total: 10, NFO: 3, EpisodeNFO: 12, Posters: 2, Title: "Show"}, false, 120)
	if strings.Contains(got, "\x1b") {
		t.Fatalf("ansi in plain: %q", got)
	}
	for _, want := range []string{"save", "3/10", "nfo:3", "episode:12", "poster:2", "Show"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	color := render(snapshot{Phase: "save", Done: 1, Total: 1, NFO: 1}, true, 120)
	if !strings.Contains(color, ansiGreen) || !strings.Contains(color, ansiYellow) || !strings.Contains(color, ansiCyan) {
		t.Fatalf("missing save colors: %q", color)
	}
}

func TestDrawNoTTY(t *testing.T) {
	var buf bytes.Buffer
	b := &bar{w: &buf, tty: false, color: false, width: 80}
	b.draw(snapshot{Phase: "match", Done: 1, Total: 1, Title: "X"})
	if buf.Len() != 0 {
		t.Fatalf("drew %q", buf.String())
	}
	b.logo()
	b.finish()
	if buf.Len() != 0 {
		t.Fatalf("finished %q", buf.String())
	}
}

func TestLogoNoColor(t *testing.T) {
	var buf bytes.Buffer
	b := &bar{w: &buf, tty: true, color: false, width: 80}
	b.logo()
	if buf.Len() != 0 {
		t.Fatalf("logo without color %q", buf.String())
	}
}

func TestRenderLogoHalfBlock(t *testing.T) {
	img, err := decodeLogo()
	if err != nil {
		t.Fatal(err)
	}
	got := renderLogo(img)
	if !strings.Contains(got, "\x1b[38;2;") {
		t.Fatalf("missing truecolor: %q", got[:min(80, len(got))])
	}
	if !strings.Contains(got, "▀") {
		t.Fatal("missing half-block")
	}
}

func TestRenderKofiHalfBlock(t *testing.T) {
	img, err := decodeKofi()
	if err != nil {
		t.Fatal(err)
	}
	got := renderLogo(img)
	if !strings.Contains(got, "\x1b[38;2;") {
		t.Fatalf("missing truecolor: %q", got[:min(80, len(got))])
	}
	if !strings.Contains(got, "▀") {
		t.Fatal("missing half-block")
	}
}

func TestDrawTTY(t *testing.T) {
	var buf bytes.Buffer
	b := &bar{w: &buf, tty: true, color: false, width: 80}
	b.draw(snapshot{Phase: "save", Done: 1, Total: 2, Title: "Film"})
	got := buf.String()
	if !strings.Contains(got, "\r") || !strings.Contains(got, "1/2") || !strings.Contains(got, "Film") {
		t.Fatalf("%q", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("same phase newline %q", got)
	}
	b.draw(snapshot{Phase: "report", Done: 1, Total: 1, Title: "out.md"})
	got = buf.String()
	if !strings.Contains(got, "\n") || !strings.Contains(got, "report") {
		t.Fatalf("phase change %q", got)
	}
	b.finish()
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Fatalf("finish %q", buf.String())
	}
}
