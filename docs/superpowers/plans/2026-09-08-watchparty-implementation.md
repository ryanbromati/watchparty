# WatchParty — Implementation Plan

> **For agentic workers:** Implement task-by-task; steps use checkbox syntax. Plano vivo: cada tarefa concluída é marcada; código detalhado é entregue inline durante a execução com TDD e commits frequentes.
> **Base:** `docs/superpowers/specs/2026-09-08-watchparty-design.md`

**Goal:** App Go de terminal que captura tela+áudio (Arch/Hyprland), transmite HLS por túnel gratuito e imprime URL+senha para até 3 espectadores assistirem sincronizados no browser.

**Architecture:** Go binário único supervisiona dois subprocessos — `wf-recorder` (vídeo da tela via FIFO) e `ffmpeg` (áudio do monitor via `-f pulse`, re-encode NVENC → HLS). Servidor HTTP local serve viewer + segmentos com auth por senha. Transportes plugáveis (local/tailscale/cloudflare).

**Tech Stack:** Go, ffmpeg (n9+ com `h264_nvenc` e demuxer `pulse`), `wf-recorder`, TypeScript viewer com hls.js.

## Global Constraints

- Módulo Go: `github.com/ryanbromati/watchparty`.
- Host de referência: Arch + Hyprland, monitor único 1920x1080 (Wayland, `WAYLAND_DISPLAY=wayland-1`).
- Monitor de áudio (PipeWire/pulse): `alsa_output.usb-TTGK_Technology_Co._Ltd_CX31993_384Khz_HIFI_AUDIO-00.analog-stereo.monitor`.
- ffmpeg **sem** `-f pipewire`; captura de vídeo só via `wf-recorder`.
- Encode final: `h264_nvenc`, keyframe 2s, AAC 192k estéreo (48kHz). Anti-drift: `aresample=async=1:first_pts=0`.
- HLS: segmentos 2s, janela 12–16s.
- Viewer embutido via `go:embed web/dist` (built com TS/Node).
- Não usar SSE (Cloudflare quick bloqueia). Presença via GET heartbeat.
- Comandos de verificação: `go test ./...`, `go vet ./...`, build `web/`, `make build`.

---

## Fase 0 — Spike de captura [CONCLUÍDA]

- [x] Confirmar Hyprland suporta captura (wf-recorder OK — 1920x1080, sem portal).
- [x] Validar áudio: ffmpeg `-f pulse -i <monitor>` captura o som do sistema.
- [x] Validar cadeia completa: `wf-recorder -m mpegts -f <fifo>` (video) + `-f pulse` (áudio) → `h264_nvenc` → mp4 com A/V (exit 0, streams h264+aac).
- [x] Decisão de backend: **wf-recorder (video-only, FIFO mpegts) + ffmpeg (pulse monitor) + NVENC → HLS**.
- [x] Resíduo a validar na Fase 6: drift A/V em filme longo; fallback = wf-recorder A/V em arquivo real (não-live) ou OBS.

## Fase 1 — Esqueleto Go [CONCLUÍDA]

- [x] `go mod init github.com/ryanbromati/watchparty` + `cmd/watchparty/main.go`.
- [x] `internal/config`: flags `--transport`, `--password`, `--quality`, `--max-viewers`, `--room`, defaults + testes.
- [x] CLI `share` valida pré-requisitos e imprime banner; erro claro em transporte/qualidade inválidos.
- [x] `go vet ./...`, `go test ./...` verdes; commit.

## Fase 2 — Pipeline de mídia [CONCLUÍDA]

- [x] `internal/capture`: builder dos args do ffmpeg (testado) e do wf-recorder.
- [x] Detecção da fonte de áudio (pactl → `.monitor`, parser testado).
- [x] Supervisor de subprocessos com restart/backoff e remoção da FIFO.
- [x] Teste de integração headless (lavfi → HLS NVENC, playlist+segmentos).
- [x] Teste de captura real de tela (`WATCHPARTY_SCREEN_TEST=1`) → HLS ao vivo, PASS.
- [x] Commit.

## Fase 3 — Viewer web TS [CONCLUÍDA]

- [x] Scaffold `web/` (Vite + TS + hls.js), build → `internal/webui/dist`.
- [x] Viewer com gate de gesto (autoplay), estados ao vivo/encerrada, fallback HLS nativo iOS.
- [x] `internal/webui` (`go:embed`) e `internal/session` (captura + HTTP `/` + `/live/*`).
- [x] `watchparty share` sobe captura real → viewer + HLS ao vivo em `http://127.0.0.1:PORT` (smoke E2E PASS: index, playlist, segmentos, stop limpo).
- [x] Commit.

## Fase 4 — Auth + presença

- [ ] `internal/auth`: senha tempo constante, sessões+cookie, rate-limit (testes unitários).
- [ ] `internal/presence`: heartbeat GET → janela 15s → contagem (testes).
- [ ] Proteger `/live/*` e playlist com cookie.

## Fase 5 — Transportes

- [ ] `internal/transport`: interface + `local` (primeiro, usado no teste E2E).
- [ ] `tailscale` (executa `tailscale funnel --bg`, lê `status --json`).
- [ ] `cloudflare` (executa `cloudflared`, parseia URL do log; teste com log fake).

## Fase 6 — Teste real

- [ ] Sessão ≥30 min com 3 espectadores reais; validar stall, latência e drift A/V; decidir 720p vs 1080p e bitrate.

## Fase 7 — Polish

- [ ] Mensagens acionáveis (ffmpeg/wf-recorder ausentes, portal), README, `make build`, push inicial.
