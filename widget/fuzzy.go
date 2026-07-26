package widget

import "strings"

// fuzzyMatch reports whether every rune of query appears, in order and
// case-insensitively, as a subsequence of candidate — the fzf-style
// "type any of the letters, in order, anywhere" match — and if so
// returns a score (higher is a better match) and the indices in
// candidate that matched, for highlighting. Scoring rewards
// consecutive runs and matches at the start of a word, the same
// general shape as fzf's default heuristic ("prefer tight, word-
// aligned matches over scattered ones"), not a reimplementation of its
// exact algorithm.
func fuzzyMatch(query, candidate string) (score int, matches []int, ok bool) {
	if query == "" {
		return 0, nil, true
	}
	q := []rune(strings.ToLower(query))
	c := []rune(candidate)
	cl := []rune(strings.ToLower(candidate))

	matches = make([]int, 0, len(q))
	qi := 0
	prevMatched := -2
	for ci := 0; ci < len(cl) && qi < len(q); ci++ {
		if cl[ci] != q[qi] {
			continue
		}
		gain := 1
		if ci == prevMatched+1 {
			gain += 3 // consecutive-match bonus
		}
		if ci == 0 || isWordBoundary(c[ci-1]) {
			gain += 2 // start-of-word bonus
		}
		score += gain
		matches = append(matches, ci)
		prevMatched = ci
		qi++
	}
	if qi < len(q) {
		return 0, nil, false
	}
	return score, matches, true
}

func isWordBoundary(r rune) bool {
	switch r {
	case ' ', '-', '_', '/', '.':
		return true
	default:
		return false
	}
}
