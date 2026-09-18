package match

import (
	"context"
	"errors"
	"strings"

	"github.com/alyshmahell/matchmedia/lib/config"
)

var errCandidateNotFound = errors.New("candidate not found")

func Run(ctx context.Context, cfg config.Config, jobs []Job) []Job {
	return RunWith(ctx, cfg, jobs, nil)
}

func RunWith(ctx context.Context, cfg config.Config, jobs []Job, rep Reporter) []Job {
	ctx = WithReporter(ctx, rep)
	httpc := newHTTP(cfg)
	out := make([]Job, len(jobs))
	for i, job := range jobs {
		out[i] = runOne(ctx, cfg, httpc, job)
	}
	return out
}

func runOne(ctx context.Context, cfg config.Config, httpc *httpClient, job Job) Job {
	job.Error = ""
	fast, ferrs, fok := collectProviders(ctx, cfg, httpc, job, true, false)
	if job, done := rankPass(ctx, cfg, httpc, job, fast); done {
		return job
	}
	slow, serrs, sok := collectProviders(ctx, cfg, httpc, job, true, true)
	cands := append(append([]Candidate(nil), fast...), slow...)
	errs := append(append([]string(nil), ferrs...), serrs...)
	ok := fok + sok
	if len(cands) == 0 && job.Type == "" {
		restFast, rferrs, rfok := collectProviders(ctx, cfg, httpc, job, false, false)
		if job, done := rankPass(ctx, cfg, httpc, job, restFast); done {
			return job
		}
		restSlow, rserrs, rsok := collectProviders(ctx, cfg, httpc, job, false, true)
		cands = append(append([]Candidate(nil), restFast...), restSlow...)
		errs = append(append(errs, rferrs...), rserrs...)
		ok += rfok + rsok
	}
	if len(cands) == 0 && len(errs) > 0 && ok == 0 {
		job.Status = "error"
		job.Error = strings.Join(errs, "; ")
		return job
	}
	if len(cands) == 0 {
		job.Status = "unmatched"
		job.Candidates = []Candidate{}
		job.Match = nil
		return job
	}
	job.Error = ""
	return finishRank(ctx, cfg, httpc, job, cands)
}

func rankPass(ctx context.Context, cfg config.Config, httpc *httpClient, job Job, cands []Candidate) (Job, bool) {
	if len(cands) == 0 {
		return job, false
	}
	done := waitStart(ctx, job, "set")
	ranked := rank(cfg, job, cands)
	done(nil)
	job.Ranker = "set"
	job.Candidates = ranked
	if !skipDefer(cfg, ranked) {
		job.Status = "manual"
		job.Match = nil
		return job, false
	}
	return applyMatch(ctx, cfg, httpc, job, ranked), true
}

func skipDefer(cfg config.Config, ranked []Candidate) bool {
	if len(ranked) < cfg.MatchMinHits() {
		return false
	}
	return ranked[0].Jaccard >= cfg.Match.MinScore
}

func finishRank(ctx context.Context, cfg config.Config, httpc *httpClient, job Job, cands []Candidate) Job {
	done := waitStart(ctx, job, "set")
	ranked := rank(cfg, job, cands)
	done(nil)
	job.Ranker = "set"
	job.Candidates = ranked
	return applyMatch(ctx, cfg, httpc, job, ranked)
}

func applyMatch(ctx context.Context, cfg config.Config, httpc *httpClient, job Job, ranked []Candidate) Job {
	best, ok := autoMatch(cfg, job, ranked, preferWant(cfg, job.Type))
	if !ok {
		job.Status = "manual"
		job.Match = nil
		return job
	}
	job.Match = &best
	job.Status = "matched"
	if sub := fetchEpisode(ctx, cfg, httpc, job, best); sub != nil {
		job.Sub = sub
	}
	fetchDetail(ctx, cfg, httpc, job, job.Match)
	return attachCatalog(ctx, cfg, httpc, job, *job.Match)
}

func matchStrength(c Candidate) float64 {
	if c.Score > c.Jaccard {
		return c.Score
	}
	return c.Jaccard
}

func autoMatch(cfg config.Config, job Job, ranked []Candidate, want map[string]string) (Candidate, bool) {
	if len(ranked) == 0 {
		return Candidate{}, false
	}
	best := ranked[0]
	if matchStrength(best) >= cfg.Match.MinScore {
		if len(ranked) < 2 || matchStrength(best)-matchStrength(ranked[1]) >= cfg.Match.MinMargin {
			return best, true
		}
	}
	if len(ranked) > 1 && best.Score >= cfg.Match.MinScore && best.Score-ranked[1].Score >= cfg.Match.MinMargin {
		return best, true
	}
	if uniqueQueryCov(cfg, job, ranked) {
		return best, true
	}
	pref := preferSubset(want, ranked)
	if len(pref) < 2 || pref[0].QueryCov < 1 {
		return Candidate{}, false
	}
	if pref[0].Score-pref[1].Score < cfg.Match.MinMargin {
		return Candidate{}, false
	}
	return pref[0], true
}

func uniqueQueryCov(cfg config.Config, job Job, ranked []Candidate) bool {
	if len(ranked) == 0 || ranked[0].QueryCov < 1 {
		return false
	}
	if len(plotQuery(job.Title, cfg.PlotStop())) == 0 {
		return false
	}
	seen := map[string]struct{}{}
	for _, c := range ranked {
		if c.QueryCov != 1 {
			continue
		}
		seen[coveringWorkKey(job.Title, c)] = struct{}{}
	}
	return len(seen) == 1
}

func preferWant(cfg config.Config, jobType string) map[string]string {
	return cfg.Match.Prefer[jobType]
}

func preferCandidates(cfg config.Config, jobType string, cands []Candidate) []Candidate {
	kept := preferSubset(preferWant(cfg, jobType), cands)
	if len(kept) == 0 {
		return cands
	}
	return kept
}

func preferSubset(want map[string]string, cands []Candidate) []Candidate {
	if len(want) == 0 {
		return nil
	}
	var keep []Candidate
	for _, c := range cands {
		if preferMatch(c, want) {
			keep = append(keep, c)
		}
	}
	return keep
}

func preferMatch(c Candidate, want map[string]string) bool {
	for k, v := range want {
		got := ""
		if c.Attrs != nil {
			got = c.Attrs[k]
		}
		if !strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(v)) {
			return false
		}
	}
	return true
}

func ApplySelect(ctx context.Context, cfg config.Config, job Job, provider, id string) (Job, error) {
	var pick *Candidate
	for i := range job.Candidates {
		if job.Candidates[i].Provider == provider && job.Candidates[i].ID == id {
			pick = &job.Candidates[i]
			break
		}
	}
	if pick == nil {
		return job, errCandidateNotFound
	}
	job.Match = pick
	job.Status = "matched"
	job.Error = ""
	httpc := newHTTP(cfg)
	if sub := fetchEpisode(ctx, cfg, httpc, job, *pick); sub != nil {
		job.Sub = sub
	}
	fetchDetail(ctx, cfg, httpc, job, job.Match)
	return attachCatalog(ctx, cfg, httpc, job, *job.Match), nil
}
