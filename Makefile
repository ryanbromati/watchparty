.PHONY: all build web test vet smoke

all: test build

build:
	go build -o bin/watchparty ./cmd/watchparty

web:
	cd web && npm install && npm run build

test:
	go test ./...

vet:
	go vet ./...

smoke: build
	./bin/watchparty share -transport local -password teste -room dev
