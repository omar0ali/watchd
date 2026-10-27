// Package cmd implements the watchd command line.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/omar0ali/watchd/internal/api"
	"github.com/omar0ali/watchd/internal/config"
	"github.com/omar0ali/watchd/internal/ipc"
	"github.com/omar0ali/watchd/internal/paths"
	"github.com/spf13/cobra"
)

var (
	flagProject string
	flagConfig  string
)

var rootCmd = &cobra.Command{
	Use:   "watchd",
	Short: "Run, watch and supervise all the processes of your projects",
	Long: `watchd runs the apps of a project (APIs, frontends, workers, ...) from a
single Lua config, restarts them when they crash or when their files change,
and lets you control every project on your machine from one CLI.

Get started:
  watchd init      create a watchd.lua in this directory
  watchd up        start the project
  watchd ps        see what is running
  watchd logs -f   follow the output

Commands act on the project in the current directory (watchd.lua here or in a
parent), or on the project chosen with -p NAME. Apps can also be addressed as
PROJECT/APP from anywhere.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagProject, "project", "p", "", "act on the loaded project with this name")
	rootCmd.PersistentFlags().StringVarP(&flagConfig, "config", "c", "", "path to the config file (default: watchd.lua here or in a parent directory)")
	rootCmd.RegisterFlagCompletionFunc("project", completeProjects)
}

// configPath returns the config file to use. If required is false, a
// missing config is not an error and "" is returned.
func configPath(required bool) (string, error) {
	if flagConfig != "" {
		return config.Canonical(flagConfig)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	p, err := config.Find(cwd)
	if err != nil && required {
		return "", err
	}
	return p, nil
}

// newRequest builds a request targeting -p, or else the config found from
// the current directory.
func newRequest(cmd string) (*api.Request, error) {
	req := &api.Request{Cmd: cmd, Project: flagProject}
	if req.Project == "" {
		p, err := configPath(flagConfig != "")
		if err != nil {
			return nil, err
		}
		req.ConfigPath = p
	}
	return req, nil
}

func call(req *api.Request, fn func(api.Message) error) error {
	sock, err := paths.Socket()
	if err != nil {
		return err
	}
	return ipc.Do(sock, req, fn)
}

func printText(m api.Message) error {
	if m.Text != "" {
		fmt.Println(m.Text)
	}
	return nil
}

// ensureDaemon starts the daemon in the background if it is not running.
func ensureDaemon() error {
	if err := call(&api.Request{Cmd: api.CmdPing}, nil); err == nil {
		return nil
	} else if !errors.Is(err, ipc.ErrNoDaemon) {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	c := exec.Command(exe, "daemon", "--background")
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // survive the terminal closing
	if err := c.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	_ = c.Process.Release()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		if err := call(&api.Request{Cmd: api.CmdPing}, nil); err == nil {
			return nil
		}
	}
	logPath, _ := paths.DaemonLog()
	return fmt.Errorf("the daemon did not start; see %s", logPath)
}
