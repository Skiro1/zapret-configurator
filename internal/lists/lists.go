package lists

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureLists first overwrites all files from sourceDir into listsDir,
// then generates any additional missing list files referenced by bat scripts.
func EnsureLists(finalDir, sourceDir string) error {
	listsDir := filepath.Join(finalDir, "lists")
	if err := os.MkdirAll(listsDir, 0o755); err != nil {
		return err
	}

	// Step 1: copy all files from sourceDir (overwrite existing)
	if sourceDir != "" {
		entries, err := os.ReadDir(sourceDir)
		if err == nil {
			copied := 0
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				src := filepath.Join(sourceDir, e.Name())
				dst := filepath.Join(listsDir, e.Name())
				data, err := os.ReadFile(src)
				if err != nil {
					continue
				}
				if err := os.WriteFile(dst, data, 0o644); err != nil {
					continue
				}
				copied++
			}
			if copied > 0 {
				fmt.Printf("copied %d list files from source\n", copied)
			}
		}
	}

	// Step 2: scan bat files for referenced lists and generate any still missing
	needed := scanNeeded(finalDir)
	existing := scanExisting(listsDir)
	missing := diff(needed, existing)
	if len(missing) == 0 {
		return nil
	}

	fmt.Printf("generating %d missing list files...\n", len(missing))
	for _, name := range missing {
		path := filepath.Join(listsDir, name)
		content := resolveEmbedded(name)
		if content == "" {
			content = defaultPlaceholder(name)
		}
		_ = os.WriteFile(path, []byte(content), 0o644)
	}
	return nil
}

func readFileStr(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func scanNeeded(finalDir string) []string {
	needed := make(map[string]bool)
	_ = filepath.WalkDir(finalDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".bat") {
			return nil
		}
		data, _ := os.ReadFile(path)
		scanBatRefs(string(data), needed)
		return nil
	})
	out := make([]string, 0, len(needed))
	for n := range needed {
		out = append(out, n)
	}
	return out
}

func scanBatRefs(content string, out map[string]bool) {
	const prefix = "%LISTS%"
	for {
		idx := strings.Index(content, prefix)
		if idx == -1 {
			return
		}
		start := idx + len(prefix)
		end := start
		for end < len(content) {
			c := content[end]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' || c == '^' || c == ';' || c == '"' {
				break
			}
			end++
		}
		if end > start {
			name := content[start:end]
			if strings.HasSuffix(name, ".txt") {
				out[name] = true
			}
		}
		content = content[end:]
	}
}

func scanExisting(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

func diff(needed, existing []string) []string {
	m := make(map[string]bool, len(existing))
	for _, n := range existing {
		m[n] = true
	}
	var out []string
	for _, n := range needed {
		if !m[n] {
			out = append(out, n)
		}
	}
	return out
}

func defaultPlaceholder(name string) string {
	return fmt.Sprintf("# %s - placeholder\n# Add entries manually\n", name)
}

func resolveEmbedded(name string) string {
	base := strings.TrimSuffix(name, ".txt")
	if c, ok := embeddedLists[base]; ok {
		return c
	}
	return ""
}
