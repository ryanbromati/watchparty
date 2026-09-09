# watchparty

Compartilhe a tela e o áudio do seu computador com até **N pessoas** (padrão 3)
através de um link + senha, assistindo **ao vivo e sincronizados** no navegador —
sem instalar nada no lado de quem assiste.

Feito para "assistir filme/série com amigos": você roda o app no terminal, o app
captura a tela inteira + o som que está tocando, transmite via HLS por um túnel
gratuito e imprime a URL pública e a senha.

## Como funciona (resumo)

```
wf-recorder (tela → FIFO)  ─┐
                            ├─ ffmpeg (NVENC H.264 + AAC) → HLS → servidor HTTP
ffmpeg -f pulse (áudio)   ─┘        │        │
                                    │        └─ viewer web (hls.js) com senha
                                    └─ túnel: local | tailscale funnel | cloudflare
```

- **Captura:** `wf-recorder` (vídeo, funciona em Hyprland/Wayland via wlr-screencopy)
  + ffmpeg `-f pulse` (áudio do monitor) — não usa microfone.
- **Encode:** `h264_nvenc` (GPU NVIDIA) → HLS (segmentos 2s).
- **Acesso externo:** sem entrada de rede — o app conecta para fora (túnel),
  então **CGNAT não impede** o acesso.
- **Segurança:** senha com comparação em tempo constante, cookie HttpOnly,
  rate-limit por IP e limite de espectadores; `/live/*` só com sessão.

## Requisitos (host)

- Go 1.22+ (build), Node/npm (build do viewer).
- `ffmpeg` com `h264_nvenc` e demuxer `pulse` (PipeWire).
- `wf-recorder` (captura de tela no Wayland).
- Para acesso público: `tailscale` (recomendado) **ou** `cloudflared`; senão use
  `-transport local` (só sua rede/LAN).

Instalação no Arch: `sudo pacman -S ffmpeg wf-recorder cloudflared` (ou `tailscale`).

## Build e uso

```sh
make web     # compila o viewer para internal/webui/dist (embutido no binário)
make build   # gera ./bin/watchparty

make run     # build + watchparty + Cloudflare (pergunta a senha; Enter = sem senha)
make run-local  # mesmo app, sem túnel (rede local)

# variáveis: make run TRANSPORT=tailscale QUALITY=1080p MAXVIEWERS=5

./bin/watchparty share -transport cloudflare -password minha-senha -room sala1
# URL pública e senha aparecem no terminal; compartilhe com os amigos.
```

A senha é sempre sua escolha: se você não passar `-password`, o app pergunta no
terminal (com eco oculto). **Enter (duas vezes) = sala sem senha** — qualquer
pessoa com o link pode entrar; nesse caso o app avisa.

Flags do `share`:

| Flag | Padrão | Descrição |
|---|---|---|
| `-transport` | `tailscale` | `local` \| `tailscale` \| `cloudflare` |
| `-password` | perguntada | senha da sala (vazia = sem senha; use `-password ""`) |
| `-max-viewers` | `3` | limite de espectadores simultâneos |
| `-quality` | `1080p` | `720p` \| `1080p` — bitrate + escala real (720p gera 1280×720) |
| `-room` | auto-gerado | slug da sala no link |
| `-audio` | auto-detectada | fonte `.monitor` do áudio do sistema |

Comandos de desenvolvimento: `make test` (`go test ./...`), `make vet`, `make smoke`,
`make run`/`make run-local`. Benchmark reproduzível em `scripts/bench/` (`run.sh` +
`viewer.go`).

## Telemetria

- `GET /api/stats` (exige sessão): `{seq, lastMediaEnd, bitrateKbps, segmentDuration,
  viewers, maxViewers}` — usado no terminal e para testes.
- Abra a URL com `?debug` e o objeto hls.js fica exposto em `window.__hls`
  (ex.: `window.__hls.latency` = latência ao vivo em segundos).

## Benchmark

Metodologia, números e artefatos em `docs/superpowers/bench/2026-09-09-bench.md`
(0 stalls em local e Cloudflare, 720p e 1080p ×3; latência ~6.6s medida antes do
ajuste B1 — re-medição pós-B1 pendente, junto com o teste de campo).

## Notas

- **Limites de túnel:** quick tunnel do Cloudflare é "para teste" (URL efêmera,
  sem SLA); Tailscale Funnel é beta com banda limitada não documentada. Se o
  upload da casa não aguentar, baixe a qualidade. Teste com o público real antes
  de depender disso.
- **Drift A/V:** mitigado com `aresample=async=1:first_pts=0`; valide numa sessão
  longa (filme inteiro).
- Design e plano detalhados em `docs/superpowers/`.
