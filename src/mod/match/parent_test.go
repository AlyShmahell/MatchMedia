package match

import (
	"path/filepath"
	"reflect"
	"testing"
)

func showChildren() (Job, Job) {
	a := Job{
		Title:  "S01",
		Parent: "Show",
		Path:   "/media/Show/S01",
		Kind:   "movie",
		Status: "pending",
		Source: "scan",
		Files:  []JobFile{{Path: "s01e01.mkv"}, {Path: "s01e02.mkv"}},
	}
	b := Job{
		Title:  "S02",
		Parent: "Show",
		Path:   "/media/Show/S02",
		Kind:   "movie",
		Status: "pending",
		Source: "scan",
		Files:  []JobFile{{Path: "s02e01.mkv"}},
	}
	return a, b
}

func TestParentJobsFirst(t *testing.T) {
	a, b := showChildren()
	got := parentJobs([]Job{a, b})
	if len(got) != 1 {
		t.Fatalf("parents %d: %#v", len(got), got)
	}
	p := got[0]
	if p.Title != "Show" || p.Path != filepath.Clean("/media/Show") {
		t.Fatalf("parent %#v", p)
	}
	if p.Status != "pending" || p.Kind != "show" || len(p.Files) != 0 {
		t.Fatalf("status/kind/files %#v", p)
	}
}

func TestParentJobsSkipExistingPath(t *testing.T) {
	if got := parentJobs([]Job{
		{Title: "Show", Parent: "Show", Path: "/media/Show", Status: "pending"},
		{Title: "S01", Parent: "Show", Path: "/media/Show/S01", Status: "pending"},
	}); len(got) != 0 {
		t.Fatalf("existing folder path: %#v", got)
	}
	if got := parentJobs([]Job{
		{Title: "Show", Parent: "Show", Path: "/media/Show", Status: "pending"},
	}); len(got) != 0 {
		t.Fatalf("parent==title: %#v", got)
	}
	if got := parentJobs([]Job{
		{Title: "X", Parent: "", Path: "/media/X", Status: "pending"},
	}); len(got) != 0 {
		t.Fatalf("empty parent: %#v", got)
	}
}

func TestFoldUnmatchedIntoParent(t *testing.T) {
	s01 := Job{
		Title:  "S01",
		Parent: "Show",
		Path:   "/media/Show/S01",
		Status: "unmatched",
		Files:  []JobFile{{Path: "a.mkv"}},
	}
	s02 := Job{
		Title:  "S02",
		Parent: "Show",
		Path:   "/media/Show/S02",
		Status: "error",
		Files:  []JobFile{{Path: "b.mkv"}},
	}
	parent := Job{
		Title:  "Show",
		Parent: "Show",
		Path:   filepath.Clean("/media/Show"),
		Status: "matched",
		Kind:   "show",
		Match:  &Candidate{Title: "Show", ID: "1"},
	}
	got := foldUnmatched([]Job{parent, s01, s02})
	if len(got) != 1 || got[0].Title != "Show" {
		t.Fatalf("want only parent, got %#v", got)
	}
	if !reflect.DeepEqual(got[0].Files, []JobFile{{Path: "a.mkv"}, {Path: "b.mkv"}}) {
		t.Fatalf("files %#v", got[0].Files)
	}
}

func TestFoldOnlyUnmatchedChild(t *testing.T) {
	ok := Job{
		Title:  "S01",
		Parent: "Show",
		Path:   "/media/Show/S01",
		Status: "matched",
		Match:  &Candidate{ID: "1"},
		Files:  []JobFile{{Path: "ok.mkv"}},
	}
	fail := Job{
		Title:  "S02",
		Parent: "Show",
		Path:   "/media/Show/S02",
		Status: "unmatched",
		Files:  []JobFile{{Path: "fail.mkv"}},
	}
	parent := Job{
		Title:  "Show",
		Parent: "Show",
		Path:   filepath.Clean("/media/Show"),
		Status: "matched",
		Match:  &Candidate{ID: "2"},
	}
	got := foldUnmatched([]Job{parent, ok, fail})
	if len(got) != 2 {
		t.Fatalf("parent + matched child: %#v", got)
	}
	if got[0].Title != "Show" || !reflect.DeepEqual(got[0].Files, []JobFile{{Path: "fail.mkv"}}) {
		t.Fatalf("folded files %#v", got[0])
	}
	if got[1].Title != "S01" {
		t.Fatalf("kept child %#v", got[1])
	}
}

func TestFoldSkipsFailedParent(t *testing.T) {
	fail := Job{
		Title:  "S01",
		Parent: "Show",
		Path:   "/media/Show/S01",
		Status: "unmatched",
		Files:  []JobFile{{Path: "a.mkv"}},
	}
	parent := Job{
		Title:  "Show",
		Parent: "Show",
		Path:   filepath.Clean("/media/Show"),
		Status: "unmatched",
	}
	got := foldUnmatched([]Job{parent, fail})
	if len(got) != 2 {
		t.Fatalf("keep children when parent failed: %#v", got)
	}
}

func TestFoldKeepsParentWhenAllMatched(t *testing.T) {
	s01 := Job{
		Title:  "S01",
		Parent: "Show",
		Path:   "/media/Show/S01",
		Status: "matched",
		Match:  &Candidate{ID: "1"},
	}
	s02 := Job{
		Title:  "S02",
		Parent: "Show",
		Path:   "/media/Show/S02",
		Status: "matched",
		Match:  &Candidate{ID: "2"},
	}
	parent := Job{
		Title:  "Show",
		Parent: "Show",
		Path:   filepath.Clean("/media/Show"),
		Status: "matched",
		Match:  &Candidate{ID: "3"},
	}
	got := foldUnmatched([]Job{parent, s01, s02})
	if len(got) != 3 {
		t.Fatalf("keep parent when all children matched: %#v", got)
	}
	if got[0].Path != filepath.Clean("/media/Show") {
		t.Fatalf("parent path %#v", got[0])
	}
}

func TestFoldManualParent(t *testing.T) {
	fail := Job{
		Title:  "S01",
		Parent: "Show",
		Path:   "/media/Show/S01",
		Status: "unmatched",
		Files:  []JobFile{{Path: "a.mkv"}},
	}
	parent := Job{
		Title:  "Show",
		Parent: "Show",
		Path:   filepath.Clean("/media/Show"),
		Status: "manual",
	}
	got := foldUnmatched([]Job{parent, fail})
	if len(got) != 1 || got[0].Status != "manual" || len(got[0].Files) != 1 {
		t.Fatalf("manual parent folds unmatched: %#v", got)
	}
}
