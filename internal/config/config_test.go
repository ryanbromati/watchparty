package config

import "testing"

func TestDefaults(t *testing.T) {
	o, err := ParseFlags(nil)
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if o.Transport != TransportTailscale {
		t.Errorf("transport default = %q, want tailscale", o.Transport)
	}
	if o.MaxViewers != 3 {
		t.Errorf("max-viewers default = %d, want 3", o.MaxViewers)
	}
	if o.Quality != Quality1080p {
		t.Errorf("quality default = %q, want 1080p", o.Quality)
	}
	if o.PasswordExplicit {
		t.Error("password não deveria estar marcada como explícita sem -password")
	}
	// Sem -password e sem terminal, senha fica vazia (sem senha é permitido).
	if o.Password != "" {
		t.Errorf("password default = %q, want vazio", o.Password)
	}
	if o.Room == "" {
		t.Error("room deve ser auto-gerada")
	}
}

func TestProvidedValuesAreKept(t *testing.T) {
	o, err := ParseFlags([]string{"-transport", "local", "-password", "minha-senha", "-room", "sala1", "-max-viewers", "2", "-quality", "720p"})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if o.Transport != TransportLocal {
		t.Errorf("transport = %q", o.Transport)
	}
	if o.Password != "minha-senha" {
		t.Errorf("password = %q", o.Password)
	}
	if !o.PasswordExplicit {
		t.Error("password deveria estar marcada como explícita")
	}
	if o.Room != "sala1" {
		t.Errorf("room = %q", o.Room)
	}
	if o.MaxViewers != 2 {
		t.Errorf("max-viewers = %d", o.MaxViewers)
	}
	if o.Quality != Quality720p {
		t.Errorf("quality = %q", o.Quality)
	}
}

func TestEmptyPasswordAllowed(t *testing.T) {
	o, err := ParseFlags([]string{"-transport", "cloudflare"})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if o.Password != "" {
		t.Errorf("senha vazia deveria ser aceita, got %q", o.Password)
	}
}

func TestInvalidTransport(t *testing.T) {
	if _, err := ParseFlags([]string{"-transport", "ftp"}); err == nil {
		t.Fatal("esperava erro para transporte inválido")
	}
}

func TestInvalidQuality(t *testing.T) {
	if _, err := ParseFlags([]string{"-quality", "4k"}); err == nil {
		t.Fatal("esperava erro para qualidade inválida")
	}
}

func TestInvalidMaxViewers(t *testing.T) {
	if _, err := ParseFlags([]string{"-max-viewers", "0"}); err == nil {
		t.Fatal("esperava erro para max-viewers 0")
	}
}

func TestUnexpectedArgs(t *testing.T) {
	if _, err := ParseFlags([]string{"extraneous"}); err == nil {
		t.Fatal("esperava erro para argumento não-flag")
	}
}

func TestVideoBitrate(t *testing.T) {
	if got := (Options{Quality: Quality1080p}).VideoBitrateKbps(); got != 6000 {
		t.Errorf("1080p bitrate = %d, want 6000", got)
	}
	if got := (Options{Quality: Quality720p}).VideoBitrateKbps(); got != 3000 {
		t.Errorf("720p bitrate = %d, want 3000", got)
	}
}

func TestGenerateSlugLength(t *testing.T) {
	s, err := GenerateSlug()
	if err != nil {
		t.Fatalf("GenerateSlug: %v", err)
	}
	if len(s) != 6 {
		t.Errorf("slug length = %d, want 6", len(s))
	}
}
