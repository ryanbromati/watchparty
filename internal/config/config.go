// Package config define opções da sessão e parsing de flags da CLI.
package config

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"strings"
)

type Transport string

const (
	TransportLocal      Transport = "local"
	TransportTailscale  Transport = "tailscale"
	TransportCloudflare Transport = "cloudflare"
)

type Quality string

const (
	Quality720p  Quality = "720p"
	Quality1080p Quality = "1080p"
)

// Options é a configuração resolvida de uma sessão `share`.
type Options struct {
	Transport        Transport
	Password         string // escolha do usuário; "" = sala sem senha
	PasswordExplicit bool   // true se -password foi passada explicitamente
	MaxViewers       int
	Quality          Quality
	Room             string // sempre preenchida (fornecida ou gerada)
	Audio            string // fonte de áudio; vazio = auto-detectada em runtime
}

// ParseFlags interpreta os argumentos do subcomando `share`.
func ParseFlags(args []string) (Options, error) {
	fs := flag.NewFlagSet("share", flag.ContinueOnError)
	transport := fs.String("transport", string(TransportTailscale), "transporte: local | tailscale | cloudflare")
	password := fs.String("password", "", "senha da sala (perguntada se não informada; vazia = sem senha)")
	maxViewers := fs.Int("max-viewers", 3, "limite de espectadores simultâneos")
	quality := fs.String("quality", string(Quality1080p), "qualidade: 720p | 1080p")
	room := fs.String("room", "", "slug da sala no link (auto-gerado se vazio)")
	audio := fs.String("audio", "", "fonte de áudio (ex.: nome .monitor); vazio = auto-detectada")

	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}
	if fs.NArg() != 0 {
		return Options{}, fmt.Errorf("argumentos inesperados: %v", fs.Args())
	}

	explicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "password" {
			explicit = true
		}
	})

	opts := Options{
		Transport:        Transport(*transport),
		Password:         *password,
		PasswordExplicit: explicit,
		MaxViewers:       *maxViewers,
		Quality:          Quality(*quality),
		Audio:            *audio,
		Room:             *room,
	}
	if err := opts.normalize(); err != nil {
		return Options{}, err
	}
	return opts, nil
}

func (o *Options) normalize() error {
	switch o.Transport {
	case TransportLocal, TransportTailscale, TransportCloudflare:
	default:
		return fmt.Errorf("transporte inválido: %q (use local|tailscale|cloudflare)", o.Transport)
	}
	switch o.Quality {
	case Quality720p, Quality1080p:
	default:
		return fmt.Errorf("qualidade inválida: %q (use 720p|1080p)", o.Quality)
	}
	if o.MaxViewers < 1 {
		return errors.New("max-viewers deve ser >= 1")
	}
	o.Password = strings.TrimSpace(o.Password)
	o.Room = strings.TrimSpace(o.Room)
	if o.Room == "" {
		r, err := GenerateSlug()
		if err != nil {
			return err
		}
		o.Room = r
	}
	return nil
}

// VideoBitrateKbps retorna o bitrate de vídeo alvo conforme a qualidade.
func (o Options) VideoBitrateKbps() int {
	if o.Quality == Quality720p {
		return 3000
	}
	return 6000
}

// OutputHeight retorna a altura de saída conforme a qualidade (720/1080).
func (o Options) OutputHeight() int {
	if o.Quality == Quality720p {
		return 720
	}
	return 1080
}

// GenerateSlug gera um slug curto para a sala (URL path).
func GenerateSlug() (string, error) {
	const pool = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		n, err := randInt(len(pool))
		if err != nil {
			return "", err
		}
		b[i] = pool[n]
	}
	return string(b), nil
}

func randInt(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("randInt: max <= 0")
	}
	// Rejeita viés de módulo: amostra até caber sem sobra.
	limit := 256 - (256 % max)
	buf := make([]byte, 1)
	for {
		if _, err := rand.Read(buf); err != nil {
			return 0, err
		}
		if int(buf[0]) < limit {
			return int(buf[0]) % max, nil
		}
	}
}
