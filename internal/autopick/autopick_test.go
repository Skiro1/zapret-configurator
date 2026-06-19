package autopick

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zapret-configurator/internal/catalog"
	"zapret-configurator/internal/report"
)

func TestCollectBatFiles(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.bat"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.bat"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "service.bat"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "c.txt"), []byte(""), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "autopick"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "autopick", "skip.bat"), []byte(""), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "sub", "d.bat"), []byte(""), 0o644)

	files, err := collectBatFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 bat files, got %d: %v", len(files), files)
	}
	for _, f := range files {
		base := strings.ToLower(filepath.Base(f))
		if strings.HasPrefix(base, "service") {
			t.Errorf("service.bat should be excluded: %s", f)
		}
		if strings.Contains(f, "autopick") {
			t.Errorf("autopick dir files should be excluded: %s", f)
		}
	}
}

func TestLimitByMode(t *testing.T) {
	files := make([]string, 100)
	for i := range files {
		files[i] = filepath.Join("dir", "file"+string(rune('a'+i%26))+".bat")
	}
	got := limitByMode(files, "quick")
	if len(got) != 30 {
		t.Errorf("quick limit: expected 30, got %d", len(got))
	}
	got = limitByMode(files, "standard")
	if len(got) != 80 {
		t.Errorf("standard limit: expected 80, got %d", len(got))
	}
	got = limitByMode(files, "full")
	if len(got) != 100 {
		t.Errorf("full limit: expected 100, got %d", len(got))
	}
	got = limitByMode(files[:5], "quick")
	if len(got) != 5 {
		t.Errorf("quick with fewer files: expected 5, got %d", len(got))
	}
}

func TestAnyProbeSuccess(t *testing.T) {
	tests := []struct {
		name   string
		probes []report.ProbeResult
		want   bool
	}{
		{"empty", nil, false},
		{"all fail", []report.ProbeResult{{Success: false}, {Success: false}}, false},
		{"one success", []report.ProbeResult{{Success: false}, {Success: true}}, true},
		{"all success", []report.ProbeResult{{Success: true}, {Success: true}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := anyProbeSuccess(tt.probes)
			if got != tt.want {
				t.Errorf("anyProbeSuccess = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafeFilenameAutopick(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello", "hello"},
		{"a:b/c", "a_b_c"},
		{"", "preset"},
		{strings.Repeat("x", 150), strings.Repeat("x", 100)},
	}
	for _, tt := range tests {
		got := safeFilename(tt.input)
		if got != tt.want {
			t.Errorf("safeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeArgForBat(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"--hostlist=lists/example.txt", "--hostlist=%LISTS%example.txt"},
		{"--hostlist=@lists/test.txt", "--hostlist=@%LISTS%test.txt"},
		{"bin/test.bin", "%BIN%test.bin"},
		{"@bin/test.bin", "@%BIN%test.bin"},
		{"--dpi-desync=fake", "--dpi-desync=fake"},
		{"bin\\test.bin", "%BIN%test.bin"},
	}
	for _, tt := range tests {
		got := normalizeArgForBat(tt.input, "", "")
		if got != tt.want {
			t.Errorf("normalizeArgForBat(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEscapePath(t *testing.T) {
	got := escapePath("path/to/dir")
	if got != "path\\to\\dir" {
		t.Errorf("escapePath = %q, want path\\to\\dir", got)
	}
}

func TestWriteCatalogPreset(t *testing.T) {
	dir := t.TempDir()
	s := catalogStrategyFixture()
	batPath := writeCatalogPreset(dir, s, "C:\\bin", "C:\\lua", "C:\\lists")
	data, err := os.ReadFile(batPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, `winws2.exe`) {
		t.Error("preset should contain winws2.exe")
	}
	if !strings.Contains(content, `%IFACE_FILTER%`) {
		t.Error("preset should contain IFACE_FILTER")
	}
	if !strings.Contains(content, `--dpi-desync=fake`) {
		t.Error("preset should contain strategy args")
	}
	if !strings.Contains(content, `lua`) {
		t.Error("preset should contain lua path")
	}
}

func catalogStrategyFixture() catalog.Strategy {
	return catalog.Strategy{
		ID:   "test_strategy",
		Name: "Test Strategy",
		Args: []string{"--dpi-desync=fake", "--dpi-desync-repeats=6"},
	}
}

func TestCopyTop(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	dstDir := filepath.Join(dir, "dst")
	_ = os.MkdirAll(srcDir, 0o755)
	_ = os.WriteFile(filepath.Join(srcDir, "good.bat"), []byte("good"), 0o644)
	_ = os.WriteFile(filepath.Join(srcDir, "bad.bat"), []byte("bad"), 0o644)

	results := []report.AutopickResult{
		{ConfigPath: filepath.Join(srcDir, "good.bat"), Success: true, Score: 100},
		{ConfigPath: filepath.Join(srcDir, "bad.bat"), Success: false, Score: 0},
	}
	if err := copyTop(results, dstDir, 5, dir, "zapret"); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dstDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in dst, got %d", len(entries))
	}
	if !strings.HasPrefix(entries[0].Name(), "01_") {
		t.Errorf("expected prefix 01_, got %s", entries[0].Name())
	}
}

func TestCopyTopEmpty(t *testing.T) {
	dir := t.TempDir()
	dstDir := filepath.Join(dir, "dst")
	results := []report.AutopickResult{}
	if err := copyTop(results, dstDir, 5, dir, "zapret"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		t.Error("dst dir should exist even with no results")
	}
}
