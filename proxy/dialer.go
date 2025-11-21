package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"golang.org/x/net/proxy"
)

// Dialer provides a centralized proxy-aware dialer for all network operations
type Dialer struct {
	proxyURL    string
	baseDialer  *net.Dialer
	proxyDialer proxy.Dialer
	httpClient  *http.Client
}

var (
	// globalDialer is the singleton instance used throughout the application
	globalDialer *Dialer
)

// Initialize sets up the global proxy dialer with the given proxy URL
// proxyURL should be in the format: socks5://[user:pass@]host:port
// If proxyURL is empty, it will check environment variables (XDCC_PROXY, ALL_PROXY, all_proxy)
func Initialize(proxyURL string) error {
	if proxyURL == "" {
		// Check environment variables
		proxyURL = os.Getenv("XDCC_PROXY")
		if proxyURL == "" {
			proxyURL = os.Getenv("ALL_PROXY")
		}
		if proxyURL == "" {
			proxyURL = os.Getenv("all_proxy")
		}
	}

	d := &Dialer{
		proxyURL: proxyURL,
		baseDialer: &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}

	// If a proxy URL is provided, set up the proxy dialer
	if proxyURL != "" {
		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			return err
		}

		// Extract authentication if present
		var auth *proxy.Auth
		if parsedURL.User != nil {
			password, _ := parsedURL.User.Password()
			auth = &proxy.Auth{
				User:     parsedURL.User.Username(),
				Password: password,
			}
		}

		// Create SOCKS5 dialer
		proxyDialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, d.baseDialer)
		if err != nil {
			return err
		}
		d.proxyDialer = proxyDialer
	}

	// Create HTTP client
	d.httpClient = d.createHTTPClient()

	globalDialer = d
	return nil
}

// createHTTPClient creates an HTTP client configured with the proxy dialer
func (d *Dialer) createHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:          10,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if d.proxyDialer != nil {
		// Use proxy dialer for all connections
		if contextDialer, ok := d.proxyDialer.(proxy.ContextDialer); ok {
			transport.DialContext = contextDialer.DialContext
		} else {
			// Fallback for dialers that don't support DialContext
			transport.Dial = d.proxyDialer.Dial
		}
	} else {
		// No proxy, use base dialer
		transport.DialContext = d.baseDialer.DialContext
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
}

// DialContext dials a network connection, optionally through a proxy
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if d.proxyDialer != nil {
		// Use proxy dialer
		if contextDialer, ok := d.proxyDialer.(proxy.ContextDialer); ok {
			return contextDialer.DialContext(ctx, network, address)
		}
		// Fallback for dialers that don't support DialContext
		return d.proxyDialer.Dial(network, address)
	}
	// No proxy, use base dialer
	return d.baseDialer.DialContext(ctx, network, address)
}

// Dial dials a network connection, optionally through a proxy
func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

// HTTPClient returns the configured HTTP client
func (d *Dialer) HTTPClient() *http.Client {
	return d.httpClient
}

// ProxyURL returns the configured proxy URL, or empty string if no proxy
func (d *Dialer) ProxyURL() string {
	return d.proxyURL
}

// GetDialer returns the global dialer instance
// If Initialize hasn't been called, it returns a dialer with no proxy
func GetDialer() *Dialer {
	if globalDialer == nil {
		// Initialize with no proxy as fallback
		_ = Initialize("")
	}
	return globalDialer
}

// HTTPClient returns the global HTTP client
func HTTPClient() *http.Client {
	return GetDialer().HTTPClient()
}

// DialContext dials using the global dialer
func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return GetDialer().DialContext(ctx, network, address)
}

// Dial dials using the global dialer
func Dial(network, address string) (net.Conn, error) {
	return GetDialer().Dial(network, address)
}

// ProxyURL returns the configured proxy URL from the global dialer
func ProxyURL() string {
	return GetDialer().ProxyURL()
}

// IsProxyConfigured returns true if a proxy is configured
func IsProxyConfigured() bool {
	return GetDialer().ProxyURL() != ""
}

// ValidateProxyURL validates that a proxy URL is in the correct format
func ValidateProxyURL(proxyURL string) error {
	if proxyURL == "" {
		return nil // Empty is valid (means no proxy)
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return err
	}

	if parsedURL.Scheme != "socks5" {
		return errors.New("only socks5:// proxy URLs are supported")
	}

	if parsedURL.Host == "" {
		return errors.New("proxy URL must include host:port")
	}

	return nil
}

