package client

import (
	"fmt"
	"net"
	"strings"

	"github.com/omar0ali/watchd/internal"
	"github.com/omar0ali/watchd/internal/app/utils"
)

func SendToDaemon(cmd string) (string, error) {
	if !internal.SocketExists() {
		return "", fmt.Errorf("socket is not running")
	}

	conn, err := net.Dial("unix", internal.SOCKET_PATH)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	io := utils.NewConnIO(conn)

	if err := io.WriteLine(cmd); err != nil {
		return "", err
	}

	result, err := io.ReadResponse()
	if err != nil {
		return "", err
	}

	return strings.Join(result, "\n"), nil
}
