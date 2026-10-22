package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/omar0ali/watchd/internal/api"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [APP | PROJECT/APP ...]",
	Short: "Show app output",
	Long: `Show recent output of the project's apps (all apps if none are named),
merged in time order. Lines from watchd itself (starts, exits, restarts,
file changes, Lua print) are marked with ».`,
	Example: `  watchd logs            # last 100 lines of every app
  watchd logs -f api     # follow api
  watchd logs shop/web -n 20`,
	ValidArgsFunction: completeApps,
	RunE: func(cmd *cobra.Command, args []string) error {
		req, err := newRequest(api.CmdLogs)
		if err != nil {
			return err
		}
		req.Apps = args
		req.Lines, _ = cmd.Flags().GetInt("lines")
		req.Follow, _ = cmd.Flags().GetBool("follow")
		showTime, _ = cmd.Flags().GetBool("timestamps")
		if req.Follow {
			return followLogs(req)
		}
		return call(req, printLog)
	},
}

var (
	showTime  bool
	nameWidth = 6
)

func printLog(m api.Message) error {
	l := m.Log
	if l == nil {
		return printText(m)
	}
	nameWidth = max(nameWidth, len(l.App))
	var b strings.Builder
	if showTime {
		b.WriteString(color(cDim, l.Time.Format("15:04:05.000")+" "))
	}
	b.WriteString(color(appColor(l.App), fmt.Sprintf("%-*s |", nameWidth, l.App)))
	b.WriteByte(' ')
	if l.Stream == api.StreamSystem {
		b.WriteString(color(cDim, "» "+l.Text))
	} else {
		b.WriteString(l.Text)
	}
	fmt.Println(b.String())
	return nil
}

// followLogs streams logs until the project goes down or Ctrl-C.
func followLogs(req *api.Request) error {
	req.Follow = true
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	go func() {
		<-sigs
		fmt.Fprintln(os.Stderr, color(cDim, "\ndetached; apps keep running (stop them with: watchd down)"))
		os.Exit(0)
	}()
	return call(req, printLog)
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "keep printing new lines")
	logsCmd.Flags().IntP("lines", "n", 100, "number of recent lines to show (0 for all kept lines)")
	logsCmd.Flags().BoolP("timestamps", "t", false, "show timestamps")
	rootCmd.AddCommand(logsCmd)
}
