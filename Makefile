all: generate lint test run

GO_BUILD_TAGS:=no_net,no_json

run: grol
	./grol -parse

GEN:=object/type_string.go parser/priority_string.go token/type_string.go

grol: Makefile *.go */*.go $(GEN)
	CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -tags "$(GO_BUILD_TAGS)" .
	ls -lh grol

tinygo: Makefile *.go */*.go $(GEN) wasm/wasm_exec.js wasm/wasm_exec.html
	CGO_ENABLED=0 tinygo build -o grol.tiny -tags "$(GO_BUILD_TAGS)" .
	strip grol.tiny
	ls -lh grol.tiny

wasm: Makefile *.go */*.go $(GEN) wasm/wasm_exec.js wasm/wasm_exec.html
#	GOOS=wasip1 GOARCH=wasm go build -o grol.wasm -trimpath -ldflags="-w -s" -tags "$(GO_BUILD_TAGS)" .
	GOOS=js GOARCH=wasm go build -o wasm/grol.wasm -trimpath -ldflags="-w -s" -tags "$(GO_BUILD_TAGS)" ./wasm
#	GOOS=wasip1 GOARCH=wasm tinygo build -target=wasi -no-debug -o grol_tiny.wasm -tags "$(GO_BUILD_TAGS)" .
# Tiny go generates errors https://github.com/tinygo-org/tinygo/issues/1140
# GOOS=js GOARCH=wasm tinygo build -no-debug -o wasm/test.wasm -tags "$(GO_BUILD_TAGS)" ./wasm
	-ls -lh wasm/*.wasm
	-pkill wasm
	go run ./wasm ./wasm &
	sleep 3
	open http://localhost:8080/

GIT_TAG:=$(shell git describe --tags --abbrev=0)
# used to copy to site a release version
wasm-release: Makefile *.go */*.go $(GEN) wasm/wasm_exec.js wasm/wasm_exec.html
	@echo "Building wasm release GIT_TAG=$(GIT_TAG)"
	GOOS=js GOARCH=wasm go install -trimpath -ldflags="-w -s" -tags "$(GO_BUILD_TAGS)" grol.io/grol/wasm@$(GIT_TAG)
	mv "$(shell go env GOPATH)/bin/js_wasm/wasm" wasm/grol.wasm
	ls -lh wasm/*.wasm

wasm/wasm_exec.js: Makefile
#	cp "$(shell tinygo env TINYGOROOT)/targets/wasm_exec.js" ./wasm/
	cp "$(shell tinygo env GOROOT)/misc/wasm/wasm_exec.js" ./wasm/

wasm/wasm_exec.html:
	cp "$(shell go env GOROOT)/misc/wasm/wasm_exec.html" ./wasm/

test: grol
	CGO_ENABLED=0 go test -tags $(GO_BUILD_TAGS) ./...
	./grol *.gr

failing-tests:
	-go test -v ./... -tags=runfailingtests -run TestLetStatementsFormerlyCrashingNowFailingOnPurpose

generate:
	go generate ./... # if this fails go install golang.org/x/tools/cmd/stringer@latest

generate: $(GEN)

object/type_string.go: object/object.go
	go generate ./object

parser/priority_string.go: parser/parser.go
	go generate ./parser

token/type_string.go: token/token.go
	go generate ./token


clean:
	rm -f grol */*_string.go *.wasm wasm/*.wasm wasm/wasm_exec.html wasm/wasm_exec.js

build: grol

lint: .golangci.yml
	CGO_ENABLED=0 golangci-lint run --build-tags $(GO_BUILD_TAGS)

.golangci.yml: Makefile
	curl -fsS -o .golangci.yml https://raw.githubusercontent.com/fortio/workflows/main/golangci.yml

.PHONY: all lint generate test clean run build wasm tinygo failing-tests wasm-release
