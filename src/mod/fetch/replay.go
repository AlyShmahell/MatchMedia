package fetch

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
)

// Replay requests each saved provider call. ok is false when any request fails
// or a body is not that section's document. An empty list from a successful
// response is ok. Season numbers present in a seasons body but missing from
// the saved episode requests are fetched from the YAML template.
func (c *Client) Replay(ctx context.Context, q rank.Query, reqs []rank.Request) (rank.Hit, []Season, bool) {
	if c == nil || c.cfg == nil || len(reqs) == 0 {
		return rank.Hit{}, nil, false
	}
	type got struct {
		req  rank.Request
		raw  []byte
		spec config.Provider
	}
	gotAll := make([]got, 0, len(reqs))
	for _, req := range reqs {
		spec, ok := c.cfg.Endpoint(req.Provider, req.NFO)
		if !ok || strings.TrimSpace(req.URL) == "" {
			return rank.Hit{}, nil, false
		}
		raw, ok := c.replayOne(ctx, spec, req)
		if !ok {
			return rank.Hit{}, nil, false
		}
		gotAll = append(gotAll, got{req: req, raw: raw, spec: spec})
	}
	primary := reqs[0]
	for _, r := range reqs {
		if r.Section == "detail" || strings.HasPrefix(r.Section, "catalog/") {
			primary = r
			break
		}
	}
	spec0, ok := c.cfg.Endpoint(primary.Provider, primary.NFO)
	if !ok {
		return rank.Hit{}, nil, false
	}
	hit := rank.Hit{
		Provider: primary.Provider, ID: savedID(spec0, primary), NFO: spec0.NFO, UniqueID: spec0.UniqueID,
		Title: q.Title, Year: q.Year, Requests: append([]rank.Request{}, reqs...),
	}
	var seasons []Season
	var flat []parsedEpisode
	limit := c.cfg.Match.SynopsisLimit
	for _, g := range gotAll {
		switch g.req.Section {
		case "detail":
			var obj any
			if err := json.Unmarshal(g.raw, &obj); err != nil {
				return rank.Hit{}, nil, false
			}
			if _, isObj := obj.(map[string]any); !isObj {
				return rank.Hit{}, nil, false
			}
			if g.spec.Detail != nil {
				mergeDetail(&hit, g.spec.Detail, obj, limit)
			}
		case "titles":
			if g.spec.Titles == nil {
				return rank.Hit{}, nil, false
			}
			if _, err := extractItems(g.raw, g.spec.Titles.Items); err != nil {
				return rank.Hit{}, nil, false
			}
		case "catalog/seasons":
			parsed, ok := parseSeasons(g.spec, g.raw, limit)
			if !ok {
				return rank.Hit{}, nil, false
			}
			seasons = append(seasons, parsed...)
		case "catalog/episodes":
			eps, ok := parseEpisodes(g.spec, g.raw, limit)
			if !ok {
				return rank.Hit{}, nil, false
			}
			if g.req.Season != "" {
				seasons = putSeasonEpisodes(seasons, g.req.Season, eps)
				continue
			}
			flat = append(flat, eps...)
		default:
			return rank.Hit{}, nil, false
		}
	}
	if len(flat) > 0 {
		grouped := map[string][]Episode{}
		for _, ep := range flat {
			grouped[ep.SeasonGroup] = append(grouped[ep.SeasonGroup], ep.Episode)
		}
		seasons = attachGroupedEpisodes(seasons, grouped)
	}
	if hit.Provider == "" || hit.ID == "" {
		return rank.Hit{}, nil, false
	}
	spec, ok := c.cfg.Endpoint(hit.Provider, hit.NFO)
	if !ok {
		return rank.Hit{}, nil, false
	}
	seasons, ok = c.replayMissingEpisodes(ctx, spec, q, hit, seasons, reqs)
	if !ok {
		return rank.Hit{}, nil, false
	}
	if seasons == nil {
		seasons = []Season{}
	}
	return hit, seasons, true
}

type parsedEpisode struct {
	Episode     Episode
	SeasonGroup string
}

func (c *Client) replayOne(ctx context.Context, spec config.Provider, req rank.Request) ([]byte, bool) {
	if err := sectionPace(ctx, req.Provider, spec, req.Section, sectionInterval(spec, req.Section)); err != nil {
		return nil, false
	}
	raw, err := providerGET(ctx, c.http.forSpec(spec), req.URL, replayQuery(spec, req.Section, req.Query), nil, endpointKey(req.Provider, spec), nil)
	if err != nil {
		return nil, false
	}
	return raw, true
}

func sectionInterval(spec config.Provider, section string) int {
	switch section {
	case "detail":
		if spec.Detail != nil {
			return spec.Detail.MinIntervalMS
		}
	case "titles":
		if spec.Titles != nil {
			return spec.Titles.MinIntervalMS
		}
	case "catalog/seasons":
		if spec.Catalog != nil && spec.Catalog.Seasons != nil {
			return spec.Catalog.Seasons.MinIntervalMS
		}
	case "catalog/episodes":
		if spec.Catalog != nil && spec.Catalog.Episodes != nil {
			return spec.Catalog.Episodes.MinIntervalMS
		}
	}
	return 0
}

