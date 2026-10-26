package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/fetch"
	"github.com/alyshmahell/matchmedia/src/mod/ingest"
	"github.com/alyshmahell/matchmedia/src/mod/match"
	"github.com/alyshmahell/matchmedia/src/mod/scan"
	"github.com/alyshmahell/matchmedia/src/mod/sidecar"
)

const helpSep = "────────────────────────────────"

func printHelp() {
	w := os.Stdout
	color := useColor(isTTY(w))
	printHelpTitle(w, color)
	hadLogo := printHelpLogo(w)
	printHelpSection(w, color, "Usage", hadLogo)
	fmt.Fprintln(w, "matchmedia [flags]")
	flag.CommandLine.SetOutput(w)
	flag.PrintDefaults()
	printHelpSection(w, color, "Support MatchMedia", true)
	fmt.Fprintln(w, "https://ko-fi.com/alyshmahell")
	printHelpKofi(w)
	printHelpSection(w, color, "License", true)
	printEmbedded(w, licenseText)
	printHelpSection(w, color, "Attribution", true)
	printEmbedded(w, attributionText)
}

func printHelpTitle(w *os.File, color bool) {
	if color {
		fmt.Fprintf(w, "%s%sMatchMedia%s\n", ansiBold, ansiGreen, ansiReset)
		fmt.Fprintf(w, "%s%s%s%s\n", ansiBold, ansiGreen, helpSep, ansiReset)
		return
	}
	fmt.Fprintln(w, "MatchMedia")
	fmt.Fprintln(w, helpSep)
}

func printHelpSection(w *os.File, color bool, title string, sep bool) {
	if sep {
		fmt.Fprintln(w)
	}
	if color {
		fmt.Fprintf(w, "%s%s%s\n", ansiBold, title, ansiReset)
	} else {
		fmt.Fprintln(w, title)
	}
	fmt.Fprintln(w, helpSep)
}

func printHelpLogo(w *os.File) bool {
	if !useColor(isTTY(w)) {
		return false
	}
	img, err := decodeLogo()
	if err != nil {
		return false
	}
	fmt.Fprint(w, renderLogo(img))
	return true
}

func printHelpKofi(w *os.File) {
	if !useColor(isTTY(w)) {
		return
	}
	img, err := decodeKofi()
	if err != nil {
		return
	}
	fmt.Fprint(w, renderLogo(img))
}

func printEmbedded(w *os.File, s string) {
	fmt.Fprint(w, s)
	if !strings.HasSuffix(s, "\n") {
		fmt.Fprintln(w)
	}
}

