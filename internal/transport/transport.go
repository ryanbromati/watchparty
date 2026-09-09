// Package transport expõe a URL pública de uma sessão de formas plugáveis:
// local (sem túnel), tailscale funnel e cloudflare quick tunnel.
package transport

import (
	"context"
	"fmt"
)

// Interface abre um túnel/URL apontando para o serviço local na porta dada.
// O processo de túnel roda até o ctx ser cancelado.
type Interface interface {
	Start(ctx context.Context, localPort int) (string, error)
}

// New seleciona o backend pelo nome (local|tailscale|cloudflare).
func New(kind string) (Interface, error) {
	switch kind {
	case "local":
		return Local{}, nil
	case "tailscale":
		return tailscaleRunner{}, nil
	case "cloudflare":
		return cloudflareRunner{}, nil
	default:
		return nil, fmt.Errorf("transporte desconhecido: %q", kind)
	}
}

// Local apenas devolve a URL local — útil para testes e LAN.
type Local struct{}

// Start devolve a URL local imediatamente.
func (Local) Start(_ context.Context, localPort int) (string, error) {
	return fmt.Sprintf("http://127.0.0.1:%d", localPort), nil
}
