// Package session orquestra uma sessão de compartilhamento: captura ao vivo
// (wf-recorder + ffmpeg → HLS) + servidor HTTP local (viewer, segmentos, auth).
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ryanbromati/watchparty/internal/auth"
	"github.com/ryanbromati/watchparty/internal/capture"
	"github.com/ryanbromati/watchparty/internal/config"
	"github.com/ryanbromati/watchparty/internal/presence"
	"github.com/ryanbromati/watchparty/internal/transport"
	"github.com/ryanbromati/watchparty/internal/webui"
)

const cookieName = "wp"

// Session representa uma sessão ativa.
type Session struct {
	opts   config.Options
	auth   *auth.Manager
	hub    *presence.Hub
	secure bool
	hlsDir string
	srv    *http.Server
	ln     net.Listener
	public string
	cancel context.CancelFunc
	pipe   *capture.Pipeline
}

// Start sobe a captura, o servidor HTTP e os mecanismos de auth/presença.
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

	s := &Session{
		opts:   opts,
		auth:   auth.New(opts.Password),
		hub:    presence.New(opts.MaxViewers),
		secure: opts.Transport != config.TransportLocal,
		hlsDir: hlsDir,
		ln:     nil,
		cancel: cancel,
		pipe:   pipe,
	}
	handler, err := s.newHandler()
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
	s.ln = ln
	s.srv = &http.Server{Handler: handler}
	go func() { _ = s.srv.Serve(ln) }()

	port := ln.Addr().(*net.TCPAddr).Port
	tr, err := transport.New(string(opts.Transport))
	if err != nil {
		cancel()
		pipe.Stop()
		_ = os.RemoveAll(hlsDir)
		return nil, err
	}
	pub, err := tr.Start(ctx, port)
	if err != nil {
		cancel()
		pipe.Stop()
		_ = os.RemoveAll(hlsDir)
		return nil, fmt.Errorf("abrir transporte %s: %w", opts.Transport, err)
	}
	s.public = pub
	return s, nil
}

// URL retorna a URL pública (túnel) ou a local se o transporte for local.
func (s *Session) URL() string {
	return s.public
}

// LocalURL retorna a URL local do viewer (ex.: http://127.0.0.1:PORT).
func (s *Session) LocalURL() string {
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

// Viewers retorna o número de espectadores ativos.
func (s *Session) Viewers() int {
	return s.hub.Count()
}

// MaxViewers retorna o limite de espectadores.
func (s *Session) MaxViewers() int {
	return s.hub.Max()
}

// Close encerra servidor, processos de captura e remove temporários.
func (s *Session) Close() {
	_ = s.srv.Close()
	s.cancel()
	s.pipe.Stop()
	_ = os.RemoveAll(s.hlsDir)
}

// newHandler monta as rotas:
//   /            viewer embutido
//   POST /api/login    autentica por senha e emite cookie
//   GET  /api/presence heartbeat (exige sessão) → contagem
//   /live/*      segmentos HLS (exige sessão)
func (s *Session) newHandler() (http.Handler, error) {
	dist, err := webui.Dist()
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/presence", s.handlePresence)
	mux.Handle("/live/", s.requireSession(http.StripPrefix("/live/", http.FileServer(http.Dir(s.hlsDir)))))
	mux.Handle("/", http.FileServer(http.FS(dist)))
	return mux, nil
}

func (s *Session) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}
	ip := clientIP(r)
	if !s.auth.Allowed(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "muitas tentativas; aguarde um minuto"})
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "json inválido"})
		return
	}
	if !s.auth.CheckPassword(body.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "senha incorreta"})
		return
	}
	s.auth.Reset(ip)
	if s.hub.Full() {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "sala cheia"})
		return
	}
	tok, err := s.auth.Issue()
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secure,
		MaxAge:   int((12 * 3600)), // sessão ~12h
	})
	s.hub.Beat(tok)
	writeJSON(w, http.StatusOK, map[string]int{"viewers": s.hub.Count(), "max": s.hub.Max()})
}

func (s *Session) handlePresence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}
	tok := readToken(r)
	if !s.auth.Valid(tok) {
		http.Error(w, "não autenticado", http.StatusUnauthorized)
		return
	}
	s.hub.Beat(tok)
	writeJSON(w, http.StatusOK, map[string]int{"viewers": s.hub.Count(), "max": s.hub.Max()})
}

// requireSession exige sessão válida antes de servir o handler interno.
func (s *Session) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := readToken(r)
		if !s.auth.Valid(tok) {
			http.Error(w, "não autenticado", http.StatusUnauthorized)
			return
		}
		s.hub.Beat(tok)
		next.ServeHTTP(w, r)
	})
}

func readToken(r *http.Request) string {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
