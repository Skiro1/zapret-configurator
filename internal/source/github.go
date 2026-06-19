package source

import (
	"archive/zip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	zruntime "zapret-configurator/internal/runtime"
)

const (
	flowsealRepoAPI = "https://api.github.com/repos/Flowseal/zapret-discord-youtube/releases/latest"
	ydRepoAPI       = "https://api.github.com/repos/youtubediscord/zapret/releases/latest"
	ydMainZipURL    = "https://github.com/youtubediscord/zapret/archive/refs/heads/main.zip"
	zapretKvnZipURL = "https://github.com/youtubediscord/zapret-kvn/archive/refs/heads/main.zip"
)

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func latestRelease(ctx context.Context, url string) (release, error) {
	var result release
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("User-Agent", "zapret-configurator")
	if githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+githubToken)
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return result, fmt.Errorf("GitHub API %s returned %s", url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, err
	}
	return result, nil
}

var downloadClient = &http.Client{
	Timeout: 5 * time.Minute,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		DisableKeepAlives:     true,
	},
}

var githubToken string

func SetGitHubToken(token string) {
	githubToken = token
}

func downloadFile(ctx context.Context, url, dst string) error {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			fmt.Printf("retry download attempt %d/3...\n", attempt)
			time.Sleep(time.Duration(attempt) * 5 * time.Second)
		}
		lastErr = doDownload(ctx, url, dst)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func doDownload(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "zapret-configurator")
	if githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+githubToken)
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("download %s returned %s", url, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, dst)
}

func extractZip(zipPath, dst string, selectedPrefixes []string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	cleanDst, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cleanDst, 0o755); err != nil {
		return err
	}

	rootPrefix := commonArchiveRoot(reader.File)
	for _, file := range reader.File {
		rel := strings.ReplaceAll(file.Name, "\\", "/")
		rel = strings.TrimPrefix(rel, rootPrefix)
		rel = strings.TrimLeft(rel, "/")
		if rel == "" {
			continue
		}
		if len(selectedPrefixes) > 0 && !matchesPrefix(rel, selectedPrefixes) {
			continue
		}
		target := filepath.Join(cleanDst, filepath.FromSlash(rel))
		if !strings.HasPrefix(target, cleanDst+string(os.PathSeparator)) && target != cleanDst {
			return fmt.Errorf("unsafe archive path %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			_ = rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		closeRCErr := rc.Close()
		closeOutErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeRCErr != nil {
			return closeRCErr
		}
		if closeOutErr != nil {
			return closeOutErr
		}
	}
	return nil
}

func commonArchiveRoot(files []*zip.File) string {
	var root string
	for _, f := range files {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		name = strings.TrimLeft(name, "/")
		if name == "" {
			continue
		}
		first := strings.SplitN(name, "/", 2)[0]
		if root == "" {
			root = first
			continue
		}
		if root != first {
			return ""
		}
	}
	if root == "" {
		return ""
	}
	return root + "/"
}

func matchesPrefix(rel string, prefixes []string) bool {
	rel = path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	for _, prefix := range prefixes {
		prefix = path.Clean(strings.ReplaceAll(prefix, "\\", "/"))
		if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
			return true
		}
	}
	return false
}

func cachedDownload(ctx context.Context, url, dst string) error {
	if info, err := os.Stat(dst); err == nil && info.Size() > 0 && time.Since(info.ModTime()) < 24*time.Hour {
		fmt.Println("cache hit:", filepath.Base(dst))
		return nil
	}
	fmt.Println("download:", url)
	return downloadFile(ctx, url, dst)
}

func copySelectedYD(srcRoot, zapretDownloaded, zapret2Downloaded string) error {
	winws1Src := filepath.Join(srcRoot, "src", "presets", "builtin", "winws1")
	winws2Src := filepath.Join(srcRoot, "src", "presets", "builtin", "winws2")
	catalog1 := filepath.Join(srcRoot, "src", "profile", "strategy_catalogs", "winws1")
	catalog2 := filepath.Join(srcRoot, "src", "profile", "strategy_catalogs", "winws2")

	if err := zruntime.CopyDirIfExists(winws1Src, filepath.Join(zapretDownloaded, "youtubediscord", "src", "presets", "builtin", "winws1")); err != nil {
		return err
	}
	if err := zruntime.CopyDirIfExists(catalog1, filepath.Join(zapretDownloaded, "youtubediscord", "src", "profile", "strategy_catalogs", "winws1")); err != nil {
		return err
	}
	if err := zruntime.CopyDirIfExists(winws2Src, filepath.Join(zapret2Downloaded, "youtubediscord", "src", "presets", "builtin", "winws2")); err != nil {
		return err
	}
	if err := zruntime.CopyDirIfExists(catalog2, filepath.Join(zapret2Downloaded, "youtubediscord", "src", "profile", "strategy_catalogs", "winws2")); err != nil {
		return err
	}
	return nil
}
