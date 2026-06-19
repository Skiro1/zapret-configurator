package runtime

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func EnsureCleanDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("refusing to clean empty path")
	}
	// Try RemoveAll first; if it fails (e.g., locked .sys files), do best-effort cleanup
	if err := os.RemoveAll(path); err != nil {
		// RemoveAll failed - try to remove directory contents individually, skipping locked files
		_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if remErr := os.Remove(p); remErr != nil {
				fmt.Printf("warning: cannot remove %s (may be locked): %v\n", p, remErr)
			}
			return nil
		})
		// Try removing the root dir itself (may still fail if locked files inside)
		_ = os.Remove(path)
	}
	return os.MkdirAll(path, 0o755)
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func CopyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dstPath := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}
		if copyErr := CopyFile(path, dstPath); copyErr != nil {
			fmt.Printf("warning: cannot copy %s (may be locked): %v\n", rel, copyErr)
		}
		return nil
	})
}

func CopyDirIfExists(src, dst string) error {
	info, err := os.Stat(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return CopyDir(src, dst)
}

func CopyFileIfExists(src, dst string) error {
	info, err := os.Stat(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	if err := CopyFile(src, dst); err != nil {
		fmt.Printf("warning: cannot copy %s (may be locked): %v\n", filepath.Base(src), err)
		return nil
	}
	return nil
}

func HasFile(root, name string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || found || d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), name) {
			found = true
		}
		return nil
	})
	return found
}

func FindFile(root, name string) (string, bool) {
	var found string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" || d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), name) {
			found = path
		}
		return nil
	})
	return found, found != ""
}

func FindDirs(root string, names ...string) map[string][]string {
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[strings.ToLower(name)] = struct{}{}
	}
	result := make(map[string][]string)
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		key := strings.ToLower(d.Name())
		if _, ok := wanted[key]; ok {
			result[key] = append(result[key], path)
		}
		return nil
	})
	return result
}

var unsafeFilenameRE = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]+`)

func SafeFilename(name string) string {
	base := strings.TrimSpace(name)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = unsafeFilenameRE.ReplaceAllString(base, "_")
	base = strings.Join(strings.Fields(base), " ")
	base = strings.Trim(base, ". ")
	if base == "" {
		base = "config"
	}
	if len(base) > 120 {
		base = base[:120]
	}
	return base
}
