package match

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeXML(t *testing.T, path, root, title, year, uidType, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	uid := ""
	if uidType != "" || id != "" {
		uid = fmt.Sprintf("\n  <uniqueid type=%q>%s</uniqueid>", uidType, id)
	}
	body := fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<%s>\n  <title>%s</title>\n  <year>%s</year>%s\n</%s>\n", root, title, year, uid, root)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writePosterAt(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seasonFolder(n string) string {
	i, err := strconv.Atoi(n)
	if err != nil {
		return "Season " + n
	}
	return fmt.Sprintf("Season %02d", i)
}

func writeCompleteMovie(t *testing.T, lib, folder, title, year, uidType, id string) {
	t.Helper()
	dir := filepath.Join(lib, folder)
	video := filepath.Join(dir, folder+".mkv")
	writeTree(t, lib, []string{filepath.ToSlash(strings.TrimPrefix(video, lib+string(os.PathSeparator)))})
	writeXML(t, filepath.Join(dir, "movie.nfo"), "movie", title, year, uidType, id)
	writePosterAt(t, filepath.Join(dir, "poster.jpg"))
}

func writeTitleShow(t *testing.T, lib, show, title, year string, episodeNFO bool) {
	t.Helper()
	dir := filepath.Join(lib, show)
	writeXML(t, filepath.Join(dir, "tvshow.nfo"), "tvshow", title, year, "", "")
	writePosterAt(t, filepath.Join(dir, "poster.jpg"))
	name := show + " S01E01.mkv"
	sdir := filepath.Join(show, seasonFolder("1"))
	if episodeNFO {
		base := strings.TrimSuffix(name, filepath.Ext(name))
		writeXML(t, filepath.Join(lib, sdir, base+".nfo"), "episodedetails", base, year, "", "")
	}
	writeTree(t, lib, []string{filepath.Join(sdir, name)})
}

func writeKodiShow(t *testing.T, lib, show, title, year, uidType, id string, eps [][2]string) {
	t.Helper()
	dir := filepath.Join(lib, show)
	writeXML(t, filepath.Join(dir, "tvshow.nfo"), "tvshow", title, year, uidType, id)
	writePosterAt(t, filepath.Join(dir, "poster.jpg"))
	var files []string
	for _, ep := range eps {
		season, name := ep[0], ep[1]
		sdir := filepath.Join(show, seasonFolder(season))
		files = append(files, filepath.Join(sdir, name))
		base := strings.TrimSuffix(name, filepath.Ext(name))
		writeXML(t, filepath.Join(lib, sdir, base+".nfo"), "episodedetails", base, year, uidType, "")
		writePosterAt(t, filepath.Join(lib, sdir, base+"-thumb.jpg"))
	}
	writeTree(t, lib, files)
}

