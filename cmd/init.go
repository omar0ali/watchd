package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/omar0ali/watchd/internal/config"
	"github.com/spf13/cobra"
)

const initTemplate = `-- watchd config. Reference: https://github.com/omar0ali/watchd#configuration
-- Check it with: watchd validate      Start it with: watchd up
return {
	name = %q,

	-- Shared by every app. Values from env_file are loaded first.
	-- env_file = ".env",
	env = {
		APP_ENV = "development",
	},

	apps = {
		{
			name = "app",
			cmd = "echo 'replace me with your command' && sleep 3600",
			-- cwd = "./backend",           -- relative to this file
			-- watch = { "**/*.go" },       -- restart when these change
			-- ignore = { "vendor" },
			-- restart = "on-failure",      -- "always" | "on-failure" | "never"
			-- healthcheck = { port = 8080 },
		},

		-- {
		-- 	name = "web",
		-- 	cmd = "npm run dev",
		-- 	cwd = "./web",
		-- 	depends_on = { "app" },      -- starts after app is ready
		-- 	env = { PORT = 3000 },
		-- },
	},
}
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a starter watchd.lua in the current directory",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		path := filepath.Join(cwd, config.DefaultFile)
		flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
		if force {
			flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		}
		f, err := os.OpenFile(path, flags, 0o644)
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%s already exists (use --force to overwrite)", config.DefaultFile)
		}
		if err != nil {
			return err
		}
		defer f.Close()

		name := strings.Map(func(r rune) rune {
			if r == ' ' {
				return '-'
			}
			return r
		}, filepath.Base(cwd))
		if _, err := fmt.Fprintf(f, initTemplate, name); err != nil {
			return err
		}
		fmt.Printf("created %s; edit it, then run: watchd up\n", config.DefaultFile)
		return nil
	},
}

var validateCmd = &cobra.Command{
	Use:     "validate",
	Aliases: []string{"check"},
	Short:   "Check the config file and show what it defines",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := configPath(true)
		if err != nil {
			return err
		}
		p, err := config.Load(path, config.Options{})
		if err != nil {
			return err
		}
		defer p.Close()

		fmt.Printf("%s project %s (%s)\n", color(cGreen, "ok"), color(cBold, p.Name), path)
		for _, name := range p.Order {
			a := p.App(name)
			fmt.Printf("\n  %s\n", color(appColor(a.Name), a.Name))
			fmt.Printf("    cmd:      %s\n", a.Cmd)
			if rel, err := filepath.Rel(p.Dir, a.Cwd); err == nil && rel != "." {
				fmt.Printf("    cwd:      %s\n", rel)
			}
			restart := string(a.Restart)
			if a.MaxRestarts > 0 {
				restart += fmt.Sprintf(" (max %d)", a.MaxRestarts)
			}
			fmt.Printf("    restart:  %s\n", restart)
			if len(a.DependsOn) > 0 {
				fmt.Printf("    needs:    %s\n", strings.Join(a.DependsOn, ", "))
			}
			if len(a.Watch) > 0 {
				fmt.Printf("    watch:    %s\n", strings.Join(a.Watch, " "))
			}
			if a.Health != nil {
				fmt.Printf("    health:   %s every %s\n", a.Health, a.Health.Interval)
			}
			if len(a.Env) > 0 {
				fmt.Printf("    env:      %d variable(s)\n", len(a.Env))
			}
			if !a.Autostart {
				fmt.Printf("    autostart: false\n")
			}
		}
		return nil
	},
}

func init() {
	initCmd.Flags().Bool("force", false, "overwrite an existing watchd.lua")
	rootCmd.AddCommand(initCmd, validateCmd)
}
