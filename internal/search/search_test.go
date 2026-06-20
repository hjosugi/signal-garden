package search

import (
	"testing"
	"time"

	"github.com/hjosugi/signal-garden/internal/model"
)

func TestRunRanksTitleAndTag(t *testing.T) {
	now := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	items := []model.Item{
		{ID: "a", Title: "Google Cloud Spanner architecture", Summary: "Distributed SQL", Tags: []string{"google-cloud", "database"}, PublishedAt: now.Add(-time.Hour)},
		{ID: "b", Title: "Frontend CSS update", Summary: "Browser styling", Tags: []string{"frontend"}, PublishedAt: now},
	}
	results := Run(items, Query{Text: "spanner", Limit: 10}, now)
	if len(results) == 0 || results[0].Item.ID != "a" {
		t.Fatalf("unexpected ranking: %#v", results)
	}
}

func TestJapaneseBigrams(t *testing.T) {
	now := time.Now().UTC()
	items := []model.Item{{ID: "a", Title: "分散システムの障害設計", Tags: []string{"distributed-systems"}, PublishedAt: now}}
	results := Run(items, Query{Text: "分散", Limit: 10}, now)
	if len(results) != 1 || results[0].Score <= 0 {
		t.Fatalf("expected Japanese query match: %#v", results)
	}
}
