package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "deep")
	if err := EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); os.IsNotExist(err) {
		t.Fatal("directory not created")
	}
}

func TestEnsureDirExisting(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureCleanDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "clean")
	_ = os.MkdirAll(target, 0o755)
	_ = os.WriteFile(filepath.Join(target, "old.txt"), []byte("old"), 0o644)
	_ = os.MkdirAll(filepath.Join(target, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(target, "sub", "nested.txt"), []byte("nested"), 0o644)
	if err := EnsureCleanDir(target); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(target)
	if len(entries) != 0 {
		t.Errorf("expected empty dir, got %d entries", len(entries))
	}
}

func TestEnsureCleanDirEmpty(t *testing.T) {
	if err := EnsureCleanDir(""); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst", "sub", "dst.txt")
	_ = os.WriteFile(src, []byte("hello"), 0o644)
	if err := CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "hello" {
		t.Errorf("content = %q, want hello", string(data))
	}
}

func TestCopyFileOverwrite(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	_ = os.WriteFile(src, []byte("new"), 0o644)
	_ = os.WriteFile(dst, []byte("old"), 0o644)
	if err := CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "new" {
		t.Errorf("content = %q, want new", string(data))
	}
}

func TestCopyDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	_ = os.MkdirAll(filepath.Join(src, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("b"), 0o644)
	if err := CopyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dst, "a.txt"))
	if string(data) != "a" {
		t.Errorf("a.txt = %q, want a", string(data))
	}
	data, _ = os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if string(data) != "b" {
		t.Errorf("sub/b.txt = %q, want b", string(data))
	}
}

func TestCopyDirNotDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(file, []byte("content"), 0o644)
	err := CopyDir(file, filepath.Join(dir, "dst"))
	if err == nil {
		t.Error("expected error when copying file as dir")
	}
}

func TestCopyDirIfExists(t *testing.T) {
	dir := t.TempDir()
	if err := CopyDirIfExists(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "dst")); err != nil {
		t.Fatal(err)
	}
}

func TestCopyDirIfExistsExists(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	_ = os.MkdirAll(src, 0o755)
	_ = os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644)
	if err := CopyDirIfExists(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "a.txt")); os.IsNotExist(err) {
		t.Error("file not copied")
	}
}

func TestCopyFileIfExists(t *testing.T) {
	dir := t.TempDir()
	if err := CopyFileIfExists(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "dst")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "dst")); !os.IsNotExist(err) {
		t.Error("file should not exist")
	}
}

func TestCopyFileIfExistsExists(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	_ = os.WriteFile(src, []byte("hello"), 0o644)
	if err := CopyFileIfExists(src, dst); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "hello" {
		t.Errorf("content = %q, want hello", string(data))
	}
}

func TestHasFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "target.txt"), []byte(""), 0o644)
	if !HasFile(dir, "target.txt") {
		t.Error("HasFile should find target.txt")
	}
	if HasFile(dir, "missing.txt") {
		t.Error("HasFile should not find missing.txt")
	}
}

func TestHasFileCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "MyFile.TXT"), []byte(""), 0o644)
	if !HasFile(dir, "myfile.txt") {
		t.Error("HasFile should be case insensitive")
	}
}

func TestFindFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "sub", "deep.txt"), []byte("deep"), 0o644)
	path, ok := FindFile(dir, "deep.txt")
	if !ok {
		t.Fatal("FindFile should find deep.txt")
	}
	if filepath.Base(path) != "deep.txt" {
		t.Errorf("found = %q, want deep.txt", path)
	}
}

func TestFindFileMissing(t *testing.T) {
	dir := t.TempDir()
	_, ok := FindFile(dir, "missing.txt")
	if ok {
		t.Error("FindFile should return false for missing file")
	}
}

func TestFindDirs(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "lua"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "bin"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "other"), 0o755)
	result := FindDirs(dir, "lua", "bin")
	if len(result["lua"]) != 1 {
		t.Errorf("expected 1 lua dir, got %d", len(result["lua"]))
	}
	if len(result["bin"]) != 1 {
		t.Errorf("expected 1 bin dir, got %d", len(result["bin"]))
	}
}

func TestSafeFilename(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello world", "hello world"},
		{"a<>:\"/\\|?*b", "a_b"},
		{"  .  ", "config"},
		{"", "config"},
		{".", "config"},
		{"...", "config"},
	}
	for _, tt := range tests {
		got := SafeFilename(tt.input)
		if got != tt.want {
			t.Errorf("SafeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSafeFilenameLong(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "a"
	}
	got := SafeFilename(long)
	if len(got) > 120 {
		t.Errorf("SafeFilename should limit to 120 chars, got %d", len(got))
	}
}

func TestSafeFilenameUnicode(t *testing.T) {
	got := SafeFilename("привет мир")
	if got == "" {
		t.Error("SafeFilename should handle unicode")
	}
}
