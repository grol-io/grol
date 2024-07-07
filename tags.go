// Make go install fail if wrong tags are set

//go:build cgo || !no_net || !no_json
// +build cgo !no_net !no_json

package main

// cause an error on purpose if not built with correct tags:
var _ = `
##############

INSTALL/BUILD ERROR: this file should not be built with cgo or without no_net or no_json tags, please re
install using

CGO_ENABLED=0 go install -trimpath -ldflags="-w -s" -tags no_net,no_json github.com/ldemailly/gorepl@latest

##############
`.(int)
