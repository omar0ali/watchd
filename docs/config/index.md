# Configuration overview

A watchd project is described by a Lua file named **`watchd.lua`**, usually at the root of
your repository. The file **returns a table** describing the project and its apps:

```lua
return {
  name = "shop",                 -- optional; defaults to the folder name
  env_file = ".env",             -- optional; shared by every app
  env = { APP_ENV = "dev" },     -- optional; shared by every app

  apps = {
    { name = "api", cmd = "go run ./cmd/api", watch = { "**/*.go" } },
    { name = "web", cmd = "npm run dev", cwd = "./web", depends_on = { "api" } },
  },
}
```

Create one with [`watchd init`](/guide/getting-started#_1-create-a-config), and check it
with `watchd validate`.

## Where watchd looks for the config

Commands look for `watchd.lua` in the **current directory, then each parent directory**, the
same way git looks for `.git`. So you can run `watchd restart api` from deep inside your
project.

To use a different file, pass `-c`:

```sh
watchd up -c ~/work/shop/dev.watchd.lua
```

### Paths are remembered, not your terminal's location

When a project is loaded, watchd records the **full, real path** of its config file (with
symlinks resolved). Everything else is worked out from there: `cwd` and `env_file` are
relative to the config's folder, and every app runs in its own `cwd`, whichever directory
you ran `watchd up` from. The daemon itself doesn't depend on any terminal's location.

So all of these refer to the same loaded project:

```sh
cd ~/work/shop && watchd restart api            # the project's folder
cd ~/work/shop/web/src && watchd restart api    # any folder inside it
cd /mnt/data/work/shop && watchd restart api    # the same folder through its real path, if ~/work is a symlink
watchd restart shop/api                         # anywhere, by name
```

## Project fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | config folder name | Project name. Letters, digits, `.`, `_` and `-` only. See [project names](/guide/projects#project-names). |
| `apps` | list or map | **required** | The apps. See [below](#list-or-map-of-apps) and [App fields](/config/apps). |
| `env` | table | `{}` | Environment variables for every app. See [Environment](/config/environment). |
| `env_file` | string or list | none | Dotenv file(s) loaded before `env`. |

### List or map of apps

`apps` can be a list, where each app has a `name`:

```lua
apps = {
  { name = "api", cmd = "go run ." },
  { name = "web", cmd = "npm run dev" },
}
```

Or a map keyed by name:

```lua
apps = {
  api = { cmd = "go run ." },
  web = { cmd = "npm run dev" },
}
```

Lists keep their order. Maps are sorted by name. Either way, apps start in
[dependency order](/reference/supervision#start-and-stop-order).

## Value formats

### Durations

Fields such as `stop_timeout`, `debounce` and health check `interval` take either:

- a **number of seconds**: `5`, `0.5`
- a **duration string**: `"300ms"`, `"2s"`, `"1m30s"`

### Paths

Relative paths (in `cwd` and `env_file`, for example) are resolved against the **folder
that contains `watchd.lua`**, not your current directory. A leading `~` expands to your home
directory.

### Lists

Fields that take a list also accept a single string: `watch = "*.go"` is the same as
`watch = { "*.go" }`.

### Environment values

Values in `env` can be strings, numbers or booleans. They're converted to strings:
`PORT = 3000` becomes `"3000"`.

## Validation

watchd checks the whole config before starting anything, and reports **every** problem at
once:

```console
$ watchd validate
error: /home/you/work/shop/watchd.lua:
  - app "web": unknown field(s) depend_on (known: autostart, cmd, cwd, debounce, depends_on, ...)
  - app "api": restart must be "always", "on-failure" or "never", got "sometimes"
  - app "worker": cwd /home/you/work/shop/wroker is not a directory
```

It catches:

- **misspelled or unknown fields**, which would otherwise be silently ignored
- wrong types, such as `cmd = 5` or `watch = true`
- invalid values: restart policies, signals, durations, glob patterns, names
- missing required fields (`cmd`, `apps`)
- `cwd` directories that don't exist, and unreadable `env_file`s
- `depends_on` naming an unknown app, and dependency cycles (`a -> b -> a`)
- duplicate app names

`watchd up` and `watchd reload` run the same checks. A `reload` with errors leaves the
running project untouched.

## Full example

```lua
local api_port = watchd.env("API_PORT", "8080")

return {
  name = "shop",
  env_file = ".env",
  env = { APP_ENV = "development" },

  apps = {
    {
      name = "postgres",
      cmd = "postgres -D ./.data/pg",
      healthcheck = { cmd = "pg_isready -h 127.0.0.1", interval = 1 },
      stop_signal = "SIGINT",
    },
    {
      name = "api",
      cmd = "go run ./cmd/api",
      env = { PORT = api_port },
      depends_on = { "postgres" },
      watch = { "**/*.go", "go.mod" },
      restart = "always",
      healthcheck = { http = "http://127.0.0.1:" .. api_port .. "/health" },
    },
    {
      name = "web",
      cmd = "pnpm run dev",
      cwd = "./web",
      depends_on = { "api" },
      env = { VITE_API_URL = "http://localhost:" .. api_port },
      watch = { "package.json" },
    },
    {
      name = "worker",
      cmd = "go run ./cmd/worker",
      depends_on = { "api" },
      watch = { "**/*.go" },
      max_restarts = 5,
      on_change = function(file)
        return not file:match("_test%.go$")
      end,
    },
  },
}
```

## Next

- [App fields](/config/apps): every option for an app
- [Health checks](/config/healthchecks)
- [Watching files](/config/watching)
- [Environment](/config/environment)
- [Hooks](/config/hooks)
- [Lua in configs](/config/lua)
