package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Ollama struct {
	baseURL         string
	chatModel       string
	embedModel      string
	summaryLanguage string
	client          *http.Client
}

func NewOllama(baseURL, chatModel, embedModel, summaryLanguage string, timeout time.Duration) *Ollama {
	return &Ollama{
		baseURL:         strings.TrimRight(baseURL, "/"),
		chatModel:       strings.TrimSpace(chatModel),
		embedModel:      strings.TrimSpace(embedModel),
		summaryLanguage: strings.TrimSpace(summaryLanguage),
		client:          &http.Client{Timeout: timeout},
	}
}

func (o *Ollama) CanSummarize() bool { return o != nil && o.chatModel != "" }
func (o *Ollama) CanEmbed() bool     { return o != nil && o.embedModel != "" }

func (o *Ollama) Summarize(ctx context.Context, title, content string) (string, error) {
	if !o.CanSummarize() {
		return "", fmt.Errorf("Ollama chat model is not configured")
	}
	content = truncate(content, 6000)
	language := o.summaryLanguage
	if language == "" {
		language = "ja"
	}
	prompt := fmt.Sprintf(`Summarize the article below for a software engineer.
Use language code %q. Write at most two short sentences. Keep product names and technical terms unchanged. Do not add facts.

Title: %s
Content: %s`, language, title, content)

	request := chatRequest{
		Model:  o.chatModel,
		Stream: false,
		Messages: []chatMessage{
			{Role: "system", Content: "You are a precise technical news summarizer."},
			{Role: "user", Content: prompt},
		},
		Options: map[string]any{"temperature": 0.1},
	}
	var response chatResponse
	if err := o.post(ctx, "/chat", request, &response); err != nil {
		return "", err
	}
	result := strings.TrimSpace(response.Message.Content)
	if result == "" {
		return "", fmt.Errorf("Ollama returned an empty summary")
	}
	return truncate(result, 500), nil
}

func (o *Ollama) Embed(ctx context.Context, inputs []string) ([][]float64, error) {
	if !o.CanEmbed() {
		return nil, fmt.Errorf("Ollama embedding model is not configured")
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	clean := make([]string, len(inputs))
	for i, input := range inputs {
		clean[i] = truncate(input, 8000)
	}
	request := embedRequest{Model: o.embedModel, Input: clean, Truncate: true}
	var response embedResponse
	if err := o.post(ctx, "/embed", request, &response); err != nil {
		return nil, err
	}
	if len(response.Embeddings) != len(inputs) {
		return nil, fmt.Errorf("embedding count mismatch: requested %d, received %d", len(inputs), len(response.Embeddings))
	}
	return response.Embeddings, nil
}

func (o *Ollama) post(ctx context.Context, path string, body any, dst any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Ollama HTTP %d: %s", resp.StatusCode, truncate(string(data), 400))
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("decode Ollama response: %w", err)
	}
	return nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string         `json:"model"`
	Messages []chatMessage  `json:"messages"`
	Stream   bool           `json:"stream"`
	Options  map[string]any `json:"options,omitempty"`
}

type chatResponse struct {
	Message chatMessage `json:"message"`
}

type embedRequest struct {
	Model    string   `json:"model"`
	Input    []string `json:"input"`
	Truncate bool     `json:"truncate"`
}

type embedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

func truncate(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:max])) + "…"
}
