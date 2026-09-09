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

// TestScreenToHLS roda a captura REAL (wf-recorder + áudio do monitor → HLS).
// Requer sessão Wayland ativa e é controlado por env var para não rodar no
// `go test` normal: WATCHPARTY_SCREEN_TEST=1 go test ./internal/capture/ -run TestScreenToHLS
func TestScreenToHLS(t *testing.T) {
	if os.Getenv("WATCHPARTY_SCREEN_TEST") != "1" {
		t.Skip("defina WATCHPARTY_SCREEN_TEST=1 para rodar a captura de tela real")
	}
	for _, bin := range []string{"ffmpeg", "wf-recorder", "pactl"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s ausente", bin)
		}
	}
	audio, err := DetectAudioSource()
	if err != nil {
		t.Skipf("sem monitor de áudio: %v", err)
	}

	dir := t.TempDir()
	fifo := filepath.Join(dir, "cap.ts")
	args := FFmpegArgs(
		Source{Format: "mpegts", URI: fifo},
		&Source{Format: "pulse", URI: audio},
		3000,
		filepath.Join(dir, "seg_%05d.ts"),
		filepath.Join(dir, "index.m3u8"),
	)
	p := NewPipeline(args, WFRecorderArgs(fifo, "30"), fifo)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop()

	deadline := time.Now().Add(15 * time.Second)
	idx := filepath.Join(dir, "index.m3u8")
	for {
		if data, err := os.ReadFile(idx); err == nil && strings.Count(string(data), "#EXTINF:") >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout esperando segmentos HLS na playlist")
		}
		time.Sleep(300 * time.Millisecond)
	}
	data, err := os.ReadFile(idx)
	if err != nil {
		t.Fatalf("ler playlist: %v", err)
	}
	if strings.Count(string(data), "#EXTINF:") < 2 {
		t.Fatalf("playlist sem segmentos suficientes:\n%s", data)
	}
}

func countSegs(entries []os.DirEntry) int {
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ts") {
			n++
		}
	}
	return n
}
