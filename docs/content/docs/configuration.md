---
title: Configuration
weight: 6
---

rommate reads `~/.config/rommate/config.toml` on every system, including macOS and Windows. If
`$XDG_CONFIG_HOME` is set, it uses `$XDG_CONFIG_HOME/rommate/config.toml` instead. `rommate init` creates the file
for you, readable only by your user because it holds your API token.

```toml {filename="~/.config/rommate/config.toml"}
server  = "https://romm.example.com"
# local_server = "http://romm.local:8080"   # used whenever it answers
token   = "rmm_…"
profile = "es-de"        # es-de | emudeck | retrodeck
# rom_dir = "/run/media/armada/Games/ROMs"   # default: taken from the profile
# hints = "auto"         # footer hints: auto | keyboard | controller
# gamepad = "auto"       # read controllers directly on Linux: auto | off
# confirm_quit = true    # ask before quitting the TUI
# auto_sync_saves = true # sync saves when the TUI starts and before it quits
# covers = true          # download cover art into ES-DE's media folder
# media_dir = "~/ES-DE/downloaded_media"   # default: taken from ES-DE / the profile

# Use a different folder for individual RomM platforms
[folders]
# gba = "gba-hacks"
```

## Settings

Most of these can also be changed in the TUI's **Settings** tab.

| Key | Default | Description |
|---|---|---|
| `server` | | Your RomM address, for example `https://romm.example.com`. |
| `local_server` | | Optional address of the same RomM server on your home network, e.g. `http://romm.local:8080`. See below. |
| `token` | | A RomM Client API Token (`rmm_…`). |
| `profile` | `es-de` | Which frontend's folder layout to use; see below. |
| `rom_dir` | from the profile | The folder that holds one sub-folder per platform. |
| `hints` | `auto` | Footer hints in the TUI: `auto`, `keyboard` or `controller`. |
| `gamepad` | `auto` | Read game controllers directly on Linux (`auto`) or only use keys (`off`). |
| `auto_sync_saves` | `true` | Sync saves when the TUI starts and before it quits. See [Save sync](../saves). |
| `covers` | `true` | Download each game's cover into the frontend's media folder. See [Folder mapping](../folders#cover-art). |
| `media_dir` | from ES-DE / the profile | The frontend's media folder (ES-DE's `downloaded_media`). |
| `confirm_quit` | `true` | Ask before quitting the TUI with Start or `q`. With running downloads it always asks. |
| `[folders]` | | Per-platform folder overrides, keyed by RomM slug. |

## Profiles

All supported frontends use ES-DE's platform folder names. They only differ in where the ROM folder lives:

| Profile | ROM folder |
|---|---|
| `es-de` | The `ROMDirectory` from `~/ES-DE/settings/es_settings.xml`, otherwise `~/ROMs` |
| `emudeck` | `~/Emulation/roms` |
| `retrodeck` | `~/retrodeck/roms` |

Set `rom_dir` when your games live somewhere else, for example on an SD card.

## Environment variables

These override the config file, which is handy for testing or for running rommate from scripts:

| Variable | Overrides |
|---|---|
| `ROMMATE_SERVER` | `server` |
| `ROMMATE_LOCAL_SERVER` | `local_server` |
| `ROMMATE_TOKEN` | `token` |
| `ROMMATE_ROM_DIR` | `rom_dir` |
| `ROMMATE_HINTS` | `hints` |
| `ROMMATE_GAMEPAD` | `gamepad` |
| `ROMMATE_AUTO_SYNC_SAVES` | `auto_sync_saves` (`true` / `false`) |
| `ROMMATE_CONFIRM_QUIT` | `confirm_quit` (`true` / `false`) |
| `ROMMATE_COVERS` | `covers` (`true` / `false`) |
| `ROMMATE_MEDIA_DIR` | `media_dir` |

## Local server

At home, downloads are faster when they don't take the detour through the internet. Set `local_server` to the address
of your RomM server on the local network, and keep `server` as the public address:

```toml {filename="~/.config/rommate/config.toml"}
server       = "https://romm.example.com"
local_server = "http://romm.local:8080"
```

On every start, rommate checks whether the local server answers (it waits at most three seconds) and uses it if it
does; otherwise it uses `server`. The TUI shows `· local` next to the server name while it uses the local address.
You can also set it during setup with `rommate init --local-server http://romm.local:8080`.

`.local` names work on every system, including Linux handhelds: rommate resolves them over mDNS itself, the same way
Avahi and macOS do. When a host announces several addresses (common with Docker or Kubernetes), rommate tries them all
at once and keeps the one that answers.

## Where rommate keeps its state

rommate records what it downloaded in a small database at `~/.local/state/rommate/state.db` (or
`$XDG_STATE_HOME/rommate/state.db`). You can delete it at any time: rommate recognises games that are already on
disk and records them again on the next pull.
