all: generate lint tests run

GO_BUILD_TAGS:=no_net,no_json

run: gorepl
	./gorepl -parse

GEN:=object/type_string.go parser/priority_string.go token/type_string.go

gorepl: Makefile *.go */*.go $(GEN)
	CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -tags "$(GO_BUILD_TAGS)" .
	ls -l gorepl

tests:
	CGO_ENABLED=0 go test -race -tags $(GO_BUILD_TAGS) ./...

failing-tests:
	-go test -v ./... -tags=runfailingtests -run TestLetStatementsFormerlyCrashingNowFailingOnPurpose

test: tests

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
	rm -f gorepl */*_string.go

build: gorepl

lint: .golangci.yml
	CGO_ENABLED=0 golangci-lint run --build-tags $(GO_BUILD_TAGS)

.golangci.yml: Makefile
	curl -fsS -o .golangci.yml https://raw.githubusercontent.com/fortio/workflows/main/golangci.yml

.PHONY: all lint generate tests test clean run build
