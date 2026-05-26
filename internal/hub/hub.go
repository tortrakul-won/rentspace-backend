package hub

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Hub manages SSE subscriptions per profile and per admin user. Thread-safe.
type Hub struct {
	mu           sync.RWMutex
	clients      map[uuid.UUID]map[chan []byte]struct{} // profileID → channels
	adminClients map[uuid.UUID]map[chan []byte]struct{} // userID → channels (is_admin users only)
}

func New() *Hub {
	return &Hub{
		clients:      make(map[uuid.UUID]map[chan []byte]struct{}),
		adminClients: make(map[uuid.UUID]map[chan []byte]struct{}),
	}
}

// Subscribe registers a new SSE client for profileID.
// Returns a receive channel and a cleanup function that must be called on disconnect.
func (h *Hub) Subscribe(profileID uuid.UUID) (chan []byte, func()) {
	ch := make(chan []byte, 8)
	h.mu.Lock()
	if h.clients[profileID] == nil {
		h.clients[profileID] = make(map[chan []byte]struct{})
	}
	h.clients[profileID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.clients[profileID], ch)
		if len(h.clients[profileID]) == 0 {
			delete(h.clients, profileID)
		}
		h.mu.Unlock()
		close(ch)
	}
}

// SubscribeAdmin registers a new SSE client for an admin user.
// Returns a receive channel and a cleanup function that must be called on disconnect.
func (h *Hub) SubscribeAdmin(userID uuid.UUID) (chan []byte, func()) {
	ch := make(chan []byte, 8)
	h.mu.Lock()
	if h.adminClients[userID] == nil {
		h.adminClients[userID] = make(map[chan []byte]struct{})
	}
	h.adminClients[userID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.adminClients[userID], ch)
		if len(h.adminClients[userID]) == 0 {
			delete(h.adminClients, userID)
		}
		h.mu.Unlock()
		close(ch)
	}
}

// Publish sends an event to all active SSE connections for profileID.
// Drops the event for any slow consumer rather than blocking.
func (h *Hub) Publish(profileID uuid.UUID, event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients[profileID] {
		select {
		case ch <- data:
		default:
		}
	}
}

// PublishToAdmins broadcasts an event to all active admin SSE connections.
// Drops the event for any slow consumer rather than blocking.
func (h *Hub) PublishToAdmins(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, channels := range h.adminClients {
		for ch := range channels {
			select {
			case ch <- data:
			default:
			}
		}
	}
}
