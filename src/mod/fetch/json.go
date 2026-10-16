package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
)

var (
	paceMu     sync.Mutex
	paceLast   = map[string]time.Time{}
	htmlTag    = regexp.MustCompile(`(?s)<[^>]*>`)
	whitespace = regexp.MustCompile(`\s+`)
)

func providerGET(ctx context.Context, httpc *httpClient, rawURL string, query map[string]string, vars map[string]string, key string, pace func(context.Context) error) ([]byte, error) {
	u, err := url.Parse(expand(rawURL, vars))
	if err != nil {
		return nil, err
	}
	qs := u.Query()
	for k, val := range query {
		exp := expand(val, vars)
		if exp != "" {
			qs.Set(k, exp)
		}
	}
	u.RawQuery = qs.Encode()
	b, code, err := httpc.get(ctx, u.String(), pace, key)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, fmt.Errorf("status %d", code)
	}
	return b, nil
}

func varsFor(spec config.Provider, q rank.Query, id string) map[string]string {
	typeParam := ""
	if spec.TypeParams != nil {
		typeParam = spec.TypeParams[q.Type]
	}
	return map[string]string{
		"base":       spec.Base,
		"title":      q.Title,
		"year":       q.Year,
		"type":       q.Type,
		"id":         id,
		"api_key":    spec.APIKey,
		"type_param": typeParam,
		"season":     "",
		"episode":    "",
	}
}

func expand(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

func extractItems(raw []byte, path string) ([]any, error) {
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	v := dig(root, path)
	switch t := v.(type) {
	case []any:
		return t, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("items %q is not an array", path)
	}
}

func hitFrom(name string, spec config.Provider, item any, limit int) (rank.Hit, bool) {
	id := asString(dig(item, spec.Fields["id"]))
	title := asString(dig(item, spec.Fields["title"]))
	if id == "" || title == "" {
		return rank.Hit{}, false
	}
	year := asString(dig(item, spec.Fields["year"]))
	if spec.Year == "prefix4" && len(year) >= 4 {
		year = year[:4]
	}
	href := asString(dig(item, spec.Fields["url"]))
	if spec.URLPrefix != "" {
		href = spec.URLPrefix + id
	}
	synopsis := ""
	if p := spec.Fields["synopsis"]; p != "" {
		synopsis = clipText(stripHTML(asString(dig(item, p))), limit)
	}
	poster := ""
	if p := spec.Fields["poster"]; p != "" {
		poster = asString(dig(item, p))
	}
	if spec.PosterPrefix != "" && poster != "" {
		poster = spec.PosterPrefix + poster
	}
	return rank.Hit{
		Provider: name,
		ID:       id,
		Title:    title,
		Year:     year,
		URL:      href,
		Synopsis: synopsis,
		Poster:   poster,
		Attrs:    mergeAttrs(extraAttrs(spec.Fields, item), spec.Attrs),
		NFO:      spec.NFO,
		UniqueID: spec.UniqueID,
	}, true
}

func mergeAttrs(extracted, static map[string]string) map[string]string {
	if len(static) == 0 {
		return extracted
	}
	out := extracted
	if out == nil {
		out = map[string]string{}
	}
	for k, v := range static {
		v = strings.TrimSpace(v)
		if v == "" || strings.TrimSpace(out[k]) != "" {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var coreFields = map[string]bool{
	"id": true, "title": true, "year": true, "url": true, "synopsis": true, "poster": true,
}

func extraAttrs(fields map[string]string, item any) map[string]string {
	var attrs map[string]string
	for key, path := range fields {
		if coreFields[key] || path == "" {
			continue
		}
		s := strings.TrimSpace(asString(dig(item, path)))
		if s == "" {
			continue
		}
		if attrs == nil {
			attrs = map[string]string{}
		}
		attrs[key] = s
	}
	return attrs
}

func dig(v any, path string) any {
	if path == "" || path == "$" {
		return v
	}
	cur := v
	for _, p := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	return cur
}

func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprint(t)
	}
}

func stripHTML(s string) string {
	s = htmlTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(whitespace.ReplaceAllString(s, " "))
}

func clipText(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n]))
}

func paceProvider(ctx context.Context, name string, minMS int) error {
	if minMS <= 0 {
		return nil
	}
	gap := time.Duration(minMS) * time.Millisecond
	paceMu.Lock()
	wait := time.Duration(0)
	if last, ok := paceLast[name]; ok {
		wait = gap - time.Since(last)
	}
	if wait <= 0 {
		paceLast[name] = time.Now()
		paceMu.Unlock()
		return nil
	}
	paceLast[name] = time.Now().Add(wait)
	paceMu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}
