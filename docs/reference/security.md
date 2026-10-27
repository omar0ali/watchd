# Security

watchd runs commands as you. It's designed so that **nobody else on the machine can make it
run anything**, and so that a broken config can't take the daemon down.

## No network exposure

The CLI and daemon talk over a **Unix socket** only. watchd never listens on a network
port.

## A private socket

- The socket lives in `$XDG_RUNTIME_DIR/watchd/`, which is usually `/run/user/<uid>/watchd/`.
  When that isn't set, it uses `/tmp/watchd-<uid>/`.
- watchd creates that directory with mode `0700`, and **refuses to use it** if it's a
  symlink or belongs to another user. This prevents a classic attack: another user
  pre-creating the directory in `/tmp` to intercept or impersonate the socket.
- The socket itself is `0600`.

## Callers are checked

On Linux, the daemon reads the user ID of every connecting process (`SO_PEERCRED`) and
rejects connections from other users, even if a misconfiguration made the socket reachable.
On macOS, access is controlled by the directory and socket permissions above.

## Configs anyone can edit are refused

watchd won't load a `watchd.lua` or `env_file` that is **world-writable**, because anyone
who could edit it could run commands as you:

```console
error: /srv/shared/watchd.lua is world-writable; refusing to load it (fix with: chmod o-w "/srv/shared/watchd.lua")
```

## The Lua sandbox

Configs and hooks run in a restricted Lua environment:

- no `io` or `debug` library
- no `os.execute`, `os.exit`, `os.remove`, `os.rename` or `os.tmpname`
- no `dofile` or `loadfile`; `require` only loads from the config's folder
- evaluating a config is limited to **5 seconds**, and each hook call to **2 seconds**

This stops a buggy config or hook from hanging, killing or writing files from inside the
daemon.

::: warning Only use configs you trust
The sandbox guards against **mistakes**, not malicious configs. A config's `cmd` runs shell
commands by design, so treat a `watchd.lua` like a Makefile or `package.json` scripts, and
only run ones you trust.
:::

## Resource limits

- Requests to the daemon are limited to 1 MiB and must arrive within 10 seconds.
- Each log line is capped at 16 KiB, and each app keeps a fixed 2000 lines.
- If a reader (`logs -f`) is too slow, lines are dropped rather than blocking your app.

## Private files

- The daemon log (`~/.local/state/watchd/daemon.log`) is `0600`, in a `0700` directory.
- Environment values are never written to the log, and `ps` and `validate` only show how
  many variables an app has.

## Other safeguards

- **One daemon per user**, enforced with a lock file.
- **No root needed.** If the daemon is started as root, it logs a warning, because every app
  would then run as root.
- **Releases are built in CI.** Release binaries are built by GitHub Actions from tagged
  commits, only after the tests pass, and published with a `checksums.txt` so you can check
  your download wasn't corrupted or altered.

## Reporting a vulnerability

Please report security issues privately through
[GitHub's security advisories](https://github.com/omar0ali/watchd/security/advisories/new)
instead of opening a public issue.
