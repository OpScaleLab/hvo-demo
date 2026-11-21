// hvo-demo is the smallest useful workload for exercising HVO's platform
// delivery: this repo holds only source code and hvo.yaml; HVO builds it
// with Buildpacks and releases it to dev, then prod after one approval.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

// storage writes an object to the bucket from hvo.yaml needs.bucket and
// reads it back.
func storage() string {
	endpoint, bucket := os.Getenv("S3_ENDPOINT"), os.Getenv("S3_BUCKET")
	if endpoint == "" || bucket == "" {
		return "no bucket"
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "bad S3_ENDPOINT"
	}
	c, err := minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""),
		Secure: u.Scheme == "https",
		Region: os.Getenv("AWS_REGION"),
	})
	if err != nil {
		return err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body := "written " + time.Now().UTC().Format(time.RFC3339)
	if _, err := c.PutObject(ctx, bucket, "hello.txt", strings.NewReader(body), int64(len(body)), minio.PutObjectOptions{}); err != nil {
		return "put: " + err.Error()
	}
	obj, err := c.GetObject(ctx, bucket, "hello.txt", minio.GetObjectOptions{})
	if err != nil {
		return "get: " + err.Error()
	}
	defer obj.Close()
	got, err := io.ReadAll(obj)
	if err != nil {
		return "read: " + err.Error()
	}
	return fmt.Sprintf("%s/hello.txt read back %q", bucket, got)
}

func main() {
	host, _ := os.Hostname()
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "hvo-demo %s on %s (run 11, Keyfold-sealed secret)\n", version, host)
		fmt.Fprintf(w, "greeting: %s\n", os.Getenv("GREETING"))
		fmt.Fprintf(w, "sealed: %s\n", os.Getenv("SEALED_GREETING"))
		fmt.Fprintf(w, "database: %s\n", database())
		fmt.Fprintf(w, "cache: %s\n", cache())
		fmt.Fprintf(w, "storage: %s\n", storage())
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	log.Printf("hvo-demo %s listening on :8080", version)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
