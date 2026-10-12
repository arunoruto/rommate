---
title: The CLI
weight: 3
---

Every TUI action is also a command, which makes rommate easy to script over SSH or from other tools.

| Command | What it does |
|---|---|
| `rommate` | Open the TUI. |
| `rommate init --server URL --token rmm_… [--local-server URL] [--profile P] [--rom-dir DIR]` | Save the connection and folder settings after checking them. |
| `rommate platforms` | List the server's platforms and the folder each one maps to. |
| `rommate roms <platform>` | List a platform's games; ✓ marks the ones on this device. `<platform>` can be the RomM slug, the folder name or the id. |
| `rommate pull <rom-id>…` | Download games by id. |
| `rommate pull --platform <platform>` | Download every game of a platform you don't have yet. |
| `rommate remove <rom-id>…` | Delete downloaded games from the device. |
| `rommate status` | List what rommate has downloaded and flag files that went missing. |
| `rommate mappings` | Show the platforms whose ES-DE folder differs from the RomM slug. |
| `rommate config` | Show the effective configuration, including environment overrides, and whether it is complete. The token is never printed. |
| `rommate version` | Print version, commit and build date. |

Flags can go before or after arguments: `rommate roms nes --json` and `rommate roms --json nes` both work.

## Examples

```sh
# What does the server have, and where will it go?
rommate platforms

# Download two games, then check
rommate pull 9 12
rommate status

# Fill up the Game Boy Advance folder
rommate pull --platform gba
```

`pull` is safe to repeat. Games that are already complete are skipped, and partial downloads continue where they
stopped.

## JSON output

Add `--json` to any command for machine-readable output. Lists come back as JSON arrays, and `pull` prints one event
per line while it works:

```json
{"event":"start","rom_id":9,"name":"Pokémon Omega Ruby","path":"/…/n3ds/omega.3ds","total":2147483648}
{"event":"progress","rom_id":9,"name":"Pokémon Omega Ruby","file":"omega.3ds","done":524288000,"total":2147483648}
{"event":"done","rom_id":9,"name":"Pokémon Omega Ruby","path":"/…/n3ds/omega.3ds","done":2147483648,"total":2147483648}
```

| Event | Meaning |
|---|---|
| `start` | A download began; `total` is the size in bytes. |
| `progress` | Bytes so far, sent at most four times per second. |
| `done` | The game is complete at `path`. |
| `skipped` | The game was already on the device. |
| `error` | The download failed; `error` says why. |

The exit code is non-zero when any download failed.
