package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	networkconfig "github.com/anhdt61/tcp-loadbalacer/internal/network"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	routes, err := networkconfig.ParseRoutes(os.Getenv("STATIC_ROUTES"))
	if err != nil {
		slog.Error("invalid static routes", "error", err)
		os.Exit(1)
	}
	if err := networkconfig.ConfigureRoutes(ctx, routes); err != nil {
		slog.Error("cannot configure return route through tcp-lb-gateway", "error", err)
		os.Exit(1)
	}
	address := env("LISTEN_ADDR", ":9000")
	listener, err := net.Listen("tcp", address)
	if err != nil {
		slog.Error("cannot listen", "address", address, "error", err)
		os.Exit(1)
	}
	defer listener.Close()
	slog.Info("TCP echo server ready", "address", address)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			if ctx.Err() != nil || errors.Is(acceptErr, net.ErrClosed) {
				return
			}
			slog.Warn("accept failed", "error", acceptErr)
			continue
		}
		go echo(connection)
	}
}

func echo(connection net.Conn) {
	defer connection.Close()
	slog.Info("emulated server accepted connection", "remote_address", connection.RemoteAddr())
	written, err := io.Copy(connection, connection)
	slog.Info("emulated server closed connection", "remote_address", connection.RemoteAddr(), "echoed_bytes", written, "error", err)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
