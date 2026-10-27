# Lua in configs

A watchd config is a real program, written in Lua 5.1. You can use variables, loops,
conditionals and functions to build the config instead of repeating yourself.

::: tip New to Lua?
You only need a few things: `local x = 1` for variables, `{ ... }` for tables (lists and
maps), `..` to join strings, `--` for comments, and `for i, v in ipairs(list) do ... end`
for loops. [Learn Lua in Y minutes](https://learnxinyminutes.com/docs/lua/) covers the rest.
:::

## The watchd helpers

### watchd.env

`watchd.env(name, default)` reads a variable from the environment of the shell that ran
`watchd up` or `watchd reload`. Without a default, it returns `nil` when the variable isn't
set.

```lua
local port = watchd.env("API_PORT", "8080")
local debug = watchd.env("DEBUG") ~= nil
```

`os.getenv` reads the same environment.

### watchd.exists

`watchd.exists(path)` reports whether a file or folder exists. Relative paths are resolved
against the config folder.

```lua
if watchd.exists("web/package.json") then
  table.insert(apps, { name = "web", cmd = "npm run dev", cwd = "web" })
end
```

### watchd.dir and watchd.os

| Value | Example |
| --- | --- |
| `watchd.dir` | `"/home/you/work/shop"`, the config folder |
| `watchd.os` | `"linux"` or `"darwin"` |

```lua
local open = watchd.os == "darwin" and "open" or "xdg-open"
```

## Building configs with code

Generate one app per service:

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

table.insert(apps, { name = "gateway", cmd = "go run ./cmd/gateway", depends_on = services })

return { name = "platform", apps = apps }
```

Use helper functions for shared settings:

```lua
local function go_service(name, port)
  return {
    name = name,
    cmd = "go run ./cmd/" .. name,
    env = { PORT = port },
    watch = { "**/*.go" },
    healthcheck = { port = port },
  }
end

return {
  apps = {
    go_service("api", 8080),
    go_service("auth", 8081),
  },
}
```

## Splitting configs

`require` loads Lua modules from the config's folder:

```lua
-- watchd_common.lua
local M = {}
function M.go_service(name, port)
  return { name = name, cmd = "go run ./cmd/" .. name, env = { PORT = port } }
end
return M
```

```lua
-- watchd.lua
local common = require("watchd_common")   -- loads ./watchd_common.lua

return {
  apps = { common.go_service("api", 8080), common.go_service("auth", 8081) },
}
```

`require("lib.services")` loads `./lib/services.lua`.

## The sandbox

Configs and hooks run in a restricted environment, to protect the long-running daemon from
mistakes.

**Available:** the Lua basics (`pairs`, `ipairs`, `tostring`, `pcall`, `error` and so on),
plus `string`, `table` and `math`, `os.time`, `os.date`, `os.clock`, `os.getenv`, `require`,
and the `watchd` helpers.

**Not available:** `io`, `debug`, `os.execute`, `os.exit`, `os.remove`, `os.rename`,
`os.tmpname`, `dofile` and `loadfile`.

**Time limits:** evaluating the config can take at most 5 seconds, and each hook call at
most 2 seconds.

::: warning The sandbox is a safety net, not a security boundary
A config runs shell commands by design, through `cmd`. Only use configs you trust, as you
would a Makefile or a `package.json` script.
:::

`print()` goes to the daemon log while the config is being evaluated, and to the app's log
inside hooks.
