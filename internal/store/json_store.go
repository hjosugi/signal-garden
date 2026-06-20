package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/hjosugi/signal-garden/internal/model"
)

type JSONStore struct {
	mu       sync.RWMutex
	path     string
	maxItems int
	items    map[string]model.Item
}

func OpenJSON(path string, maxItems int) (*JSONStore, error) {
	s := &JSONStore{
		path:     path,
		maxItems: maxItems,
		items:    make(map[string]model.Item),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *JSONStore) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read store: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	var items []model.Item
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("parse store: %w", err)
	}
	for _, item := range items {
		if item.ID != "" {
			s.items[item.ID] = item
		}
	}
	return nil
}

func (s *JSONStore) Upsert(incoming []model.Item) (added, updated int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range incoming {
		if item.ID == "" {
			continue
		}
		current, exists := s.items[item.ID]
		if !exists {
			s.items[item.ID] = item
			added++
			continue
		}
		if item.CollectedAt.Before(current.CollectedAt) {
			item.CollectedAt = current.CollectedAt
		}
		if len(item.Embedding) == 0 && len(current.Embedding) > 0 {
			item.Embedding = current.Embedding
		}
		if !equivalent(current, item) {
			s.items[item.ID] = item
			updated++
		}
	}

	s.trimLocked()
	if added == 0 && updated == 0 {
		return added, updated, nil
	}
	if err := s.saveLocked(); err != nil {
		return 0, 0, err
	}
	return added, updated, nil
}

func (s *JSONStore) All() []model.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]model.Item, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, clone(item))
	}
	sortItems(items)
	return items
}

func (s *JSONStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

func (s *JSONStore) UpdateEmbeddings(vectors map[string][]float64) error {
	if len(vectors) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for id, vector := range vectors {
		item, ok := s.items[id]
		if !ok || len(vector) == 0 {
			continue
		}
		item.Embedding = append([]float64(nil), vector...)
		s.items[id] = item
		changed = true
	}
	if !changed {
		return nil
	}
	return s.saveLocked()
}

func (s *JSONStore) saveLocked() error {
	items := make([]model.Item, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	sortItems(items)

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".items-*.json")
	if err != nil {
		return fmt.Errorf("create temp store: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)

	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(items); err != nil {
		tmp.Close()
		return fmt.Errorf("encode store: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close store: %w", err)
	}
	if err := os.Rename(name, s.path); err != nil {
		return fmt.Errorf("replace store: %w", err)
	}
	return nil
}

func (s *JSONStore) trimLocked() {
	if len(s.items) <= s.maxItems {
		return
	}
	items := make([]model.Item, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	sortItems(items)
	keep := make(map[string]model.Item, s.maxItems)
	for _, item := range items[:s.maxItems] {
		keep[item.ID] = item
	}
	s.items = keep
}

func equivalent(a, b model.Item) bool {
	if a.Title != b.Title || a.URL != b.URL || a.Summary != b.Summary || a.Content != b.Content || a.Author != b.Author || a.SourceName != b.SourceName || a.SourceKind != b.SourceKind || !a.PublishedAt.Equal(b.PublishedAt) {
		return false
	}
	if len(a.Tags) != len(b.Tags) {
		return false
	}
	for i := range a.Tags {
		if a.Tags[i] != b.Tags[i] {
			return false
		}
	}
	return true
}

func sortItems(items []model.Item) {
	sort.Slice(items, func(i, j int) bool {
		left := items[i].PublishedAt
		right := items[j].PublishedAt
		if left.IsZero() {
			left = items[i].CollectedAt
		}
		if right.IsZero() {
			right = items[j].CollectedAt
		}
		if left.Equal(right) {
			return items[i].ID < items[j].ID
		}
		return left.After(right)
	})
}

func clone(item model.Item) model.Item {
	item.Tags = append([]string(nil), item.Tags...)
	item.Embedding = append([]float64(nil), item.Embedding...)
	return item
}

func Age(item model.Item, now time.Time) time.Duration {
	t := item.PublishedAt
	if t.IsZero() {
		t = item.CollectedAt
	}
	return now.Sub(t)
}
