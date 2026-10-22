package cmd

import (
	"fmt"
	"strings"

	"github.com/omar0ali/watchd/internal/app/client"
	"github.com/omar0ali/watchd/internal/app/utils"
	"github.com/spf13/cobra"
)

var loadCmd = &cobra.Command{
	Use:   "load [file_name]",
	Short: "Environment *.lua file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fileName := args[0]
		cwd, err := utils.GetCWD()
		if err != nil {
			panic(err)
		}
		filePath := strings.TrimSpace(cwd + "/" + fileName)
		rst, err := client.SendToDaemon(fmt.Sprintf("load %s", filePath))
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(rst)
	},
}

func init() {
	rootCmd.AddCommand(loadCmd)
}
