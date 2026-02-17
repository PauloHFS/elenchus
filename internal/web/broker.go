package web

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/PauloHFS/goth/internal/db"
)

// Broker handles SSE connections and targeted broadcasting
type Broker struct {
	// Mapeia UserID -> lista de canais (um usuário pode ter múltiplas abas abertas)
	userClients map[int64][]chan string
	mu          sync.Mutex

	newClient     chan clientRegistration
	closingClient chan clientRegistration
	message       chan targetedMessage
	stop          chan struct{}
}

type clientRegistration struct {
	userID int64
	ch     chan string
}

type targetedMessage struct {
	userID int64 // 0 para broadcast global
	event  string
	data   string
}

var globalBroker *Broker

func init() {
	globalBroker = &Broker{
		userClients:   make(map[int64][]chan string),
		newClient:     make(chan clientRegistration),
		closingClient: make(chan clientRegistration),
		message:       make(chan targetedMessage),
		stop:          make(chan struct{}),
	}
	go globalBroker.listen()
}

func (b *Broker) listen() {
	for {
		select {
		case <-b.stop:
			b.mu.Lock()
			for _, channels := range b.userClients {
				for _, ch := range channels {
					close(ch)
				}
			}
			b.userClients = make(map[int64][]chan string)
			b.mu.Unlock()
			return
		case reg := <-b.newClient:
			b.mu.Lock()
			b.userClients[reg.userID] = append(b.userClients[reg.userID], reg.ch)
			b.mu.Unlock()

		case reg := <-b.closingClient:
			b.mu.Lock()
			clients := b.userClients[reg.userID]
			for i, ch := range clients {
				if ch == reg.ch {
					b.userClients[reg.userID] = append(clients[:i], clients[i+1:]...)
					break
				}
			}
			if len(b.userClients[reg.userID]) == 0 {
				delete(b.userClients, reg.userID)
			}
			b.mu.Unlock()

		case tm := <-b.message:
			b.mu.Lock()
			msg := fmt.Sprintf("event: %s\ndata: %s\n\n", tm.event, tm.data)

			if tm.userID == 0 {
				// Broadcast Global
				for _, channels := range b.userClients {
					for _, ch := range channels {
						ch <- msg
					}
				}
			} else {
				// Broadcast Direcionado
				if channels, ok := b.userClients[tm.userID]; ok {
					for _, ch := range channels {
						ch <- msg
					}
				}
			}
			b.mu.Unlock()
		}
	}
}

// Broadcast sends a message to EVERYONE
func Broadcast(event string, data string) {
	globalBroker.message <- targetedMessage{userID: 0, event: event, data: data}
}

// BroadcastToUser sends a message to a specific user ID
func BroadcastToUser(userID int64, event string, data string) {
	globalBroker.message <- targetedMessage{userID: userID, event: event, data: data}
}

// Shutdown gracefully closes the broker
func Shutdown() {
	close(globalBroker.stop)
}

// GlobalSSEHandler registers new SSE connections to the broker
func GlobalSSEHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(contextkeys.UserContextKey).(db.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	messageChan := make(chan string, 10) // Buffer para evitar bloqueio
	reg := clientRegistration{userID: user.ID, ch: messageChan}

	globalBroker.newClient <- reg

	defer func() {
		globalBroker.closingClient <- reg
	}()

	flusher.Flush()

	for {
		select {
		case msg := <-messageChan:
			fmt.Fprint(w, msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
