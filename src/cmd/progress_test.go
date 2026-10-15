package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderProgressPlain(t *testing.T) {
	got := render(snapshot{
		Phase:     "match",
		Done:      12,
		Total:     40,
		Matched:   9,
		Manual:    2,
		Unmatched: 1,
		Error:     0,
		Title:     "Cowboy Bebop",
	}, false, 120)
	if strings.Contains(got, "\x1b") {
		t.Fatalf("ansi in plain: %q", got)
	}
	for _, want := range []string{"match", "12/40", "matched:9", "manual:2", "unmatched:1", "error:0", "Cowboy Bebop"} {
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
