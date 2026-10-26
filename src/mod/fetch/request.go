package fetch

import (
	"net/url"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
)

func savedRequest(section, provider, nfo, rawURL, season string, query map[string]string, vars map[string]string) rank.Request {
	return rank.Request{
		Section:  section,
		Provider: provider,
		NFO:      nfo,
		URL:      expand(rawURL, vars),
		Query:    publicQuery(query, vars),
		Season:   season,
	}
}

func publicQuery(query map[string]string, vars map[string]string) string {
	vals := url.Values{}
	for k, v := range query {
		if secretQuery(k) {
			continue
		}
		exp := expand(v, vars)
		if exp == "" {
			continue
		}
		vals.Set(k, exp)
	}
	return vals.Encode()
}

func secretQuery(k string) bool {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "api_key", "apikey":
		return true
	default:
		return false
	}
}

func parseQuery(q string) map[string]string {
	vals, err := url.ParseQuery(q)
	if err != nil {
		return nil
	}
	out := map[string]string{}
	for k, vs := range vals {
		if secretQuery(k) || len(vs) == 0 || vs[0] == "" {
			continue
		}
		out[k] = vs[0]
	}
	return out
}

func sectionQuery(spec config.Provider, section string) map[string]string {
	switch section {
	case "detail":
		if spec.Detail != nil {
			return spec.Detail.Query
		}
	case "titles":
		if spec.Titles != nil {
			return spec.Titles.Query
		}
	case "catalog/seasons":
		if spec.Catalog != nil && spec.Catalog.Seasons != nil {
			return spec.Catalog.Seasons.Query
		}
	case "catalog/episodes":
		if spec.Catalog != nil && spec.Catalog.Episodes != nil {
			return spec.Catalog.Episodes.Query
		}
	}
	return nil
}

func sectionURL(spec config.Provider, section string) string {
	switch section {
	case "detail":
		if spec.Detail != nil {
			return spec.Detail.URL
		}
	case "titles":
		if spec.Titles != nil {
			return spec.Titles.URL
		}
	case "catalog/seasons":
		if spec.Catalog != nil && spec.Catalog.Seasons != nil {
			return spec.Catalog.Seasons.URL
		}
	case "catalog/episodes":
		if spec.Catalog != nil && spec.Catalog.Episodes != nil {
			return spec.Catalog.Episodes.URL
		}
	}
	return ""
}

func replayQuery(spec config.Provider, section, saved string) map[string]string {
	out := parseQuery(saved)
	if out == nil {
		out = map[string]string{}
	}
	vars := varsFor(spec, rank.Query{}, "")
	for k, v := range sectionQuery(spec, section) {
		if secretQuery(k) {
			out[k] = expand(v, vars)
		}
	}
	return out
}
