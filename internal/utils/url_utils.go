package utils

import (
	"errors"
	"fmt"
	"net"
	"net/url"
)

func ValidateURL(rawURL string) error {
	parsed, err := validateStructure(rawURL)
	if err != nil {
		return err
	}

	if err := validateSSRF(parsed); err != nil {
		return err
	}

	return nil
}

func validateStructure(raw string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL structure: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", parsed.Scheme)
	}

	if parsed.Host == "" {
		return nil, errors.New("missing host in URL")
	}

	return parsed, nil
}

func validateSSRF(u *url.URL) error {
	host := u.Hostname()

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve host: %w", err)
	}

	for _, ip := range ips {
		if IsPrivateIP(ip) {
			return fmt.Errorf("blocked SSRF target: %s (%s)", u.String(), ip.String())
		}
	}

	return nil
}

func IsPrivateIP(ip net.IP) bool {
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}

	privateCIDRs := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"0.0.0.0/8",
	}

	for _, block := range privateCIDRs {
		_, network, _ := net.ParseCIDR(block)
		if network.Contains(ip) {
			return true
		}
	}

	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return true
	}

	return false
}
