# Simple Makefile for go

build:
	go build

# Universal macOS binary (Apple Silicon + Intel), with cgo enabled for SQLite.
build-mac:
	mkdir -p dist
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -trimpath -o dist/whatscli-darwin-arm64 .
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 CC="clang -arch x86_64" go build -trimpath -o dist/whatscli-darwin-amd64 .
	lipo -create -output dist/whatscli dist/whatscli-darwin-arm64 dist/whatscli-darwin-amd64

test:
	go test ./...

clean:
	go clean

run:
	go run .

install:
	go install .

get:
	go get

update:
	go get -u

release:
	./release.sh
