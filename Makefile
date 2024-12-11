all: generate lint check test run

GO_BUILD_TAGS:=no_net,no_pprof

#GROL_FLAGS:=-no-register

run: grol
	# Interactive debug run: use logger with file and line numbers
	LOGGER_IGNORE_CLI_MODE=true GOMEMLIMIT=1GiB ./grol -panic -parse -loglevel debug $(GROL_FLAGS)

GEN:=object/type_string.go ast/priority_string.go token/type_string.go

grol: Makefile *.go */*.go $(GEN)
	CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -tags "$(GO_BUILD_TAGS)" .
	ls -lh grol

tinygo-tests: Makefile *.go */*.go $(GEN)
	CGO_ENABLED=0 tinygo test $(TINYGO_STACKS) -tags "$(GO_BUILD_TAGS)" -v ./...

TINY_TEST_PACKAGE:=./ast
tiny_test:
	# Make a binary that can be debugged, use
	# make TINY_TEST_PACKAGE=.
	# to set the package to test to . for instance
	-rm -f $@
	CGO_ENABLED=0 tinygo test -tags "no_net,no_json" -c -o $@ -x $(TINY_TEST_PACKAGE)
	./tiny_test -test.v

tinygo: Makefile *.go */*.go $(GEN)
	CGO_ENABLED=0 tinygo build -o grol.tiny $(TINYGO_STACKS) -tags "$(GO_BUILD_TAGS)" .
	strip grol.tiny
	ls -lh grol.tiny

parser-test:
	LOGGER_LOG_FILE_AND_LINE=false LOGGER_IGNORE_CLI_MODE=true LOGGER_LEVEL=debug go test \
		-v -run '^TestFormat$$' ./parser | logc

TINYGO_STACKS:=-stack-size=40mb

wasm: Makefile *.go */*.go $(GEN) wasm/wasm_exec.js wasm/wasm_exec.html wasm/grol_wasm.html
#	GOOS=wasip1 GOARCH=wasm go build -o grol.wasm -trimpath -ldflags="-w -s" -tags "$(GO_BUILD_TAGS)" .
	GOOS=js GOARCH=wasm $(WASM_GO) build -o wasm/grol.wasm -trimpath -ldflags="-w -s" -tags "$(GO_BUILD_TAGS)" ./wasm
#	GOOS=wasip1 GOARCH=wasm tinygo build -target=wasi -no-debug -o grol_tiny.wasm -tags "$(GO_BUILD_TAGS)" .
# Tiny go generates errors https://github.com/tinygo-org/tinygo/issues/1140
#	GOOS=js GOARCH=wasm tinygo build $(TINYGO_STACKS) -no-debug -o wasm/grol.wasm -tags "$(GO_BUILD_TAGS)" ./wasm
	echo '<!doctype html><html><head><meta charset="utf-8"><title>Grol</title></head>' > wasm/index.html
	cat wasm/grol_wasm.html >> wasm/index.html
	echo '</html>' >> wasm/index.html
	-ls -lh wasm/*.wasm
	-pkill wasm
	go run ./wasm ./wasm &
	sleep 3
	open http://localhost:8080/


#WASM_GO:=/opt/homebrew/Cellar/go/1.23.1/bin/go
WASM_GO:=go

GIT_TAG=$(shell git describe --tags --always --dirty)
# used to copy to site a release version
wasm-release: Makefile *.go */*.go $(GEN) wasm/wasm_exec.js wasm/wasm_exec.html
	@echo "Building wasm release GIT_TAG=$(GIT_TAG)"
	GOOS=js GOARCH=wasm $(WASM_GO) install -trimpath -ldflags="-w -s" -tags "$(GO_BUILD_TAGS)" grol.io/grol/wasm@$(GIT_TAG)
	# No buildinfo and no tinygo install so we set version old style:
#	GOOS=js GOARCH=wasm tinygo build $(TINYGO_STACKS) -o wasm/grol.wasm -no-debug -ldflags="-X main.TinyGoVersion=$(GIT_TAG)" -tags  "$(GO_BUILD_TAGS)" ./wasm
	mv "$(shell go env GOPATH)/bin/js_wasm/wasm" wasm/grol.wasm
	ls -lh wasm/*.wasm

install:
	CGO_ENABLED=0 go install -trimpath -ldflags="-w -s" -tags "$(GO_BUILD_TAGS)" grol.io/grol@$(GIT_TAG)
	ls -lh "$(shell go env GOPATH)/bin/grol"
	grol version

wasm/wasm_exec.js: Makefile
#	cp "$(shell tinygo env TINYGOROOT)/targets/wasm_exec.js" ./wasm/
	cp "$(shell $(WASM_GO) env GOROOT)/misc/wasm/wasm_exec.js" ./wasm/

wasm/wasm_exec.html:
	cp "$(shell $(WASM_GO) env GOROOT)/misc/wasm/wasm_exec.html" ./wasm/

test: grol unit-tests examples grol-tests

unit-tests:
	CGO_ENABLED=0 go test -tags $(GO_BUILD_TAGS) ./...

examples: grol
	GOMEMLIMIT=1GiB ./grol -panic $(GROL_FLAGS) examples/*.gr

grol-tests: grol
	GOMEMLIMIT=1GiB ./grol -panic -shared-state $(GROL_FLAGS) tests/*.gr

check: grol
	./check_samples_double_format.sh examples/*.gr
	./check_tests_double_format.sh

generate: $(GEN)

object/type_string.go: object/object.go
	go generate ./object # if this fails go install golang.org/x/tools/cmd/stringer@latest

ast/priority_string.go: ast/ast.go
	go generate ./ast

token/type_string.go: token/token.go
	go generate ./token


clean:
	rm -f grol */*_string.go *.wasm wasm/*.wasm wasm/wasm_exec.html wasm/wasm_exec.js

build: grol

lint: .golangci.yml
	CGO_ENABLED=0 golangci-lint run --build-tags $(GO_BUILD_TAGS)

.golangci.yml: Makefile
	curl -fsS -o .golangci.yml https://raw.githubusercontent.com/fortio/workflows/main/golangci.yml

.PHONY: all lint generate test clean run build wasm tinygo wasm-release tiny_test tinygo-tests check install unit-tests examples grol-tests
