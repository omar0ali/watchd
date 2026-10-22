# Watchd
`watchd` is a lightweight process and development orchestration tool.

Allows developers to define applications inside a simple configuration file and ensures those applications are automatically started, monitored, and restarted when changes occur or failures happen.

Its designed to work with any type of application regardless of language, framework, or runtime by treating everything as a process managed through a unified daemon.

## Objective

The goal of `watchd` is to simplify how developers run and manage multiple applications during development and in lightweight production environments.

Instead of manually running and restarting processes, `watchd` provides a single daemon that:

- Runs multiple applications concurrently
- Watches file changes in real time
- Restarts applications automatically based on defined rules
- Exposes a simple CLI for control and inspection

Configuration is written in Lua, allowing developers to define flexible behaviors, custom hooks, and per-app rules without modifying the core system.

## How it works
`watchd` is built around three core components:

### 1. CLI (stateless)
The CLI acts as a control interface similar to tools like Docker or systemctl.

It sends commands to the daemon such as:

- Start project
- Stop application
- List running processes
- View status

### 2. Daemon (stateful core)
It is responsible for:

- Running all applications
- Managing process lifecycles
- Watching file changes
- Applying restart policies
- Executing Lua-defined behavior

It exposes a local socket API that the CLI communicates with.

### 3. Lua configuration layer
Each project defines a configuration file that describes:

- Applications to run
- Commands to execute
- Directories to watch
- Restart behavior
- Optional hooks (e.g. on_change)

Makes `watchd` flexible without requiring recompilation or code changes.

## Example Configuration
```lua
return {
	name = "myapp",
	apps = {
		{
			name = "backend",
			cmd = "go run ./backend/cmd/web/main.go",
			cwd = "./backend",
			watch = { "**/*.go" },
			ignore = { ".git" },
			restart = "always",
			on_change = function(file)
				print("backend file changed: " .. file)
			end,
		},

		{
			name = "frontend",
			cmd = "npm run dev",
			cwd = "./frontend",
			watch = { "src/**/*.{js,ts,jsx,tsx}" },
			ignore = { "node_modules", "dist", ".git" },
			restart = "always",
			on_change = function(file)
				print("frontend file changed: " .. file)
			end,
		},
	},
}
```

## CLI Usage Example
```
watchd start
watchd run
watchd ps
watchd status app_name
watchd stop app_name
watchd shutdown
```
