package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
)

type Client struct {
	cfg   *config.Config
	http  *httpClient
	cool  *Circuit
	cache *memo
}

func New(cfg *config.Config) *Client {
	cool := NewCircuit()
	return &Client{cfg: cfg, http: newHTTP(cfg, cool), cool: cool, cache: newMemo()}
}

func (c *Client) trip(key string) {
	if c == nil || c.cfg == nil {
		return
	}
	cd := c.cfg.HTTP.Cooldown
	c.cool.Fail(key, cd.Fails, cd.MinMS, cd.MaxMS)
}

func (c *Client) Search(ctx context.Context, q rank.Query) ([]rank.Hit, []string, int) {
	return c.collect(ctx, q, true)
}

func (c *Client) SearchOther(ctx context.Context, q rank.Query) ([]rank.Hit, []string, int) {
	return c.collect(ctx, q, false)
}

func (c *Client) collect(ctx context.Context, q rank.Query, wantedOnly bool) ([]rank.Hit, []string, int) {
	v := c.cache.doSearch(searchKey(q, wantedOnly), func() searchVal {
		hits, errs, ok := c.collectWaves(ctx, q, wantedOnly)
		return searchVal{hits: hits, errs: errs, ok: ok}
	})
	return v.hits, v.errs, v.ok
}

func (c *Client) collectWaves(ctx context.Context, q rank.Query, wantedOnly bool) ([]rank.Hit, []string, int) {
	hits, errs, ok := c.collectUncached(ctx, q, wantedOnly, config.UsePrimary)
	h2, e2, o2 := c.collectUncached(ctx, q, wantedOnly, config.UseDefer)
	hits = append(hits, h2...)
	errs = append(errs, e2...)
	ok += o2
	if ok == 0 {
		h3, e3, o3 := c.collectUncached(ctx, q, wantedOnly, config.UseFallback)
		hits = append(hits, h3...)
		errs = append(errs, e3...)
		ok += o3
	}
	return hits, errs, ok
}

func (c *Client) collectUncached(ctx context.Context, q rank.Query, wantedOnly bool, use string) ([]rank.Hit, []string, int) {
	type task struct {
		name string
		spec config.Provider
	}
	var tasks []task
	for _, ep := range c.cfg.Endpoints() {
		if providerUse(ep.Spec) != use {
			continue
		}
		wanted := providerWanted(ep.Spec, q.Type)
		if wantedOnly && !wanted {
			continue
		}
		if !wantedOnly && wanted {
			continue
		}
		if ep.Spec.Require == "api_key" && ep.Spec.APIKey == "" {
			continue
		}
		tasks = append(tasks, task{name: ep.Name, spec: ep.Spec})
	}
	if len(tasks) == 0 {
		return nil, nil, 0
	}
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		out  []rank.Hit
		errs []string
		ok   int
	)
	wg.Add(len(tasks))
	for _, t := range tasks {
		go func(name string, spec config.Provider) {
			defer wg.Done()
			key := endpointKey(name, spec)
			if err := c.cool.Wait(ctx, key); err != nil {
				mu.Lock()
				errs = append(errs, name+": "+err.Error())
				mu.Unlock()
				return
			}
			cs, err := c.searchEndpoint(ctx, name, spec, q)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, name+": "+err.Error())
				return
			}
			c.cool.OK(key)
			ok++
			out = append(out, cs...)
		}(t.name, t.spec)
	}
	wg.Wait()
	return out, errs, ok
}

func endpointKey(name string, spec config.Provider) string {
	if spec.NFO != "" {
		return name + "/" + spec.NFO
	}
	return name
}

func providerUse(spec config.Provider) string {
	if spec.Use == "" {
		return config.UsePrimary
	}
	return spec.Use
}

func providerWanted(spec config.Provider, jobType string) bool {
	if jobType == "" || len(spec.Types) == 0 {
		return true
	}
	for _, t := range spec.Types {
		if t == jobType {
			return true
		}
	}
	return false
}

func (c *Client) searchEndpoint(ctx context.Context, name string, spec config.Provider, q rank.Query) ([]rank.Hit, error) {
	if spec.Search == nil || strings.TrimSpace(spec.Search.URL) == "" {
		return nil, nil
	}
	key := endpointKey(name, spec)
	s := spec.Search
	raw, err := providerGET(ctx, c.http.forSpec(spec), s.URL, s.Query, varsFor(spec, q, ""), key, func(ctx context.Context) error {
		return sectionPace(ctx, name, spec, "search", s.MinIntervalMS)
	})
	if err != nil {
		return nil, err
	}
	items, err := extractItems(raw, s.Items)
	if err != nil {
		c.trip(key)
		return nil, err
	}
	out := make([]rank.Hit, 0, len(items))
	for _, item := range items {
		h, ok := hitFrom(name, spec, item, c.cfg.Match.SynopsisLimit)
		if ok {
			out = append(out, h)
		}
	}
	c.hydrateTitles(ctx, name, spec, q, out)
	return out, nil
}

