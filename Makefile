.PHONY: run-backend run-web build lint check format test contract sample-media clean \
	core-test core-apple core-android core-wasm apple-test android-test

# dev database (compose service `db` published on 5432)
DEV_DB ?= postgres://couchverse:couchverse@localhost:5432/couchverse
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X couchverse/internal/version.Version=$(VERSION)

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
	cd backend && go build -ldflags="$(LDFLAGS)" -o bin/couchverse ./cmd/couchverse

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

# API conformance tests create throwaway databases on the dev Postgres
test:
	cd backend && TEST_DATABASE_URL=$(DEV_DB) go test ./...

# regenerate everything derived from contract/ (and the spec itself from the Go handlers)
contract:
	cd backend && go run ./cmd/couchverse openapi > ../contract/openapi.json
	cd backend && go run ./cmd/couchverse couch-schema > ../contract/couch-protocol.schema.json
	cd core && cargo xtask codegen

core-test:
	cd core && cargo fmt --check
	cd core && cargo clippy --workspace --all-targets -- -D warnings
	cd core && cargo test --workspace

# the core packaged for each shell (build output, gitignored)
core-apple:
	cd core && cargo xtask apple

core-android:
	cd core && cargo xtask android

core-wasm:
	cd core && cargo xtask wasm

apple-test: core-apple
	cd clients/apple/Packages/CouchverseCore && swift test

android-test: core-android
	cd clients/android && ./gradlew :core:testDebugUnitTest

sample-media:
	./scripts/gen-sample-media.sh data/samples

clean:
	rm -rf backend/bin clients/web/build clients/web/.svelte-kit
