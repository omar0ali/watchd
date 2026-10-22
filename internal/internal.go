package internal

import "os"

const SOCKET_PATH = "/tmp/watchd.sock"

func SocketExists() bool {
	_, err := os.Stat(SOCKET_PATH)
	return err == nil
}
