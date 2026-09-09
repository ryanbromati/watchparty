package auth

import (
	"testing"
	"time"
)

func TestCheckPassword(t *testing.T) {
	m := New("segredo-42")
	if !m.CheckPassword("segredo-42") {
		t.Fatal("senha correta rejeitada")
	}
	if m.CheckPassword("errada") {
		t.Fatal("senha errada aceita")
	}
}

func TestRateLimit(t *testing.T) {
	m := New("x")
	m.window = time.Minute
	// 5 tentativas permitidas dentro da janela, 6ª bloqueada.
	for i := 0; i < maxAttempts; i++ {
		if !m.Allowed("1.2.3.4") {
			t.Fatalf("tentativa %d deveria ser permitida", i+1)
		}
	}
	if m.Allowed("1.2.3.4") {
		t.Fatal("6ª tentativa deveria ser bloqueada")
	}
	// IP diferente segue permitido.
	if !m.Allowed("5.6.7.8") {
		t.Fatal("outro IP deveria estar permitido")
	}
}

func TestRateLimitWindowReset(t *testing.T) {
	m := New("x")
	m.window = 20 * time.Millisecond
	for i := 0; i < maxAttempts; i++ {
		m.Allowed("9.9.9.9")
	}
	if m.Allowed("9.9.9.9") {
		t.Fatal("deveria estar bloqueado")
	}
	time.Sleep(30 * time.Millisecond)
	if !m.Allowed("9.9.9.9") {
		t.Fatal("janela deveria ter sido resetada")
	}
}

func TestResetClearsAttempts(t *testing.T) {
	m := New("x")
	for i := 0; i < maxAttempts; i++ {
		m.Allowed("1.1.1.1")
	}
	m.Reset("1.1.1.1")
	if !m.Allowed("1.1.1.1") {
		t.Fatal("Reset deveria liberar o IP")
	}
}

func TestIssueAndValid(t *testing.T) {
	m := New("x")
	tok, err := m.Issue()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 2*tokenBytes {
		t.Fatalf("token com tamanho inesperado: %d", len(tok))
	}
	if !m.Valid(tok) {
		t.Fatal("token recém-emitido deveria ser válido")
	}
	if m.Valid("") || m.Valid("qualquer-coisa") {
		t.Fatal("tokens inválidos deveriam ser rejeitados")
	}
}

func TestTokenExpires(t *testing.T) {
	m := New("x")
	m.ttl = 20 * time.Millisecond
	tok, _ := m.Issue()
	time.Sleep(30 * time.Millisecond)
	if m.Valid(tok) {
		t.Fatal("token deveria ter expirado")
	}
}

func TestTokensDistintos(t *testing.T) {
	m := New("x")
	a, _ := m.Issue()
	b, _ := m.Issue()
	if a == b {
		t.Fatal("tokens não podem se repetir")
	}
}
