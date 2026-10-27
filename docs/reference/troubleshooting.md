# Troubleshooting

## An app keeps restarting, or shows `failed`

Look at its logs. The lines just before `» exited (...)` usually show the error:

```sh
watchd logs api -n 50
```

`watchd ps` shows the exit code, and for `failed` apps the reason. Common causes:

- a compile error after an edit (fix it and save, and the app restarts by itself)
- a missing dependency or environment variable
- the port is already in use (see [below](#address-already-in-use))

## `command not found` in an app's logs

The command works in your terminal, but not under watchd.

Apps get the environment of the shell that ran `watchd up` or `watchd reload`. If you changed
your `PATH` afterwards (with nvm, pyenv, asdf, mise or a virtualenv, for example), run
`watchd reload` from the updated shell. You can also use full paths in `cmd`, such as
`.venv/bin/python`.

## `address already in use`

Something else is using the port. That might be:

- another loaded project (check `watchd ps -a`)
- a process you started by hand
- apps left over after the daemon was force-killed with `kill -9`

Find out what's using it:

```sh
lsof -i :8080             # or: ss -ltnp | grep 8080
```

Shut down with `watchd off` to avoid leftovers.

## An app sits in `waiting`

One of its [`depends_on`](/config/apps#depends-on) apps isn't ready. Check `watchd ps`:

- a dependency with a health check must show `(healthy)`. Look at its logs and make sure
  the check matches what the app really listens on: port, host and path.
- a dependency that's `stopped` or `failed` never becomes ready. Start it or fix it.
  (A dependency that `exited` with code 0, such as a finished migration, counts as ready.)

## File changes don't trigger a restart

- Patterns are relative to the app's `cwd`, not the project root.
- A pattern without `/` matches file names at any depth, but `src/*.ts` matches only files
  directly in `src/`. See [pattern syntax](/config/watching#pattern-syntax).
- Make sure the file isn't excluded by `ignore` or the [default ignores](/config/watching#ignoring-files).
- Apps stopped with `watchd stop` ignore changes.
- `watchd validate` shows the configured patterns, and `watchd logs APP` shows
  `» restarting: changed ...` when a change is noticed.

## `too many directories to watch`

Linux limits how many directories one user can watch. Add big directories
(`node_modules`, `vendor`, `target`, build output) to `ignore`, or raise the limit:

```sh
sudo sysctl fs.inotify.max_user_watches=524288
```

See [Watch limits on Linux](/config/watching#watch-limits-on-linux).

## `daemon is not running` or `the daemon did not start`

```sh
watchd info                # shows the daemon's state and log location
```

Read the daemon log it points to. To see errors directly, run the daemon in the foreground:

```sh
watchd daemon
```

## `the watchd daemon is already running`

A daemon is already running for your user; only one can run at a time. Use it, or stop it
with `watchd off`.

## `socket path ... is too long`

Unix socket paths are limited to about 100 characters. Point watchd to a shorter
directory:

```sh
export WATCHD_RUNTIME_DIR=/tmp/watchd-$USER
```

## `several projects are loaded`

You ran a command outside any project directory while more than one project is loaded. Add
`-p NAME`, use `PROJECT/APP`, or `cd` into the project. See
[Working with many projects](/guide/projects#how-watchd-decides-which-project-you-mean).

## `a project named "x" is already loaded from ...`

Two configs use the same project name. Give one of them a different
`name = "..."`, or unload the other one with `watchd down x`.

## `refusing to load it ... world-writable`

Any user on the machine could edit that file. Fix it with:

```sh
chmod o-w watchd.lua       # or the env file named in the message
```

## `unknown field(s) ...`

There's a typo in the config: watchd rejects fields it doesn't know, rather than silently
ignoring them. The message lists the known fields. See [App fields](/config/apps).

## Still stuck?

[Open an issue](https://github.com/omar0ali/watchd/issues) with the output of
`watchd version`, `watchd validate`, and the relevant `watchd logs`.