func main() {
	overlay := flag.String("overlay", "", "path to overlay YAML")
	path := flag.String("path", "", "library directory to scan")
	mode := flag.String("mode", scan.ModeRescan, "rescan | changes | missing | nfo")
	ingestPath := flag.String("ingest", "", "CSV or JSON file instead of a scan")
	report := flag.String("report", "match.md", "markdown report path")
	logPath := flag.String("log", "matchmedia.log", "run log path")
	saveOn := flag.Bool("save", false, "write NFO and posters next to the media")
	helpOn := flag.Bool("help", false, "print usage, license, and attribution")
	flag.Usage = printHelp
	flag.Parse()

	if *helpOn {
		printHelp()
		return
	}

	rl, err := openRunLog(*logPath)
	if err != nil {
		log.Fatal(err)
	}
	defer rl.Close()
	fetch.SetRequestLog(func(method, rawURL string) {
		rl.lines(time.Now().Format("15:04:05.000") + " " + method + " " + rawURL)
	})

	cfg, err := config.New(*overlay)
	if err != nil {
		rl.fatal(err)
	}
	client := fetch.New(cfg)

	bar := newBar()
	defer bar.finish()
	bar.logo()

	rl.header("MatchMedia")
	switch {
	case *ingestPath != "":
		rl.lines("ingest: "+*ingestPath, "mode: "+*mode)
	case *path != "":
		rl.lines("path: "+*path, "mode: "+*mode)
	default:
		rl.lines("mode: " + *mode)
		rl.fatal("set -path or -ingest")
	}

	var jobs []match.Job
	switch {
	case *ingestPath != "":
		name := filepath.Base(*ingestPath)
		bar.draw(snapshot{Phase: "scan", Done: 0, Total: 1, Title: name})
		f, err := os.Open(*ingestPath)
		if err != nil {
			rl.fatal(err)
		}
		defer f.Close()
		jobs, err = ingest.ParseJobs(cfg, f, name, "")
		if err != nil {
			rl.fatal(err)
		}
		bar.draw(snapshot{Phase: "scan", Done: 1, Total: 1, Title: name})
	default:
		jobs, err = scan.RunProgress(cfg, *path, *mode, func(done, total int, name string) {
			bar.draw(snapshot{Phase: "scan", Done: done, Total: total, Title: name})
		})
		if err != nil {
			rl.fatal(err)
		}
	}
	rl.header("scan")
	rl.lines(itemCount(len(jobs)))

	needMatch := false
	for _, j := range jobs {
		if j.Status == "pending" || j.Status == "error" {
			needMatch = true
			break
		}
	}
	if needMatch {
		rl.header("match")
		rl.lines(queueLines(match.Batch(jobs))...)
		var mu sync.Mutex
		var matched, sidecarN, replayN, manual, unmatched, errored int
		var lastDone, lastTotal int
		var prefetch func(context.Context, match.Job)
		if *saveOn {
			prefetch = func(ctx context.Context, job match.Job) {
				sidecar.Prefetch(ctx, client, job, *mode)
			}
		}
		var collected []match.Job
		for job := range match.RunProgress(context.Background(), client, jobs, func(done, total int, job match.Job) {
			mu.Lock()
			liveCountsFrom(job, &matched, &sidecarN, &replayN, &manual, &unmatched, &errored)
			lastDone, lastTotal = done, total
			s := snapshot{
				Phase:     "match",
				Done:      done,
				Total:     total,
				Matched:   matched,
				Sidecar:   sidecarN,
				Replay:    replayN,
				Manual:    manual,
				Unmatched: unmatched,
				Error:     errored,
				Title:     job.Title,
			}
			mu.Unlock()
			bar.draw(s)
			rl.errorJob(job)
		}, prefetch) {
			collected = append(collected, job)
		}
		jobs = collected
		folded := 0
		if lastTotal > len(jobs) {
			folded = lastTotal - len(jobs)
		}
		bar.draw(snapshot{
			Phase:     "match",
			Done:      lastDone,
			Total:     lastTotal,
			Matched:   matched,
			Sidecar:   sidecarN,
			Replay:    replayN,
			Manual:    manual,
			Unmatched: unmatched,
			Error:     errored,
			Folded:    folded,
		})
	}
	if *saveOn {
		nSave := matchedCount(jobs)
		bar.draw(snapshot{Phase: "save", Done: 0, Total: nSave})
		if _, _, err := sidecar.WriteProgress(context.Background(), client, *mode, jobs, func(done, total int, job match.Job, st sidecar.Stats) {
			bar.draw(snapshot{Phase: "save", Done: done, Total: total, NFO: st.TitleNFO, EpisodeNFO: st.EpisodeNFO, Posters: st.Posters, Title: job.Title})
		}); err != nil {
			rl.fatal(err)
		}
	}
	bar.draw(snapshot{Phase: "report", Done: 0, Total: 1, Title: *report})
	if err := writeReport(*report, jobs); err != nil {
		rl.fatal(err)
	}
	rl.header("report")
	rl.lines("wrote " + *report)
	bar.draw(snapshot{Phase: "report", Done: 1, Total: 1, Title: *report})
	bar.finish()
	fmt.Printf("wrote %s\n", *report)
}
