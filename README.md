# watchd

**`docker compose` for local processes, configured in Lua.**

📖 **Documentation: [omar0ali.github.io/watchd](https://omar0ali.github.io/watchd/)**

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
- **Quick setup**: `watchd init` (or a plain `watchd up`) looks at your project and writes a
  ready-to-run config for common stacks.
- **A live dashboard**: `watchd ui` shows every project and app, with keys to restart, stop,
  start and view logs.
- **A single static binary**: no Node.js, Python, Docker or tmux needed.

> watchd runs processes directly on your machine. It does **not** isolate them the way
> containers do. Use it for development environments and simple deployments, alongside
> Docker rather than instead of it.

---

## Contents

- [Install](#install)
- [Usage guide](#usage-guide): your first project, step by step
- [Everyday commands](#everyday-commands)
- [Recipes](#recipes): Node, Python, databases, monorepos and more
- [Working with many projects](#working-with-many-projects)
- [CLI reference](#cli-reference)
- [Configuration](#configuration)
  - [Project fields](#project-fields) · [App fields](#app-fields) · [Health checks](#health-checks)
  - [Watch patterns](#watch-patterns) · [Environment](#environment) · [Hooks](#hooks)
  - [Lua helpers](#lua-helpers) · [Splitting configs](#splitting-configs)
- [How supervision works](#how-supervision-works)
- [Security](#security)
- [Running the daemon as a service](#running-the-daemon-as-a-service)
- [Troubleshooting](#troubleshooting)
- [Architecture](#architecture)
- [Development](#development) · [Releasing](#releasing)
- [Limitations and roadmap](#limitations-and-roadmap)

---

## Install

**Requirements:** Linux or macOS (Intel or ARM). watchd is a single binary with no runtime
dependencies. You don't need Lua installed, because it is built in. Go is only needed to build
from source.

### Option 1: download a prebuilt binary

Every release on the [Releases page](https://github.com/omar0ali/watchd/releases) has archives
for Linux and macOS on amd64 and arm64. For example, to install the latest release on Linux
amd64 into `~/.local/bin`:

```sh
VERSION=$(curl -fsSL https://api.github.com/repos/omar0ali/watchd/releases/latest | grep -m1 '"tag_name"' | cut -d'"' -f4)
curl -fsSLO "https://github.com/omar0ali/watchd/releases/download/${VERSION}/watchd_${VERSION#v}_linux_amd64.tar.gz"
tar xzf "watchd_${VERSION#v}_linux_amd64.tar.gz" watchd
install -m 755 watchd ~/.local/bin/
```

Replace `linux_amd64` with `linux_arm64`, `darwin_amd64` or `darwin_arm64` to match your
machine. Each release also has a `checksums.txt` you can check the download against with
`sha256sum -c --ignore-missing checksums.txt`.

### Option 2: `go install`

Requires [Go 1.25 or newer](https://go.dev/dl/).

```sh
go install github.com/omar0ali/watchd@latest
```

This puts the binary in `$(go env GOPATH)/bin`, usually `~/go/bin`.

### Option 3: build from source

```sh
git clone https://github.com/omar0ali/watchd.git
cd watchd
scripts/run.sh install          # go install, with the version number stamped in
```

Or build the binary and put it wherever you like:

```sh
scripts/run.sh build            # creates ./build/watchd
install -m 755 build/watchd ~/.local/bin/
```

### Put it on your PATH

If `watchd` is "command not found" after installing, add Go's bin directory to your `PATH`
(in `~/.bashrc`, `~/.zshrc`, or the equivalent for your shell):

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Check that it works

```sh
watchd version
watchd info          # shows where the socket and the daemon log live
```

### Shell completion (optional)

Tab completion covers commands, flags, and the names of your loaded projects and apps:

```sh
# bash
watchd completion bash > ~/.local/share/bash-completion/completions/watchd
# zsh (make sure the directory is in your $fpath)
watchd completion zsh > "${fpath[1]}/_watchd"
# fish
watchd completion fish > ~/.config/fish/completions/watchd.fish
```

### Update and uninstall

To update, run the same install command again, then restart the daemon so it uses the new
binary:

```sh
watchd off && watchd on
```

To uninstall:

```sh
watchd off                                    # stop everything first
rm "$(command -v watchd)"
rm -rf ~/.local/state/watchd                  # the daemon log
```

---

## Usage guide

This walkthrough takes a project with a Go API and a Node frontend from nothing to running.
The same steps apply to any language.

### 1. Create a config

In the root of your project:

```sh
cd ~/work/shop
watchd init
```

watchd looks at the project folder and its immediate subfolders for a few common project
types, and pre-fills an app for each one it finds:

| It finds | It adds |
| --- | --- |
| `Procfile` | One app per line (when there is a Procfile, nothing else is detected) |
| `go.mod` and a `main` package | `go run .`, plus one app per `cmd/<name>/main.go` |
| `package.json` with a `dev` or `start` script | `npm run dev`, or `pnpm`, `yarn` or `bun` based on the lockfile |
| `Cargo.toml` | `cargo run` |
| `manage.py` | Django `runserver --noreload`, using `.venv` if there is one |

In a terminal, you can confirm the project name (it defaults to the folder name, cleaned up,
so `My App (old)` becomes `my-app-old`) and untick apps you don't want:

```console
┃ Project name
┃ > shop

  Apps found
  > ✓ api  go run .      ./api
    ✓ web  pnpm run dev  ./web

  Create ~/work/shop/watchd.lua?   Create   Cancel
```

If nothing is recognised, you get a commented template to fill in. `watchd init --yes` skips
the questions (useful in scripts), and `--name` sets the project name.

**Shortcut:** running `watchd up` in a folder without a config offers this same setup, then
asks whether to start the project. When there's no terminal to ask in, as in scripts and CI,
it never creates files on its own.

The result is a normal Lua file. Commit it with your code so your whole team gets the same
setup.

### 2. Describe your apps

Check what was generated and adjust it. An **app** is any long-running command. A config
looks like this:

```lua
return {
	name = "shop",

	apps = {
		{
			name = "api",
			cmd = "go run ./cmd/api",                  -- any shell command
			watch = { "**/*.go" },                     -- restart when Go files change
			healthcheck = { port = 8080 },             -- ready once port 8080 accepts connections
		},
		{
			name = "web",
			cmd = "npm run dev",
			cwd = "./web",                             -- relative to watchd.lua
			depends_on = { "api" },                    -- start only after api is ready
			env = { API_URL = "http://localhost:8080" },
		},
	},
}
```

Only `name` and `cmd` are required. Everything else is optional; see
[App fields](#app-fields) for the full list.

### 3. Check it

```sh
watchd validate
```

This lists every app as watchd understands it, or **every** mistake at once (misspelled
fields, wrong types, unknown dependencies). It doesn't start anything.

### 4. Start it

```sh
watchd up
```

That's it. The first time you run it, watchd starts its background daemon automatically. `up`
returns right away and your apps keep running after you close the terminal.

Add `-f` to watch the output as everything starts (press Ctrl-C to stop watching; the apps
keep running):

```sh
watchd up -f
```

### 5. See what's running

```console
$ watchd ps
PROJECT  APP  STATE              PID    UPTIME  RESTARTS
shop     api  running (healthy)  41233  12s     0
shop     web  running            41260  11s     0
```

If something isn't `running`, its logs usually say why:

```sh
watchd logs              # recent output of every app, merged in time order
watchd logs api -f       # follow one app
```

Lines starting with `»` come from watchd itself: starts, exits, restarts, changed files.

Or open the live dashboard:

```sh
watchd ui
```

```
 watchd   2 project(s) · 3/3 apps running

 shop  ~/work/shop/watchd.lua
❯ ● api    running (healthy)   pid 41233   up 3m12s
  ● web    running             pid 41260   up 3m11s

 blog  ~/work/blog/watchd.lua
  ● clock  running             pid 41302   up 40s

 ↑/↓ move · enter logs · r restart · s stop/start · R restart project · c reload · d down · q quit
```

It refreshes every second. Press `enter` on an app to follow its logs (`esc` goes back). `q`
closes the dashboard, and your apps keep running.

### 6. Write code

Edit a `.go` file and save. watchd notices, waits for you to stop saving (300ms by default),
and restarts `api`. If your change made the app crash, it retries with increasing delays.
Fix the code and save again, and it comes straight back.

### 7. Control individual apps

```sh
watchd restart api       # restart one app
watchd stop web          # stop one app (the project stays loaded)
watchd start web         # start it again
```

### 8. Change the config

After editing `watchd.lua`, apply it:

```sh
watchd reload
```

If the new config has a mistake, `reload` shows the errors and leaves the running version
alone.

### 9. Stop

```sh
watchd down              # stop this project's apps and unload it
watchd off               # stop every project and shut the daemon down
```

You can run any of these commands from the project directory or any subdirectory. To control
a project from elsewhere, see [Working with many projects](#working-with-many-projects).

### Try the examples

[`examples/microservices`](examples/microservices) is a runnable project with three Go
services, health checks, dependencies and hooks.
[`examples/hello`](examples/hello) is the smallest possible config.

```sh
cd examples/microservices && watchd up -f    # then open http://localhost:3000
```

---

## Everyday commands

| I want to… | Run |
| --- | --- |
| Create a config for this project | `watchd init` |
| Start the project in this directory | `watchd up` |
| Watch and control everything live | `watchd ui` |
| Start it and watch the output | `watchd up -f` |
| Start only some apps | `watchd up api worker` |
| See what's running here / everywhere | `watchd ps` / `watchd ps -a` |
| See every loaded project | `watchd ls` |
| Read or follow logs | `watchd logs` / `watchd logs -f api` |
| Restart one app | `watchd restart api` |
| Apply config changes | `watchd reload` |
| Stop one app, keep the project loaded | `watchd stop api` |
| Stop and unload this project | `watchd down` |
| Control another project from anywhere | `watchd restart shop/api`, `watchd logs -p shop` |
| Stop everything | `watchd down --all` or `watchd off` |
| Find out why the daemon won't start | `watchd info`, then read the daemon log it shows |

---

## Recipes

### Go API with a fast rebuild

`go run` works, but a separate build step makes compile errors easier to spot and leaves you a
reusable binary:

```lua
{
	name = "api",
	cmd = "go build -o ./tmp/api ./cmd/api && ./tmp/api",
	watch = { "**/*.go", "go.mod" },
	ignore = { "tmp" },
	healthcheck = { http = "http://127.0.0.1:8080/healthz" },
}
```

### Node / Vite / Next.js frontend

Dev servers like Vite and Next.js already reload in the browser when you edit code, so don't
set `watch` for the source files. Let watchd keep the process alive, and restart it only when
dependencies or config change:

```lua
{
	name = "web",
	cmd = "npm run dev",
	cwd = "./web",
	watch = { "package.json", "vite.config.*" },
	env = { PORT = 3000 },
	healthcheck = { port = 3000 },
}
```

### Python (Django, Flask, FastAPI)

Turn off the framework's own reloader and let watchd do the restarting, so you don't get two
reloaders fighting:

```lua
{
	name = "django",
	cmd = "python manage.py runserver --noreload 0.0.0.0:8000",
	cwd = "./backend",
	watch = { "**/*.py" },
	ignore = { ".venv", "__pycache__" },
	env_file = "backend/.env",
	healthcheck = { port = 8000 },
}
```

Because apps inherit the environment of the shell that ran `watchd up`, activate your
virtualenv first. Or point at its Python directly: `cmd = ".venv/bin/python manage.py ..."`.

### Local Postgres and Redis (without Docker)

Make other apps depend on them so they start only once the database accepts connections:

```lua
{
	name = "postgres",
	cmd = "postgres -D ./.data/pg -p 5432",
	healthcheck = { cmd = "pg_isready -h 127.0.0.1 -p 5432", interval = 1 },
	stop_signal = "SIGINT",          -- Postgres "fast shutdown"
	stop_timeout = 30,
},
{
	name = "redis",
	cmd = "redis-server --port 6379 --save ''",
	healthcheck = { cmd = "redis-cli -p 6379 ping" },
},
{
	name = "api",
	cmd = "go run ./cmd/api",
	depends_on = { "postgres", "redis" },
},
```

(Create the data directory once with `initdb -D ./.data/pg`.)

### Using Docker for some services

watchd can supervise `docker run` like any other command. Use `--rm` and a fixed container name
so restarts don't pile up containers:

```lua
{
	name = "db",
	cmd = "docker run --rm --name shop-db -p 5432:5432 -e POSTGRES_PASSWORD=dev postgres:16",
	healthcheck = { port = 5432 },
	stop_timeout = 20,
}
```

### Apps you only need sometimes

`autostart = false` keeps an app out of a plain `watchd up`. Start it when you need it:

```lua
{ name = "storybook", cmd = "npm run storybook", cwd = "./web", autostart = false },
```

```sh
watchd start storybook
```

### Scripts that run once

For setup steps such as migrations or code generation, use `restart = "never"`. A clean exit
shows up as `exited (exit 0)` in `watchd ps`:

```lua
{ name = "migrate", cmd = "go run ./cmd/migrate up", restart = "never", depends_on = { "postgres" } },
```

Other apps can depend on a task like this. They start once it **finishes successfully** (exit
code 0), and keep waiting if it fails:

```lua
{ name = "api", cmd = "go run ./cmd/api", depends_on = { "migrate" } },
```

### Many similar services

The config is Lua, so generate repetitive entries with a loop. See [Lua helpers](#lua-helpers)
for an example that builds one app per service in a monorepo.

### Personal overrides without editing the shared config

Read values from your environment, with defaults:

```lua
local api_port = watchd.env("API_PORT", "8080")
```

```sh
API_PORT=9090 watchd up      # or: export API_PORT=9090 in your shell profile
```

---

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
| `watchd init [-y] [--name N] [--force]` | Create `watchd.lua`, pre-filled with the apps found in the project. `-y` skips the questions. |
| `watchd validate` | Load the config, report **all** errors at once, and summarise the apps. |
| `watchd up [APP...] [-f]` | Load the project and start its apps (only the named ones, plus their dependencies). Safe to repeat. `-f` follows the logs afterwards. |
| `watchd down [PROJECT...] [-a]` | Stop a project's apps and unload it. `-a` does every project. |
| `watchd reload` | Re-read the config and restart with it. If the new config has errors, the running project is left alone. |
| `watchd start [APP...]` | Start apps of a loaded project. With no apps, starts every `autostart` app. |
| `watchd stop [APP...]` | Stop apps but keep the project loaded. With no apps, stops all of them. |
| `watchd restart [APP...]` | Restart apps, starting them if they are stopped. With no apps, restarts every running app. |
| `watchd ps [PROJECT] [-a] [--json]` | App state, PID, uptime and restart count. Shows the current project if it is loaded, otherwise every project. |
| `watchd ls [--json]` | List loaded projects. |
| `watchd ui` | Live full-screen dashboard of every project and app, with keys to restart, stop or start, follow logs, reload and unload. |
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
| `depends_on` | list | `{}` | Apps that must be **ready** before this one starts: running (and healthy, if they have a health check), or finished with exit code 0. See [How supervision works](#how-supervision-works). |
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
  ready (running and healthy, or finished with exit code 0). `down` and `stop` stop dependents
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

## Troubleshooting

**An app keeps restarting, or shows `failed`.**
Run `watchd logs APP`. The lines just before `» exited (...)` usually show the error. `watchd ps`
shows the exit code, and for `failed` apps the reason.

**`command not found` in an app's logs, even though the command works in my terminal.**
Apps get the environment of the shell that ran `watchd up` or `watchd reload`. If you changed
your `PATH` (with nvm, pyenv, a virtualenv and so on) afterwards, run `watchd reload` from the
updated shell.

**`address already in use`.**
Something else is using the port. That might be another project, a process you started by
hand, or apps left over after the daemon was force-killed (`kill -9`). Find it with
`lsof -i :PORT` (or `ss -ltnp | grep PORT`). Shut down with `watchd off` to avoid leftovers.

**An app sits in `waiting`.**
One of its `depends_on` apps isn't ready yet. Check `watchd ps`. A dependency with a health
check must show `(healthy)`, so look at its logs and make sure the health check matches what
the app actually listens on.

**File changes don't trigger a restart.**
Check that the pattern is relative to the app's `cwd`. A pattern without `/` matches file names
at any depth, while `src/*.ts` matches only directly inside `src/`. The file also mustn't be
excluded by `ignore`. `watchd validate` shows the patterns, and `watchd logs APP` shows
`» changed ...` when a change is noticed.

**`too many directories to watch`.**
Linux limits how many directories one user can watch. Add big directories (`node_modules`,
`vendor`, `target`, build output) to `ignore`, or raise the limit:
`sudo sysctl fs.inotify.max_user_watches=524288`.

**`daemon is not running` or `the daemon did not start`.**
Run `watchd info` to see the daemon's state and log location, then read that log. Running
`watchd daemon` in the foreground shows errors directly.

**`socket path ... is too long`.**
Unix socket paths are limited to about 100 characters. Set `WATCHD_RUNTIME_DIR` to a shorter
directory, for example `export WATCHD_RUNTIME_DIR=/tmp/watchd-$USER`.

**`several projects are loaded`.**
You ran a command outside any project directory while more than one project is loaded. Add
`-p NAME`, use `PROJECT/APP`, or `cd` into the project.

**`refusing to load it ... world-writable`.**
Any user on the machine could edit that file. Fix it with `chmod o-w watchd.lua` (or the env
file named in the message).

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
| `cmd/` | Cobra CLI. Turns commands into requests and formats the output. `ui.go` is the Bubble Tea dashboard, and `init.go` holds the Huh prompts. |
| `internal/detect` | Recognises common project types and renders a starter `watchd.lua`. |
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

### Releasing

Releases are fully automated. Every push to `main` runs the tests on Linux and macOS. If they
pass, [`.github/workflows/release.yml`](.github/workflows/release.yml) works out the next
version from the commit messages since the last tag, pushes that tag, and publishes a GitHub
release with binaries, checksums and release notes grouped by type.

The version comes from the commit messages, following
[Conventional Commits](https://www.conventionalcommits.org):

| Commit message | Example | Release |
| --- | --- | --- |
| `feat!:`, `fix!:`, or a `BREAKING CHANGE:` footer | `feat!: rename restart policies` | major (minor while below v1.0.0) |
| `feat:` | `feat(logs): add --since flag` | minor: `v0.2.0` → `v0.3.0` |
| `fix:`, `perf:`, `refactor:` or no prefix | `fix: crash on empty env_file` | patch: `v0.2.0` → `v0.2.1` |
| only `docs:`, `ci:`, `chore:`, `test:`, `style:`, `build:` | `docs: add recipes` | no release |

- The first release is `v0.1.0`.
- Add `[skip release]` to a commit message to push without releasing.
- To release by hand (for example, to force a version bump), go to **Actions → Release →
  Run workflow** and pick `patch`, `minor` or `major`.
- Preview the next version locally with `scripts/next-version.sh`, and test the build without
  publishing with `goreleaser release --snapshot --clean`.

Dependabot opens weekly PRs for Go modules (`fix(deps):`, which ship in a patch release) and
GitHub Actions (`ci:`, which don't release).

## Limitations and roadmap

- Linux and macOS only (it relies on Unix sockets and process groups).
- Logs are kept in memory only. Writing per-app log files with rotation is planned.
- Projects don't survive a daemon restart. Persisting them is planned.
- If the daemon is killed with `SIGKILL`, the apps it was running keep running. Use
  `watchd off` (or `SIGTERM`) for a clean stop.
- Ideas: restart an app when it turns unhealthy, `watchd exec APP -- cmd` to run a command
  with an app's environment, a TUI dashboard, and per-app resource limits.
