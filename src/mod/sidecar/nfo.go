package sidecar

import (
	"bytes"
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

type savedURL struct {
	XMLName  xml.Name `xml:"url"`
	Section  string   `xml:"section,attr"`
	Provider string   `xml:"provider,attr"`
	NFO      string   `xml:"nfo,attr"`
	Season   string   `xml:"season,attr,omitempty"`
	Query    string   `xml:"query,attr,omitempty"`
	Value    string   `xml:",chardata"`
}

type savedURLs struct {
	XMLName xml.Name   `xml:"urls"`
	URL     []savedURL `xml:"url"`
}

type tvshowNFO struct {
	XMLName   xml.Name   `xml:"tvshow"`
	Title     string     `xml:"title"`
	Type      string     `xml:"type,omitempty"`
	Plot      string     `xml:"plot,omitempty"`
	Year      string     `xml:"year,omitempty"`
	Premiered string     `xml:"premiered,omitempty"`
	Runtime   string     `xml:"runtime,omitempty"`
	Rating    string     `xml:"rating,omitempty"`
	Language  string     `xml:"language,omitempty"`
	Ended     string     `xml:"ended,omitempty"`
	Genre     []string   `xml:"genre,omitempty"`
	Country   []string   `xml:"country,omitempty"`
	Network   []string   `xml:"network,omitempty"`
	UniqueID  uniqueID   `xml:"uniqueid"`
	URLs      *savedURLs `xml:"urls,omitempty"`
}

type movieNFO struct {
	XMLName   xml.Name   `xml:"movie"`
	Title     string     `xml:"title"`
	Type      string     `xml:"type,omitempty"`
	Plot      string     `xml:"plot,omitempty"`
	Year      string     `xml:"year,omitempty"`
	Premiered string     `xml:"premiered,omitempty"`
	Runtime   string     `xml:"runtime,omitempty"`
	Rating    string     `xml:"rating,omitempty"`
	Language  string     `xml:"language,omitempty"`
	Ended     string     `xml:"ended,omitempty"`
	Genre     []string   `xml:"genre,omitempty"`
	Country   []string   `xml:"country,omitempty"`
	Network   []string   `xml:"network,omitempty"`
	UniqueID  uniqueID   `xml:"uniqueid"`
	URLs      *savedURLs `xml:"urls,omitempty"`
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
		Network: splitAttr(cand.Attrs, "network"), UniqueID: makeID(uniqueType, cand.ID), URLs: urlsOf(cand.Requests),
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
		Network: splitAttr(cand.Attrs, "network"), UniqueID: makeID(uniqueType, cand.ID), URLs: urlsOf(cand.Requests),
	})
}

func urlsOf(reqs []rank.Request) *savedURLs {
	if len(reqs) == 0 {
		return nil
	}
	out := &savedURLs{}
	for _, r := range reqs {
		if strings.TrimSpace(r.URL) == "" || strings.TrimSpace(r.Section) == "" {
			continue
		}
		out.URL = append(out.URL, savedURL{
			Section: r.Section, Provider: r.Provider, NFO: r.NFO, Season: r.Season, Query: r.Query, Value: r.URL,
		})
	}
	if len(out.URL) == 0 {
		return nil
	}
	return out
}

func spliceURLs(path string, reqs []rank.Request) (bool, error) {
	block := urlsOf(reqs)
	if block == nil {
		return false, nil
	}
	frag, err := xml.Marshal(block)
	if err != nil {
		return false, err
	}
	frag = append(append([]byte("  "), frag...), '\n')
	b, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if i := bytes.Index(b, []byte("<urls>")); i >= 0 {
		j := bytes.Index(b[i:], []byte("</urls>"))
		if j < 0 {
			return false, os.ErrInvalid
		}
		j = i + j + len("</urls>")
		b = append(append(append([]byte{}, b[:i]...), frag...), b[j:]...)
	} else {
		end := bytes.LastIndex(b, []byte("</tvshow>"))
		if end < 0 {
			end = bytes.LastIndex(b, []byte("</movie>"))
		}
		if end < 0 {
			return false, os.ErrInvalid
		}
		b = append(append(append([]byte{}, b[:end]...), frag...), b[end:]...)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func writeEpisodeNFO(path, provider, season string, e fetch.Episode) error {
	aired := strings.TrimSpace(e.Aired)
	if aired == "" {
		aired = strings.TrimSpace(e.Year)
	}
	return writeNFO(path, episodeNFO{
		Title: e.Title, Plot: e.Synopsis, Year: e.Year, Aired: aired, Season: season, Episode: e.Number, UniqueID: makeID(provider, e.ID),
	})
}
