// Package presence rastreia espectadores ativos por heartbeat (janela ~15s)
// e aplica o limite máximo de espectadores.
package presence

import (
	"sync"
	"time"
)

const defaultTTL = 15 * time.Second

// Hub guarda o último heartbeat por token.
type Hub struct {
	mu       sync.Mutex
	lastSeen map[string]time.Time
	max      int
	ttl      time.Duration
}

// New cria um Hub com limite max de espectadores.
func New(max int) *Hub {
	return &Hub{
		lastSeen: make(map[string]time.Time),
		max:      max,
		ttl:      defaultTTL,
	}
}

// Beat registra atividade do espectador (token) agora.
func (h *Hub) Beat(token string) {
	if token == "" {
		return
	}
	h.mu.Lock()
	h.lastSeen[token] = time.Now()
	h.mu.Unlock()
}

// Count retorna o número de espectadores ativos (prune de expirados).
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for tok, last := range h.lastSeen {
		if now.Sub(last) > h.ttl {
			delete(h.lastSeen, tok)
		}
	}
	return len(h.lastSeen)
}

// Full indica se a sala atingiu o limite de espectadores.
func (h *Hub) Full() bool {
	return h.Count() >= h.max
}

// Max retorna o limite de espectadores.
func (h *Hub) Max() int {
	return h.max
}
