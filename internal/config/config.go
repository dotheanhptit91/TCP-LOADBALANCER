package config

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// PortRange is an inclusive TCP port range.
type PortRange struct {
	Start int
	End   int
}

func ParsePortRange(value string) (PortRange, error) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 2 {
		return PortRange{}, fmt.Errorf("invalid port range %q, expected start-end", value)
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid start port %q: %w", parts[0], err)
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid end port %q: %w", parts[1], err)
	}
	if start < 1 || end > 65535 || start > end {
		return PortRange{}, fmt.Errorf("invalid port range %q", value)
	}
	return PortRange{Start: start, End: end}, nil
}

func (r PortRange) String() string { return fmt.Sprintf("%d-%d", r.Start, r.End) }

func (r PortRange) Ports() []int {
	ports := make([]int, 0, r.End-r.Start+1)
	for port := r.Start; port <= r.End; port++ {
		ports = append(ports, port)
	}
	return ports
}

// Mapping assigns a TCP source/destination port range to one worker network.
type Mapping struct {
	Range      PortRange
	WorkerName string
	WorkerCIDR *net.IPNet
}

var serviceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// ParseMappings parses: 1000-1010=worker1@10.10.0.0/24,2000-2010=worker2@10.20.0.0/24.
func ParseMappings(value string) ([]Mapping, error) {
	var mappings []Mapping
	seen := make(map[int]struct{})
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid mapping %q, expected range=worker@cidr", item)
		}
		portRange, err := ParsePortRange(parts[0])
		if err != nil {
			return nil, err
		}
		target := strings.SplitN(strings.TrimSpace(parts[1]), "@", 2)
		if len(target) != 2 || !serviceNamePattern.MatchString(target[0]) {
			return nil, fmt.Errorf("invalid worker target %q, expected worker@cidr", parts[1])
		}
		_, workerCIDR, err := net.ParseCIDR(target[1])
		if err != nil {
			return nil, fmt.Errorf("invalid worker CIDR %q: %w", target[1], err)
		}
		for _, port := range portRange.Ports() {
			if _, ok := seen[port]; ok {
				return nil, fmt.Errorf("destination port %d is mapped more than once", port)
			}
			seen[port] = struct{}{}
		}
		mappings = append(mappings, Mapping{Range: portRange, WorkerName: target[0], WorkerCIDR: workerCIDR})
	}
	if len(mappings) == 0 {
		return nil, fmt.Errorf("at least one port mapping is required")
	}
	return mappings, nil
}
