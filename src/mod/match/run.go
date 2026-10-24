package match

import (
	"context"
	"errors"
	"iter"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
	"github.com/alyshmahell/matchmedia/src/mod/fetch"
)

var ErrCandidateNotFound = errors.New("candidate not found")

func Run(ctx context.Context, client *fetch.Client, jobs []Job) iter.Seq[Job] {
	return RunProgress(ctx, client, jobs, nil, nil)
}

func Collect(seq iter.Seq[Job]) []Job {
	if seq == nil {
		return nil
	}
	var out []Job
	for job := range seq {
		out = append(out, job)
	}
	return out
}

func RunProgress(ctx context.Context, client *fetch.Client, jobs []Job, report func(done, total int, job Job), prefetch func(context.Context, Job)) iter.Seq[Job] {
	return func(yield func(Job) bool) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		ch := make(chan Job, len(jobs)+len(parentJobs(jobs)))
		go func() {
			defer close(ch)
			runProgress(ctx, client, jobs, report, prefetch, ch)
		}()
		for job := range ch {
			if !yield(job) {
				cancel()
				for range ch {
				}
				return
			}
		}
	}
}

func runProgress(ctx context.Context, client *fetch.Client, jobs []Job, report func(done, total int, job Job), prefetch func(context.Context, Job), emit chan<- Job) {
	cfg := client.Config()
	n := 1
	if cfg != nil && cfg.Match.Workers > 0 {
		n = cfg.Match.Workers
	}
	batch := append(append([]Job(nil), parentJobs(jobs)...), jobs...)
	keys := make([]string, len(batch))
	childKeys := map[string]struct{}{}
	for _, j := range batch {
		if isChild(j) {
			childKeys[parentGroupKey(strings.TrimSpace(j.Parent), parentDir(j.Path))] = struct{}{}
		}
	}
	left := map[string]int{}
	for i, j := range batch {
		keys[i] = batchGroupKey(j, childKeys, i)
		left[keys[i]]++
	}
	got := make([]Job, len(batch))
	var mu sync.Mutex
	var noted int
	total := len(batch)
	note := func(job Job) {
		if report == nil {
			return
		}
		mu.Lock()
		noted++
		d := noted
		mu.Unlock()
		report(d, total, job)
	}
	finish := func(idx int, job Job) {
		mu.Lock()
		got[idx] = job
		key := keys[idx]
		left[key]--
		if left[key] > 0 {
			mu.Unlock()
			return
		}
		var members []Job
		for i, k := range keys {
			if k == key {
				members = append(members, got[i])
			}
		}
		mu.Unlock()
		for _, folded := range foldUnmatched(members) {
			if prefetch != nil {
				prefetch(ctx, folded)
			}
			select {
			case <-ctx.Done():
				return
			case emit <- folded:
			}
		}
	}
	jobsCh := make(chan int)
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			for i := range jobsCh {
				job := runOne(ctx, cfg, client, batch[i])
				note(job)
				finish(i, job)
			}
		}()
	}
	for i := range batch {
		if batch[i].Status == "matched" || batch[i].Status == "unmatched" {
			note(batch[i])
			finish(i, batch[i])
			continue
		}
		select {
		case <-ctx.Done():
			close(jobsCh)
			wg.Wait()
			return
		case jobsCh <- i:
		}
	}
	close(jobsCh)
	wg.Wait()
}

func batchGroupKey(j Job, childKeys map[string]struct{}, index int) string {
	if isChild(j) {
		return parentGroupKey(strings.TrimSpace(j.Parent), parentDir(j.Path))
	}
	key := parentGroupKey(j.Title, filepath.Clean(j.Path))
	if _, ok := childKeys[key]; ok {
		return key
	}
	return "job\t" + strconv.Itoa(index)
}

func runOne(ctx context.Context, cfg *config.Config, client *fetch.Client, job Job) Job {
	if job.Status == "matched" || job.Status == "unmatched" {
		return job
	}
	if client != nil && len(job.Requests) > 0 {
		if hit, seasons, ok := client.Replay(ctx, job.Query(), job.Requests); ok {
			job.Match = &hit
			job.Status = "matched"
			job.Ranker = "replay"
			job.Catalog = seasons
			job.CatalogFor = hit.Key()
			job.Error = ""
			return MapFilesOntoCatalog(job)
		}
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
	cat, forKey, reqs := client.Catalog(ctx, job.Query(), cand)
	if job.Match != nil && len(reqs) > 0 {
		job.Match.Requests = append(job.Match.Requests, reqs...)
	}
	if cat != nil {
		job.Catalog = cat
		job.CatalogFor = forKey
		return MapFilesOntoCatalog(job)
	}
	return job
}

func Select(ctx context.Context, client *fetch.Client, job Job, provider, id string) (Job, error) {
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
	client.Detail(ctx, job.Query(), job.Match)
	return attachCatalog(client, ctx, job, *job.Match), nil
}
