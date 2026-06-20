package model

import "time"

// Item is one normalized entry collected from RSS, Atom, or a YouTube feed.
// Embedding is optional and is populated only when Ollama embedding is enabled.
type Item struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"source_id"`
	SourceName  string    `json:"source_name"`
	SourceKind  string    `json:"source_kind"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Author      string    `json:"author,omitempty"`
	Summary     string    `json:"summary"`
	Content     string    `json:"content,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	CollectedAt time.Time `json:"collected_at"`
	Tags        []string  `json:"tags"`
	Embedding   []float64 `json:"embedding,omitempty"`
}

// SourceStatus is exposed through the status API and SSE stream.
type SourceStatus struct {
	SourceID     string    `json:"source_id"`
	SourceName   string    `json:"source_name"`
	LastAttempt  time.Time `json:"last_attempt,omitempty"`
	LastSuccess  time.Time `json:"last_success,omitempty"`
	ItemsSeen    int       `json:"items_seen"`
	ItemsAdded   int       `json:"items_added"`
	LastError    string    `json:"last_error,omitempty"`
	ResponseCode int       `json:"response_code,omitempty"`
}

// CollectionReport summarizes one refresh run.
type CollectionReport struct {
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at"`
	Added      int            `json:"added"`
	Updated    int            `json:"updated"`
	Failed     int            `json:"failed"`
	Sources    []SourceStatus `json:"sources"`
}
