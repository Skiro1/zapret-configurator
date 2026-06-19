package bat

import (
	"regexp"
	"strings"
)

type Engine string

const (
	EngineZapret  Engine = "zapret"
	EngineZapret2 Engine = "zapret2"
)

const interfaceBlockWithoutSetlocal = `set "PHY_IDX="
for /f "skip=1 tokens=1" %%a in ('netsh int ip show interfaces 2^>nul ^| findstr /i "connected"') do (
    if not defined PHY_IDX set "PHY_IDX=%%a"
)
set "IFACE_FILTER="
if defined PHY_IDX set "IFACE_FILTER=--wf-iface=!PHY_IDX!"`

const wrapThreshold = 5000
const wrapChunkSize = 3500

func Patch(content string, engine Engine) string {
	text := normalizeNewlines(content)
	text = fixEscapedExclamation(text)
	text = fixEmptyArgs(text)
	text = fixMissingBinPrefix(text)
	text = fixSingleDash(text)
	text = removeMissingLuaInits(text)
	text = removeUnsupportedArgs(text)
	text = fixHostlistCommas(text)
	text = sanitizeNameArgs(text)
	lines := strings.Split(text, "\n")
	lines = patchEngine(lines, engine)
	lines = ensureInterfaceBlock(lines)
	lines = ensureIfaceArg(lines)
	lines = wrapLongCommands(lines)
	return strings.Join(lines, "\r\n")
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimRight(s, "\n")
}

func patchEngine(lines []string, engine Engine) []string {
	if engine != EngineZapret2 {
		return lines
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.ReplaceAll(line, "winws.exe", "winws2.exe")
		out[i] = strings.ReplaceAll(out[i], "winws.EXE", "winws2.exe")
	}
	return out
}

func ensureInterfaceBlock(lines []string) []string {
	if containsFold(lines, `set "PHY_IDX="`) && containsFold(lines, `set "IFACE_FILTER="`) {
		return lines
	}
	hasSetlocal := containsFold(lines, "setlocal enabledelayedexpansion")
	block := interfaceBlockWithoutSetlocal
	if !hasSetlocal {
		block = "setlocal enabledelayedexpansion\n" + block
	}
	insert := -1
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), `set "lists=%~dp0lists\"`) {
			insert = i + 1
			break
		}
	}
	if insert < 0 {
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), "cd /d %bin%") {
				insert = i
				break
			}
		}
	}
	if insert < 0 {
		return lines
	}
	blockLines := strings.Split(block, "\n")
	out := make([]string, 0, len(lines)+len(blockLines)+1)
	out = append(out, lines[:insert]...)
	if insert > 0 && strings.TrimSpace(lines[insert-1]) != "" {
		out = append(out, "")
	}
	out = append(out, blockLines...)
	if insert < len(lines) && strings.TrimSpace(lines[insert]) != "" {
		out = append(out, "")
	}
	out = append(out, lines[insert:]...)
	return out
}

var startExeRE = regexp.MustCompile(`(?i)("?%BIN%\\?winws2?\.exe"?)`)

func ensureIfaceArg(lines []string) []string {
	out := make([]string, len(lines))
	copy(out, lines)
	for i, line := range out {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "winws.exe") && !strings.Contains(lower, "winws2.exe") {
			continue
		}
		if strings.Contains(lower, "%iface_filter%") {
			continue
		}
		if idx := firstWFTCPIndex(line); idx >= 0 {
			out[i] = line[:idx] + "%IFACE_FILTER% " + line[idx:]
			continue
		}
		if loc := startExeRE.FindStringIndex(line); loc != nil {
			out[i] = line[:loc[1]] + " %IFACE_FILTER%" + line[loc[1]:]
		}
	}
	return out
}

func firstWFTCPIndex(line string) int {
	lower := strings.ToLower(line)
	indexes := []int{
		strings.Index(lower, "--wf-tcp-out"),
		strings.Index(lower, "--wf-tcp="),
		strings.Index(lower, "--wf-tcp "),
	}
	best := -1
	for _, idx := range indexes {
		if idx >= 0 && (best < 0 || idx < best) {
			best = idx
		}
	}
	return best
}

func containsFold(lines []string, needle string) bool {
	needle = strings.ToLower(needle)
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), needle) {
			return true
		}
	}
	return false
}

func fixEscapedExclamation(text string) string {
	return strings.ReplaceAll(text, "^!", "")
}

func fixEmptyArgs(text string) string {
	re1 := regexp.MustCompile(`--dpi-desync-fake-tls=[!^]`)
	text = re1.ReplaceAllString(text, "")
	re2 := regexp.MustCompile(`--dpi-desync-fake-tls=\s`)
	return re2.ReplaceAllString(text, "")
}

func fixMissingBinPrefix(text string) string {
	re := regexp.MustCompile(`--dpi-desync-fake-tls=([a-zA-Z0-9_]+\.bin)`)
	return re.ReplaceAllString(text, `--dpi-desync-fake-tls=%BIN%$1`)
}

func fixSingleDash(text string) string {
	re := regexp.MustCompile(`(?m)([^-])(-dpi-desync)`)
	return re.ReplaceAllString(text, "$1--dpi-desync")
}

var unsupportedArgs = []string{
	`--dpi-desync-fake-tls-mod=[^\s]*`,
	`--dpi-desync-fake-discord=[^\s]*`,
	`--dpi-desync-fake-stun=[^\s]*`,
	`--dpi-desync-split-pos=sld\+[^\s]*`,
	`--dpi-desync-split-pos=host\+[^\s]*`,
}

func removeUnsupportedArgs(text string) string {
	for _, pattern := range unsupportedArgs {
		re := regexp.MustCompile(pattern)
		text = re.ReplaceAllString(text, "")
	}
	return text
}

