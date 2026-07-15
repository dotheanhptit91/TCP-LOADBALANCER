package selfheal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	serviceAccountPath = "/var/run/secrets/kubernetes.io/serviceaccount"
	checkInterval      = 10 * time.Second
	deleteTimeout      = 5 * time.Second
	missThreshold      = 3
)

// Start monitors the interfaces listed in REQUIRED_NETWORK_INTERFACES. When a
// Multus interface disappears, it deletes the current Pod so Kubernetes creates
// a fresh network namespace and runs the CNI ADD operation again.
func Start(ctx context.Context) error {
	required := parseInterfaces(os.Getenv("REQUIRED_NETWORK_INTERFACES"))
	if len(required) == 0 {
		return nil
	}

	if missing := missingInterfaces(required); len(missing) > 0 {
		if err := deleteSelf(ctx, missing); err != nil {
			return fmt.Errorf("required network interfaces missing (%s), cannot request Pod recreation: %w", strings.Join(missing, ","), err)
		}
		return fmt.Errorf("required network interfaces missing (%s); Pod recreation requested", strings.Join(missing, ","))
	}

	go monitor(ctx, required)
	return nil
}

func monitor(ctx context.Context, required []string) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	misses := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			missing := missingInterfaces(required)
			if len(missing) == 0 {
				misses = 0
				continue
			}

			misses++
			slog.Warn("required network interface missing", "interfaces", strings.Join(missing, ","), "consecutive_checks", misses)
			if misses < missThreshold {
				continue
			}
			if err := deleteSelf(ctx, missing); err != nil {
				slog.Error("cannot request Pod recreation", "error", err)
				continue
			}
			slog.Warn("Pod recreation requested to restore Multus interfaces", "interfaces", strings.Join(missing, ","))
			return
		}
	}
}

func parseInterfaces(value string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, item := range strings.Split(value, ",") {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

func missingInterfaces(required []string) []string {
	var missing []string
	for _, name := range required {
		if _, err := net.InterfaceByName(name); err != nil {
			missing = append(missing, name)
		}
	}
	return missing
}

func deleteSelf(parent context.Context, missing []string) error {
	namespaceBytes, err := os.ReadFile(serviceAccountPath + "/namespace")
	if err != nil {
		return fmt.Errorf("read namespace: %w", err)
	}
	tokenBytes, err := os.ReadFile(serviceAccountPath + "/token")
	if err != nil {
		return fmt.Errorf("read service account token: %w", err)
	}
	caBytes, err := os.ReadFile(serviceAccountPath + "/ca.crt")
	if err != nil {
		return fmt.Errorf("read Kubernetes CA: %w", err)
	}
	podName, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("read Pod hostname: %w", err)
	}

	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS")
	if host == "" || port == "" {
		return fmt.Errorf("Kubernetes service environment is unavailable")
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caBytes) {
		return fmt.Errorf("parse Kubernetes CA")
	}

	ctx, cancel := context.WithTimeout(parent, deleteTimeout)
	defer cancel()
	url := fmt.Sprintf("https://%s/api/v1/namespaces/%s/pods/%s", net.JoinHostPort(host, port), strings.TrimSpace(string(namespaceBytes)), podName)
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(tokenBytes)))
	request.Header.Set("X-TCP-LB-Recovery-Reason", "missing-interfaces="+strings.Join(missing, ","))

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: caPool, MinVersion: tls.VersionTLS12}}}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusAccepted || response.StatusCode == http.StatusNotFound {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
	return fmt.Errorf("Kubernetes API returned %s: %s", response.Status, strings.TrimSpace(string(body)))
}
