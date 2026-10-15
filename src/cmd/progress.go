package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/alyshmahell/matchmedia/src/mod/match"
)

const (
	barWidth     = 12
	defaultWidth = 80
	minWidth     = 20

	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiCyan   = "\x1b[36m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiDim    = "\x1b[90m"
)

type snapshot struct {
	Phase      string
	Done       int
	Total      int
	Matched    int
	Manual     int
	Unmatched  int
	Error      int
	NFO        int
	EpisodeNFO int
	Posters    int
	Title      string
}

type bar struct {
	mu    sync.Mutex
	w     io.Writer
	tty   bool
	color bool
	width int
	last  int
	phase string
}

func newBar() *bar {
	tty := isTTY(os.Stderr)
	width := defaultWidth
	if c := strings.TrimSpace(os.Getenv("COLUMNS")); c != "" {
		if n, err := strconv.Atoi(c); err == nil && n >= minWidth {
			width = n
		}
	}
	return &bar{
		w:     os.Stderr,
		tty:   tty,
		color: useColor(tty),
		width: width,
	}
}

func useColor(tty bool) bool {
	return tty && os.Getenv("NO_COLOR") == ""
}

func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func (b *bar) draw(s snapshot) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.tty {
		return
	}
	if b.phase != "" && b.phase != s.Phase {
		fmt.Fprint(b.w, "\n")
		b.last = 0
	}
	b.phase = s.Phase
	line := render(s, b.color, b.width)
	n := utf8.RuneCountInString(stripANSI(line))
	pad := ""
	if b.last > n {
		pad = strings.Repeat(" ", b.last-n)
	}
	b.last = n
	fmt.Fprint(b.w, "\r"+line+pad)
}

func (b *bar) finish() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.tty || b.last == 0 {
		return
	}
	fmt.Fprint(b.w, "\n")
	b.last = 0
	b.phase = ""
}

func (b *bar) logo() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.tty || !b.color {
		return
	}
	img, err := decodeLogo()
	if err != nil {
		return
	}
	fmt.Fprint(b.w, renderLogo(img))
}

func render(s snapshot, color bool, width int) string {
	if width < minWidth {
		width = minWidth
	}
	filled := fillCount(s.Done, s.Total, barWidth)
	plainFill := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	extra := ""
	switch s.Phase {
	case "match":
		extra = fmt.Sprintf("  matched:%d  manual:%d  unmatched:%d  error:%d", s.Matched, s.Manual, s.Unmatched, s.Error)
	case "save":
		extra = fmt.Sprintf("  nfo:%d  episode:%d  poster:%d", s.NFO, s.EpisodeNFO, s.Posters)
	}
	head := fmt.Sprintf("%-6s [%s] %d/%d%s", s.Phase, plainFill, s.Done, s.Total, extra)
	title := flattenTitle(s.Title)
	remain := width - utf8.RuneCountInString(head)
	if title != "" {
		remain -= 2
		title = truncateRunes(title, remain)
	} else {
		title = ""
	}
	if !color {
		if title == "" {
			return truncateRunes(head, width)
		}
		return truncateRunes(head+"  "+title, width)
	}
	fill := ansiGreen + strings.Repeat("█", filled) + ansiDim + strings.Repeat("░", barWidth-filled) + ansiReset
	phase := ansiCyan + fmt.Sprintf("%-6s", s.Phase) + ansiReset
	switch s.Phase {
	case "match":
		extra = fmt.Sprintf("  %smatched:%d%s  %smanual:%d%s  unmatched:%d  %serror:%d%s",
			ansiGreen, s.Matched, ansiReset,
			ansiYellow, s.Manual, ansiReset,
			s.Unmatched,
			ansiRed, s.Error, ansiReset)
	case "save":
		extra = fmt.Sprintf("  %snfo:%d%s  %sepisode:%d%s  %sposter:%d%s",
			ansiGreen, s.NFO, ansiReset,
			ansiYellow, s.EpisodeNFO, ansiReset,
			ansiCyan, s.Posters, ansiReset)
	}
	out := fmt.Sprintf("%s [%s] %d/%d%s", phase, fill, s.Done, s.Total, extra)
	if title != "" {
		out += "  " + title
	}
	return out
}

func fillCount(done, total, width int) int {
	if width <= 0 || total <= 0 || done <= 0 {
		return 0
	}
	if done >= total {
		return width
	}
	n := width * done / total
	if n < 1 {
		return 1
	}
	return n
}

func flattenTitle(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			if j := strings.IndexByte(s[i:], 'm'); j >= 0 {
				i += j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func countsFrom(job match.Job, matched, manual, unmatched, errored *int) {
	switch job.Status {
	case "matched":
		*matched++
	case "manual":
		*manual++
	case "unmatched":
		*unmatched++
	case "error":
		*errored++
	}
}

func matchedCount(jobs []match.Job) int {
	n := 0
	for _, j := range jobs {
		if j.Status == "matched" && j.Match != nil {
			n++
		}
	}
	return n
}
