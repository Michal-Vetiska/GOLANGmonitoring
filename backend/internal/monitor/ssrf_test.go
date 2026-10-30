package monitor

import (
	"net"
	"testing"
)

func TestValidatePublicURL_AllowsHTTPS(t *testing.T) {
	u, err := ValidatePublicURL("https://example.com/health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Host != "example.com" {
		t.Fatalf("host = %q", u.Host)
	}
}

func TestValidatePublicURL_RejectsLocalhost(t *testing.T) {
	cases := []string{
		"http://localhost/x",
		"http://127.0.0.1/x",
		"http://[::1]/x",
		"http://0.0.0.0/x",
		"ftp://example.com/x",
		"https://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://172.16.5.5/",
	}
	for _, c := range cases {
		if _, err := ValidatePublicURL(c); err == nil {
			t.Errorf("expected rejection for %q", c)
		}
	}
}

func TestIsPublicIP(t *testing.T) {
	privates := []string{"127.0.0.1", "::1", "10.1.2.3", "192.168.0.1", "172.16.0.1", "169.254.1.1", "100.64.1.1", "224.0.0.1"}
	for _, s := range privates {
		if isPublicIP(net.ParseIP(s)) {
			t.Errorf("%s should not be public", s)
		}
	}
	if !isPublicIP(net.ParseIP("8.8.8.8")) {
		t.Error("8.8.8.8 should be public")
	}
	if !isPublicIP(net.ParseIP("1.1.1.1")) {
		t.Error("1.1.1.1 should be public")
	}
}
