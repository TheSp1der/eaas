package main

import (
	"context"
	"os"
	"sync"
	"syscall"

	"log/slog"
	"net/http"
	"os/signal"

	"github.com/go-chi/chi/v5"
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

	// initialize webserver router
	router := chi.NewRouter()

	ws := &endpoint{log: log}
	// add healthcheck router
	router.Mount("/health", http.HandlerFunc(ws.healthcheck))
	// add entropy router
	router.Mount("/entropy", http.HandlerFunc(ws.entropy))

	// start webserver
	wg.Add(1)
	server := httpServer(wg, router, log, "0.0.0.0", 8080)

	forever(server, wg, log)
}

func forever(webserver *http.Server, wg *sync.WaitGroup, log *slog.Logger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		wg.Wait()
		c <- syscall.SIGABRT
	}()

	sig := <-c
	log.Warn("Shutting down, triggered by signal", "signal", sig)

	// send shutdown to webserver
	if err := webserver.Shutdown(context.TODO()); err != nil {
		log.Warn("Error shutting down", "error", err)
	}
}
