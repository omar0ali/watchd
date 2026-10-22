package cmd

import (
	"os"

	"github.com/omar0ali/watchd/internal/api"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up [APP...]",
	Short: "Load the project and start its apps",
	Long: `Load the project from watchd.lua (or -c FILE) and start its apps. With no
APP arguments, every app with autostart = true is started; dependencies of
the named apps are always started too.

The daemon is started automatically if needed. Running up again is safe: it
starts whatever is not running yet. Use 'watchd reload' to apply config
changes.`,
	Example: `  watchd up             # start the project in this directory
  watchd up api         # start only api (and what it depends on)
  watchd up -f          # start, then follow the logs
  watchd up -c ~/work/shop/watchd.lua`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := configPath(true)
		if err != nil {
			return err
		}
		if err := ensureDaemon(); err != nil {
			return err
		}
		req := &api.Request{Cmd: api.CmdUp, ConfigPath: path, Apps: args, Env: os.Environ()}
		if err := call(req, printText); err != nil {
			return err
		}
		if follow, _ := cmd.Flags().GetBool("follow"); follow {
			return followLogs(&api.Request{Cmd: api.CmdLogs, ConfigPath: path, Apps: args, Lines: 100, Follow: true})
		}
		return nil
	},
}

var downCmd = &cobra.Command{
	Use:   "down [PROJECT...]",
	Short: "Stop a project's apps and unload it",
	Example: `  watchd down              # the project in this directory
  watchd down shop blog    # projects by name, from anywhere
  watchd down --all        # everything`,
	ValidArgsFunction: completeProjectArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		req, err := newRequest(api.CmdDown)
		if err != nil {
			return err
		}
		req.All = all
		req.Apps = args
		return call(req, printText)
	},
}

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Re-read the config and restart the project with it",
	Long: `Re-read the project's config file and restart its apps with the new
settings. If the config has errors, the running project is left untouched.
Apps that were running are started again, as are newly added apps with
autostart = true.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		req, err := newRequest(api.CmdReload)
		if err != nil {
			return err
		}
		req.Env = os.Environ()
		return call(req, printText)
	},
}

func init() {
	upCmd.Flags().BoolP("follow", "f", false, "follow the project's logs after starting (Ctrl-C detaches)")
	downCmd.Flags().BoolP("all", "a", false, "stop and unload every project")
	rootCmd.AddCommand(upCmd, downCmd, reloadCmd)
}
