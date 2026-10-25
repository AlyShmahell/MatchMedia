package clean

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/config"
)

var (
	hashRe   = regexp.MustCompile(`(?i)[\[(][0-9a-f]{8}[\])]`)
	trailRe  = regexp.MustCompile(`(?i)\s*-\s*([a-z][a-z0-9]*)\s*$`)
	trailSE  = regexp.MustCompile(`(?i)^s\d{1,2}e\d{1,3}v?\d*$`)
	spaceRe  = regexp.MustCompile(`\s+`)
	nonAlnum = regexp.MustCompile(`[^0-9A-Za-z]+`)
	blockRe  = regexp.MustCompile(`\[([^\[\]]*)\]|\(([^()]*)\)`)
)

// Apply removes comparison tags from a folder or file name. The path on disk
// is left unchanged by callers.
func Apply(cfg *config.Config, name string) string {
	tags := tagPhrases(cfg)
	keep := keepBlocks(cfg)
	s := hashRe.ReplaceAllString(name, " ")
	s = blockRe.ReplaceAllStringFunc(s, func(block string) string {
		if strings.HasPrefix(block, "[") {
			return rewriteBracket(block, tags, keep)
		}
		return rewriteParen(block, tags)
	})
	s = dropTrailing(s)
	s = stripTags(s, tags)
	s = separateBlocks(s)
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

func dropTrailing(s string) string {
	m := trailRe.FindStringSubmatch(s)
	if len(m) != 2 || trailSE.MatchString(m[1]) {
		return s
	}
	return trailRe.ReplaceAllString(s, "")
}

func separateBlocks(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i > 0 && (s[i] == '[' || s[i] == '(') && s[i-1] != ' ' {
			b.WriteByte(' ')
		}
		if i > 0 && isAlnum(s[i]) && (s[i-1] == ']' || s[i-1] == ')') {
			b.WriteByte(' ')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isAlnum(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func tagPhrases(cfg *config.Config) map[string]struct{} {
	out := map[string]struct{}{}
	if cfg == nil {
		return out
	}
	add := func(list []string) {
		for _, raw := range list {
			phrase := normPhrase(raw)
			if phrase != "" {
				out[phrase] = struct{}{}
			}
		}
	}
	add(cfg.Clean.Tags.Quality)
	add(cfg.Clean.Tags.Codec)
	add(cfg.Clean.Tags.Source)
	add(cfg.Clean.Tags.Audio)
	add(cfg.Clean.Tags.Subs)
	return out
}

func keepBlocks(cfg *config.Config) map[string]struct{} {
	out := map[string]struct{}{}
	if cfg == nil {
		return out
	}
	for _, raw := range cfg.Clean.Keep {
		block := strings.ToLower(strings.TrimSpace(raw))
		if block != "" {
			out[block] = struct{}{}
		}
	}
	return out
}

func normPhrase(raw string) string {
	return strings.Join(wordsOf(raw), " ")
}

func wordsOf(raw string) []string {
	parts := nonAlnum.Split(strings.ToLower(raw), -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func rewriteBracket(block string, tags, keep map[string]struct{}) string {
	if _, ok := keep[strings.ToLower(block)]; ok {
		return block
	}
	words := wordsOf(block)
	if len(words) == 1 || covers(words, tags) {
		return " "
	}
	return block
}

func rewriteParen(block string, tags map[string]struct{}) string {
	words := wordsOf(block)
	if len(words) == 1 && (year(words[0]) || smallNum(words[0])) {
		return block
	}
	if covers(words, tags) {
		return " "
	}
	return block
}

func year(w string) bool {
	if len(w) != 4 {
		return false
	}
	_, err := strconv.Atoi(w)
	return err == nil
}

func smallNum(w string) bool {
	if len(w) < 1 || len(w) > 2 {
		return false
	}
	_, err := strconv.Atoi(w)
	return err == nil
}

func covers(words []string, tags map[string]struct{}) bool {
	if len(words) == 0 {
		return false
	}
	i := 0
	for i < len(words) {
		matched := false
		for n := len(words) - i; n >= 1; n-- {
			if _, ok := tags[strings.Join(words[i:i+n], " ")]; ok {
				i += n
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func stripTags(s string, tags map[string]struct{}) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '[' || s[i] == '(' {
			end := closeBlock(s, i)
			if end > i {
				b.WriteString(s[i:end])
				i = end
				continue
			}
			b.WriteByte(' ')
			i++
			continue
		}
		j := i
		for j < len(s) && s[j] != '[' && s[j] != '(' {
			j++
		}
		b.WriteString(stripFree(s[i:j], tags))
		i = j
	}
	return b.String()
}

func closeBlock(s string, i int) int {
	open := s[i]
	close := byte(']')
	if open == '(' {
		close = ')'
	}
	for j := i + 1; j < len(s); j++ {
		if s[j] == close {
			return j + 1
		}
		if s[j] == '[' || s[j] == '(' {
			return i
		}
	}
	return i
}

type rawWord struct {
	text string
}

func stripFree(s string, tags map[string]struct{}) string {
	var words []rawWord
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		words = append(words, rawWord{text: cur.String()})
		cur.Reset()
	}
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			cur.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	drop := make([]bool, len(words))
	for i := 0; i < len(words); {
		matched := 0
		for n := len(words) - i; n >= 1; n-- {
			parts := make([]string, n)
			for k := 0; k < n; k++ {
				parts[k] = strings.ToLower(words[i+k].text)
			}
			if _, ok := tags[strings.Join(parts, " ")]; ok {
				matched = n
				break
			}
		}
		if matched == 0 {
			i++
			continue
		}
		for k := 0; k < matched; k++ {
			drop[i+k] = true
		}
		i += matched
	}
	var kept []string
	for i, w := range words {
		if !drop[i] {
			kept = append(kept, w.text)
		}
	}
	return strings.Join(kept, " ")
}
