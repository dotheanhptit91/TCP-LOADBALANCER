package network

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
)

type Route struct {
	Destination *net.IPNet
	Gateway     net.IP
}

// ParseRoutes parses comma-separated destinationCIDR=gatewayIP routes.
func ParseRoutes(value string) ([]Route, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var routes []Route
	for _, item := range strings.Split(value, ",") {
		parts := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid route %q, expected cidr=gateway", item)
		}
		_, destination, err := net.ParseCIDR(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid route destination %q: %w", parts[0], err)
		}
		gateway := net.ParseIP(parts[1])
		if gateway == nil {
			return nil, fmt.Errorf("invalid route gateway %q", parts[1])
		}
		routes = append(routes, Route{Destination: destination, Gateway: gateway})
	}
	return routes, nil
}

func ConfigureRoutes(ctx context.Context, routes []Route) error {
	for _, route := range routes {
		command := exec.CommandContext(ctx, "ip", "route", "replace", route.Destination.String(), "via", route.Gateway.String())
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("configure route %s via %s: %w: %s", route.Destination, route.Gateway, err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}
