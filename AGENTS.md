# AGENTS.md

Guidance for AI coding agents (Claude Code, Codex, OpenCode, …) working in this repository. `CLAUDE.md` is a symlink
to this file.

## What this is

rommate syncs games from a [RomM](https://romm.app) server to Linux handhelds (Armada OS, Steam Deck, Steam Machines)
and stores them in the folders ES-DE, EmuDeck or RetroDECK expect. It is a single Go binary with a controller-friendly
Bubble Tea TUI, a CLI, and a `--json` mode for scripts and a future Decky plugin. There is deliberately **no daemon**.

## Development environment

**If `nix` is available, always work inside the project's dev shell.** It pins Go, gopls, golangci-lint, delve and
just, so builds, lints and language-server results match what the maintainer uses. Do not install tools globally or
with `go install` instead.

- If direnv is active (`.envrc` is loaded), you are already in the shell.
- Otherwise run commands through the shell:

  ```sh
  nix develop github:arunoruto/flake#go-lsp -c just check
  nix develop github:arunoruto/flake#go-lsp -c just test ./internal/tui/...
  ```

  For a long session, enter it once with `nix develop github:arunoruto/flake#go-lsp`.
- `#go-lsp` includes the language servers (`gopls`, `golangci-lint-langserver`); `#go` is the same without them.

Without nix, you need Go (version in `go.mod`), `just` and `golangci-lint` on the PATH.

## Commands

All tasks are [just](https://just.systems) recipes; the `justfile` is the single source of truth for local work and
CI. Prefer a recipe over retyping its command.

```sh
just                          # list recipes
just check                    # fmt-check, tidy-check, vet, lint, tests: run before every commit
just test ./internal/core/... # tests, optionally limited to packages
just build                    # → dist/rommate
just build-target linux arm64 # cross-compile (CGO is off, builds are static)
just package                  # release binaries + checksums.txt → dist/release
just run platforms            # go run with arguments
```

## Layout

```text
cmd/rommate/        CLI entry point, subcommands, --json output, version info (set via -ldflags)
internal/core/      sync engine: downloads, resume, file locking (lock_unix.go / lock_windows.go), events
internal/romm/      RomM REST API client
internal/folders/   RomM slug → ES-DE folder mapping, frontend profiles (es-de, emudeck, retrodeck)
internal/state/     SQLite record of downloaded games (pure Go driver, no CGO)
internal/config/    ~/.config/rommate/config.toml (on every OS, including macOS)
internal/tui/       Bubble Tea TUI: tabs, download queue, views, help, input-mode detection
```

## Conventions

- **No CGO.** Everything must cross-compile for linux/amd64, linux/arm64, darwin/arm64 and windows/amd64. Put
  OS-specific code behind build tags.
- **Controller first.** Every TUI action must be reachable with the keys Steam Input maps to a gamepad: arrows,
  `enter` (A), `esc` (B), `y`, `x`, `[` `]` (L1/R1), `f1` (Select) and `q` (Start). Free-text input such as search is
  keyboard-only. When you change a binding, update `helpSections` in `internal/tui/help.go`, the footer hints and the
  README table.
- **The TUI must fit any terminal size.** `TestViewFits` renders every screen at several sizes; add new screens to it.
- **The download queue lives in the model** and is only mutated from `Update`. Downloads run in tea.Cmds and report
  back through messages; do not add goroutines that touch model state.
- **Core logic stays frontend-independent.** The CLI, TUI and JSON mode are thin layers over `internal/core`.
- **Never commit secrets.** RomM tokens (`rmm_…`) belong in `~/.config/rommate/config.toml` or `ROMMATE_TOKEN`.
- Commits follow [Conventional Commits](https://www.conventionalcommits.org) (`feat:`, `fix:`, `build:`, `docs:`, …).
- Keep the README in sync with user-facing changes (flags, config keys, keybindings).

## Testing against a real server

Use environment variables instead of a config file, and a throwaway ROM folder:

```sh
ROMMATE_SERVER=https://… ROMMATE_TOKEN=rmm_… ROMMATE_ROM_DIR=$(mktemp -d) XDG_STATE_HOME=$(mktemp -d) just run platforms
```
