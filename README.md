# watchd

**`docker compose` for local processes, configured in Lua.**

watchd runs all the pieces of a project (APIs, frontends, workers, databases you run
natively) from one config file. It keeps them running, restarts them when their files change
or when they crash, starts them in dependency order, and lets you control **every project on
your machine** from one CLI.

```console
$ cd ~/work/shop && watchd up
shop: started api, web, worker

$ watchd ps
PROJECT  APP     STATE              PID     UPTIME  RESTARTS
shop     api     running (healthy)  351546  8s      0
shop     web     running (healthy)  351608  7s      0
shop     worker  running            351609  7s      0

$ watchd logs -f
api    | » started (pid 351546): go run .
api    | 2026/10/22 19:35:02 api listening on :8080
api    | » healthy (http http://127.0.0.1:8080/health)
web    | » dependencies ready
web    | » started (pid 351608): go run .
worker | 2026/10/22 19:35:06 processed job #1
```

- **Any language, any runtime.** An app is just a shell command: `go run .`, `npm run dev`,
  `cargo watch`, `python -m http.server`, `postgres -D ./data`.
- **Restarts when files change**, with debouncing (saving 20 files means one restart) and
  gitignore-style patterns.
- **Supervises processes**: restart policies with exponential backoff, a retry limit, and
  graceful stops (signal first, then `SIGKILL` after a timeout) that take down the whole
  process tree, so `go run` and `npm` don't leave orphans behind.
- **Dependencies and health checks**: `web` waits until `api` actually answers on its port, at
  its URL, or passes a check command.
- **Many projects, one daemon**: list everything that is running and stop or restart any
  project or app from anywhere (`watchd stop shop/worker`).
- **The config is code**: variables, loops, conditionals, env lookups, and hooks that decide
  whether a change should restart an app.
- **A single static binary**: no Node.js, Python, Docker or tmux needed.

> watchd runs processes directly on your machine. It does **not** isolate them the way
> containers do. Use it for development environments and simple deployments, alongside
> Docker rather than instead of it.

---

## Contents

