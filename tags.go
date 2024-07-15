// Make go install fail if wrong tags are set

//go:build cgo || !no_net || !no_json
// +build cgo !no_net !no_json

package main

import "fmt"

// Indicate proper install if tags were missing or cgo is used.
func init() {
	fmt.Println(`##############
INSTALL/BUILD ERROR: this file should not be built with cgo or without no_net or no_json tags, please re
install using

CGO_ENABLED=0 go install -trimpath -ldflags="-w -s" -tags no_net,no_json grol.io/grol@latest

##############`)
}
