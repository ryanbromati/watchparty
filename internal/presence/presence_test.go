package presence

import (
	"testing"
	"time"
)

func TestCountAndFull(t *testing.T) {
	h := New(3)
	h.Beat("a")
	h.Beat("b")
	if got := h.Count(); got != 2 {
		t.Fatalf("Count = %d, want 2", got)
	}
	if h.Full() {
		t.Fatal("2/3 não deveria estar cheia")
	}
	h.Beat("c")
	if !h.Full() {
		t.Fatal("3/3 deveria estar cheia")
	}
	if h.Max() != 3 {
		t.Fatalf("Max = %d", h.Max())
	}
}

func TestExpiry(t *testing.T) {
	h := New(3)
	h.ttl = 20 * time.Millisecond
	h.Beat("a")
	if h.Count() != 1 {
		t.Fatal("deveria contar 1 antes de expirar")
	}
	time.Sleep(30 * time.Millisecond)
	if h.Count() != 0 {
		t.Fatal("espectador deveria ter expirado")
	}
}

func TestEmptyBeatIgnored(t *testing.T) {
	h := New(3)
	h.Beat("")
	if h.Count() != 0 {
		t.Fatal("token vazio não deveria contar")
	}
}
