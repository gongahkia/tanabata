package main

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPServerTimeouts(t *testing.T) {
	server := newHTTPServer("0", http.NewServeMux())
	if server.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %s, want 5s", server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != 15*time.Second {
		t.Fatalf("ReadTimeout = %s, want 15s", server.ReadTimeout)
	}
	if server.WriteTimeout != 30*time.Second {
		t.Fatalf("WriteTimeout = %s, want 30s", server.WriteTimeout)
	}
	if server.IdleTimeout != 60*time.Second {
		t.Fatalf("IdleTimeout = %s, want 60s", server.IdleTimeout)
	}
}
