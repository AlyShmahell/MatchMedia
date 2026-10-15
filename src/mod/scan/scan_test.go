package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alyshmahell/matchmedia/src/mod/config"
)

func TestRunGroupsAndModes(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	show := filepath.Join(root, "Example Show")
	touch(t, filepath.Join(show, "Season 1", "Example Show S01E01.mkv"))
	touch(t, filepath.Join(show, "My Movie", "My Movie Title.mkv"))

	jobs, err := Run(cfg, root, ModeRescan)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) < 2 {
		t.Fatalf("jobs %d", len(jobs))
	}
	var showJob, movieJob bool
	for _, j := range jobs {
		if j.Status != "pending" {
			t.Fatalf("rescan status %s", j.Status)
		}
		if j.Title == "Example Show" && j.Kind == "show" {
			showJob = true
			if len(j.Files) != 1 || j.Files[0].Season != "1" {
				t.Fatalf("show files %#v", j.Files)
			}
		}
		if j.Title == "My Movie" && j.Kind == "movie" {
			movieJob = true
		}
	}
	if !showJob || !movieJob {
		t.Fatalf("titles %#v", jobs)
	}

	nfo := filepath.Join(show, "tvshow.nfo")
	if err := os.WriteFile(nfo, []byte(`<tvshow><title>Example Show</title><year>1998</year></tvshow>`), 0o644); err != nil {
		t.Fatal(err)
	}
	missing, err := Run(cfg, root, ModeMissing)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range missing {
		if j.Title == "Example Show" {
			t.Fatal("missing must skip a title with title NFO")
		}
	}

	nfoJobs, err := Run(cfg, root, ModeNFO)
	if err != nil {
		t.Fatal(err)
	}
	var unmatchedMovie bool
	for _, j := range nfoJobs {
		if j.Title == "My Movie" && j.Status == "unmatched" {
			unmatchedMovie = true
		}
	}
	if !unmatchedMovie {
		t.Fatalf("nfo jobs %#v", nfoJobs)
	}
}

func TestSplitTitleYear(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	touch(t, filepath.Join(root, "A Silent Voice (2016)", "A Silent Voice (2016).mkv"))
	jobs, err := Run(cfg, root, ModeRescan)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs %#v", jobs)
	}
	if jobs[0].Title != "A Silent Voice" || jobs[0].Year != "2016" {
		t.Fatalf("title=%q year=%q", jobs[0].Title, jobs[0].Year)
	}
}

func TestSplitTitleYearKeepsBareYear(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	touch(t, filepath.Join(root, "Blade Runner 2049.mkv"))
	jobs, err := Run(cfg, root, ModeRescan)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs %#v", jobs)
	}
	if jobs[0].Title != "Blade Runner 2049" || jobs[0].Year != "" {
		t.Fatalf("title=%q year=%q", jobs[0].Title, jobs[0].Year)
	}
}

func TestRunProgress(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	touch(t, filepath.Join(root, "Example Show", "Season 1", "Example Show S01E01.mkv"))
	touch(t, filepath.Join(root, "notes.txt"))
	var n int
	var last string
	jobs, err := RunProgress(cfg, root, ModeRescan, func(done, total int, name string) {
		n++
		last = name
		if total != 2 {
			t.Fatalf("total %d", total)
		}
		if done != n {
			t.Fatalf("done %d n %d", done, n)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("callbacks %d last %s jobs %d", n, last, len(jobs))
	}
}

func TestNormalizeMode(t *testing.T) {
	got, err := NormalizeMode("")
	if err != nil || got != ModeRescan {
		t.Fatal(got, err)
	}
	if _, err := NormalizeMode("nope"); err == nil {
		t.Fatal("want error")
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}
