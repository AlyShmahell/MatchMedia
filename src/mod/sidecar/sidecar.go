package sidecar

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
	"github.com/alyshmahell/matchmedia/src/mod/match"
)

var posterExt = []string{".jpg", ".png", ".jpeg"}

func TitleNFO(job match.Job) bool {
	nfo, _, ok := titleSidecar(job)
	return ok && fileExists(nfo)
}

func Complete(job match.Job, requireEpisodeNFO bool) bool {
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

func FromSidecar(job match.Job) match.Job {
	job = fillSidecar(job)
	nfo, _, ok := titleSidecar(job)
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
	c := rank.Hit{
		Provider: meta.provider,
		ID:       meta.id,
		Title:    title,
		Year:     year,
		Score:    1,
		Jaccard:  1,
		Synopsis: meta.plot,
		Attrs:    meta.attrs(),
	}
	job.Status = "matched"
	job.Ranker = "set"
	job.Match = &c
	job.Candidates = []rank.Hit{c}
	job.Catalog = []match.CatalogSeason{}
	return job
}

func Fill(job match.Job) match.Job {
	return fillSidecar(job)
}

func fillSidecar(job match.Job) match.Job {
	job = stampSidecarPaths(job)
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

// stampSidecarPaths records the files beside the media. Poster on the hit stays a provider URL.
func stampSidecarPaths(job match.Job) match.Job {
	nfo, poster, ok := titleSidecar(job)
	if ok {
		job.NFOPath = nfo
	}
	if poster != "" {
		job.PosterPath = poster
	}
	for i, f := range job.Files {
		if p := episodeNFOPath(f.Path); p != "" {
			job.Files[i].NFOPath = p
		}
		if f.Path != "" {
			job.Files[i].PosterPath = thumbPath(f.Path)
		}
	}
	return job
}

func titleSidecar(job match.Job) (nfo, poster string, ok bool) {
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
		poster = findPoster(job.Path, "")
		return nfo, poster, fileExists(nfo)
	}
	dir := filepath.Dir(job.Path)
	base := trimExt(filepath.Base(job.Path))
	nfo = filepath.Join(dir, base+".nfo")
	poster = findPoster(dir, base)
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

func applyEpisodeNFO(files []match.JobFile) []match.JobFile {
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

func thumbPath(video string) string {
	dir := filepath.Dir(video)
	base := trimExt(filepath.Base(video))
	for _, ext := range posterExt {
		p := filepath.Join(dir, base+"-thumb"+ext)
		if fileExists(p) {
			return p
		}
	}
	return filepath.Join(dir, base+"-thumb.jpg")
}

func findPoster(dir, base string) string {
	if dir == "" {
		return ""
	}
	stems := []string{"poster", "folder"}
	if base != "" {
		stems = append(stems, base+"-poster", base+"-thumb")
	}
	for _, s := range stems {
		for _, ext := range posterExt {
			p := filepath.Join(dir, s+ext)
			if fileExists(p) {
				return p
			}
		}
	}
	return ""
}

func trimExt(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func canonNum(s string) string {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return strings.TrimSpace(s)
	}
	return strconv.Itoa(n)
}

type nfoMeta struct {
	root, title, year, plot, provider, id, season, episode string
	runtime, rating, language, premiered, ended            string
	genres, countries, networks                            []string
}

func (m nfoMeta) attrs() map[string]string {
	out := map[string]string{}
	put := func(k, v string) {
		v = strings.TrimSpace(v)
		if v != "" {
			out[k] = v
		}
	}
	put("genres", strings.Join(m.genres, ", "))
	put("rating", m.rating)
	put("runtime", m.runtime)
	put("language", m.language)
	put("premiered", m.premiered)
	put("ended", m.ended)
	put("country", strings.Join(m.countries, ", "))
	put("network", strings.Join(m.networks, ", "))
	if len(out) == 0 {
		return nil
	}
	return out
}

func readSidecarNFO(path string) (nfoMeta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nfoMeta{}, err
	}
	var n struct {
		XMLName   xml.Name
		Title     string   `xml:"title"`
		Year      string   `xml:"year"`
		Plot      string   `xml:"plot"`
		Season    string   `xml:"season"`
		Episode   string   `xml:"episode"`
		Runtime   string   `xml:"runtime"`
		Rating    string   `xml:"rating"`
		Language  string   `xml:"language"`
		Premiered string   `xml:"premiered"`
		Ended     string   `xml:"ended"`
		Genre     []string `xml:"genre"`
		Country   []string `xml:"country"`
		Network   []string `xml:"network"`
		UniqueID  struct {
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
			root: n.XMLName.Local, title: n.Title, year: n.Year, plot: n.Plot,
			provider: n.UniqueID.Type, id: n.UniqueID.Value, season: n.Season, episode: n.Episode,
			runtime: n.Runtime, rating: n.Rating, language: n.Language, premiered: n.Premiered, ended: n.Ended,
			genres: n.Genre, countries: n.Country, networks: n.Network,
		}, nil
	default:
		return nfoMeta{}, os.ErrInvalid
	}
}
