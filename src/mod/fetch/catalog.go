package fetch

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
)

type Episode struct {
	ID       string   `json:"id,omitempty"`
	Number   string   `json:"number,omitempty"`
	Title    string   `json:"title"`
	Synopsis string   `json:"synopsis,omitempty"`
	Poster   string   `json:"poster,omitempty"`
	URL      string   `json:"url,omitempty"`
	Year     string   `json:"year,omitempty"`
	Runtime  string   `json:"runtime,omitempty"`
	Aired    string   `json:"aired,omitempty"`
	Path     string   `json:"path,omitempty"`
	Paths    []string `json:"paths,omitempty"`
}

type Season struct {
	ID       string    `json:"id,omitempty"`
	Number   string    `json:"number,omitempty"`
	Title    string    `json:"title"`
	Synopsis string    `json:"synopsis,omitempty"`
	Poster   string    `json:"poster,omitempty"`
	URL      string    `json:"url,omitempty"`
	Year     string    `json:"year,omitempty"`
	Episodes []Episode `json:"episodes,omitempty"`
}

func (c *Client) Catalog(ctx context.Context, q rank.Query, cand rank.Hit) ([]Season, string) {
	spec, ok := c.cfg.Endpoint(cand.Provider, cand.NFO)
	if !ok || spec.Catalog == nil {
		return nil, ""
	}
	return c.cache.doCatalog(idKey(cand.Provider, cand.ID), func() ([]Season, string) {
		forKey := cand.Key()
		seasons, err := c.loadCatalog(ctx, cand.Provider, spec, q, cand.ID)
		if err != nil || seasons == nil {
			return []Season{}, forKey
		}
		return seasons, forKey
	})
}

func (c *Client) HasCatalog(cand rank.Hit) bool {
	spec, ok := c.cfg.Endpoint(cand.Provider, cand.NFO)
	return ok && spec.Catalog != nil
}

func (c *Client) loadCatalog(ctx context.Context, name string, spec config.Provider, q rank.Query, showID string) ([]Season, error) {
	cat := spec.Catalog
	limit := c.cfg.Match.SynopsisLimit
	seasons := make([]Season, 0)
	if cat.Seasons != nil && cat.Seasons.URL != "" {
		items, err := c.fetchCatalogList(ctx, name, spec, q, cat.Seasons, "catalog/seasons", showID, "", "")
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			s, ok := seasonFrom(spec, cat.Seasons, item, limit)
			if ok {
				seasons = append(seasons, s)
			}
		}
		sortCatalogSeasons(seasons)
	}
	if cat.Episodes == nil || cat.Episodes.URL == "" {
		return seasons, nil
	}
	if perSeasonURL(cat.Episodes.URL) {
		for i := range seasons {
			if strings.Contains(cat.Episodes.URL, "{season}") && seasons[i].Number == "" {
				continue
			}
			if strings.Contains(cat.Episodes.URL, "{season_id}") && seasons[i].ID == "" {
				continue
			}
			items, err := c.fetchCatalogList(ctx, name, spec, q, cat.Episodes, "catalog/episodes", showID, seasons[i].Number, seasons[i].ID)
			if err != nil {
				if ctx.Err() != nil {
					return nil, err
				}
				continue
			}
			seasons[i].Episodes = episodesFrom(spec, cat.Episodes, items, limit)
		}
		return seasons, nil
	}
	items, err := c.fetchCatalogList(ctx, name, spec, q, cat.Episodes, "catalog/episodes", showID, "", "")
	if err != nil {
		return seasons, nil
	}
	grouped := map[string][]Episode{}
	for _, item := range items {
		ep, ok := episodeFrom(spec, cat.Episodes, item, limit)
		if !ok {
			continue
		}
		key := episodeSeasonKey(cat.Episodes, item)
		grouped[key] = append(grouped[key], ep)
	}
	return attachGroupedEpisodes(seasons, grouped), nil
}

func (c *Client) fetchCatalogList(ctx context.Context, name string, spec config.Provider, q rank.Query, list *config.CatalogList, section, showID, season, seasonID string) ([]any, error) {
	if err := sectionPace(ctx, name, spec, section, list.MinIntervalMS); err != nil {
		return nil, err
	}
	v := varsFor(spec, q, showID)
	v["season"] = season
	v["season_id"] = seasonID
	key := endpointKey(name, spec)
	raw, err := providerGET(ctx, c.http.forSpec(spec), list.URL, list.Query, v, key, nil)
	if err != nil {
		return nil, err
	}
	items, err := extractItems(raw, list.Items)
	if err != nil {
		c.trip(key)
		return nil, err
	}
	return items, nil
}

func perSeasonURL(u string) bool {
	return strings.Contains(u, "{season}") || strings.Contains(u, "{season_id}")
}

