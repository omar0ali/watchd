package main

import (
	"strings"

	"github.com/omar0ali/watchd/cmd"
	"github.com/omar0ali/watchd/internal"
	"github.com/omar0ali/watchd/internal/app/daemon"
	"github.com/omar0ali/watchd/internal/app/daemon/core/models"
)

func main() {
	cmd.WD = &cmd.WatchDaemon{
		CurrProject: models.New(),
		Daemon:      daemon.New(internal.SOCKET_PATH),
	}

	cmd.WD.Daemon.StartListening(func(line string) (string, error) {
		args := strings.Split(line, " ")
		switch args[0] {
		case "load":
			return cmd.Load(args[1])

		case "ps":
			return cmd.PS(args)

		case "off":
			return cmd.Off()

		case "start":
			return cmd.Start()

		case "stop":
			return cmd.Stop()

		default:
			return "can't do that yet", nil
		}
	})
}
