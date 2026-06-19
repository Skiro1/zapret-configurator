package probes

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"zapret-configurator/internal/report"
)

const (
	defaultHTTPSTimeout = 2 * time.Second
	defaultSTUNTimeout  = 2 * time.Second
	defaultUDPTimeout   = 2 * time.Second
)

var stunServers = []string{
	"stun.l.google.com:19302",
	"stun1.l.google.com:19302",
	"stun2.l.google.com:19302",
	"stun.cloudflare.com:3478",
}

var udpTestTargets = []struct {
	Addr string
	Port int
	Name string
}{
	{"discord.com", 443, "discord QUIC"},
	{"google.com", 443, "google QUIC"},
	{"youtube.com", 443, "youtube QUIC"},
}

func RunAll(ctx context.Context, target string) []report.ProbeResult {
	targets := strings.Split(target, ",")
	var httpsResults []report.ProbeResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			r := probeHTTPS(ctx, host)
			mu.Lock()
			httpsResults = append(httpsResults, r)
			mu.Unlock()
		}(t)
	}
	wg.Wait()
	results := httpsResults
	results = append(results, probeSTUN(ctx))
	results = append(results, probeUDP(ctx)...)
	return results
}

func probeHTTPS(ctx context.Context, target string) report.ProbeResult {
	host := normalizeHost(target)
	dialer := &net.Dialer{Timeout: defaultHTTPSTimeout}
	start := time.Now()
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, "443"), &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return report.ProbeResult{Kind: report.ProbeHTTPS, Error: err.Error()}
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := conn.Write([]byte("HEAD / HTTP/1.1\r\nHost: " + host + "\r\nConnection: close\r\n\r\n")); err != nil {
		return report.ProbeResult{Kind: report.ProbeHTTPS, Error: err.Error()}
	}
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		return report.ProbeResult{Kind: report.ProbeHTTPS, Error: err.Error()}
	}
	latency := time.Since(start)
	return report.ProbeResult{
		Kind:      report.ProbeHTTPS,
		Success:   true,
		LatencyMS: float64(latency.Microseconds()) / 1000.0,
	}
}

func probeSTUN(ctx context.Context) report.ProbeResult {
	for _, server := range stunServers {
		if result := trySTUN(ctx, server); result.Success {
			return result
		}
	}
	return report.ProbeResult{Kind: report.ProbeSTUN, Error: "all STUN servers failed"}
}

func trySTUN(ctx context.Context, server string) report.ProbeResult {
	dialer := net.Dialer{Timeout: defaultSTUNTimeout}
	conn, err := dialer.DialContext(ctx, "udp4", server)
	if err != nil {
		return report.ProbeResult{Kind: report.ProbeSTUN, Error: err.Error()}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(defaultSTUNTimeout))

	msg := makeSTUNBindingRequest()
	start := time.Now()
	if _, err := conn.Write(msg); err != nil {
		return report.ProbeResult{Kind: report.ProbeSTUN, Error: err.Error()}
	}
	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		return report.ProbeResult{Kind: report.ProbeSTUN, Error: err.Error()}
	}
	latency := time.Since(start)
	if n < 20 {
		return report.ProbeResult{Kind: report.ProbeSTUN, Error: "STUN response too short"}
	}
	type_ := binary.BigEndian.Uint16(buf[0:2])
	if type_ != 0x0101 {
		return report.ProbeResult{Kind: report.ProbeSTUN, Error: fmt.Sprintf("unexpected STUN response type: 0x%04X", type_)}
	}
	return report.ProbeResult{
		Kind:      report.ProbeSTUN,
		Success:   true,
		LatencyMS: float64(latency.Microseconds()) / 1000.0,
	}
}

func makeSTUNBindingRequest() []byte {
	msg := make([]byte, 20)
	binary.BigEndian.PutUint16(msg[0:2], 0x0001)
	binary.BigEndian.PutUint16(msg[2:4], 20)
	cookie := uint32(0x2112A442)
	binary.BigEndian.PutUint32(msg[4:8], cookie)
	txID := make([]byte, 12)
	_, _ = rand.Read(txID)
	copy(msg[8:20], txID)
	return msg
}

func probeUDP(ctx context.Context) []report.ProbeResult {
	var results []report.ProbeResult
	for _, t := range udpTestTargets {
		results = append(results, probeOneUDP(ctx, t.Addr, t.Port, t.Name))
	}
	return results
}

func probeOneUDP(ctx context.Context, host string, port int, label string) report.ProbeResult {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	dialer := net.Dialer{Timeout: defaultUDPTimeout}
	conn, err := dialer.DialContext(ctx, "udp4", addr)
	if err != nil {
		return report.ProbeResult{Kind: report.ProbeUDP, Error: fmt.Sprintf("%s: %v", label, err)}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(defaultUDPTimeout))

	payload := makeQUICInitial(payloadSize)
	start := time.Now()
	if _, err := conn.Write(payload); err != nil {
		return report.ProbeResult{Kind: report.ProbeUDP, Error: fmt.Sprintf("%s: %v", label, err)}
	}
	buf := make([]byte, 1500)
	conn.Read(buf)
	latency := time.Since(start)

	return report.ProbeResult{
		Kind:      report.ProbeUDP,
		Success:   true,
		LatencyMS: float64(latency.Microseconds()) / 1000.0,
	}
}

const payloadSize = 1200

func makeQUICInitial(size int) []byte {
	if size < 20 {
		size = 20
	}
	buf := make([]byte, size)
	_, _ = rand.Read(buf)
	if len(buf) > 7 {
		buf[0] = 0xC0
		buf[1] = 0x00
		buf[2] = 0x00
		buf[3] = 0x00
		buf[4] = 0x01
		buf[5] = 0x00
		buf[6] = 0x00
		buf[7] = 0x00
	}
	return buf
}

func normalizeHost(target string) string {
	host := strings.TrimSpace(target)
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(host, prefix) {
			host = host[len(prefix):]
		}
	}
	if i := strings.IndexByte(host, '/'); i >= 0 {
		host = host[:i]
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "discord.com"
	}
	return host
}
