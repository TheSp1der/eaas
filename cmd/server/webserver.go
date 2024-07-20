package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type endpoint struct{ log *slog.Logger }

func httpServer(wg *sync.WaitGroup, r *chi.Mux, log *slog.Logger, addr string, port int) *http.Server {
	s := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", addr, port),
		Handler: r,
		// timeouts (open-socket dos prevention)
		ReadTimeout:       2 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      2 * time.Second,
		IdleTimeout:       2 * time.Second,
	}

	// start webserver
	go func(wg *sync.WaitGroup, log *slog.Logger) {
		defer wg.Done()

		if err := s.ListenAndServe(); err != nil {
			log.Error("internal webserver stopped", "error", err)
		}
	}(wg, log)

	return s
}

func httpAccessLog(log *slog.Logger, r *http.Request) {
	log.Debug("http activity", "method", r.Method, "remote_address", r.RemoteAddr, "url", r.RequestURI)
}

func (endpoint *endpoint) healthcheck(w http.ResponseWriter, r *http.Request) {
	httpAccessLog(endpoint.log, r)

	hc := struct {
		Status string `json:"status"`
		Method string `json:"method"`
	}{
		Status: "healthy",
		Method: r.Method,
	}

	w.Header().Add("Content-Type", "application/json")
	output, err := json.MarshalIndent(hc, "", "  ")
	if err != nil {
		endpoint.log.Error("unable to marshal healthcheck output", "error", err)
	}

	_, err = w.Write(output)
	if err != nil {
		endpoint.log.Error("unable to send data to client", "error", err)
	}
}

func (endpoint *endpoint) entropy(w http.ResponseWriter, r *http.Request) {
	httpAccessLog(endpoint.log, r)

	var (
		output struct {
			Error    bool   `json:"error"`
			ErrorMsg string `json:"error-message,omitempty"`
			Data     string `json:"data-base64,omitempty"`
		}
		ServerStatusCode int = http.StatusOK
	)

	// check method
	if !strings.EqualFold(r.Method, "GET") {
		output.Error = true
		output.ErrorMsg = fmt.Sprintf("invalid request, incorrect method requested %s", r.Method)
		ServerStatusCode = http.StatusBadRequest
	}

	// read, convert to requested bytes to int, and validate request
	raw := r.URL.Query().Get("bytes")
	reqBytes, err := strconv.ParseInt(raw, 10, 0)
	switch {
	case raw == "":
		output.Error = true
		output.ErrorMsg = "invalid request, no bytes in request"
		ServerStatusCode = http.StatusBadRequest
	case err != nil:
		output.Error = true
		output.ErrorMsg = "invalid request, bytes must be integer"
		ServerStatusCode = http.StatusBadRequest
	case reqBytes <= 0:
		output.Error = true
		output.ErrorMsg = "invalid request, bytes must larger than 0"
		ServerStatusCode = http.StatusBadRequest
	case reqBytes > 1024*1024*2:
		output.Error = true
		output.ErrorMsg = "invalid request, bytes must be less than 2 MiB"
		ServerStatusCode = http.StatusBadRequest
	}

	// get data
	if !output.Error {
		d, err := getRandomBytesSystem("/dev/random", int(reqBytes))
		if err != nil {
			output.Error = true
			output.ErrorMsg = "internal server error, unable to get random data from source"
			ServerStatusCode = http.StatusInternalServerError
		}
		output.Data = base64.StdEncoding.EncodeToString(d)
	}
	if output.Error {
		endpoint.log.Error("server error", "error", output.ErrorMsg)
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(ServerStatusCode)

	outputJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		endpoint.log.Error("unable to marshal healthcheck output", "error", err)
	}

	_, err = w.Write(outputJSON)
	if err != nil {
		endpoint.log.Error("unable to send data to client", "error", err)
	}
}
