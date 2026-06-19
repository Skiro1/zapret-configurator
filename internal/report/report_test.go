package report

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScoreResults(t *testing.T) {
	results := []AutopickResult{
		{
			Success: true,
			Probes: []ProbeResult{
				{Kind: ProbeHTTPS, Success: true, LatencyMS: 50},
				{Kind: ProbeSTUN, Success: true, LatencyMS: 30},
				{Kind: ProbeUDP, Success: true, LatencyMS: 20},
			},
		},
		{
			Success: true,
			Probes: []ProbeResult{
				{Kind: ProbeHTTPS, Success: true, LatencyMS: 200},
				{Kind: ProbeSTUN, Success: false},
				{Kind: ProbeUDP, Success: false},
			},
		},
		{
			Success: false,
		},
	}
	ScoreResults(results)
	if results[0].Score <= results[1].Score {
		t.Errorf("all-probes-passing should score higher: %.1f vs %.1f", results[0].Score, results[1].Score)
	}
	if results[2].Score != 0 {
		t.Errorf("failed config should score 0, got %.1f", results[2].Score)
	}
}

func TestScoreResultsFastHTTPS(t *testing.T) {
	results := []AutopickResult{
		{
			Success: true,
			Probes: []ProbeResult{
				{Kind: ProbeHTTPS, Success: true, LatencyMS: 50},
			},
		},
		{
			Success: true,
			Probes: []ProbeResult{
				{Kind: ProbeHTTPS, Success: true, LatencyMS: 600},
			},
		},
	}
	ScoreResults(results)
	if results[0].Score <= results[1].Score {
		t.Errorf("fast HTTPS should score higher: %.1f vs %.1f", results[0].Score, results[1].Score)
	}
}

func TestSortByScore(t *testing.T) {
	results := []AutopickResult{
		{ConfigPath: "b.bat", Score: 50},
		{ConfigPath: "a.bat", Score: 100},
		{ConfigPath: "c.bat", Score: 75},
	}
	SortByScore(results)
	if results[0].ConfigPath != "a.bat" {
		t.Errorf("first should be a.bat, got %s", results[0].ConfigPath)
	}
	if results[1].ConfigPath != "c.bat" {
		t.Errorf("second should be c.bat, got %s", results[1].ConfigPath)
	}
	if results[2].ConfigPath != "b.bat" {
		t.Errorf("third should be b.bat, got %s", results[2].ConfigPath)
	}
}

func TestSortByScoreEqual(t *testing.T) {
	results := []AutopickResult{
		{ConfigPath: "b.bat", Score: 100},
		{ConfigPath: "a.bat", Score: 100},
	}
	SortByScore(results)
	if results[0].ConfigPath != "a.bat" {
		t.Errorf("equal score should sort by path: first = %s", results[0].ConfigPath)
	}
}

func TestSortByScoreEmpty(t *testing.T) {
	var results []AutopickResult
	SortByScore(results)
	if len(results) != 0 {
		t.Error("sorting empty slice should not panic")
	}
}

func TestNewAutopickReport(t *testing.T) {
	results := []AutopickResult{
		{Success: true},
		{Success: false},
		{Success: true},
	}
	r := NewAutopickReport("discord.com", "quick", results)
	if r.TotalTested != 3 {
		t.Errorf("TotalTested = %d, want 3", r.TotalTested)
	}
	if r.TotalPassed != 2 {
		t.Errorf("TotalPassed = %d, want 2", r.TotalPassed)
	}
	if r.Target != "discord.com" {
		t.Errorf("Target = %q, want discord.com", r.Target)
	}
	if r.Mode != "quick" {
		t.Errorf("Mode = %q, want quick", r.Mode)
	}
	if r.GeneratedAt == "" {
		t.Error("GeneratedAt should not be empty")
	}
}

func TestWriteAutopickReport(t *testing.T) {
	dir := t.TempDir()
	results := []AutopickResult{
		{
			Engine:     "zapret",
			ConfigPath: "test.bat",
			Success:    true,
			Score:      100,
			Probes: []ProbeResult{
				{Kind: ProbeHTTPS, Success: true, LatencyMS: 42.5},
				{Kind: ProbeSTUN, Success: true, LatencyMS: 15},
				{Kind: ProbeUDP, Success: false, Error: "timeout"},
			},
		},
	}
	r := NewAutopickReport("discord.com", "quick", results)
	if err := WriteAutopickReport(dir, r); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(dir, "autopick-results.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Fatal("autopick-results.json not created")
	}
	mdPath := filepath.Join(dir, "autopick-results.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	md := string(data)
	if len(md) == 0 {
		t.Fatal("markdown report is empty")
	}
}

func TestMarkdownTable(t *testing.T) {
	results := []AutopickResult{
		{
			Engine:     "zapret",
			ConfigPath: "test.bat",
			Success:    true,
			Score:      95,
			Probes: []ProbeResult{
				{Kind: ProbeHTTPS, Success: true, LatencyMS: 30},
				{Kind: ProbeSTUN, Success: true, LatencyMS: 10},
				{Kind: ProbeUDP, Success: false, Error: "fail"},
			},
		},
	}
	r := NewAutopickReport("discord.com", "quick", results)
	md := markdown(r)
	if !containsStr(md, "| zapret |") {
		t.Error("markdown should contain engine column")
	}
	if !containsStr(md, "95") {
		t.Error("markdown should contain score")
	}
	if !containsStr(md, "ms") {
		t.Error("markdown should contain latency with ms suffix")
	}
	if !containsStr(md, "FAIL") {
		t.Error("markdown should contain FAIL for unsuccessful probe")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestFormatFloatEdgeCases(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0, "0"},
		{0.0, "0"},
		{1.10, "1.1"},
		{1.00, "1"},
		{123.456, "123.456"},
	}
	for _, tt := range tests {
		got := formatFloat(tt.input)
		if got != tt.want {
			t.Errorf("formatFloat(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatInt(t *testing.T) {
	got := formatInt(42)
	if got != "42" {
		t.Errorf("formatInt(42) = %q, want 42", got)
	}
}

func TestProbeStatus(t *testing.T) {
	probes := []ProbeResult{
		{Kind: ProbeHTTPS, Success: true, LatencyMS: 42},
		{Kind: ProbeSTUN, Success: false},
	}
	got := probeStatus(probes, ProbeHTTPS)
	if got != "42ms" {
		t.Errorf("probeStatus(HTTPS) = %q, want 42ms", got)
	}
	got = probeStatus(probes, ProbeSTUN)
	if got != "FAIL" {
		t.Errorf("probeStatus(STUN) = %q, want FAIL", got)
	}
	got = probeStatus(probes, ProbeUDP)
	if got != "-" {
		t.Errorf("probeStatus(UDP) = %q, want -", got)
	}
}

func TestProbeError(t *testing.T) {
	probes := []ProbeResult{
		{Kind: ProbeHTTPS, Success: true},
		{Kind: ProbeSTUN, Success: false, Error: "timeout"},
	}
	got := probeError(probes)
	if got != "timeout" {
		t.Errorf("probeError = %q, want timeout", got)
	}
}

func TestProbeErrorEmpty(t *testing.T) {
	probes := []ProbeResult{
		{Kind: ProbeHTTPS, Success: true},
	}
	got := probeError(probes)
	if got != "" {
		t.Errorf("probeError = %q, want empty", got)
	}
}