func titlesMax(spec config.Provider) int {
	if spec.Titles != nil && spec.Titles.Max > 0 {
		return spec.Titles.Max
	}
	return 3
}

func (c *Client) hydrateTitles(ctx context.Context, name string, spec config.Provider, q rank.Query, cands []rank.Hit) {
	if spec.Titles == nil || strings.TrimSpace(spec.Titles.URL) == "" || len(cands) == 0 {
		return
	}
	stop := c.cfg.PlotStop()
	jobContent := rank.ContentSet(q.Title, stop)
	jobTokens := rank.TokenSet(q.Title)
	n := 0
	max := titlesMax(spec)
	for i := range cands {
		if n >= max {
			return
		}
		if rank.Coverage(jobTokens, rank.TokenSet(cands[i].Title)) >= 1 {
			continue
		}
		n++
		_ = c.fetchTitleAkas(ctx, name, spec, q, &cands[i], jobContent, stop)
	}
}

func (c *Client) fetchTitleAkas(ctx context.Context, name string, spec config.Provider, q rank.Query, cand *rank.Hit, jobContent map[string]struct{}, stop map[string]struct{}) error {
	tl := spec.Titles
	raw, err := providerGET(ctx, c.http.forSpec(spec), tl.URL, tl.Query, varsFor(spec, q, cand.ID), endpointKey(name, spec), func(ctx context.Context) error {
		return sectionPace(ctx, name, spec, "titles", tl.MinIntervalMS)
	})
	if err != nil {
		return err
	}
	items, err := extractItems(raw, tl.Items)
	if err != nil {
		return err
	}
	path := tl.Fields["title"]
	if path == "" {
		path = "name"
	}
	var kept []string
	for _, item := range items {
		s := strings.TrimSpace(asString(dig(item, path)))
		if s == "" || !rank.SharesContent(jobContent, rank.ContentSet(s, stop)) {
			continue
		}
		kept = append(kept, s)
	}
	if len(kept) == 0 {
		return nil
	}
	if cand.Attrs == nil {
		cand.Attrs = map[string]string{}
	}
	for i, s := range kept {
		key := "title_aka"
		if i > 0 {
			key = "title_aka_" + strconv.Itoa(i+1)
		}
		cand.Attrs[key] = s
	}
	return nil
}

func (c *Client) Detail(ctx context.Context, q rank.Query, cand *rank.Hit) {
	if cand == nil {
		return
	}
	spec, ok := c.cfg.Endpoint(cand.Provider, cand.NFO)
	if !ok || spec.Detail == nil {
		return
	}
	hit, ok := c.cache.doDetail(idKey(cand.Provider, cand.ID), func() (rank.Hit, bool) {
		src := *cand
		if err := sectionPace(ctx, cand.Provider, spec, "detail", spec.Detail.MinIntervalMS); err != nil {
			return src, false
		}
		raw, err := providerGET(ctx, c.http.forSpec(spec), spec.Detail.URL, spec.Detail.Query, varsFor(spec, q, cand.ID), endpointKey(cand.Provider, spec), nil)
		if err != nil {
			return src, false
		}
		var obj any
		if err := json.Unmarshal(raw, &obj); err != nil {
			return src, false
		}
		mergeDetail(&src, spec.Detail, obj, c.cfg.Match.SynopsisLimit)
		return src, true
	})
	if ok {
		applyDetail(cand, hit)
	}
}

func mergeDetail(cand *rank.Hit, ep *config.Episode, item any, limit int) {
	if cand == nil || ep == nil || ep.Fields == nil {
		return
	}
	if p := ep.Fields["synopsis"]; p != "" && cand.Synopsis == "" {
		cand.Synopsis = clipText(stripHTML(asString(dig(item, p))), limit)
	}
	if p := ep.Fields["title"]; p != "" && cand.Title == "" {
		cand.Title = asString(dig(item, p))
	}
	if p := ep.Fields["year"]; p != "" && cand.Year == "" {
		year := asString(dig(item, p))
		if ep.Year == "prefix4" && len(year) >= 4 {
			year = year[:4]
		}
		cand.Year = year
	}
	if p := ep.Fields["url"]; p != "" && cand.URL == "" {
		cand.URL = asString(dig(item, p))
	}
	if p := ep.Fields["poster"]; p != "" && cand.Poster == "" {
		cand.Poster = asString(dig(item, p))
	}
	if extra := extraAttrs(ep.Fields, item); len(extra) > 0 {
		cand.Attrs = overlayAttrs(cand.Attrs, extra)
	}
}

func overlayAttrs(base, extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return base
	}
	if base == nil {
		base = map[string]string{}
	}
	for k, v := range extra {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		base[k] = v
	}
	if len(base) == 0 {
		return nil
	}
	return base
}

func SearchErr(errs []string, ok int) error {
	if len(errs) > 0 && ok == 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
