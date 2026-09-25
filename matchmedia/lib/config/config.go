package config

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const appName = "matchmedia"

type Config struct {
	HTTP        HTTP                `yaml:"http"`
	DataDir     string              `yaml:"data_dir"`
	BrowseRoots []string            `yaml:"browse_roots"`
	Version     string              `yaml:"version"`
	Match       Match               `yaml:"match"`
	Session     Session             `yaml:"session"`
	Group       Group               `yaml:"group"`
	Ingest      Ingest              `yaml:"ingest"`
	Scan        Scan                `yaml:"scan"`
	Providers   map[string]Provider `yaml:"providers"`
	ConfigPath  string              `yaml:"-"`
	ExeDir      string              `yaml:"-"`
	StateDir    string              `yaml:"-"`
}

type Group struct {
	SeqThreshold float64  `yaml:"seq_threshold"`
	VideoExt     []string `yaml:"video_ext"`
	Extras       []string `yaml:"extras"`
	Release      []string `yaml:"release"`
	Kinds        []string `yaml:"kinds"`
}

type Ingest struct {
	SampleRows int               `yaml:"sample_rows"`
	Aliases    map[string]string `yaml:"aliases"`
	Types      map[string]string `yaml:"types"`
}

type HTTP struct {
	Addr              string   `yaml:"addr"`
	TimeoutMS         int      `yaml:"timeout_ms"`
	Retries           int      `yaml:"retries"`
	Backoff           ExpRange `yaml:"backoff"`
	ProviderTimeoutMS int      `yaml:"provider_timeout_ms"`
}

type Scan struct {
	SampleVideos int `yaml:"sample_videos"`
}

type Session struct {
	TTLMS    int `yaml:"ttl_ms"`
	TTLMaxMS int `yaml:"ttl_max_ms"`
}

func (c Config) SessionTTLMax() time.Duration {
	if c.Session.TTLMaxMS <= 0 {
		return 0
	}
	return time.Duration(c.Session.TTLMaxMS) * time.Millisecond
}

func (c Config) SessionTTL() time.Duration {
	max := c.SessionTTLMax()
	if c.Session.TTLMS <= 0 {
		return max
	}
	d := time.Duration(c.Session.TTLMS) * time.Millisecond
	if max > 0 && d > max {
		return max
	}
	return d
}

type Match struct {
	MinScore      float64                      `yaml:"min_score"`
	SoloMinScore  float64                      `yaml:"solo_min_score"`
	MinMargin     float64                      `yaml:"min_margin"`
	MinHits       int                          `yaml:"min_hits"`
	Workers       int                          `yaml:"workers"`
	CooldownFails int                          `yaml:"cooldown_fails"`
	Cooldown      ExpRange                     `yaml:"cooldown"`
	Prefer        map[string]map[string]string `yaml:"prefer"`
	PlotStop      []string                     `yaml:"plot_stop"`
	TitleLift     float64                      `yaml:"title_lift"`
	ExactLift     float64                      `yaml:"exact_lift"`
	WaitCap       int                          `yaml:"wait_cap"`
	SynopsisLimit int                          `yaml:"synopsis_limit"`
}

type ExpRange struct {
	MinExp int `yaml:"min_exp"`
	MaxExp int `yaml:"max_exp"`
}

type Provider struct {
	Types             []string          `yaml:"types"`
	Require           string            `yaml:"require"`
	Secret            string            `yaml:"secret"`
	APIKey            string            `yaml:"-"`
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
	ProviderTimeoutMS int               `yaml:"provider_timeout_ms"`
	Defer             bool              `yaml:"defer"`
	TypeParams        map[string]string `yaml:"type_params"`
	NFO               string            `yaml:"nfo"`
	UniqueID          string            `yaml:"uniqueid"`
	Episode           *Episode          `yaml:"episode"`
	Detail            *Episode          `yaml:"detail"`
	Catalog           *Catalog          `yaml:"catalog"`
	Titles            *Titles           `yaml:"titles"`
	Attrs             map[string]string `yaml:"attrs"`
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

func ExeDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe), nil
}

func resolvePath(base, p, fallback string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		p = fallback
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}

func xdgBase(envKey, homeRel string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(envKey)); filepath.IsAbs(v) {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" || !filepath.IsAbs(home) {
		return "", fmt.Errorf("%s unset and HOME unavailable", envKey)
	}
	return filepath.Join(home, homeRel), nil
}

