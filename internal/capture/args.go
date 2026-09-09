// Package capture monta e supervisiona o pipeline de captura:
// wf-recorder (vídeo da tela via FIFO) + ffmpeg (áudio do monitor + encode NVENC → HLS).
package capture

import (
	"fmt"
	"os/exec"
	"strings"
)

// Source descreve uma entrada do ffmpeg.
type Source struct {
	Format string // ex.: "mpegts" (wf-recorder) ou "pulse" (monitor)
	URI    string // ex.: caminho da FIFO ou nome da fonte do monitor
}

// WFRecorderArgs devolve os args do wf-recorder para gravar só vídeo,
// em framerate constante, muxado como mpegts, na FIFO.
func WFRecorderArgs(fifo string, fps string) []string {
	return []string{"-y", "-r", fps, "-m", "mpegts", "-f", fifo}
}

// FFmpegArgs devolve os args do ffmpeg para o pipeline ao vivo:
// decodifica vídeo (fifo mpegts) e áudio (pulse monitor) e re-encodeia
// h264_nvenc + aac em HLS (segmentos 2s, janela ~16s).
// Se audio for nil, o pipeline fica sem áudio.
func FFmpegArgs(video Source, audio *Source, vbKbps int, segPattern, hlsIndex string) []string {
	args := []string{
		"-hide_banner", "-loglevel", "warning",
		"-analyzeduration", "2000000", "-probesize", "20M",
		"-f", video.Format, "-i", video.URI,
	}
	if audio != nil {
		args = append(args, "-f", audio.Format, "-i", audio.URI)
	}
	args = append(args, "-map", "0:v:0")
	if audio != nil {
		args = append(args, "-map", "1:a:0")
	}
	args = append(args,
		"-c:v", "h264_nvenc", "-preset", "p4", "-rc", "vbr", "-cq", "23",
		"-b:v", fmt.Sprintf("%dk", vbKbps),
		"-maxrate", fmt.Sprintf("%dk", vbKbps*4/3),
		"-bufsize", fmt.Sprintf("%dk", vbKbps*2),
		"-g", "60", "-keyint_min", "60", "-pix_fmt", "yuv420p",
	)
	if audio != nil {
		args = append(args,
			"-c:a", "aac", "-b:a", "192k", "-ac", "2", "-ar", "48000",
			"-af", "aresample=async=1:first_pts=0",
		)
	}
	args = append(args,
		"-f", "hls",
		"-hls_time", "2",
		"-hls_list_size", "8",
		"-hls_flags", "delete_segments+independent_segments+temp_file",
		"-hls_segment_filename", segPattern,
		hlsIndex,
	)
	return args
}

// DetectAudioSource descobre o monitor do sink padrão via `pactl`.
func DetectAudioSource() (string, error) {
	out, err := exec.Command("pactl", "list", "short", "sources").Output()
	if err != nil {
		return "", fmt.Errorf("falha ao executar pactl: %w", err)
	}
	src := parseMonitorSource(string(out))
	if src == "" {
		return "", fmt.Errorf("nenhuma fonte .monitor encontrada em pactl list short sources")
	}
	return src, nil
}

// parseMonitorSource extrai o primeiro nome de fonte terminado em ".monitor".
func parseMonitorSource(output string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasSuffix(fields[1], ".monitor") {
			return fields[1]
		}
	}
	return ""
}
