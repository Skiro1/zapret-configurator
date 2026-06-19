package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zapret-configurator/internal/config"
	zruntime "zapret-configurator/internal/runtime"
)

func Sync(ctx context.Context, opts config.Options) error {
	SetGitHubToken(opts.GitHubToken)
	if err := zruntime.EnsureDir(opts.OutputDir); err != nil {
		return err
	}
	if opts.WantsZapret() {
		if err := syncFlowseal(ctx, opts); err != nil {
			return err
		}
	}
	if opts.WantsZapret() || opts.WantsZapret2() {
		if err := syncYDSource(ctx, opts); err != nil {
			return err
		}
	}
	if opts.WantsZapret2() {
		if err := syncZapret2InstallerAndRuntime(ctx, opts); err != nil {
			return err
		}
	}
	cleanPostSync(opts)
	return nil
}

func cleanPostSync(opts config.Options) {
	globalCache := filepath.Join(opts.OutputDir, "_downloaded", "_cache")
	_ = os.RemoveAll(globalCache)
}

func syncFlowseal(ctx context.Context, opts config.Options) error {
	fmt.Println("sync flowseal")
	rel, err := latestRelease(ctx, flowsealRepoAPI)
	if err != nil {
		return err
	}
	var chosen asset
	for _, item := range rel.Assets {
		if strings.HasSuffix(strings.ToLower(item.Name), ".zip") {
			chosen = item
			break
		}
	}
	if chosen.BrowserDownloadURL == "" {
		return fmt.Errorf("flowseal release %s has no .zip asset", rel.TagName)
	}
	root := opts.DownloadedDir("zapret")
	cache := filepath.Join(root, "_cache")
	if err := zruntime.EnsureDir(cache); err != nil {
		return err
	}
	zipPath := filepath.Join(cache, chosen.Name)
	if err := cachedDownload(ctx, chosen.BrowserDownloadURL, zipPath); err != nil {
		return err
	}
	sourceDir := filepath.Join(root, "flowseal")
	if err := zruntime.EnsureCleanDir(sourceDir); err != nil {
		return err
	}
	return extractZip(zipPath, sourceDir, nil)
}

func syncYDSource(ctx context.Context, opts config.Options) error {
	fmt.Println("sync youtubediscord presets")
	cache := filepath.Join(opts.OutputDir, "_downloaded", "_cache")
	if err := zruntime.EnsureDir(cache); err != nil {
		return err
	}
	zipPath := filepath.Join(cache, "youtubediscord-zapret-main.zip")
	if err := cachedDownload(ctx, ydMainZipURL, zipPath); err != nil {
		return err
	}
	tmp := filepath.Join(cache, "youtubediscord-zapret-main")
	if err := zruntime.EnsureCleanDir(tmp); err != nil {
		return err
	}
	prefixes := []string{
		"src/presets/builtin/winws1",
		"src/presets/builtin/winws2",
		"src/profile/strategy_catalogs",
	}
	if err := extractZip(zipPath, tmp, prefixes); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.DownloadedDir("zapret"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.DownloadedDir("zapret2"), 0o755); err != nil {
		return err
	}
	return copySelectedYD(tmp, opts.DownloadedDir("zapret"), opts.DownloadedDir("zapret2"))
}

func syncZapret2InstallerAndRuntime(ctx context.Context, opts config.Options) error {
	fmt.Println("sync zapret2 runtime from zapret-kvn")
	runtimeDir := filepath.Join(opts.DownloadedDir("zapret2"), "runtime")
	if zruntime.HasFile(runtimeDir, "winws2.exe") {
		fmt.Println("zapret2 runtime cache hit:", runtimeDir)
		return nil
	}
	cache := filepath.Join(opts.DownloadedDir("zapret2"), "_cache")
	if err := zruntime.EnsureDir(cache); err != nil {
		return err
	}
	zipPath := filepath.Join(cache, "zapret-kvn-main.zip")
	if err := cachedDownload(ctx, zapretKvnZipURL, zipPath); err != nil {
		return err
	}
	tmpDir := filepath.Join(cache, "zapret-kvn-extract")
	if err := zruntime.EnsureCleanDir(tmpDir); err != nil {
		return err
	}
	prefixes := []string{"zapret/"}
	if err := extractZip(zipPath, tmpDir, prefixes); err != nil {
		return err
	}
	return zruntime.PrepareZapret2RuntimeFromZip(tmpDir, runtimeDir)
}