- [Install](#install)
- [Quick start](#quick-start)
- [Working with many projects](#working-with-many-projects)
- [CLI reference](#cli-reference)
- [Configuration](#configuration)
  - [Project fields](#project-fields) · [App fields](#app-fields) · [Health checks](#health-checks)
  - [Watch patterns](#watch-patterns) · [Environment](#environment) · [Hooks](#hooks)
  - [Lua helpers](#lua-helpers) · [Splitting configs](#splitting-configs)
- [How supervision works](#how-supervision-works)
- [Security](#security)
- [Running the daemon as a service](#running-the-daemon-as-a-service)
- [Architecture](#architecture)
- [Development](#development)
- [Limitations and roadmap](#limitations-and-roadmap)

---

## Install

Requires Go 1.25+ and runs on Linux and macOS.

```sh
go install github.com/omar0ali/watchd@latest
```

Or from a clone:

```sh
scripts/run.sh install   # or: scripts/run.sh build  ->  ./build/watchd
```

Shell completion (it completes project and app names too):

```sh
watchd completion bash > ~/.local/share/bash-completion/completions/watchd
watchd completion zsh  > "${fpath[1]}/_watchd"
watchd completion fish > ~/.config/fish/completions/watchd.fish
```

## Quick start

```sh
cd my-project
watchd init        # writes a commented starter watchd.lua
$EDITOR watchd.lua
watchd validate    # checks the config and shows what it defines
watchd up -f       # starts everything and follows the logs (Ctrl-C detaches)
```

You never start the daemon by hand. The first command that needs it starts it in the
background.

A typical config:

```lua
return {
	name = "shop",
	env_file = ".env",

	apps = {
		{
			name = "api",
			cmd = "go run ./cmd/api",
			watch = { "**/*.go" },
			ignore = { "vendor" },
			healthcheck = { http = "http://127.0.0.1:8080/health" },
		},
		{
			name = "web",
			cmd = "npm run dev",
			cwd = "./web",
			depends_on = { "api" },
			env = { PORT = 3000, API_URL = "http://127.0.0.1:8080" },
		},
		{
			name = "worker",
			cmd = "go run ./cmd/worker",
			depends_on = { "api" },
			watch = { "**/*.go" },
			restart = "on-failure",
			max_restarts = 5,
		},
	},
}
```

A runnable version is in [`examples/microservices`](examples/microservices): three Go
services with health checks, dependencies and hooks. [`examples/hello`](examples/hello) is the
smallest possible config.

```sh
cd examples/microservices && watchd up -f    # then open http://localhost:3000
```

## Working with many projects

One daemon manages every project you bring up. Commands figure out which project you mean
like this:

| You run…                         | It acts on…                                                       |
| -------------------------------- | ----------------------------------------------------------------- |
| `watchd stop api` in a project   | the project whose `watchd.lua` is in this directory or a parent   |
| `watchd stop -p shop api`        | the loaded project named `shop`, from anywhere                    |
| `watchd stop shop/api blog/web`  | apps in several projects at once, from anywhere                   |
| `watchd stop api` elsewhere      | the only loaded project, if exactly one is loaded                 |

```console
$ watchd ls
PROJECT  RUNNING  LOADED  CONFIG
blog     1/1      2h10m   ~/work/blog/watchd.lua
shop     2/3      14m03s  ~/work/shop/watchd.lua

$ watchd ps -a                 # every app of every project
$ watchd restart shop/api      # from any directory
$ watchd logs -f blog/web
$ watchd down blog             # stop and unload one project
$ watchd down --all            # stop and unload all of them
$ watchd off                   # stop everything and exit the daemon
```

Project names must be unique within the daemon. The name defaults to the config's directory
name, and you can set `name = "..."` to override it.

## CLI reference

| Command | What it does |
| --- | --- |
| `watchd init [--force]` | Create a starter `watchd.lua` in the current directory. |
| `watchd validate` | Load the config, report **all** errors at once, and summarise the apps. |
| `watchd up [APP...] [-f]` | Load the project and start its apps (only the named ones, plus their dependencies). Safe to repeat. `-f` follows the logs afterwards. |
| `watchd down [PROJECT...] [-a]` | Stop a project's apps and unload it. `-a` does every project. |
| `watchd reload` | Re-read the config and restart with it. If the new config has errors, the running project is left alone. |
| `watchd start [APP...]` | Start apps of a loaded project. With no apps, starts every `autostart` app. |
| `watchd stop [APP...]` | Stop apps but keep the project loaded. With no apps, stops all of them. |
| `watchd restart [APP...]` | Restart apps, starting them if they are stopped. With no apps, restarts every running app. |
| `watchd ps [PROJECT] [-a] [--json]` | App state, PID, uptime and restart count. Shows the current project if it is loaded, otherwise every project. |
| `watchd ls [--json]` | List loaded projects. |
| `watchd logs [APP...] [-f] [-n N] [-t]` | Recent output (the last 100 lines by default; `-n 0` shows everything kept). `-f` follows, `-t` adds timestamps. Lines marked `»` come from watchd itself. |
| `watchd on` / `watchd off` | Start the daemon / stop every project and exit the daemon. |
| `watchd info` | Daemon status, socket path and log file location. |
| `watchd daemon` | Run the daemon in the foreground (for systemd or debugging). |
| `watchd version` | Print the version. |

Global flags: `-p, --project NAME` targets a loaded project by name, and `-c, --config FILE`
uses a config file other than `./watchd.lua`. Wherever an app is expected, you can also pass
`PROJECT/APP`.

## Configuration

The config is a Lua file (named `watchd.lua` by default) that **returns a table**. watchd
checks it strictly: misspelled fields, wrong types, unknown dependencies and dependency
cycles are all reported together, before anything starts:

```
error: /home/me/shop/watchd.lua:
  - app "web": unknown field(s) depend_on (known: autostart, cmd, cwd, debounce, depends_on, ...)
  - app "api": restart must be "always", "on-failure" or "never", got "sometimes"
```

**Durations** can be a number of seconds (`5`, `0.5`) or a string (`"300ms"`, `"2s"`,
`"1m30s"`). **Relative paths** are resolved against the config file's directory, and `~` is
expanded.

### Project fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | config directory name | Project name. Letters, digits, `.`, `_` and `-` only. |
| `apps` | list or map | **required** | The apps. Either a list of tables with `name`, or a map such as `apps = { api = {...}, web = {...} }`. |
| `env` | table | `{}` | Environment variables for every app. |
| `env_file` | string or list | none | Dotenv file(s) loaded before `env`. |

### App fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | map key | App name, unique within the project. |
| `cmd` | string | **required** | Command run with `/bin/sh -c`, so pipes, `&&` and `$VARS` work. |
| `cwd` | string | config directory | Working directory. Watch patterns are relative to it too. |
| `env` | table | `{}` | Extra environment variables. Numbers and booleans are converted to strings. |
| `env_file` | string or list | none | Dotenv file(s) for this app. |
| `depends_on` | list | `{}` | Apps that must be **ready** before this one starts. See [How supervision works](#how-supervision-works). |
| `watch` | string or list | none | Restart when matching files change. Leave it out to disable watching. |
| `ignore` | string or list | none | Files and directories to skip. `.git`, editor swap files and similar are always ignored. |
| `debounce` | duration | `"300ms"` | How long to wait after the last change before restarting. |
| `restart` | string | `"on-failure"` | `"always"` restarts on any exit; `"on-failure"` only on a non-zero exit or a signal; `"never"` doesn't restart. |
| `max_restarts` | number | `0` (unlimited) | Give up after this many quick crashes in a row. |
| `stop_signal` | string | `"SIGTERM"` | Signal sent on stop or restart: `SIGTERM`, `SIGINT`, `SIGQUIT`, `SIGHUP`, `SIGUSR1`, `SIGUSR2` or `SIGKILL` (the `SIG` prefix is optional). |
| `stop_timeout` | duration | `10` | How long to wait after the stop signal before sending `SIGKILL`. |
| `healthcheck` | table | none | How to tell the app is ready. See [Health checks](#health-checks). |
| `autostart` | boolean | `true` | Set to `false` to skip the app on a plain `watchd up`. Start it with `watchd start APP` or `watchd up APP`. |
| `on_change` | function | none | `function(file, app)`, called for each changed file. Return `false` to skip the restart. |
| `on_start` | function | none | `function(app)`, called after the process starts. |
| `on_exit` | function | none | `function(app, code)`, called when the process exits on its own. |

### Health checks

Without a health check, an app is ready as soon as its process starts. With one, it is ready
after the first passing check. Set **exactly one** of `port`, `http` or `cmd`:

```lua
healthcheck = { port = 5432 }                                  -- TCP connect to host:port
healthcheck = { http = "http://127.0.0.1:8080/health" }        -- any status below 400 passes
healthcheck = { cmd = "pg_isready -h localhost", interval = 1 } -- exit code 0 passes
```

| Field | Default | Description |
| --- | --- | --- |
| `host` | `"127.0.0.1"` | Host for `port` checks. |
| `interval` | `2` | Time between checks. The first check runs about 0.5s after start. |
| `timeout` | `1` | Time allowed for one check. |
| `retries` | `3` | Consecutive failures before the app is marked `unhealthy`. |

`watchd ps` shows `running (starting)`, `running (healthy)` or `running (unhealthy)`. An
unhealthy app is **not** restarted automatically; the status is there so you can see it.

### Watch patterns

Patterns use `**` and `{a,b}` and follow the same rules as `.gitignore`:

| Pattern | Matches |
| --- | --- |
| `*.go` | Every `.go` file at **any** depth, because a pattern without `/` matches the file name. |
| `src/**/*.{ts,tsx}` | `.ts` and `.tsx` files anywhere under `src/`. A pattern with `/` matches the path from `cwd`. |
| `node_modules` (in `ignore`) | The directory and everything below it, wherever it appears. |
| `build/out` (in `ignore`) | Only that path. |

New directories are picked up automatically. If you hit the inotify limit on a huge tree,
watchd tells you; add large directories to `ignore`.

### Environment

Each app's environment is built in this order, with later layers winning:

1. The environment of the shell that ran `watchd up` or `watchd reload` (so `PATH`, `nvm`,
   `pyenv` and the like behave as they would in your terminal)
2. The project's `env_file`, then the project's `env`
3. The app's `env_file`, then the app's `env`
4. `WATCHD_PROJECT` and `WATCHD_APP`

Dotenv files support `KEY=value`, `export KEY=value`, `"double quoted\n"` (with escapes),
`'single quoted'` (literal) and `# comments`.

### Hooks

Hooks are Lua functions that run inside the daemon. `app` is a table:
`{ name, project, cwd, pid }`.

```lua
on_change = function(file, app)
	-- don't restart for tests or docs
	if file:match("_test%.go$") or file:match("%.md$") then
		return false
	end
	print("rebuilding " .. app.name .. " because " .. file .. " changed")
end,

on_exit = function(app, code)
	if code ~= 0 then print(app.name .. " crashed with " .. code) end
end,
```

`print()` in a hook writes to the app's log (it shows up in `watchd logs`, marked with `»`).
Each hook call is limited to **2 seconds**. A hook that errors or times out is reported in the
logs and never takes the daemon down.

### Lua helpers

The config runs in a restricted Lua 5.1 environment with `string`, `table` and `math`,
`os.time`, `os.date` and `os.clock`, plus:

| Helper | Description |
| --- | --- |
| `watchd.env(name, default)` | A variable from the environment of the shell that ran `watchd up` (`os.getenv` reads the same). |
| `watchd.exists(path)` | Whether a file exists (relative to the config directory). |
| `watchd.dir` | Absolute path of the config directory. |
| `watchd.os` | `"linux"` or `"darwin"`. |

Because the config is code, you can compute it:

```lua
local services = { "users", "orders", "billing" }
local apps = {}
for i, svc in ipairs(services) do
	table.insert(apps, {
		name = svc,
		cmd = "go run ./cmd/" .. svc,
		env = { PORT = 9000 + i },
		watch = { "cmd/" .. svc .. "/**/*.go", "internal/**/*.go" },
		healthcheck = { port = 9000 + i },
	})
end

if watchd.exists("web/package.json") then
	table.insert(apps, { name = "web", cmd = "npm run dev", cwd = "web", depends_on = services })
end

return { name = "platform", apps = apps }
```

### Splitting configs

`require` loads modules from the config's directory:

```lua
-- watchd.lua
local common = require("watchd_common")   -- loads ./watchd_common.lua
return { apps = { common.go_service("api", 8080), common.go_service("auth", 8081) } }
```

## How supervision works

**States** (shown by `watchd ps`):

| State | Meaning |
| --- | --- |
| `waiting` | Started, waiting for `depends_on` apps to become ready. |
| `running` | The process is running (health is shown in parentheses if there is a check). |
| `backoff` | Crashed; a restart is scheduled. |
| `exited` | Exited cleanly and its policy says not to restart. |
| `failed` | Couldn't start, crashed with `restart = "never"`, or hit `max_restarts`. |
| `stopped` | Not supervised (never started, or stopped with `watchd stop`). |

- **Crash restarts** back off exponentially: 1s, 2s, 4s… up to 30s. A run that lasts longer
  than 10 seconds resets the backoff and the `max_restarts` counter.
- **File changes** restart the app immediately, even when it is in `backoff`, `exited` or
  `failed`. Fix the bug, save, and it comes back.
- **Stopping** sends `stop_signal` to the app's whole **process group**, waits up to
  `stop_timeout`, then sends `SIGKILL`. Anything the command started (the compiled binary
  behind `go run`, the node process behind `npm run`) is stopped with it.
- **Order**: `up` starts dependencies first. Each dependent waits until its dependencies are
  ready (running, and healthy if they have a health check). `down` and `stop` stop dependents
  before their dependencies.
- **Logs**: watchd keeps the last 2000 lines per app in memory. They are lost when the project
  is unloaded or the daemon stops.

## Security

watchd runs commands as you, so it is designed so that nobody else can make it run anything:

- **No network listener.** The CLI talks to the daemon over a Unix socket only.
- **The socket is private.** It lives in `$XDG_RUNTIME_DIR/watchd/` (or
  `/tmp/watchd-<uid>/`). watchd creates that directory with mode `0700` and refuses to use it
  if it is a symlink or belongs to another user, which blocks socket hijacking through a
  pre-created directory in `/tmp`. The socket itself is `0600`.
- **Callers are verified.** On Linux, every connection's user ID is checked with `SO_PEERCRED`
  and connections from other users are rejected.
- **Configs anyone can edit are refused.** watchd won't load a config or env file that is
  world-writable, because anyone who can edit it could run commands as you.
- **The Lua sandbox protects the daemon.** Configs and hooks have no `io` or `debug` library,
  no `os.execute`, `os.exit`, `os.remove` or `os.rename`, and no `dofile` or `loadfile`.
  Evaluating a config is limited to 5s and each hook to 2s, so a bad config can't hang, kill or
  write files from the daemon. (This guards against mistakes, not malicious configs: a config
  runs shell commands by design, so only load configs you trust, as you would a Makefile.)
- **Requests are bounded.** They are limited to 1 MiB and must arrive within 10 seconds.
  Every log line is capped at 16 KiB and every per-app buffer has a fixed size.
- **Files are private.** The daemon log (`~/.local/state/watchd/daemon.log`) is `0600`, and
  env values are never written to it or shown by `ps` or `validate`.
- **Only one daemon runs per user,** enforced with a lock file.
- **No root needed.** The daemon logs a warning if it is started as root.

## Running the daemon as a service

Auto-start is enough for development. To keep projects running across logins on a server,
run the daemon under systemd (user service):

```ini
# ~/.config/systemd/user/watchd.service
[Unit]
Description=watchd process supervisor

[Service]
ExecStart=%h/go/bin/watchd daemon
Restart=on-failure

[Install]
WantedBy=default.target
```

```sh
systemctl --user enable --now watchd
loginctl enable-linger "$USER"   # keep it running when you're logged out
```

Projects aren't remembered across daemon restarts yet, so run `watchd up` for each project
once the daemon is up (from a script or an `ExecStartPost=` line).

## Architecture

```
 watchd CLI ──(JSON over unix socket)──▶ watchd daemon
 (stateless)                             ├── project "shop"  (Lua state for hooks)
                                         │    ├── app api     process group, log buffer, health checks
                                         │    ├── app web     waits for api
                                         │    └── watchers    fsnotify + debounce, one per app
                                         └── project "blog"
```

| Package | Responsibility |
| --- | --- |
| `cmd/` | Cobra CLI. Turns commands into requests and formats the output. |
| `internal/config` | Lua sandbox, strict parsing and validation, dotenv files, hooks. |
| `internal/supervisor` | Per-app supervision loop, process groups, restart policies, health checks, log buffers. |
| `internal/watcher` | Recursive fsnotify watching, glob matching, debouncing. |
| `internal/daemon` | Project registry, request routing, lifecycle, signals, lock. |
| `internal/ipc` | Socket server and client, peer credential checks. |
| `internal/paths` | Runtime and state directories and their permission checks. |
| `internal/api` | Request, response and status types shared by the CLI and the daemon. |

## Development

```sh
scripts/run.sh test        # go vet + go test -race ./...
scripts/run.sh dev ps      # run the CLI from source
scripts/run.sh build       # ./build/watchd with the version stamped in
```

To try it without touching your real daemon, point it at separate directories:

```sh
export WATCHD_RUNTIME_DIR=/tmp/watchd-dev XDG_STATE_HOME=/tmp/watchd-dev-state
```

## Limitations and roadmap

- Linux and macOS only (it relies on Unix sockets and process groups).
- Logs are kept in memory only. Writing per-app log files with rotation is planned.
- Projects don't survive a daemon restart. Persisting them is planned.
- If the daemon is killed with `SIGKILL`, the apps it was running keep running. Use
  `watchd off` (or `SIGTERM`) for a clean stop.
- Ideas: restart an app when it turns unhealthy, `watchd exec APP -- cmd` to run a command
  with an app's environment, a TUI dashboard, and per-app resource limits.
