-- Example: three services that depend on each other.
--   cd examples/microservices && watchd up -f
--   open http://localhost:3000

local api_port = watchd.env("API_PORT", "8080")

return {
	name = "shop",
	env_file = ".env",

	apps = {
		{
			name = "api",
			cmd = "go run .",
			cwd = "./api",
			env = { PORT = api_port },
			watch = { "*.go" },
			restart = "always",
			-- dependents wait until this passes
			healthcheck = { http = "http://127.0.0.1:" .. api_port .. "/health", interval = "1s" },
			on_start = function(app)
				print("api is up with pid " .. app.pid)
			end,
		},

		{
			name = "web",
			cmd = "go run .",
			cwd = "./web",
			depends_on = { "api" },
			env = { PORT = 3000, API_URL = "http://127.0.0.1:" .. api_port },
			watch = { "*.go", "templates/**/*.html" },
			healthcheck = { port = 3000 },
		},

		{
			name = "worker",
			cmd = "go run .",
			cwd = "./worker",
			depends_on = { "api" },
			watch = { "*.go" },
			restart = "on-failure",
			max_restarts = 5,
			stop_signal = "SIGINT",
			stop_timeout = 5,
			on_change = function(file)
				-- test files don't affect the running worker
				return not file:match("_test%.go$")
			end,
			on_exit = function(app, code)
				if code ~= 0 then
					print("worker exited with code " .. code)
				end
			end,
		},
	},
}
