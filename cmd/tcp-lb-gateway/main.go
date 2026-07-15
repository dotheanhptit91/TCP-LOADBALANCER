package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/anhdt61/tcp-loadbalacer/internal/config"
	"github.com/anhdt61/tcp-loadbalacer/internal/selfheal"
	"github.com/anhdt61/tcp-loadbalacer/internal/state"
)

type gateway struct {
	instance        string
	serverNet       *net.IPNet
	gatewayServerIP net.IP
	mappings        []config.Mapping
	store           *state.Store
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := selfheal.Start(ctx); err != nil {
		fatal("network self-healing", err)
	}

	mappings, err := config.ParseMappings(env("PORT_MAPPINGS", "1000-1010=tcp-backend-worker1@10.10.0.0/24,2000-2010=tcp-backend-worker2@10.20.0.0/24"))
	if err != nil {
		fatal("invalid port mapping", err)
	}
	_, serverNet, err := net.ParseCIDR(env("SERVER_CIDR", "10.30.0.0/24"))
	if err != nil {
		fatal("invalid server CIDR", err)
	}
	gatewayServerIP := net.ParseIP(env("GATEWAY_SERVER_IP", "10.30.0.254"))
	if gatewayServerIP == nil || gatewayServerIP.To4() == nil || !serverNet.Contains(gatewayServerIP) {
		fatal("invalid gateway server-side IP", fmt.Errorf("GATEWAY_SERVER_IP must be an IPv4 address inside %s", serverNet))
	}
	if err := enableForwarding(); err != nil {
		fatal("cannot enable IPv4 forwarding", err)
	}
	if err := installFirewall(ctx, serverNet, gatewayServerIP, mappings); err != nil {
		fatal("cannot install nftables data plane", err)
	}

	hostname, _ := os.Hostname()
	g := &gateway{
		instance: hostname, serverNet: serverNet, gatewayServerIP: gatewayServerIP, mappings: mappings,
		store: state.New(env("REDIS_ADDR", "redis:6379")),
	}
	defer g.store.Close()
	portMappings := make(map[int]string)
	for _, mapping := range mappings {
		value := mapping.WorkerName + "@" + mapping.WorkerCIDR.String() + " via " + gatewayServerIP.String()
		for _, port := range mapping.Range.Ports() {
			portMappings[port] = value
		}
		slog.Info("stateful NAT route ready", "worker", mapping.WorkerName, "worker_cidr", mapping.WorkerCIDR, "port_range", mapping.Range, "gateway_server_ip", gatewayServerIP, "server_cidr", serverNet)
	}
	redisCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	if err := g.store.ReplacePortMappings(redisCtx, portMappings); err != nil {
		cancel()
		fatal("cannot replace Redis port mappings", err)
	}
	cancel()

	go g.serveHealth(ctx, env("HEALTH_ADDR", ":8080"))
	<-ctx.Done()
}

func enableForwarding() error {
	value, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(value)) != "1" {
		return fmt.Errorf("net.ipv4.ip_forward is disabled; set the container sysctl to 1")
	}
	return nil
}

func installFirewall(ctx context.Context, serverNet *net.IPNet, gatewayServerIP net.IP, mappings []config.Mapping) error {
	// A stale table can remain after an ungraceful restart; deletion is idempotent here.
	_ = exec.CommandContext(ctx, "nft", "delete", "table", "inet", "tcp_lb").Run()
	ruleSet := buildFirewallRules(serverNet, gatewayServerIP, mappings)
	command := exec.CommandContext(ctx, "nft", "-f", "-")
	command.Stdin = strings.NewReader(ruleSet)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("nft failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func buildFirewallRules(serverNet *net.IPNet, gatewayServerIP net.IP, mappings []config.Mapping) string {
	var rules strings.Builder
	rules.WriteString("table inet tcp_lb {\n")
	rules.WriteString(" chain forward {\n")
	rules.WriteString("  type filter hook forward priority filter; policy drop;\n")
	rules.WriteString("  ct state invalid counter drop\n")
	for _, mapping := range mappings {
		fmt.Fprintf(&rules, "  ip saddr %s ip daddr %s tcp sport %s ct state new,established counter accept comment \"%s outbound\"\n",
			mapping.WorkerCIDR, serverNet, mapping.Range, mapping.WorkerName)
		fmt.Fprintf(&rules, "  ip saddr %s ip daddr %s tcp dport %s ct state established counter accept comment \"%s return\"\n",
			serverNet, mapping.WorkerCIDR, mapping.Range, mapping.WorkerName)
	}
	rules.WriteString(" }\n")
	rules.WriteString(" chain postrouting {\n")
	rules.WriteString("  type nat hook postrouting priority srcnat; policy accept;\n")
	for _, mapping := range mappings {
		fmt.Fprintf(&rules, "  ip saddr %s ip daddr %s tcp sport %s counter snat ip to %s comment \"%s SNAT\"\n",
			mapping.WorkerCIDR, serverNet, mapping.Range, gatewayServerIP, mapping.WorkerName)
	}
	rules.WriteString(" }\n}\n")
	return rules.String()
}

func (g *gateway) activeConnections(ctx context.Context) int {
	output, err := exec.CommandContext(ctx, "conntrack", "-L", "-p", "tcp", "--state", "ESTABLISHED").Output()
	if err != nil {
		return 0
	}
	active := 0
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		for _, field := range fields {
			if !strings.HasPrefix(field, "sport=") {
				continue
			}
			port, parseErr := strconv.Atoi(strings.TrimPrefix(field, "sport="))
			if parseErr == nil && g.ownsPort(port) {
				active++
			}
			break // Only inspect the original direction's first sport field.
		}
	}
	return active
}

func (g *gateway) ownsPort(port int) bool {
	for _, mapping := range g.mappings {
		if port >= mapping.Range.Start && port <= mapping.Range.End {
			return true
		}
	}
	return false
}

func (g *gateway) serveHealth(ctx context.Context, address string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"service": "tcp-lb-gateway", "instance": g.instance, "status": "healthy",
			"mode": "linux-routing+nftables-snat", "gateway_server_ip": g.gatewayServerIP.String(),
			"active_connections": g.activeConnections(request.Context()),
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
	slog.Error(message, "error", err)
	os.Exit(1)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
