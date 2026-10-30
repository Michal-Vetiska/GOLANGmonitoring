package monitor

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/pulsewatch/pulsewatch/backend/internal/models"
)

const (
	DefaultHTTPTimeout = 5 * time.Second
	maxResponseBody    = 64 * 1024 // 64 KiB — enough to drain, not to parse
)

// Checker performs SSRF-safe HTTP GET checks against public endpoints.
type Checker struct {
	client *http.Client
}

// NewChecker builds a shared HTTP client with a custom dialer that validates
// the resolved destination IP before connecting (DNS-rebinding defense).
func NewChecker(timeout time.Duration) *Checker {
	if timeout <= 0 {
		timeout = DefaultHTTPTimeout
	}

	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy:                 nil, // never honor HTTP_PROXY for user-supplied URLs
		DialContext:           safeDialContext(dialer),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: timeout,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		// Disable automatic redirects so we never follow to a private IP.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &Checker{client: client}
}

// safeDialContext resolves the hostname, filters non-public IPs, and dials
// only an approved address. This closes the DNS-rebinding window between
// ValidatePublicURL and the actual TCP connect.
func safeDialContext(d *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid dial address: %w", err)
		}

		var ips []net.IP
		if ip := net.ParseIP(host); ip != nil {
			ips = []net.IP{ip}
		} else {
			resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("dns lookup failed: %w", err)
			}
			for _, a := range resolved {
				ips = append(ips, a.IP)
			}
		}

		public, err := filterPublicIPs(ips)
		if err != nil {
			return nil, err
		}

		var lastErr error
		for _, ip := range public {
			target := net.JoinHostPort(ip.String(), port)
			conn, err := d.DialContext(ctx, network, target)
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("no public address could be dialed")
		}
		return nil, lastErr
	}
}

// Check performs a single GET against the monitor URL and returns a CheckResult.
func (c *Checker) Check(ctx context.Context, monitorID, rawURL string) models.CheckResult {
	result := models.CheckResult{
		MonitorID: monitorID,
		CheckedAt: time.Now().UTC(),
		IsUp:      false,
	}

	u, err := ValidatePublicURL(rawURL)
	if err != nil {
		msg := err.Error()
		result.ErrorMessage = &msg
		return result
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		msg := fmt.Sprintf("failed to build request: %v", err)
		result.ErrorMessage = &msg
		return result
	}
	req.Header.Set("User-Agent", "PulseWatch/1.0 (+https://github.com/pulsewatch)")
	req.Header.Set("Accept", "*/*")

	start := time.Now()
	resp, err := c.client.Do(req)
	elapsed := time.Since(start).Milliseconds()
	result.ResponseTimeMs = &elapsed

	if err != nil {
		msg := err.Error()
		result.ErrorMessage = &msg
		return result
	}
	defer resp.Body.Close()

	// Drain a limited amount of the body so connections can be reused,
	// without allocating unbounded memory for large responses.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBody))

	code := resp.StatusCode
	result.StatusCode = &code
	result.IsUp = code >= 200 && code < 400
	if !result.IsUp {
		msg := "HTTP " + strconv.Itoa(code)
		result.ErrorMessage = &msg
	}
	return result
}

// NewTestChecker creates a Checker that talks to local httptest servers.
// It MUST only be used in tests — production always uses NewChecker.
// SSRF URL validation still runs; tests pass URLs that ValidatePublicURL
// accepts when we temporarily skip host checks via CheckWithClient, or
// use httptest with a checker that skips ValidatePublicURL for the host.
func NewTestChecker(timeout time.Duration) *Checker {
	if timeout <= 0 {
		timeout = DefaultHTTPTimeout
	}
	return &Checker{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// CheckRaw performs a check without SSRF host validation — tests only.
func (c *Checker) CheckRaw(ctx context.Context, monitorID, rawURL string) models.CheckResult {
	result := models.CheckResult{
		MonitorID: monitorID,
		CheckedAt: time.Now().UTC(),
		IsUp:      false,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		msg := fmt.Sprintf("failed to build request: %v", err)
		result.ErrorMessage = &msg
		return result
	}
	req.Header.Set("User-Agent", "PulseWatch/1.0-test")

	start := time.Now()
	resp, err := c.client.Do(req)
	elapsed := time.Since(start).Milliseconds()
	result.ResponseTimeMs = &elapsed

	if err != nil {
		msg := err.Error()
		result.ErrorMessage = &msg
		return result
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBody))

	code := resp.StatusCode
	result.StatusCode = &code
	result.IsUp = code >= 200 && code < 400
	if !result.IsUp {
		msg := "HTTP " + strconv.Itoa(code)
		result.ErrorMessage = &msg
	}
	return result
}
