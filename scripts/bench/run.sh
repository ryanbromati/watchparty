#!/usr/bin/env bash
# Executa um cenário de benchmark do watchparty e salva resultados em scripts/bench/results/.
# Uso: run.sh TRANSPORT QUALITY AGENTS SECONDS OUTPREFIX [PASSWORD]
set -u

T=$1; Q=$2; A=$3; S=$4; OUT=$5; PW=${6:-benchpw}
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
mkdir -p scripts/bench/results
FULL="scripts/bench/results/$OUT"

# --- limpeza do ambiente (só por nome de processo) ---
for pid in $(ps -eo pid,comm | awk '$2=="watchparty"||$2=="ffmpeg"||$2=="wf-recorder"||$2=="cloudflared"{print $1}'); do kill -9 "$pid" 2>/dev/null; done
rm -rf /tmp/watchparty-*

LOG=/tmp/bench-run-$OUT.log
./bin/watchparty share -transport "$T" -quality "$Q" -max-viewers 8 -password "$PW" -room bench >"$LOG" 2>&1 &
WPP=$!

URL=""
for i in $(seq 1 75); do
  URL=$(grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com|http://127\.0\.0\.1:[0-9]+' "$LOG" | head -n1)
  [ -n "$URL" ] && break
  sleep 1
done
if [ -z "$URL" ]; then
  echo "FALHA: sem URL em $T/$Q"; cat "$LOG"
  kill "$WPP" 2>/dev/null; exit 1
fi
echo "URL=$URL"
# espera o túnel/live assentar (evita login no cold start)
sleep 12

# amostras do host no meio do teste
METRIC_SAMPLES=$((S / 3)); [ "$METRIC_SAMPLES" -lt 4 ] && METRIC_SAMPLES=4
(
  sleep "$METRIC_SAMPLES"
  for i in 1 2 3; do
    echo "--- sample $i ---"
    ps -eo pid,comm,%cpu,%mem,args | awk '$2=="ffmpeg"||$2=="wf-recorder"'
    nvidia-smi --query-gpu=utilization.gpu,utilization.encoder,memory.used --format=csv,noheader
    sleep "$METRIC_SAMPLES"
  done
) > "$FULL.host.txt" &
SAMPLER=$!

./scripts/bench/viewer -url "$URL" -password "$PW" -agents "$A" -seconds "$S" > "$FULL.csv"

echo "PREFIX=$OUT TRANSPORT=$T QUALITY=$Q AGENTS=$A SECONDS=$S URL=$URL" >> "$FULL.host.txt"
kill "$SAMPLER" 2>/dev/null
kill "$WPP" 2>/dev/null
for pid in $(ps -eo pid,comm | awk '$2=="watchparty"||$2=="ffmpeg"||$2=="wf-recorder"||$2=="cloudflared"{print $1}'); do kill -9 "$pid" 2>/dev/null; done
rm -rf /tmp/watchparty-*

# --- resumo ---
{
  echo "== $OUT  ($T/$Q, $A agentes, ${S}s) =="
  echo "linhas: $(wc -l < "$FULL.csv")"
  echo "stall_count: $(awk -F, '$7==1{n++} END{print n+0}' "$FULL.csv")"
  echo "segFetchMs p50/p95/max: $(awk -F, '$5>=0{print $5}' "$FULL.csv" | sort -n | awk '{a[NR]=$1} END{n=NR; if(n)print a[int(n*0.5)+1]"/"a[int(n*0.95)+1]"/"a[n]; else print "-"}' | tr '\n' ' ')"
  echo "statsRttMs p50: $(awk -F, '$6>=0{print $6}' "$FULL.csv" | sort -n | awk '{a[NR]=$1} END{n=NR; print (n?a[int(n*0.5)+1]:-1)}')"
  echo "seq primeiro/último: $(awk -F, '$3>=0{n++; if(n==1)f=$3; l=$3} END{print f"/"l}' "$FULL.csv")"
  echo "spread ultimo segIdx entre agentes: $(awk -F, '$4 ~ /^[0-9]+$/ && $4>=0 {if(min==""||$4<min)min=$4; if($4>max)max=$4} END{print (min!=""?max-min:"-")}' "$FULL.csv")"
} > "$FULL.summary.txt"
cat "$FULL.summary.txt"
echo "done: $FULL.csv"
