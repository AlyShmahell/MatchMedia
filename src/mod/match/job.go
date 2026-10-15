package match

import (
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
	"github.com/alyshmahell/matchmedia/src/mod/fetch"
)

type Candidate = rank.Hit
type CatalogSeason = fetch.Season
type CatalogEpisode = fetch.Episode

type JobFile struct {
	Path    string `json:"path"`
	Season  string `json:"season,omitempty"`
	Episode string `json:"episode,omitempty"`
}

type Job struct {
	ID         string          `json:"id"`
	Source     string          `json:"source"`
	Title      string          `json:"title"`
	Year       string          `json:"year,omitempty"`
	Parent     string          `json:"parent,omitempty"`
	Type       string          `json:"type,omitempty"`
	Season     string          `json:"season,omitempty"`
	Episode    string          `json:"episode,omitempty"`
	IMDB       string          `json:"imdb,omitempty"`
	Path       string          `json:"path,omitempty"`
	Kind       string          `json:"kind,omitempty"`
	Role       string          `json:"role,omitempty"`
	Files      []JobFile       `json:"files,omitempty"`
	Status     string          `json:"status"`
	Ranker     string          `json:"ranker,omitempty"`
	Match      *Candidate      `json:"match,omitempty"`
	Candidates []Candidate     `json:"candidates,omitempty"`
	Catalog    []CatalogSeason `json:"catalog"`
	CatalogFor string          `json:"catalog_for,omitempty"`
	Error      string          `json:"error,omitempty"`
}

func (j Job) Query() rank.Query {
	return rank.Query{Title: j.Title, Year: j.Year, Parent: j.Parent, Type: j.Type}
}

func MapFilesOntoCatalog(job Job) Job {
	if len(job.Files) == 0 || len(job.Catalog) == 0 {
		return job
	}
	bySE := map[string][]string{}
	for _, f := range job.Files {
		if strings.TrimSpace(f.Season) == "" || strings.TrimSpace(f.Episode) == "" {
			continue
		}
		key := fetch.CatalogNumKey(f.Season) + ":" + fetch.CatalogNumKey(f.Episode)
		bySE[key] = append(bySE[key], f.Path)
	}
	for i := range job.Catalog {
		sk := fetch.CatalogNumKey(job.Catalog[i].Number)
		for j := range job.Catalog[i].Episodes {
			key := sk + ":" + fetch.CatalogNumKey(job.Catalog[i].Episodes[j].Number)
			paths := bySE[key]
			if len(paths) == 0 {
				continue
			}
			job.Catalog[i].Episodes[j].Path = paths[0]
			if len(paths) > 1 {
				job.Catalog[i].Episodes[j].Paths = paths
			}
		}
	}
	return job
}
