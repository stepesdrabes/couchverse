.PHONY: run-backend run-web build lint check format test e2e contract sample-media clean \
	core-test core-apple core-android core-wasm apple-test android-test \
	hls-check hls-apple ingest-samples hls-server-check e2e-playback

# dev database (compose service `db` published on 5432)
DEV_DB ?= postgres://couchverse:couchverse@localhost:5432/couchverse
# a running server for the checks that drive one (ingest-samples, hls-server-check, e2e-playback)
SERVER ?= http://localhost:8080
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X couchverse/internal/version.Version=$(VERSION)
# the wasm core the web runs (build output); dev targets build it once, `make core-wasm` refreshes it
WEB_CORE := clients/web/src/lib/core/pkg

run-backend:
	cd backend && DATABASE_URL=$(DEV_DB) DATA_DIR=../data \
		ADMIN_USERNAME=admin ADMIN_PASSWORD=admin \
		go run ./cmd/couchverse

run-web: $(WEB_CORE)
	cd clients/web && npm run dev

$(WEB_CORE):
	cd core && cargo xtask wasm

build: core-wasm
	cd clients/web && npm run build
	rm -rf backend/web/dist && mkdir -p backend/web/dist
	cp -R clients/web/build/. backend/web/dist/
	touch backend/web/dist/.gitkeep
	cd backend && go build -ldflags="$(LDFLAGS)" -o bin/couchverse ./cmd/couchverse

lint:
	cd backend && go vet ./...
	@if command -v golangci-lint >/dev/null; then cd backend && golangci-lint run; \
	else echo "golangci-lint not installed, ran go vet only"; fi

check: $(WEB_CORE)
	cd clients/web && npm run check
	cd clients/web && npm run lint
	cd clients/web && npm test

format:
	cd clients/web && npm run format
	cd backend && gofmt -w .

# API conformance tests create throwaway databases on the dev Postgres
test:
	cd backend && TEST_DATABASE_URL=$(DEV_DB) go test ./...

# Playwright smoke suite: builds the app, serves it on a throwaway seeded database on the dev
# Postgres (scripts/e2e-server.sh) and drives the web client in Chromium, then WebKit
e2e:
	cd clients/web && npx playwright install chromium webkit
	cd clients/web && E2E_DATABASE_URL=$(DEV_DB) npm run e2e

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

# every playback tier on generated media, through the real probe and transcode jobs,
# checked by the HLS validator; on macOS AVFoundation also plays each presentation
hls-check:
	cd backend && COUCHVERSE_AVPLAYER=$(if $(filter Darwin,$(shell uname)),1,0) TEST_DATABASE_URL=$(DEV_DB) \
		go test -count=1 -timeout 30m -run TestPlaybackTiers -v ./internal/server/

# the same with Apple's mediastreamvalidator, for those who installed it
hls-apple:
	cd backend && COUCHVERSE_AVPLAYER=1 COUCHVERSE_APPLE_HLS_TOOLS=1 TEST_DATABASE_URL=$(DEV_DB) \
		go test -count=1 -timeout 30m -run TestPlaybackTiers -v ./internal/server/

# upload data/samples into the server at SERVER (as admin/admin)
ingest-samples:
	SERVER=$(SERVER) ./scripts/ingest-samples.sh data/samples

# every movie of the server at SERVER for every device profile in contract/fixtures
hls-server-check:
	SERVER=$(SERVER) ./scripts/hls-check.sh

# the web player in Chromium against the server at SERVER (CHANNEL=chrome for Google Chrome)
e2e-playback:
	cd clients/web && SERVER=$(SERVER) node e2e/playback.mjs

clean:
	rm -rf backend/bin clients/web/build clients/web/.svelte-kit
	rm -rf clients/web/test-results clients/web/playwright-report
