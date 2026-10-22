package core

import (
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/omar0ali/watchd/internal"
)

type Env int

const (
	DEV Env = iota
	PROD
)

// start daemon if not started

func Load(e Env) {
	// check socket
	if internal.SocketExists() {
		log.Println("already running: /tmp/watchd.sock")
		return
	}

	var start *exec.Cmd

	switch e {
	case DEV:
		start = exec.Command("go", "run", "../cmd/watchd-daemon/") //TODO: this must be modofied if needed
	case PROD:
		path, err := exec.LookPath("watchd-daemon")
		if err != nil {
			log.Fatalf("daemon not found: %v", err)
		}
		start = exec.Command(path)
	}

	start.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	start.Stdout = os.Stdout
	start.Stderr = os.Stderr

	err := start.Start()
	if err != nil {
		panic(err)
	}

	log.Println("running: /tmp/watchd.sock")
}
