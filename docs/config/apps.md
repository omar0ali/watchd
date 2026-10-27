# App fields

Each entry in `apps` describes one long-running command. Only `name` (or a map key) and
`cmd` are required.

## Summary

| Field | Type | Default |
| --- | --- | --- |
| [`name`](#name) | string | map key |
| [`cmd`](#cmd) | string | **required** |
| [`cwd`](#cwd) | string | config folder |
| [`env`](#env-and-env-file) | table | `{}` |
| [`env_file`](#env-and-env-file) | string or list | none |
| [`depends_on`](#depends-on) | list | `{}` |
| [`watch`](#watch-ignore-and-debounce) | string or list | none |
| [`ignore`](#watch-ignore-and-debounce) | string or list | none |
| [`debounce`](#watch-ignore-and-debounce) | duration | `"300ms"` |
| [`restart`](#restart) | string | `"on-failure"` |
| [`max_restarts`](#max-restarts) | number | `0` (unlimited) |
| [`stop_signal`](#stop-signal-and-stop-timeout) | string | `"SIGTERM"` |
| [`stop_timeout`](#stop-signal-and-stop-timeout) | duration | `10` |
| [`healthcheck`](#healthcheck) | table | none |
| [`autostart`](#autostart) | boolean | `true` |
| [`on_change`, `on_start`, `on_exit`](#hooks) | function | none |

## name

The app's name, unique within the project. You use it in commands (`watchd restart api`) and
it prefixes the app's log lines. Letters, digits, `.`, `_` and `-` only.

If `apps` is a [map](/config/#list-or-map-of-apps), the key is the name and `name` can be
left out.

## cmd

The command to run. It runs through `/bin/sh -c`, so shell features work:

```lua
cmd = "go build -o ./tmp/api . && ./tmp/api"
cmd = "npm run dev -- --port $PORT"           -- $PORT comes from the app's environment
cmd = "python -m http.server 8000 | tee server.log"
```

The command runs in its own process group. When watchd stops or restarts the app, it
signals the **whole group**, so child processes (the binary that `go run` compiles, the node
process behind `npm run`) stop too.

## cwd

The working directory, relative to the folder that contains `watchd.lua`. It must exist.
Watch patterns are relative to it too.

```lua
cwd = "./web"
cwd = "~/work/shared-service"
```

## env and env_file

Extra environment variables for this app, on top of the project-wide ones:

```lua
env = { PORT = 3000, DEBUG = true, API_URL = "http://localhost:8080" },
env_file = { ".env", "web/.env.local" },
```

See [Environment](/config/environment) for the order in which they're applied and the
dotenv format.

## depends_on

Apps that must be **ready** before this one starts:

```lua
depends_on = { "postgres", "api" }
```

"Ready" means the dependency is running and, if it has a [health check](#healthcheck), that
the check passed. While it waits, the app shows as `waiting` in `watchd ps`.

Starting an app also starts its dependencies: `watchd up web` starts `api` first if `web`
depends on it. Unknown names and cycles are reported by `watchd validate`.

## watch, ignore and debounce

Restart the app when matching files change:

```lua
watch = { "**/*.go", "go.mod" },
ignore = { "vendor", "tmp" },
debounce = "500ms",
```

- Leave out `watch` to disable file watching for the app.
- `debounce` is how long watchd waits after the **last** change before restarting, so saving
  many files at once causes one restart.

See [Watching files](/config/watching) for the pattern syntax.

## restart

What to do when the process exits on its own:

| Value | Restarts when… |
| --- | --- |
| `"on-failure"` (default) | it exits with a non-zero code or is killed by a signal |
| `"always"` | it exits for any reason, including a clean exit 0 |
| `"never"` | never; the app stays `exited` or `failed` |

Restarts after crashes wait longer each time: 1s, 2s, 4s… up to 30s. See
[How supervision works](/reference/supervision#crash-restarts-and-backoff).

Whatever the policy, a [file change](#watch-ignore-and-debounce) or `watchd restart` brings
back an app that has exited or failed.

## max_restarts

Stop retrying after this many quick crashes in a row. The app then shows as `failed`:

```lua
restart = "on-failure",
max_restarts = 5,
```

`0` (the default) retries forever. A run lasting longer than 10 seconds resets the count.

## stop_signal and stop_timeout

How the app is stopped (on `watchd stop`, `down`, `restart`, or a restart caused by a file
change):

1. watchd sends `stop_signal` to the app's process group.
2. If the app hasn't exited after `stop_timeout`, watchd sends `SIGKILL`.

```lua
stop_signal = "SIGINT",     -- SIGTERM, SIGINT, SIGQUIT, SIGHUP, SIGUSR1, SIGUSR2 or SIGKILL
stop_timeout = 30,          -- seconds, or "500ms", "1m"
```

The `SIG` prefix is optional: `"INT"` works too.

## healthcheck

Defines when the app counts as ready, for `depends_on` and for display in `watchd ps`:

```lua
healthcheck = { port = 8080 }
healthcheck = { http = "http://127.0.0.1:8080/health", interval = "1s" }
healthcheck = { cmd = "pg_isready -h localhost" }
```

See [Health checks](/config/healthchecks).

## autostart

Set to `false` to leave the app out of a plain `watchd up`:

```lua
{ name = "storybook", cmd = "npm run storybook", autostart = false }
```

Start it on demand with `watchd start storybook` or `watchd up storybook`. Apps that other
apps depend on are always started with them.

## Hooks

Lua functions called when the app's files change, when it starts, and when it exits:

```lua
on_change = function(file, app) return not file:match("%.md$") end,
on_start  = function(app) print("started with pid " .. app.pid) end,
on_exit   = function(app, code) print("exited with " .. code) end,
```

See [Hooks](/config/hooks).
