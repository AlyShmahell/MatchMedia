package sidecar

import (
	"encoding/xml"
	"os"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
	"github.com/alyshmahell/matchmedia/src/mod/fetch"
)

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

type uniqueID struct {
	Type    string `xml:"type,attr"`
	Default string `xml:"default,attr,omitempty"`
	Value   string `xml:",chardata"`
}

type tvshowNFO struct {
	XMLName   xml.Name `xml:"tvshow"`
	Title     string   `xml:"title"`
	Type      string   `xml:"type,omitempty"`
	Plot      string   `xml:"plot,omitempty"`
	Year      string   `xml:"year,omitempty"`
	Premiered string   `xml:"premiered,omitempty"`
	Runtime   string   `xml:"runtime,omitempty"`
	Rating    string   `xml:"rating,omitempty"`
	Language  string   `xml:"language,omitempty"`
	Ended     string   `xml:"ended,omitempty"`
	Genre     []string `xml:"genre,omitempty"`
	Country   []string `xml:"country,omitempty"`
	Network   []string `xml:"network,omitempty"`
	UniqueID  uniqueID `xml:"uniqueid"`
}

type movieNFO struct {
	XMLName   xml.Name `xml:"movie"`
	Title     string   `xml:"title"`
	Type      string   `xml:"type,omitempty"`
	Plot      string   `xml:"plot,omitempty"`
	Year      string   `xml:"year,omitempty"`
	Premiered string   `xml:"premiered,omitempty"`
	Runtime   string   `xml:"runtime,omitempty"`
	Rating    string   `xml:"rating,omitempty"`
	Language  string   `xml:"language,omitempty"`
	Ended     string   `xml:"ended,omitempty"`
	Genre     []string `xml:"genre,omitempty"`
	Country   []string `xml:"country,omitempty"`
	Network   []string `xml:"network,omitempty"`
	UniqueID  uniqueID `xml:"uniqueid"`
}

type episodeNFO struct {
	XMLName  xml.Name `xml:"episodedetails"`
	Title    string   `xml:"title"`
	Plot     string   `xml:"plot,omitempty"`
	Year     string   `xml:"year,omitempty"`
	Aired    string   `xml:"aired,omitempty"`
	Season   string   `xml:"season,omitempty"`
	Episode  string   `xml:"episode,omitempty"`
	UniqueID uniqueID `xml:"uniqueid"`
}

func makeID(provider, id string) uniqueID {
	return uniqueID{Type: provider, Default: "true", Value: id}
}

func writeNFO(path string, v any) error {
	b, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte(xmlHeader), b...), 0o644)
}

func splitAttr(attrs map[string]string, key string) []string {
	raw := strings.TrimSpace(attr(attrs, key))
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func attr(attrs map[string]string, key string) string {
	if attrs == nil {
		return ""
	}
	return strings.TrimSpace(attrs[key])
}

func premieredOf(cand rank.Hit) string {
	if p := attr(cand.Attrs, "premiered"); p != "" {
		return p
	}
	return cand.Year
}

func writeShowNFO(path string, cand rank.Hit, uniqueType, jobType string) error {
	if uniqueType == "" {
		uniqueType = cand.Provider
	}
	return writeNFO(path, tvshowNFO{
		Title: cand.Title, Type: jobType, Plot: cand.Synopsis, Year: cand.Year, Premiered: premieredOf(cand),
		Runtime: attr(cand.Attrs, "runtime"), Rating: attr(cand.Attrs, "rating"), Language: attr(cand.Attrs, "language"),
		Ended: attr(cand.Attrs, "ended"), Genre: splitAttr(cand.Attrs, "genres"), Country: splitAttr(cand.Attrs, "country"),
		Network: splitAttr(cand.Attrs, "network"), UniqueID: makeID(uniqueType, cand.ID),
	})
}

func writeMovieNFO(path string, cand rank.Hit, uniqueType, jobType string) error {
	if uniqueType == "" {
		uniqueType = cand.Provider
	}
	return writeNFO(path, movieNFO{
		Title: cand.Title, Type: jobType, Plot: cand.Synopsis, Year: cand.Year, Premiered: premieredOf(cand),
		Runtime: attr(cand.Attrs, "runtime"), Rating: attr(cand.Attrs, "rating"), Language: attr(cand.Attrs, "language"),
		Ended: attr(cand.Attrs, "ended"), Genre: splitAttr(cand.Attrs, "genres"), Country: splitAttr(cand.Attrs, "country"),
		Network: splitAttr(cand.Attrs, "network"), UniqueID: makeID(uniqueType, cand.ID),
	})
}

func writeEpisodeNFO(path, provider, season string, e fetch.Episode) error {
	return writeNFO(path, episodeNFO{
		Title: e.Title, Plot: e.Synopsis, Year: e.Year, Aired: e.Year, Season: season, Episode: e.Number, UniqueID: makeID(provider, e.ID),
	})
}
