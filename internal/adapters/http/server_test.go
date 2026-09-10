package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutes(t *testing.T) {
	h, err := NewHandler()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	get := func(p string) (*http.Response, string) {
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		defer resp.Body.Close()
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		return resp, sb.String()
	}

	resp, body := get("/api/health")
	if resp.StatusCode != 200 {
		t.Fatalf("health status %d", resp.StatusCode)
	}
	var health map[string]string
	if err := json.Unmarshal([]byte(body), &health); err != nil || health["status"] != "ok" {
		t.Fatalf("health body %q err=%v", body, err)
	}

	resp, body = get("/api/agents")
	if resp.StatusCode != 200 {
		t.Fatalf("agents status %d", resp.StatusCode)
	}
	var list []agentView
	if err := json.Unmarshal([]byte(body), &list); err != nil || len(list) != 7 {
		t.Fatalf("agents body %q err=%v", body, err)
	}

	resp, body = get("/")
	if resp.StatusCode != 200 || !strings.Contains(body, "<html") {
		t.Fatalf("index status %d body %q", resp.StatusCode, body)
	}

	resp, body2 := get("/some/client/route")
	if resp.StatusCode != 200 || body2 != body {
		t.Fatalf("SPA fallback status %d, body differs=%v", resp.StatusCode, body2 != body)
	}

	resp, _ = get("/api/missing")
	if resp.StatusCode != 404 {
		t.Fatalf("unknown api route status %d, want 404", resp.StatusCode)
	}
}
