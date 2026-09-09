# WatchParty — Design

Data: 2026-09-08
Status: Aprovado para implementação

## Objetivo

Aplicação de terminal (Arch Linux, hoje) que captura a **tela inteira + áudio do
sistema** do host, transmite ao vivo via **HLS** por um **túnel gratuito** e
imprime **URL + senha**. Até **3 espectadores** abrem a URL em qualquer browser
(sem instalar nada), digitam a senha e assistem **sincronizados**. Uso real:
ver filmes/séries com amigos; voz por aplicativo externo (Discord etc.).

Requisito-chave: acesso de qualquer lugar na internet ("fora da rede").
**CGNAT é irrelevante por construção** porque o host sempre conecta para fora
(relay/túnel outbound); nenhuma conexão de entrada é necessária.

## Decisões fechadas

| Tema | Decisão |
|---|---|
| Host MVP | Arch Linux + Hyprland (Wayland). Outros Linux/Windows em fase futura. |
| Linguagens | Go (núcleo + CLI + servidor) · ffmpeg (captura/encode, subprocesso) · TypeScript (viewer web embutido via `go:embed`). |
| Entrega | HLS/HTTP unidirecional. **Sem WebRTC** (túneis são TCP; HLS atende filme). |
| Transporte | `tailscale` (padrão) · `cloudflare` (fallback) · `local` (teste) — trocável por flag. |
| Qualidade | NVENC H.264; padrão 1080p (~6 Mbps); primeiro teste real em 720p (~3 Mbps). |
| Áudio | Monitor do sink PipeWire do host (som que toca), AAC 192k 48kHz, downmix estéreo. |
| Segurança | Senha (tempo constante) → cookie HttpOnly; mídia protegida. |
| Presença | Heartbeat GET (não SSE) → contagem 0/3 no terminal e na página. |
| Não-escopo v1 | Voz no app, controle remoto, WebRTC, múltiplos monitores, janela/região, multicast ABR. |

## Arquitetura

```
cmd/watchparty/main.go      CLI: flags + ciclo de vida da sessão
internal/config/            flags: --transport, --password, --quality, --max-viewers, --room
internal/capture/           Capturer (tela+áudio) → subprocesso ffmpeg → HLS
internal/stream/            serve /live/*, GC de segmentos, estado "ao vivo"
internal/auth/              senha (tempo constante), sessões, rate-limit
internal/presence/          heartbeat dos espectadores → contagem
internal/transport/         interface Transport + local / tailscale / cloudflare
web/                        viewer TS (gate senha + hls.js + <video>) → web/dist (go:embed)
```

Interface `Transport`:

```go
type Transport interface {
    Start(ctx context.Context, localPort int) (publicURL string, err error)
}
```

Interface `Capturer` (esconde o backend de captura, decidido na Fase 0):

```go
type Capturer interface {
    // Inicia o pipeline de captura/encode; o HLS sai em outDir.
    Start(ctx context.Context, outDir string) error
}
```

Fluxo da sessão (`watchparty share`):

1. Valida pré-requisitos (ffmpeg, transporte escolhido).
2. Sobe servidor HTTP em `127.0.0.1:<porta efêmera>`.
3. Inicia ffmpeg → HLS em diretório temporário.
4. Sobe o transporte → URL pública.
5. Imprime:

```
🔒 watchparty · sala ativa
URL  : https://pc.seutailnet.ts.net/w/ab3f
Senha: bananeira-azul-42
Ver  : 0/3  ·  NVENC 1080p @ 6Mbps  ·  Ctrl+C encerra
```

## Pipeline de mídia

- **Áudio (decidido):** ffmpeg `-f pulse -i alsa_output.usb-...CX31993....monitor`
  (compat PipeWire do host). AAC 192k, 48 kHz, estéreo.
- **Vídeo (spike Fase 0 decide):** o ffmpeg do Arch **não é compilado com o
  dispositivo `pipewire`** e o Hyprland não é wlroots; portanto **não há captura
  turnkey**. Candidatos:
  1. Helper nativo portal (`xdg-desktop-portal-hyprland`) + PipeWire →
     rawvideo para o stdin do ffmpeg (mais "terminal-puro"; exige libpipewire/cgo).
  2. OBS (portal-hyprland) → virtual cam `v4l2loopback` → ffmpeg `-f v4l2`
     (caminho que o ecossistema Hyprland usa; adiciona OBS + módulo de kernel).
  3. GStreamer (`pipewiresrc` + portal) → fds para o ffmpeg.
