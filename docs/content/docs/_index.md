---
title: Documentation
linkTitle: Docs
next: getting-started
weight: 1
---

rommate syncs games from a [RomM](https://romm.app) server to Linux handhelds such as
[Armada](https://armadaos.dev) devices, the Steam Deck and Steam Machines. It stores every game in the folder your
frontend expects, so ES-DE, EmuDeck and RetroDECK find it without extra setup.

It is a single binary with two faces:

- a **TUI** you drive with a controller or keyboard, and
- a **CLI** for scripts, with a `--json` mode for other programs.

{{< cards >}}
  {{< card link="getting-started" title="Getting started" icon="play" subtitle="Install rommate, connect it to RomM and download your first game." >}}
  {{< card link="tui" title="The TUI" icon="view-grid" subtitle="Screens, keyboard shortcuts and controller setup." >}}
  {{< card link="cli" title="The CLI" icon="terminal" subtitle="Every command, plus JSON output for scripts." >}}
  {{< card link="configuration" title="Configuration" icon="adjustments" subtitle="config.toml, frontend profiles and environment variables." >}}
  {{< card link="folders" title="Folder mapping" icon="folder-open" subtitle="Which RomM platform ends up in which folder." >}}
  {{< card link="development" title="Development" icon="code" subtitle="Build from source, run the checks and cut a release." >}}
  {{< card link="roadmap" title="Roadmap" icon="map" subtitle="What's done and what comes next." >}}
{{< /cards >}}
