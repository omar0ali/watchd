package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/omar0ali/watchd/internal/config"
	"github.com/omar0ali/watchd/internal/detect"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a watchd.lua for the project in the current directory",
	Long: `Create a watchd.lua in the current directory.

watchd looks at this directory and its subdirectories for a few common project
types and pre-fills the apps it finds:

  Procfile                       one app per line
  go.mod + package main          go run . (also cmd/<name>/main.go)
  package.json with dev/start    npm/pnpm/yarn/bun run dev
  Cargo.toml                     cargo run
  manage.py                      Django runserver

Anything else gets a commented template to fill in. In a terminal you can
rename the project and pick which apps to keep; use --yes to skip the
questions.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		yes, _ := cmd.Flags().GetBool("yes")
		name, _ := cmd.Flags().GetString("name")
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		_, err = createConfig(cwd, createOptions{
			interactive: interactive() && !yes,
			force:       force,
			name:        name,
		})
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Println("cancelled; nothing was written")
			return nil
		}
		return err
	},
}

type createOptions struct {
	interactive bool
	force       bool
	name        string
	intro       string // shown above the questions
}

type created struct {
	path string
	apps []detect.App
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	dimStyle   = lipgloss.NewStyle().Faint(true)
	cmdStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
)

// interactive reports whether we can ask questions: both stdin and stdout
// are terminals.
func interactive() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout} {
		fi, err := f.Stat()
		if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
			return false
		}
	}
	return true
}

// createConfig detects the project's apps, optionally lets the user adjust
// the result, and writes dir/watchd.lua.
func createConfig(dir string, opts createOptions) (*created, error) {
	path := filepath.Join(dir, config.DefaultFile)
	if _, err := os.Stat(path); err == nil && !opts.force {
		return nil, fmt.Errorf("%s already exists (use --force to overwrite)", config.DefaultFile)
	}

	apps := detect.Detect(dir)
	name := opts.name
	if name == "" {
		name = config.SanitizeName(filepath.Base(dir))
	}
	if !config.ValidName(name) {
		return nil, fmt.Errorf("invalid project name %q: use letters, digits, '.', '_' or '-'", name)
	}

	if opts.interactive {
		var err error
		if name, apps, err = askConfig(path, name, apps, opts.intro); err != nil {
			return nil, err
		}
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if opts.force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("%s already exists (use --force to overwrite)", config.DefaultFile)
	}
	if err != nil {
		return nil, err
	}
	if _, err := f.WriteString(detect.Render(name, apps)); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}

	printCreated(path, name, apps)
	return &created{path: path, apps: apps}, nil
}

func askConfig(path, name string, apps []detect.App, intro string) (string, []detect.App, error) {
	var fields []huh.Field
	if intro != "" {
		fields = append(fields, huh.NewNote().Title(intro))
	}
	fields = append(fields, huh.NewInput().
		Title("Project name").
		Description("Used to address the project from anywhere: watchd ps -p NAME, NAME/app").
		Value(&name).
		Validate(func(s string) error {
			if !config.ValidName(s) {
				return errors.New("use letters, digits, '.', '_' or '-'")
			}
			return nil
		}))

	var selected []string
	if len(apps) > 0 {
		width := 0
		for _, a := range apps {
			width = max(width, len(a.Name))
		}
		var options []huh.Option[string]
		for _, a := range apps {
			label := fmt.Sprintf("%-*s  %s", width, a.Name, a.Cmd)
			if a.Cwd != "" {
				label += dimStyle.Render("  ./" + a.Cwd)
			}
			options = append(options, huh.NewOption(label, a.Name).Selected(true))
		}
		fields = append(fields, huh.NewMultiSelect[string]().
			Title("Apps found").
			Description("space to toggle, enter to confirm; you can edit everything in the file later").
			Options(options...).
			Value(&selected))
	} else {
		fields = append(fields, huh.NewNote().
			Title("No known project type found").
			Description("You'll get a commented template; fill in the commands of your apps."))
	}

	confirm := true
	fields = append(fields, huh.NewConfirm().
		Title("Create "+tildePath(path)+"?").
		Affirmative("Create").
		Negative("Cancel").
		Value(&confirm))

	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return "", nil, err
	}
	if !confirm {
		return "", nil, huh.ErrUserAborted
	}

	if len(apps) > 0 {
		keep := map[string]bool{}
		for _, s := range selected {
			keep[s] = true
		}
		var picked []detect.App
		for _, a := range apps {
			if keep[a.Name] {
				picked = append(picked, a)
			}
		}
		apps = picked
	}
	return name, apps, nil
}

func printCreated(path, name string, apps []detect.App) {
	if len(apps) == 0 {
		fmt.Printf("%s Created %s for project %s from the starter template.\n",
			okStyle.Render("✓"), tildePath(path), titleStyle.Render(name))
		fmt.Println(dimStyle.Render("  Fill in your apps' commands, then run: watchd up"))
		return
	}
	fmt.Printf("%s Created %s for project %s with %d app(s):\n",
		okStyle.Render("✓"), tildePath(path), titleStyle.Render(name), len(apps))
	width := 0
	for _, a := range apps {
		width = max(width, len(a.Name))
	}
	for _, a := range apps {
		where := ""
		if a.Cwd != "" {
			where = dimStyle.Render("  ./" + a.Cwd)
		}
		fmt.Printf("  %s  %s%s\n", titleStyle.Render(fmt.Sprintf("%-*s", width, a.Name)), cmdStyle.Render(a.Cmd), where)
	}
}

func tildePath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
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
	initCmd.Flags().BoolP("yes", "y", false, "don't ask questions; include every detected app")
	initCmd.Flags().String("name", "", "project name (default: the directory name)")
	rootCmd.AddCommand(initCmd, validateCmd)
}
