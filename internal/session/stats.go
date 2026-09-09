package session

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// stats resume o estado vivo do HLS para medição e monitoramento.
type stats struct {
	Seq             int `json:"seq"`             // EXT-X-MEDIA-SEQUENCE atual
	LastMediaEnd    int `json:"lastMediaEnd"`    // fim (em segundos de mídia) do último segmento
	BitrateKbps     int `json:"bitrateKbps"`     // bitrate médio estimado dos últimos segmentos
	SegmentDuration int `json:"segmentDuration"` // duração alvo do segmento (s)
	Viewers         int `json:"viewers"`
	MaxViewers      int `json:"maxViewers"`
}

// readStats monta as estatísticas a partir do dir HLS e da presença.
func (s *Session) readStats() (stats, error) {
	st := stats{
		SegmentDuration: 2,
		Viewers:         s.hub.Count(),
		MaxViewers:      s.hub.Max(),
	}
	data, err := os.ReadFile(filepath.Join(s.hlsDir, "index.m3u8"))
	if err != nil {
		return st, err
	}
	seq, count := parseIndex(data)
	st.Seq = seq
	st.LastMediaEnd = (seq + count) * st.SegmentDuration

	sizes := segmentSizes(s.hlsDir, 5)
	if n := len(sizes); n > 0 {
		var total int64
		for _, sz := range sizes {
			total += sz
		}
		bytesPerSec := float64(total) / float64(n*st.SegmentDuration)
		st.BitrateKbps = int(bytesPerSec * 8 / 1000)
	}
	return st, nil
}

// parseIndex extrai #EXT-X-MEDIA-SEQUENCE e conta segmentos (#EXTINF).
func parseIndex(data []byte) (seq, count int) {
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:") {
			if v, err := strconv.Atoi(strings.TrimPrefix(line, "#EXT-X-MEDIA-SEQUENCE:")); err == nil {
				seq = v
			}
		}
		if strings.HasPrefix(line, "#EXTINF:") {
			count++
		}
	}
	return seq, count
}

// segmentSizes devolve o tamanho dos últimos n segmentos .ts (por mtime).
func segmentSizes(dir string, n int) []int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ts") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	if len(files) == 0 {
		return nil
	}
	// mais recentes primeiro
	sort.Slice(files, func(i, j int) bool {
		mi, _ := os.Stat(files[i])
		mj, _ := os.Stat(files[j])
		return mi.ModTime().After(mj.ModTime())
	})
	if len(files) > n {
		files = files[:n]
	}
	out := make([]int64, 0, len(files))
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil {
			out = append(out, fi.Size())
		}
	}
	return out
}
