// Command watchparty compartilha a tela do host com espectadores via browser.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/ryanbromati/watchparty/internal/config"
	"github.com/ryanbromati/watchparty/internal/session"
)

const version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "share":
		return runShare(args[1:])
	case "version", "--version", "-v":
		fmt.Printf("watchparty %s\n", version)
		return 0
	case "help", "--help", "-h":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "watchparty: subcomando desconhecido %q\n", args[0])
		usage()
		return 2
	}
}

func runShare(args []string) int {
	opts, err := config.ParseFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 2
	}
	for _, bin := range []string{"ffmpeg", "wf-recorder"} {
		if _, err := exec.LookPath(bin); err != nil {
			fmt.Fprintf(os.Stderr, "pré-requisito ausente: %q não encontrado no PATH\n", bin)
			return 1
		}
	}

	// Senha é sempre sua escolha: se não veio por -password e o terminal está
	// disponível, pergunta (Enter duas vezes = sem senha).
	if !opts.PasswordExplicit && isTerminal(os.Stdin.Fd()) {
		pw, err := choosePassword()
		if err != nil {
			fmt.Fprintln(os.Stderr, "erro:", err)
			return 1
		}
		opts.Password = pw
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sess, err := session.Start(ctx, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 1
	}

	fmt.Printf("watchparty %s\n", version)
	fmt.Printf("URL        : %s\n", sess.URL())
	if sess.LocalURL() != sess.URL() {
		fmt.Printf("Local      : %s\n", sess.LocalURL())
	}
	fmt.Printf("Sala       : %s\n", sess.Room())
	if opts.Password == "" {
		fmt.Println("Senha      : (sem senha — qualquer pessoa com o link pode entrar)")
	} else {
		fmt.Printf("Senha      : %s\n", opts.Password)
	}
	fmt.Printf("Qualidade  : %s (%d kbps)\n", opts.Quality, opts.VideoBitrateKbps())
	fmt.Printf("Áudio      : %s\n", sess.AudioSource())
	fmt.Println("Ctrl+C encerra a sessão")

	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				fmt.Printf("\rEspectadores: %d/%d   ", sess.Viewers(), sess.MaxViewers())
			}
		}
	}()

	<-ctx.Done()
	fmt.Println("\nsessão encerrada")
	sess.Close()
	return 0
}

func usage() {
	fmt.Fprintln(os.Stderr, `uso: watchparty <comando> [flags]

comandos:
  share     inicia uma sessão de compartilhamento
  version   exibe a versão

share:
  -transport local|tailscale|cloudflare   transporte (padrão tailscale)
  -password SENHA                         senha da sala (perguntada se não informada; vazia = sem senha)
  -max-viewers N                          limite de espectadores (padrão 3)
  -quality 720p|1080p                     qualidade de vídeo (padrão 1080p)
  -room SLUG                              slug da sala no link (auto-gerado)
  -audio FONTE                            fonte de áudio (auto-detectada se vazio)`)
}
