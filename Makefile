.PHONY: run-backend run-frontend build lint check test sample-media clean

# dev database (compose service `db` published on 5432)
DEV_DB ?= postgres://couchverse:couchverse@localhost:5432/couchverse

run-backend:
	cd backend && DATABASE_URL=$(DEV_DB) DATA_DIR=../data \
		ADMIN_USERNAME=admin ADMIN_PASSWORD=admin \
		go run ./cmd/couchverse

run-frontend:
	cd frontend && npm run dev

build:
	cd frontend && npm run build
	rm -rf backend/web/dist && mkdir -p backend/web/dist
	cp -R frontend/build/. backend/web/dist/
	touch backend/web/dist/.gitkeep
	cd backend && go build -ldflags="-s -w" -o bin/couchverse ./cmd/couchverse

lint:
	cd backend && go vet ./...
	@command -v golangci-lint >/dev/null && (cd backend && golangci-lint run) || echo "golangci-lint not installed, ran go vet only"

check:
	cd frontend && npm run check
	cd frontend && npx prettier --check src

format:
	cd frontend && npx prettier --write src
	cd backend && gofmt -w .

test:
	cd backend && go test ./...

sample-media:
	./scripts/gen-sample-media.sh data/samples

clean:
	rm -rf backend/bin frontend/build frontend/.svelte-kit
