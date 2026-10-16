package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIHelp(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "-help")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "MatchMedia") {
		t.Fatalf("missing title in %s", got)
	}
	if ui, si := strings.Index(got, "Usage"), strings.Index(got, "Support MatchMedia"); ui < 0 || si < 0 || ui > si {
		t.Fatalf("Usage should come before Support: %s", got)
	}
	for _, want := range []string{
		"Support MatchMedia",
		"ko-fi.com/alyshmahell",
		"Usage",
		"License",
		"Attribution",
		"-path",
		"-overlay",
		"BSD 3-Clause",
		"This product uses the TMDb API but is not endorsed or certified by TMDb.",
		"themoviedb.org",
		"tvmaze.com",
		"omdbapi.com",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, "wrote ") {
		t.Fatalf("scanned on -help: %s", got)
	}
}

func TestCLIScan(t *testing.T) {
	root := t.TempDir()
	show := filepath.Join(root, "Show")
	if err := os.MkdirAll(filepath.Join(show, "Season 1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "Season 1", "Show S01E01.mkv"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(root, "out.md")
	cmd := exec.Command("go", "run", ".", "-path", root, "-mode", "nfo", "-report", report)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	want := "wrote " + report
	if !strings.Contains(string(out), want) {
		t.Fatalf("output %s", out)
	}
	body, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "Show") {
		t.Fatalf("report %s", got)
	}
	if !strings.Contains(got, "| Show |") || !strings.Contains(got, "unmatched") {
		t.Fatalf("nfo without sidecar should be unmatched:\n%s", got)
	}
}
