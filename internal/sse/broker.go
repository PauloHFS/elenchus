package sse

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// Client represents a connected SSE client
type Client struct {
	Events chan string
}

// Broker manages SSE connections globally
type Broker struct {
	clients map[string]map[*Client]bool // resourceKey -> clients
	mutex   sync.RWMutex
}

// NewBroker creates a new global SSE broker
func NewBroker() *Broker {
	return &Broker{
		clients: make(map[string]map[*Client]bool),
	}
}

// GetResourceKey creates a unique key for a resource
func (b *Broker) GetResourceKey(resourceType, resourceID string) string {
	return fmt.Sprintf("%s:%s", resourceType, resourceID)
}

// Subscribe registers a client for a specific resource
func (b *Broker) Subscribe(resourceType, resourceID string) *Client {
	key := b.GetResourceKey(resourceType, resourceID)

	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.clients[key] == nil {
		b.clients[key] = make(map[*Client]bool)
	}

	client := &Client{
		Events: make(chan string, 100),
	}

	b.clients[key][client] = true
	return client
}

// Unsubscribe removes a client
func (b *Broker) Unsubscribe(client *Client, resourceType, resourceID string) {
	key := b.GetResourceKey(resourceType, resourceID)
	
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if clients, ok := b.clients[key]; ok {
		delete(clients, client)
		close(client.Events)
		if len(clients) == 0 {
			delete(b.clients, key)
		}
	}
}

// SendHTML sends HTML content to all clients subscribed to a resource
func (b *Broker) SendHTML(resourceType, resourceID, eventType, html string) {
	key := b.GetResourceKey(resourceType, resourceID)

	b.mutex.RLock()
	defer b.mutex.RUnlock()

	// Format multi-line data correctly for SSE
	var formattedData string
	lines := strings.Split(html, "\n")
	for i, line := range lines {
		formattedData += "data: " + line
		if i < len(lines)-1 {
			formattedData += "\n"
		}
	}

	message := fmt.Sprintf("event: %s\n%s\n\n", eventType, formattedData)

	for client := range b.clients[key] {
		select {
		case client.Events <- message:
			// Sent successfully
		default:
			// Client buffer full, skip
		}
	}
}

// SendEvaluationProgress sends a progress update
func (b *Broker) SendEvaluationProgress(evaluationID, phase string, progress, total int, html string) {
	b.SendHTML("evaluation", evaluationID, "evaluation_progress", html)
}

// SendEvaluationComplete sends completion with result HTML
func (b *Broker) SendEvaluationComplete(evaluationID, html string) {
	b.SendHTML("evaluation", evaluationID, "evaluation_complete", html)
}

// SendEvaluationError sends error HTML
func (b *Broker) SendEvaluationError(evaluationID, html string) {
	b.SendHTML("evaluation", evaluationID, "evaluation_error", html)
}

// Handler returns HTTP handler for SSE connections
func (b *Broker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resourceType := r.URL.Query().Get("type")
		resourceID := r.URL.Query().Get("id")

		if resourceType == "" || resourceID == "" {
			http.Error(w, "type and id required", http.StatusBadRequest)
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Subscribe
		client := b.Subscribe(resourceType, resourceID)
		defer b.Unsubscribe(client, resourceType, resourceID)

		// Send initial comment to keep the connection alive and acknowledge
		fmt.Fprintf(w, ": ok\n\n")
		flusher.Flush()

		// Stream events
		for {
			select {
			case message, ok := <-client.Events:
				if !ok {
					return
				}
				fmt.Fprint(w, message)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
