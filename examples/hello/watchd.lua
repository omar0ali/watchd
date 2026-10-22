-- The smallest useful config: one shell command, restarted if it exits.
return {
	apps = {
		{ name = "clock", cmd = "while true; do date; sleep 5; done", restart = "always" },
	},
}
