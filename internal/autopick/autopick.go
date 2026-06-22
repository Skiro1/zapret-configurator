package autopick

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"zapret-configurator/internal/catalog"
	"zapret-configurator/internal/config"
	"zapret-configurator/internal/probes"
	"zapret-configurator/internal/report"
	zruntime "zapret-configurator/internal/runtime"
)

const (
	configStartTimeout = 15 * time.Second
	configReadyTimeout = 8 * time.Second
	configWarmupDelay  = 5 * time.Second
	configSettleDelay  = 500 * time.Millisecond
)

func Run(ctx context.Context, opts config.Options) error {
	var all []report.AutopickResult
	if opts.WantsZapret() {
		results, err := runForEngine(ctx, opts, "zapret", "winws.exe")
		if err != nil {
			return err
		}
		all = append(all, results...)
	}
	if opts.WantsZapret2() {
		results, err := runForEngine(ctx, opts, "zapret2", "winws2.exe")
		if err != nil {
			return err
		}
		all = append(all, results...)
	}
	if err := catalogScanAndTest(ctx, opts, &all); err != nil {
		fmt.Println("catalog scan warning:", err)
	}
	report.ScoreResults(all)
	report.SortByScore(all)
	printTopResults(all, opts.Top)
	return report.WriteAutopickReport(filepath.Join(opts.OutputDir, "final", "autopick"), report.NewAutopickReport(opts.Target, opts.Mode, all))
}

func printTopResults(results []report.AutopickResult, top int) {
	if len(results) == 0 {
		fmt.Println("no results")
		return
	}
	if top <= 0 {
		top = 10
	}
	if top > len(results) {
		top = len(results)
	}
	best := results[0]
	fmt.Println()
	fmt.Println("========================================================")
	fmt.Printf("  BEST CONFIG: %s  (score=%.0f)\n", filepath.Base(best.ConfigPath), best.Score)
	fmt.Printf("  %s\n", probeSummary(best.Probes))
	fmt.Printf("  %s\n", best.ConfigPath)
	fmt.Println("========================================================")
	if top > 1 {
		fmt.Println()
		fmt.Println("TOP RESULTS:")
		for i := 0; i < top; i++ {
			r := results[i]
			marker := "  "
			if i == 0 {
				marker = "* "
			}
			fmt.Printf("%s#%-2d score=%-4.0f  %-45s  %s\n", marker, i+1, r.Score, filepath.Base(r.ConfigPath), probeSummary(r.Probes))
		}
	}
	fmt.Println()
}

func probeSummary(probes []report.ProbeResult) string {
	var parts []string
	for _, p := range probes {
		if p.Success {
			parts = append(parts, fmt.Sprintf("%s=%.0fms", p.Kind, p.LatencyMS))
		} else {
			parts = append(parts, fmt.Sprintf("%s=FAIL", p.Kind))
		}
	}
	return strings.Join(parts, " ")
}

func runForEngine(ctx context.Context, opts config.Options, engine, exeName string) ([]report.AutopickResult, error) {
	finalDir := opts.FinalDir(engine)
	if _, err := os.Stat(finalDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s final directory not found: %s; run build-final first", engine, finalDir)
	}
	files, err := collectBatFiles(finalDir)
	if err != nil {
		return nil, err
	}
	files = limitByMode(files, opts.Mode)
	fmt.Printf("autopick %s: %d configs\n", engine, len(files))

	var results []report.AutopickResult
	for i, file := range files {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}
		fmt.Printf("[%s %d/%d] %s\n", engine, i+1, len(files), filepath.Base(file))
		result := testConfig(ctx, engine, exeName, file, opts.Target)
		results = append(results, result)
		cleanupAfterRun(ctx, exeName)
		time.Sleep(configSettleDelay)
	}
	if err := copyTop(results, opts.AutopickDir(engine), opts.Top, finalDir, engine); err != nil {
		return results, err
	}
	return results, nil
}

func testConfig(ctx context.Context, engine, exeName, batPath, target string) report.AutopickResult {
	result := report.AutopickResult{
		Engine:     engine,
		ConfigPath: batPath,
	}

	cmd := exec.CommandContext(ctx, "cmd.exe", "/c", batPath)
	cmd.Dir = filepath.Dir(batPath)
	cmd.SysProcAttr = hideWindow()
	if err := cmd.Start(); err != nil {
		result.Error = err.Error()
		return result
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
	case <-time.After(configStartTimeout):
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}

	if exeName != "" && !waitForProcessRunning(ctx, exeName, configReadyTimeout) {
		fmt.Printf("warning: %s did not appear within %s\n", exeName, configReadyTimeout)
	}

	time.Sleep(configWarmupDelay)
	result.Probes = probes.RunAll(ctx, target)
	result.Success = anyProbeSuccess(result.Probes)
	return result
}

