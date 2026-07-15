package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/anhdt61/tcp-loadbalacer/internal/config"
	networkconfig "github.com/anhdt61/tcp-loadbalacer/internal/network"
	"github.com/anhdt61/tcp-loadbalacer/internal/state"
)

type worker struct {
	name            string
	instance        string
	portRange       config.PortRange
	remoteAddr      string
	localIP         net.IP
	sessionDuration time.Duration
	store           *state.Store
	active          atomic.Int64
	total           atomic.Uint64
	succeeded       atomic.Uint64
	errorMu         sync.RWMutex
	lastError       string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	portRange, err := config.ParsePortRange(env("SOURCE_PORT_RANGE", "1000-1010"))
	if err != nil {
		fatal("invalid source port range", err)
	}
	routes, err := networkconfig.ParseRoutes(os.Getenv("STATIC_ROUTES"))
	if err != nil {
		fatal("invalid static routes", err)
	}
	if err := networkconfig.ConfigureRoutes(ctx, routes); err != nil {
		fatal("cannot configure route through tcp-lb-gateway", err)
	}
	interval, err := time.ParseDuration(env("CONNECT_INTERVAL", "3s"))
	if err != nil || interval <= 0 {
		fatal("invalid connect interval", err)
	}
	sessionDuration, err := time.ParseDuration(env("SESSION_DURATION", "5s"))
	if err != nil || sessionDuration < 0 {
		fatal("invalid session duration", err)
	}
	hostname, _ := os.Hostname()
	w := &worker{
		name: env("WORKER_NAME", "tcp-backend-worker1"), instance: hostname,
		portRange: portRange, remoteAddr: env("REMOTE_ADDR", "10.30.0.10:9000"),
		localIP: net.ParseIP(os.Getenv("SOURCE_IP")), sessionDuration: sessionDuration,
		store: state.New(env("REDIS_ADDR", "redis:6379")),
	}
	defer w.store.Close()
	go w.serveHealth(ctx, env("HEALTH_ADDR", ":8080"))
	slog.Info("TCP client worker ready", "name", w.name, "source_port_range", portRange, "remote", w.remoteAddr, "interval", interval)

	// Start immediately, then create a new client connection on every tick.
	go w.runSession(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go w.runSession(ctx)
		}
	}
}

func (w *worker) runSession(ctx context.Context) {
	w.total.Add(1)
	connection, sourcePort, err := w.dialRemote(ctx)
	if err != nil {
		w.recordError(err)
		slog.Warn("server connection failed", "remote", w.remoteAddr, "error", err)
		return
	}
	w.active.Add(1)
	w.changeActive(ctx, 1)
	defer func() {
		_ = connection.Close()
		w.active.Add(-1)
		w.changeActive(context.Background(), -1)
	}()

	payload := []byte(fmt.Sprintf("worker=%s instance=%s source_port=%d time=%s\n", w.name, w.instance, sourcePort, time.Now().UTC().Format(time.RFC3339Nano)))
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := connection.Write(payload); err != nil {
		w.recordError(err)
		return
	}
	echo := make([]byte, len(payload))
	if _, err := io.ReadFull(connection, echo); err != nil {
		w.recordError(err)
		return
	}
	if !bytes.Equal(payload, echo) {
		w.recordError(fmt.Errorf("echo payload mismatch"))
		return
	}
	_ = connection.SetDeadline(time.Time{})
	w.succeeded.Add(1)
	w.recordError(nil)
	slog.Info("end-to-end TCP session established", "local", connection.LocalAddr(), "remote", connection.RemoteAddr(), "source_port", sourcePort)

	timer := time.NewTimer(w.sessionDuration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (w *worker) dialRemote(ctx context.Context) (net.Conn, int, error) {
	count := w.portRange.End - w.portRange.Start + 1
	var randomBytes [8]byte
	_, _ = rand.Read(randomBytes[:])
	startOffset := int(binary.LittleEndian.Uint64(randomBytes[:]) % uint64(count))
	var lastErr error
	for attempt := 0; attempt < count; attempt++ {
		port := w.portRange.Start + (startOffset+attempt)%count
		dialer := net.Dialer{
			Timeout:   5 * time.Second,
			LocalAddr: &net.TCPAddr{IP: w.localIP, Port: port},
			Control: func(_, _ string, raw syscall.RawConn) error {
				var socketErr error
				if err := raw.Control(func(fd uintptr) {
					socketErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				}); err != nil {
					return err
				}
				return socketErr
			},
		}
		connection, err := dialer.DialContext(ctx, "tcp", w.remoteAddr)
		if err == nil {
			return connection, port, nil
		}
		lastErr = err
	}
	return nil, 0, fmt.Errorf("all %d source ports in %s unavailable: %w", count, w.portRange, lastErr)
}

func (w *worker) recordError(err error) {
	w.errorMu.Lock()
	defer w.errorMu.Unlock()
	if err == nil {
		w.lastError = ""
		return
	}
	w.lastError = err.Error()
}

func (w *worker) changeActive(parent context.Context, delta int64) {
	ctx, cancel := context.WithTimeout(parent, 500*time.Millisecond)
	defer cancel()
	_ = w.store.ChangeActive(ctx, w.name, w.instance, delta)
}

func (w *worker) serveHealth(ctx context.Context, address string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(response http.ResponseWriter, _ *http.Request) {
		w.errorMu.RLock()
		lastError := w.lastError
		w.errorMu.RUnlock()
		status := "healthy"
		if w.total.Load() > 0 && w.succeeded.Load() == 0 && lastError != "" {
			status = "degraded"
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"service": w.name, "instance": w.instance, "status": status,
			"active_connections": w.active.Load(), "source_port_range": w.portRange.String(),
			"remote_address": w.remoteAddr, "total_connections": w.total.Load(),
			"successful_connections": w.succeeded.Load(), "last_error": lastError,
		})
	})
	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("health server stopped", "error", err)
	}
}

func fatal(message string, err error) {
	if err == nil {
		err = errors.New("value must be greater than or equal to zero")
	}
	slog.Error(message, "error", err)
	os.Exit(1)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
