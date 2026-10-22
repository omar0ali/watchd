package cmd

import (
	"fmt"

	"github.com/omar0ali/watchd/internal/app/client"
	"github.com/spf13/cobra"
)

var offCmd = &cobra.Command{
	Use:   "off",
	Short: "Shutdown running daemon",
	Run: func(cmd *cobra.Command, args []string) {
		if rst, err := client.SendToDaemon("off"); err == nil {
			fmt.Println(rst)
		} else {
			fmt.Println("shutting down - ", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(offCmd)
}
