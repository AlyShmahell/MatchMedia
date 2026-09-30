package match

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alyshmahell/matchmedia/src/internal/config"
)

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

func ApplyNFO(cfg config.Config, jobs []Job, requireEpisodeNFO bool) []Job {
	out := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		if len(job.Files) == 0 {
			continue
		}
		job = fillSidecar(cfg, job)
		if SidecarComplete(cfg, job, requireEpisodeNFO) {
			job = JobFromSidecar(cfg, job)
		} else {
			job.Status = "unmatched"
			job.Match = nil
			job.Candidates = nil
			job.Ranker = ""
		}
		out = append(out, job)
	}
	return out
}

func SidecarComplete(_ config.Config, job Job, requireEpisodeNFO bool) bool {
	if len(job.Files) == 0 {
		return false
	}
	nfo, poster, ok := titleSidecar(job)
	if !ok || poster == "" {
		return false
	}
	meta, err := readSidecarNFO(nfo)
	if err != nil || !kindAgrees(job.Kind, meta.root) {
		return false
	}
	if job.Kind == "show" && requireEpisodeNFO {
		for _, f := range job.Files {
			if !episodeNFOOK(f.Path) {
				return false
			}
		}
	}
	return true
}

func JobFromSidecar(cfg config.Config, job Job) Job {
	job = fillSidecar(cfg, job)
	nfo, poster, ok := titleSidecar(job)
	title, year := job.Title, job.Year
	var meta nfoMeta
	if ok {
		if m, err := readSidecarNFO(nfo); err == nil {
			meta = m
			if m.title != "" {
				title = m.title
			}
			if m.year != "" {
				year = m.year
			}
		}
	}
	if title != "" {
		job.Title = title
	}
	if year != "" {
		job.Year = year
	}
	c := Candidate{
		Provider: meta.provider,
		ID:       meta.id,
		Title:    title,
		Year:     year,
		Score:    1,
		Jaccard:  1,
		Synopsis: meta.plot,
		Poster:   poster,
	}
	job.Status = "matched"
	job.Ranker = "set"
	job.Match = &c
	job.Candidates = []Candidate{c}
	job.Catalog = []CatalogSeason{}
	return job
}

func fillSidecar(_ config.Config, job Job) Job {
	nfo, _, ok := titleSidecar(job)
	if ok {
		if meta, err := readSidecarNFO(nfo); err == nil && kindAgrees(job.Kind, meta.root) {
			if meta.title != "" {
				job.Title = meta.title
			}
			if meta.year != "" {
				job.Year = meta.year
			}
		}
	}
	if job.Kind == "show" {
		job.Files = applyEpisodeNFO(job.Files)
	}
	return job
}

func titleSidecar(job Job) (nfo, poster string, ok bool) {
	if job.Path == "" {
		return "", "", false
	}
	st, err := os.Stat(job.Path)
	if err != nil {
		return "", "", false
	}
	if st.IsDir() {
		name := "movie.nfo"
		if job.Kind == "show" {
			name = "tvshow.nfo"
		}
		nfo = filepath.Join(job.Path, name)
		poster = findPoster(job.Path, "", "")
		return nfo, poster, fileExists(nfo)
	}
	dir := filepath.Dir(job.Path)
	base := trimExt(filepath.Base(job.Path))
	nfo = filepath.Join(dir, base+".nfo")
	poster = findPoster(dir, base, "")
	return nfo, poster, fileExists(nfo)
}

func kindAgrees(kind, root string) bool {
	switch strings.ToLower(root) {
	case "tvshow":
		return kind == "show"
	case "movie":
		return kind == "movie"
	default:
		return false
	}
}

func applyEpisodeNFO(files []JobFile) []JobFile {
	for i, f := range files {
		path := episodeNFOPath(f.Path)
		if path == "" {
			continue
		}
		meta, err := readSidecarNFO(path)
		if err != nil {
			continue
		}
		if meta.season != "" {
			files[i].Season = canonNum(meta.season)
		}
		if meta.episode != "" {
			files[i].Episode = canonNum(meta.episode)
		}
	}
	return files
}

func episodeNFOPath(video string) string {
	dir := filepath.Dir(video)
	base := trimExt(filepath.Base(video))
	for _, name := range []string{base + ".nfo", "episode.nfo"} {
		p := filepath.Join(dir, name)
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func episodeNFOOK(video string) bool {
	return episodeNFOPath(video) != ""
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

type nfoMeta struct {
	root, title, year, plot, provider, id, season, episode string
}

func readSidecarNFO(path string) (nfoMeta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nfoMeta{}, err
	}
	var n struct {
		XMLName  xml.Name
		Title    string `xml:"title"`
		Year     string `xml:"year"`
		Plot     string `xml:"plot"`
		Season   string `xml:"season"`
		Episode  string `xml:"episode"`
		UniqueID struct {
			Type  string `xml:"type,attr"`
			Value string `xml:",chardata"`
		} `xml:"uniqueid"`
	}
	if err = xml.Unmarshal(b, &n); err != nil {
		return nfoMeta{}, err
	}
	switch strings.ToLower(n.XMLName.Local) {
	case "movie", "tvshow", "episodedetails":
		return nfoMeta{
			root:     n.XMLName.Local,
			title:    n.Title,
			year:     n.Year,
			plot:     n.Plot,
			provider: n.UniqueID.Type,
			id:       n.UniqueID.Value,
			season:   n.Season,
			episode:  n.Episode,
		}, nil
	default:
		return nfoMeta{}, os.ErrInvalid
	}
}
