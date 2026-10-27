---
layout: home

hero:
  name: watchd
  text: docker compose for local processes
  tagline: Run every app of your projects from one Lua config. watchd restarts them when files change or when they crash, starts them in the right order, and lets you control everything from one CLI.
  image:
    src: /logo.svg
    alt: watchd
  actions:
    - theme: brand
      text: Get started
      link: /guide/getting-started
    - theme: alt
      text: Install
      link: /guide/installation
    - theme: alt
      text: View on GitHub
      link: https://github.com/omar0ali/watchd

features:
  - icon: 🔁
    title: Restarts on file changes
    details: Watches your files with gitignore-style patterns and debouncing, so saving 20 files means one restart.
  - icon: 🛟
    title: Supervises processes
    details: Restart policies with exponential backoff and a retry limit. Stops take down the whole process tree, so go run and npm leave no orphans.
  - icon: 🧭
    title: Dependencies and health checks
    details: The web app waits until the API actually answers on its port, at its URL, or passes a check command.
  - icon: 🗂️
    title: Many projects, one daemon
    details: List everything that's running and control any project or app from anywhere, for example watchd stop shop/worker.
  - icon: 🌙
    title: The config is Lua
    details: Variables, loops, conditionals, env lookups, and hooks that decide whether a change should restart an app.
  - icon: 📺
    title: A live dashboard
    details: watchd ui shows every project and app, with keys to restart, stop, start and follow logs.
---

<div class="vp-doc" style="max-width: 860px; margin: 3rem auto 0; padding: 0 24px;">

## Thirty seconds to running

```console
$ cd ~/work/shop
$ watchd up
No watchd.lua found in ~/work/shop or its parents. Let's create one.
  ✓ api  go run .      ./api
  ✓ web  pnpm run dev  ./web
✓ Created ~/work/shop/watchd.lua for project shop with 2 app(s)
shop: started api, web

$ watchd ps
PROJECT  APP  STATE              PID    UPTIME  RESTARTS
shop     api  running (healthy)  41233  8s      0
shop     web  running            41260  7s      0
```

A typical config:

```lua
return {
  name = "shop",
  apps = {
    {
      name = "api",
      cmd = "go run ./cmd/api",
      watch = { "**/*.go" },
      healthcheck = { http = "http://127.0.0.1:8080/health" },
    },
    {
      name = "web",
      cmd = "npm run dev",
      cwd = "./web",
      depends_on = { "api" },
      env = { PORT = 3000 },
    },
  },
}
```

Any language works, because an app is just a shell command. watchd is a single static
binary for Linux and macOS, with no Node.js, Python, Docker or tmux required.

</div>
