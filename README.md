<a id="readme-top"></a>

[![CI][ci-shield]][ci-url]
[![Release][release-shield]][release-url]
[![Go][go-shield]][go-url]
[![Issues][issues-shield]][issues-url]
[![Stargazers][stars-shield]][stars-url]
[![MIT License][license-shield]][license-url]

<br />
<div align="center">
<h3 align="center">rommate</h3>

  <p align="center">
    Your RomM server's roommate on the handheld: sync games straight into the right ES-DE folders,
    from a controller-friendly TUI or the command line.
    <br />
    <br />
    <a href="https://github.com/arunoruto/rommate/issues/new?labels=bug">Report Bug</a>
    &middot;
    <a href="https://github.com/arunoruto/rommate/issues/new?labels=enhancement">Request Feature</a>
  </p>
</div>

<details>
  <summary>Table of Contents</summary>
  <ol>
    <li>
      <a href="#about-the-project">About The Project</a>
      <ul>
        <li><a href="#built-with">Built With</a></li>
      </ul>
    </li>
    <li>
      <a href="#getting-started">Getting Started</a>
      <ul>
        <li><a href="#prerequisites">Prerequisites</a></li>
        <li><a href="#installation">Installation</a></li>
        <li><a href="#configuration">Configuration</a></li>
      </ul>
    </li>
    <li><a href="#usage">Usage</a></li>
    <li><a href="#development">Development</a></li>
    <li><a href="#roadmap">Roadmap</a></li>
    <li><a href="#contributing">Contributing</a></li>
    <li><a href="#license">License</a></li>
    <li><a href="#acknowledgments">Acknowledgments</a></li>
  </ol>
</details>

## About The Project