func catalogScanAndTest(ctx context.Context, opts config.Options, all *[]report.AutopickResult) error {
	if !opts.WantsZapret2() {
		return nil
	}
	catalogsDir := filepath.Join(opts.DownloadedDir("zapret2"), "youtubediscord", "src", "profile", "strategy_catalogs")
	if _, err := os.Stat(catalogsDir); os.IsNotExist(err) {
		return nil
	}
	strategies, err := catalog.ScanCatalogs(catalogsDir, "winws2")
	if err != nil {
		return fmt.Errorf("scan catalogs: %w", err)
	}
	if len(strategies) == 0 {
		return nil
	}
	quick := catalog.FilterByLabel(strategies, "recommended")
	if len(quick) == 0 {
		quick = strategies
	}
	if opts.Mode == "quick" && len(quick) > 30 {
		quick = quick[:30]
	}
	fmt.Printf("catalog scan zapret2: %d strategies (testing %d)\n", len(strategies), len(quick))
	runtimeDir := filepath.Join(opts.DownloadedDir("zapret2"), "runtime")
	binDir := filepath.Join(runtimeDir, "bin")
	luaDir := filepath.Join(runtimeDir, "lua")
	listsDir := filepath.Join(runtimeDir, "lists")
	tmpDir := filepath.Join(opts.OutputDir, "_tmp_catalog")
	if err := zruntime.EnsureCleanDir(tmpDir); err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	for i, strat := range quick {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		fmt.Printf("[catalog %d/%d] %s\n", i+1, len(quick), strat.Name)
		batPath := writeCatalogPreset(tmpDir, strat, binDir, luaDir, listsDir)
		result := report.AutopickResult{
			Engine:     "zapret2",
			ConfigPath: batPath,
		}
		cmd := exec.CommandContext(ctx, "cmd.exe", "/c", batPath)
		cmd.Dir = tmpDir
		cmd.SysProcAttr = hideWindow()
		if err := cmd.Start(); err != nil {
			result.Error = err.Error()
			*all = append(*all, result)
			continue
		}
		_ = cmd.Wait()
		if !waitForProcessRunning(ctx, "winws2.exe", configReadyTimeout) {
			fmt.Println("warning: winws2.exe did not become visible after catalog preset start")
		}
		time.Sleep(configWarmupDelay)
		result.Probes = probes.RunAll(ctx, opts.Target)
		result.Success = anyProbeSuccess(result.Probes)
		*all = append(*all, result)
		cleanupAfterRun(ctx, "winws2.exe")
		time.Sleep(time.Second)
	}
	return nil
}

func writeCatalogPreset(tmpDir string, strat catalog.Strategy, binDir, luaDir, listsDir string) string {
	var b strings.Builder
	b.WriteString("@echo off\r\n")
	b.WriteString("chcp 65001 > nul\r\n")
	b.WriteString(":: 65001 - UTF-8\r\n")
	b.WriteString("set \"BIN=" + escapePath(binDir) + "\\\"\r\n")
	b.WriteString("set \"LISTS=" + escapePath(listsDir) + "\\\"\r\n")
	b.WriteString("setlocal enabledelayedexpansion\r\n")
	b.WriteString("set \"PHY_IDX=\"\r\n")
	b.WriteString("for /f \"skip=1 tokens=1\" %%a in ('netsh int ip show interfaces 2^>nul ^| findstr /i \"connected\"') do (\r\n")
	b.WriteString("    if not defined PHY_IDX set \"PHY_IDX=%%a\"\r\n")
	b.WriteString(")\r\n")
	b.WriteString("set \"IFACE_FILTER=\"\r\n")
	b.WriteString("if defined PHY_IDX set \"IFACE_FILTER=--wf-iface=!PHY_IDX!\"\r\n")
	b.WriteString("\r\n")
	b.WriteString("cd /d %BIN%\r\n\r\n")
	b.WriteString("start \"zapret: %~n0\" /min \"%BIN%winws2.exe\" %IFACE_FILTER%")
	if len(luaDir) > 0 {
		luaBase := escapePath(luaDir)
		b.WriteString(" --lua-init=@" + luaBase + "\\zapret-lib.lua")
		b.WriteString(" --lua-init=@" + luaBase + "\\zapret-antidpi.lua")
		b.WriteString(" --lua-init=@" + luaBase + "\\zapret-auto.lua")
	}
	for _, arg := range strat.Args {
		b.WriteString(" ")
		b.WriteString(normalizeArgForBat(arg, binDir, listsDir))
	}
	b.WriteString("\r\n")
	name := safeFilename(strat.ID) + ".bat"
	path := filepath.Join(tmpDir, name)
	_ = os.WriteFile(path, []byte(b.String()), 0o644)
	return path
}

