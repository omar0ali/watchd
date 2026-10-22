package daemon

import (
	"errors"
	"log"
	"net"
	"os"
	"sync"

	"github.com/omar0ali/watchd/internal/app/utils"
)

type Daemon struct {
	listener net.Listener
	path     string
	once     sync.Once
}

func New(socketPath string) (daemon *Daemon) {
	os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		panic(err)
	}

	return &Daemon{
		listener: listener,
		path:     socketPath,
		once:     sync.Once{},
	}
}

func (d *Daemon) StartListening(digestMessage func(line string) (string, error)) {
	for {
		con, err := d.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}

			log.Println("accept error:", err)
			continue

		}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Println("goroutine panic:", r)
				}
				con.Close()
			}()

			io := utils.NewConnIO(con)

			cmd, err := io.ReadLine()
			if err != nil {
				log.Println(err)
				return
			}

			result, err := digestMessage(cmd)
			if err != nil {
				log.Println(err)
				return
			}

			if err := io.WriteResponse(result); err != nil {
				log.Println(err)
				return
			}
		}()
	}
}

func (d *Daemon) Close() {
	d.once.Do(func() {
		log.Println("Cleaning up socket")
		d.listener.Close()
		os.Remove(d.path)
	})
}
