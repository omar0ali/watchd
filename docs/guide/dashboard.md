# The dashboard

`watchd ui` opens a live, full-screen dashboard of every loaded project and app in your
terminal, similar to `htop` or `lazygit`.

```sh
watchd ui
```

```text
 watchd   2 project(s) · 3/4 apps running

 shop  ~/work/shop/watchd.lua
❯ ● api     running (healthy)   pid 41233   up 3m12s
  ● web     running             pid 41260   up 3m11s
  ✗ worker  failed (exit 1)                            ↻ 5  exit code 1

 blog  ~/work/blog/watchd.lua
  ● site    running             pid 40211   up 2h10m

 ↑/↓ move · enter logs · r restart · s stop/start · R restart project · c reload · d down · q quit
```

It refreshes every second. Closing it with `q` doesn't affect your apps; they keep running.

## Keys

### App list

| Key | Action |
| --- | --- |
| <kbd>↑</kbd> <kbd>↓</kbd> or <kbd>k</kbd> <kbd>j</kbd> | Move between apps |
| <kbd>enter</kbd> or <kbd>l</kbd> | Open the selected app's logs |
| <kbd>r</kbd> | Restart the selected app |
| <kbd>s</kbd> | Stop the selected app, or start it if it's stopped |
| <kbd>R</kbd> | Restart every running app of the selected app's project |
| <kbd>c</kbd> | Reload the project's config (same as `watchd reload`) |
| <kbd>d</kbd> | Stop and unload the project. Asks for confirmation (<kbd>y</kbd>/<kbd>n</kbd>) |
| <kbd>q</kbd> or <kbd>esc</kbd> | Quit the dashboard |

### Logs view

| Key | Action |
| --- | --- |
| <kbd>↑</kbd> <kbd>↓</kbd>, <kbd>PgUp</kbd> <kbd>PgDn</kbd>, mouse wheel | Scroll |
| <kbd>G</kbd> or <kbd>End</kbd> | Jump to the newest lines and keep following |
| <kbd>g</kbd> or <kbd>Home</kbd> | Jump to the oldest lines |
| <kbd>r</kbd> | Restart the app |
| <kbd>esc</kbd> or <kbd>q</kbd> | Back to the app list |

The logs view follows new output as long as you're at the bottom. Scroll up to pause it (the
header says *paused*), and press <kbd>G</kbd> to resume.

## Reading the list

| Symbol | Meaning |
| --- | --- |
| <span style="color:#2a2">●</span> green | Running (and healthy, if it has a health check) |
| <span style="color:#c90">●</span> yellow | Running, health check not passing yet |
| <span style="color:#c33">●</span> red | Running but unhealthy |
| <span style="color:#c90">◐</span> | Waiting for dependencies, or about to restart after a crash |
| <span style="color:#c33">✗</span> | Failed: couldn't start, or gave up restarting. The reason is shown at the end of the row |
| ◌ | Exited cleanly and not restarted |
| ○ | Stopped |
| ↻ *n* | Number of restarts |

See [How supervision works](/reference/supervision#states) for what each state means.

## When to use the dashboard and when to use commands

The dashboard is for watching and quick actions while you work. The regular commands
(`ps`, `logs`, `restart` and so on) print plain text, so they're the right choice for
scripts, CI and SSH sessions, or for piping into other tools. They can do everything the
dashboard does.
