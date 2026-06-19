package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCatalogFile(t *testing.T) {
	content := `[test_strategy]
name = Test Strategy
author = tester
label = recommended
description = A test strategy
--dpi-desync=fake
--dpi-desync-repeats=6

[another]
name = Another One
author = community
--dpi-desync=multisplit
--dpi-desync-split-pos=1
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tcp.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	strats, err := parseCatalogFile(path, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if len(strats) != 2 {
		t.Fatalf("expected 2 strategies, got %d", len(strats))
	}
	s := strats[0]
	if s.ID != "test_strategy" {
		t.Errorf("ID = %q, want test_strategy", s.ID)
	}
	if s.Name != "Test Strategy" {
		t.Errorf("Name = %q, want Test Strategy", s.Name)
	}
	if s.Author != "tester" {
		t.Errorf("Author = %q, want tester", s.Author)
	}
	if s.Label != "recommended" {
		t.Errorf("Label = %q, want recommended", s.Label)
	}
	if s.Category != "tcp" {
		t.Errorf("Category = %q, want tcp", s.Category)
	}
	if len(s.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(s.Args))
	}
	if s.Args[0] != "--dpi-desync=fake" {
		t.Errorf("Args[0] = %q, want --dpi-desync=fake", s.Args[0])
	}
}

func TestParseCatalogFileCommentsAndBlanks(t *testing.T) {
	content := `[s1]
name = Strategy 1
# this is a comment
--dpi-desync=fake
# another comment

--dpi-desync-repeats=6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tcp.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	strats, err := parseCatalogFile(path, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if len(strats) != 1 {
		t.Fatalf("expected 1 strategy, got %d", len(strats))
	}
	if len(strats[0].Args) != 2 {
		t.Errorf("expected 2 args, got %d: %v", len(strats[0].Args), strats[0].Args)
	}
}

func TestParseCatalogFileNoArgs(t *testing.T) {
	content := `[s1]
name = Strategy 1
author = tester
label = stable
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tcp.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	strats, err := parseCatalogFile(path, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if len(strats) != 1 {
		t.Fatalf("expected 1 strategy, got %d", len(strats))
	}
	if len(strats[0].Args) != 0 {
		t.Errorf("expected 0 args, got %d", len(strats[0].Args))
	}
}

func TestParseCatalogFileNoSections(t *testing.T) {
	content := `--some-arg=value
another-line
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tcp.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	strats, err := parseCatalogFile(path, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if len(strats) != 0 {
		t.Errorf("expected 0 strategies without sections, got %d", len(strats))
	}
}

func TestFilterByLabel(t *testing.T) {
	strategies := []Strategy{
		{ID: "a", Label: "recommended"},
		{ID: "b", Label: "stable"},
		{ID: "c", Label: "recommended"},
		{ID: "d", Label: ""},
	}
	got := FilterByLabel(strategies, "recommended")
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	got = FilterByLabel(strategies, "stable")
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	got = FilterByLabel(strategies, "")
	if len(got) != 4 {
		t.Fatalf("expected 4 (all), got %d", len(got))
	}
	got = FilterByLabel(strategies, "nonexistent")
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

func TestFilterByCategory(t *testing.T) {
	strategies := []Strategy{
		{ID: "a", Category: "tcp"},
		{ID: "b", Category: "udp"},
		{ID: "c", Category: "tcp"},
		{ID: "d", Category: "voice"},
	}
	got := FilterByCategory(strategies, "tcp")
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	got = FilterByCategory(strategies, "tcp", "udp")
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	got = FilterByCategory(strategies)
	if len(got) != 4 {
		t.Fatalf("expected 4 (all), got %d", len(got))
	}
}

func TestScanCatalogs(t *testing.T) {
	dir := t.TempDir()
	catDir := filepath.Join(dir, "winws2")
	if err := os.MkdirAll(catDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `[s1]
name = Strategy 1
label = recommended
--dpi-desync=fake
`
	if err := os.WriteFile(filepath.Join(catDir, "tcp.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	strats, err := ScanCatalogs(dir, "winws2")
	if err != nil {
		t.Fatal(err)
	}
	if len(strats) != 1 {
		t.Fatalf("expected 1 strategy, got %d", len(strats))
	}
	if strats[0].Category != "tcp" {
		t.Errorf("Category = %q, want tcp", strats[0].Category)
	}
}

func TestScanCatalogsMissing(t *testing.T) {
	dir := t.TempDir()
	strats, err := ScanCatalogs(dir, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(strats) != 0 {
		t.Fatalf("expected 0, got %d", len(strats))
	}
}

func TestScanCatalogsMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	catDir := filepath.Join(dir, "winws1")
	if err := os.MkdirAll(catDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tcp := `[s1]
name = TCP Strategy
--dpi-desync=fake
`
	udp := `[s2]
name = UDP Strategy
--dpi-desync=fake
--dpi-desync-any-protocol
`
	voice := `[s3]
name = Voice Strategy
--dpi-desync=fake
`
	os.WriteFile(filepath.Join(catDir, "tcp.txt"), []byte(tcp), 0o644)
	os.WriteFile(filepath.Join(catDir, "udp.txt"), []byte(udp), 0o644)
	os.WriteFile(filepath.Join(catDir, "voice.txt"), []byte(voice), 0o644)

	strats, err := ScanCatalogs(dir, "winws1")
	if err != nil {
		t.Fatal(err)
	}
	if len(strats) != 3 {
		t.Fatalf("expected 3 strategies, got %d", len(strats))
	}
	cats := make(map[string]bool)
	for _, s := range strats {
		cats[s.Category] = true
	}
	if !cats["tcp"] || !cats["udp"] || !cats["voice"] {
		t.Errorf("expected all categories present: %v", cats)
	}
}

func TestScanCatalogsSortOrder(t *testing.T) {
	dir := t.TempDir()
	catDir := filepath.Join(dir, "winws1")
	os.MkdirAll(catDir, 0o755)
	content := `[z_strat]
name = Z Strategy
label = stable

[a_strat]
name = A Strategy
label = recommended

[b_strat]
name = B Strategy
label = stable
`
	os.WriteFile(filepath.Join(catDir, "tcp.txt"), []byte(content), 0o644)
	strats, err := ScanCatalogs(dir, "winws1")
	if err != nil {
		t.Fatal(err)
	}
	if len(strats) != 3 {
		t.Fatalf("expected 3, got %d", len(strats))
	}
	if strats[0].ID != "a_strat" {
		t.Errorf("first should be recommended, got %s", strats[0].ID)
	}
}
