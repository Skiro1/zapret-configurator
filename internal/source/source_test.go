package source

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchesPrefix(t *testing.T) {
	tests := []struct {
		rel    string
		prefix []string
		want   bool
	}{
		{"src/presets/builtin/winws1/file.txt", []string{"src/presets/builtin/winws1"}, true},
		{"src/other/file.txt", []string{"src/presets/builtin/winws1"}, false},
		{"file.txt", []string{"src"}, false},
		{"src/presets/builtin/winws1/file.txt", []string{"src/presets"}, true},
	}
	for _, tt := range tests {
		got := matchesPrefix(tt.rel, tt.prefix)
		if got != tt.want {
			t.Errorf("matchesPrefix(%q, %v) = %v, want %v", tt.rel, tt.prefix, got, tt.want)
		}
	}
}

func TestExtractZipSelective(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"keep/file2.txt": "content2",
		"skip/file3.txt": "content3",
	})
	dst := filepath.Join(dir, "out")
	err := extractZip(zipPath, dst, []string{"keep"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "keep", "file2.txt")); os.IsNotExist(err) {
		t.Error("selected file not extracted")
	}
	if _, err := os.Stat(filepath.Join(dst, "skip", "file3.txt")); !os.IsNotExist(err) {
		t.Error("non-selected file should not be extracted")
	}
}

func createTestZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
}
