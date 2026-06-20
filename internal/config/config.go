package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type Project struct {
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	DemoURL    string   `json:"demo_url,omitempty"`
	Summary    string   `json:"summary"`
	Stack      []string `json:"stack"`
	Highlights []string `json:"highlights"`
	Featured   bool     `json:"featured"`
}

type SkillGroup struct {
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

type Site struct {
	Handle       string       `json:"handle"`
	DisplayName  string       `json:"display_name"`
	Headline     string       `json:"headline"`
	Location     string       `json:"location"`
	Availability string       `json:"availability"`
	About        string       `json:"about"`
	Email        string       `json:"email,omitempty"`
	Links        []Link       `json:"links"`
	Projects     []Project    `json:"projects"`
	Skills       []SkillGroup `json:"skills"`
	Interests    []string     `json:"interests"`
}

type Feed struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Kind        string   `json:"kind"`
	Enabled     bool     `json:"enabled"`
	Tags        []string `json:"tags"`
	PollMinutes int      `json:"poll_minutes,omitempty"`
}

type App struct {
	Addr             string
	SitePath         string
	FeedsPath        string
	DataPath         string
	AdminToken       string
	InboxToken       string
	RefreshInterval  time.Duration
	RequestTimeout   time.Duration
	MaxItems         int
	FeedWorkers      int
	OllamaBaseURL    string
	OllamaChatModel  string
	OllamaEmbedModel string
	SummaryLanguage  string
	EnableLLMSummary bool
	EnableEmbeddings bool
	PublicBaseURL    string
	LogJSON          bool
	InitialRefresh   bool
}

func LoadApp() (App, error) {
	refresh, err := durationEnv("REFRESH_INTERVAL", 30*time.Minute)
	if err != nil {
		return App{}, err
	}
	timeout, err := durationEnv("REQUEST_TIMEOUT", 15*time.Second)
	if err != nil {
		return App{}, err
	}
	maxItems, err := intEnv("MAX_ITEMS", 5000)
	if err != nil {
		return App{}, err
	}
	workers, err := intEnv("FEED_WORKERS", 6)
	if err != nil {
		return App{}, err
	}

	app := App{
		Addr:             env("ADDR", ":8080"),
		SitePath:         env("SITE_CONFIG", "config/site.json"),
		FeedsPath:        env("FEEDS_CONFIG", "config/feeds.json"),
		DataPath:         env("DATA_PATH", "data/items.json"),
		AdminToken:       os.Getenv("ADMIN_TOKEN"),
		InboxToken:       os.Getenv("INBOX_TOKEN"),
		RefreshInterval:  refresh,
		RequestTimeout:   timeout,
		MaxItems:         maxItems,
		FeedWorkers:      workers,
		OllamaBaseURL:    strings.TrimRight(env("OLLAMA_BASE_URL", "http://localhost:11434/api"), "/"),
		OllamaChatModel:  os.Getenv("OLLAMA_CHAT_MODEL"),
		OllamaEmbedModel: os.Getenv("OLLAMA_EMBED_MODEL"),
		SummaryLanguage:  env("SUMMARY_LANGUAGE", "ja"),
		EnableLLMSummary: boolEnv("ENABLE_LLM_SUMMARY", false),
		EnableEmbeddings: boolEnv("ENABLE_EMBEDDINGS", false),
		PublicBaseURL:    strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/"),
		LogJSON:          boolEnv("LOG_JSON", false),
		InitialRefresh:   boolEnv("INITIAL_REFRESH", true),
	}
	if app.MaxItems < 100 {
		return App{}, errors.New("MAX_ITEMS must be at least 100")
	}
	if app.FeedWorkers < 1 || app.FeedWorkers > 32 {
		return App{}, errors.New("FEED_WORKERS must be between 1 and 32")
	}
	if app.EnableLLMSummary && app.OllamaChatModel == "" {
		return App{}, errors.New("ENABLE_LLM_SUMMARY=true requires OLLAMA_CHAT_MODEL")
	}
	if app.EnableEmbeddings && app.OllamaEmbedModel == "" {
		return App{}, errors.New("ENABLE_EMBEDDINGS=true requires OLLAMA_EMBED_MODEL")
	}
	return app, nil
}

func LoadSite(path string) (Site, error) {
	var cfg Site
	if err := loadJSON(path, &cfg); err != nil {
		return Site{}, err
	}
	if strings.TrimSpace(cfg.Handle) == "" || strings.TrimSpace(cfg.Headline) == "" {
		return Site{}, errors.New("site config requires handle and headline")
	}
	return cfg, nil
}

func LoadFeeds(path string) ([]Feed, error) {
	var feeds []Feed
	if err := loadJSON(path, &feeds); err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for i := range feeds {
		feeds[i].ID = strings.TrimSpace(feeds[i].ID)
		feeds[i].Name = strings.TrimSpace(feeds[i].Name)
		feeds[i].URL = strings.TrimSpace(feeds[i].URL)
		if feeds[i].ID == "" || feeds[i].Name == "" || feeds[i].URL == "" {
			return nil, fmt.Errorf("feed at index %d requires id, name, and url", i)
		}
		if _, ok := seen[feeds[i].ID]; ok {
			return nil, fmt.Errorf("duplicate feed id %q", feeds[i].ID)
		}
		seen[feeds[i].ID] = struct{}{}
		if feeds[i].Kind == "" {
			feeds[i].Kind = "rss"
		}
	}
	return feeds, nil
}

func loadJSON(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}

func intEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
