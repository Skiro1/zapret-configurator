package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ProbeKind string

const (
	ProbeHTTPS ProbeKind = "https"
	ProbeSTUN  ProbeKind = "stun"
	ProbeUDP   ProbeKind = "udp"
)

type ProbeResult struct {
	Kind      ProbeKind `json:"kind"`
	Success   bool      `json:"success"`
	LatencyMS float64   `json:"latency_ms,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type AutopickResult struct {
	Engine     string        `json:"engine"`
	ConfigPath string        `json:"config_path"`
	Success    bool          `json:"success"`
	Score      float64       `json:"score"`
	Probes     []ProbeResult `json:"probes"`
	Error      string        `json:"error,omitempty"`
}

type AutopickReport struct {
	GeneratedAt string           `json:"generated_at"`
	Target      string           `json:"target"`
	Mode        string           `json:"mode"`
	TotalTested int              `json:"total_tested"`
	TotalPassed int              `json:"total_passed"`
	Results     []AutopickResult `json:"results"`
}

func WriteAutopickReport(dir string, r AutopickReport) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "autopick-results.json"), data, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "autopick-results.md"), []byte(markdown(r)), 0o644)
}

func NewAutopickReport(target, mode string, results []AutopickResult) AutopickReport {
	passed := 0
	for _, r := range results {
		if r.Success {
			passed++
		}
	}
	return AutopickReport{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Target:      target,
		Mode:        mode,
		TotalTested: len(results),
		TotalPassed: passed,
		Results:     results,
	}
}

func ScoreResults(results []AutopickResult) {
	for i := range results {
		results[i].Score = calcScore(results[i])
	}
}

func calcScore(r AutopickResult) float64 {
	if !r.Success {
		return 0
	}
	score := 100.0
	for _, p := range r.Probes {
		if !p.Success {
			score -= 25
		} else if p.LatencyMS > 0 {
			if p.LatencyMS < 100 {
				score += 5
			} else if p.LatencyMS > 500 {
				score -= 10
			}
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

func SortByScore(results []AutopickResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ConfigPath < results[j].ConfigPath
	})
}

func markdown(r AutopickReport) string {
	var b strings.Builder
	b.WriteString("# Autopick Results\n\n")
	b.WriteString("- Generated: " + r.GeneratedAt + "\n")
	b.WriteString("- Target: `" + r.Target + "`\n")
	b.WriteString("- Mode: `" + r.Mode + "`\n")
	b.WriteString("- Tested: " + formatInt(r.TotalTested) + "\n")
	b.WriteString("- Passed: " + formatInt(r.TotalPassed) + "\n\n")
	b.WriteString("| # | Engine | Score | HTTPS | STUN | UDP | Config | Error |\n")
	b.WriteString("| ---: | --- | ---: | ---: | ---: | ---: | --- | --- |\n")
	for i, row := range r.Results {
		httpsSt := probeStatus(row.Probes, ProbeHTTPS)
		stunSt := probeStatus(row.Probes, ProbeSTUN)
		udpSt := probeStatus(row.Probes, ProbeUDP)
		b.WriteString("| ")
		b.WriteString(formatInt(i + 1))
		b.WriteString(" | ")
		b.WriteString(row.Engine)
		b.WriteString(" | ")
		b.WriteString(formatFloat(row.Score))
		b.WriteString(" | ")
		b.WriteString(httpsSt)
		b.WriteString(" | ")
		b.WriteString(stunSt)
		b.WriteString(" | ")
		b.WriteString(udpSt)
		b.WriteString(" | `")
		b.WriteString(strings.ReplaceAll(row.ConfigPath, "|", "\\|"))
		b.WriteString("` | ")
		errMsg := row.Error
		if errMsg == "" {
			errMsg = probeError(row.Probes)
		}
		b.WriteString(strings.ReplaceAll(errMsg, "|", "\\|"))
		b.WriteString(" |\n")
	}
	return b.String()
}

func probeStatus(probes []ProbeResult, kind ProbeKind) string {
	for _, p := range probes {
		if p.Kind == kind {
			if p.Success {
				return formatFloat(p.LatencyMS) + "ms"
			}
			return "FAIL"
		}
	}
	return "-"
}

func probeError(probes []ProbeResult) string {
	for _, p := range probes {
		if p.Error != "" {
			return p.Error
		}
	}
	return ""
}

func formatFloat(v float64) string {
	text := strings.TrimRight(strings.TrimRight(jsonNumber(v), "0"), ".")
	if text == "" {
		return "0"
	}
	return text
}

func formatInt(v int) string {
	return jsonNumber(float64(v))
}

func jsonNumber(v float64) string {
	data, _ := json.Marshal(v)
	return string(data)
}
