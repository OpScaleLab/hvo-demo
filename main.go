// hvo-demo is the smallest useful workload for exercising an HVO
// ApplicationWorkflow end to end: Tekton builds this repo, HVO pins
// deploy/deployment.yaml to the pushed digest, Argo CD rolls it out.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

// version is set at build time (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	host, _ := os.Hostname()
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "hvo-demo %s on %s\n", version, host)
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	log.Printf("hvo-demo %s listening on :8080", version)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
