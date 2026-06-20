package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hjosugi/signal-garden/internal/model"
)

func TestJSONStoreUpsertAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.json")
	store, err := OpenJSON(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	item := model.Item{ID: "one", Title: "First", URL: "https://example.com/1", CollectedAt: now, PublishedAt: now}
	added, updated, err := store.Upsert([]model.Item{item})
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 || updated != 0 {
		t.Fatalf("unexpected counts: added=%d updated=%d", added, updated)
	}

	reloaded, err := OpenJSON(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Count(); got != 1 {
		t.Fatalf("expected 1 item, got %d", got)
	}
}
