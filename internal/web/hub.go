package web

import (
	"encoding/json"
	"sync"
	"time"
)

type Event struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

type Hub struct {
	mu      sync.RWMutex
	nextID  int
	clients map[int]chan Event
}

func NewHub() *Hub {
	return &Hub{clients: make(map[int]chan Event)}
}

func (h *Hub) Register() (int, <-chan Event, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := h.nextID
	h.nextID++
	ch := make(chan Event, 32)
	h.clients[id] = ch
	count := len(h.clients)
	return id, ch, count
}

func (h *Hub) Unregister(id int) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ch, ok := h.clients[id]; ok {
		delete(h.clients, id)
		close(ch)
	}
	return len(h.clients)
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) Publish(eventType string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	event := Event{Type: eventType, Data: data, Timestamp: time.Now().UTC()}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.clients {
		select {
		case ch <- event:
		default:
		}
	}
}
