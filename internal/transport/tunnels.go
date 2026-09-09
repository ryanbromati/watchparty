package transport

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
)

var (
	reTryCloudflare = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)
	reTsNet         = regexp.MustCompile(`https://[a-zA-Z0-9-]+(?:\.[a-zA-Z0-9-]+)*\.ts\.net`)
)

type tailscaleRunner struct{}

// Start abre um tunnel funnel foreground e captura a URL impressa.
// Requer `tailscale` instalado e autenticado (tailscale up).
func (tailscaleRunner) Start(ctx context.Context, localPort int) (string, error) {
	return runScanURL(ctx, "tailscale", []string{"funnel", fmt.Sprint(localPort)}, true, matchTsNet,
		"tailscale não encontrado: instale com `pacman -S tailscale` e rode `tailscale up`")
}

type cloudflareRunner struct{}

// Start abre um quick tunnel do cloudflared e parseia a URL atribuída (stderr).
func (cloudflareRunner) Start(ctx context.Context, localPort int) (string, error) {
	return runScanURL(ctx, "cloudflared",
		[]string{"tunnel", "--url", fmt.Sprintf("http://127.0.0.1:%d", localPort)},
		false, matchTryCloudflare,
		"cloudflared não encontrado: instale-o para usar o transporte cloudflare")
}

// runScanURL executa o binário, lê a stream indicada e devolve a primeira URL
// aceita por match. O processo segue vivo até o ctx ser cancelado.
func runScanURL(ctx context.Context, bin string, args []string, useStdout bool, match func(string) (string, bool), missingMsg string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var pipe io.ReadCloser
	var err error
	if useStdout {
		pipe, err = cmd.StdoutPipe()
		cmd.Stderr = io.Discard
	} else {
		pipe, err = cmd.StderrPipe()
		cmd.Stdout = io.Discard
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", bin, err)
	}
	if err := cmd.Start(); err != nil {
		if _, lookErr := exec.LookPath(bin); lookErr != nil {
			return "", fmt.Errorf("%s", missingMsg)
		}
		return "", fmt.Errorf("iniciar %s: %w", bin, err)
	}

	urlCh := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(pipe)
		for sc.Scan() {
			if u, ok := match(sc.Text()); ok {
				urlCh <- u
				return
			}
		}
	}()

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	select {
	case u := <-urlCh:
		// mantém drenando a stream para o processo não bloquear até o ctx.
		go func() {
			sc := bufio.NewScanner(pipe)
			for sc.Scan() {
			}
		}()
		return u, nil
	case err := <-waitCh:
		if err == nil {
			err = fmt.Errorf("%s encerrou sem imprimir uma URL", bin)
		}
		return "", fmt.Errorf("%s: %w", bin, err)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func matchTsNet(line string) (string, bool) {
	if u := reTsNet.FindString(line); u != "" {
		return u, true
	}
	return "", false
}

func matchTryCloudflare(line string) (string, bool) {
	if u := reTryCloudflare.FindString(line); u != "" {
		return u, true
	}
	return "", false
}

// parseURL é usado em testes para extrair a URL de uma linha qualquer.
func parseURL(line string) (string, bool) {
	if u, ok := matchTsNet(line); ok {
		return u, ok
	}
	return matchTryCloudflare(line)
}
