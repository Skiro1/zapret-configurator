package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{"valid both", Options{OutputDir: "/tmp", Engine: "both", Mode: "quick"}, false},
		{"valid zapret", Options{OutputDir: "/tmp", Engine: "zapret", Mode: "standard"}, false},
		{"valid zapret2", Options{OutputDir: "/tmp", Engine: "zapret2", Mode: "full"}, false},
		{"bad engine", Options{OutputDir: "/tmp", Engine: "invalid", Mode: "quick"}, true},
		{"bad mode", Options{OutputDir: "/tmp", Engine: "zapret", Mode: "invalid"}, true},
		{"empty output", Options{OutputDir: "", Engine: "zapret", Mode: "quick"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWantsZapret(t *testing.T) {
	if !(Options{Engine: "zapret"}).WantsZapret() {
		t.Error("zapret should want zapret")
	}
	if !(Options{Engine: "both"}).WantsZapret() {
		t.Error("both should want zapret")
	}
	if (Options{Engine: "zapret2"}).WantsZapret() {
		t.Error("zapret2 should not want zapret")
	}
}

func TestWantsZapret2(t *testing.T) {
	if !(Options{Engine: "zapret2"}).WantsZapret2() {
		t.Error("zapret2 should want zapret2")
	}
	if !(Options{Engine: "both"}).WantsZapret2() {
		t.Error("both should want zapret2")
	}
	if (Options{Engine: "zapret"}).WantsZapret2() {
		t.Error("zapret should not want zapret2")
	}
}

func TestDirPaths(t *testing.T) {
	opts := Options{OutputDir: filepath.Join("project", "output")}
	downloaded := filepath.Join(opts.OutputDir, "_downloaded", "zapret")
	converted := filepath.Join(opts.OutputDir, "_converted", "zapret2")
	final := filepath.Join(opts.OutputDir, "final", "zapret")
	autopick := filepath.Join(opts.OutputDir, "final", "autopick", "zapret2")
	if got := opts.DownloadedDir("zapret"); !pathsEqual(got, downloaded) {
		t.Errorf("DownloadedDir = %q, want %q", got, downloaded)
	}
	if got := opts.ConvertedDir("zapret2"); !pathsEqual(got, converted) {
		t.Errorf("ConvertedDir = %q, want %q", got, converted)
	}
	if got := opts.FinalDir("zapret"); !pathsEqual(got, final) {
		t.Errorf("FinalDir = %q, want %q", got, final)
	}
	if got := opts.AutopickDir("zapret2"); !pathsEqual(got, autopick) {
		t.Errorf("AutopickDir = %q, want %q", got, autopick)
	}
}

func pathsEqual(a, b string) bool {
	return filepath.ToSlash(a) == filepath.ToSlash(b) || strings.EqualFold(a, b)
}
