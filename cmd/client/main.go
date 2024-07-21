package main

import (
	"os"
	"sync"
	"syscall"
	"time"

	"log/slog"
	"os/signal"
)

func main() {
	// set-up slog
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// create a wait group
	wg := new(sync.WaitGroup)

	// configure shutdown sequence
	defer func(*sync.WaitGroup) {
		wg.Wait()
		log.Info("shutdown sequence complete")
	}(wg)

	// initialize entropy client
	ec := make(chan bool, 1)
	wg.Add(1)
	c := entropy{
		log:            log,
		minimumEntropy: 1200,
		shutdown:       ec,
		source:         "http://127.0.0.1:8080/entropy",
		timeout:        time.Second,
		wg:             wg,
	}
	go c.client()

	// wait
	forever(ec, wg, log)
}

func forever(ec chan<- bool, wg *sync.WaitGroup, log *slog.Logger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		wg.Wait()
		c <- syscall.SIGABRT
	}()

	sig := <-c
	log.Warn("Shutting down, triggered by signal", "signal", sig)

	log.Info("shutting down entropy client")
	ec <- true
}
