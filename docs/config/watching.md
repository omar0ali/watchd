# Watching files

Set `watch` on an app to restart it when matching files change:

```lua
{
  name = "api",
  cmd = "go run .",
  watch = { "**/*.go", "go.mod" },
  ignore = { "vendor", "tmp" },
  debounce = "300ms",
}
```

Patterns are relative to the app's [`cwd`](/config/apps#cwd).

## Pattern syntax

Patterns support `*`, `**`, `?`, `[abc]` and `{a,b}`, and follow the same rules as
`.gitignore`:

| Pattern | Matches |
| --- | --- |
| `*.go` | Every `.go` file at **any depth**. A pattern **without** `/` matches the file name. |
| `src/**/*.{ts,tsx}` | `.ts` and `.tsx` files anywhere under `src/`. A pattern **with** `/` matches the path from `cwd`. |
| `src/*.ts` | `.ts` files directly in `src/` only |
| `go.mod` | `go.mod` at any depth |
| `templates/**` | Everything under `templates/` |

`watchd validate` rejects invalid patterns.

## Ignoring files

`ignore` uses the same syntax. When a pattern matches a directory, **everything inside it**
is ignored too, and watchd doesn't watch that directory at all, which keeps large trees cheap:

```lua
ignore = {
  "node_modules",     -- any directory named node_modules, at any depth
  "build/out",        -- only this path
  "*.generated.go",   -- generated files anywhere
}
```

These are always ignored, even without an `ignore` field:

```text
.git  .hg  .svn  *.swp  *.swx  *~  .#*  #*#  4913  .DS_Store
```

These are version control directories, plus temporary files that editors write while
saving.

## Debouncing

Saving a file often produces several events, and tools like `git checkout` or code
generators touch many files at once. watchd collects changes and restarts **once**, after
nothing has changed for `debounce`:

```lua
debounce = "300ms"   -- default
debounce = 1         -- wait one second of quiet
```

## What counts as a change

Creating, writing, renaming or deleting a matching file, or changing its attributes.
Running `touch file.go` therefore forces a restart.

New directories are picked up automatically, including files created inside them.

## Deciding in Lua

For rules patterns can't express, add an [`on_change` hook](/config/hooks#on-change). It's
called for every changed file, and returning `false` skips the restart:

```lua
on_change = function(file)
  return not file:match("_test%.go$")
end,
```

## Restarting a crashed app

A file change restarts the app **whatever its state**: running, waiting out a crash backoff,
exited, or failed after `max_restarts`. Fix the bug, save, and the app comes back.

## Watch limits on Linux

Linux limits how many directories a user can watch (`fs.inotify.max_user_watches`). If a
project is too big, watchd logs:

```text
» file watching disabled: too many directories to watch under /home/you/work/shop; add large directories to `ignore` or raise fs.inotify.max_user_watches
```

Add large directories (`node_modules`, `vendor`, `target`, build output) to `ignore`, or
raise the limit:

```sh
sudo sysctl fs.inotify.max_user_watches=524288
echo fs.inotify.max_user_watches=524288 | sudo tee /etc/sysctl.d/90-inotify.conf   # keep it after reboots
```
