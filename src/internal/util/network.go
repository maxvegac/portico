package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// GetServerIP gets the first non-loopback IPv4 address of the server
func GetServerIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Only return IPv4 addresses
			if ip != nil && ip.To4() != nil {
				return ip.String(), nil
			}
		}
	}

	return "", nil
}

// IPToSSlipIO converts an IP address to sslip.io format (with dashes)
// Example: "192.168.0.1" -> "192-168-0-1.sslip.io"
func IPToSSlipIO(ip string) string {
	return strings.ReplaceAll(ip, ".", "-") + ".sslip.io"
}

// AppNameToSSlipIO generates appname.IP.sslip.io format
// Example: "facturacion-api", "192.168.0.1" -> "facturacion-api.192-168-0-1.sslip.io"
func AppNameToSSlipIO(appName, ip string) string {
	ipFormatted := strings.ReplaceAll(ip, ".", "-")
	return fmt.Sprintf("%s.%s.sslip.io", appName, ipFormatted)
}

// IPInfoResponse represents the response from ipinfo.io/json
type IPInfoResponse struct {
	IP string `json:"ip"`
}

// GetExternalIP gets the external IP address using ipinfo.io/json
func GetExternalIP() (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("https://ipinfo.io/json")
	if err != nil {
		return "", fmt.Errorf("error fetching external IP: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ipinfo.io returned status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	var ipInfo IPInfoResponse
	if err := json.Unmarshal(body, &ipInfo); err != nil {
		return "", fmt.Errorf("error parsing JSON response: %w", err)
	}

	if ipInfo.IP == "" {
		return "", fmt.Errorf("no IP found in response")
	}

	return ipInfo.IP, nil
}

// GetServerIPWithFallback gets the server IP, using configured IP if provided,
// otherwise trying local first, then external IP as fallback
func GetServerIPWithFallback(configuredIP string) (string, error) {
	// If IP is configured, use it
	if configuredIP != "" {
		return configuredIP, nil
	}

	// Try to get local IP first
	localIP, err := GetServerIP()
	if err == nil && localIP != "" {
		return localIP, nil
	}

	// Fallback to external IP
	externalIP, err := GetExternalIP()
	if err != nil {
		return "", fmt.Errorf("failed to get both local and external IP: local=%v, external=%w", err, err)
	}

	return externalIP, nil
}