func appDir(envKey, homeRel string) (string, error) {
	base, err := xdgBase(envKey, homeRel)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}

func dataDirDefault() (string, error) {
	return appDir("XDG_DATA_HOME", filepath.Join(".local", "share"))
}

func mergeExisting(base []byte, path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return base, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return base, nil
	}
	return merge(base, b)
}

func DefaultConfigPath() (string, error) {
	dir, err := dataDirDefault()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config", "default.yaml"), nil
}

func PublicDir() (string, error) {
	dir, err := dataDirDefault()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "public"), nil
}

func resolveBrowseRoots(entries []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, entry := range entries {
		p := filepath.Clean(resolveBrowseEntry(entry))
		if p == "" || p == "." {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		st, err := os.Stat(p)
		if err != nil || !st.IsDir() {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func resolveBrowseEntry(entry string) string {
	entry = strings.Trim(strings.TrimSpace(entry), `"'`)
	switch entry {
	case "$XDG_VIDEOS_DIR", "${XDG_VIDEOS_DIR}":
		return userMediaDir("XDG_VIDEOS_DIR", "Videos")
	case "$XDG_MUSIC_DIR", "${XDG_MUSIC_DIR}":
		return userMediaDir("XDG_MUSIC_DIR", "Music")
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" || !filepath.IsAbs(home) {
		home = ""
	}
	if home != "" {
		entry = strings.ReplaceAll(entry, "${HOME}", home)
		entry = strings.ReplaceAll(entry, "$HOME", home)
	}
	if entry == "" {
		return ""
	}
	if filepath.IsAbs(entry) || home == "" {
		return entry
	}
	return filepath.Join(home, entry)
}

func userMediaDir(key, fallback string) string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" || !filepath.IsAbs(home) {
		return ""
	}
	raw := userDirValue(key)
	if raw == "" {
		return filepath.Join(home, fallback)
	}
	raw = strings.ReplaceAll(raw, "${HOME}", home)
	raw = strings.ReplaceAll(raw, "$HOME", home)
	if filepath.IsAbs(raw) {
		return raw
	}
	return filepath.Join(home, raw)
}

func userDirValue(key string) string {
	base, err := xdgBase("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(base, "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.HasPrefix(line, prefix) {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		return strings.Trim(val, `"'`)
	}
	return ""
}

func resolveDataDir(exeDir, p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return dataDirDefault()
	}
	if filepath.IsAbs(p) {
		return p, nil
	}
	return filepath.Join(exeDir, p), nil
}

func Load(path string) (Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Config{}, fmt.Errorf("-config is required")
	}
	root, err := ExeDir()
	if err != nil {
		return Config{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	base, err := decode(raw)
	if err != nil {
		return Config{}, err
	}
	dataDir, err := resolveDataDir(root, base.DataDir)
	if err != nil {
		return Config{}, err
	}
	raw, err = mergeExisting(raw, overlayPath(dataDir))
	if err != nil {
		return Config{}, err
	}
	cfg, err := decode(raw)
	if err != nil {
		return Config{}, err
	}
	dataDir, err = resolveDataDir(root, cfg.DataDir)
	if err != nil {
		return Config{}, err
	}
	stateDir, err := appDir("XDG_STATE_HOME", filepath.Join(".local", "state"))
	if err != nil {
		return Config{}, err
	}
	cfg.ExeDir = root
	cfg.StateDir = stateDir
	cfg.DataDir = dataDir
	cfg.BrowseRoots = resolveBrowseRoots(cfg.BrowseRoots)
	if cfg.Providers == nil {
		cfg.Providers = map[string]Provider{}
	}
	cfg.ConfigPath = path
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	applySecrets(&cfg)
	return cfg, nil
}

func Validate(c Config) error {
	if strings.TrimSpace(c.Version) == "" {
		return fmt.Errorf("version is empty")
	}
	if strings.TrimSpace(c.HTTP.Addr) == "" {
		return fmt.Errorf("http.addr is empty")
	}
	if c.HTTP.TimeoutMS <= 0 {
		return fmt.Errorf("http.timeout_ms must be > 0")
	}
	if c.HTTP.Retries <= 0 {
		return fmt.Errorf("http.retries must be > 0")
	}
	if c.HTTP.ProviderTimeoutMS <= 0 {
		return fmt.Errorf("http.provider_timeout_ms must be > 0")
	}
	if err := validateExp("http.backoff", c.HTTP.Backoff); err != nil {
		return err
	}
	if c.Session.TTLMS <= 0 {
		return fmt.Errorf("session.ttl_ms must be > 0")
	}
	if c.Session.TTLMaxMS <= 0 {
		return fmt.Errorf("session.ttl_max_ms must be > 0")
	}
	if c.Scan.SampleVideos <= 0 {
		return fmt.Errorf("scan.sample_videos must be > 0")
	}
	if c.Match.WaitCap <= 0 {
		return fmt.Errorf("match.wait_cap must be > 0")
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
	if c.Match.CooldownFails < 0 {
		return fmt.Errorf("match.cooldown_fails must be >= 0")
	}
	if err := validateExp("match.cooldown", c.Match.Cooldown); err != nil {
		return err
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
	if c.Group.SeqThreshold <= 0 || c.Group.SeqThreshold > 1 {
		return fmt.Errorf("group.seq_threshold must be in (0, 1]")
	}
	if len(wordList(c.Group.VideoExt)) == 0 {
		return fmt.Errorf("group.video_ext is empty")
	}
	if len(wordList(c.Group.Extras)) == 0 {
		return fmt.Errorf("group.extras is empty")
	}
	if len(wordList(c.Group.Release)) == 0 {
		return fmt.Errorf("group.release is empty")
	}
	if len(wordList(c.Group.Kinds)) == 0 {
		return fmt.Errorf("group.kinds is empty")
	}
	if c.Ingest.SampleRows < 1 {
		return fmt.Errorf("ingest.sample_rows must be >= 1")
	}
	return nil
}

func validateExp(name string, r ExpRange) error {
	if r.MinExp < 0 {
		return fmt.Errorf("%s.min_exp must be >= 0", name)
	}
	if r.MaxExp < r.MinExp+2 {
		return fmt.Errorf("%s.max_exp must be >= min_exp+2", name)
	}
	return nil
}

func (c Config) IngestSampleRows() int {
	return c.Ingest.SampleRows
}

func (c Config) SampleVideos() int {
	return c.Scan.SampleVideos
}

func (c Config) WaitCap() int {
	return c.Match.WaitCap
}

func (c Config) SynopsisLimit() int {
	return c.Match.SynopsisLimit
}

func (c Config) SeqThreshold() float64 {
	return c.Group.SeqThreshold
}

func (c Config) GroupVideoExt() map[string]struct{} {
	return extSet(c.Group.VideoExt)
}

func (c Config) GroupExtras() map[string]struct{} {
	return wordSet(c.Group.Extras)
}

func (c Config) GroupRelease() map[string]struct{} {
	return wordSet(c.Group.Release)
}

func (c Config) GroupKinds() map[string]struct{} {
	return wordSet(c.Group.Kinds)
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

func (c Config) MatchWorkers() int {
	return c.Match.Workers
}

func (c Config) MatchSoloScore() float64 {
	if c.Match.SoloMinScore <= 0 {
		return c.Match.MinScore
	}
	return c.Match.SoloMinScore
}

func (c Config) MatchMinHits() int {
	return c.Match.MinHits
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

func (c Config) MatchCooldownFails() int {
	return c.Match.CooldownFails
}

func (c Config) MatchCooldown() ExpRange {
	return c.Match.Cooldown
}

func (c Config) HTTPBackoff() ExpRange {
	return c.HTTP.Backoff
}

func JitterExp(exp int) time.Duration {
	if exp < 1 {
		exp = 1
	}
	if exp > 62 {
		exp = 62
	}
	lo := int64(1) << (exp - 1)
	hi := int64(1) << exp
	n := lo
	if hi > lo {
		n = lo + rand.Int64N(hi-lo+1)
	}
	return time.Duration(n) * time.Millisecond
}

func (c Config) HTTPTimeout() time.Duration {
	return time.Duration(c.HTTP.TimeoutMS) * time.Millisecond
}

func (c Config) ProviderTimeout() time.Duration {
	return time.Duration(c.HTTP.ProviderTimeoutMS) * time.Millisecond
}

func SecretKeys(cfg Config) []string {
	seen := map[string]struct{}{}
	extra := make([]string, 0)
	for name, p := range cfg.Providers {
		k := strings.TrimSpace(p.Secret)
		if k == "" {
			if p.Require != "api_key" {
				continue
			}
			k = name
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		extra = append(extra, k)
	}
	sort.Strings(extra)
	return extra
}

func SetSecrets(cfg *Config, updates map[string]string) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	allowed := map[string]struct{}{}
	for _, k := range SecretKeys(*cfg) {
		allowed[k] = struct{}{}
	}
	for k := range updates {
		if _, ok := allowed[k]; !ok {
			return fmt.Errorf("unknown secret key %q", k)
		}
	}
	keys, err := readSecretsMap(cfg.DataDir)
	if err != nil {
		return err
	}
	for k, v := range updates {
		v = strings.TrimSpace(v)
		if v == "" {
			delete(keys, k)
		} else {
			keys[k] = v
		}
	}
	return writeSecretsMap(cfg.DataDir, keys)
}

func SecretsStatus(cfg Config) map[string]bool {
	keys, err := readSecretsMap(cfg.DataDir)
	if err != nil {
		keys = map[string]string{}
	}
	out := make(map[string]bool, len(SecretKeys(cfg)))
	for _, k := range SecretKeys(cfg) {
		out[k] = strings.TrimSpace(keys[k]) != ""
	}
	return out
}

func secretsPath(dataDir string) string {
	return filepath.Join(dataDir, "config", "secrets")
}

func readSecretsMap(dataDir string) (map[string]string, error) {
	b, err := os.ReadFile(secretsPath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return map[string]string{}, nil
	}
	var keys map[string]string
	if err := yaml.Unmarshal(b, &keys); err != nil {
		return nil, err
	}
	if keys == nil {
		keys = map[string]string{}
	}
	return keys, nil
}

func writeSecretsMap(dataDir string, keys map[string]string) error {
	if keys == nil {
		keys = map[string]string{}
	}
	if err := os.MkdirAll(filepath.Dir(secretsPath(dataDir)), 0o700); err != nil {
		return err
	}
	b, err := yaml.Marshal(keys)
	if err != nil {
		return err
	}
	path := secretsPath(dataDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func applySecrets(cfg *Config) {
	for name, p := range cfg.Providers {
		p.APIKey = ""
		cfg.Providers[name] = p
	}
	keys, err := readSecretsMap(cfg.DataDir)
	if err != nil || len(keys) == 0 {
		return
	}
	for name, p := range cfg.Providers {
		keyName := name
		if s := strings.TrimSpace(p.Secret); s != "" {
			keyName = s
		}
		k, ok := keys[keyName]
		if !ok {
			continue
		}
		p.APIKey = strings.TrimSpace(k)
		cfg.Providers[name] = p
	}
}

func overlayPath(dataDir string) string {
	return filepath.Join(dataDir, "config", "overlay.yaml")
}

func ReadOverlay(dataDir string) (map[string]any, error) {
	b, err := os.ReadFile(overlayPath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return jsonMap(m), nil
}

func validateOverlay(cfg Config, overlay map[string]any) error {
	base, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		return err
	}
	over, err := yaml.Marshal(overlay)
	if err != nil {
		return err
	}
	raw, err := merge(base, over)
	if err != nil {
		return err
	}
	decoded, err := decode(raw)
	if err != nil {
		return err
	}
	return Validate(decoded)
}

func Overlay(cfg *Config, patch map[string]any) (map[string]any, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if patch == nil {
		return nil, fmt.Errorf("json object required")
	}
	cur, err := ReadOverlay(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	mergeMap(cur, patch)
	if err := validateOverlay(*cfg, cur); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(overlayPath(cfg.DataDir)), 0o700); err != nil {
		return nil, err
	}
	b, err := yaml.Marshal(cur)
	if err != nil {
		return nil, err
	}
	path := overlayPath(cfg.DataDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}
	return jsonMap(cur), nil
}

func jsonMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = jsonValue(v)
	}
	return out
}

func jsonValue(v any) any {
	if m, ok := asMap(v); ok {
		return jsonMap(m)
	}
	return v
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
			out[stringify(k)] = val
		}
		return out, true
	default:
		return nil, false
	}
}

func stringify(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func decode(b []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
