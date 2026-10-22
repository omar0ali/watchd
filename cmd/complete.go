package cmd

import (
	"github.com/omar0ali/watchd/internal/api"
	"github.com/spf13/cobra"
)

// Shell completion helpers. They ask the daemon, and quietly return nothing
// if it is not running.

func loadedApps() []api.AppStatus {
	var apps []api.AppStatus
	_ = call(&api.Request{Cmd: api.CmdPS, All: true}, func(m api.Message) error {
		apps = append(apps, m.Apps...)
		return nil
	})
	return apps
}

// completeApps offers PROJECT/APP for every loaded app, plus bare APP names
// for the project the command would target from here.
func completeApps(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var out []string
	for _, a := range loadedApps() {
		out = append(out, a.Project+"/"+a.Name)
	}
	if req, err := newRequest(api.CmdPS); err == nil && (req.Project != "" || req.ConfigPath != "") {
		var local []api.AppStatus
		_ = call(req, func(m api.Message) error {
			local = append(local, m.Apps...)
			return nil
		})
		// ps falls back to every project when this one isn't loaded.
		if len(local) > 0 && local[0].Project == local[len(local)-1].Project {
			for _, a := range local {
				out = append(out, a.Name)
			}
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func completeProjects(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var out []string
	_ = call(&api.Request{Cmd: api.CmdList}, func(m api.Message) error {
		for _, p := range m.Projects {
			out = append(out, p.Name)
		}
		return nil
	})
	return out, cobra.ShellCompDirectiveNoFileComp
}

func completeProjectArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeProjects(cmd, args, toComplete)
}
