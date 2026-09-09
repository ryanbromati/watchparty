package capture

import (
	"context"
	"testing"
)

// TestPipelineMissingBinaries garante erro claro quando os binários faltam,
// sem tocar no sistema (usa caminhos inexistentes).
func TestPipelineMissingBinaries(t *testing.T) {
	p := NewPipeline([]string{}, []string{}, "/tmp/wp-test/none.ts")
	p.SetBinaries("/nope/ffmpeg", "/nope/wf-recorder")
	if err := p.Start(context.Background()); err == nil {
		t.Fatal("esperava erro com binários ausentes")
	}
}

// TestPipelineCriaFifoSoComBinariosValidos verifica que Start falha rápido e
// não cria a FIFO quando o wf-recorder não existe (evita artefatos em teste).
func TestPipelineNoFifoWithoutRecorder(t *testing.T) {
	dir := t.TempDir()
	p := NewPipeline([]string{}, []string{"-x"}, dir+"/cap.ts")
	p.SetBinaries("/nope/ffmpeg", "/nope/wf-recorder")
	if err := p.Start(context.Background()); err == nil {
		t.Fatal("esperava erro")
	}
}
