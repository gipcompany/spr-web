VERSION ?= dev

.PHONY: build backend frontend test test-go test-web dev-web clean

# Full build: frontend first (go:embed requires web/dist), then the binary.
build: frontend backend

backend:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/spr-web ./cmd/spr-web

frontend:
	cd web && pnpm install --frozen-lockfile && pnpm build

test: test-go test-web

test-go:
	go vet ./...
	go test ./...

test-web:
	cd web && pnpm typecheck && pnpm exec biome ci .

# Development: run the Go backend inside a spr-configured repository
# (`go run ./cmd/spr-web --no-open`), then start the Vite dev server here;
# /api requests are proxied to port 7780.
dev-web:
	cd web && pnpm dev

clean:
	rm -rf bin web/dist/*
	touch web/dist/.gitkeep
