---
title: Getting started
weight: 1
---

{{% steps %}}

### Install rommate

{{< tabs >}}
  {{< tab name="Install script" >}}
The script picks the right binary for your system, checks its SHA-256 checksum and installs it to `~/.local/bin`:

```sh
curl -fsSL https://arunoruto.github.io/rommate/install.sh | sh
```

Set `ROMMATE_VERSION=v0.1.0` to install a specific release, or `ROMMATE_INSTALL_DIR` to install somewhere else. The
script is [in the repository](https://github.com/arunoruto/rommate/blob/main/install.sh) if you want to read it first.
  {{< /tab >}}
  {{< tab name="bin" >}}
[bin](https://github.com/marcosnils/bin) by Marcos Nils is a small, excellent manager for binaries from GitHub
releases. It picks the right file for your system and can update rommate later:

```sh
bin install github.com/arunoruto/rommate
bin update rommate
```
  {{< /tab >}}
  {{< tab name="Manual download" >}}
Download the binary for your device from the [latest release](https://github.com/arunoruto/rommate/releases/latest):

| Device | Asset |
|---|---|
| Linux ARM64: Armada and other ARM handhelds | `rommate-linux-arm64` |
| Linux x86-64: Steam Deck, Steam Machine, PCs | `rommate-linux-amd64` |
| macOS on Apple Silicon | `rommate-darwin-arm64` |
| Windows x86-64 (untested) | `rommate-windows-amd64.exe` |

```sh
mkdir -p ~/.local/bin
curl -fL -o ~/.local/bin/rommate \
  https://github.com/arunoruto/rommate/releases/latest/download/rommate-linux-arm64
chmod +x ~/.local/bin/rommate
```

On macOS, run `xattr -d com.apple.quarantine rommate` if Gatekeeper blocks the unsigned binary. Each release has a
`checksums.txt` with SHA-256 sums.
  {{< /tab >}}
{{< /tabs >}}

Check it works with `rommate --version`.

### Create a RomM API token

In RomM, open **Administration → Client API Tokens** and create a token for your device. It starts with `rmm_`.

{{< callout type="info" >}}
  rommate only needs to read your library. Give the token the read scopes for ROMs and platforms.
{{< /callout >}}

### Connect rommate to RomM

```sh
rommate init --server https://romm.example.com --token rmm_…
```

`init` checks that it can reach your server before it saves anything to `~/.config/rommate/config.toml`. It also
prints where your games will go. With ES-DE, rommate reads the ROM folder from ES-DE's own settings, so on most
devices there is nothing else to configure. See [Configuration](../configuration) for EmuDeck, RetroDECK and custom
folders.

### Download a game

Start the TUI with `rommate`, open a platform with **A** (`Enter`) and press **A** again on a game. The download bar at
the bottom shows its progress. Press **Select** (`?`) at any time for the list of shortcuts.

Prefer the command line?

```sh
rommate platforms              # what's on the server
rommate roms 3ds               # games of a platform
rommate pull 42                # download a game by id
rommate pull --platform gba    # everything you don't have yet
```

{{% /steps %}}

## Playing in Steam's Game Mode

Add rommate to Steam as a non-Steam game so you can open it from Game Mode:

1. In Desktop Mode, open Steam and choose **Add a Game → Add a Non-Steam Game**.
2. Browse to a terminal such as Konsole and add it. In its properties, set the launch options to `-e rommate`.
3. Give the controller a layout that maps the buttons to keys: see [the TUI page](../tui#controller-setup).

When Steam starts rommate, the footer automatically shows controller buttons instead of keys.
