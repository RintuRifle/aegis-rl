package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractIdentity_APIKey(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-API-Key", "my-secret-key")

	identity := extractIdentity(req, true)
	if identity != "key:my-secret-key" {
		t.Errorf("expected 'key:my-secret-key', got %q", identity)
	}
}

func TestExtractIdentity_IPFallback(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	identity := extractIdentity(req, true)
	if identity != "ip:192.168.1.1" {
		t.Errorf("expected 'ip:192.168.1.1', got %q", identity)
	}
}

func TestExtractIdentity_XForwardedForLastHop(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	// Simulates spoofed first hop + real last hop added by proxy
	req.Header.Set("X-Forwarded-For", "spoofed-ip, real-proxy-ip")
	req.RemoteAddr = "10.0.0.1:54321"

	identity := extractIdentity(req, true)
	// Should trust the LAST IP in XFF (the one our proxy added)
	if identity != "ip:real-proxy-ip" {
		t.Errorf("expected 'ip:real-proxy-ip', got %q", identity)
	}
}

func TestExtractIdentity_APIKeyTakesPriority(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-API-Key", "my-key")
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.RemoteAddr = "5.6.7.8:9999"

	identity := extractIdentity(req, true)
	// API key should always win over IP
	if identity != "key:my-key" {
		t.Errorf("expected 'key:my-key', got %q", identity)
	}
}

func TestExtractIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "203.0.113.50")
	req.RemoteAddr = "10.0.0.1:54321"

	ip := extractIP(req, true)
	if ip != "203.0.113.50" {
		t.Errorf("expected '203.0.113.50', got %q", ip)
	}
}

func TestExtractIP_RemoteAddrFallback(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.16.0.1:8080"

	ip := extractIP(req, true)
	if ip != "172.16.0.1" {
		t.Errorf("expected '172.16.0.1', got %q", ip)
	}
}

func TestExtractIP_RemoteAddrNoPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.16.0.1" // no port

	ip := extractIP(req, true)
	if ip != "172.16.0.1" {
		t.Errorf("expected '172.16.0.1', got %q", ip)
	}
}

func TestCORS_SetsHeaders(t *testing.T) {
	handler := CORS("http://localhost:3000")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Error("CORS origin header not set correctly")
	}
}

func TestCORS_HandlesPreflight(t *testing.T) {
	handler := CORS("http://localhost:3000")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for OPTIONS preflight")
	}))

	req := httptest.NewRequest("OPTIONS", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for preflight, got %d", rec.Code)
	}
}

func TestExtractIdentity_IgnoresXFFWithoutTrustedProxy(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	// Attacker on a DIRECT connection tries to rotate identity via fake XFF
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "5.6.7.8")
	req.RemoteAddr = "203.0.113.9:1234"

	identity := extractIdentity(req, false) // trustProxy = false
	if identity != "ip:203.0.113.9" {
		t.Errorf("without a trusted proxy, forwarded headers must be ignored; got %q", identity)
	}
}
