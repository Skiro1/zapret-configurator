package bat

import (
	"strings"
	"testing"
)

func TestPatchInsertsInterfaceBeforeCD(t *testing.T) {
	src := `@echo off
set "BIN=%~dp0bin\"
set "LISTS=%~dp0lists\"
cd /d %BIN%
"%BIN%winws.exe" --wf-tcp=80,443`

	got := Patch(src, EngineZapret)
	lists := strings.Index(got, `set "LISTS=%~dp0lists\"`)
	block := strings.Index(got, `set "PHY_IDX="`)
	cd := strings.Index(got, `cd /d %BIN%`)
	if !(lists >= 0 && block > lists && cd > block) {
		t.Fatalf("interface block not inserted between LISTS and cd:\n%s", got)
	}
	if !strings.Contains(got, `"%BIN%winws.exe" %IFACE_FILTER% --wf-tcp=80,443`) {
		t.Fatalf("IFACE_FILTER not inserted before --wf-tcp:\n%s", got)
	}
}

func TestPatchIdempotent(t *testing.T) {
	src := `@echo off
setlocal enabledelayedexpansion
set "BIN=%~dp0bin\"
set "LISTS=%~dp0lists\"
cd /d %BIN%
"%BIN%winws.exe" --wf-tcp=80,443`
	once := Patch(src, EngineZapret)
	twice := Patch(once, EngineZapret)
	if strings.Count(strings.ToLower(twice), `set "phy_idx="`) != 1 {
		t.Fatalf("patch duplicated PHY_IDX block:\n%s", twice)
	}
	if strings.Count(strings.ToLower(twice), `"%bin%winws.exe" %iface_filter% --wf-tcp=80,443`) != 1 {
		t.Fatalf("unexpected winws IFACE_FILTER patch after idempotent patch:\n%s", twice)
	}
}

func TestPatchZapret2AndTcpOut(t *testing.T) {
	src := `@echo off
set "BIN=%~dp0bin\"
set "LISTS=%~dp0lists\"
cd /d %BIN%
"%BIN%winws.exe" --wf-tcp-out=443`
	got := Patch(src, EngineZapret2)
	if !strings.Contains(got, `"%BIN%winws2.exe" %IFACE_FILTER% --wf-tcp-out=443`) {
		t.Fatalf("zapret2 patch failed:\n%s", got)
	}
}

func TestPatchZapret2ExeCaseInsensitive(t *testing.T) {
	src := `@echo off
set "BIN=%~dp0bin\"
set "LISTS=%~dp0lists\"
cd /d %BIN%
"%BIN%winws.EXE" --wf-tcp=80,443`
	got := Patch(src, EngineZapret2)
	if !strings.Contains(got, `winws2.exe`) {
		t.Fatalf("uppercase EXE not replaced:\n%s", got)
	}
	if strings.Contains(got, `winws.EXE`) {
		t.Fatalf("uppercase EXE still present:\n%s", got)
	}
}

func TestPatchIdempotentThreeTimes(t *testing.T) {
	src := `@echo off
set "BIN=%~dp0bin\"
set "LISTS=%~dp0lists\"
cd /d %BIN%
"%BIN%winws.exe" --wf-tcp=80,443`
	r1 := Patch(src, EngineZapret)
	r2 := Patch(r1, EngineZapret)
	r3 := Patch(r2, EngineZapret)
	if r2 != r3 {
		t.Fatal("patch not idempotent after 3 runs")
	}
}

func TestPatchWithExistingSetlocal(t *testing.T) {
	src := `@echo off
setlocal enabledelayedexpansion
set "BIN=%~dp0bin\"
set "LISTS=%~dp0lists\"
cd /d %BIN%
"%BIN%winws.exe" --wf-tcp=80,443`
	got := Patch(src, EngineZapret)
	count := strings.Count(strings.ToLower(got), "setlocal enabledelayedexpansion")
	if count != 1 {
		t.Fatalf("expected exactly 1 setlocal, got %d:\n%s", count, got)
	}
}

func TestPatchNoWinwsLine(t *testing.T) {
	src := `@echo off
set "BIN=%~dp0bin\"
set "LISTS=%~dp0lists\"
cd /d %BIN%
echo no winws here`
	got := Patch(src, EngineZapret)
	if strings.Contains(got, "%IFACE_FILTER%") {
		t.Fatalf("IFACE_FILTER should not be added without winws line:\n%s", got)
	}
}

func TestPatchMultilineStart(t *testing.T) {
	src := `@echo off
set "BIN=%~dp0bin\"
set "LISTS=%~dp0lists\"
cd /d %BIN%
"%BIN%winws.exe" --wf-tcp=80,443 --wf-udp=443 ^
--filter-tcp=443 --dpi-desync=fake`
	got := Patch(src, EngineZapret)
	if !strings.Contains(got, `%IFACE_FILTER% --wf-tcp=80,443`) {
		t.Fatalf("IFACE_FILTER not on first winws line:\n%s", got)
	}
}

func TestPatchCarriageReturn(t *testing.T) {
	src := "@echo off\r\nset \"BIN=%~dp0bin\\\"\r\nset \"LISTS=%~dp0lists\\\"\r\ncd /d %BIN%\r\n\"%BIN%winws.exe\" --wf-tcp=80,443"
	got := Patch(src, EngineZapret)
	if !strings.Contains(got, "%IFACE_FILTER%") {
		t.Fatalf("IFACE_FILTER missing with CRLF input:\n%s", got)
	}
	if !strings.Contains(got, "\r\n") {
		t.Fatal("output should use CRLF")
	}
}

func TestNormalizeNewlines(t *testing.T) {
	got := normalizeNewlines("a\r\nb\nc\r")
	if got != "a\nb\nc" {
		t.Errorf("normalizeNewlines = %q, want a\\nb\\nc", got)
	}
}

func TestPatchEmptyContent(t *testing.T) {
	got := Patch("", EngineZapret)
	if strings.TrimSpace(got) != "" {
		t.Fatal("patching empty content should produce empty output")
	}
}

func TestPatchOnlyServiceBat(t *testing.T) {
	src := `@echo off
set "BIN=%~dp0bin\"
set "LISTS=%~dp0lists\"
cd /d %BIN%
call service.bat status_zapret`
	got := Patch(src, EngineZapret)
	if strings.Contains(got, "%IFACE_FILTER%") {
		t.Fatalf("IFACE_FILTER should not be added when no winws line:\n%s", got)
	}
}

func TestPatchFirstWFTCPIndex(t *testing.T) {
	tests := []struct {
		line string
		want int
	}{
		{"--wf-tcp=80,443", 0},
		{"--wf-tcp-out=443", 0},
		{"--wf-tcp 80,443", 0},
		{"--dpi-desync=fake --wf-tcp=80", 18},
		{"--dpi-desync=fake", -1},
		{"", -1},
	}
	for _, tt := range tests {
		got := firstWFTCPIndex(tt.line)
		if got != tt.want {
			t.Errorf("firstWFTCPIndex(%q) = %d, want %d", tt.line, got, tt.want)
		}
	}
}

func TestContainsFold(t *testing.T) {
	lines := []string{"set \"PHY_IDX=\"", "set \"IFACE_FILTER=\""}
	if !containsFold(lines, `set "phy_idx="`) {
		t.Error("containsFold should be case insensitive")
	}
	if containsFold(lines, "missing") {
		t.Error("containsFold should return false for missing")
	}
}
