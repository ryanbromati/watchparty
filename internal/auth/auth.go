// Package auth cuida da autenticação por senha da sala: comparação em tempo
// constante, emissão/validação de tokens de sessão e rate-limit por IP.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"sync"
	"time"
)

const (
	sessionTTL    = 12 * time.Hour
	attemptWindow = time.Minute
	maxAttempts   = 5
	tokenBytes    = 32
)

// Manager guarda o hash da senha, sessões ativas e tentativas por IP.
type Manager struct {
	mu       sync.Mutex
	pwHash   [sha256.Size]byte
	sessions map[string]time.Time
	attempts map[string]*attempt
	ttl      time.Duration
	window   time.Duration
}

type attempt struct {
	count int
	start time.Time
}

// New cria um Manager para a senha dada.
func New(password string) *Manager {
	return &Manager{
		pwHash:   sha256.Sum256([]byte(password)),
		sessions: make(map[string]time.Time),
		attempts: make(map[string]*attempt),
		ttl:      sessionTTL,
		window:   attemptWindow,
	}
}

// CheckPassword compara a senha em tempo constante.
func (m *Manager) CheckPassword(pw string) bool {
	h := sha256.Sum256([]byte(pw))
	return subtle.ConstantTimeCompare(h[:], m.pwHash[:]) == 1
}

// Allowed verifica se o IP ainda pode tentar (janela deslizante de tentativas).
func (m *Manager) Allowed(ip string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	a := m.attempts[ip]
	if a == nil || now.Sub(a.start) >= m.window {
		m.attempts[ip] = &attempt{count: 1, start: now}
		return true
	}
	if a.count >= maxAttempts {
		return false
	}
	a.count++
	return true
}

// Reset limpa as tentativas do IP (após login bem-sucedido).
func (m *Manager) Reset(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.attempts, ip)
}

// Issue cria um token de sessão aleatório com validade ttl.
func (m *Manager) Issue() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(buf)
	m.mu.Lock()
	m.sessions[tok] = time.Now().Add(m.ttl)
	m.mu.Unlock()
	return tok, nil
}

// Valid informa se o token é válido (e remove os expirados).
func (m *Manager) Valid(tok string) bool {
	if tok == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	exp, ok := m.sessions[tok]
	if !ok {
		return false
	}
	if now.After(exp) {
		delete(m.sessions, tok)
		return false
	}
	return true
}