- Encode (comum a qualquer backend): `h264_nvenc`, preset `p4`, `rc-lookahead`,
  ~3 Mbps (720p) / ~6 Mbps (1080p), keyframe a cada 2s.
- **Anti-drift A/V (filme de ~2h):** `-use_wallclock_as_timestamps` +
  `aresample=async=1:first_pts=0`; validar num filme inteiro.
- **HLS robusto:** segmentos 2s, janela 12–16s (`hls_list_size`),
  `#EXT-X-INDEPENDENT-SEGMENTS`; hls.js com `liveSyncDurationCount` folgado
  (sem `lowLatencyMode` agressivo). Latência resultante ~4–8s — aceitável; os
  3 espectadores recebem da mesma origem e ficam sincronizados entre si.

## Segurança e sessão

- Página pública pede senha; comparação em tempo constante (`crypto/subtle`).
- Rate-limit de tentativas (~5/min) com backoff; lockout após N falhas.
- Senha correta → cookie HttpOnly + SameSite curto; `/live/*` (playlist e
  segmentos) exige cookie — sem isso a mídia vazaria.
- `Secure` no cookie quando o transporte é HTTPS; desligado em `local` (http).
- Limite: com `max-viewers` ativos, nova entrada → "sala cheia" (429).
- Ctrl+C encerra sessão: derruba ffmpeg e túnel; viewer mostra "transmissão encerrada".

## Presença

Viewer envia GET de heartbeat a cada ~10s; host conta sessões ativas nos
últimos 15s; terminal e página exibem a contagem. GET simples atravessa
qualquer túnel (Cloudflare quick bloqueia SSE — por isso não se usa SSE).

## Transportes

- `tailscale` (padrão): requer `tailscale` instalado e `tailscale up`. O app
  executa `tailscale funnel --bg <porta>` e obtém a URL via
  `tailscale funnel status --json`. Erro claro se ausente/não autenticado.
- `cloudflare` (fallback): executa `cloudflared tunnel --url
  http://127.0.0.1:<porta>` e parseia a URL `https://*.trycloudflare.com` do log.
- `local` (teste): imprime `http://127.0.0.1:<porta>`.
- Nota: Funnel exige portas `443/8443/10000` para o lado externo; o backend
  local continua em porta efêmera (reverse proxy `http://127.0.0.1`).

## Robustez e erros

- ffmpeg caiu → restart com backoff; falha repetida → mensagem acionável
  (ex.: portal negado).
- Portal negado/usuário recusou captura → sair com dica clara.
- Disco temporário com limite (ffmpeg `delete_segments` + teto de ~30 MB).
- Túnel caiu → reiniciar; falha do host derruba a sessão (esperado do modelo).
- Erros de rede no viewer → aviso + reconexão (hls.js).

## Testes e verificação

- Unit (Go): auth (tempo constante, rate-limit, expiração), parser da URL do
  cloudflared, presença (janela 15s), flags/config, GC de segmentos.
- Smoke local: `--transport local`; Playwright abre a página, digita senha e
  valida que o `<video>` decodifica — sem depender de internet.
- Comandos: `go test ./...`, `go vet`, build do `web/`, `make build`.

## Critérios de aceite (produto)

`watchparty share` → URL + senha na tela → 3 amigos abrem, digitam a senha e
assistem **sincronizados** por um filme inteiro, sem stall nem dessincronização
A/V; terminal mostra "0/3" atualizado.

## Riscos e mitigação

| Risco | Severidade | Mitigação |
|---|---|---|
| Banda do túnel desconhecida (Funnel beta, limite não publicado) | Alta | Transporte trocável; teste real ≥30 min com 3 espectadores; bitrate configurável |
| Cloudflare quick = só teste (sem SLA, 200 req, sem SSE) | Média | Funnel padrão; CF apenas fallback |
| Drift A/V em filme longo | Alta | `wallclock` + `aresample async=1`; validar filme inteiro |
| Stall no live edge do hls.js | Média | Janela 12–16s, sem LL-HLS agressivo |
| Captura pede confirmação na tela (portal) por sessão | Baixa | Aceitar 1 clique; documentar |
| ffmpeg sem `-f pipewire` + Hyprland ≠ wlroots | Alta | Spike Fase 0 decide backend de captura antes de mais código |
