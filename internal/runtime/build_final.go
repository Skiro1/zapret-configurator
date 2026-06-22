package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"zapret-configurator/internal/config"
	zlists "zapret-configurator/internal/lists"
)

func BuildFinal(opts config.Options) error {
	if opts.WantsZapret() {
		if err := buildFinalForEngine(opts, "zapret"); err != nil {
			return err
		}
	}
	if opts.WantsZapret2() {
		if err := buildFinalForEngine(opts, "zapret2"); err != nil {
			return err
		}
	}
	cleanDownloadedAfterBuild(opts)
	cleanConvertedAfterBuild(opts)
	return nil
}

func buildFinalForEngine(opts config.Options, engine string) error {
	fmt.Println("build final:", engine)
	finalDir := opts.FinalDir(engine)
	if err := EnsureCleanDir(finalDir); err != nil {
		return err
	}
	converted := opts.ConvertedDir(engine)
	if err := flattenConverted(converted, finalDir); err != nil {
		return err
	}
	switch engine {
	case "zapret":
		sourceRoot := filepath.Join(opts.DownloadedDir("zapret"), "flowseal")
		for _, name := range []string{"bin", "lists", "utils"} {
			if err := CopyDirIfExists(filepath.Join(sourceRoot, name), filepath.Join(finalDir, name)); err != nil {
				return err
			}
		}
		if err := CopyFileIfExists(filepath.Join(sourceRoot, "service.bat"), filepath.Join(finalDir, "service.bat")); err != nil {
			return err
		}
	case "zapret2":
		runtimeRoot := filepath.Join(opts.DownloadedDir("zapret2"), "runtime")
		if !HasFile(runtimeRoot, "winws2.exe") {
			return fmt.Errorf("zapret2 runtime missing winws2.exe under %s; run sync first", runtimeRoot)
		}
		for _, name := range []string{"exe", "bin", "lua", "windivert.filter", "lists"} {
			if err := CopyDirIfExists(filepath.Join(runtimeRoot, name), filepath.Join(finalDir, name)); err != nil {
				return err
			}
		}
		if err := CopyFileIfExists(filepath.Join(runtimeRoot, "winws2.exe"), filepath.Join(finalDir, "bin", "winws2.exe")); err != nil {
			return err
		}
		if err := ensureZapret2ExeLayout(finalDir); err != nil {
			return err
		}
		if err := CopyDirIfExists(filepath.Join("utils_z", "zapret2", "utils"), filepath.Join(finalDir, "utils")); err != nil {
			return err
		}
		if err := CopyFileIfExists(filepath.Join("utils_z", "zapret2", "service2.bat"), filepath.Join(finalDir, "service2.bat")); err != nil {
			return err
		}
		if err := CopyFileIfExists(filepath.Join("utils_z", "zapret2", "test_zapret2.ps1"), filepath.Join(finalDir, "utils", "test_zapret2.ps1")); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(finalDir, "utils", "test results"), 0o755); err != nil {
			return err
		}
	}
	listsSource := findListsSource(opts)
	if err := zlists.EnsureLists(finalDir, listsSource); err != nil {
		return err
	}
	restoreIpsetAllFromBackup(finalDir)
	patchMissingBinRefs(finalDir)
	return nil
}

func restoreIpsetAllFromBackup(finalDir string) {
	listsDir := filepath.Join(finalDir, "lists")
	ipsetPath := filepath.Join(listsDir, "ipset-all.txt")
	backupPath := filepath.Join(listsDir, "ipset-all.txt.backup")
	data, err := os.ReadFile(ipsetPath)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > 100 {
		return
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		return
	}
	_ = os.WriteFile(ipsetPath, backup, 0o644)
}

func flattenConverted(srcDir, dstDir string) error {
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		baseName := filepath.Base(rel)
		baseName = sanitizeBatFilename(baseName)
		dst := filepath.Join(dstDir, baseName)
		if d.IsDir() {
			return nil
		}
		return CopyFile(path, dst)
	})
}

func sanitizeBatFilename(name string) string {
	name = strings.ReplaceAll(name, "&", "and")
	return name
}

func cleanDownloadedAfterBuild(opts config.Options) {
	_ = os.RemoveAll(filepath.Join(opts.OutputDir, "_downloaded"))
}

func cleanConvertedAfterBuild(opts config.Options) {
	_ = os.RemoveAll(filepath.Join(opts.OutputDir, "_converted"))
}

func findListsSource(opts config.Options) string {
	candidates := []string{
		filepath.Join(opts.OutputDir, "lists_source"),
		filepath.Join(opts.OutputDir, "..", "lists_source"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}

func patchMissingBinRefs(finalDir string) {
	binDir := filepath.Join(finalDir, "bin")
	available := make(map[string]bool)
	if entries, err := os.ReadDir(binDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				available[strings.ToLower(e.Name())] = true
			}
		}
	}
	fallbackTLS := "tls_clienthello_www_google_com.bin"
	fallbackQUIC := "quic_initial_www_google_com.bin"
	_ = filepath.WalkDir(finalDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".bat") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		modified := false

		// Fix bare bin filenames without %BIN% prefix (e.g. --dpi-desync-fake-tls=tls_clienthello_18.bin)
		for _, re := range bareBinFixers {
			newContent := re.ReplaceAllStringFunc(content, func(match string) string {
				eqIdx := strings.Index(match, "=")
				binName := match[eqIdx+1:]
				if available[strings.ToLower(binName)] {
					return match[:eqIdx+1] + "%BIN%" + binName
				}
				fb := fallbackTLS
				if strings.HasPrefix(strings.ToLower(binName), "quic") {
					fb = fallbackQUIC
				}
				return match[:eqIdx+1] + "%BIN%" + fb
			})
			if newContent != content {
				content = newContent
				modified = true
			}
		}

		// Fix existing %BIN% references to missing bins
		prefix := "%BIN%"
		offset := 0
		for {
			idx := strings.Index(content[offset:], prefix)
			if idx == -1 {
				break
			}
			absIdx := offset + idx
			start := absIdx + len(prefix)
			end := start
			for end < len(content) {
				c := content[end]
				if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '"' || c == '^' || c == '%' || c == '(' {
					break
				}
				end++
			}
			if end > start {
				binName := content[start:end]
				if !available[strings.ToLower(binName)] {
					replacement := fallbackTLS
					if strings.HasPrefix(strings.ToLower(binName), "quic") {
						replacement = fallbackQUIC
					}
					content = content[:absIdx] + prefix + replacement + content[end:]
					modified = true
					offset = absIdx + len(prefix) + len(replacement)
				} else {
					offset = end
				}
			} else {
				offset = start
			}
		}
		if modified {
			_ = os.WriteFile(path, []byte(content), 0o644)
		}
		return nil
	})
}

var bareBinFixers = []*regexp.Regexp{
	regexp.MustCompile(`--dpi-desync-fake-tls=([a-zA-Z0-9_]+\.bin)`),
	regexp.MustCompile(`--dpi-desync-fake-quic=([a-zA-Z0-9_]+\.bin)`),
	regexp.MustCompile(`--dpi-desync-fake=([a-zA-Z0-9_]+\.bin)`),
}

func ensureZapret2ExeLayout(finalDir string) error {
	exePath := filepath.Join(finalDir, "bin", "winws2.exe")
	if _, err := os.Stat(exePath); err == nil {
		return nil
	}
	if found, ok := FindFile(finalDir, "winws2.exe"); ok {
		return CopyFile(found, exePath)
	}
	return fmt.Errorf("winws2.exe not found after final assembly")
}
