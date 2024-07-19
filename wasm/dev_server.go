//go:build !wasm
// +build !wasm

// Not to be used for anything but localhost testing
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := ":8080"
	path := os.Args[1]
	fmt.Println("Serving", path, "on", port)
	fs := http.FileServer(http.Dir(path))
	log.Fatalf("%v", http.ListenAndServe(port, fs)) //nolint:gosec // just a test server
}
