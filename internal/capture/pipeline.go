package capture

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// Pipeline supervisiona os dois subprocessos de captura ao vivo:
// wf-recorder (vídeo → FIFO) e ffmpeg (FIFO + áudio → HLS), com restart
// com backoff caso algum caia antes do esperado.
type Pipeline struct {
	ffmpegPath   string
	recorderPath string
	ffmpegArgs   []string
	recorderArgs []string
	fifo         string
	outDir       string

	maxFFmpegRestarts   int
	maxRecorderRestarts int
}

// NewPipeline cria um supervisor. ffmpegArgs e recorderArgs são os args prontos
// (ex.: FFmpegArgs e WFRecorderArgs). fifo é o caminho da FIFO de vídeo.
func NewPipeline(ffmpegArgs, recorderArgs []string, fifo string) *Pipeline {
	return &Pipeline{
		ffmpegPath:          "ffmpeg",
		recorderPath:        "wf-recorder",
		ffmpegArgs:          ffmpegArgs,
		recorderArgs:        recorderArgs,
		fifo:                fifo,
		maxFFmpegRestarts:   3,
		maxRecorderRestarts: 5,
	}
}

// SetBinaries permite injetar caminhos (útil em testes).
func (p *Pipeline) SetBinaries(ffmpeg, recorder string) {
	p.ffmpegPath = ffmpeg
	p.recorderPath = recorder
}

// Start valida pré-requisitos e lança ambos os processos.
func (p *Pipeline) Start(ctx context.Context) error {
	if _, err := exec.LookPath(p.ffmpegPath); err != nil {
		return fmt.Errorf("ffmpeg não encontrado: %w", err)
	}
	if _, err := exec.LookPath(p.recorderPath); err != nil {
		return fmt.Errorf("wf-recorder não encontrado: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p.fifo), 0o755); err != nil {
		return err
	}
	_ = os.Remove(p.fifo)
	if err := syscall.Mkfifo(p.fifo, 0o600); err != nil {
		return fmt.Errorf("criar fifo %s: %w", p.fifo, err)
	}

	recorder := exec.CommandContext(ctx, p.recorderPath, p.recorderArgs...)
	if err := recorder.Start(); err != nil {
		return fmt.Errorf("iniciar wf-recorder: %w", err)
	}
	ffmpeg := exec.CommandContext(ctx, p.ffmpegPath, p.ffmpegArgs...)
	if err := ffmpeg.Start(); err != nil {
		_ = recorder.Process.Kill()
		return fmt.Errorf("iniciar ffmpeg: %w", err)
	}

	go p.watch(ctx, recorder, p.recorderArgs, p.maxRecorderRestarts, p.recorderPath, nil)
	go p.watch(ctx, ffmpeg, p.ffmpegArgs, p.maxFFmpegRestarts, p.ffmpegPath, nil)
	return nil
}

// watch reinicia o processo até o limite ou até o ctx ser cancelado.
func (p *Pipeline) watch(ctx context.Context, cmd *exec.Cmd, args []string, maxRestarts int, name string, errCh chan<- error) {
	backoff := 400 * time.Millisecond
	restarts := 0
	for {
		err := cmd.Wait()
		if ctx.Err() != nil {
			return
		}
		if restarts >= maxRestarts {
			if errCh != nil {
				errCh <- fmt.Errorf("%s encerrou inesperadamente após %d restarts: %v", name, maxRestarts, err)
			}
			return
		}
		restarts++
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		cmd = exec.CommandContext(ctx, name, args...)
		if startErr := cmd.Start(); startErr != nil {
			if ctx.Err() != nil {
				return
			}
			if errCh != nil {
				errCh <- fmt.Errorf("restart %s: %v", name, startErr)
			}
			return
		}
	}
}

// Stop encerra os processos (via cancelamento do ctx) e remove a FIFO.
func (p *Pipeline) Stop() {
	_ = os.Remove(p.fifo)
}
