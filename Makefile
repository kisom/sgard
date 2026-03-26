VERSION := $(shell git describe --tags --abbrev=0 | sed 's/^v//')

.PHONY: proto build test lint clean version

proto:
	protoc \
		--go_out=. --go_opt=module=github.com/kisom/sgard \
		--go-grpc_out=. --go-grpc_opt=module=github.com/kisom/sgard \
		-I proto \
		proto/sgard/v1/sgard.proto

version:
	@echo $(VERSION) > VERSION

build:
	go build -ldflags "-X main.version=$(VERSION)" ./...

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f sgard
