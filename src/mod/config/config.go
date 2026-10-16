package config

import (
	_ "embed"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed default.yaml
var shippedDefault []byte

type Config struct {
	HTTP        HTTP                `yaml:"http"`
	DataDir     string              `yaml:"data_dir"`
	Version     string              `yaml:"version"`
	Match       Match               `yaml:"match"`
	Group       Group               `yaml:"group"`
	Ingest      Ingest              `yaml:"ingest"`
	Providers   map[string]Provider `yaml:"providers"`
	ConfigPath  string              `yaml:"-"`
	ExeDir      string              `yaml:"-"`
	OverlayPath string              `yaml:"-"`
}

type Group struct {
	ParentWeight      float64  `yaml:"parent_weight"`
	DistanceThreshold float64  `yaml:"distance_threshold"`
	VideoExt          []string `yaml:"video_ext"`
	SeasonPattern     string   `yaml:"season_pattern"`
}

type Ingest struct {
	SeqThreshold float64           `yaml:"seq_threshold"`
	SampleRows   int               `yaml:"sample_rows"`
	Aliases      map[string]string `yaml:"aliases"`
	Types        map[string]string `yaml:"types"`
}

type HTTP struct {
	DeadlineMS int      `yaml:"deadline_ms"`
	Retries    int      `yaml:"retries"`
	Backoff    MSRange  `yaml:"backoff"`
	AttemptMS  int      `yaml:"attempt_ms"`
	Cooldown   Cooldown `yaml:"cooldown"`
}

type Cooldown struct {
	Fails int `yaml:"fails"`
	MinMS int `yaml:"min_ms"`
	MaxMS int `yaml:"max_ms"`
}

type Match struct {
	MinScore      float64                      `yaml:"min_score"`
	MinMargin     float64                      `yaml:"min_margin"`
	MinHits       int                          `yaml:"min_hits"`
	Workers       int                          `yaml:"workers"`
	Prefer        map[string]map[string]string `yaml:"prefer"`
	PlotStop      []string                     `yaml:"plot_stop"`
	TitleLift     float64                      `yaml:"title_lift"`
	ExactLift     float64                      `yaml:"exact_lift"`
	SynopsisLimit int                          `yaml:"synopsis_limit"`
}

type MSRange struct {
	MinMS int `yaml:"min_ms"`
	MaxMS int `yaml:"max_ms"`
}

type Provider struct {
	Types             []string          `yaml:"types"`
	Require           string            `yaml:"require"`
	Secret            string            `yaml:"secret"`
	APIKey            string            `yaml:"api_key"`
	Base              string            `yaml:"base"`
	URL               string            `yaml:"url"`
	Query             map[string]string `yaml:"query"`
	Items             string            `yaml:"items"`
	Fields            map[string]string `yaml:"fields"`
	Year              string            `yaml:"year"`
	URLPrefix         string            `yaml:"url_prefix"`
	PosterPrefix      string            `yaml:"poster_prefix"`
	MinIntervalMS     int               `yaml:"min_interval_ms"`
	Retries           int               `yaml:"retries"`
	AttemptMS         int               `yaml:"attempt_ms"`
	Use               string            `yaml:"use"`
	TypeParams        map[string]string `yaml:"type_params"`
	NFO               string            `yaml:"nfo"`
	UniqueID          string            `yaml:"uniqueid"`
	Episode           *Episode          `yaml:"episode"`
	Detail            *Episode          `yaml:"detail"`
	Catalog           *Catalog          `yaml:"catalog"`
	Titles            *Titles           `yaml:"titles"`
	Attrs             map[string]string `yaml:"attrs"`
	Movie             *Provider         `yaml:"movie"`
	TV                *Provider         `yaml:"tv"`
}

type Episode struct {
	URL    string            `yaml:"url"`
	Query  map[string]string `yaml:"query"`
	Fields map[string]string `yaml:"fields"`
	Year   string            `yaml:"year"`
}

type Catalog struct {
	Seasons  *CatalogList `yaml:"seasons"`
	Episodes *CatalogList `yaml:"episodes"`
}

type CatalogList struct {
	URL          string            `yaml:"url"`
	Query        map[string]string `yaml:"query"`
	Items        string            `yaml:"items"`
	Fields       map[string]string `yaml:"fields"`
	Year         string            `yaml:"year"`
	PosterPrefix string            `yaml:"poster_prefix"`
}

type Titles struct {
	URL    string            `yaml:"url"`
	Query  map[string]string `yaml:"query"`
	Items  string            `yaml:"items"`
	Fields map[string]string `yaml:"fields"`
	Max    int               `yaml:"max"`
}

const (
	UsePrimary  = "primary"
	UseDefer    = "defer"
	UseFallback = "fallback"
)

type Endpoint struct {
	Name string
	Spec Provider
}

func New(overlayPath string) (*Config, error) {
	exeDir, err := exeDir()
	if err != nil {
		return nil, err
	}
	raw := shippedDefault
	overlayPath = strings.TrimSpace(overlayPath)
	if overlayPath != "" {
		over, err := os.ReadFile(overlayPath)
		if err != nil {
			return nil, err
		}
		raw, err = merge(raw, over)
		if err != nil {
			return nil, err
		}
	}
	cfg, err := decode(raw)
	if err != nil {
		return nil, err
	}
	dataDir, err := resolveDataDir(exeDir, cfg.DataDir)
	if err != nil {
		return nil, err
	}
	cfg.DataDir = dataDir
	cfg.ExeDir = exeDir
	cfg.OverlayPath = overlayPath
	if cfg.Providers == nil {
		cfg.Providers = map[string]Provider{}
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Endpoints() []Endpoint {
	if c == nil {
		return nil
	}
	var out []Endpoint
	for name, spec := range c.Providers {
		out = append(out, expandProvider(name, spec)...)
	}
	return out
}

func (c *Config) Endpoint(provider, nfo string) (Provider, bool) {
	for _, ep := range c.Endpoints() {
		if ep.Name != provider {
			continue
		}
		if nfo == "" || ep.Spec.NFO == nfo {
			return ep.Spec, true
		}
	}
	for _, ep := range c.Endpoints() {
		if ep.Name == provider {
			return ep.Spec, true
		}
	}
	return Provider{}, false
}

func expandProvider(name string, spec Provider) []Endpoint {
	if spec.Movie == nil && spec.TV == nil {
		return []Endpoint{{Name: name, Spec: spec}}
	}
	var out []Endpoint
	if spec.Movie != nil {
		out = append(out, Endpoint{Name: name, Spec: inherit(spec, *spec.Movie, "movie")})
	}
	if spec.TV != nil {
		out = append(out, Endpoint{Name: name, Spec: inherit(spec, *spec.TV, "tvshow")})
	}
	return out
}

func inherit(parent, child Provider, defaultNFO string) Provider {
	out := parent
	out.Movie = nil
	out.TV = nil
	if len(child.Types) > 0 {
		out.Types = child.Types
	}
	if child.URL != "" {
		out.URL = child.URL
	}
	if child.Query != nil {
		out.Query = child.Query
	}
	if child.Items != "" {
		out.Items = child.Items
	}
	if child.Fields != nil {
		out.Fields = child.Fields
	}
	if child.Year != "" {
		out.Year = child.Year
	}
	if child.URLPrefix != "" {
		out.URLPrefix = child.URLPrefix
	}
	if child.PosterPrefix != "" {
		out.PosterPrefix = child.PosterPrefix
	}
	if child.NFO != "" {
		out.NFO = child.NFO
	} else if out.NFO == "" {
		out.NFO = defaultNFO
	}
	if child.UniqueID != "" {
		out.UniqueID = child.UniqueID
	}
	if child.Catalog != nil {
		out.Catalog = child.Catalog
	}
	if child.Episode != nil {
		out.Episode = child.Episode
	}
	if child.Detail != nil {
		out.Detail = child.Detail
	}
	if child.Titles != nil {
		out.Titles = child.Titles
	}
	if child.Attrs != nil {
		out.Attrs = child.Attrs
	}
	if child.APIKey != "" {
		out.APIKey = child.APIKey
	}
	if child.Require != "" {
		out.Require = child.Require
	}
	if child.MinIntervalMS > 0 {
		out.MinIntervalMS = child.MinIntervalMS
	}
	if child.Retries > 0 {
		out.Retries = child.Retries
	}
	if child.AttemptMS > 0 {
		out.AttemptMS = child.AttemptMS
	}
	if child.Use != "" {
		out.Use = child.Use
	}
	if out.Use == "" {
		out.Use = UsePrimary
	}
	return out
}

func Validate(c Config) error {
	if strings.TrimSpace(c.Version) == "" {
		return fmt.Errorf("version is empty")
	}
	if c.HTTP.DeadlineMS <= 0 {
		return fmt.Errorf("http.deadline_ms must be > 0")
	}
	if c.HTTP.Retries <= 0 {
		return fmt.Errorf("http.retries must be > 0")
	}
	if c.HTTP.AttemptMS <= 0 {
		return fmt.Errorf("http.attempt_ms must be > 0")
	}
	if err := validateMS("http.backoff", c.HTTP.Backoff); err != nil {
		return err
	}
	if c.HTTP.Cooldown.Fails < 0 {
		return fmt.Errorf("http.cooldown.fails must be >= 0")
	}
	if err := validateMS("http.cooldown", MSRange{MinMS: c.HTTP.Cooldown.MinMS, MaxMS: c.HTTP.Cooldown.MaxMS}); err != nil {
		return err
	}
	if c.Match.SynopsisLimit <= 0 {
		return fmt.Errorf("match.synopsis_limit must be > 0")
	}
	if c.Match.MinScore <= 0 {
		return fmt.Errorf("match.min_score must be > 0")
	}
	if c.Match.MinMargin < 0 {
		return fmt.Errorf("match.min_margin must be >= 0")
	}
	if c.Match.MinHits < 1 {
		return fmt.Errorf("match.min_hits must be >= 1")
	}
	if c.Match.Workers < 1 {
		return fmt.Errorf("match.workers must be >= 1")
	}
	if len(wordList(c.Match.PlotStop)) == 0 {
		return fmt.Errorf("match.plot_stop is empty")
	}
	if c.Match.TitleLift < 0 {
		return fmt.Errorf("match.title_lift must be >= 0")
	}
	if c.Match.ExactLift < 0 {
		return fmt.Errorf("match.exact_lift must be >= 0")
	}
	if c.Group.ParentWeight < 0 {
		return fmt.Errorf("group.parent_weight must be >= 0")
	}
	if c.Group.DistanceThreshold <= 0 {
		return fmt.Errorf("group.distance_threshold must be > 0")
	}
	if len(wordList(c.Group.VideoExt)) == 0 {
		return fmt.Errorf("group.video_ext is empty")
	}
	if strings.TrimSpace(c.Group.SeasonPattern) == "" {
		return fmt.Errorf("group.season_pattern is empty")
	}
	if _, err := regexp.Compile(c.Group.SeasonPattern); err != nil {
		return fmt.Errorf("group.season_pattern: %w", err)
	}
	if c.Ingest.SeqThreshold <= 0 || c.Ingest.SeqThreshold > 1 {
		return fmt.Errorf("ingest.seq_threshold must be in (0, 1]")
	}
	if c.Ingest.SampleRows < 1 {
		return fmt.Errorf("ingest.sample_rows must be >= 1")
	}
	for name, spec := range c.Providers {
		if err := validateProviderUse(name, spec); err != nil {
			return err
		}
	}
	return nil
}

func validateProviderUse(name string, spec Provider) error {
	if err := checkUse(name, spec.Use); err != nil {
		return err
	}
	if spec.Movie != nil {
		if err := checkUse(name+".movie", spec.Movie.Use); err != nil {
			return err
		}
	}
	if spec.TV != nil {
		if err := checkUse(name+".tv", spec.TV.Use); err != nil {
			return err
		}
	}
	return nil
}

func checkUse(name, use string) error {
	switch use {
	case "", UsePrimary, UseDefer, UseFallback:
		return nil
	default:
		return fmt.Errorf("providers.%s.use %q is not primary, defer, or fallback", name, use)
	}
}

func validateMS(name string, r MSRange) error {
	if r.MinMS <= 0 {
		return fmt.Errorf("%s.min_ms must be > 0", name)
	}
	if r.MaxMS < r.MinMS {
		return fmt.Errorf("%s.max_ms must be >= min_ms", name)
	}
	return nil
}

func (c Config) GroupVideoExt() map[string]struct{} {
	return extSet(c.Group.VideoExt)
}

func (c Config) Season() *regexp.Regexp {
	re, err := regexp.Compile(c.Group.SeasonPattern)
	if err != nil {
		return regexp.MustCompile(`a^`)
	}
	return re
}

func (c Config) PlotStop() map[string]struct{} {
	return wordSet(c.Match.PlotStop)
}

func (c Config) TitleLift() float64 {
	if c.Match.TitleLift <= 0 {
		return 0.15
	}
	return c.Match.TitleLift
}

func (c Config) ExactLift() float64 {
	if c.Match.ExactLift <= 0 {
		return 0.25
	}
	return c.Match.ExactLift
}

func (c Config) HTTPTimeout() time.Duration {
	return time.Duration(c.HTTP.DeadlineMS) * time.Millisecond
}

func (c Config) AttemptTimeout() time.Duration {
	return time.Duration(c.HTTP.AttemptMS) * time.Millisecond
}

func JitterMS(minMS, maxMS int) time.Duration {
	if minMS < 1 {
		minMS = 1
	}
	if maxMS < minMS {
		maxMS = minMS
	}
	n := int64(minMS)
	span := int64(maxMS) - n
	if span > 0 {
		n += rand.Int64N(span + 1)
	}
	return time.Duration(n) * time.Millisecond
}

func exeDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe), nil
}

func resolveDataDir(exeDir, p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		if v := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); filepath.IsAbs(v) {
			return filepath.Join(v, "matchmedia"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return "", fmt.Errorf("data_dir unset and HOME unavailable")
		}
		return filepath.Join(home, ".local", "share", "matchmedia"), nil
	}
	if filepath.IsAbs(p) {
		return p, nil
	}
	return filepath.Join(exeDir, p), nil
}

