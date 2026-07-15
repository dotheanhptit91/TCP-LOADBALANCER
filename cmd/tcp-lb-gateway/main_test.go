package main

import (
	"net"
	"strings"
	"testing"

	"github.com/anhdt61/tcp-loadbalacer/internal/config"
)

func TestFirewallRulesValidateBothDirections(t *testing.T) {
	mappings, err := config.ParseMappings("1000-1010=worker1@10.10.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	_, serverNet, _ := net.ParseCIDR("10.30.0.0/24")
	rules := buildFirewallRules(serverNet, net.ParseIP("10.30.0.254"), mappings)
	for _, expected := range []string{
		"ip saddr 10.10.0.0/24 ip daddr 10.30.0.0/24 tcp sport 1000-1010 ct state new,established",
		"ip saddr 10.30.0.0/24 ip daddr 10.10.0.0/24 tcp dport 1000-1010 ct state established",
		"ip saddr 10.10.0.0/24 ip daddr 10.30.0.0/24 tcp sport 1000-1010 counter snat ip to 10.30.0.254",
	} {
		if !strings.Contains(rules, expected) {
			t.Fatalf("expected rule %q in:\n%s", expected, rules)
		}
	}
}
