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

	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/ingest"
	"github.com/alyshmahell/matchmedia/src/mod/match"
	"github.com/alyshmahell/matchmedia/src/mod/sidecar"
	"github.com/alyshmahell/matchmedia/src/mod/scan"
)

const helpSep = "────────────────────────────────"

func printHelp() {
	w := os.Stdout
	color := useColor(isTTY(w))
	hadLogo := printHelpLogo(w)
	printHelpSection(w, color, "Usage", hadLogo)
	fmt.Fprintln(w, "matchmedia [flags]")
	flag.CommandLine.SetOutput(w)
	flag.PrintDefaults()
	printHelpSection(w, color, "License", true)
	printEmbedded(w, licenseText)
	printHelpSection(w, color, "Attribution", true)
	printEmbedded(w, attributionText)
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
	saveOn := flag.Bool("save", false, "write NFO and posters next to the media")
	helpOn := flag.Bool("help", false, "print usage, license, and attribution")
	flag.Usage = printHelp
	flag.Parse()

	if *helpOn {
		printHelp()
		return
	}

	cfg, err := config.New(*overlay)
	if err != nil {
		log.Fatal(err)
	}

	bar := newBar()
	defer bar.finish()
	bar.logo()

	var jobs []match.Job
	switch {
	case *ingestPath != "":
		name := filepath.Base(*ingestPath)
		bar.draw(snapshot{Phase: "scan", Done: 0, Total: 1, Title: name})
		f, err := os.Open(*ingestPath)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		jobs, err = ingest.ParseJobs(cfg, f, name, "")
		if err != nil {
			log.Fatal(err)
		}
		bar.draw(snapshot{Phase: "scan", Done: 1, Total: 1, Title: name})
	case *path != "":
		jobs, err = scan.RunProgress(cfg, *path, *mode, func(done, total int, name string) {
			bar.draw(snapshot{Phase: "scan", Done: done, Total: total, Title: name})
		})
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("set -path or -ingest")
	}

	needMatch := false
	for _, j := range jobs {
		if j.Status == "pending" || j.Status == "error" {
			needMatch = true
			break
		}
	}
	if needMatch {
		var mu sync.Mutex
		var matched, manual, unmatched, errored int
		jobs = match.RunProgress(context.Background(), cfg, jobs, func(done, total int, job match.Job) {
			mu.Lock()
			countsFrom(job, &matched, &manual, &unmatched, &errored)
			s := snapshot{
				Phase:     "match",
				Done:      done,
				Total:     total,
				Matched:   matched,
				Manual:    manual,
				Unmatched: unmatched,
				Error:     errored,
				Title:     job.Title,
			}
			mu.Unlock()
			bar.draw(s)
		})
	}
	if *saveOn {
		nSave := matchedCount(jobs)
		bar.draw(snapshot{Phase: "save", Done: 0, Total: nSave})
		if _, err := sidecar.WriteProgress(context.Background(), cfg, jobs, func(done, total int, job match.Job, st sidecar.Stats) {
			bar.draw(snapshot{Phase: "save", Done: done, Total: total, NFO: st.TitleNFO, EpisodeNFO: st.EpisodeNFO, Posters: st.Posters, Title: job.Title})
		}); err != nil {
			log.Fatal(err)
		}
	}
	bar.draw(snapshot{Phase: "report", Done: 0, Total: 1, Title: *report})
	if err := writeReport(*report, jobs); err != nil {
		log.Fatal(err)
	}
	bar.draw(snapshot{Phase: "report", Done: 1, Total: 1, Title: *report})
	bar.finish()
	fmt.Printf("wrote %s\n", *report)
}
