// hvo-demo is the smallest useful workload for exercising HVO's platform
// delivery: this repo holds only source code and hvo.yaml; HVO builds it
// with Buildpacks and releases it to dev, then prod after one approval.
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// version is set at build time (-ldflags "-X main.version=...").
var version = "dev"

// database reports whether DATABASE_URL (from hvo.yaml needs.postgres) is
// set and its server accepts connections.
func database() string {
	raw := os.Getenv("DATABASE_URL")
	if raw == "" {
		return "no DATABASE_URL"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "bad DATABASE_URL"
	}
	conn, err := net.DialTimeout("tcp", u.Host, 2*time.Second)
	if err != nil {
		return fmt.Sprintf("%s unreachable: %v", u.Host, err)
	}
	conn.Close()
	return fmt.Sprintf("%s%s reachable", u.Host, u.Path)
}

// cache reports whether REDIS_URL (from hvo.yaml needs.redis) answers a
// PING.
func cache() string {
	raw := os.Getenv("REDIS_URL")
	if raw == "" {
		return "no REDIS_URL"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "bad REDIS_URL"
	}
	conn, err := net.DialTimeout("tcp", u.Host, 2*time.Second)
	if err != nil {
		return fmt.Sprintf("%s unreachable: %v", u.Host, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("PING\r\n")); err != nil {
		return fmt.Sprintf("%s: %v", u.Host, err)
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Sprintf("%s: %v", u.Host, err)
	}
	return fmt.Sprintf("%s answered %q", u.Host, strings.TrimSpace(string(buf[:n])))
}

func main() {
	host, _ := os.Hostname()
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "hvo-demo %s on %s (run 8, with a cache)\n", version, host)
		fmt.Fprintf(w, "greeting: %s\n", os.Getenv("GREETING"))
		fmt.Fprintf(w, "database: %s\n", database())
		fmt.Fprintf(w, "cache: %s\n", cache())
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	log.Printf("hvo-demo %s listening on :8080", version)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
