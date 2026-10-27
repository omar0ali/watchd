# CLI commands

```text
watchd <command> [arguments] [flags]
```

Run `watchd help <command>` for built-in help on any command.

## Global flags

These work with every command:

| Flag | Description |
| --- | --- |
| `-p, --project NAME` | Act on the loaded project with this name, from any directory. |
| `-c, --config FILE` | Use this config file instead of searching for `watchd.lua`. |
| `-h, --help` | Show help. |

## Choosing the project

Commands that act on a project pick it in this order:

1. The `-p NAME` flag.
2. The `watchd.lua` in the current directory or a parent.
3. The only loaded project, if exactly one is loaded.

Wherever a command takes app names, you can also write `PROJECT/APP` to target any project:
`watchd restart shop/api blog/site`. See
[Working with many projects](/guide/projects).

## Setup

### watchd init

Create `watchd.lua` in the current directory, pre-filled with the apps watchd recognises in
the project. See [Getting started](/guide/getting-started#_1-create-a-config).

```sh
watchd init [flags]
```

| Flag | Description |
| --- | --- |
| `-y, --yes` | Don't ask questions; include every detected app. |
| `--name NAME` | Project name (default: the folder name, cleaned up). |
| `--force` | Overwrite an existing `watchd.lua`. |

### watchd validate

Load the config, report every error, and summarise the apps without starting anything.
Alias: `check`.

```sh
watchd validate
watchd validate -c path/to/watchd.lua
```

Exits with status 1 if the config has errors, so you can use it in CI.

## Running projects

### watchd up

Load the project and start its apps. The daemon starts automatically if needed.

```sh
watchd up [APP...] [flags]
```

| Flag | Description |
| --- | --- |
| `-f, --follow` | Follow the logs after starting. Ctrl-C stops following; the apps keep running. |

- With no apps named, every app with `autostart = true` starts.
- Naming apps starts only those, **plus their dependencies**.
- If the project is already loaded, `up` starts whatever isn't running. It doesn't re-read
  the config; use [`reload`](#watchd-reload) for that.
- If there's no `watchd.lua` and you're in a terminal, `up` offers to create one.

```sh
watchd up
watchd up api worker
watchd up -f
watchd up -c ~/work/shop/watchd.lua
```

### watchd down

Stop a project's apps (dependents first) and unload the project.

```sh
watchd down [PROJECT...] [flags]
```

| Flag | Description |
| --- | --- |
| `-a, --all` | Stop and unload every project. |

```sh
watchd down              # the project in this directory
watchd down shop blog    # by name, from anywhere
watchd down --all
```

### watchd reload

Re-read the project's config and restart the project with it.

```sh
watchd reload
watchd reload -p shop
```

- If the new config has errors, they're shown and **the running project is left untouched**.
- Apps that were running start again, as do new apps with `autostart = true`.
- Apps get the environment of the shell you run `reload` from.

### watchd start

Start apps of a loaded project.

```sh
watchd start [APP | PROJECT/APP ...]
```

With no apps named, starts every app with `autostart = true`. Dependencies of the named apps
are started too.

### watchd stop

Stop apps, keeping the project loaded.

```sh
watchd stop [APP | PROJECT/APP ...]
```

With no apps named, stops every app of the project. Stopped apps stay visible in `watchd ps`
and don't restart on file changes until you start them again.

### watchd restart

Restart apps.

```sh
watchd restart [APP | PROJECT/APP ...]
```

With no apps named, restarts every **running** app of the project. Naming a stopped app
starts it.

## Inspecting

### watchd ps

List apps with their state, PID, uptime and restart count.

```sh
watchd ps [PROJECT] [flags]
```

| Flag | Description |
| --- | --- |
| `-a, --all` | Show every loaded project. |
| `--json` | Print JSON. |

Without arguments, shows the current directory's project if it's loaded, and every project
otherwise.

```console
$ watchd ps
PROJECT  APP     STATE              PID    UPTIME  RESTARTS
shop     api     running (healthy)  41233  3m12s   0
shop     web     running            41260  3m11s   0
shop     worker  failed (exit 1)    -      -       5         exit code 1
```

The [states](/reference/supervision#states) are explained in How supervision works.

::: details JSON output
```console
$ watchd ps --json
[
  {
    "project": "shop",
    "name": "api",
    "state": "running",
    "health": "healthy",
    "pid": 41233,
    "started_at": "2026-10-22T19:35:01.53+02:00",
    "restarts": 0
  },
  {
    "project": "shop",
    "name": "worker",
    "state": "failed",
    "started_at": "2026-10-22T19:36:40.12+02:00",
    "restarts": 5,
    "exit_code": 1,
    "last_error": "exit code 1"
  }
]
```
Fields that don't apply are left out: `pid` for an app that isn't running, `health` for an
app without a health check, `exit_code` before the first exit. `started_at` is the most
recent start.
:::

### watchd ls

List loaded projects. Alias: `projects`.

```sh
watchd ls [--json]
```

```console
$ watchd ls
PROJECT  RUNNING  LOADED  CONFIG
blog     1/1      2h10m   ~/work/blog/watchd.lua
shop     2/3      14m03s  ~/work/shop/watchd.lua
```

### watchd logs

Show app output, merged in time order.

```sh
watchd logs [APP | PROJECT/APP ...] [flags]
```

| Flag | Description |
| --- | --- |
| `-f, --follow` | Keep printing new lines. |
| `-n, --lines N` | Number of recent lines to show (default 100; `0` for everything kept). |
| `-t, --timestamps` | Show timestamps. |

```sh
watchd logs
watchd logs -f api
watchd logs shop/web -n 20 -t
```

Lines marked `»` come from watchd itself: starts, exits, restarts, file changes and hook
output. watchd keeps the last 2000 lines per app in memory.

Following stops by itself when the project is unloaded.

### watchd ui

Open the live [dashboard](/guide/dashboard). Alias: `dash`. Needs a terminal.

```sh
watchd ui
```

## Daemon

### watchd on

Start the daemon in the background. You rarely need this: any command that needs the daemon
starts it.

### watchd off

Stop every project and shut the daemon down. Alias: `shutdown`.

### watchd info

Show the daemon's status, and where watchd keeps its socket and log.

```console
$ watchd info
cli:        watchd v0.2.0
socket:     /run/user/1000/watchd/watchd.sock
daemon log: /home/you/.local/state/watchd/daemon.log
daemon:     watchd v0.2.0 (pid 40102), 2 project(s) loaded
```

### watchd daemon

Run the daemon in the foreground, logging to stderr as well as the daemon log. Use it under a
[service manager](/reference/service) or for debugging.

## Other

### watchd version

Print the version.

### watchd completion

Generate shell completion scripts for `bash`, `zsh`, `fish` or `powershell`. See
[Installation](/guide/installation#shell-completion).

## Environment variables

| Variable | Effect |
| --- | --- |
| `WATCHD_RUNTIME_DIR` | Directory for the socket and lock. Default: `$XDG_RUNTIME_DIR/watchd`, or `/tmp/watchd-<uid>`. |
| `XDG_STATE_HOME` | Base directory for the daemon log. Default: `~/.local/state`. |
| `NO_COLOR` | Turns off colours in command output. |

## Exit status

Commands exit with `0` on success and `1` on any error, with the error printed to stderr.
