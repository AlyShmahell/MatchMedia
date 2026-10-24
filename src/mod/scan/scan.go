package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/group"
	"github.com/alyshmahell/matchmedia/src/mod/match"
	"github.com/alyshmahell/matchmedia/src/mod/sidecar"
)

const (
	ModeRescan  = "rescan"
	ModeChanges = "changes"
	ModeMissing = "missing"
	ModeNFO     = "nfo"
)

var (
	yearRe       = regexp.MustCompile(`\((\d{4})\)`)
	yearSuffixRe = regexp.MustCompile(`\s*\((\d{4})\)\s*$`)
	episodeRe    = regexp.MustCompile(`(?i)s(\d{1,2})e(\d{1,3})`)
)

func NormalizeMode(mode string) (string, error) {
	switch mode {
	case "", ModeRescan:
		return ModeRescan, nil
	case ModeChanges, ModeMissing, ModeNFO:
		return mode, nil
	default:
		return "", fmt.Errorf("unknown mode")
	}
}

func Run(cfg *config.Config, path, mode string) ([]match.Job, error) {
	return RunProgress(cfg, path, mode, nil)
}

func RunProgress(cfg *config.Config, path, mode string, report func(done, total int, name string)) ([]match.Job, error) {
	mode, err := NormalizeMode(mode)
	if err != nil {
		return nil, err
	}
	path = filepath.Clean(strings.TrimSpace(path))
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", path)
	}
	ents, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	lib := group.New(cfg)
	var jobs []match.Job
	total := len(ents)
	for i, e := range ents {
		child := filepath.Join(path, e.Name())
		if e.IsDir() {
			titles, err := lib.Group(child)
			if err != nil {
				return nil, err
			}
			jobs = append(jobs, jobsFromTitles(cfg, child, titles)...)
		} else if _, ok := cfg.GroupVideoExt()[strings.ToLower(filepath.Ext(e.Name()))]; ok {
			jobs = append(jobs, fileJob(path, e.Name()))
		}
		if report != nil {
			report(i+1, total, e.Name())
		}
	}
	return applyMode(jobs, mode), nil
}

func jobsFromTitles(cfg *config.Config, child string, titles []group.Title) []match.Job {
	parent, parentYear := splitTitleYear(filepath.Base(child))
	out := make([]match.Job, 0, len(titles))
	season := cfg.Season()
	for _, t := range titles {
		title, year := splitTitleYear(t.Name)
		if year == "" {
			year = parentYear
		}
		job := match.Job{
			Source: "scan",
			Title:  title,
			Year:   year,
			Parent: parent,
			Role:   "title",
			Status: "pending",
			Kind:   kindOf(season, t),
		}
		job.Files = make([]match.JobFile, 0, len(t.Files))
		for _, rel := range t.Files {
			abs := filepath.Join(child, rel)
			se, ep := inferSE(rel)
			job.Files = append(job.Files, match.JobFile{Path: abs, Season: se, Episode: ep})
		}
		job.Path = jobPath(child, t)
		out = append(out, job)
	}
	return out
}

func fileJob(root, name string) match.Job {
	abs := filepath.Join(root, name)
	title, year := splitTitleYear(strings.TrimSuffix(name, filepath.Ext(name)))
	se, ep := inferSE(name)
	kind := "movie"
	if se != "" {
		kind = "show"
	}
	return match.Job{
		Source: "scan",
		Title:  title,
		Year:   year,
		Parent: filepath.Base(root),
		Path:   abs,
		Kind:   kind,
		Role:   "title",
		Status: "pending",
		Files:  []match.JobFile{{Path: abs, Season: se, Episode: ep}},
	}
}

func kindOf(season *regexp.Regexp, t group.Title) string {
	for _, rel := range t.Files {
		for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
			if season.MatchString(part) {
				return "show"
			}
		}
		if episodeRe.MatchString(rel) {
			return "show"
		}
	}
	return "movie"
}

func jobPath(child string, t group.Title) string {
	if len(t.Files) == 0 {
		return child
	}
	if t.Name == filepath.Base(child) {
		return child
	}
	first := filepath.ToSlash(t.Files[0])
	if i := strings.Index(first, "/"); i > 0 {
		dir := first[:i]
		if dir == t.Name {
			return filepath.Join(child, dir)
		}
	}
	if !strings.Contains(first, "/") {
		return filepath.Join(child, t.Files[0])
	}
	return filepath.Join(child, filepath.Dir(t.Files[0]))
}

func splitTitleYear(name string) (title, year string) {
	title = strings.TrimSpace(name)
	if m := yearSuffixRe.FindStringSubmatch(title); len(m) == 2 {
		return strings.TrimSpace(yearSuffixRe.ReplaceAllString(title, "")), m[1]
	}
	return title, yearOf(title)
}

func yearOf(name string) string {
	m := yearRe.FindStringSubmatch(name)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func inferSE(rel string) (season, episode string) {
	m := episodeRe.FindStringSubmatch(filepath.Base(rel))
	if len(m) == 3 {
		return atoi(m[1]), atoi(m[2])
	}
	return "", ""
}

func atoi(s string) string {
	n, err := strconv.Atoi(s)
	if err != nil {
		return s
	}
	return strconv.Itoa(n)
}

func applyMode(jobs []match.Job, mode string) []match.Job {
	out := make([]match.Job, 0, len(jobs))
	for _, job := range jobs {
		if len(job.Files) == 0 {
			continue
		}
		switch mode {
		case ModeRescan:
			out = append(out, job)
		case ModeChanges:
			if sidecar.Complete(job, true) {
				out = append(out, sidecar.FromSidecar(job))
				continue
			}
			if sidecar.Complete(job, false) {
				job = sidecar.Fill(job)
				job.Requests = sidecar.SavedRequests(job)
				out = append(out, job)
				continue
			}
			out = append(out, job)
		case ModeMissing:
			if sidecar.TitleNFO(job) {
				continue
			}
			out = append(out, job)
		case ModeNFO:
			job = sidecar.Fill(job)
			if sidecar.Complete(job, false) {
				out = append(out, sidecar.FromSidecar(job))
			} else {
				job.Status = "unmatched"
				job.Match = nil
				job.Candidates = nil
				job.Ranker = ""
				out = append(out, job)
			}
		}
	}
	return out
}
