package fetch

import (
	"strconv"
	"sync"

	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
)

type searchVal struct {
	hits []rank.Hit
	errs []string
	ok   int
}

type searchFlight struct {
	wg  sync.WaitGroup
	val searchVal
}

type detailFlight struct {
	wg  sync.WaitGroup
	hit rank.Hit
	ok  bool
}

type catalogFlight struct {
	wg      sync.WaitGroup
	seasons []Season
	forKey  string
}

type memo struct {
	mu      sync.Mutex
	search  map[string]*searchFlight
	detail  map[string]*detailFlight
	catalog map[string]*catalogFlight
}

func newMemo() *memo {
	return &memo{
		search:  map[string]*searchFlight{},
		detail:  map[string]*detailFlight{},
		catalog: map[string]*catalogFlight{},
	}
}

func searchKey(q rank.Query, wantedOnly bool) string {
	return rank.TitleNorm(q.Title) + "\t" + q.Year + "\t" + q.Type + "\t" + strconv.FormatBool(wantedOnly)
}

func idKey(provider, id string) string {
	return provider + "\t" + id
}

func (m *memo) doSearch(key string, fn func() searchVal) searchVal {
	m.mu.Lock()
	if e, ok := m.search[key]; ok {
		m.mu.Unlock()
		e.wg.Wait()
		return copySearch(e.val)
	}
	e := &searchFlight{}
	e.wg.Add(1)
	m.search[key] = e
	m.mu.Unlock()
	e.val = fn()
	e.wg.Done()
	return copySearch(e.val)
}

func (m *memo) doDetail(key string, fn func() (rank.Hit, bool)) (rank.Hit, bool) {
	m.mu.Lock()
	if e, ok := m.detail[key]; ok {
		m.mu.Unlock()
		e.wg.Wait()
		return copyHit(e.hit), e.ok
	}
	e := &detailFlight{}
	e.wg.Add(1)
	m.detail[key] = e
	m.mu.Unlock()
	e.hit, e.ok = fn()
	e.wg.Done()
	return copyHit(e.hit), e.ok
}

func (m *memo) doCatalog(key string, fn func() ([]Season, string)) ([]Season, string) {
	m.mu.Lock()
	if e, ok := m.catalog[key]; ok {
		m.mu.Unlock()
		e.wg.Wait()
		return copySeasons(e.seasons), e.forKey
	}
	e := &catalogFlight{}
	e.wg.Add(1)
	m.catalog[key] = e
	m.mu.Unlock()
	e.seasons, e.forKey = fn()
	e.wg.Done()
	return copySeasons(e.seasons), e.forKey
}

func copySearch(v searchVal) searchVal {
	return searchVal{hits: copyHits(v.hits), errs: append([]string(nil), v.errs...), ok: v.ok}
}

func copyHits(in []rank.Hit) []rank.Hit {
	if in == nil {
		return nil
	}
	out := make([]rank.Hit, len(in))
	for i, h := range in {
		out[i] = copyHit(h)
	}
	return out
}

func copyHit(h rank.Hit) rank.Hit {
	if h.Attrs != nil {
		a := make(map[string]string, len(h.Attrs))
		for k, v := range h.Attrs {
			a[k] = v
		}
		h.Attrs = a
	}
	return h
}

func copySeasons(in []Season) []Season {
	if in == nil {
		return nil
	}
	out := make([]Season, len(in))
	for i, s := range in {
		out[i] = s
		if s.Episodes != nil {
			eps := make([]Episode, len(s.Episodes))
			copy(eps, s.Episodes)
			out[i].Episodes = eps
		}
	}
	return out
}

func applyDetail(dst *rank.Hit, src rank.Hit) {
	if dst == nil {
		return
	}
	if src.Synopsis != "" {
		dst.Synopsis = src.Synopsis
	}
	if src.Title != "" {
		dst.Title = src.Title
	}
	if src.Year != "" {
		dst.Year = src.Year
	}
	if src.URL != "" {
		dst.URL = src.URL
	}
	if src.Poster != "" {
		dst.Poster = src.Poster
	}
}
