package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/anhdt61/tcp-loadbalacer/internal/state"
)

type serviceTarget struct {
	Name string
	Host string
	Port int
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	targets, err := parseTargets(env("MANAGED_SERVICES", "tcp-lb-gateway=tcp-lb-gateway:8080,tcp-backend-worker1=tcp-backend-worker1:8080,tcp-backend-worker2=tcp-backend-worker2:8080"))
	if err != nil {
		slog.Error("invalid managed services", "error", err)
		os.Exit(1)
	}
	interval, err := time.ParseDuration(env("POLL_INTERVAL", "5s"))
	if err != nil {
		slog.Error("invalid poll interval", "error", err)
		os.Exit(1)
	}
	manager := &manager{
		store:  state.New(env("REDIS_ADDR", "redis:6379")),
		client: &http.Client{Timeout: 2 * time.Second}, targets: targets,
	}
	defer manager.store.Close()

	manager.poll(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			manager.poll(ctx)
		}
	}
}

type manager struct {
	store   *state.Store
	client  *http.Client
	targets []serviceTarget
}

func (m *manager) poll(ctx context.Context) {
	for _, target := range m.targets {
		addresses, err := net.DefaultResolver.LookupHost(ctx, target.Host)
		if err != nil {
			slog.Warn("service discovery failed", "service", target.Name, "error", err)
			m.save(ctx, target.Name, target.Host, map[string]any{"status": "unhealthy", "error": err.Error()})
			continue
		}
		for _, address := range addresses {
			m.check(ctx, target, address)
		}
	}
}

func (m *manager) check(ctx context.Context, target serviceTarget, address string) {
	url := "http://" + net.JoinHostPort(address, strconv.Itoa(target.Port)) + "/healthz"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	response, err := m.client.Do(request)
	if err != nil {
		m.save(ctx, target.Name, address, map[string]any{"status": "unhealthy", "error": err.Error(), "address": address})
		return
	}
	defer response.Body.Close()
	fields := make(map[string]any)
	if decodeErr := json.NewDecoder(response.Body).Decode(&fields); decodeErr != nil {
		fields["error"] = decodeErr.Error()
	}
	fields["address"] = address
	fields["health_http_status"] = response.StatusCode
	if response.StatusCode != http.StatusOK {
		fields["status"] = "unhealthy"
	}
	instance, _ := fields["instance"].(string)
	if instance == "" {
		instance = address
	}
	m.save(ctx, target.Name, instance, fields)
}

func (m *manager) save(parent context.Context, service, instance string, fields map[string]any) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	if err := m.store.UpsertInstance(ctx, service, instance, fields); err != nil {
		slog.Warn("cannot update Redis state", "service", service, "instance", instance, "error", err)
		return
	}
	slog.Info("instance state updated", "service", service, "instance", instance, "status", fields["status"])
}

func parseTargets(value string) ([]serviceTarget, error) {
	var targets []serviceTarget
	for _, item := range strings.Split(value, ",") {
		parts := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid service target %q", item)
		}
		host, portValue, err := net.SplitHostPort(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid service address %q: %w", parts[1], err)
		}
		port, err := strconv.Atoi(portValue)
		if err != nil {
			return nil, fmt.Errorf("invalid health port %q: %w", portValue, err)
		}
		targets = append(targets, serviceTarget{Name: parts[0], Host: host, Port: port})
	}
	return targets, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
