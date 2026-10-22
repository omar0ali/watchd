package cmd

import (
	"github.com/omar0ali/watchd/internal/api"
	"github.com/spf13/cobra"
)

func appCommand(use, short, long, apiCmd string) *cobra.Command {
	return &cobra.Command{
		Use:               use + " [APP | PROJECT/APP ...]",
		Short:             short,
		Long:              long,
		ValidArgsFunction: completeApps,
		RunE: func(cmd *cobra.Command, args []string) error {
			req, err := newRequest(apiCmd)
			if err != nil {
				return err
			}
			req.Apps = args
			return call(req, printText)
		},
	}
}

func init() {
	rootCmd.AddCommand(
		appCommand("start", "Start apps of a loaded project",
			"Start the named apps (and their dependencies). With no arguments, start\nevery app with autostart = true.",
			api.CmdStart),
		appCommand("stop", "Stop apps without unloading the project",
			"Stop the named apps. With no arguments, stop every app of the project but\nkeep it loaded (use 'watchd down' to unload it).",
			api.CmdStop),
		appCommand("restart", "Restart apps",
			"Restart the named apps, starting them if they are stopped. With no\narguments, restart every running app of the project.",
			api.CmdRestart),
	)
}