func parseSeasons(spec config.Provider, raw []byte, limit int) ([]Season, bool) {
	if spec.Catalog == nil || spec.Catalog.Seasons == nil {
		return nil, false
	}
	items, err := extractItems(raw, spec.Catalog.Seasons.Items)
	if err != nil {
		return nil, false
	}
	var seasons []Season
	for _, item := range items {
		s, ok := seasonFrom(spec, spec.Catalog.Seasons, item, limit)
		if ok {
			seasons = append(seasons, s)
		}
	}
	sortCatalogSeasons(seasons)
	return seasons, true
}

func parseEpisodes(spec config.Provider, raw []byte, limit int) ([]parsedEpisode, bool) {
	if spec.Catalog == nil || spec.Catalog.Episodes == nil {
		return nil, false
	}
	list := spec.Catalog.Episodes
	items, err := extractItems(raw, list.Items)
	if err != nil {
		return nil, false
	}
	var out []parsedEpisode
	for _, item := range items {
		ep, ok := episodeFrom(spec, list, item, limit)
		if !ok {
			continue
		}
		out = append(out, parsedEpisode{Episode: ep, SeasonGroup: episodeSeasonKey(list, item)})
	}
	return out, true
}

func putSeasonEpisodes(seasons []Season, number string, eps []parsedEpisode) []Season {
	var list []Episode
	for _, ep := range eps {
		list = append(list, ep.Episode)
	}
	sortCatalogEpisodes(list)
	for i := range seasons {
		if seasons[i].Number == number {
			seasons[i].Episodes = list
			return seasons
		}
	}
	return append(seasons, Season{Number: number, Title: "Season " + number, Episodes: list})
}

func (c *Client) replayMissingEpisodes(ctx context.Context, spec config.Provider, q rank.Query, hit rank.Hit, seasons []Season, reqs []rank.Request) ([]Season, bool) {
	if spec.Catalog == nil || spec.Catalog.Episodes == nil || spec.Catalog.Episodes.URL == "" {
		return seasons, true
	}
	seen := map[string]struct{}{}
	var savedFlat bool
	for _, req := range reqs {
		if req.Section != "catalog/episodes" || req.Provider != hit.Provider {
			continue
		}
		if req.Season == "" {
			savedFlat = true
			continue
		}
		seen[req.Season] = struct{}{}
	}
	list := spec.Catalog.Episodes
	limit := c.cfg.Match.SynopsisLimit
	if !perSeasonURL(list.URL) {
		if savedFlat {
			return seasons, true
		}
		items, _, err := c.fetchCatalogList(ctx, hit.Provider, spec, q, list, "catalog/episodes", hit.ID, "", "")
		if err != nil {
			return nil, false
		}
		grouped := map[string][]Episode{}
		for _, item := range items {
			ep, ok := episodeFrom(spec, list, item, limit)
			if !ok {
				continue
			}
			key := episodeSeasonKey(list, item)
			grouped[key] = append(grouped[key], ep)
		}
		return attachGroupedEpisodes(seasons, grouped), true
	}
	for i := range seasons {
		if _, ok := seen[seasons[i].Number]; ok {
			continue
		}
		if strings.Contains(list.URL, "{season}") && seasons[i].Number == "" {
			continue
		}
		if strings.Contains(list.URL, "{season_id}") && seasons[i].ID == "" {
			continue
		}
		items, _, err := c.fetchCatalogList(ctx, hit.Provider, spec, q, list, "catalog/episodes", hit.ID, seasons[i].Number, seasons[i].ID)
		if err != nil {
			return nil, false
		}
		seasons[i].Episodes = episodesFrom(spec, list, items, limit)
	}
	return seasons, true
}

func savedID(spec config.Provider, req rank.Request) string {
	vals := parseQuery(req.Query)
	for k, v := range sectionQuery(spec, req.Section) {
		if strings.TrimSpace(v) == "{id}" {
			if id := vals[k]; id != "" {
				return id
			}
		}
	}
	return captureBrace(sectionURL(spec, req.Section), spec.Base, req.URL, "id")
}

func captureBrace(template, base, actual, name string) string {
	template = strings.ReplaceAll(template, "{base}", base)
	token := "{" + name + "}"
	i := strings.Index(template, token)
	if i < 0 || !strings.HasPrefix(actual, template[:i]) {
		return ""
	}
	rest := actual[len(template[:i]):]
	if q := strings.IndexByte(rest, '?'); q >= 0 {
		rest = rest[:q]
	}
	suffix := template[i+len(token):]
	if cut := strings.Index(suffix, "{"); cut >= 0 {
		suffix = suffix[:cut]
	}
	if suffix == "" {
		return rest
	}
	j := strings.Index(rest, suffix)
	if j < 0 {
		return ""
	}
	return rest[:j]
}
