package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "watchd",
	Short: "`watchd` is a lightweight process and development orchestration tool.",
	Long: `Allows developers to define applications inside a simple configuration file and ensures those applications are automatically started, monitored, and restarted when changes occur or failures happen.

Its designed to work with any type of application regardless of language, framework, or runtime by treating everything as a process managed through a unified daemon.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
