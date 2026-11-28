package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIndexLoadsHVOBrowserBootstrap(t *testing.T) {
	token := "hvor_team-acme-dev_hvo-demo_abc123_secret"
	endpoint := "https://api.nalyx.dev/api/tenant/v1/rum/collect"
	script := "https://hvo.think-lab.opscale.ir/hvo-rum.js"
	projectKey := "public-project-key"
	t.Setenv("HVO_RUM_TOKEN", token)
	t.Setenv("HVO_RUM_COLLECT_URL", endpoint)
	t.Setenv("HVO_RUM_SCRIPT_URL", script)
	t.Setenv("OPENREPLAY_PROJECT_KEY", projectKey)
	t.Setenv("SEALED_GREETING", "</pre><script>alert(1)</script>")

	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content type = %q", ct)
	}
	body := rec.Body.String()
	// html/template escapes forward slashes in JavaScript string contexts;
	// the browser sees the original endpoint.
	jsEndpoint := strings.ReplaceAll(endpoint, "/", `\/`)
	for _, want := range []string{"window.HVO_RUM", token, jsEndpoint, projectKey, `ingestPoint: location.origin + "/ingest"`, `<script src="` + script + `" defer></script>`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatal("app value was not HTML-escaped")
	}
}

func TestIndexOmitsBootstrapWithoutConfig(t *testing.T) {
	t.Setenv("HVO_RUM_TOKEN", "")
	t.Setenv("HVO_RUM_COLLECT_URL", "")
	t.Setenv("HVO_RUM_SCRIPT_URL", "")
	t.Setenv("OPENREPLAY_PROJECT_KEY", "")

	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "HVO_RUM") {
		t.Fatal("bootstrap loaded without public config")
	}
}

func TestHealthAndFailRoutesStayAvailable(t *testing.T) {
	mux := newMux()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/fail", nil))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("fail = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "payment provider timed out") {
		t.Fatalf("fail body = %q", rec.Body.String())
	}
}

// HVO's Redis needs the password from REDIS_URL: the check authenticates
// before PING instead of reporting NOAUTH.
func TestCacheAuthenticates(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 256)
		n, _ := c.Read(buf)
		got <- string(buf[:n])
		_, _ = c.Write([]byte("+OK\r\n+PONG\r\n"))
	}()
	t.Setenv("REDIS_URL", "redis://:s3cr%40t@"+ln.Addr().String())
	if out := cache(); !strings.HasSuffix(out, `answered "+PONG"`) {
		t.Fatalf("cache() = %q", out)
	}
	if req := <-got; req != "*2\r\n$4\r\nAUTH\r\n$6\r\ns3cr@t\r\nPING\r\n" {
		t.Errorf("sent %q", req)
	}
}
