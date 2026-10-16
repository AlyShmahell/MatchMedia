package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/match"
)

func writeReport(path string, jobs []match.Job) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(renderReport(jobs)), 0o644)
}

func renderReport(jobs []match.Job) string {
	matched, manual, unmatched, errored := tally(jobs)

	var b strings.Builder
	b.WriteString("# MatchMedia\n\n")
	fmt.Fprintf(&b, "jobs: %d · %s: %d · %s: %d · %s: %d · %s: %d\n\n",
		len(jobs),
		statusLabel("matched"), matched,
		statusLabel("manual"), manual,
		statusLabel("unmatched"), unmatched,
		statusLabel("error"), errored)

	b.WriteString("## Summary\n\n")
	b.WriteString("| Title | Year | Kind | Files | Status | Match | Score | Link |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, j := range jobs {
		b.WriteString(summaryRow(j))
		b.WriteByte('\n')
	}
	for _, j := range jobs {
		b.WriteByte('\n')
		b.WriteString(jobSection(j))
	}
	return b.String()
}

func summaryRow(j match.Job) string {
	matchTitle, score, link := "", "", ""
	if j.Match != nil {
		matchTitle = j.Match.Title
		score = fmt.Sprintf("%.3f", j.Match.Score)
		link = j.Match.URL
	}
	return "| " + strings.Join([]string{
		mdCell(j.Title),
		mdCell(j.Year),
		mdCell(j.Kind),
		strconv.Itoa(len(j.Files)),
		mdCell(statusLabel(j.Status)),
		mdCell(matchTitle),
		score,
		mdCell(link),
	}, " | ") + " |"
}

func jobSection(j match.Job) string {
	var b strings.Builder
	title := strings.TrimSpace(strings.ReplaceAll(j.Title, "\n", " "))
	if title == "" {
		title = "(untitled)"
	}
	fmt.Fprintf(&b, "## %s\n\n", title)
	fmt.Fprintf(&b, "path: %s\n\n", j.Path)
	fmt.Fprintf(&b, "kind: %s\n\n", j.Kind)
	fmt.Fprintf(&b, "status: %s\n\n", statusLabel(j.Status))
	if j.Error != "" {
		fmt.Fprintf(&b, "error: %s\n\n", j.Error)
	}
	if len(j.Candidates) == 0 {
		b.WriteString("no candidates\n")
		return b.String()
	}
	b.WriteString("| # | Chosen | Title | Year | Score | Jaccard | Provider | Link |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for i, c := range j.Candidates {
		chosen := ""
		if j.Match != nil && c.Provider == j.Match.Provider && c.ID == j.Match.ID {
			chosen = "yes"
		}
		b.WriteString("| " + strings.Join([]string{
			strconv.Itoa(i + 1),
			chosen,
			mdCell(c.Title),
			mdCell(c.Year),
			fmt.Sprintf("%.3f", c.Score),
			fmt.Sprintf("%.3f", c.Jaccard),
			mdCell(c.Provider),
			mdCell(c.URL),
		}, " | ") + " |\n")
	}
	return b.String()
}

func statusLabel(s string) string {
	switch s {
	case "matched":
		return "✅ matched"
	case "manual":
		return "✋ manual"
	case "unmatched":
		return "❌ unmatched"
	case "error":
		return "💥 error"
	default:
		return s
	}
}

func mdCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "`", "\\`")
	return s
}
