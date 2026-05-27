package utils

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		private bool
	}{
		{"loopback", "127.0.0.1", true},
		{"loopback high", "127.255.255.255", true},
		{"10.x", "10.0.0.1", true},
		{"172.16.x", "172.16.0.1", true},
		{"192.168.x", "192.168.1.1", true},
		{"link-local", "169.254.1.1", true},
		{"zero network", "0.0.0.1", true},
		{"public 8.8.8.8", "8.8.8.8", false},
		{"public 1.1.1.1", "1.1.1.1", false},
		{"public 93.184.216.34", "93.184.216.34", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			got := IsPrivateIP(ip)
			if got != tt.private {
				t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}

func TestValidateURL_Schemes(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"ftp blocked", "ftp://example.com/file", true},
		{"javascript blocked", "javascript:alert(1)", true},
		{"file blocked", "file:///etc/passwd", true},
		{"empty string", "", true},
		{"no scheme", "example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL_PublicHTTPS(t *testing.T) {
	// A well-known public domain should pass.
	// Note: this test requires DNS resolution, so it may fail in fully offline environments.
	err := ValidateURL("https://example.com/webhook")
	if err != nil {
		t.Errorf("expected public HTTPS URL to pass, got: %v", err)
	}
}

func TestValidateURL_PrivateHosts(t *testing.T) {
	// These use IP literals so DNS resolution returns the IP directly.
	tests := []struct {
		name string
		url  string
	}{
		{"loopback", "http://127.0.0.1/hook"},
		{"private 10.x", "http://10.0.0.1/hook"},
		{"private 192.168.x", "http://192.168.1.1/hook"},
		{"link-local", "http://169.254.169.254/latest/meta-data/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if err == nil {
				t.Errorf("expected ValidateURL(%q) to block private IP, but it passed", tt.url)
			}
		})
	}
}
