package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Options struct {
	OutputDir           string
	Engine              string
	Mode                string
	Target              string
	Zapret2InstallerURL string
	Top                 int
	GitHubToken         string
}

func (o Options) Validate() error {
	switch strings.ToLower(o.Engine) {
	case "zapret", "zapret2", "both":
	default:
		return fmt.Errorf("unsupported --engine %q", o.Engine)
	}
	switch strings.ToLower(o.Mode) {
	case "quick", "standard", "full":
	default:
		return fmt.Errorf("unsupported --mode %q", o.Mode)
	}
	if strings.TrimSpace(o.OutputDir) == "" {
		return fmt.Errorf("--output must not be empty")
	}
	return nil
}

func (o Options) WantsZapret() bool {
	engine := strings.ToLower(o.Engine)
	return engine == "zapret" || engine == "both"
}

func (o Options) WantsZapret2() bool {
	engine := strings.ToLower(o.Engine)
	return engine == "zapret2" || engine == "both"
}

func (o Options) DownloadedDir(engine string) string {
	return filepath.Join(o.OutputDir, "_downloaded", engine)
}

func (o Options) ConvertedDir(engine string) string {
	return filepath.Join(o.OutputDir, "_converted", engine)
}

func (o Options) FinalDir(engine string) string {
	return filepath.Join(o.OutputDir, "final", engine)
}

func (o Options) AutopickDir(engine string) string {
	return filepath.Join(o.OutputDir, "final", "autopick", engine)
}
