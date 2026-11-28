// hvo-demo is the smallest useful workload for exercising HVO's platform
// delivery: this repo holds only source code and hvo.yaml; HVO builds it
// with Buildpacks and releases it to dev, then prod after one approval.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"html/template"
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
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
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
	// HVO's Redis requires the password in REDIS_URL: authenticate first
	// (RESP array, so the password needs no escaping), then PING.
	cmd := "PING\r\n"
	if pw, ok := u.User.Password(); ok {
		cmd = fmt.Sprintf("*2\r\n$4\r\nAUTH\r\n$%d\r\n%s\r\n", len(pw), pw) + cmd
	}
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return fmt.Sprintf("%s: %v", u.Host, err)
	}
	r := bufio.NewReader(conn)
	var reply string
	for range strings.Count(cmd, "\r\n$4\r\nAUTH") + 1 { // AUTH reply, then PING's
		line, err := r.ReadString('\n')
		if err != nil {
			return fmt.Sprintf("%s: %v", u.Host, err)
		}
		reply = strings.TrimSpace(line)
		if strings.HasPrefix(reply, "-") {
			break // an error (e.g. wrong password) is the answer to show
		}
	}
	return fmt.Sprintf("%s answered %q", u.Host, reply)
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

var tracer = otel.Tracer("hvo-demo")

// indexPage is deliberately small: HVO owns the browser telemetry code and
// all scope injection; this app only passes the public config HVO injected
// as environment variables. html/template escapes values in HTML and JS.
var indexPage = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>hvo-demo</title>
{{if .RUMToken}}
  <script>
    window.HVO_RUM = { token: "{{.RUMToken}}", endpoint: "{{.RUMEndpoint}}"{{if .SessionsKey}}, sessions: { projectKey: "{{.SessionsKey}}", ingestPoint: location.origin + "/ingest" }{{end}} };
  </script>
  <script src="{{.RUMScript}}" defer></script>
{{end}}
</head>
<body>
<pre>
hvo-demo {{.Version}} on {{.Host}} (run 12, traced)
greeting: {{.Greeting}}
sealed: {{.Sealed}}
database: {{.Database}}
cache: {{.Cache}}
storage: {{.Storage}}
</pre>
</body>
</html>
`))

type pageData struct {
	Version, Host                    string
	Greeting, Sealed                 string
	Database, Cache, Storage         string
	RUMToken, RUMEndpoint, RUMScript string
	SessionsKey                      string
}

func index(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	host, _ := os.Hostname()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexPage.Execute(w, pageData{
		Version:     version,
		Host:        host,
		Greeting:    os.Getenv("GREETING"),
		Sealed:      os.Getenv("SEALED_GREETING"),
		Database:    traced(ctx, "check database", database),
		Cache:       traced(ctx, "check cache", cache),
		Storage:     traced(ctx, "check storage", storage),
		RUMToken:    os.Getenv("HVO_RUM_TOKEN"),
		RUMEndpoint: os.Getenv("HVO_RUM_COLLECT_URL"),
		RUMScript:   os.Getenv("HVO_RUM_SCRIPT_URL"),
		SessionsKey: os.Getenv("OPENREPLAY_PROJECT_KEY"),
	}); err != nil {
		log.Printf("render index: %v", err)
	}
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", index)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// /fail records an error, for trying the platform's error views.
	mux.HandleFunc("/fail", func(w http.ResponseWriter, req *http.Request) {
		_, span := tracer.Start(req.Context(), "charge card")
		err := errors.New("payment provider timed out")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		log.Printf("error: %v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
	})
	return mux
}

// tracing exports spans when the platform sets OTEL_EXPORTER_OTLP_ENDPOINT
// (HVO does for every app); service name, environment and version come
// from OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES.
func tracing(ctx context.Context) func() {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return func() {}
	}
	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		log.Printf("tracing off: %v", err)
		return func() {}
	}
	res, _ := resource.New(ctx, resource.WithFromEnv(), resource.WithTelemetrySDK())
	tp := trace.NewTracerProvider(trace.WithBatcher(exp), trace.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return func() { _ = tp.Shutdown(context.Background()) }
}

// traced runs check in a child span of the request.
func traced(ctx context.Context, name string, check func() string) string {
	_, span := tracer.Start(ctx, name)
	defer span.End()
	return check()
}

func main() {
	defer tracing(context.Background())()
	log.Printf("hvo-demo %s listening on :8080", version)
	log.Fatal(http.ListenAndServe(":8080", otelhttp.NewHandler(newMux(), "http", otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/healthz" }))))
}