var hostlistCommaRe = regexp.MustCompile(`--hostlist=([^\s]+),([^\s]+)`)

func fixHostlistCommas(text string) string {
	for hostlistCommaRe.MatchString(text) {
		text = hostlistCommaRe.ReplaceAllStringFunc(text, func(match string) string {
			parts := strings.SplitN(match, "=", 2)
			if len(parts) != 2 {
				return match
			}
			vals := strings.Split(parts[1], ",")
			if len(vals) < 2 {
				return match
			}
			var result []string
			result = append(result, "--hostlist="+vals[0])
			result = append(result, "--hostlist-domains="+strings.Join(vals[1:], ","))
			return strings.Join(result, " ")
		})
	}
	return text
}

var missingLuaInits = []string{
	"fakemultisplit.lua",
	"fakemultidisorder.lua",
}

func removeMissingLuaInits(text string) string {
	for _, name := range missingLuaInits {
		re := regexp.MustCompile(`--lua-init=@[^\s]*` + regexp.QuoteMeta(name) + `\s*`)
		text = re.ReplaceAllString(text, " ")
	}
	return text
}

func FixArgs(text string) string {
	text = fixEmptyArgs(text)
	text = fixMissingBinPrefix(text)
	text = fixSingleDash(text)
	text = removeMissingLuaInits(text)
	text = removeUnsupportedArgs(text)
	text = fixHostlistCommas(text)
	return text
}

var nonASCIIRe = regexp.MustCompile(`[^\x00-\x7F]+`)

func sanitizeNameArgs(text string) string {
	text = nonASCIIRe.ReplaceAllString(text, "_")
	return fixNameSpaces(text)
}

func fixNameSpaces(text string) string {
	const prefix = "--name="
	searchFrom := 0
	for {
		idx := strings.Index(text[searchFrom:], prefix)
		if idx < 0 {
			break
		}
		absIdx := searchFrom + idx
		start := absIdx + len(prefix)
		end := start
		for end < len(text) {
			c := text[end]
			if c == ' ' {
				nextNonSpace := end + 1
				for nextNonSpace < len(text) && text[nextNonSpace] == ' ' {
					nextNonSpace++
				}
				if nextNonSpace < len(text) && text[nextNonSpace] == '-' {
					break
				}
				end = nextNonSpace
			} else {
				end++
			}
		}
		nameVal := text[start:end]
		fixed := strings.ReplaceAll(nameVal, " ", "_")
		text = text[:start] + fixed + text[end:]
		searchFrom = start + len(fixed)
	}
	return text
}

func isStartWinwsLine(line string) bool {
	l := strings.ToLower(line)
	return strings.HasPrefix(l, "start ") && (strings.Contains(l, "winws.exe") || strings.Contains(l, "winws2.exe"))
}

func wrapLongCommands(lines []string) []string {
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		lower := strings.TrimSpace(line)
		if !isStartWinwsLine(lower) {
			out = append(out, line)
			i++
			continue
		}

		full, originalLines := collectStartCommand(lines, i)
		i += len(originalLines)

		if estimateExpandedLen(full) <= wrapThreshold {
			out = append(out, originalLines...)
			continue
		}

		wrapper := generateArgsWrapper(full)
		out = append(out, wrapper...)
	}
	return out
}

func collectStartCommand(lines []string, startIdx int) (string, []string) {
	full := lines[startIdx]
	var collected []string
	collected = append(collected, lines[startIdx])
	idx := startIdx
	for strings.HasSuffix(strings.TrimSpace(full), "^") {
		idx++
		if idx >= len(lines) {
			break
		}
		trimmed := strings.TrimSpace(lines[idx])
		full = strings.TrimSuffix(strings.TrimSpace(full), "^") + " " + trimmed
		collected = append(collected, lines[idx])
	}
	return full, collected
}

func estimateExpandedLen(cmd string) int {
	l := len(cmd)
	binCount := strings.Count(cmd, "%BIN%")
	listsCount := strings.Count(cmd, "%LISTS%")
	l += binCount*75 + listsCount*75
	return l
}

var startCmdRE = regexp.MustCompile(`(?i)^(?:start\s+"zapret:[^"]*"\s+/min\s+)?\"([^\"]+)\"(.*)$`)

func generateArgsWrapper(fullCmd string) []string {
	m := startCmdRE.FindStringSubmatch(strings.TrimSpace(fullCmd))
	if m == nil {
		return []string{fullCmd}
	}
	args := strings.TrimSpace(m[2])
	exeRef := m[1]

	sections := strings.Split(args, " --new ")

	var result []string
	firstLine := true

	for i, section := range sections {
		chunks := splitArgsIntoChunks(section, wrapChunkSize)
		for _, chunk := range chunks {
			if firstLine {
				result = append(result, `set "ARGS=`+chunk+`"`)
				firstLine = false
			} else {
				result = append(result, `set "ARGS=!ARGS! `+chunk+`"`)
			}
		}
		if i < len(sections)-1 {
			result = append(result, `set "ARGS=!ARGS! --new"`)
		}
	}

	result = append(result, `"`+exeRef+`" !ARGS!`)
	return result
}

func splitArgsIntoChunks(args string, maxChunkSize int) []string {
	words := strings.Fields(args)
	var chunks []string
	current := ""
	for _, word := range words {
		candidate := current
		if candidate != "" {
			candidate += " "
		}
		candidate += word
		if len(candidate) > maxChunkSize && current != "" {
			chunks = append(chunks, current)
			current = word
		} else {
			current = candidate
		}
	}
	if current != "" {
		chunks = append(chunks, current)
	}
	return chunks
}
