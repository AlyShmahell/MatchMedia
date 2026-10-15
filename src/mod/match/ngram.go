package match

import (
	"regexp"
	"strings"

	"github.com/alyshmahell/matchmedia/src/mod/engine/rank"
)

const ngramMin = 3

var possessiveRe = regexp.MustCompile(`(?i)'s\b`)

func wordNgrams(title string) []string {
	toks := strings.Fields(rank.TitleNorm(possessiveRe.ReplaceAllString(title, "")))
	if len(toks) < ngramMin+1 {
		return nil
	}
	var out []string
	for n := len(toks) - 1; n >= ngramMin; n-- {
		for i := 0; i+n <= len(toks); i++ {
			out = append(out, strings.Join(toks[i:i+n], " "))
		}
	}
	return out
}
