// The fake PMS: a booking system for the demo, serving the agent's guest
// tools over MCP and the Guardian's permissions read from the bookings file,
// which it re-reads when the file changes.
package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/bookings"
	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/concierge"
	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/config"
	"github.com/disciplinedware/acs-openclaw-demo/fake-pms/internal/server"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if len(os.Args) >= 2 && os.Args[1] == "bookings-move" {
		os.Exit(bookingsMove(os.Args[2:]))
	}
	log, _ := zap.NewProduction()
	defer func() { _ = log.Sync() }()
	if err := run(log); err != nil {
		log.Fatal("fake PMS stopped", zap.Error(err))
	}
}

func run(log *zap.Logger) error {
	cfg, err := config.Load(os.Environ)
	if err != nil {
		return err
	}
	store, err := bookings.OpenStore(cfg.BookingsFile, os.Getenv, log)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           server.Handler(concierge.New(store.Book, time.Now), cfg.APIKey, log),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go store.Watch(ctx, cfg.ReloadInterval)
	served := make(chan error, 1)
	go func() { served <- srv.ListenAndServe() }()
	log.Info("fake PMS listening", zap.String("addr", cfg.ListenAddr),
		zap.String("business", store.Book().BusinessName), zap.Int("reservations", len(store.Book().Reservations)))
	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// healthcheck is the container's health probe (the image has no shell):
// exit 0 when the server on PMS_LISTEN_ADDR answers its health check.
func healthcheck() int {
	cfg, err := config.Load(os.Environ)
	if err != nil {
		return 1
	}
	_, port, err := net.SplitHostPort(cfg.ListenAddr)
	if err != nil {
		return 1
	}
	client := &http.Client{Timeout: cfg.ReadHeaderTimeout}
	resp, err := client.Get("http://" + net.JoinHostPort("127.0.0.1", port) + server.HealthPath)
	if err != nil {
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