func wordList(list []string) []string {
	out := make([]string, 0, len(list))
	for _, w := range list {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" {
			continue
		}
		out = append(out, w)
	}
	return out
}

func extSet(list []string) map[string]struct{} {
	out := make(map[string]struct{}, len(list))
	for _, e := range wordList(list) {
		e = strings.TrimPrefix(e, ".")
		if e == "" {
			continue
		}
		out["."+e] = struct{}{}
	}
	return out
}

func wordSet(list []string) map[string]struct{} {
	out := make(map[string]struct{}, len(list))
	for _, w := range wordList(list) {
		out[w] = struct{}{}
	}
	return out
}

func merge(base, over []byte) ([]byte, error) {
	var a, b map[string]any
	if err := yaml.Unmarshal(base, &a); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(over, &b); err != nil {
		return nil, err
	}
	mergeMap(a, b)
	return yaml.Marshal(a)
}

func mergeMap(dst, src map[string]any) {
	for k, v := range src {
		if sm, ok := asMap(v); ok {
			if dm, ok := asMap(dst[k]); ok {
				mergeMap(dm, sm)
				dst[k] = dm
				continue
			}
		}
		dst[k] = v
	}
}

func asMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			if s, ok := k.(string); ok {
				out[s] = val
			}
		}
		return out, true
	default:
		return nil, false
	}
}

func decode(b []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	for name, p := range cfg.Providers {
		use, err := parseUse(p.Use)
		if err != nil {
			return Config{}, fmt.Errorf("providers.%s: %w", name, err)
		}
		if use == "" {
			use = UsePrimary
		}
		p.Use = use
		if p.Movie != nil {
			u, err := parseUse(p.Movie.Use)
			if err != nil {
				return Config{}, fmt.Errorf("providers.%s.movie: %w", name, err)
			}
			p.Movie.Use = u
		}
		if p.TV != nil {
			u, err := parseUse(p.TV.Use)
			if err != nil {
				return Config{}, fmt.Errorf("providers.%s.tv: %w", name, err)
			}
			p.TV.Use = u
		}
		cfg.Providers[name] = p
	}
	return cfg, nil
}

func parseUse(s string) (string, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "", nil
	}
	switch s {
	case UsePrimary, UseDefer, UseFallback:
		return s, nil
	default:
		return "", fmt.Errorf("use %q is not primary, defer, or fallback", s)
	}
}