rommate downloads games from a [RomM](https://romm.app) server onto Linux handhelds such as
[Armada](https://armadaos.dev), the Steam Deck and Steam Machines, and puts them where your frontend expects them.

Why another sync tool?

* **Correct folders out of the box.** RomM and ES-DE name some platforms differently (`3ds` vs `n3ds`, `ngc` vs
  `gc`, `sms` vs `mastersystem`, …). rommate ships a mapping for every platform both projects know, and reads your
  ROM directory from ES-DE's own settings.
* **Made for a controller.** The TUI uses only D-pad, A/B/X/Y, L1/R1 and Start, so it works in Game Mode through
  Steam Input as well as in Desktop Mode.
* **Nothing running in the background.** One small static binary (~11 MB) that only runs while you use it.
* **Scriptable.** Every command has a `--json` mode (JSON lines for download progress), which makes it easy to
  build on, for example a future Decky plugin.
* **Safe and resumable.** Downloads go to `.part` files, resume after interruptions, are locked against concurrent
  writers and are checked against the expected size. Games already on disk are detected and kept.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

### Built With

* [![Go][go-shield]][go-url]
* [Bubble Tea](https://github.com/charmbracelet/bubbletea) & [Lip Gloss](https://github.com/charmbracelet/lipgloss) for the TUI
* [modernc.org/sqlite](https://modernc.org/sqlite) for local state (pure Go, no CGO)
* [Nix](https://nixos.org) and [just](https://just.systems) for the development environment

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Getting Started

### Prerequisites

* A RomM server and a **Client API Token** (in RomM: *Administration → Client API Tokens*, it starts with `rmm_`)
* A Linux device (`arm64` or `amd64`) with ES-DE, EmuDeck or RetroDECK

### Installation

Download the binary for your device from the [latest release](https://github.com/arunoruto/rommate/releases/latest):

| Platform                                  | Asset                       |
|-------------------------------------------|-----------------------------|
| Linux ARM64 (Armada and other ARM handhelds) | `rommate-linux-arm64`    |
| Linux x86-64 (Steam Deck, Steam Machine, PCs) | `rommate-linux-amd64`   |
| macOS Apple Silicon                       | `rommate-darwin-arm64`      |
| Windows x86-64 (untested)                 | `rommate-windows-amd64.exe` |

For example on a handheld:

```sh
mkdir -p ~/.local/bin
curl -fL -o ~/.local/bin/rommate \
  https://github.com/arunoruto/rommate/releases/latest/download/rommate-linux-arm64
chmod +x ~/.local/bin/rommate
rommate --version
```

Each release includes `checksums.txt` (SHA-256). On macOS the downloaded binary is unsigned; if Gatekeeper blocks it,
run `xattr -d com.apple.quarantine rommate-darwin-arm64`.

To build from source instead, see [Development](#development).

### Configuration

Run `init` once on the device. It checks the connection before saving anything:

```sh
rommate init --server https://romm.example.com --token rmm_...
```

This writes `~/.config/rommate/config.toml` (readable only by you):

```toml
server  = "https://romm.example.com"
token   = "rmm_..."
profile = "es-de"        # es-de | emudeck | retrodeck
# rom_dir = "/run/media/armada/Games/ROMs"   # default: taken from the profile
# hints = "auto"         # footer hints: auto | keyboard | controller

# Override the folder for individual RomM platform slugs
[folders]
# gba = "gba-hacks"
```

| Profile     | ROM root                                                   |
|-------------|------------------------------------------------------------|
| `es-de`     | `ROMDirectory` from `~/ES-DE/settings/es_settings.xml`, else `~/ROMs` |
| `emudeck`   | `~/Emulation/roms`                                         |
| `retrodeck` | `~/retrodeck/roms`                                         |

`ROMMATE_SERVER`, `ROMMATE_TOKEN`, `ROMMATE_ROM_DIR` and `ROMMATE_HINTS` override the config file.

With `hints = "auto"` the footer shows controller buttons when rommate was launched through Steam (Game Mode or a
non-Steam shortcut) and keyboard keys otherwise. With Steam Input a controller sends ordinary key presses, so rommate
cannot tell the two apart while running; set `keyboard` or `controller` to force one.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Usage

### TUI

Run `rommate` with no arguments. To use it in Game Mode, add it to Steam as a non-Steam game (for example through
Konsole: `konsole -e rommate`) and use a controller layout that maps buttons to these keys:

Press `?` (or `F1`, `h`, or Select on a controller) inside the TUI to see all shortcuts.

| Action                        | Keyboard                    | Controller (Steam Input → key) |
|-------------------------------|-----------------------------|--------------------------------|
| Switch tab (Library/Downloads)| `[` `]` or `Tab`            | L1 R1 → `[` `]`                |
| Move up / down                | `↑` `↓` or `k` `j`          | D-pad ↑ ↓ → `↑` `↓`            |
| Previous / next page          | `←` `→` or `PgUp` `PgDn`    | D-pad ← → → `←` `→`            |
| First / last entry            | `Home` / `End`              | –                              |
| Open platform / download game | `Enter`                     | A → `Enter`                    |
| Back                          | `Esc` or `Backspace`        | B → `Esc`                      |
| Download all missing games    | `y`                         | Y → `y`                        |
| Remove game (press twice)     | `x`                         | X → `x`                        |
| Reload from server            | `r`                         | –                              |
| Downloads: retry failed       | `Enter`                     | A                              |
| Downloads: next / retry all   | `y`                         | Y                              |
| Downloads: cancel / dismiss   | `x`                         | X                              |
| Show / hide help              | `?`, `F1` or `h`            | Select → `F1`                  |
| Quit (press twice while downloading) | `q` or `Ctrl+C`      | Start → `q`                    |

### CLI

```sh
rommate platforms                 # platforms on the server and their local folder
rommate roms 3ds                  # games of a platform (slug, folder or id); ✓ = downloaded
rommate pull 9 12                 # download games by id
rommate pull --platform n3ds      # download every missing game of a platform
rommate status                    # what's on this device
rommate remove 9                  # delete a downloaded game
```

### JSON output

Add `--json` to any command. `pull` prints one event per line:

```json
{"event":"start","rom_id":9,"name":"Pokémon Omega Ruby","path":"/…/n3ds/omega.3ds","total":2147483648}
{"event":"progress","rom_id":9,"name":"Pokémon Omega Ruby","file":"omega.3ds","done":524288000,"total":2147483648}
{"event":"done","rom_id":9,"name":"Pokémon Omega Ruby","path":"/…/n3ds/omega.3ds","done":2147483648,"total":2147483648}
```

Event types: `start`, `progress`, `done`, `skipped`, `error`.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Development

The development shell comes from [github:arunoruto/flake](https://github.com/arunoruto/flake) (`#go-lsp`): Go,
gopls, golangci-lint, delve and just. With [nix-direnv](https://github.com/nix-community/nix-direnv) it loads
automatically via `.envrc`; otherwise use `nix develop github:arunoruto/flake#go-lsp`. All common tasks are
[just](https://just.systems) recipes:

```sh
direnv allow                    # or: nix develop github:arunoruto/flake#go-lsp
just                            # list all recipes

just build                      # → dist/rommate (current machine)
just build-target linux arm64   # → dist/rommate-linux-arm64
just build-all                  # every release target
just run platforms              # go run with arguments
just deploy                     # build for arm64 + copy to armada@armada.local (or: just deploy user@host)

just test                       # unit tests
just check                      # what CI runs: fmt, tidy, vet, lint, tests
just package                    # release archives + checksums → dist/release
```

### Releasing

Releases follow [Semantic Versioning](https://semver.org) and are derived from
[Conventional Commits](https://www.conventionalcommits.org): `fix:` bumps the patch version, `feat:` the minor
version, and a breaking change (`feat!:` or a `BREAKING CHANGE:` footer) the major version (the minor version while
on 0.x). To publish, run the **Release** workflow from the Actions tab or with `gh workflow run release.yml`. It runs
CI, computes the next version with [go-semantic-release](https://github.com/go-semantic-release/semantic-release),
builds the binaries with `just package` and creates the GitHub release with changelog and checksums.

Project layout:

```text
cmd/rommate/        CLI entry point and --json output
internal/core/      sync engine: downloads, resume, locking, events
internal/romm/      RomM REST API client
internal/folders/   RomM slug → ES-DE folder mapping, frontend profiles
internal/state/     SQLite record of downloaded games
internal/config/    config.toml handling
internal/tui/       Bubble Tea TUI
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Roadmap

- [x] RomM API client with Client API Token auth
- [x] RomM → ES-DE folder mapping with EmuDeck/RetroDECK profiles
- [x] Resumable, locked downloads with JSON progress events
- [x] Controller-friendly TUI
- [x] Tabs, Downloads page and Steam-style download bar
- [ ] Search/filter in the TUI
- [ ] Save sync (RetroArch first, then standalone emulators)
- [ ] Native gamepad input via evdev when launched outside Steam
- [ ] Optional cover art and `gamelist.xml` for ES-DE
- [ ] Decky plugin on top of `--json`

See the [open issues](https://github.com/arunoruto/rommate/issues) for a full list of proposed features and known issues.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Contributing

Contributions are welcome. If you have a suggestion, open an issue with the tag "enhancement", or fork the repo and
create a pull request:

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'feat: add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## License

Distributed under the MIT License. See [`LICENSE`](LICENSE) for more information.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Acknowledgments

* [RomM](https://github.com/rommapp/romm), the self-hosted ROM manager this talks to
* [ES-DE](https://es-de.org), whose system names rommate follows
* [Armada](https://armadaos.dev)
* [Best-README-Template](https://github.com/othneildrew/Best-README-Template)

<p align="right">(<a href="#readme-top">back to top</a>)</p>

[ci-shield]: https://img.shields.io/github/actions/workflow/status/arunoruto/rommate/ci.yml?branch=main&style=for-the-badge&label=CI
[ci-url]: https://github.com/arunoruto/rommate/actions/workflows/ci.yml
[release-shield]: https://img.shields.io/github/v/release/arunoruto/rommate?style=for-the-badge&include_prereleases
[release-url]: https://github.com/arunoruto/rommate/releases/latest
[go-shield]: https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white
[go-url]: https://go.dev
[issues-shield]: https://img.shields.io/github/issues/arunoruto/rommate.svg?style=for-the-badge
[issues-url]: https://github.com/arunoruto/rommate/issues
[stars-shield]: https://img.shields.io/github/stars/arunoruto/rommate.svg?style=for-the-badge
[stars-url]: https://github.com/arunoruto/rommate/stargazers
[license-shield]: https://img.shields.io/github/license/arunoruto/rommate.svg?style=for-the-badge
[license-url]: https://github.com/arunoruto/rommate/blob/main/LICENSE
