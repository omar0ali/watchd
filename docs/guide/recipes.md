# Recipes

Copy-paste setups for common stacks. Each snippet is one entry in the `apps` list of your
`watchd.lua`.

## Go API with a build step

`go run` works, but a separate build step makes compile errors easier to spot and leaves you
a reusable binary:

```lua
{
  name = "api",
  cmd = "go build -o ./tmp/api ./cmd/api && ./tmp/api",
  watch = { "**/*.go", "go.mod" },
  ignore = { "tmp" },
  healthcheck = { http = "http://127.0.0.1:8080/healthz" },
},
```

## Node, Vite or Next.js frontend

Dev servers like Vite and Next.js already reload in the browser when you edit code, so
**don't** watch the source files. Let watchd keep the process alive, and restart it only when
dependencies or config change:

```lua
{
  name = "web",
  cmd = "npm run dev",
  cwd = "./web",
  watch = { "package.json", "vite.config.*" },
  env = { PORT = 3000 },
  healthcheck = { port = 3000 },
},
```

For a plain Node server without its own reloading, watch the sources:

```lua
{
  name = "server",
  cmd = "node server.js",
  watch = { "**/*.js", "package.json" },
  ignore = { "node_modules" },
},
```

## Python: Django, Flask, FastAPI

Turn off the framework's own reloader and let watchd do the restarting, so two reloaders
don't fight:

::: code-group

```lua [Django]
{
  name = "django",
  cmd = "python manage.py runserver --noreload 0.0.0.0:8000",
  cwd = "./backend",
  watch = { "**/*.py" },
  ignore = { ".venv", "__pycache__" },
  env_file = "backend/.env",
  healthcheck = { port = 8000 },
},
```

```lua [FastAPI]
{
  name = "api",
  cmd = "uvicorn app.main:app --port 8000",   -- without --reload
  watch = { "**/*.py" },
  ignore = { ".venv", "__pycache__" },
  healthcheck = { http = "http://127.0.0.1:8000/docs" },
},
```

```lua [Flask]
{
  name = "flask",
  cmd = "flask --app app run --port 5000",    -- without --debug's reloader
  watch = { "**/*.py", "templates/**/*.html" },
  ignore = { ".venv", "__pycache__" },
},
```

:::

Apps inherit the environment of the shell that ran `watchd up`, so activate your virtualenv
first. Alternatively, point at its Python directly: `cmd = ".venv/bin/python manage.py ..."`.

## Rust

```lua
{
  name = "server",
  cmd = "cargo run --bin server",
  watch = { "src/**/*.rs", "Cargo.toml" },
  ignore = { "target" },
  stop_timeout = 5,
},
```

## Local Postgres and Redis, without Docker

Make other apps depend on them so they start only once the database accepts connections:

```lua
{
  name = "postgres",
  cmd = "postgres -D ./.data/pg -p 5432",
  healthcheck = { cmd = "pg_isready -h 127.0.0.1 -p 5432", interval = 1 },
  stop_signal = "SIGINT",          -- Postgres "fast shutdown"
  stop_timeout = 30,
},
{
  name = "redis",
  cmd = "redis-server --port 6379 --save ''",
  healthcheck = { cmd = "redis-cli -p 6379 ping" },
},
{
  name = "api",
  cmd = "go run ./cmd/api",
  depends_on = { "postgres", "redis" },
},
```

Create the Postgres data directory once with `initdb -D ./.data/pg`.

## Using Docker for some services

watchd can supervise `docker run` like any other command. Use `--rm` and a fixed container
name so restarts don't pile up containers:

```lua
{
  name = "db",
  cmd = "docker run --rm --name shop-db -p 5432:5432 -e POSTGRES_PASSWORD=dev postgres:16",
  healthcheck = { port = 5432 },
  stop_timeout = 20,
},
```

## Apps you only need sometimes

`autostart = false` keeps an app out of a plain `watchd up`:

```lua
{ name = "storybook", cmd = "npm run storybook", cwd = "./web", autostart = false },
```

Start it when you need it with `watchd start storybook` (or `watchd up storybook`).

## Scripts that run once

For setup steps such as migrations or code generation, use `restart = "never"`. A clean exit
shows as `exited (exit 0)` in `watchd ps`:

```lua
{
  name = "migrate",
  cmd = "go run ./cmd/migrate up",
  restart = "never",
  depends_on = { "postgres" },
},
```

## Many similar services

The config is Lua, so you can generate repetitive entries with a loop:

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

return { name = "platform", apps = apps }
```

## Personal overrides without editing the shared config

Read values from your environment, with defaults:

```lua
local api_port = watchd.env("API_PORT", "8080")

return {
  apps = {
    { name = "api", cmd = "go run .", env = { PORT = api_port } },
  },
}
```

```sh
API_PORT=9090 watchd up      # or set API_PORT in your shell profile
```

## Skip restarts for some files

Use an `on_change` hook to ignore changes that don't affect the running app:

```lua
{
  name = "api",
  cmd = "go run .",
  watch = { "**/*.go", "**/*.md" },
  on_change = function(file)
    -- tests and docs don't change the running server
    return not (file:match("_test%.go$") or file:match("%.md$"))
  end,
},
```

## Graceful shutdown with a custom signal

Some servers drain connections on `SIGINT` or `SIGQUIT` instead of `SIGTERM`:

```lua
{
  name = "worker",
  cmd = "bundle exec sidekiq",
  stop_signal = "SIGQUIT",
  stop_timeout = 25,       -- allow time to finish in-flight jobs
},
```
