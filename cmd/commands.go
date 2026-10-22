package cmd

import (
	"errors"
	"fmt"

	"github.com/omar0ali/watchd/internal/app/daemon"
	"github.com/omar0ali/watchd/internal/app/daemon/core/models"
	"github.com/omar0ali/watchd/internal/app/daemon/core/parser"
)

type WatchDaemon struct {
	CurrProject *models.CurrentProjects
	Daemon      *daemon.Daemon
}

var WD *WatchDaemon

func Load(fileName string) (string, error) {

	// check wd
	if WD == nil {
		return "", errors.New("Daemon isn't running")
	}

	// parse the file here
	fileParser, err := parser.New(fileName)
	if err != nil {
		fmt.Println(err)
	}
	// parsing
	project, err := fileParser.Start()
	if err != nil {
		return "", err
	}
	// close parser
	fileParser.Close()

	if err := WD.CurrProject.NewProject(project.Name, project); err != nil {
		return "", err
	}

	return fmt.Sprintf(
			"Project %s - Apps: %d\n",
			project.Name,
			len(project.Apps)),
		nil
}

func PS(args []string) (string, error) {
	if len(args) > 1 {
		appName := args[1]
		return WD.CurrProject.ListAppsOf(appName), nil
	}
	return WD.CurrProject.ListProjects(), nil

}

// TODO:
func Start() (string, error) {
	return "", nil
}

// TODO:
func Stop() (string, error) {
	return "", nil
}

func Off() (string, error) {
	WD.Daemon.Close()
	return "", nil
}
