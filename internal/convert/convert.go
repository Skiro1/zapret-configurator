package convert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zapret-configurator/internal/bat"
	"zapret-configurator/internal/config"
	zruntime "zapret-configurator/internal/runtime"
)

var skippedConfigs = map[string]bool{
	"general (FAKE TLS AUTO).bat":    true,
	"general FAKE TLS AUTO 1.9.bat": true,
}

func ConvertAll(opts config.Options) error {
	if opts.WantsZapret() {
		if err := convertZapret(opts); err != nil {
			return err
		}
	}
	if opts.WantsZapret2() {
		if err := convertZapret2(opts); err != nil {
			return err
		}
	}
	return nil
}

func convertZapret(opts config.Options) error {
	fmt.Println("convert zapret")
	outDir := opts.ConvertedDir("zapret")
	if err := zruntime.EnsureCleanDir(outDir); err != nil {
		return err
	}
	flowseal := filepath.Join(opts.DownloadedDir("zapret"), "flowseal")
	if err := convertFlowsealBats(flowseal, outDir, bat.EngineZapret); err != nil {
		return err
	}
	winws1 := filepath.Join(opts.DownloadedDir("zapret"), "youtubediscord", "src", "presets", "builtin", "winws1")
	if err := convertPresetDir(winws1, filepath.Join(outDir, "youtubediscord"), "winws.exe", bat.EngineZapret); err != nil {
		return err
	}
	return nil
}

func convertZapret2(opts config.Options) error {
	fmt.Println("convert zapret2")
	outDir := opts.ConvertedDir("zapret2")
	if err := zruntime.EnsureCleanDir(outDir); err != nil {
		return err
	}
	winws2 := filepath.Join(opts.DownloadedDir("zapret2"), "youtubediscord", "src", "presets", "builtin", "winws2")
	if err := convertPresetDir(winws2, filepath.Join(outDir, "youtubediscord"), "winws2.exe", bat.EngineZapret2); err != nil {
		return err
	}
	return nil
}

func convertFlowsealBats(srcDir, dstDir string, engine bat.Engine) error {
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".bat") || strings.HasPrefix(strings.ToLower(d.Name()), "service") {
			return nil
		}
		if skippedConfigs[d.Name()] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(dstDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte(bat.Patch(string(data), engine)), 0o644)
	})
}

func convertPresetDir(srcDir, dstDir, exeName string, engine bat.Engine) error {
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".txt") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name := zruntime.SafeFilename(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))) + ".bat"
		if skippedConfigs[name] {
			return nil
		}
		dst := filepath.Join(dstDir, name)
		content := PresetTextToBat(string(data), exeName)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte(bat.Patch(content, engine)), 0o644)
	})
}

func PresetTextToBat(presetText, exeName string) string {
	args := presetLinesToArgs(presetText)
	isZapret2 := strings.EqualFold(exeName, "winws2.exe")
	var b strings.Builder
	b.WriteString("@echo off\r\n")
	b.WriteString("chcp 65001 > nul\r\n")
	b.WriteString(":: 65001 - UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString("cd /d \"%~dp0\"\r\n")
	if isZapret2 {
	} else {
		b.WriteString("call service.bat status_zapret\r\n")
		b.WriteString("call service.bat check_updates\r\n")
		b.WriteString("call service.bat load_game_filter\r\n")
		b.WriteString("call service.bat load_user_lists\r\n")
		b.WriteString("echo:\r\n")
		b.WriteString("\r\n")
	}
	b.WriteString("set \"BIN=%~dp0bin\\\"\r\n")
	b.WriteString("set \"LISTS=%~dp0lists\\\"\r\n")
	b.WriteString("\r\n")
	b.WriteString("\"%BIN%")
	b.WriteString(exeName)
	b.WriteString("\" ")
	if len(args) == 0 {
		b.WriteString("--wf-tcp=80,443")
	} else {
		argsText := strings.Join(args, " ")
		argsText = bat.FixArgs(argsText)
		writeWrappedArgs(&b, strings.Fields(argsText))
	}
	b.WriteString("\r\n")
	return b.String()
}

func presetLinesToArgs(text string) []string {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	var args []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := splitInlineArgs(line)
		args = append(args, parts...)
	}
	return args
}

func splitInlineArgs(line string) []string {
	if !strings.HasPrefix(line, "--") {
		return []string{line}
	}
	fields := strings.Fields(line)
	if len(fields) <= 1 {
		return []string{line}
	}
	var parts []string
	var current strings.Builder
	for _, field := range fields {
		if strings.HasPrefix(field, "--") && current.Len() > 0 {
			parts = append(parts, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(field)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func writeWrappedArgs(b *strings.Builder, args []string) {
	for i, arg := range args {
		if i > 0 {
			if i%6 == 0 {
				b.WriteString(" ^\r\n")
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteString(batchQuoteArg(arg))
	}
}

func batchQuoteArg(arg string) string {
	replacer := strings.NewReplacer(
		"@bin/", "@%BIN%",
		"@bin\\", "@%BIN%",
		"bin/", "%BIN%",
		"bin\\", "%BIN%",
		"@lists/", "@%LISTS%",
		"@lists\\", "@%LISTS%",
		"lists/", "%LISTS%",
		"lists\\", "%LISTS%",
		"@lua/", "@%~dp0lua\\",
		"@lua\\", "@%~dp0lua\\",
		"lua/", "%~dp0lua\\",
		"lua\\", "%~dp0lua\\",
		"@windivert.filter/", "@%~dp0windivert.filter\\",
		"@windivert.filter\\", "@%~dp0windivert.filter\\",
		"windivert.filter/", "%~dp0windivert.filter\\",
		"windivert.filter\\", "%~dp0windivert.filter\\",
	)
	return replacer.Replace(arg)
}
