.PHONY: all unit e2e bin tests

bin:
	go build -ldflags="-X main.version=$(shell git describe --always --long --dirty)"

unit:
	go test -race $(shell go list ./... | grep -v e2e)

e2e: bin
	./hobson -config e2e/fixtures/hobson.yaml &
	go test -race -count=1 -p 1 -v $(shell go list ./... | grep e2e)
	pkill -kill hobson

vet:
	go vet ./...

tests: vet unit e2e

all: unit e2e bin
