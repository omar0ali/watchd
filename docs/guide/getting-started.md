# Getting started

This walkthrough takes a project with a Go API and a Node frontend from nothing to running.
The same steps work for any language.

::: info Before you start
[Install watchd](/guide/installation) and check that `watchd version` works.
:::

## 1. Create a config

Go to the root of your project and run `watchd init`:

```sh
cd ~/work/shop
watchd init
```

watchd looks at the folder and its immediate subfolders for a few common project types,
and pre-fills an app for each one it finds:

| It finds | It adds |
| --- | --- |
| `Procfile` | One app per line. When there's a Procfile, nothing else is detected. |
| `go.mod` and a `main` package | `go run .`, plus one app per `cmd/<name>/main.go` |
| `package.json` with a `dev` or `start` script | `npm run dev`, or `pnpm`, `yarn` or `bun` based on the lockfile |
| `Cargo.toml` | `cargo run` |
| `manage.py` | Django `runserver --noreload`, using `.venv` if there is one |

In a terminal, you confirm the project name and untick any apps you don't want:

```text
┃ Project name
┃ Used to address the project from anywhere: watchd ps -p NAME, NAME/app
┃ > shop

  Apps found
  space to toggle, enter to confirm; you can edit everything in the file later
  > ✓ api  go run .      ./api
    ✓ web  pnpm run dev  ./web

  Create ~/work/shop/watchd.lua?   Create   Cancel
```

The project name defaults to the folder name, cleaned up so it's valid:
`My App (old)` becomes `my-app-old`.

If nothing is recognised, you get a commented template to fill in. Useful flags:

- `watchd init --yes` skips the questions and includes every detected app.
- `watchd init --name shop` sets the project name.
- `watchd init --force` overwrites an existing `watchd.lua`.

::: tip Shortcut
Running `watchd up` in a folder without a config offers this same setup, then asks whether
to start the project. In scripts and CI, where there's no terminal to ask in, it never
creates files on its own.
:::

The config is a normal Lua file. **Commit it** with your code so your whole team gets the
same setup.

## 2. Review the config

Open `watchd.lua` and adjust it. A config for this project might look like this:

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

Only `name` and `cmd` are required for each app. The [configuration reference](/config/)
covers everything else.

## 3. Check it

```sh
watchd validate
```

```console
ok project shop (/home/you/work/shop/watchd.lua)

  api
    cmd:      go run ./cmd/api
    restart:  on-failure
    watch:    **/*.go
    health:   tcp 127.0.0.1:8080 every 2s

  web
    cmd:      npm run dev
    cwd:      web
    restart:  on-failure
    needs:    api
    env:      1 variable(s)
```

If something is wrong, `validate` reports **every** problem at once (misspelled fields,
wrong types, unknown dependencies) without starting anything:

```console
error: /home/you/work/shop/watchd.lua:
  - app "web": unknown field(s) depend_on (known: autostart, cmd, cwd, debounce, depends_on, ...)
  - app "api": restart must be "always", "on-failure" or "never", got "sometimes"
```

## 4. Start it

```sh
watchd up
```

The first time, watchd starts its background daemon automatically. `up` returns right away,
and your apps keep running after you close the terminal.

To watch the output while everything starts, add `-f`. Press Ctrl-C to stop watching; the
apps keep running.

```sh
watchd up -f
```

## 5. See what's running

```console
$ watchd ps
PROJECT  APP  STATE              PID    UPTIME  RESTARTS
shop     api  running (healthy)  41233  12s     0
shop     web  running            41260  11s     0
```

If an app isn't `running`, its logs usually say why:

```sh
watchd logs              # recent output of every app, merged in time order
watchd logs api -f       # follow one app
```

```console
api | » started (pid 41233): go run ./cmd/api
api | 2026/10/22 19:35:02 listening on :8080
api | » healthy (tcp 127.0.0.1:8080)
web | » dependencies ready
web | » started (pid 41260): npm run dev
```

Lines starting with `»` come from watchd itself: starts, exits, restarts and changed files.

For a live view of everything, open the [dashboard](/guide/dashboard):

```sh
watchd ui
```

## 6. Write code

Edit a `.go` file and save. watchd notices, waits for you to stop saving (300ms by default),
and restarts `api`:

```console
api | » restarting: changed internal/orders/handler.go
api | » started (pid 41377): go run ./cmd/api
```

If your change made the app crash, watchd retries with increasing delays. Fix the code and
save again, and it comes straight back.

## 7. Control individual apps

```sh
watchd restart api       # restart one app
watchd stop web          # stop one app (the project stays loaded)
watchd start web         # start it again
```

## 8. Change the config

After editing `watchd.lua`, apply the changes:

```sh
watchd reload
```

If the new config has a mistake, `reload` shows the errors and leaves the running version
alone.

## 9. Stop

```sh
watchd down              # stop this project's apps and unload it
watchd off               # stop every project and shut the daemon down
```

You can run every command from the project directory or any subdirectory. To control a
project from somewhere else, see [Working with many projects](/guide/projects).

## Try the examples

The repository includes two runnable examples:

- [`examples/microservices`](https://github.com/omar0ali/watchd/tree/main/examples/microservices):
  three Go services with health checks, dependencies and hooks.
- [`examples/hello`](https://github.com/omar0ali/watchd/tree/main/examples/hello): the
  smallest possible config.

```sh
git clone https://github.com/omar0ali/watchd.git
cd watchd/examples/microservices
watchd up -f            # then open http://localhost:3000
```
