package capture

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFFmpegArgsGeneratesLiveHLS roda o pipeline de encode HLS de ponta a ponta
// com fontes sintéticas (lavfi) — não exige sessão gráfica. Valida que nossos
// args produzem playlist + segmentos com áudio e vídeo.
func TestFFmpegArgsGeneratesLiveHLS(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg ausente")
	}
	dir := t.TempDir()
	video := Source{Format: "lavfi", URI: "testsrc2=size=640x360:rate=30:duration=9"}
	audio := Source{Format: "lavfi", URI: "sine=frequency=440:sample_rate=48000:duration=9"}
	args := FFmpegArgs(video, &audio, 2000, 0,
		filepath.Join(dir, "seg_%05d.ts"),
		filepath.Join(dir, "index.m3u8"))

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// NVENC indisponível ou encoder ausente: pula em vez de falhar o repo.
		if strings.Contains(stderr.String(), "nvenc") {
			t.Skipf("h264_nvenc indisponível: %v", err)
		}
		t.Fatalf("ffmpeg: %v\n%s", err, stderr.String())
	}

	idx := filepath.Join(dir, "index.m3u8")
	data, err := os.ReadFile(idx)
	if err != nil {
		t.Fatalf("ler playlist: %v", err)
	}
	playlist := string(data)
	segs := 0
	for _, line := range strings.Split(playlist, "\n") {
		if strings.HasPrefix(line, "#EXTINF:") {
			segs++
		}
	}
	if segs < 2 {
		t.Fatalf("esperava >=2 segmentos na playlist, achei %d\n%s", segs, playlist)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	ts := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ts") {
			ts++
		}
	}
	if ts < 2 {
		t.Fatalf("esperava >=2 arquivos .ts, achei %d", ts)
	}
}
