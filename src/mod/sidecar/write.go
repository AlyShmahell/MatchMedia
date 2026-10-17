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
	_, st, err := WriteProgress(ctx, cfg, "", jobs, nil)
	return st, err
}

// WriteProgress writes sidecars for mode and returns the jobs with the paths it used.
// mode "" keeps an existing poster. rescan replaces it. changes skips a complete sidecar.
// missing skips a title that already has an NFO. nfo writes nothing.
func WriteProgress(ctx context.Context, cfg *config.Config, mode string, jobs []match.Job, report func(done, total int, job match.Job, st Stats)) ([]match.Job, Stats, error) {
	var all Stats
	total := 0
	for _, job := range jobs {
		if allowWrite(job, mode) {
			total++
		}
	}
	done := 0
	out := make([]match.Job, len(jobs))
	copy(out, jobs)
	for i := range out {
		if !allowWrite(out[i], mode) {
			continue
		}
		wrote, st, err := writeJob(ctx, cfg, out[i], *out[i].Match, replacePoster(mode))
		if err != nil {
			return nil, all, err
		}
		out[i] = wrote
		all.TitleNFO += st.TitleNFO
		all.EpisodeNFO += st.EpisodeNFO
		all.Posters += st.Posters
		done++
		if report != nil {
			report(done, total, out[i], all)
		}
	}
	return out, all, nil
}

func allowWrite(job match.Job, mode string) bool {
	if job.Status != "matched" || job.Match == nil {
		return false
	}
	switch strings.TrimSpace(mode) {
	case "nfo", "local":
		return false
	case "changes", "matchmedia-changes":
		if job.NFOPath != "" && job.PosterPath != "" {
			return false
		}
		return !Complete(job, false)
	case "missing":
		return !TitleNFO(job)
	default:
		return true
	}
}

func replacePoster(mode string) bool {
	switch strings.TrimSpace(mode) {
	case "rescan", "matchmedia":
		return true
	default:
		return false
	}
}

func writeJob(ctx context.Context, cfg *config.Config, job match.Job, cand rank.Hit, replace bool) (match.Job, Stats, error) {
	var st Stats
	if cand.Provider == "" || cand.ID == "" || strings.TrimSpace(job.Path) == "" {
		return job, st, nil
	}
	info, err := os.Stat(job.Path)
	if err != nil {
		return job, st, nil
	}
	uid := ident(cfg, cand)
	show := job.Kind == "show"
	nfoPath, posterDir, posterBase := titleTargets(job, info.IsDir(), show)
	if show {
		if err := writeShowNFO(nfoPath, cand, uid, job.Type); err != nil {
			return job, st, err
		}
		if info.IsDir() {
			_ = os.Remove(filepath.Join(job.Path, "movie.nfo"))
		}
	} else {
		if err := writeMovieNFO(nfoPath, cand, uid, job.Type); err != nil {
			return job, st, err
		}
		if info.IsDir() {
			_ = os.Remove(filepath.Join(job.Path, "tvshow.nfo"))
		}
	}
	job.NFOPath = nfoPath
	st.TitleNFO++
	if posterPath, saved := savePoster(ctx, cfg, cand.Provider, cand.Poster, posterDir, posterBase, replace); posterPath != "" {
		job.PosterPath = posterPath
		if saved {
			st.Posters++
		}
	}
	for i := range job.Files {
		if job.Files[i].Path != "" {
			job.Files[i].PosterPath = thumbPath(job.Files[i].Path)
		}
	}
	if !show || job.Catalog == nil || job.CatalogFor != cand.Key() {
		return job, st, nil
	}
	for _, s := range job.Catalog {
		for _, e := range s.Episodes {
			for _, p := range episodePaths(e) {
				dir := filepath.Dir(p)
				base := trimExt(filepath.Base(p))
				nfo := filepath.Join(dir, base+".nfo")
				if err := writeEpisodeNFO(nfo, uid, s.Number, e); err != nil {
					return job, st, err
				}
				stampEpisode(job.Files, p, nfo, thumbPath(p))
				st.EpisodeNFO++
			}
		}
	}
	return job, st, nil
}

func stampEpisode(files []match.JobFile, video, nfo, thumb string) {
	for i := range files {
		if files[i].Path == video {
			files[i].NFOPath = nfo
			files[i].PosterPath = thumb
		}
	}
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

func savePoster(ctx context.Context, cfg *config.Config, provider, url, dir, base string, replace bool) (path string, saved bool) {
	existing := existingPoster(dir, base)
	if strings.TrimSpace(url) == "" || ctx.Err() != nil {
		return existing, false
	}
	if existing != "" && !replace {
		return existing, false
	}
	body, ctype, err := fetch.Bytes(ctx, cfg, url, provider)
	if err != nil || len(body) == 0 {
		return existing, false
	}
	path = filepath.Join(dir, base+extFromType(ctype, url))
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return existing, false
	}
	if existing != "" && existing != path {
		_ = os.Remove(existing)
	}
	return path, true
}

func existingPoster(dir, base string) string {
	for _, ext := range []string{".jpg", ".png", ".webp", ".jpeg", ".gif"} {
		p := filepath.Join(dir, base+ext)
		if fileExists(p) {
			return p
		}
	}
	return ""
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
