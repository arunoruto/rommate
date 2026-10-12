// Command rommate syncs games from a RomM server to a handheld.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/dustin/go-humanize"

	"github.com/arunoruto/rommate/internal/config"
	"github.com/arunoruto/rommate/internal/core"
	"github.com/arunoruto/rommate/internal/folders"
	"github.com/arunoruto/rommate/internal/gamepad"
	"github.com/arunoruto/rommate/internal/romm"
	"github.com/arunoruto/rommate/internal/state"
	"github.com/arunoruto/rommate/internal/tui"
)

// Set at build time via -ldflags (see the justfile).
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

const usage = `rommate - sync games from RomM to your handheld

Usage:
  rommate                         open the TUI
  rommate init --server URL --token rmm_... [--local-server URL] [--profile es-de|emudeck|retrodeck] [--rom-dir DIR]
  rommate platforms               list platforms and their local folders
  rommate roms <platform>         list a platform's games (slug or id)
  rommate pull <rom-id>...        download games
  rommate pull --platform <p>     download every missing game of a platform
  rommate remove <rom-id>...      delete downloaded games
  rommate status                  show downloaded games
  rommate mappings                show RomM platforms whose ES-DE folder differs
  rommate config                  show the current configuration (never the token)
  rommate gamepad                 show detected controllers and test their buttons (Linux)
  rommate version                 print version information

Every command accepts --json to print machine-readable output (JSON lines for
pull progress), e.g. for scripts or a Decky plugin.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "rommate:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return withEngine(func(e *core.Engine, cfg *config.Config) error {
			return tui.Run(ctx, e, tui.Options{Hints: cfg.Hints, Gamepad: cfg.Gamepad})
		})
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "tui":
		return withEngine(func(e *core.Engine, cfg *config.Config) error {
			return tui.Run(ctx, e, tui.Options{Hints: cfg.Hints, Gamepad: cfg.Gamepad})
		})
	case "init":
		return cmdInit(rest)
	case "platforms":
		return cmdPlatforms(ctx, rest)
	case "roms":
		return cmdRoms(ctx, rest)
	case "pull":
		return cmdPull(ctx, rest)
	case "remove", "rm":
		return cmdRemove(ctx, rest)
	case "status":
		return cmdStatus(rest)
	case "mappings":
		return cmdMappings(rest)
	case "config":
		return cmdConfig(rest)
	case "gamepad":
		return cmdGamepad(ctx)
	case "version", "--version", "-v":
		fmt.Printf("rommate %s (commit %s, built %s)\n", version, commit, date)
		return nil
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, usage)
	}
}

// withEngine loads config, opens state and builds the engine.
func withEngine(fn func(*core.Engine, *config.Config) error) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("%w\nrun `rommate init --server URL --token rmm_...` first", err)
	}
	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}
	store, err := state.Open(dataDir)
	if err != nil {
		return err
	}
	defer store.Close()
	return fn(&core.Engine{
		Client: romm.NewWithFallback(context.Background(), cfg.Server, cfg.LocalServer, cfg.Token),
		Mapper: cfg.Mapper(),
		Store:  store,
	}, cfg)
}

func newFlags(name string) (*flag.FlagSet, *bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print JSON output")
	return fs, asJSON
}

// parse parses flags anywhere in args (the flag package stops at the first
// positional argument, which would break `rommate roms nes --json`).
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		if args[0] == "--" {
			return append(pos, args[1:]...), nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	server := fs.String("server", "", "RomM server URL")
	localServer := fs.String("local-server", "", "RomM address on the local network, used when reachable (e.g. http://sado.local:8080)")
	token := fs.String("token", "", "RomM Client API Token (rmm_...)")
	profile := fs.String("profile", "", "es-de, emudeck or retrodeck")
	romDir := fs.String("rom-dir", "", "ROM root directory (default: from profile)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("%s takes no arguments, got %q", "init", pos)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if *server != "" {
		cfg.Server = strings.TrimRight(*server, "/")
	}
	if *token != "" {
		cfg.Token = *token
	}
	if *localServer != "" {
		cfg.LocalServer = strings.TrimRight(*localServer, "/")
	}
	if *profile != "" {
		cfg.Profile = folders.Profile(*profile)
	}
	if *romDir != "" {
		cfg.ROMDir = *romDir
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	client := romm.NewWithFallback(context.Background(), cfg.Server, cfg.LocalServer, cfg.Token)
	if _, err := client.Platforms(context.Background()); err != nil {
		return fmt.Errorf("could not reach RomM at %s: %w", client.BaseURL, err)
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	path, _ := config.Path()
	fmt.Printf("Saved %s\nConnected to %s\nROMs go to %s\n", path, client.BaseURL, cfg.Mapper().Root)
	if cfg.LocalServer != "" && !client.Local {
		fmt.Printf("Note: the local server %s did not answer; using %s for now.\n", cfg.LocalServer, client.BaseURL)
	}
	return nil
}

func cmdPlatforms(ctx context.Context, args []string) error {
	fs, asJSON := newFlags("platforms")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("%s takes no arguments, got %q", "platforms", pos)
	}
	return withEngine(func(e *core.Engine, _ *config.Config) error {
		ps, err := e.Platforms(ctx)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(ps)
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tSLUG\tNAME\tGAMES\tFOLDER")
		for _, p := range ps {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%s\n", p.ID, p.Slug, p.DisplayName(), p.RomCount, p.Folder)
		}
		return tw.Flush()
	})
}

// findPlatform resolves a platform by id or slug.
func findPlatform(ctx context.Context, e *core.Engine, ref string) (core.PlatformInfo, error) {
	ps, err := e.Platforms(ctx)
	if err != nil {
		return core.PlatformInfo{}, err
	}
	id, _ := strconv.Atoi(ref)
	for _, p := range ps {
		if p.ID == id || p.Slug == ref || p.FSSlug == ref || p.Folder == ref {
			return p, nil
		}
	}
	return core.PlatformInfo{}, fmt.Errorf("no platform %q with games on the server (see `rommate platforms`)", ref)
}

func cmdRoms(ctx context.Context, args []string) error {
	fs, asJSON := newFlags("roms")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("usage: rommate roms <platform>")
	}
	return withEngine(func(e *core.Engine, _ *config.Config) error {
		p, err := findPlatform(ctx, e, pos[0])
		if err != nil {
			return err
		}
		roms, err := e.Roms(ctx, p.ID)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(roms)
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\t\tNAME\tSIZE")
		for _, r := range roms {
			mark := " "
			if r.Installed {
				mark = "✓"
			}
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", r.ID, mark, r.Title(), humanize.IBytes(uint64(r.FSSizeBytes)))
		}
		return tw.Flush()
	})
}

func cmdPull(ctx context.Context, args []string) error {
	fs, asJSON := newFlags("pull")
	platform := fs.String("platform", "", "download every missing game of this platform")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if *platform == "" && len(pos) == 0 {
		return errors.New("usage: rommate pull <rom-id> [rom-id ...] | --platform <platform>")
	}
	return withEngine(func(e *core.Engine, _ *config.Config) error {
		var roms []romm.Rom
		if *platform != "" {
			p, err := findPlatform(ctx, e, *platform)
			if err != nil {
				return err
			}
			infos, err := e.Roms(ctx, p.ID)
			if err != nil {
				return err
			}
			for _, r := range infos {
				if !r.Installed {
					roms = append(roms, r.Rom)
				}
			}
		}
		for _, a := range pos {
			id, err := strconv.Atoi(a)
			if err != nil {
				return fmt.Errorf("not a rom id: %q", a)
			}
			r, err := e.Client.Rom(ctx, id)
			if err != nil {
				return err
			}
			roms = append(roms, r)
		}

		emit := textEmitter()
		if *asJSON {
			enc := json.NewEncoder(os.Stdout)
			emit = func(ev core.Event) { _ = enc.Encode(ev) }
		}
		var failed int
		for _, r := range roms {
			if err := e.Pull(ctx, r, emit); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				failed++
			}
		}
		if failed > 0 {
			return fmt.Errorf("%d of %d downloads failed", failed, len(roms))
		}
		return nil
	})
}

// textEmitter prints human-readable progress, redrawing one line per game.
func textEmitter() core.Emit {
	tty := isTerminal(os.Stdout)
	return func(ev core.Event) {
		switch ev.Type {
		case core.EventStart:
			fmt.Printf("↓ %s (%s)\n", ev.Name, humanize.IBytes(uint64(ev.Total)))
		case core.EventProgress:
			if tty && ev.Total > 0 {
				fmt.Printf("\r  %5.1f%%  %s / %s", float64(ev.Done)*100/float64(ev.Total),
					humanize.IBytes(uint64(ev.Done)), humanize.IBytes(uint64(ev.Total)))
			}
		case core.EventDone:
			if tty {
				fmt.Print("\r\033[K")
			}
			fmt.Printf("✓ %s → %s\n", ev.Name, ev.Path)
		case core.EventSkipped:
			fmt.Printf("· %s already downloaded\n", ev.Name)
		case core.EventError:
			if tty {
				fmt.Print("\r\033[K")
			}
			fmt.Fprintf(os.Stderr, "✗ %s: %s\n", ev.Name, ev.Error)
		}
	}
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func cmdRemove(ctx context.Context, args []string) error {
	fs, asJSON := newFlags("remove")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return errors.New("usage: rommate remove <rom-id> [rom-id ...]")
	}
	return withEngine(func(e *core.Engine, _ *config.Config) error {
		var removed []int
		for _, a := range pos {
			id, err := strconv.Atoi(a)
			if err != nil {
				return fmt.Errorf("not a rom id: %q", a)
			}
			r, err := e.Client.Rom(ctx, id)
			if err != nil {
				return err
			}
			if err := e.Remove(r); err != nil {
				return err
			}
			removed = append(removed, id)
			if !*asJSON {
				fmt.Printf("removed %s\n", r.Title())
			}
		}
		if *asJSON {
			return printJSON(map[string]any{"removed": removed})
		}
		return nil
	})
}

func cmdStatus(args []string) error {
	fs, asJSON := newFlags("status")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("%s takes no arguments, got %q", "status", pos)
	}
	return withEngine(func(e *core.Engine, _ *config.Config) error {
		entries, err := e.Status()
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(entries)
		}
		if len(entries) == 0 {
			fmt.Println("Nothing downloaded yet.")
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tPLATFORM\tNAME\tSIZE\tPATH")
		for _, en := range entries {
			path := en.Path
			if !en.Present {
				path += "  (missing!)"
			}
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n", en.RomID, en.PlatformSlug, en.Name,
				humanize.IBytes(uint64(en.SizeBytes)), path)
		}
		return tw.Flush()
	})
}

// cmdMappings prints the built-in RomM slug → ES-DE folder aliases. It needs
// no config, so it also feeds the documentation site (just site-data).
func cmdMappings(args []string) error {
	fs, asJSON := newFlags("mappings")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("mappings takes no arguments, got %q", pos)
	}
	aliases := folders.Aliases()
	if *asJSON {
		return printJSON(aliases)
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ROMM SLUG\tES-DE FOLDER")
	for _, a := range aliases {
		fmt.Fprintf(tw, "%s\t%s\n", a.Slug, a.Folder)
	}
	fmt.Fprintln(tw, "\nAll other platforms use the RomM slug as folder name. Override any of them under [folders] in config.toml.")
	return tw.Flush()
}

// configInfo is the output of `rommate config`. It never contains the token.
type configInfo struct {
	Configured  bool              `json:"configured"`
	Problem     string            `json:"problem,omitempty"`
	ConfigPath  string            `json:"config_path"`
	Server      string            `json:"server"`
	LocalServer string            `json:"local_server,omitempty"`
	TokenSet    bool              `json:"token_set"`
	Profile     string            `json:"profile"`
	ROMDir      string            `json:"rom_dir"`
	Hints       string            `json:"hints"`
	Folders     map[string]string `json:"folders,omitempty"`
}

// cmdConfig shows the effective configuration, including environment
// overrides. It works without a valid config so frontends such as the Decky
// plugin can tell whether setup is still needed.
func cmdConfig(args []string) error {
	fs, asJSON := newFlags("config")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("config takes no arguments, got %q", pos)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	path, _ := config.Path()
	hints := cfg.Hints
	if hints == "" {
		hints = "auto"
	}
	info := configInfo{
		Configured:  true,
		ConfigPath:  path,
		Server:      cfg.Server,
		LocalServer: cfg.LocalServer,
		TokenSet:    cfg.Token != "",
		Profile:     string(cfg.Profile),
		ROMDir:      cfg.Mapper().Root,
		Hints:       hints,
		Folders:     cfg.Folders,
	}
	if err := cfg.Validate(); err != nil {
		info.Configured, info.Problem = false, err.Error()
	}
	if *asJSON {
		return printJSON(info)
	}
	token := "not set"
	if info.TokenSet {
		token = "set"
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "config file\t%s\n", info.ConfigPath)
	fmt.Fprintf(tw, "server\t%s\n", info.Server)
	if info.LocalServer != "" {
		fmt.Fprintf(tw, "local server\t%s (used when reachable)\n", info.LocalServer)
	}
	fmt.Fprintf(tw, "token\t%s\n", token)
	fmt.Fprintf(tw, "profile\t%s\n", info.Profile)
	fmt.Fprintf(tw, "rom dir\t%s\n", info.ROMDir)
	fmt.Fprintf(tw, "hints\t%s\n", info.Hints)
	for slug, folder := range info.Folders {
		fmt.Fprintf(tw, "folder override\t%s → %s\n", slug, folder)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if !info.Configured {
		fmt.Printf("\nNot ready: %s\n", info.Problem)
	}
	return nil
}

// cmdGamepad lists controllers rommate can read and prints every recognised
// button press, to check the mapping on a device. Press Start to quit.
func cmdGamepad(ctx context.Context) error {
	devices := gamepad.Find()
	if len(devices) == 0 {
		return errors.New("no readable game controller found (Linux only; the user needs read access to /dev/input/event*, usually via the input group)")
	}
	for _, d := range devices {
		fmt.Printf("found %s (%s)\n", d.Name, d.Path)
	}
	fmt.Println("Press buttons to see how rommate reads them. Start quits.")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	gamepad.Listen(ctx, devices, func(b gamepad.Button) {
		fmt.Printf("  %s\n", b)
		if b == gamepad.Start {
			cancel()
		}
	})
	<-ctx.Done()
	return nil
}
