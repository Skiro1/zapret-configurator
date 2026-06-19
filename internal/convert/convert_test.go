package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zapret-configurator/internal/bat"
)

func TestPresetTextToBat(t *testing.T) {
	src := `# Preset: sample
--wf-tcp=80,443
--filter-tcp=443
--hostlist=lists/example.txt
--dpi-desync=fake`
	got := PresetTextToBat(src, "winws.exe")
	if !strings.Contains(got, `"%BIN%winws.exe" --wf-tcp=80,443`) {
		t.Fatalf("missing winws start:\n%s", got)
	}
	if !strings.Contains(got, `--hostlist=%LISTS%example.txt`) {
		t.Fatalf("hostlist was not normalized:\n%s", got)
	}
}

func TestPresetTextToBatEmpty(t *testing.T) {
	got := PresetTextToBat("", "winws.exe")
	if !strings.Contains(got, `"%BIN%winws.exe" --wf-tcp=80,443`) {
		t.Fatalf("empty preset should use default wf-tcp:\n%s", got)
	}
}

func TestPresetTextToBatWinws2(t *testing.T) {
	got := PresetTextToBat("--wf-tcp=80,443", "winws2.exe")
	if !strings.Contains(got, `winws2.exe`) {
		t.Fatalf("should use winws2.exe:\n%s", got)
	}
}

func TestPresetTextToBatLineWrapping(t *testing.T) {
	args := ""
	for i := 0; i < 20; i++ {
		args += "--dpi-desync=fake\n"
	}
	got := PresetTextToBat(args, "winws.exe")
	if !strings.Contains(got, "^\r\n") {
		t.Fatal("expected line wrapping with ^")
	}
}

func TestSplitInlineArgs(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"--dpi-desync=fake", 1},
		{"--dpi-desync=fake --filter-tcp=443", 2},
		{"--hostlist=lists/example.txt --hostlist-exclude=lists/exclude.txt --dpi-desync=fake", 3},
		{"single_arg", 1},
	}
	for _, tt := range tests {
		got := splitInlineArgs(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitInlineArgs(%q) returned %d parts, want %d: %v", tt.input, len(got), tt.want, got)
		}
	}
}

func TestSplitInlineArgsKeepsFull(t *testing.T) {
	line := "--hostlist=lists/a.txt --hostlist-exclude=lists/b.txt"
	got := splitInlineArgs(line)
	if got[0] != "--hostlist=lists/a.txt" {
		t.Errorf("first part = %q", got[0])
	}
	if got[1] != "--hostlist-exclude=lists/b.txt" {
		t.Errorf("second part = %q", got[1])
	}
}

func TestBatchQuoteArg(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"--dpi-desync=fake", "--dpi-desync=fake"},
		{"bin/test.bin", "%BIN%test.bin"},
		{"@bin/test.bin", "@%BIN%test.bin"},
		{"lists/example.txt", "%LISTS%example.txt"},
		{"@lists/example.txt", "@%LISTS%example.txt"},
		{"@lua/script.lua", "@%~dp0lua\\script.lua"},
		{"lua/script.lua", "%~dp0lua\\script.lua"},
		{"@windivert.filter/test.txt", "@%~dp0windivert.filter\\test.txt"},
		{"windivert.filter/test.txt", "%~dp0windivert.filter\\test.txt"},
		{"bin\\test.bin", "%BIN%test.bin"},
	}
	for _, tt := range tests {
		got := batchQuoteArg(tt.input)
		if got != tt.want {
			t.Errorf("batchQuoteArg(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPresetLinesToArgs(t *testing.T) {
	src := `# comment
--filter-tcp=443
--dpi-desync=fake

--hostlist=lists/example.txt`
	got := presetLinesToArgs(src)
	if len(got) != 3 {
		t.Fatalf("expected 3 args, got %d: %v", len(got), got)
	}
}

func TestPresetLinesToArgsSkipsComments(t *testing.T) {
	src := `# This is a comment
# Another comment
--filter-tcp=443`
	got := presetLinesToArgs(src)
	if len(got) != 1 {
		t.Fatalf("expected 1 arg, got %d: %v", len(got), got)
	}
}

func TestWriteWrappedArgs(t *testing.T) {
	var b strings.Builder
	args := []string{"--a=1", "--b=2", "--c=3", "--d=4", "--e=5", "--f=6", "--g=7"}
	writeWrappedArgs(&b, args)
	result := b.String()
	if !strings.Contains(result, "^\r\n") {
		t.Fatal("expected line wrapping after 6 args")
	}
}

func TestConvertFlowsealBatsSkipService(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	_ = createTestFile(t, srcDir, "service.bat", "echo service")
	_ = createTestFile(t, srcDir, "general.bat", "echo general")
	err := convertFlowsealBats(srcDir, dstDir, bat.EngineZapret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "service.bat")); !os.IsNotExist(err) {
		t.Error("service.bat should be skipped")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "general.bat")); os.IsNotExist(err) {
		t.Error("general.bat should be copied")
	}
}

func TestConvertFlowsealBatsMissing(t *testing.T) {
	err := convertFlowsealBats("/nonexistent", "/tmp/dst", bat.EngineZapret)
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
}

func TestConvertPresetDirMissing(t *testing.T) {
	err := convertPresetDir("/nonexistent", "/tmp/dst", "winws.exe", bat.EngineZapret)
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
}

func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := dir + "/" + name
	_ = os.WriteFile(path, []byte(content), 0o644)
	return path
}
