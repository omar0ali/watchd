package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/omar0ali/watchd/internal/api"
	"github.com/spf13/cobra"
)

var psCmd = &cobra.Command{
	Use:   "ps [PROJECT]",
	Short: "List apps and their state",
	Long: `List apps with their state, PID, uptime and restart count.

Shows the project in the current directory if it is loaded, otherwise every
loaded project. Use -a to always show every project.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeProjectArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		asJSON, _ := cmd.Flags().GetBool("json")
		req, err := newRequest(api.CmdPS)
		if err != nil {
			return err
		}
		if len(args) == 1 {
			req.Project = args[0]
		}
		req.All = all

		var apps []api.AppStatus
		if err := call(req, func(m api.Message) error {
			apps = append(apps, m.Apps...)
			return nil
		}); err != nil {
			return err
		}
		if asJSON {
			return printJSON(apps)
		}
		if len(apps) == 0 {
			fmt.Println("no projects are loaded (start one with: watchd up)")
			return nil
		}

		var rows [][]cell
		for _, a := range apps {
			pid, uptime := "-", "-"
			if a.PID != 0 {
				pid = strconv.Itoa(a.PID)
				uptime = humanDuration(time.Since(a.StartedAt))
			}
			note := ""
			if a.State == api.StateFailed {
				note = a.LastError
			}
			rows = append(rows, []cell{
				{a.Project, ""},
				{a.Name, appColor(a.Name)},
				stateCell(a),
				{pid, ""},
				{uptime, ""},
				{strconv.Itoa(a.Restarts), ""},
				{note, cDim},
			})
		}
		printTable([]string{"PROJECT", "APP", "STATE", "PID", "UPTIME", "RESTARTS", ""}, rows)
		return nil
	},
}

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"projects"},
	Short:   "List loaded projects",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		var projects []api.ProjectStatus
		if err := call(&api.Request{Cmd: api.CmdList}, func(m api.Message) error {
			projects = append(projects, m.Projects...)
			return nil
		}); err != nil {
			return err
		}
		if asJSON {
			return printJSON(projects)
		}
		if len(projects) == 0 {
			fmt.Println("no projects are loaded (start one with: watchd up)")
			return nil
		}
		home, _ := os.UserHomeDir()
		var rows [][]cell
		for _, p := range projects {
			running := fmt.Sprintf("%d/%d", p.Running, p.Apps)
			c := cGreen
			if p.Running == 0 {
				c = cDim
			} else if p.Running < p.Apps {
				c = cYellow
			}
			path := p.ConfigPath
			if rel, err := filepath.Rel(home, path); err == nil && home != "" && rel[0] != '.' {
				path = "~/" + rel
			}
			rows = append(rows, []cell{
				{p.Name, cBold},
				{running, c},
				{humanDuration(time.Since(p.LoadedAt)), ""},
				{path, cDim},
			})
		}
		printTable([]string{"PROJECT", "RUNNING", "LOADED", "CONFIG"}, rows)
		return nil
	},
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func init() {
	psCmd.Flags().BoolP("all", "a", false, "show every loaded project")
	psCmd.Flags().Bool("json", false, "print JSON")
	lsCmd.Flags().Bool("json", false, "print JSON")
	rootCmd.AddCommand(psCmd, lsCmd)
}
