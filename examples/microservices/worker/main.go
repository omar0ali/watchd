// worker pretends to process jobs, used by the watchd example.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	interval, err := time.ParseDuration(os.Getenv("JOB_INTERVAL"))
	if err != nil {
		interval = 2 * time.Second
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for job := 1; ; job++ {
		select {
		case <-ticker.C:
			log.Printf("processed job #%d", job)
		case s := <-stop:
			log.Printf("got %s, finishing up", s)
			return
		}
	}
}
