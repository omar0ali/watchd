package models

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

var mu sync.Mutex

type RestartPolicy string

const (
	RestartAlways RestartPolicy = "always"
	RestartNever  RestartPolicy = "never"
)

type AppStatus bool

const (
	Running AppStatus = true
	Stopped AppStatus = false
)

type App struct {
	Name        string
	CMD         string
	CWD         string
	WatchExts   []string
	IgnoreFiles []string
	Restart     RestartPolicy
	OnChange    *lua.LFunction
	Status      AppStatus

	// MANAGE RUNNERS
	running  bool
	stopping bool

	cmd *exec.Cmd

	pid int

	ctx    context.Context
	cancel context.CancelFunc
}

type Project struct {
	Apps []App
	Name string
}

type CurrentProjects struct {
	cp map[string]*Project
}

func New() *CurrentProjects {
	return &CurrentProjects{
		cp: map[string]*Project{},
	}
}

func (cp *CurrentProjects) NewProject(projectName string, project *Project) error {
	_, exists := cp.cp[projectName]
	fmt.Println(projectName, exists)
	if exists {
		return fmt.Errorf("Error: Project name already exist.")
	} else {
		cp.cp[projectName] = project
	}
	return nil
}

func (cp *CurrentProjects) ListProjects() string {
	result := strings.Builder{}
	result.WriteString("Projects \n")
	for projectName := range cp.cp {
		fmt.Fprintf(&result, "Name: %s\n", projectName)
	}
	return result.String()
}

func (cp *CurrentProjects) ListAppsOf(projectName string) string {
	result := strings.Builder{}
	result.WriteString("Apps \n")
	if _, exists := cp.cp[projectName]; exists {
		for _, app := range cp.cp[projectName].Apps {
			fmt.Fprintf(&result, "(Running: %v) %s, %s, %s\n", app.Status, app.Name, app.CMD, app.CWD)
		}
	}
	return result.String()
}

func (cp *CurrentProjects) Start(name string) error {
	mu.Lock()
	defer mu.Unlock()

	for _, app := range cp.cp[name].Apps {
		if app.running {
			return errors.New("already running")
		}

		app.ctx, app.cancel = context.WithCancel(context.Background())

		cmd := exec.Command(app.CMD)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return nil
}
