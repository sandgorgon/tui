package widget

import (
	"reflect"
	"testing"
)

func TestFuzzyMatchEmptyQueryMatchesEverything(t *testing.T) {
	score, matches, ok := fuzzyMatch("", "anything")
	if !ok || score != 0 || matches != nil {
		t.Errorf("fuzzyMatch(\"\", ...) = (%d, %v, %v), want (0, nil, true)", score, matches, ok)
	}
}

func TestFuzzyMatchSubsequence(t *testing.T) {
	_, matches, ok := fuzzyMatch("gto", "Go to file")
	if !ok {
		t.Fatal("expected \"gto\" to match \"Go to file\" as a subsequence")
	}
	want := []int{0, 3, 4} // 'G', 't', 'o' in "Go to file"
	if !reflect.DeepEqual(matches, want) {
		t.Errorf("matches = %v, want %v", matches, want)
	}
}

func TestFuzzyMatchCaseInsensitive(t *testing.T) {
	if _, _, ok := fuzzyMatch("GTF", "go to file"); !ok {
		t.Error("expected case-insensitive match")
	}
}

func TestFuzzyMatchRejectsOutOfOrderOrMissingRunes(t *testing.T) {
	if _, _, ok := fuzzyMatch("otg", "Go to file"); ok {
		t.Error("\"otg\" is not a subsequence of \"Go to file\" in order, should not match")
	}
	if _, _, ok := fuzzyMatch("xyz", "Go to file"); ok {
		t.Error("\"xyz\" has no letters in \"Go to file\", should not match")
	}
}

func TestFuzzyMatchScoresConsecutiveAndWordStartHigher(t *testing.T) {
	// "gf" as a tight, word-aligned match ("Go File") should outscore
	// the same two letters scattered ("Go to file").
	tight, _, ok1 := fuzzyMatch("gf", "Go File")
	scattered, _, ok2 := fuzzyMatch("gf", "xxGxxxFxx")
	if !ok1 || !ok2 {
		t.Fatal("expected both candidates to match")
	}
	if tight <= scattered {
		t.Errorf("tight/word-aligned score = %d, scattered score = %d; want tight strictly higher", tight, scattered)
	}
}