func writeCompleteShow(t *testing.T, lib, show, title, year, uidType, id string, eps [][2]string) {
	t.Helper()
	dir := filepath.Join(lib, show)
	writeXML(t, filepath.Join(dir, "tvshow.nfo"), "tvshow", title, year, uidType, id)
	writePosterAt(t, filepath.Join(dir, "poster.jpg"))
	seen := map[string]bool{}
	var files []string
	for _, ep := range eps {
		season, name := ep[0], ep[1]
		sdir := filepath.Join(show, seasonFolder(season))
		files = append(files, filepath.Join(sdir, name))
		if !seen[season] {
			writeXML(t, filepath.Join(lib, sdir, "season.nfo"), "season", "Season "+season, year, uidType, id)
			writePosterAt(t, filepath.Join(lib, sdir, "poster.jpg"))
			seen[season] = true
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		writeXML(t, filepath.Join(lib, sdir, base+".nfo"), "episodedetails", base, year, uidType, "")
	}
	writeTree(t, lib, files)
}

func absJobs(lib string, shows []Grouped) []Job {
	var out []Job
	for _, s := range shows {
		job := Job{
			Source: "scan",
			Title:  s.Title,
			Year:   s.Year,
			Parent: s.Parent,
			Status: "pending",
			Path:   filepath.Join(lib, filepath.FromSlash(s.Path)),
		}
		for _, f := range s.Files {
			job.Files = append(job.Files, JobFile{
				Path:    filepath.Join(lib, filepath.FromSlash(f.Path)),
				Season:  f.Season,
				Episode: f.Episode,
			})
		}
		out = append(out, job)
	}
	return out
}

func scanLib(t *testing.T, lib string, changes bool) []Job {
	t.Helper()
	return scanLibNFO(t, lib, changes, true)
}

func scanLibNFO(t *testing.T, lib string, changes, requireEpisodeNFO bool) []Job {
	t.Helper()
	cfg := testCfg(t)
	ents, err := os.ReadDir(lib)
	if err != nil {
		t.Fatal(err)
	}
	var all []Job
	for _, e := range ents {
		jobs := absJobs(lib, Group(cfg, lib, filepath.Join(lib, e.Name())))
		if changes {
			jobs = ApplyChanges(cfg, jobs, requireEpisodeNFO)
		}
		all = append(all, jobs...)
	}
	return all
}

func jobByTitle(t *testing.T, jobs []Job, title string) Job {
	t.Helper()
	var titles []string
	for _, j := range jobs {
		titles = append(titles, j.Title)
		if j.Title == title {
			return j
		}
	}
	t.Fatalf("missing title %q in %v", title, titles)
	return Job{}
}

func requirePending(t *testing.T, job Job) {
	t.Helper()
	if job.Status != "pending" {
		t.Fatalf("status=%s title=%s", job.Status, job.Title)
	}
	if job.Match != nil || len(job.Candidates) != 0 {
		t.Fatalf("unexpected match for %s", job.Title)
	}
}

func requireMatchedSidecar(t *testing.T, job Job, provider, id string) {
	t.Helper()
	if job.Status != "matched" {
		t.Fatalf("status=%s title=%s", job.Status, job.Title)
	}
	if job.Ranker != "set" {
		t.Fatalf("ranker=%s", job.Ranker)
	}
	if job.Catalog == nil {
		t.Fatal("catalog nil")
	}
	if len(job.Catalog) != 0 {
		t.Fatalf("catalog=%+v", job.Catalog)
	}
	if NeedsCatalog(testCfg(t), job) {
		t.Fatal("needs catalog")
	}
	if len(job.Candidates) != 1 {
		t.Fatalf("candidates=%+v", job.Candidates)
	}
	c := job.Candidates[0]
	if c.Score != 1 || c.Jaccard != 1 {
		t.Fatalf("score=%v jaccard=%v", c.Score, c.Jaccard)
	}
	if c.Provider != provider || c.ID != id {
		t.Fatalf("provider=%s id=%s", c.Provider, c.ID)
	}
	if c.Poster == "" {
		t.Fatal("missing poster")
	}
	if job.Match == nil {
		t.Fatal("nil match")
	}
	if job.Match.Score != 1 || job.Match.Provider != provider || job.Match.ID != id {
		t.Fatalf("match=%+v", job.Match)
	}
	if job.Source != "scan" || job.Type != "" {
		t.Fatalf("source=%s type=%s", job.Source, job.Type)
	}
}

func TestScanRescanStaysPending(t *testing.T) {
	lib := t.TempDir()
	writeCompleteMovie(t, lib, "Spirited Away (2001)", "Spirited Away", "2001", "tmdb-movie", "129")
	jobs := scanLib(t, lib, false)
	if len(jobs) != 1 {
		t.Fatalf("jobs=%d", len(jobs))
	}
	requirePending(t, jobs[0])
}

func TestChangesAllSidecarsMatched(t *testing.T) {
	lib := t.TempDir()
	writeCompleteMovie(t, lib, "Spirited Away (2001)", "Spirited Away", "2001", "tmdb-movie", "129")
	writeCompleteShow(t, lib, "Girls (2012)", "Girls", "2012", "tmdb-tv", "139", [][2]string{{"1", "Girls S01E01.mkv"}})
	jobs := scanLib(t, lib, true)
	if len(jobs) != 2 {
		t.Fatalf("jobs=%d titles=%v", len(jobs), titlesOfJobs(jobs))
	}
	requireMatchedSidecar(t, jobByTitle(t, jobs, "Spirited Away"), "tmdb-movie", "129")
	requireMatchedSidecar(t, jobByTitle(t, jobs, "Girls"), "tmdb-tv", "139")
}

func TestChangesNewMovieAmongCompletePending(t *testing.T) {
	lib := t.TempDir()
	writeCompleteMovie(t, lib, "Spirited Away (2001)", "Spirited Away", "2001", "tmdb-movie", "129")
	writeTree(t, lib, []string{"New Film (2021)/New Film (2021).mkv"})
	jobs := scanLib(t, lib, true)
	requireMatchedSidecar(t, jobByTitle(t, jobs, "Spirited Away"), "tmdb-movie", "129")
	requirePending(t, jobByTitle(t, jobs, "New Film"))
}

func TestChangesNewSeasonMakesShowPending(t *testing.T) {
	lib := t.TempDir()
	writeCompleteShow(t, lib, "Show", "Show", "2020", "tmdb-tv", "1", [][2]string{{"1", "Show S01E01.mkv"}})
	writeTree(t, lib, []string{"Show/Season 03/Show S03E01.mkv"})
	jobs := scanLib(t, lib, true)
	if len(jobs) != 1 {
		t.Fatalf("jobs=%v", titlesOfJobs(jobs))
	}
	job := jobs[0]
	requirePending(t, job)
	if len(job.Files) != 2 {
		t.Fatalf("files=%+v", job.Files)
	}
	e1 := fileBySuffix(t, job.Files, "Show S01E01.mkv")
	if e1.Season != "1" || e1.Episode != "1" {
		t.Fatalf("s01e01=%+v", e1)
	}
	e3 := fileBySuffix(t, job.Files, "Show S03E01.mkv")
	if e3.Season != "3" || e3.Episode != "1" {
		t.Fatalf("s03e01=%+v", e3)
	}
}

func TestChangesSpecialsWithoutNFOPending(t *testing.T) {
	lib := t.TempDir()
	writeCompleteShow(t, lib, "Show", "Show", "2020", "tmdb-tv", "1", [][2]string{{"1", "Show S01E01.mkv"}})
	writeTree(t, lib, []string{"Show/Specials/ova.mkv"})
	jobs := scanLib(t, lib, true)
	if len(jobs) != 1 {
		t.Fatalf("jobs=%v", titlesOfJobs(jobs))
	}
	requirePending(t, jobs[0])
	ova := fileBySuffix(t, jobs[0].Files, "/ova.mkv")
	if ova.Season != "0" {
		t.Fatalf("ova=%+v", ova)
	}
}

func TestChangesNewShowWithoutTvshowPending(t *testing.T) {
	lib := t.TempDir()
	writeTree(t, lib, []string{"Show/Season 01/Show S01E01.mkv"})
	jobs := scanLib(t, lib, true)
	if len(jobs) != 1 {
		t.Fatalf("jobs=%v", titlesOfJobs(jobs))
	}
	requirePending(t, jobs[0])
}

func TestChangesMovieUniqueID(t *testing.T) {
	lib := t.TempDir()
	writeCompleteMovie(t, lib, "Spirited Away (2001)", "Spirited Away", "2001", "tmdb-movie", "129")
	jobs := scanLib(t, lib, true)
	if len(jobs) != 1 {
		t.Fatalf("jobs=%v", titlesOfJobs(jobs))
	}
	requireMatchedSidecar(t, jobs[0], "tmdb-movie", "129")
}

func TestChangesMovieMissingUniqueIDStillMatched(t *testing.T) {
	lib := t.TempDir()
	writeCompleteMovie(t, lib, "Spirited Away (2001)", "Spirited Away", "2001", "", "")
	jobs := scanLib(t, lib, true)
	if len(jobs) != 1 {
		t.Fatalf("jobs=%v", titlesOfJobs(jobs))
	}
	requireMatchedSidecar(t, jobs[0], "", "")
}

func TestChangesNFOWithoutPosterPending(t *testing.T) {
	lib := t.TempDir()
	writeTree(t, lib, []string{"Film (2020)/Film (2020).mkv"})
	writeXML(t, filepath.Join(lib, "Film (2020)", "movie.nfo"), "movie", "Film", "2020", "tmdb-movie", "1")
	jobs := scanLib(t, lib, true)
	requirePending(t, jobByTitle(t, jobs, "Film"))
}

func TestChangesPosterWithoutNFOPending(t *testing.T) {
	lib := t.TempDir()
	writeTree(t, lib, []string{"Film (2020)/Film (2020).mkv"})
	writePosterAt(t, filepath.Join(lib, "Film (2020)", "poster.jpg"))
	jobs := scanLib(t, lib, true)
	requirePending(t, jobByTitle(t, jobs, "Film"))
}

func TestChangesDeletedMovieOmitted(t *testing.T) {
	lib := t.TempDir()
	writeCompleteMovie(t, lib, "Keep (2020)", "Keep", "2020", "tmdb-movie", "1")
	writeCompleteMovie(t, lib, "Gone (2019)", "Gone", "2019", "tmdb-movie", "2")
	if err := os.Remove(filepath.Join(lib, "Gone (2019)", "Gone (2019).mkv")); err != nil {
		t.Fatal(err)
	}
	jobs := scanLib(t, lib, true)
	if len(jobs) != 1 {
		t.Fatalf("jobs=%v", titlesOfJobs(jobs))
	}
	requireMatchedSidecar(t, jobByTitle(t, jobs, "Keep"), "tmdb-movie", "1")
}

func TestChangesRenameWithoutSidecarPendingAtNewPath(t *testing.T) {
	lib := t.TempDir()
	writeTree(t, lib, []string{"Old Film (2018)/Old Film (2018).mkv"})
	old := scanLib(t, lib, true)
	if len(old) != 1 || !strings.Contains(old[0].Path, "Old Film") {
		t.Fatalf("old=%+v", old)
	}
	requirePending(t, old[0])
	if err := os.Rename(filepath.Join(lib, "Old Film (2018)"), filepath.Join(lib, "New Film (2018)")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(lib, "New Film (2018)", "Old Film (2018).mkv"), filepath.Join(lib, "New Film (2018)", "New Film (2018).mkv")); err != nil {
		t.Fatal(err)
	}
	jobs := scanLib(t, lib, true)
	if len(jobs) != 1 {
		t.Fatalf("jobs=%v", titlesOfJobs(jobs))
	}
	requirePending(t, jobs[0])
	if strings.Contains(jobs[0].Path, "Old Film") {
		t.Fatalf("old path still present: %s", jobs[0].Path)
	}
	if !strings.Contains(jobs[0].Path, "New Film") {
		t.Fatalf("path=%s", jobs[0].Path)
	}
}

func TestChangesKodiThumbsWithoutSeasonNFOMatched(t *testing.T) {
	lib := t.TempDir()
	writeKodiShow(t, lib, "7seeds", "7seeds", "2019", "", "", [][2]string{
		{"1", "7seeds S01E01.mkv"},
		{"1", "7seeds S01E02.mkv"},
	})
	if _, err := os.Stat(filepath.Join(lib, "7seeds", "Season 01", "season.nfo")); !os.IsNotExist(err) {
		t.Fatalf("season.nfo should be absent: %v", err)
	}
	jobs := scanLib(t, lib, true)
	if len(jobs) != 1 {
		t.Fatalf("jobs=%v", titlesOfJobs(jobs))
	}
	requireMatchedSidecar(t, jobs[0], "", "")
}

func TestChangesKodiThumbMissingMatched(t *testing.T) {
	lib := t.TempDir()
	writeKodiShow(t, lib, "7seeds", "7seeds", "2019", "", "", [][2]string{
		{"1", "7seeds S01E01.mkv"},
		{"1", "7seeds S01E02.mkv"},
	})
	if err := os.Remove(filepath.Join(lib, "7seeds", "Season 01", "7seeds S01E02-thumb.jpg")); err != nil {
		t.Fatal(err)
	}
	jobs := scanLib(t, lib, true)
	requireMatchedSidecar(t, jobByTitle(t, jobs, "7seeds"), "", "")
}

func TestChangesNoEpisodeThumbsMatched(t *testing.T) {
	lib := t.TempDir()
	writeTitleShow(t, lib, "Darling", "Darling", "2018", true)
	jobs := scanLib(t, lib, true)
	requireMatchedSidecar(t, jobByTitle(t, jobs, "Darling"), "", "")
}

func TestChangesSeasonPosterFallbackMatched(t *testing.T) {
	lib := t.TempDir()
	writeXML(t, filepath.Join(lib, "Show", "tvshow.nfo"), "tvshow", "Show", "2020", "", "")
	writePosterAt(t, filepath.Join(lib, "Show", "Season 01", "poster.jpg"))
	writeXML(t, filepath.Join(lib, "Show", "Season 01", "Show S01E01.nfo"), "episodedetails", "E1", "2020", "", "")
	writeTree(t, lib, []string{"Show/Season 01/Show S01E01.mkv"})
	jobs := scanLib(t, lib, true)
	requireMatchedSidecar(t, jobByTitle(t, jobs, "Show"), "", "")
}

func TestChangesKodiEpisodeNFOMissingPending(t *testing.T) {
	lib := t.TempDir()
	writeKodiShow(t, lib, "7seeds", "7seeds", "2019", "", "", [][2]string{
		{"1", "7seeds S01E01.mkv"},
		{"1", "7seeds S01E02.mkv"},
	})
	if err := os.Remove(filepath.Join(lib, "7seeds", "Season 01", "7seeds S01E02.nfo")); err != nil {
		t.Fatal(err)
	}
	jobs := scanLib(t, lib, true)
	requirePending(t, jobByTitle(t, jobs, "7seeds"))
}

func TestChangesEpisodeNFOOptionalMatched(t *testing.T) {
	lib := t.TempDir()
	writeKodiShow(t, lib, "7seeds", "7seeds", "2019", "", "", [][2]string{
		{"1", "7seeds S01E01.mkv"},
		{"1", "7seeds S01E02.mkv"},
	})
	if err := os.Remove(filepath.Join(lib, "7seeds", "Season 01", "7seeds S01E02.nfo")); err != nil {
		t.Fatal(err)
	}
	jobs := scanLibNFO(t, lib, true, false)
	requireMatchedSidecar(t, jobByTitle(t, jobs, "7seeds"), "", "")
}

func TestChangesSpecialsWithoutNFOOptionalMatched(t *testing.T) {
	lib := t.TempDir()
	writeCompleteShow(t, lib, "Show", "Show", "2020", "tmdb-tv", "1", [][2]string{{"1", "Show S01E01.mkv"}})
	writeTree(t, lib, []string{"Show/Specials/ova.mkv"})
	jobs := scanLibNFO(t, lib, true, false)
	if len(jobs) != 1 {
		t.Fatalf("jobs=%v", titlesOfJobs(jobs))
	}
	requireMatchedSidecar(t, jobs[0], "tmdb-tv", "1")
}

func TestChangesShowDropsGoneEpisode(t *testing.T) {
	lib := t.TempDir()
	writeCompleteShow(t, lib, "Show", "Show", "2020", "tmdb-tv", "1", [][2]string{
		{"1", "Show S01E01.mkv"},
		{"1", "Show S01E02.mkv"},
	})
	if err := os.Remove(filepath.Join(lib, "Show", "Season 01", "Show S01E02.mkv")); err != nil {
		t.Fatal(err)
	}
	jobs := scanLib(t, lib, true)
	if len(jobs) != 1 {
		t.Fatalf("jobs=%v", titlesOfJobs(jobs))
	}
	job := jobs[0]
	if len(job.Files) != 1 {
		t.Fatalf("files=%+v", job.Files)
	}
	if !strings.HasSuffix(job.Files[0].Path, "Show S01E01.mkv") {
		t.Fatalf("kept=%+v", job.Files[0])
	}
	for _, f := range job.Files {
		if strings.Contains(f.Path, "S01E02") {
			t.Fatalf("gone file still listed: %+v", job.Files)
		}
	}
	requireMatchedSidecar(t, job, "tmdb-tv", "1")
}

func titlesOfJobs(jobs []Job) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.Title
	}
	return out
}
