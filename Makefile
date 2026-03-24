.PHONY: proto build test lint clean

proto:
	protoc \
		--go_out=. --go_opt=module=github.com/kisom/sgard \
		--go-grpc_out=. --go-grpc_opt=module=github.com/kisom/sgard \
		-I proto \
		proto/sgard/v1/sgard.proto

build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f sgard
