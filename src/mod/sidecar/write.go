package sidecar

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
	"github.com/alyshmahell/matchmedia/src/mod/fetch"
	"github.com/alyshmahell/matchmedia/src/mod/match"
)

type Stats struct {
	TitleNFO   int
	EpisodeNFO int
	Posters    int
}

func Write(ctx context.Context, cfg *config.Config, jobs []match.Job) (Stats, error) {
	return WriteProgress(ctx, cfg, jobs, nil)
}

func WriteProgress(ctx context.Context, cfg *config.Config, jobs []match.Job, report func(done, total int, job match.Job, st Stats)) (Stats, error) {
	var all Stats
	total := 0
	for _, job := range jobs {
		if job.Status == "matched" && job.Match != nil {
			total++
		}
	}
	done := 0
	for _, job := range jobs {
		if job.Status != "matched" || job.Match == nil {
			continue
		}
		st, err := writeJob(ctx, cfg, job, *job.Match)
		if err != nil {
			return all, err
		}
		all.TitleNFO += st.TitleNFO
		all.EpisodeNFO += st.EpisodeNFO
		all.Posters += st.Posters
		done++
		if report != nil {
			report(done, total, job, all)
		}
	}
	return all, nil
}

func writeJob(ctx context.Context, cfg *config.Config, job match.Job, cand rank.Hit) (Stats, error) {
	var st Stats
	if cand.Provider == "" || cand.ID == "" || strings.TrimSpace(job.Path) == "" {
		return st, nil
	}
	info, err := os.Stat(job.Path)
	if err != nil {
		return st, nil
	}
	uid := ident(cfg, cand)
	show := job.Kind == "show"
	nfoPath, posterDir, posterBase := titleTargets(job, info.IsDir(), show)
	if show {
		if err := writeShowNFO(nfoPath, cand, uid, job.Type); err != nil {
			return st, err
		}
		if info.IsDir() {
			_ = os.Remove(filepath.Join(job.Path, "movie.nfo"))
		}
	} else {
		if err := writeMovieNFO(nfoPath, cand, uid, job.Type); err != nil {
			return st, err
		}
		if info.IsDir() {
			_ = os.Remove(filepath.Join(job.Path, "tvshow.nfo"))
		}
	}
	st.TitleNFO++
	if savePoster(ctx, cfg, cand.Provider, cand.Poster, posterDir, posterBase) {
		st.Posters++
	}
	if !show || job.Catalog == nil || job.CatalogFor != cand.Key() {
		return st, nil
	}
	for _, s := range job.Catalog {
		for _, e := range s.Episodes {
			for _, p := range episodePaths(e) {
				dir := filepath.Dir(p)
				base := trimExt(filepath.Base(p))
				if err := writeEpisodeNFO(filepath.Join(dir, base+".nfo"), uid, s.Number, e); err != nil {
					return st, err
				}
				st.EpisodeNFO++
			}
		}
	}
	return st, nil
}

func titleTargets(job match.Job, dir, show bool) (nfoPath, posterDir, posterBase string) {
	if dir {
		nfo := "movie.nfo"
		if show {
			nfo = "tvshow.nfo"
		}
		return filepath.Join(job.Path, nfo), job.Path, "poster"
	}
	posterDir = filepath.Dir(job.Path)
	base := trimExt(filepath.Base(job.Path))
	return filepath.Join(posterDir, base+".nfo"), posterDir, base + "-poster"
}

func episodePaths(e fetch.Episode) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(e.Path)
	for _, p := range e.Paths {
		add(p)
	}
	return out
}

func ident(cfg *config.Config, cand rank.Hit) string {
	if cand.UniqueID != "" {
		return cand.UniqueID
	}
	if cfg != nil {
		if spec, ok := cfg.Endpoint(cand.Provider, cand.NFO); ok {
			if s := strings.TrimSpace(spec.UniqueID); s != "" {
				return s
			}
		}
	}
	return cand.Provider
}

func savePoster(ctx context.Context, cfg *config.Config, provider, url, dir, base string) bool {
	if strings.TrimSpace(url) == "" || ctx.Err() != nil {
		return false
	}
	if posterExists(dir, base) {
		return false
	}
	body, ctype, err := fetch.Bytes(ctx, cfg, url, provider)
	if err != nil || len(body) == 0 {
		return false
	}
	if err := os.WriteFile(filepath.Join(dir, base+extFromType(ctype, url)), body, 0o644); err != nil {
		return false
	}
	return true
}

func posterExists(dir, base string) bool {
	for _, ext := range []string{".jpg", ".png", ".webp", ".jpeg", ".gif"} {
		if _, err := os.Stat(filepath.Join(dir, base+ext)); err == nil {
			return true
		}
	}
	return false
}

func extFromType(ctype, rawURL string) string {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(ctype, ";")[0]))
	switch ct {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	}
	u := strings.ToLower(strings.TrimSpace(rawURL))
	switch {
	case strings.Contains(u, ".png"):
		return ".png"
	case strings.Contains(u, ".webp"):
		return ".webp"
	case strings.Contains(u, ".gif"):
		return ".gif"
	}
	return ".jpg"
}
