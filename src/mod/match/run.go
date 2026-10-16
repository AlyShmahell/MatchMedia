package match

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
	"github.com/alyshmahell/matchmedia/src/mod/fetch"
)

var ErrCandidateNotFound = errors.New("candidate not found")

func Run(ctx context.Context, cfg *config.Config, jobs []Job) []Job {
	return RunProgress(ctx, cfg, jobs, nil)
}

func RunProgress(ctx context.Context, cfg *config.Config, jobs []Job, report func(done, total int, job Job)) []Job {
	client := fetch.New(cfg)
	n := cfg.Match.Workers
	if n < 1 {
		n = 1
	}
	parents := parentJobs(jobs)
	batch := append(append([]Job(nil), parents...), jobs...)
	var mu sync.Mutex
	done := 0
	total := len(batch)
	note := func(job Job) {
		if report == nil {
			return
		}
		mu.Lock()
		done++
		d, t := done, total
		mu.Unlock()
		report(d, t, job)
	}
	out := runBatch(ctx, cfg, client, n, batch, note)
	return foldUnmatched(out)
}

func runBatch(ctx context.Context, cfg *config.Config, client *fetch.Client, n int, jobs []Job, note func(Job)) []Job {
	out := make([]Job, len(jobs))
	jobsCh := make(chan int)
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			for i := range jobsCh {
				out[i] = runOne(ctx, cfg, client, jobs[i])
				note(out[i])
			}
		}()
	}
	for i := range jobs {
		if jobs[i].Status == "matched" || jobs[i].Status == "unmatched" {
			out[i] = jobs[i]
			note(out[i])
			continue
		}
		jobsCh <- i
	}
	close(jobsCh)
	wg.Wait()
	return out
}

func runOne(ctx context.Context, cfg *config.Config, client *fetch.Client, job Job) Job {
	if job.Status == "matched" || job.Status == "unmatched" {
		return job
	}
	job.Error = ""
	q := job.Query()
	cands, errs, ok := search(ctx, client, q, job.Type == "")
	if len(cands) == 0 && ok > 0 {
		for _, phrase := range wordNgrams(q.Title) {
			nq := q
			nq.Title = phrase
			nc, _, _ := search(ctx, client, nq, job.Type == "")
			if len(nc) > 0 {
				cands = nc
				break
			}
		}
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
	return finishRank(cfg, client, ctx, job, cands)
}

func search(ctx context.Context, client *fetch.Client, q rank.Query, other bool) ([]Candidate, []string, int) {
	cands, errs, ok := client.Search(ctx, q)
	if len(cands) == 0 && other {
		rest, rerrs, rok := client.SearchOther(ctx, q)
		cands = rest
		errs = append(errs, rerrs...)
		ok += rok
	}
	return cands, errs, ok
}

func finishRank(cfg *config.Config, client *fetch.Client, ctx context.Context, job Job, cands []Candidate) Job {
	ranked := rank.Rank(cfg, job.Query(), cands)
	job.Ranker = "set"
	job.Candidates = ranked
	return applyMatch(cfg, client, ctx, job, ranked)
}

func applyMatch(cfg *config.Config, client *fetch.Client, ctx context.Context, job Job, ranked []Candidate) Job {
	best, ok := rank.Auto(cfg, job.Query(), ranked)
	if !ok {
		job.Status = "manual"
		job.Match = nil
		return job
	}
	job.Match = &best
	job.Status = "matched"
	client.Detail(ctx, job.Query(), job.Match)
	return attachCatalog(client, ctx, job, *job.Match)
}

func attachCatalog(client *fetch.Client, ctx context.Context, job Job, cand Candidate) Job {
	cat, forKey := client.Catalog(ctx, job.Query(), cand)
	if cat != nil {
		job.Catalog = cat
		job.CatalogFor = forKey
		return MapFilesOntoCatalog(job)
	}
	return job
}

func Select(ctx context.Context, cfg *config.Config, job Job, provider, id string) (Job, error) {
	var pick *Candidate
	for i := range job.Candidates {
		if job.Candidates[i].Provider == provider && job.Candidates[i].ID == id {
			pick = &job.Candidates[i]
			break
		}
	}
	if pick == nil {
		return job, ErrCandidateNotFound
	}
	job.Match = pick
	job.Status = "matched"
	job.Error = ""
	client := fetch.New(cfg)
	client.Detail(ctx, job.Query(), job.Match)
	return attachCatalog(client, ctx, job, *job.Match), nil
}
