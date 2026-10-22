package cmd

import (
	"github.com/omar0ali/watchd/internal/app/client/core"
	"github.com/spf13/cobra"
)

var onCmd = &cobra.Command{
	Use:   "on",
	Short: "Start daemon",
	Run: func(cmd *cobra.Command, args []string) {
		core.Load(core.DEV)
	},
}

func init() {
	rootCmd.AddCommand(onCmd)
}
