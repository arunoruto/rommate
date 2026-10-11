---
title: Roadmap
weight: 7
---

What's done and what comes next. Every planned item has a
[GitHub issue](https://github.com/arunoruto/rommate/issues?q=is%3Aissue+label%3Aroadmap) where you can follow progress or
share ideas.

## Done

- RomM client with Client API Token auth, paginated listings and resumable downloads
- RomM → ES-DE folder mapping for every platform, with ES-DE, EmuDeck and RetroDECK profiles
- Safe downloads: resume, file locking, size checks, path-traversal guard, adoption of games already on disk
- CLI with `--json` output for scripts
- Controller-friendly TUI with Library and Downloads tabs, a Steam-style download bar and a help screen
- Keyboard or controller hints depending on how rommate was started
- Releases for Linux (x86-64, ARM64), macOS (Apple Silicon) and Windows, an install script and this website

## Next: browsing and settings

- [#1 Search games with `/`](https://github.com/arunoruto/rommate/issues/1) (keyboard)
- [#2 Collections tab](https://github.com/arunoruto/rommate/issues/2): download a whole RomM collection at once
- [#3 Settings tab](https://github.com/arunoruto/rommate/issues/3): change settings without editing `config.toml`
- [#4 Prefer a local server URL](https://github.com/arunoruto/rommate/issues/4) when it is reachable

## Then: everything around the games

- [#5 BIOS / firmware sync](https://github.com/arunoruto/rommate/issues/5)
- [#6 Save sync](https://github.com/arunoruto/rommate/issues/6) with backups and conflict handling
- [#11 Multi-file games](https://github.com/arunoruto/rommate/issues/11): `.cue`/`.bin` checks and `.m3u` playlists
- [#10 Cover art and gamelist.xml](https://github.com/arunoruto/rommate/issues/10) for ES-DE

## Later

- [#7 Mod manager](https://github.com/arunoruto/rommate/issues/7) for emulators that support mods, linked by title ID
- [#8 Decky plugin](https://github.com/arunoruto/rommate/issues/8) for Game Mode on the Steam Deck
- [#9 Native gamepad input](https://github.com/arunoruto/rommate/issues/9) when started outside Steam
- [#12 Other servers](https://github.com/arunoruto/rommate/issues/12) such as Retrom
- [#13 Packages](https://github.com/arunoruto/rommate/issues/13) for nixpkgs, Flathub and the AUR

{{< callout type="info" >}}
  Missing something? [Open an issue](https://github.com/arunoruto/rommate/issues/new?labels=enhancement) and describe
  your use case.
{{< /callout >}}
