package cmd

import (
	"errors"
	"fmt"

	"github.com/omar0ali/watchd/internal/api"
	"github.com/omar0ali/watchd/internal/daemon"
	"github.com/omar0ali/watchd/internal/ipc"
	"github.com/omar0ali/watchd/internal/paths"
	"github.com/omar0ali/watchd/internal/version"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the daemon in the foreground (for systemd, launchd or debugging)",
	Long: `Run the daemon in the foreground, logging to stderr and to the daemon log.

You rarely need this: other commands start the daemon in the background when
needed. Use it to run watchd under a service manager.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		background, _ := cmd.Flags().GetBool("background")
		return daemon.Run(!background)
	},
}

var onCmd = &cobra.Command{
	Use:   "on",
	Short: "Start the daemon in the background",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDaemon(); err != nil {
			return err
		}
		return call(&api.Request{Cmd: api.CmdPing}, printText)
	},
}

var offCmd = &cobra.Command{
	Use:     "off",
	Aliases: []string{"shutdown"},
	Short:   "Stop every project and shut the daemon down",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := call(&api.Request{Cmd: api.CmdShutdown}, printText)
		if errors.Is(err, ipc.ErrNoDaemon) {
			fmt.Println("daemon is not running")
			return nil
		}
		return err
	},
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show daemon status and where watchd keeps its files",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		sock, _ := paths.Socket()
		logPath, _ := paths.DaemonLog()
		fmt.Printf("cli:        watchd %s\n", version.Version)
		fmt.Printf("socket:     %s\n", sock)
		fmt.Printf("daemon log: %s\n", logPath)
		err := call(&api.Request{Cmd: api.CmdPing}, func(m api.Message) error {
			fmt.Printf("daemon:     %s\n", m.Text)
			return nil
		})
		if errors.Is(err, ipc.ErrNoDaemon) {
			fmt.Println("daemon:     not running")
			return nil
		}
		return err
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the watchd version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("watchd", version.Version)
	},
}

func init() {
	daemonCmd.Flags().Bool("background", false, "detached mode used by auto-start (no stderr logging)")
	_ = daemonCmd.Flags().MarkHidden("background")
	rootCmd.AddCommand(daemonCmd, onCmd, offCmd, infoCmd, versionCmd)
}
