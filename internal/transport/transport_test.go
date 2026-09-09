package transport

import (
	"context"
	"strings"
	"testing"
)

func TestNewUnknown(t *testing.T) {
	if _, err := New("ftp"); err == nil {
		t.Fatal("esperava erro para transporte desconhecido")
	}
}

func TestNewKnown(t *testing.T) {
	for _, k := range []string{"local", "tailscale", "cloudflare"} {
		if _, err := New(k); err != nil {
			t.Fatalf("New(%q): %v", k, err)
		}
	}
}

func TestLocalStart(t *testing.T) {
	tr, _ := New("local")
	url, err := tr.Start(context.Background(), 4321)
	if err != nil {
		t.Fatal(err)
	}
	if url != "http://127.0.0.1:4321" {
		t.Fatalf("url = %q", url)
	}
}

func TestParseTsNet(t *testing.T) {
	line := "Available on the internet:\nhttps://meu-pc.tailnet-abc.ts.net"
	u, ok := parseURL(line)
	if !ok || u != "https://meu-pc.tailnet-abc.ts.net" {
		t.Fatalf("got %q ok=%v", u, ok)
	}
}

func TestParseTryCloudflare(t *testing.T) {
	line := `2026-09-08T00:00:00Z INF +--------------------------------------------------------------------------------------------+`
	line2 := `INF |  https://abelha-aleatoria-123.trycloudflare.com                    |`
	if u, ok := parseURL(line2); !ok || !strings.Contains(u, "trycloudflare.com") {
		t.Fatalf("got %q ok=%v", u, ok)
	}
	_ = line
}

func TestParseNoMatch(t *testing.T) {
	if u, ok := parseURL("nada aqui"); ok {
		t.Fatalf("não deveria casar, got %q", u)
	}
}
