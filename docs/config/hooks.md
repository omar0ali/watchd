# Hooks

Hooks are Lua functions in your config that run inside the daemon when something happens to
an app.

| Hook | Called | Arguments |
| --- | --- | --- |
| [`on_change`](#on-change) | for each changed file, before restarting | `file`, `app` |
| [`on_start`](#on-start) | after the process starts | `app` |
| [`on_exit`](#on-exit) | when the process exits on its own | `app`, `code` |

The `app` argument is a table:

| Field | Value |
| --- | --- |
| `app.name` | App name |
| `app.project` | Project name |
| `app.cwd` | Absolute working directory |
| `app.pid` | Process ID (in `on_start` and `on_exit`) |

## on_change

Called once per changed file, after the [debounce](/config/watching#debouncing) period and
before the restart. Return `false` to skip the restart. Any other value, including no return
value, lets it happen. If any file in the batch allows a restart, the app restarts.

```lua
on_change = function(file, app)
  -- tests and docs don't affect the running server
  if file:match("_test%.go$") or file:match("%.md$") then
    return false
  end
  print(app.name .. ": rebuilding because " .. file .. " changed")
end,
```

`file` is relative to the app's `cwd`, for example `internal/orders/handler.go`.

## on_start

Called after the process has started:

```lua
on_start = function(app)
  print("started with pid " .. app.pid)
end,
```

## on_exit

Called when the process exits by itself. It isn't called when watchd stops it. `code` is the
exit code:

```lua
on_exit = function(app, code)
  if code ~= 0 then
    print(app.name .. " crashed with exit code " .. code)
  end
end,
```

## Output

`print()` in a hook writes to the app's log. It shows up in `watchd logs` and the
dashboard, marked with `»`:

```console
api | » api: rebuilding because handler.go changed
```

## Limits

Hooks run inside the long-lived daemon, so they're restricted to keep it safe:

- **Time limit:** each call can run for at most **2 seconds**. A hook that runs longer
  (an endless loop, for example) is stopped, and the error is logged.
- **Errors are contained:** a hook that raises an error is reported in the app's log, and
  everything carries on. A failing `on_change` doesn't block the restart.
- **Sandbox:** hooks can't run shell commands, write files or exit the process. See
  [Lua in configs](/config/lua#the-sandbox).

Hooks share the config's Lua state, so they can use variables and functions defined at the
top of the file:

```lua
local restarts = 0

return {
  apps = {
    {
      name = "api",
      cmd = "go run .",
      watch = { "*.go" },
      on_change = function(file)
        restarts = restarts + 1
        print("restart #" .. restarts .. " caused by " .. file)
      end,
    },
  },
}
```
