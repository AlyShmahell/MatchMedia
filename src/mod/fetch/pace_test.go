package fetch

import (
	"context"
	"testing"
	"time"

	"github.com/alyshmahell/matchmedia/src/mod/config"
)

func TestSectionPace(t *testing.T) {
	ctx := context.Background()
	spec := config.Provider{NFO: "tvshow", MinIntervalMS: 80}
	start := time.Now()
	if err := sectionPace(ctx, "pace-default", spec, "search", 0); err != nil {
		t.Fatal(err)
	}
	if err := sectionPace(ctx, "pace-default", spec, "titles", 0); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 40*time.Millisecond {
		t.Fatal("titles used the search clock")
	}
	if err := sectionPace(ctx, "pace-default", spec, "search", 0); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 60*time.Millisecond {
		t.Fatal("provider interval did not pace search")
	}

	spec = config.Provider{NFO: "tvshow", MinIntervalMS: 400}
	start = time.Now()
	if err := sectionPace(ctx, "pace-override", spec, "search", 70); err != nil {
		t.Fatal(err)
	}
	if err := sectionPace(ctx, "pace-override", spec, "search", 70); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Fatalf("section interval %s", elapsed)
	}

	spec = config.Provider{NFO: "tvshow"}
	start = time.Now()
	if err := sectionPace(ctx, "pace-tvmaze", spec, "search", 70); err != nil {
		t.Fatal(err)
	}
	if err := sectionPace(ctx, "pace-tvmaze", spec, "titles", 0); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 30*time.Millisecond {
		t.Fatal("titles paced")
	}
	if err := sectionPace(ctx, "pace-tvmaze", spec, "search", 70); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("search not paced")
	}
}
