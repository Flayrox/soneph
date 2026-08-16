# soneph — commandes pratiques
# Usage : make dev | make build | make test | make vet | make fmt

.PHONY: dev build build-go test vet fmt

## dev — lance le backend Go + le dev server Vite (Ctrl+C arrête tout)
dev:
	./scripts/dev.sh

## build-go — build le frontend Vite et le copie dans backend/web/dist (embarqué dans le binaire Go)
build-go:
	cd frontend && npm run build:go

## build — frontend embarqué + binaire Go (dans backend/bin/)
build: build-go
	cd backend && mkdir -p bin && go build -o bin/soneph-server .

## test — tests Go du backend
test:
	cd backend && go test ./...

## vet — go vet du backend
vet:
	cd backend && go vet ./...

## fmt — formatte Go et TypeScript
fmt:
	cd backend && gofmt -w .
	cd frontend && npx tsc --noEmit || true
