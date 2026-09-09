// Package session orquestra uma sessão de compartilhamento: captura ao vivo
// (wf-recorder + ffmpeg → HLS) + servidor HTTP local (viewer + segmentos).
package session

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ryanbromati/watchparty/internal/capture"
	"github.com/ryanbromati/watchparty/internal/config"
	"github.com/ryanbromati/watchparty/internal/webui"
)

// Session representa uma sessão ativa.
type Session struct {
	opts   config.Options
	hlsDir string
	srv    *http.Server
	ln     net.Listener
	cancel context.CancelFunc
	pipe   *capture.Pipeline
}

// Start sobe a captura e o servidor HTTP local.
func Start(parent context.Context, opts config.Options) (*Session, error) {
	if opts.Audio == "" {
		src, err := capture.DetectAudioSource()
		if err != nil {
			return nil, fmt.Errorf("detectar fonte de áudio: %w", err)
		}
		opts.Audio = src
	}

	ctx, cancel := context.WithCancel(parent)
	hlsDir, err := os.MkdirTemp("", "watchparty-")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("criar dir temporário: %w", err)
	}

	fifo := filepath.Join(hlsDir, "cap.ts")
	ffArgs := capture.FFmpegArgs(
		capture.Source{Format: "mpegts", URI: fifo},
		&capture.Source{Format: "pulse", URI: opts.Audio},
		opts.VideoBitrateKbps(),
		filepath.Join(hlsDir, "seg_%05d.ts"),
		filepath.Join(hlsDir, "index.m3u8"),
	)
	pipe := capture.NewPipeline(ffArgs, capture.WFRecorderArgs(fifo, "30"), fifo)
	if err := pipe.Start(ctx); err != nil {
		cancel()
		_ = os.RemoveAll(hlsDir)
		return nil, err
	}

	handler, err := newHandler(hlsDir)
	if err != nil {
		cancel()
		pipe.Stop()
		_ = os.RemoveAll(hlsDir)
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		pipe.Stop()
		_ = os.RemoveAll(hlsDir)
		return nil, fmt.Errorf("ouvir porta local: %w", err)
	}
	srv := &http.Server{Handler: handler}
	go func() { _ = srv.Serve(ln) }()

	return &Session{
		opts:   opts,
		hlsDir: hlsDir,
		srv:    srv,
		ln:     ln,
		cancel: cancel,
		pipe:   pipe,
	}, nil
}

// URL retorna a URL local do viewer (ex.: http://127.0.0.1:PORT).
func (s *Session) URL() string {
	return "http://" + s.ln.Addr().String()
}

// Room retorna o slug da sala.
func (s *Session) Room() string {
	return s.opts.Room
}

// AudioSource retorna a fonte de áudio resolvida (monitor do sink).
func (s *Session) AudioSource() string {
	return s.opts.Audio
}

// Close encerra servidor, processos de captura e remove temporários.
func (s *Session) Close() {
	_ = s.srv.Close()
	s.cancel()
	s.pipe.Stop()
	_ = os.RemoveAll(s.hlsDir)
}

// newHandler monta as rotas: /live/* serve o HLS, / serve o viewer embutido.
func newHandler(hlsDir string) (http.Handler, error) {
	dist, err := webui.Dist()
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/live/", http.StripPrefix("/live/", http.FileServer(http.Dir(hlsDir))))
	mux.Handle("/", http.FileServer(http.FS(dist)))
	return mux, nil
}
