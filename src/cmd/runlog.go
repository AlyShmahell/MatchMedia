package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alyshmahell/matchmedia/src/mod/match"
)

type runLog struct {
	mu      sync.Mutex
	f       *os.File
	needGap bool
}

func openRunLog(path string) (*runLog, error) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &runLog{f: f}, nil
}

func (l *runLog) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

func (l *runLog) header(title string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	if l.needGap {
		fmt.Fprintln(l.f)
	}
	fmt.Fprintln(l.f, title)
	fmt.Fprintln(l.f, helpSep)
	l.needGap = false
}

func (l *runLog) lines(lines ...string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	for _, s := range lines {
		fmt.Fprintln(l.f, s)
	}
	if len(lines) > 0 {
		l.needGap = true
	}
}

func (l *runLog) errorJob(job match.Job) {
	if l == nil || job.Status != "error" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	if l.needGap {
		fmt.Fprintln(l.f)
	}
	title := strings.TrimSpace(strings.ReplaceAll(job.Title, "\n", " "))
	if title == "" {
		title = "(untitled)"
	}
	fmt.Fprintln(l.f, title)
	fmt.Fprintf(l.f, "path: %s\n", job.Path)
	fmt.Fprintf(l.f, "error: %s\n", job.Error)
	l.needGap = true
}

func (l *runLog) fatal(v ...any) {
	msg := fmt.Sprint(v...)
	if l != nil {
		l.header("fatal")
		l.lines(msg)
		_ = l.Close()
	}
	log.Fatal(msg)
}

func itemCount(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}
