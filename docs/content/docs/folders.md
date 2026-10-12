---
title: Folder mapping
weight: 4
---

RomM and ES-DE do not always agree on platform names. RomM calls the Nintendo 3DS `3ds`, while ES-DE expects its
games in `n3ds`. A game stored under the wrong name is invisible to ES-DE, even when the emulator is installed.

rommate decides each platform's folder in this order:

1. your override in the `[folders]` section of [`config.toml`](../configuration),
2. the built-in mapping below,
3. otherwise the RomM slug itself. That covers most platforms, such as `gba`, `nes`, `ps2` and `switch`, whose names
   are the same in both.

You can print the same table on your device with `rommate mappings`.

## Built-in mappings

{{< mappings >}}

## Overriding a folder

```toml {filename="~/.config/rommate/config.toml"}
[folders]
gba = "gba-hacks"   # RomM slug = folder name inside your ROM folder
```

## Cover art

When a game downloads, rommate also saves its cover from RomM where ES-DE looks for it:
`<media folder>/<system>/covers/<game file name>.png` (or `.jpg`). Removing the game removes its cover.

The media folder is ES-DE's `MediaDirectory` setting when set (EmuDeck sets it), otherwise `~/ES-DE/downloaded_media`,
`~/Emulation/tools/downloaded_media` for EmuDeck or `~/retrodeck/downloaded_media` for RetroDECK. Set
[`media_dir`](../configuration) to use another folder, or `covers = false` to turn covers off.

For games you downloaded before, run:

```sh
rommate covers
```

{{< callout type="info" >}}
  Missing a mapping? Open an [issue](https://github.com/arunoruto/rommate/issues/new?labels=enhancement) with the
  RomM slug and the folder name your frontend expects.
{{< /callout >}}
