package parser

import (
	"fmt"
	"log"

	"github.com/omar0ali/watchd/internal/app/daemon/core/models"
	"github.com/omar0ali/watchd/internal/app/utils"
	lua "github.com/yuin/gopher-lua"
)

type Parser struct {
	luaState *lua.LState
}

func New(file string) (*Parser, error) {
	L := lua.NewState()
	err := L.DoFile(file)
	if err != nil {
		return nil, err
	}
	return &Parser{
		luaState: L,
	}, nil
}

func (p *Parser) Start() (*models.Project, error) {
	lv := p.luaState.Get(-1) // this is a table
	tbl, ok := lv.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("Its not a table")
	}

	// name of project
	projectName := tbl.RawGetString("name").String()

	apps := tbl.RawGetString("apps")
	appsTable, ok := apps.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("Apps must be a table")
	}

	listOfApps := []models.App{}
	appsTable.ForEach(func(_, value lua.LValue) {
		appTbl, ok := value.(*lua.LTable)
		if !ok {
			return
		}

		name := appTbl.RawGetString("name").String()
		cmd := appTbl.RawGetString("cmd").String()

		// TODO:
		// cwd := appTbl.RawGetString("cwd").String()

		cwd, err := utils.GetCWD()
		if err != nil {
			log.Println("Can't get current directory.")
			return
		}

		watchList := []string{}
		watch := appTbl.RawGetString("watch")
		if watchTbl, ok := watch.(*lua.LTable); ok {
			watchTbl.ForEach(func(_, v lua.LValue) {
				watchList = append(watchList, v.String())
			})
		}

		// ignore (array)
		ignoreList := []string{}
		ignore := appTbl.RawGetString("ignore")
		if ignoreTbl, ok := ignore.(*lua.LTable); ok {
			ignoreTbl.ForEach(func(_, v lua.LValue) {
				ignoreList = append(ignoreList, v.String())
			})
		}

		// restart
		v := appTbl.RawGetString("restart")
		var restart models.RestartPolicy
		if str, ok := v.(lua.LString); ok {
			restart = models.RestartPolicy(str)
		} else {
			fmt.Println("Error reading restart [always, never] only accepted")
			return
		}

		// on_change (function)
		onChange := appTbl.RawGetString("on_change")
		fn, ok := onChange.(*lua.LFunction)
		if !ok {
			fmt.Println("Error failed to read on_change")
			return
		}

		// TODO:
		listOfApps = append(listOfApps, models.App{
			Name:        name,
			CMD:         cmd, // need to fix the path
			CWD:         cwd, // need to fix the path
			WatchExts:   watchList,
			IgnoreFiles: ignoreList,
			OnChange:    fn,
			Restart:     restart,
		})

	})
	return &models.Project{
		Name: projectName,
		Apps: listOfApps,
	}, nil
}

func (p *Parser) Close() {
	p.luaState.Close()
}
