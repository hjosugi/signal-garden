package search

import (
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/hjosugi/signal-garden/internal/model"
)

type Query struct {
	Text        string
	Tag         string
	Source      string
	Limit       int
	QueryVector []float64
}

type Result struct {
	Item          model.Item `json:"item"`
	Score         float64    `json:"score"`
	LexicalScore  float64    `json:"lexical_score"`
	SemanticScore float64    `json:"semantic_score"`
	RecencyScore  float64    `json:"recency_score"`
}

type Facets struct {
	Tags    []Facet `json:"tags"`
	Sources []Facet `json:"sources"`
}

type Facet struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func Run(items []model.Item, query Query, now time.Time) []Result {
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 50
	}
	filtered := filter(items, query.Tag, query.Source)
	if len(filtered) == 0 {
		return nil
	}

	qTokens := unique(tokenize(query.Text))
	docs := make([]document, len(filtered))
	totalLength := 0
	for i, item := range filtered {
		docs[i] = buildDocument(item)
		totalLength += docs[i].length
	}
	avgLength := float64(totalLength) / float64(len(docs))
	if avgLength == 0 {
		avgLength = 1
	}

	df := map[string]int{}
	for _, token := range qTokens {
		for _, doc := range docs {
			if doc.tf[token] > 0 {
				df[token]++
			}
		}
	}

	results := make([]Result, 0, len(filtered))
	maxLexical := 0.0
	for i, item := range filtered {
		lexical := bm25(docs[i], qTokens, df, len(docs), avgLength)
		lexical += phraseBoost(item, query.Text)
		if lexical > maxLexical {
			maxLexical = lexical
		}
		semantic := 0.0
		if len(query.QueryVector) > 0 && len(item.Embedding) == len(query.QueryVector) {
			semantic = cosine(query.QueryVector, item.Embedding)
			semantic = (semantic + 1) / 2
			semantic = clamp(semantic, 0, 1)
		}
		recency := recencyScore(item, now)
		results = append(results, Result{
			Item:          item,
			LexicalScore:  lexical,
			SemanticScore: semantic,
			RecencyScore:  recency,
		})
	}

	for i := range results {
		lexical := 0.0
		if maxLexical > 0 {
			lexical = results[i].LexicalScore / maxLexical
		}
		if strings.TrimSpace(query.Text) == "" {
			results[i].Score = results[i].RecencyScore
		} else if len(query.QueryVector) > 0 && len(results[i].Item.Embedding) == len(query.QueryVector) {
			results[i].Score = 0.68*lexical + 0.27*results[i].SemanticScore + 0.05*results[i].RecencyScore
		} else {
			results[i].Score = 0.94*lexical + 0.06*results[i].RecencyScore
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if math.Abs(results[i].Score-results[j].Score) > 1e-9 {
			return results[i].Score > results[j].Score
		}
		return itemTime(results[i].Item).After(itemTime(results[j].Item))
	})
	if len(results) > query.Limit {
		results = results[:query.Limit]
	}
	return results
}

func BuildFacets(items []model.Item) Facets {
	tags := map[string]int{}
	sources := map[string]int{}
	for _, item := range items {
		for _, tag := range item.Tags {
			tags[tag]++
		}
		sources[item.SourceName]++
	}
	return Facets{Tags: toFacets(tags), Sources: toFacets(sources)}
}

type document struct {
	tf     map[string]int
	length int
}

func buildDocument(item model.Item) document {
	tf := map[string]int{}
	add := func(value string, weight int) {
		for _, token := range tokenize(value) {
			tf[token] += weight
		}
	}
	add(item.Title, 4)
	add(strings.Join(item.Tags, " "), 5)
	add(item.Summary, 2)
	add(item.Content, 1)
	add(item.SourceName, 2)
	length := 0
	for _, count := range tf {
		length += count
	}
	return document{tf: tf, length: length}
}

func bm25(doc document, queryTokens []string, df map[string]int, documentCount int, avgLength float64) float64 {
	if len(queryTokens) == 0 {
		return 0
	}
	const k1 = 1.2
	const b = 0.75
	score := 0.0
	for _, token := range queryTokens {
		tf := float64(doc.tf[token])
		if tf == 0 {
			continue
		}
		frequency := float64(df[token])
		idf := math.Log(1 + (float64(documentCount)-frequency+0.5)/(frequency+0.5))
		denominator := tf + k1*(1-b+b*float64(doc.length)/avgLength)
		score += idf * (tf * (k1 + 1)) / denominator
	}
	return score
}

func phraseBoost(item model.Item, rawQuery string) float64 {
	query := strings.ToLower(strings.TrimSpace(rawQuery))
	if query == "" {
		return 0
	}
	title := strings.ToLower(item.Title)
	summary := strings.ToLower(item.Summary)
	content := strings.ToLower(item.Content)
	boost := 0.0
	if title == query {
		boost += 8
	} else if strings.Contains(title, query) {
		boost += 4
	}
	for _, tag := range item.Tags {
		if strings.EqualFold(tag, query) {
			boost += 5
		} else if strings.Contains(strings.ToLower(tag), query) {
			boost += 2
		}
	}
	if strings.Contains(summary, query) {
		boost += 1.5
	} else if strings.Contains(content, query) {
		boost += 0.5
	}
	return boost
}

func filter(items []model.Item, tag, source string) []model.Item {
	tag = strings.TrimSpace(strings.ToLower(tag))
	source = strings.TrimSpace(strings.ToLower(source))
	out := make([]model.Item, 0, len(items))
	for _, item := range items {
		if source != "" && strings.ToLower(item.SourceName) != source && strings.ToLower(item.SourceID) != source {
			continue
		}
		if tag != "" && !containsFold(item.Tags, tag) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func tokenize(value string) []string {
	value = strings.ToLower(value)
	var tokens []string
	var current []rune
	flush := func() {
		if len(current) == 0 {
			return
		}
		word := string(current)
		if len([]rune(word)) > 1 || isASCIIWord(word) {
			tokens = append(tokens, word)
		}
		if containsCJK(current) && len(current) >= 2 {
			for i := 0; i+1 < len(current); i++ {
				tokens = append(tokens, string(current[i:i+2]))
			}
		}
		current = current[:0]
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_' || r == '.' || r == '+' || r == '#' {
			current = append(current, r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

func containsCJK(runes []rune) bool {
	for _, r := range runes {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana) {
			return true
		}
	}
	return false
}

func isASCIIWord(value string) bool {
	for _, r := range value {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return value != ""
}

func unique(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	dot, normA, normB := 0.0, 0.0, 0.0
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func recencyScore(item model.Item, now time.Time) float64 {
	age := now.Sub(itemTime(item)).Hours() / 24
	if age < 0 {
		age = 0
	}
	return math.Exp(-age / 90)
}

func itemTime(item model.Item) time.Time {
	if !item.PublishedAt.IsZero() {
		return item.PublishedAt
	}
	return item.CollectedAt
}

func toFacets(counts map[string]int) []Facet {
	out := make([]Facet, 0, len(counts))
	for name, count := range counts {
		if strings.TrimSpace(name) != "" {
			out = append(out, Facet{Name: name, Count: count})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Name < out[j].Name
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
