---
title: Getting started
weight: 1
---

{{% steps %}}

### Download rommate

Pick the binary for your device from the [latest release](https://github.com/arunoruto/rommate/releases/latest):

| Device | Asset |
|---|---|
| Linux ARM64: Armada and other ARM handhelds | `rommate-linux-arm64` |
| Linux x86-64: Steam Deck, Steam Machine, PCs | `rommate-linux-amd64` |
| macOS on Apple Silicon | `rommate-darwin-arm64` |
| Windows x86-64 (untested) | `rommate-windows-amd64.exe` |

{{< tabs >}}
  {{< tab name="ARM handheld" >}}
```sh
mkdir -p ~/.local/bin
curl -fL -o ~/.local/bin/rommate \
  https://github.com/arunoruto/rommate/releases/latest/download/rommate-linux-arm64
chmod +x ~/.local/bin/rommate
```
  {{< /tab >}}
  {{< tab name="Steam Deck / x86" >}}
```sh
mkdir -p ~/.local/bin
curl -fL -o ~/.local/bin/rommate \
  https://github.com/arunoruto/rommate/releases/latest/download/rommate-linux-amd64
chmod +x ~/.local/bin/rommate
```
  {{< /tab >}}
  {{< tab name="macOS" >}}
```sh
curl -fL -o rommate https://github.com/arunoruto/rommate/releases/latest/download/rommate-darwin-arm64
chmod +x rommate
xattr -d com.apple.quarantine rommate   # the binary is not signed
```
  {{< /tab >}}
{{< /tabs >}}

Check it works with `rommate --version`. Each release also has a `checksums.txt` with SHA-256 sums.

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
