#.PHONY all build bake push login

VERSION?=v0.0.1
MODULE=github.com/mmlt/kcertwatch


all: build test release

build: 
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.Version=$(VERSION)" .

lint:
	golangci-lint run

test: testunit

testunit:
	./hack/test.sh

release:
	./hack/release.sh $(VERSION) $(VERSION)

