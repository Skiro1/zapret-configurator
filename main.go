package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zapret-configurator/internal/autopick"
	"zapret-configurator/internal/config"
	"zapret-configurator/internal/convert"
	zruntime "zapret-configurator/internal/runtime"
	"zapret-configurator/internal/source"
)

const usageText = `zapret-configurator

Commands:
  sync          download upstream configs and runtime artifacts
  convert       convert downloaded configs/presets to flowseal-like .bat files
  build-final   assemble final zapret and zapret2 folders
  autopick      test and rank configs, then copy the best to final/autopick
  all           run sync -> convert -> build-final

Common flags:
  --output PATH
  --engine zapret|zapret2|both
  --mode quick|standard|full
  --target DOMAIN
  --zapret2-installer-url URL
  --github-token TOKEN    (or set GITHUB_TOKEN env)
  --top N

Examples:
  zapret-configurator all --engine zapret
  zapret-configurator all --engine zapret2 --github-token ghp_xxxx
  set GITHUB_TOKEN=ghp_xxxx && zapret-configurator all
  zapret-configurator autopick --engine zapret --mode full --target rutracker.org
  zapret-configurator sync --engine zapret2
  zapret-configurator convert --engine both
  zapret-configurator build-final --engine zapret
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Print(usageText)
		return nil
	}

	command := strings.ToLower(strings.TrimSpace(args[0]))
	opts, err := parseOptions(command, args[1:])
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	defer cancel()

	switch command {
	case "sync":
		return source.Sync(ctx, opts)
	case "convert":
		return convert.ConvertAll(opts)
	case "build-final":
		return zruntime.BuildFinal(opts)
	case "autopick":
		return autopick.Run(ctx, opts)
	case "all":
		if err := source.Sync(ctx, opts); err != nil {
			return err
		}
		if err := convert.ConvertAll(opts); err != nil {
			return err
		}
		return zruntime.BuildFinal(opts)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", command, usageText)
	}
}

func parseOptions(command string, args []string) (config.Options, error) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	var opts config.Options
	fs.StringVar(&opts.OutputDir, "output", defaultOutputDir(), "output directory")
	fs.StringVar(&opts.Engine, "engine", "both", "zapret, zapret2, or both")
	fs.StringVar(&opts.Mode, "mode", "quick", "quick, standard, or full")
	fs.StringVar(&opts.Target, "target", "", "additional target domains (comma-separated, appended to defaults)")
	fs.StringVar(&opts.Zapret2InstallerURL, "zapret2-installer-url", "", "override Zapret2 installer URL")
	fs.IntVar(&opts.Top, "top", 5, "number of best autopick configs to copy")
	fs.StringVar(&opts.GitHubToken, "github-token", "", "GitHub API token (or set GITHUB_TOKEN env)")

	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if opts.GitHubToken == "" {
		opts.GitHubToken = os.Getenv("GITHUB_TOKEN")
	}
	opts.OutputDir = filepath.Clean(opts.OutputDir)
	opts.Target = mergeTargets(defaultTargets, opts.Target)
	if opts.Top <= 0 {
		opts.Top = 5
	}
	if err := opts.Validate(); err != nil {
		return opts, err
	}
	return opts, nil
}

var defaultTargets = []string{
	"discord.com",
	"gateway.discord.gg",
	"cdn.discordapp.com",
	"updates.discord.com",
	"youtube.com",
	"youtu.be",
	"i.ytimg.com",
	"googlevideo.com",
	"google.com",
	"gstatic.com",
	"cloudflare.com",
	"one.one.one.one",
	"dns.google",
	"dns.quad9.net",
}

func mergeTargets(defaults []string, user string) string {
	seen := make(map[string]bool)
	var result []string
	for _, d := range defaults {
		dl := strings.ToLower(d)
		if !seen[dl] {
			seen[dl] = true
			result = append(result, d)
		}
	}
	for _, u := range strings.Split(user, ",") {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		ul := strings.ToLower(u)
		if !seen[ul] {
			seen[ul] = true
			result = append(result, u)
		}
	}
	return strings.Join(result, ",")
}

func defaultOutputDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "output"
	}
	if strings.EqualFold(filepath.Base(cwd), "zapret-configurator") {
		return filepath.Join(cwd, "output")
	}
	if _, err := os.Stat(filepath.Join(cwd, "zapret-configurator")); err == nil {
		return filepath.Join(cwd, "zapret-configurator", "output")
	}
	return filepath.Join(cwd, "output")
}
