package probes

import (
	"context"
	"testing"
	"time"
)

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"discord.com", "discord.com"},
		{"https://discord.com", "discord.com"},
		{"https://discord.com/path", "discord.com"},
		{"http://example.com:8080/x", "example.com"},
		{"", "discord.com"},
		{"https://", "discord.com"},
		{"  discord.com  ", "discord.com"},
		{"https://discord.com:443", "discord.com"},
	}
	for _, tt := range tests {
		got := normalizeHost(tt.input)
		if got != tt.want {
			t.Errorf("normalizeHost(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMakeSTUNBindingRequest(t *testing.T) {
	msg := makeSTUNBindingRequest()
	if len(msg) != 20 {
		t.Fatalf("expected 20 bytes, got %d", len(msg))
	}
	type_ := uint16(msg[0])<<8 | uint16(msg[1])
	if type_ != 0x0001 {
		t.Fatalf("expected STUN Binding Request type 0x0001, got 0x%04X", type_)
	}
	cookie := uint32(msg[4])<<24 | uint32(msg[5])<<16 | uint32(msg[6])<<8 | uint32(msg[7])
	if cookie != 0x2112A442 {
		t.Fatalf("expected cookie 0x2112A442, got 0x%08X", cookie)
	}
}

func TestMakeQUICInitial(t *testing.T) {
	payload := makeQUICInitial(1200)
	if len(payload) != 1200 {
		t.Fatalf("expected 1200 bytes, got %d", len(payload))
	}
	if payload[0] != 0xC0 {
		t.Fatalf("expected first byte 0xC0, got 0x%02X", payload[0])
	}
}

func TestMakeQUICInitialSmall(t *testing.T) {
	payload := makeQUICInitial(10)
	if len(payload) < 20 {
		t.Fatalf("minimum size should be 20, got %d", len(payload))
	}
}

func TestProbeSTUNTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := probeSTUN(ctx)
	if result.Kind != "stun" {
		t.Fatalf("expected kind stun, got %s", result.Kind)
	}
}

func TestProbeUDPNoPanic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := probeUDP(ctx)
	if len(results) == 0 {
		t.Fatal("expected at least one UDP probe result")
	}
	for _, r := range results {
		if r.Kind != "udp" {
			t.Fatalf("expected kind udp, got %s", r.Kind)
		}
	}
}

func TestRunAllNoPanic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	results := RunAll(ctx, "example.com")
	if len(results) < 3 {
		t.Fatalf("expected at least 3 probe results, got %d", len(results))
	}
	kindCount := make(map[string]int)
	for _, r := range results {
		kindCount[string(r.Kind)]++
	}
	if kindCount["https"] < 1 {
		t.Error("expected at least 1 HTTPS probe")
	}
	if kindCount["stun"] < 1 {
		t.Error("expected at least 1 STUN probe")
	}
	if kindCount["udp"] < 1 {
		t.Error("expected at least 1 UDP probe")
	}
}

func TestTrySTUNInvalidServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := trySTUN(ctx, "127.0.0.1:19999")
	if result.Success {
		t.Error("should fail for invalid server")
	}
	if result.Error == "" {
		t.Error("should have error message")
	}
}

func TestProbeHTTPSInvalidTarget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result := probeHTTPS(ctx, "192.0.2.1")
	if result.Success {
		t.Error("should fail for invalid target")
	}
}
