---
title: The TUI
weight: 2
---

Run `rommate` without arguments to open the TUI. It is built to be used with a controller from the couch, and works
just as well with a keyboard.

![The Library tab showing platforms and how many games of each are on the device](/images/screenshots/library.png)

## Tabs

**Library**
: Every platform on your server with a `downloaded/total` count. Open a platform to see its games: ✓ means the game
  is on this device, ↓ that it is downloading and • that it is queued. The details pane shows the size, the file name
  and where the game will be stored.

**Downloads**
: The queue: the running download with speed and time left, queued games in order, failed downloads with their error,
  and what finished recently. Cancelled downloads keep their partial file and resume next time.

While something downloads, a bar at the bottom shows its progress on every tab, like Steam does.

![The Downloads tab with one running, two queued, one failed and two finished downloads](/images/screenshots/downloads.png)

## Shortcuts

Press `?` or **Select** to see these in the app. The footer always shows the actions that fit the selected item.

| Action | Keyboard | Controller |
|---|---|---|
| Switch tab | `[` `]` or `Tab` | L1 R1 |
| Move up / down | `↑` `↓` or `k` `j` | D-pad ↑ ↓ |
| Previous / next page | `←` `→` or `PgUp` `PgDn` | D-pad ← → |
| First / last entry | `Home` `End` | |
| Open platform / download game | `Enter` | A |
| Back | `Esc` or `Backspace` | B |
| Download all missing games of a platform | `y` | Y |
| Remove a game (press twice) | `x` | X |
| Reload from the server | `r` | |
| Downloads: retry a failed download | `Enter` | A |
| Downloads: download next / retry all failed | `y` | Y |
| Downloads: cancel, remove or dismiss | `x` | X |
| Show or hide help | `?`, `F1` or `h` | Select |
| Quit (press twice while downloading) | `q` or `Ctrl+C` | Start |

## Controller setup

Through Steam Input, a controller sends ordinary key presses, so rommate needs a controller layout that maps buttons
to the keys above. Create a keyboard layout for the rommate shortcut in Steam with these bindings:

| Button | Key |
|---|---|
| D-pad | arrow keys |
| A | `Enter` |
| B | `Esc` |
| X | `x` |
| Y | `y` |
| L1 / R1 | `[` / `]` |
| Select / View | `F1` |
| Start / Menu | `q` |

{{< callout type="info" >}}
  Select maps to `F1` rather than `?`, because `?` needs Shift and Steam Input binds single keys.
{{< /callout >}}

## Keyboard or controller hints

The footer shows either keyboard keys or controller buttons. With `hints = "auto"` (the default), rommate shows
controller buttons when Steam started it, and keyboard keys in a normal terminal. Set `hints = "keyboard"` or
`hints = "controller"` in the [configuration](../configuration) to force one.

![The help screen listing every action with keyboard and controller bindings](/images/screenshots/help.png)
