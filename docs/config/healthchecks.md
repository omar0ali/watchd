# Health checks

A health check tells watchd when an app is actually **ready**, not just started. It matters
in two places:

- **`depends_on`**: dependents wait until the check passes before starting.
- **`watchd ps` and `watchd ui`** show `running (starting)`, `running (healthy)` or
  `running (unhealthy)`.

Without a health check, an app is ready as soon as its process starts.

## Kinds of checks

Set **exactly one** of `port`, `http` or `cmd`.

### TCP port

Passes when a TCP connection to the port succeeds:

```lua
healthcheck = { port = 5432 }
healthcheck = { port = 6379, host = "127.0.0.1" }
```

### HTTP

Passes when a `GET` returns a status code below 400:

```lua
healthcheck = { http = "http://127.0.0.1:8080/health" }
```

### Command

Passes when the command exits with code 0. It runs through `/bin/sh -c` in the app's `cwd`,
with the app's environment:

```lua
healthcheck = { cmd = "pg_isready -h 127.0.0.1 -p 5432" }
healthcheck = { cmd = "redis-cli ping" }
healthcheck = { cmd = "curl -fsS localhost:8080/ready" }
```

## Options

| Field | Default | Description |
| --- | --- | --- |
| `host` | `"127.0.0.1"` | Host for `port` checks. |
| `interval` | `2` | Time between checks. The first check runs about 0.5s after the app starts. |
| `timeout` | `1` | Time allowed for a single check. |
| `retries` | `3` | Consecutive failures before the app is marked `unhealthy`. |

```lua
healthcheck = {
  http = "http://127.0.0.1:8080/health",
  interval = "500ms",
  timeout = 2,
  retries = 5,
}
```

## Lifecycle

1. When the process starts, its health is **starting**.
2. On the first passing check, it becomes **healthy**, and apps waiting on it start.
3. After `retries` failed checks in a row, it becomes **unhealthy**. A passing check makes it
   healthy again.
4. When the process exits or restarts, checking starts over.

::: warning Unhealthy apps are not restarted
An unhealthy status is shown so you can see it. watchd doesn't restart the app because of
it. Apps are restarted when they exit, when their files change, or on request.
:::

## Examples

A database that dependents must wait for:

```lua
{
  name = "postgres",
  cmd = "postgres -D ./.data/pg",
  healthcheck = { cmd = "pg_isready -h 127.0.0.1", interval = 1 },
},
{
  name = "api",
  cmd = "go run ./cmd/api",
  depends_on = { "postgres" },          -- starts once pg_isready succeeds
  healthcheck = { http = "http://127.0.0.1:8080/health" },
},
{
  name = "web",
  cmd = "npm run dev",
  depends_on = { "api" },               -- starts once /health answers
},
```
