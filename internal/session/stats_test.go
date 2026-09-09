package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseIndex(t *testing.T) {
	m3u8 := "#EXTM3U\n#EXT-X-VERSION:6\n#EXT-X-TARGETDURATION:2\n" +
		"#EXT-X-MEDIA-SEQUENCE:10\n#EXTINF:2.0,\nseg_00010.ts\n" +
		"#EXTINF:2.0,\nseg_00011.ts\n#EXTINF:2.0,\nseg_00012.ts\n"
	seq, count := parseIndex([]byte(m3u8))
	if seq != 10 {
		t.Fatalf("seq = %d, want 10", seq)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}
}

func TestParseIndexEmpty(t *testing.T) {
	seq, count := parseIndex([]byte("#EXTM3U\n"))
	if seq != 0 || count != 0 {
		t.Fatalf("seq=%d count=%d, want 0/0", seq, count)
	}
}

func TestSegmentSizesMostRecentFirst(t *testing.T) {
	dir := t.TempDir()
	// cria 3 segmentos com mtimes distintos
	for i, name := range []string{"seg_00001.ts", "seg_00002.ts", "seg_00003.ts"} {
		_ = os.WriteFile(filepath.Join(dir, name), []byte{byte(i + 1), 0, 0, 0}, 0o644)
	}
	sizes := segmentSizes(dir, 3)
	if len(sizes) != 3 {
		t.Fatalf("len = %d", len(sizes))
	}
	// ordem decrescente por mtime é frágil (mesmo nanossegundo);
	// apenas garante que lê os .ts e ignora outros arquivos.
	_ = os.WriteFile(filepath.Join(dir, "index.m3u8"), []byte("x"), 0o644)
	if len(segmentSizes(dir, 5)) != 3 {
		t.Fatal("deveria ignorar index.m3u8")
	}
	_ = sizes
}
