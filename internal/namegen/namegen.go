package namegen

import (
	_ "embed"
	"math/rand"
	"strings"
)

//go:embed adjectives.txt
var adjectivesRaw string

//go:embed nouns.txt
var nounsRaw string

var adjectives, nouns []string

func init() {
	adjectives = parseWords(adjectivesRaw)
	nouns = parseWords(nounsRaw)
}

func Generate() string {
	return adjectives[rand.Intn(len(adjectives))] + "-" + nouns[rand.Intn(len(nouns))]
}

func parseWords(s string) []string {
	var words []string
	for _, line := range strings.Split(s, "\n") {
		if w := strings.TrimSpace(line); w != "" {
			words = append(words, w)
		}
	}
	return words
}
