package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hjosugi/signal-garden/internal/ai"
	"github.com/hjosugi/signal-garden/internal/config"
	"github.com/hjosugi/signal-garden/internal/feed"
	"github.com/hjosugi/signal-garden/internal/model"
	"github.com/hjosugi/signal-garden/internal/store"
	"github.com/hjosugi/signal-garden/internal/tagger"
)

var ErrAlreadyRunning = errors.New("collection is already running")

type Notifier func(event string, payload any)

type Collector struct {
	feeds            []config.Feed
	fetcher          *feed.Fetcher
	store            *store.JSONStore
	tagger           *tagger.Tagger
	ollama           *ai.Ollama
	enableLLMSummary bool
	enableEmbeddings bool
	workers          int
	logger           *slog.Logger
	notify           Notifier

	mu         sync.RWMutex
	running    bool
	lastReport model.CollectionReport
	statuses   map[string]model.SourceStatus
}

func New(
	feeds []config.Feed,
	fetcher *feed.Fetcher,
	store *store.JSONStore,
	tagger *tagger.Tagger,
	ollama *ai.Ollama,
	enableLLMSummary bool,
	enableEmbeddings bool,
	workers int,
	logger *slog.Logger,
	notify Notifier,
) *Collector {
	statuses := make(map[string]model.SourceStatus, len(feeds))
	for _, cfg := range feeds {
		statuses[cfg.ID] = model.SourceStatus{SourceID: cfg.ID, SourceName: cfg.Name}
	}
	return &Collector{
		feeds:            append([]config.Feed(nil), feeds...),
		fetcher:          fetcher,
		store:            store,
		tagger:           tagger,
		ollama:           ollama,
		enableLLMSummary: enableLLMSummary,
		enableEmbeddings: enableEmbeddings,
		workers:          workers,
		logger:           logger,
		notify:           notify,
		statuses:         statuses,
	}
}

