package catalog

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Strategy struct {
	ID          string
	Name        string
	Author      string
	Label       string
	Description string
	Args        []string
	Category    string
}

func ScanCatalogs(catalogsDir string, profile string) ([]Strategy, error) {
	catDir := filepath.Join(catalogsDir, profile)
	if _, err := os.Stat(catDir); os.IsNotExist(err) {
		return nil, nil
	}
	var all []Strategy
	err := filepath.WalkDir(catDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(d.Name()), ".txt") {
			return nil
		}
		category := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		strats, parseErr := parseCatalogFile(path, category)
		if parseErr != nil {
			return nil
		}
		all = append(all, strats...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Label == "recommended" && all[j].Label != "recommended" {
			return true
		}
		if all[i].Label != "recommended" && all[j].Label == "recommended" {
			return false
		}
		if all[i].Label == "stable" && all[j].Label != "stable" {
			return true
		}
		if all[i].Label != "stable" && all[j].Label == "stable" {
			return false
		}
		return all[i].Name < all[j].Name
	})
	return all, nil
}

func parseCatalogFile(path string, category string) ([]Strategy, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var strategies []Strategy
	var current *Strategy
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if current != nil {
				strategies = append(strategies, *current)
			}
			id := strings.Trim(line, "[]")
			current = &Strategy{
				ID:       id,
				Category: category,
			}
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "--") {
			current.Args = append(current.Args, line)
		} else if idx := strings.IndexByte(line, '='); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			switch strings.ToLower(key) {
			case "name":
				current.Name = val
			case "author":
				current.Author = val
			case "label":
				current.Label = val
			case "description":
				current.Description = val
			}
		}
	}
	if current != nil {
		strategies = append(strategies, *current)
	}
	return strategies, scanner.Err()
}

func FilterByLabel(strategies []Strategy, label string) []Strategy {
	if label == "" {
		return strategies
	}
	label = strings.ToLower(label)
	var out []Strategy
	for _, s := range strategies {
		if strings.ToLower(s.Label) == label {
			out = append(out, s)
		}
	}
	return out
}

func FilterByCategory(strategies []Strategy, categories ...string) []Strategy {
	if len(categories) == 0 {
		return strategies
	}
	wanted := make(map[string]bool, len(categories))
	for _, c := range categories {
		wanted[strings.ToLower(c)] = true
	}
	var out []Strategy
	for _, s := range strategies {
		if wanted[strings.ToLower(s.Category)] {
			out = append(out, s)
		}
	}
	return out
}
