package cmd

import (
	"fmt"

	"github.com/omar0ali/watchd/internal/app/client"
	"github.com/spf13/cobra"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "Current running projects",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var result string
		var err error
		if len(args) == 0 {
			result, err = client.SendToDaemon("ps")
			if err != nil {
				fmt.Println("ps error: ", err)
				return
			}
		} else {
			result, err = client.SendToDaemon(fmt.Sprintf("ps %s", args[0]))
			if err != nil {
				fmt.Println("ps error: ", err)
				return
			}
		}
		fmt.Println(result)
	},
}

func init() {
	rootCmd.AddCommand(psCmd)
}
