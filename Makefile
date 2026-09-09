.PHONY: all build web test vet smoke run run-local

all: test build

build:
	go build -o bin/watchparty ./cmd/watchparty

web:
	cd web && npm install && npm run build

test:
	go test ./...

vet:
	go vet ./...

# Rodar tudo: build + watchparty + túnel (cloudflare por padrão).
# Sem -password o app pergunta a senha no terminal (Enter = sem senha).
TRANSPORT ?= cloudflare
QUALITY ?= 1080p
MAXVIEWERS ?= 3

run: build
	./bin/watchparty share -transport $(TRANSPORT) -quality $(QUALITY) -max-viewers $(MAXVIEWERS)

run-local: build
	./bin/watchparty share -transport local -quality $(QUALITY)

smoke: run-local
