package protocol

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestNewUpgraderAllowsSameOriginBrowserConnection(t *testing.T) {
	t.Parallel()

	server := newTestWebSocketServer(t, UpgraderConfig{})
	defer server.Close()

	conn, resp, err := dialWebSocket(server.URL, server.URL)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("expected same-origin handshake to succeed, status=%d err=%v", status, err)
	}
	defer conn.Close()
}

func TestNewUpgraderRejectsForeignOriginByDefault(t *testing.T) {
	t.Parallel()

	server := newTestWebSocketServer(t, UpgraderConfig{})
	defer server.Close()

	conn, resp, err := dialWebSocket(server.URL, "https://evil.example")
	if conn != nil {
		conn.Close()
		t.Fatal("expected foreign origin handshake to fail")
	}
	if err == nil {
		t.Fatal("expected foreign origin handshake to return an error")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403 for foreign origin, got %+v", resp)
	}
}

func TestNewUpgraderAllowsConfiguredCrossOriginFrontend(t *testing.T) {
	t.Parallel()

	const frontendOrigin = "https://frontend.example"

	server := newTestWebSocketServer(t, UpgraderConfig{
		AllowedOrigins: []string{frontendOrigin},
	})
	defer server.Close()

	conn, resp, err := dialWebSocket(server.URL, frontendOrigin)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("expected allowlisted origin handshake to succeed, status=%d err=%v", status, err)
	}
	defer conn.Close()
}

func newTestWebSocketServer(t *testing.T, cfg UpgraderConfig) *httptest.Server {
	t.Helper()

	upgrader := NewUpgrader(cfg)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("ok"))
	}))
}

func dialWebSocket(httpURL, origin string) (*websocket.Conn, *http.Response, error) {
	wsURL := "ws" + strings.TrimPrefix(httpURL, "http")
	header := http.Header{}
	if origin != "" {
		header.Set("Origin", origin)
	}
	return websocket.DefaultDialer.Dial(wsURL, header)
}