func (c *Collector) Refresh(ctx context.Context) (model.CollectionReport, error) {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return model.CollectionReport{}, ErrAlreadyRunning
	}
	c.running = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
	}()

	report := model.CollectionReport{StartedAt: time.Now().UTC()}
	c.emit("collection_started", report)
	c.logger.Info("feed collection started")

	existing := make(map[string]model.Item)
	for _, item := range c.store.All() {
		existing[item.ID] = item
	}

	jobs := make(chan config.Feed)
	results := make(chan sourceResult)
	var wg sync.WaitGroup
	workerCount := c.workers
	if workerCount > len(c.feeds) {
		workerCount = len(c.feeds)
	}
	if workerCount < 1 {
		workerCount = 1
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cfg := range jobs {
				results <- c.collectSource(ctx, cfg, existing)
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, cfg := range c.feeds {
			if !cfg.Enabled {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case jobs <- cfg:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		report.Sources = append(report.Sources, result.status)
		if result.err != nil {
			report.Failed++
			c.logger.Warn("feed collection failed", "source", result.status.SourceName, "error", result.err)
			continue
		}
		report.Added += result.added
		report.Updated += result.updated
	}

	report.FinishedAt = time.Now().UTC()
	sort.Slice(report.Sources, func(i, j int) bool { return report.Sources[i].SourceName < report.Sources[j].SourceName })
	c.mu.Lock()
	c.lastReport = report
	for _, status := range report.Sources {
		c.statuses[status.SourceID] = status
	}
	c.mu.Unlock()

	c.emit("collection_finished", report)
	c.logger.Info("feed collection finished", "added", report.Added, "updated", report.Updated, "failed", report.Failed)
	if ctx.Err() != nil {
		return report, ctx.Err()
	}
	return report, nil
}

func (c *Collector) collectSource(ctx context.Context, cfg config.Feed, existing map[string]model.Item) sourceResult {
	status := c.statusFor(cfg)
	status.LastAttempt = time.Now().UTC()
	fetched, err := c.fetcher.Fetch(ctx, cfg)
	status.ResponseCode = fetched.StatusCode
	if err != nil {
		status.LastError = err.Error()
		c.updateStatus(status)
		return sourceResult{status: status, err: err}
	}
	status.ItemsSeen = len(fetched.Items)

	items := fetched.Items
	if len(items) > 50 {
		items = items[:50]
	}
	for i := range items {
		items[i].Tags = c.tagger.Apply(items[i].Title, items[i].Content, items[i].Tags)
		old, exists := existing[items[i].ID]
		if exists && len(old.Embedding) > 0 {
			items[i].Embedding = append([]float64(nil), old.Embedding...)
		}
	}

	if c.enableLLMSummary && c.ollama.CanSummarize() {
		c.addSummaries(ctx, items, existing)
	}
	if c.enableEmbeddings && c.ollama.CanEmbed() {
		c.addEmbeddings(ctx, items)
	}

	added, updated, err := c.store.Upsert(items)
	if err != nil {
		status.LastError = err.Error()
		c.updateStatus(status)
		return sourceResult{status: status, err: err}
	}
	status.LastSuccess = time.Now().UTC()
	status.LastError = ""
	status.ItemsAdded = added
	c.updateStatus(status)
	return sourceResult{status: status, added: added, updated: updated}
}

func (c *Collector) addSummaries(ctx context.Context, items []model.Item, existing map[string]model.Item) {
	remaining := 8
	for i := range items {
		if remaining == 0 || ctx.Err() != nil {
			return
		}
		if old, exists := existing[items[i].ID]; exists && strings.TrimSpace(old.Summary) != "" {
			continue
		}
		if strings.TrimSpace(items[i].Content) == "" {
			continue
		}
		callCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		summary, err := c.ollama.Summarize(callCtx, items[i].Title, items[i].Content)
		cancel()
		if err != nil {
			c.logger.Debug("LLM summary skipped", "item", items[i].ID, "error", err)
			continue
		}
		items[i].Summary = summary
		remaining--
	}
}

func (c *Collector) addEmbeddings(ctx context.Context, items []model.Item) {
	var indexes []int
	var inputs []string
	for i := range items {
		if len(items[i].Embedding) > 0 {
			continue
		}
		text := strings.TrimSpace(items[i].Title + "\n" + items[i].Summary + "\n" + items[i].Content)
		if text == "" {
			continue
		}
		indexes = append(indexes, i)
		inputs = append(inputs, text)
	}
	const batchSize = 16
	for start := 0; start < len(inputs); start += batchSize {
		end := start + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		callCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		vectors, err := c.ollama.Embed(callCtx, inputs[start:end])
		cancel()
		if err != nil {
			c.logger.Debug("embedding batch skipped", "error", err)
			return
		}
		for offset, vector := range vectors {
			items[indexes[start+offset]].Embedding = vector
		}
	}
}

func (c *Collector) Snapshot() (bool, model.CollectionReport, []model.SourceStatus) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	statuses := make([]model.SourceStatus, 0, len(c.statuses))
	for _, status := range c.statuses {
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].SourceName < statuses[j].SourceName })
	return c.running, c.lastReport, statuses
}

func (c *Collector) statusFor(cfg config.Feed) model.SourceStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	status := c.statuses[cfg.ID]
	status.SourceID = cfg.ID
	status.SourceName = cfg.Name
	return status
}

func (c *Collector) updateStatus(status model.SourceStatus) {
	c.mu.Lock()
	c.statuses[status.SourceID] = status
	c.mu.Unlock()
	c.emit("source_status", status)
}

func (c *Collector) emit(event string, payload any) {
	if c.notify != nil {
		c.notify(event, payload)
	}
}

type sourceResult struct {
	status  model.SourceStatus
	added   int
	updated int
	err     error
}

func (c *Collector) RefreshAsync(parent context.Context) error {
	c.mu.RLock()
	running := c.running
	c.mu.RUnlock()
	if running {
		return ErrAlreadyRunning
	}
	go func() {
		ctx, cancel := context.WithTimeout(parent, 15*time.Minute)
		defer cancel()
		if _, err := c.Refresh(ctx); err != nil && !errors.Is(err, ErrAlreadyRunning) && !errors.Is(err, context.Canceled) {
			c.logger.Error("asynchronous collection failed", "error", err)
		}
	}()
	return nil
}

func ValidateFeeds(feeds []config.Feed) error {
	enabled := 0
	for _, cfg := range feeds {
		if cfg.Enabled {
			enabled++
		}
	}
	if enabled == 0 {
		return fmt.Errorf("no feeds are enabled")
	}
	return nil
}
