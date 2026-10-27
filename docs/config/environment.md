# Environment

Every app runs with a set of environment variables, built from several layers.

## Precedence

From lowest to highest; later layers win:

1. **The shell that ran `watchd up` or `watchd reload`.** Apps see the same `PATH` (and
   nvm, pyenv, virtualenv, asdf and mise setup) as your terminal.
2. The project's `env_file`, then the project's `env`.
3. The app's `env_file`, then the app's `env`.
4. `WATCHD_PROJECT` and `WATCHD_APP`, set by watchd.

```lua
return {
  env_file = ".env",                       -- 2
  env = { LOG_LEVEL = "debug" },           -- 2

  apps = {
    {
      name = "api",
      cmd = "go run .",
      env_file = "api/.env.local",         -- 3
      env = { PORT = 8080 },               -- 3
    },
  },
}
```

::: tip Changed your shell setup?
The base environment is captured when you run `watchd up` or `watchd reload`. If you change
`PATH` or switch Node or Python versions afterwards, run `watchd reload` from the updated
shell.
:::

## Variables set by watchd

| Variable | Value |
| --- | --- |
| `WATCHD_PROJECT` | The project name |
| `WATCHD_APP` | The app name |

## Dotenv files

`env_file` takes one path or a list. Paths are relative to the config's folder:

```lua
env_file = ".env"
env_file = { ".env", ".env.local" }    -- later files win
```

Supported syntax:

```sh
# comments and blank lines are ignored
PORT=8080
export DATABASE_URL=postgres://localhost/shop   # "export " is optional
GREETING="hello\nworld"                         # double quotes: escapes like \n work
PATTERN='literal $not_expanded'                 # single quotes: taken literally
NAME=value # trailing comments are removed from unquoted values
```

Variables aren't expanded inside dotenv files.

## Using variables in commands

`cmd` runs through `/bin/sh -c`, so the shell expands variables:

```lua
env = { PORT = 3000 },
cmd = "npm run dev -- --port $PORT",
```

## Reading variables in the config

Use [`watchd.env`](/config/lua#watchd-env) to read the environment of the shell that ran
`watchd up`, with a default:

```lua
local port = watchd.env("API_PORT", "8080")
```

## Secrets

- Keep secrets in dotenv files that aren't committed (add them to `.gitignore`).
- watchd never writes environment values to its log, and `ps` and `validate` only show how
  many variables an app has.
- watchd refuses to load env files that are writable by every user on the machine. See
  [Security](/reference/security).