func normalizeArgForBat(arg, binDir, listsDir string) string {
	arg = strings.ReplaceAll(arg, "bin/", "%BIN%")
	arg = strings.ReplaceAll(arg, "bin\\", "%BIN%")
	arg = strings.ReplaceAll(arg, "lists/", "%LISTS%")
	arg = strings.ReplaceAll(arg, "lists\\", "%LISTS%")
	arg = strings.ReplaceAll(arg, "@bin/", "@%BIN%")
	arg = strings.ReplaceAll(arg, "@bin\\", "@%BIN%")
	arg = strings.ReplaceAll(arg, "@lists/", "@%LISTS%")
	arg = strings.ReplaceAll(arg, "@lists\\", "@%LISTS%")
	return arg
}

func escapePath(p string) string {
	return strings.ReplaceAll(p, "/", "\\")
}

func safeFilename(name string) string {
	unsafe := []byte(`<>:"/\|?*`)
	for _, c := range unsafe {
		name = strings.ReplaceAll(name, string(c), "_")
	}
	name = strings.TrimSpace(name)
	if len(name) > 100 {
		name = name[:100]
	}
	if name == "" {
		name = "preset"
	}
	return name
}

func collectBatFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.EqualFold(d.Name(), "autopick") {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.HasSuffix(name, ".bat") && !strings.HasPrefix(name, "service") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func limitByMode(files []string, mode string) []string {
	limit := len(files)
	switch strings.ToLower(mode) {
	case "quick":
		limit = 30
	case "standard":
		limit = 80
	case "full":
		limit = len(files)
	}
	if len(files) < limit {
		return files
	}
	return files[:limit]
}

func anyProbeSuccess(probes []report.ProbeResult) bool {
	for _, p := range probes {
		if p.Success {
			return true
		}
	}
	return false
}

func cleanupAfterRun(ctx context.Context, exeName string) {
	killProcess(ctx, exeName)
	_ = waitForProcessExit(ctx, exeName, 5*time.Second)
	zruntime.CleanupWinDivert(ctx)
}

func waitForProcessRunning(ctx context.Context, exeName string, timeout time.Duration) bool {
	exeName = normalizeExeName(exeName)
	if exeName == "" {
		return false
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		if processRunning(exeName) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-ticker.C:
		}
	}
}

func waitForProcessExit(ctx context.Context, exeName string, timeout time.Duration) bool {
	exeName = normalizeExeName(exeName)
	if exeName == "" {
		return true
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		if !processRunning(exeName) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-ticker.C:
		}
	}
}

func processRunning(exeName string) bool {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq "+exeName).Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(exeName))
}

func normalizeExeName(exeName string) string {
	exeName = strings.TrimSpace(exeName)
	if exeName == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(exeName), ".exe") {
		exeName += ".exe"
	}
	return exeName
}

func killProcess(ctx context.Context, exeName string) {
	exeName = normalizeExeName(exeName)
	if exeName == "" {
		return
	}
	_ = exec.CommandContext(ctx, "taskkill", "/IM", exeName, "/T", "/F").Run()
}

func copyTop(results []report.AutopickResult, dstDir string, top int, finalDir, engine string) error {
	if err := zruntime.EnsureCleanDir(dstDir); err != nil {
		return err
	}

	// Copy support directories
	var dirsToCopy []string
	switch engine {
	case "zapret":
		dirsToCopy = []string{"bin", "lists", "utils"}
	case "zapret2":
		dirsToCopy = []string{"bin", "exe", "lists", "lua", "windivert.filter"}
	}
	for _, dir := range dirsToCopy {
		src := filepath.Join(finalDir, dir)
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(dstDir, dir)
			if err := zruntime.CopyDir(src, dst); err != nil {
				fmt.Printf("warning: copy %s: %v\n", dir, err)
			}
		}
	}

	// Copy top bat files
	var working []report.AutopickResult
	for _, result := range results {
		if result.Success {
			working = append(working, result)
		}
	}
	report.ScoreResults(working)
	report.SortByScore(working)
	if top > len(working) {
		top = len(working)
	}
	for i := 0; i < top; i++ {
		src := working[i].ConfigPath
		if src == "" {
			continue
		}
		name := fmt.Sprintf("%02d_%s", i+1, filepath.Base(src))
		if err := zruntime.CopyFile(src, filepath.Join(dstDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func hideWindow() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}
