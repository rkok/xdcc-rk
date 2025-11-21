package proxy

import (
	"testing"
)

func TestValidateProxyURL(t *testing.T) {
	tests := []struct {
		name      string
		proxyURL  string
		wantError bool
	}{
		{
			name:      "empty URL is valid",
			proxyURL:  "",
			wantError: false,
		},
		{
			name:      "valid socks5 URL",
			proxyURL:  "socks5://localhost:1080",
			wantError: false,
		},
		{
			name:      "valid socks5 URL with auth",
			proxyURL:  "socks5://user:pass@localhost:1080",
			wantError: false,
		},
		{
			name:      "invalid scheme",
			proxyURL:  "http://localhost:8080",
			wantError: true,
		},
		{
			name:      "missing host",
			proxyURL:  "socks5://",
			wantError: true,
		},
		{
			name:      "invalid URL format",
			proxyURL:  "not a url",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProxyURL(tt.proxyURL)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateProxyURL() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestInitialize(t *testing.T) {
	tests := []struct {
		name      string
		proxyURL  string
		wantError bool
	}{
		{
			name:      "initialize with no proxy",
			proxyURL:  "",
			wantError: false,
		},
		{
			name:      "initialize with valid proxy",
			proxyURL:  "socks5://localhost:1080",
			wantError: false,
		},
		{
			name:      "initialize with invalid URL",
			proxyURL:  "://invalid",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Initialize(tt.proxyURL)
			if (err != nil) != tt.wantError {
				t.Errorf("Initialize() error = %v, wantError %v", err, tt.wantError)
			}

			if err == nil {
				// Verify the dialer was created
				d := GetDialer()
				if d == nil {
					t.Error("GetDialer() returned nil after successful Initialize()")
				}
				if d.ProxyURL() != tt.proxyURL {
					t.Errorf("ProxyURL() = %v, want %v", d.ProxyURL(), tt.proxyURL)
				}
			}
		})
	}
}

func TestIsProxyConfigured(t *testing.T) {
	// Test with no proxy
	err := Initialize("")
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}
	if IsProxyConfigured() {
		t.Error("IsProxyConfigured() = true, want false")
	}

	// Test with proxy
	err = Initialize("socks5://localhost:1080")
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}
	if !IsProxyConfigured() {
		t.Error("IsProxyConfigured() = false, want true")
	}
}

