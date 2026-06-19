package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func PrepareZapret2RuntimeFromZip(zipExtractDir, runtimeDir string) error {
	if err := EnsureCleanDir(runtimeDir); err != nil {
		return err
	}
	// The zip extracts to zapret-kvn-main/zapret/...
	// Find the zapret/ directory inside the extracted tree
	zapretDir := findZapretDir(zipExtractDir)
	if zapretDir == "" {
		return fmt.Errorf("zapret/ directory not found in %s", zipExtractDir)
	}
	// Copy exe/ to runtime root (for DLLs and winws2.exe)
	if err := CopyDirIfExists(filepath.Join(zapretDir, "exe"), filepath.Join(runtimeDir, "exe")); err != nil {
		return err
	}
	// Copy bin/, lua/, lists/, windivert.filter/ to runtime root
	for _, d := range []string{"bin", "lua", "lists", "windivert.filter"} {
		if err := CopyDirIfExists(filepath.Join(zapretDir, d), filepath.Join(runtimeDir, d)); err != nil {
			return err
		}
	}
	// Copy winws2.exe to root for convenience
	_ = CopyFileIfExists(filepath.Join(zapretDir, "exe", "winws2.exe"), filepath.Join(runtimeDir, "winws2.exe"))
	// Copy DLLs to bin/ so winws2.exe can run from there
	binDir := filepath.Join(runtimeDir, "bin")
	for _, f := range []string{"cygwin1.dll", "WinDivert.dll", "Monkey64.sys"} {
		src := filepath.Join(zapretDir, "exe", f)
		if _, err := os.Stat(src); err == nil {
			_ = CopyFileIfExists(src, filepath.Join(binDir, f))
		}
	}
	if !HasFile(runtimeDir, "winws2.exe") {
		return fmt.Errorf("winws2.exe not found after copying from zip")
	}
	return nil
}

func findZapretDir(root string) string {
	// Check if root itself contains exe/ and bin/
	if dirExists(filepath.Join(root, "exe")) && dirExists(filepath.Join(root, "bin")) {
		return root
	}
	// Search one level deep
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			p := filepath.Join(root, e.Name())
			if dirExists(filepath.Join(p, "exe")) && dirExists(filepath.Join(p, "bin")) {
				return p
			}
			// Two levels deep (zapret-kvn-main/zapret/)
			sub, _ := os.ReadDir(p)
			for _, s := range sub {
				if s.IsDir() {
					sp := filepath.Join(p, s.Name())
					if dirExists(filepath.Join(sp, "exe")) && dirExists(filepath.Join(sp, "bin")) {
						return sp
					}
				}
			}
		}
	}
	return ""
}

func PrepareZapret2Runtime(ctx context.Context, installerPath, runtimeDir string) error {
	if HasFile(runtimeDir, "winws2.exe") {
		return nil
	}
	workDir := filepath.Join(filepath.Dir(runtimeDir), "_runtime_extract")
	if err := EnsureCleanDir(workDir); err != nil {
		return err
	}
	exe7z, found7z := find7z()
	if found7z {
		fmt.Println("extracting with 7z:", exe7z)
		cmd := exec.CommandContext(ctx, exe7z, "x", "-y", "-o"+workDir, installerPath)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err == nil && HasFile(workDir, "winws2.exe") {
			return copyZapret2RuntimeFromTree(workDir, runtimeDir)
		}
	}
	fmt.Println("7z extraction failed, trying Inno Setup silent install...")
	if err := extractInnoSilent(ctx, installerPath, workDir); err != nil {
		return fmt.Errorf("failed to extract Zapret2Setup: %v", err)
	}
	return copyZapret2RuntimeFromTree(workDir, runtimeDir)
}

func find7z() (string, bool) {
	for _, name := range []string{"7z.exe", "7za.exe", "7zr.exe"} {
		exe, err := exec.LookPath(name)
		if err == nil {
			return exe, true
		}
	}
	for _, p := range []string{
		`C:\Program Files\7-Zip\7z.exe`,
		`C:\Program Files (x86)\7-Zip\7z.exe`,
		filepath.Join(os.Getenv("ProgramFiles"), "7-Zip", "7z.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "7-Zip", "7z.exe"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

func copyZapret2RuntimeFromTree(srcRoot, runtimeDir string) error {
	if err := EnsureCleanDir(runtimeDir); err != nil {
		return err
	}
	winws2, ok := FindFile(srcRoot, "winws2.exe")
	if !ok {
		return fmt.Errorf("winws2.exe not found under %s", srcRoot)
	}
	base := selectRuntimeBase(srcRoot, winws2)

	for _, dirName := range []string{"exe", "bin", "lua", "windivert.filter", "lists"} {
		src := filepath.Join(base, dirName)
		if err := CopyDirIfExists(src, filepath.Join(runtimeDir, dirName)); err != nil {
			return err
		}
	}
	if err := CopyFileIfExists(filepath.Join(base, "winws2.exe"), filepath.Join(runtimeDir, "winws2.exe")); err != nil {
		return err
	}
	if err := CopyFileIfExists(winws2, filepath.Join(runtimeDir, "winws2.exe")); err != nil {
		return err
	}
	if err := CopyFileIfExists(filepath.Join(base, "exe", "winws2.exe"), filepath.Join(runtimeDir, "exe", "winws2.exe")); err != nil {
		return err
	}

	// Copy required DLLs and driver files to bin/ so winws2.exe can run from there
	binDir := filepath.Join(runtimeDir, "bin")
	for _, f := range []string{"cygwin1.dll", "WinDivert.dll", "WinDivert.sys", "Monkey64.sys", "aaaaaaa1", "stop.bat"} {
		src := filepath.Join(base, "exe", f)
		if _, err := os.Stat(src); err == nil {
			_ = CopyFileIfExists(src, filepath.Join(binDir, f))
		}
	}

	dirs := FindDirs(srcRoot, "lua", "windivert.filter", "bin", "lists", "exe")
	for name, candidates := range dirs {
		if len(candidates) == 0 {
			continue
		}
		dst := filepath.Join(runtimeDir, name)
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		if err := CopyDir(candidates[0], dst); err != nil {
			return err
		}
	}
	if !HasFile(runtimeDir, "winws2.exe") {
		return fmt.Errorf("copied zapret2 runtime, but winws2.exe is still missing under %s", runtimeDir)
	}
	return nil
}

func selectRuntimeBase(root, winws2Path string) string {
	dir := filepath.Dir(winws2Path)
	if strings.EqualFold(filepath.Base(dir), "exe") {
		return filepath.Dir(dir)
	}
	for {
		if dir == root || dir == filepath.Dir(dir) {
			return dir
		}
		hasLua := dirExists(filepath.Join(dir, "lua"))
		hasBin := dirExists(filepath.Join(dir, "bin"))
		hasExe := dirExists(filepath.Join(dir, "exe"))
		if hasLua || hasBin || hasExe {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func extractInnoSilent(ctx context.Context, installerPath, workDir string) error {
	if err := EnsureCleanDir(workDir); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, installerPath, "/SILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/DIR="+workDir)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start installer: %w", err)
	}
	// Inno Setup spawns a child process; wait for the child to finish
	time.Sleep(15 * time.Second)
	_ = cmd.Wait()
	return nil
}

func CleanupWinDivert(ctx context.Context) {
	for _, service := range []string{"WinDivert", "windivert", "WinDivert14"} {
		_ = exec.CommandContext(ctx, "sc", "query", service).Run()
		_ = exec.CommandContext(ctx, "sc", "stop", service).Run()
		_ = exec.CommandContext(ctx, "sc", "delete", service).Run()
	}
}
