package match

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alyshmahell/matchmedia/lib/config"
)

var genericParents = map[string]struct{}{
	"movies": {}, "movie": {}, "films": {}, "film": {}, "tv": {}, "anime": {}, "media": {},
}

var posterExt = []string{".jpg", ".png", ".jpeg"}

func ApplyChanges(cfg config.Config, jobs []Job, requireEpisodeNFO bool) []Job {
	out := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		if len(job.Files) == 0 {
			continue
		}
		if SidecarComplete(cfg, job, requireEpisodeNFO) {
			job = JobFromSidecar(cfg, job)
		}
		out = append(out, job)
	}
	return out
}

func SidecarComplete(cfg config.Config, job Job, requireEpisodeNFO bool) bool {
	if len(job.Files) == 0 {
		return false
	}
	if showJob(job) {
		dir := jobDir(job)
		if !fileExists(filepath.Join(dir, "tvshow.nfo")) || findShowPoster(dir) == "" {
			return false
		}
		if requireEpisodeNFO {
			for _, f := range job.Files {
				if !episodeNFOOK(f.Path) {
					return false
				}
			}
		}
		return true
	}
	for _, f := range job.Files {
		if !movieSidecarOK(cfg, f.Path) {
			return false
		}
	}
	return true
}

func JobFromSidecar(cfg config.Config, job Job) Job {
	path, poster := sidecarMetaPaths(cfg, job)
	title, year, plot, provider, id := job.Title, job.Year, "", "", ""
	if path != "" {
		if t, y, p, pr, i, err := readSidecarNFO(path); err == nil {
			if t != "" {
				title = t
			}
			if y != "" {
				year = y
			}
			plot, provider, id = p, pr, i
		}
	}
	if title != "" {
		job.Title = title
	}
	if year != "" {
		job.Year = year
	}
	c := Candidate{
		Provider: provider,
		ID:       id,
		Title:    title,
		Year:     year,
		Score:    1,
		Jaccard:  1,
		Synopsis: plot,
		Poster:   poster,
	}
	job.Status = "matched"
	job.Ranker = "set"
	job.Match = &c
	job.Candidates = []Candidate{c}
	job.Catalog = []CatalogSeason{}
	return job
}

func showJob(job Job) bool {
	if fileExists(filepath.Join(jobDir(job), "tvshow.nfo")) {
		return true
	}
	for _, f := range job.Files {
		if f.Season != "" || f.Episode != "" {
			return true
		}
		if _, _, ok := parseFilenameSE(filepath.Base(f.Path)); ok {
			return true
		}
	}
	return false
}

func jobDir(job Job) string {
	if st, err := os.Stat(job.Path); err == nil && st.IsDir() {
		return job.Path
	}
	if job.Path != "" {
		return filepath.Dir(job.Path)
	}
	if len(job.Files) > 0 {
		return filepath.Dir(job.Files[0].Path)
	}
	return ""
}

func episodeNFOOK(video string) bool {
	dir := filepath.Dir(video)
	base := trimExt(filepath.Base(video))
	return fileExists(filepath.Join(dir, "episode.nfo")) || fileExists(filepath.Join(dir, base+".nfo"))
}

func findShowPoster(dir string) string {
	if p := findPoster(dir, "", ""); p != "" {
		return p
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		child := filepath.Join(dir, e.Name())
		season := ""
		if n, ok := parseFolderSeason(e.Name()); ok {
			season = n
		}
		if p := findPoster(child, "", season); p != "" {
			return p
		}
	}
	return ""
}

func movieSidecarOK(cfg config.Config, video string) bool {
	dir := filepath.Dir(video)
	base := trimExt(filepath.Base(video))
	generic := isGenericParent(filepath.Base(dir))
	multi := countPrimaryVideos(cfg, dir) > 1
	if generic || multi {
		if !fileExists(filepath.Join(dir, base+".nfo")) {
			return false
		}
	} else if !fileExists(filepath.Join(dir, "movie.nfo")) && !fileExists(filepath.Join(dir, base+".nfo")) {
		return false
	}
	return findPoster(dir, base, "") != ""
}

func sidecarMetaPaths(cfg config.Config, job Job) (nfo, poster string) {
	dir := jobDir(job)
	if showJob(job) {
		nfo = filepath.Join(dir, "tvshow.nfo")
		poster = findShowPoster(dir)
		if fileExists(nfo) {
			return nfo, poster
		}
	}
	var video string
	if st, err := os.Stat(job.Path); err == nil && !st.IsDir() {
		video = job.Path
	} else if len(job.Files) > 0 {
		video = job.Files[0].Path
	}
	if video == "" {
		return "", findPoster(dir, "", "")
	}
	vdir := filepath.Dir(video)
	base := trimExt(filepath.Base(video))
	for _, name := range []string{"movie.nfo", base + ".nfo", "tvshow.nfo", "episode.nfo"} {
		p := filepath.Join(vdir, name)
		if fileExists(p) {
			nfo = p
			break
		}
	}
	return nfo, findPoster(vdir, base, "")
}

func isGenericParent(name string) bool {
	_, ok := genericParents[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func countPrimaryVideos(cfg config.Config, dir string) int {
	ext := cfg.GroupVideoExt()
	extras := cfg.GroupExtras()
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() {
			if _, ok := extras[strings.ToLower(e.Name())]; ok {
				continue
			}
			continue
		}
		if _, ok := ext[strings.ToLower(filepath.Ext(e.Name()))]; ok {
			n++
		}
	}
	return n
}

func findPoster(dir, base, season string) string {
	if dir == "" {
		return ""
	}
	for _, name := range posterNames(base, season) {
		p := filepath.Join(dir, name)
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func posterNames(base, season string) []string {
	stems := []string{"poster", "folder"}
	if base != "" {
		stems = append(stems, base+"-poster", base+"-thumb")
	}
	if season != "" {
		stems = append(stems, "season"+season+"-poster")
		if n, err := strconv.Atoi(season); err == nil {
			stems = append(stems, "season"+strconv.Itoa(n)+"-poster")
			if n < 100 {
				stems = append(stems, "season"+pad2(n)+"-poster")
			}
		}
	}
	var out []string
	for _, s := range stems {
		for _, ext := range posterExt {
			out = append(out, s+ext)
		}
	}
	return out
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func trimExt(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func readSidecarNFO(path string) (title, year, plot, provider, id string, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", "", "", "", err
	}
	var n struct {
		XMLName  xml.Name
		Title    string `xml:"title"`
		Year     string `xml:"year"`
		Plot     string `xml:"plot"`
		UniqueID struct {
			Type  string `xml:"type,attr"`
			Value string `xml:",chardata"`
		} `xml:"uniqueid"`
	}
	if err = xml.Unmarshal(b, &n); err != nil {
		return "", "", "", "", "", err
	}
	switch strings.ToLower(n.XMLName.Local) {
	case "movie", "tvshow", "episodedetails":
		return n.Title, n.Year, n.Plot, n.UniqueID.Type, n.UniqueID.Value, nil
	default:
		return "", "", "", "", "", os.ErrInvalid
	}
}
