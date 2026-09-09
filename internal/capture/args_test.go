package capture

import (
	"reflect"
	"strings"
	"testing"
)

func TestFFmpegArgsVideoAudio(t *testing.T) {
	audio := &Source{Format: "pulse", URI: "alsa_output.x.monitor"}
	got := FFmpegArgs(
		Source{Format: "mpegts", URI: "/tmp/fifo.ts"},
		audio,
		6000,
		720,
		"/tmp/hls/seg_%05d.ts",
		"/tmp/hls/index.m3u8",
	)
	want := []string{
		"-hide_banner", "-loglevel", "warning",
		"-analyzeduration", "2000000", "-probesize", "20M",
		"-f", "mpegts", "-i", "/tmp/fifo.ts",
		"-f", "pulse", "-i", "alsa_output.x.monitor",
		"-map", "0:v:0", "-map", "1:a:0",
		"-vf", "scale=-2:720",
		"-c:v", "h264_nvenc", "-preset", "p4", "-rc", "vbr", "-cq", "23",
		"-rc-lookahead", "20", "-multipass", "2",
		"-b:v", "6000k", "-maxrate", "8000k", "-bufsize", "12000k",
		"-g", "60", "-keyint_min", "60", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "192k", "-ac", "2", "-ar", "48000",
		"-af", "aresample=async=1:first_pts=0",
		"-f", "hls", "-hls_time", "2", "-hls_list_size", "6",
		"-hls_flags", "delete_segments+independent_segments+temp_file",
		"-hls_segment_filename", "/tmp/hls/seg_%05d.ts",
		"/tmp/hls/index.m3u8",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args divergem:\n got: %q\nwant: %q", got, want)
	}
}

func TestFFmpegArgsVideoOnly(t *testing.T) {
	got := FFmpegArgs(Source{Format: "mpegts", URI: "/f"}, nil, 3000, 0, "/o/seg_%05d.ts", "/o/index.m3u8")
	joined := strings.Join(got, " ")
	for _, frag := range []string{"-f pulse", "-map 1:a:0", "-c:a aac", "aresample"} {
		if strings.Contains(joined, frag) {
			t.Fatalf("pipeline sem áudio não deveria conter %q\nargs: %s", frag, joined)
		}
	}
	if !strings.Contains(joined, "-b:v 3000k") {
		t.Fatalf("bitrate de vídeo ausente\nargs: %s", joined)
	}
}

func TestWFRecorderArgs(t *testing.T) {
	got := WFRecorderArgs("/tmp/f.ts", "30")
	want := []string{"-y", "-r", "30", "-c", "libx264", "-p", "preset=ultrafast", "-m", "mpegts", "-f", "/tmp/f.ts"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParseMonitorSource(t *testing.T) {
	out := "100\talsa_output.usb_X.monitor\tPipeWire\ts32le 2ch 48000Hz\tIDLE\n" +
		"101\talsa_input.usb_X.mic\tPipeWire\ts32le 2ch 48000Hz\tIDLE\n"
	if got := parseMonitorSource(out); got != "alsa_output.usb_X.monitor" {
		t.Fatalf("got %q", got)
	}
	if got := parseMonitorSource(""); got != "" {
		t.Fatalf("esperava vazio, got %q", got)
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
