# Introduction

watchd is a **process supervisor for development**. It runs all the pieces of a project
(APIs, frontends, workers, databases you run natively) from one config file, and keeps
them running while you work.

If you've ever had five terminal tabs open, one per service, and had to restart them
by hand whenever something changed or crashed, watchd replaces that with:

```sh
watchd up
```

## What it does

- **Runs every app of a project** with one command, in dependency order.
- **Restarts apps when their files change**, with debouncing and gitignore-style patterns.
- **Restarts apps when they crash**, waiting longer after each crash in a row (backoff),
  with an optional retry limit.
- **Waits for dependencies to be ready**, not just started. A frontend can wait until the
  API answers on its health URL.
- **Stops cleanly**: it signals each app's whole process tree, then forces it to stop after
  a timeout.
- **Collects output** from every app into one log stream you can read or follow.
- **Manages many projects at once** through one background daemon, and lets you control
  any of them from any directory.

## Core concepts

| Term | Meaning |
| --- | --- |
| **App** | One long-running command, such as `go run .` or `npm run dev`. It has a name, a working directory and options such as `watch` and `restart`. |
| **Project** | A group of apps defined in one `watchd.lua` file, usually at the root of a repository. |
| **Daemon** | A background process that runs and supervises the apps of every loaded project. It starts automatically the first time you need it. |
| **CLI** | The `watchd` command. It sends requests to the daemon over a private local socket and prints the results. |

```text
 watchd CLI ──(local socket)──▶ watchd daemon
                                 ├── project "shop"
                                 │    ├── app api      restarted on *.go changes
                                 │    ├── app web      waits for api to be healthy
                                 │    └── app worker
                                 └── project "blog"
                                      └── app site
```

Because the daemon owns the processes, your apps keep running when you close the
terminal, and you can check on them later from anywhere with `watchd ps` or `watchd ui`.

## How it compares

| | watchd | docker compose | pm2 | overmind / foreman |
| --- | --- | --- | --- | --- |
| Runs processes natively | ✅ | ❌ (containers) | ✅ | ✅ |
| Restarts on file changes | ✅ built in | ⚠️ `compose watch` (rebuilds containers) | ✅ | ❌ |
| Dependency order and health checks | ✅ | ✅ | ❌ | ❌ |
| Several projects at once | ✅ | ✅ | ✅ | ❌ |
| Config is a real language | ✅ Lua | YAML | JS / JSON | Procfile |
| Runtime needed | none (one binary) | Docker | Node.js | tmux (overmind) |

::: tip watchd is not a container runtime
watchd runs processes directly on your machine and does **not** isolate them. Use it for
development environments and simple deployments, alongside Docker rather than instead of it.
watchd can even [supervise `docker run` commands](/guide/recipes#using-docker-for-some-services).
:::

## Next steps

- [Install watchd](/guide/installation)
- [Get your first project running](/guide/getting-started)
