---
title: Configuration
weight: 4
---

rommate reads `~/.config/rommate/config.toml` on every system, including macOS and Windows. If
`$XDG_CONFIG_HOME` is set, it uses `$XDG_CONFIG_HOME/rommate/config.toml` instead. `rommate init` creates the file
for you, readable only by your user because it holds your API token.

```toml {filename="~/.config/rommate/config.toml"}
server  = "https://romm.example.com"
token   = "rmm_…"
profile = "es-de"        # es-de | emudeck | retrodeck
# rom_dir = "/run/media/armada/Games/ROMs"   # default: taken from the profile
# hints = "auto"         # footer hints: auto | keyboard | controller

# Use a different folder for individual RomM platforms
[folders]
# gba = "gba-hacks"
```

## Settings

| Key | Default | Description |
|---|---|---|
| `server` | | Your RomM address, for example `https://romm.example.com`. |
| `token` | | A RomM Client API Token (`rmm_…`). |
| `profile` | `es-de` | Which frontend's folder layout to use; see below. |
| `rom_dir` | from the profile | The folder that holds one sub-folder per platform. |
| `hints` | `auto` | Footer hints in the TUI: `auto`, `keyboard` or `controller`. |
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
| `ROMMATE_TOKEN` | `token` |
| `ROMMATE_ROM_DIR` | `rom_dir` |
| `ROMMATE_HINTS` | `hints` |

## Where rommate keeps its state

rommate records what it downloaded in a small database at `~/.local/state/rommate/state.db` (or
`$XDG_STATE_HOME/rommate/state.db`). You can delete it at any time: rommate recognises games that are already on
disk and records them again on the next pull.
