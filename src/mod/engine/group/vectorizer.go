package group

import (
	"math"
	"regexp"
	"slices"
	"strings"
)

const (
	ngramMin = 3
	ngramMax = 5
)

var wsCollapse = regexp.MustCompile(`\s{2,}`)

type Vectorizer struct {
	index map[string]int
	idf   []float64
}

func NewVectorizer() *Vectorizer {
	return &Vectorizer{}
}

func (v *Vectorizer) FitTransform(docs []string) [][]float64 {
	counts := parallelMap(len(docs), func(i int) map[string]int {
		return charWBCounts(docs[i])
	})
	v.fit(counts)
	return v.rows(counts)
}

func (v *Vectorizer) Transform(docs []string) [][]float64 {
	counts := parallelMap(len(docs), func(i int) map[string]int {
		return charWBCounts(docs[i])
	})
	return v.rows(counts)
}

func (v *Vectorizer) fit(counts []map[string]int) {
	df := make(map[string]int)
	for _, c := range counts {
		for term := range c {
			df[term]++
		}
	}
	vocab := make([]string, 0, len(df))
	for term := range df {
		vocab = append(vocab, term)
	}
	slices.Sort(vocab)
	v.index = make(map[string]int, len(vocab))
	v.idf = make([]float64, len(vocab))
	n := float64(len(counts))
	for i, term := range vocab {
		v.index[term] = i
		v.idf[i] = math.Log((1+n)/(1+float64(df[term]))) + 1
	}
}

func (v *Vectorizer) rows(counts []map[string]int) [][]float64 {
	dim := len(v.idf)
	return parallelMap(len(counts), func(i int) []float64 {
		row := make([]float64, dim)
		for term, tf := range counts[i] {
			j, ok := v.index[term]
			if !ok {
				continue
			}
			row[j] = float64(tf) * v.idf[j]
		}
		l2normalize(row)
		return row
	})
}

func charWBCounts(text string) map[string]int {
	text = strings.ToLower(text)
	text = wsCollapse.ReplaceAllString(text, " ")
	counts := make(map[string]int)
	for _, word := range strings.Fields(text) {
		padded := []rune(" " + word + " ")
		for n := ngramMin; n <= ngramMax; n++ {
			if n > len(padded) {
				continue
			}
			limit := len(padded) - n + 1
			for off := 0; off < limit; off++ {
				counts[string(padded[off:off+n])]++
			}
		}
	}
	return counts
}

func l2normalize(row []float64) {
	var sum float64
	for _, x := range row {
		sum += x * x
	}
	if sum == 0 {
		return
	}
	inv := 1 / math.Sqrt(sum)
	for i := range row {
		row[i] *= inv
	}
}
