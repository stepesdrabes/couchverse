.PHONY: run-backend run-web build lint check format test contract sample-media clean

# dev database (compose service `db` published on 5432)
DEV_DB ?= postgres://couchverse:couchverse@localhost:5432/couchverse

run-backend:
	cd backend && DATABASE_URL=$(DEV_DB) DATA_DIR=../data \
		ADMIN_USERNAME=admin ADMIN_PASSWORD=admin \
		go run ./cmd/couchverse

run-web:
	cd clients/web && npm run dev

build:
	cd clients/web && npm run build
	rm -rf backend/web/dist && mkdir -p backend/web/dist
	cp -R clients/web/build/. backend/web/dist/
	touch backend/web/dist/.gitkeep
	cd backend && go build -ldflags="-s -w" -o bin/couchverse ./cmd/couchverse

lint:
	cd backend && go vet ./...
	@if command -v golangci-lint >/dev/null; then cd backend && golangci-lint run; \
	else echo "golangci-lint not installed, ran go vet only"; fi

check:
	cd clients/web && npm run check
	cd clients/web && npm run lint

format:
	cd clients/web && npx prettier --write src
	cd backend && gofmt -w .

test:
	cd backend && go test ./...

# regenerate everything derived from contract/ (and the spec itself from the Go handlers)
contract:
	cd backend && go run ./cmd/couchverse openapi > ../contract/openapi.json
	cd core && cargo xtask codegen

sample-media:
	./scripts/gen-sample-media.sh data/samples

clean:
	rm -rf backend/bin clients/web/build clients/web/.svelte-kit
