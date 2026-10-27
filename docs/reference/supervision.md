# How supervision works

This page explains what watchd does with your processes: when it starts, restarts and stops
them.

## States

`watchd ps` and the dashboard show one of these states for each app:

| State | Meaning |
| --- | --- |
| `waiting` | Started, waiting for its [`depends_on`](/config/apps#depends-on) apps to become ready. |
| `running` | The process is running. If the app has a [health check](/config/healthchecks), its health is shown in parentheses: `starting`, `healthy` or `unhealthy`. |
| `backoff` | Crashed; a restart is scheduled after a delay. |
| `exited` | Exited cleanly and its `restart` policy says not to restart. |
| `failed` | Couldn't start, crashed with `restart = "never"`, or gave up after `max_restarts`. |
| `stopped` | Not supervised: never started, or stopped with `watchd stop`. |

```text
             watchd up / start
 stopped ───────────────────────▶ waiting ──(deps ready)──▶ running
    ▲                                                          │
    │ watchd stop / down                         process exits │
    │                                                          ▼
    └──────────────── any state ◀──── restart policy ──▶ backoff ──(delay)──▶ running
                                           │
                                           └─(no restart)──▶ exited / failed
```

## Starting

When an app starts, watchd:

1. waits for its dependencies to be [ready](#start-and-stop-order)
2. builds its [environment](/config/environment)
3. runs `cmd` with `/bin/sh -c` in its `cwd`, **in a new process group**
4. records the PID and start time, captures stdout and stderr into the app's log, and calls
   [`on_start`](/config/hooks#on-start)
5. starts the [health check](/config/healthchecks), if there is one

## Crash restarts and backoff

When a process exits on its own, the app's [`restart`](/config/apps#restart) policy
decides what happens:

| Policy | Exit code 0 | Non-zero exit or killed by a signal |
| --- | --- | --- |
| `on-failure` (default) | `exited` | restart |
| `always` | restart | restart |
| `never` | `exited` | `failed` |

Restarts after crashes are **delayed**, and the delay doubles with each crash in a row:
1s, 2s, 4s, 8s, 16s, then 30s from then on. That way an app that crashes on startup doesn't
use all your CPU.

A run that lasts **longer than 10 seconds** counts as stable and resets the delay. If
[`max_restarts`](/config/apps#max-restarts) is set, watchd gives up after that many quick
crashes in a row and marks the app `failed`:

```console
worker | » exited (exit code 1)
worker | » restarting in 4s (restart = on-failure)
worker | » started (pid 41391): go run ./cmd/worker
worker | » exited (exit code 1)
worker | » giving up after 5 restarts in a row (max_restarts = 5)
```

## Restarts on file changes and on request

A [file change](/config/watching) or `watchd restart` restarts the app **immediately**,
whatever state it's in: running, in backoff, exited or failed. These restarts don't count as
crashes and reset the backoff.

So when a bug makes your app crash, you don't need to do anything in watchd. Fix the code and
save, and the app comes back.

Apps stopped with `watchd stop` stay stopped and ignore file changes until you start them.

## Stopping

To stop an app (on `stop`, `down`, `off`, or before a restart), watchd:

1. sends [`stop_signal`](/config/apps#stop-signal-and-stop-timeout) (default `SIGTERM`) to
   the app's **whole process group**
2. waits up to `stop_timeout` (default 10s) for it to exit
3. sends `SIGKILL` to the group if it's still running

Signalling the group is what makes `go run`, `npm run`, `cargo run` and shell pipelines stop
completely. They start child processes that would otherwise keep running, and keep holding
your ports.

After an app exits on its own, watchd also kills anything left in its process group.

## Start and stop order

- `up` and `start` start dependencies before the apps that need them.
- An app waits until all of its dependencies are **ready**: running (and healthy, if they have
  a health check), or finished successfully with exit code 0. While waiting, its state is
  `waiting`.
- `down`, `stop` and `off` stop apps in reverse order: dependents before their dependencies.
- Apps without dependencies on each other start in parallel.

## Logs

- watchd captures each app's stdout and stderr line by line.
- It keeps the **last 2000 lines per app** in memory. Lines longer than 16 KiB are split.
- watchd adds its own lines, marked `»`: starts, exits, restarts, changed files, health
  changes and `print()` output from hooks.
- Logs are lost when the project is unloaded or the daemon stops.

## The daemon

- One daemon runs per user. It starts on demand, keeps running in the background, and
  survives closing the terminal.
- It listens on a private Unix socket (see [Security](/reference/security)).
- It writes its own log to `~/.local/state/watchd/daemon.log`.
- On `SIGTERM`, `SIGINT` or `SIGHUP`, and on `watchd off`, it stops every project cleanly
  before exiting.

::: warning If the daemon is killed with SIGKILL
A daemon killed with `kill -9` can't clean up, so the apps it was running keep running. Find
leftovers with `lsof -i :PORT` or `ps`. Use `watchd off` (or plain `kill`) to stop it.
:::

Projects aren't remembered across daemon restarts yet. Run `watchd up` again in each project.