func seasonFrom(spec config.Provider, list *config.CatalogList, item any, limit int) (Season, bool) {
	id, number, title, synopsis, poster, href, year, _ := catalogFields(spec, list, item, "Season ", limit)
	if title == "" {
		return Season{}, false
	}
	return Season{ID: id, Number: number, Title: title, Synopsis: synopsis, Poster: poster, URL: href, Year: year}, true
}

func episodeFrom(spec config.Provider, list *config.CatalogList, item any, limit int) (Episode, bool) {
	id, number, title, synopsis, poster, href, year, _ := catalogFields(spec, list, item, "Episode ", limit)
	if title == "" {
		return Episode{}, false
	}
	ep := Episode{ID: id, Number: number, Title: title, Synopsis: synopsis, Poster: poster, URL: href, Year: year}
	if list != nil && list.Fields != nil {
		if p := list.Fields["runtime"]; p != "" {
			ep.Runtime = runtimeAttr(dig(item, p))
		}
		if p := list.Fields["airdate"]; p != "" {
			ep.Aired = strings.TrimSpace(asString(dig(item, p)))
		}
	}
	return ep, true
}

func runtimeAttr(v any) string {
	s := strings.TrimSpace(attrString(v))
	if s == "" || s == "0" {
		return ""
	}
	return s
}

func episodesFrom(spec config.Provider, list *config.CatalogList, items []any, limit int) []Episode {
	out := make([]Episode, 0, len(items))
	for _, item := range items {
		ep, ok := episodeFrom(spec, list, item, limit)
		if ok {
			out = append(out, ep)
		}
	}
	sortCatalogEpisodes(out)
	return out
}

func catalogFields(spec config.Provider, list *config.CatalogList, item any, emptyPrefix string, limit int) (id, number, title, synopsis, poster, href, year, season string) {
	fields := list.Fields
	id = asString(dig(item, fields["id"]))
	number = asString(dig(item, fields["number"]))
	title = strings.TrimSpace(asString(dig(item, fields["title"])))
	if title == "" && number != "" {
		title = emptyPrefix + number
	}
	if id == "" {
		id = number
	}
	if p := fields["synopsis"]; p != "" {
		synopsis = clipText(stripHTML(asString(dig(item, p))), limit)
	}
	if p := fields["poster"]; p != "" {
		poster = asString(dig(item, p))
	}
	prefix := list.PosterPrefix
	if prefix == "" {
		prefix = spec.PosterPrefix
	}
	if prefix != "" && poster != "" {
		poster = prefix + poster
	}
	if p := fields["url"]; p != "" {
		href = asString(dig(item, p))
	}
	year = asString(dig(item, fields["year"]))
	if list.Year == "prefix4" && len(year) >= 4 {
		year = year[:4]
	}
	season = episodeSeasonKey(list, item)
	return
}

func episodeSeasonKey(list *config.CatalogList, item any) string {
	path := ""
	if list != nil && list.Fields != nil {
		path = list.Fields["season"]
	}
	if path == "" {
		path = "season"
	}
	return asString(dig(item, path))
}

func attachGroupedEpisodes(seasons []Season, grouped map[string][]Episode) []Season {
	used := map[string]bool{}
	for i := range seasons {
		key := seasons[i].Number
		if eps, ok := grouped[key]; ok {
			sortCatalogEpisodes(eps)
			seasons[i].Episodes = eps
			used[key] = true
		}
	}
	extras := make([]string, 0)
	for k := range grouped {
		if !used[k] {
			extras = append(extras, k)
		}
	}
	sort.Slice(extras, func(i, j int) bool {
		return catalogNumLess(extras[i], extras[j])
	})
	for _, k := range extras {
		eps := grouped[k]
		sortCatalogEpisodes(eps)
		title := "Season " + k
		if k == "" {
			title = "Season"
		}
		seasons = append(seasons, Season{Number: k, Title: title, Episodes: eps})
	}
	sortCatalogSeasons(seasons)
	return seasons
}

func sortCatalogSeasons(seasons []Season) {
	sort.SliceStable(seasons, func(i, j int) bool {
		return catalogNumLess(seasons[i].Number, seasons[j].Number)
	})
}

func sortCatalogEpisodes(eps []Episode) {
	sort.SliceStable(eps, func(i, j int) bool {
		return catalogNumLess(eps[i].Number, eps[j].Number)
	})
}

func catalogNumLess(a, b string) bool {
	ai, aerr := strconv.Atoi(a)
	bi, berr := strconv.Atoi(b)
	if aerr == nil && berr == nil {
		return ai < bi
	}
	if aerr == nil {
		return true
	}
	if berr == nil {
		return false
	}
	return a < b
}

func CatalogNumKey(s string) string {
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil {
		return s
	}
	return strconv.Itoa(n)
}
