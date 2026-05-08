VERSION ?= dev

.PHONY: build ui types lint test release clean

build: ui
	go build -ldflags="-X main.version=$(VERSION) -s -w" -o dist/sigilbridge ./cmd/sigilbridge

ui:
	@if [ -f ui/package.json ]; then cd ui && pnpm install --frozen-lockfile && pnpm run build; else echo "ui/package.json not present yet; skipping UI build"; fi
	@if [ -d ui/dist ]; then rm -rf internal/admin/ui/dist && mkdir -p internal/admin/ui/dist && cp -R ui/dist/. internal/admin/ui/dist/; fi
	@if command -v gzip >/dev/null 2>&1 && [ -d ui/dist ]; then find ui/dist -type f \( -name '*.js' -o -name '*.css' -o -name '*.html' \) -exec gzip -9kf {} \;; fi
	@if command -v brotli >/dev/null 2>&1 && [ -d ui/dist ]; then find ui/dist -type f \( -name '*.js' -o -name '*.css' -o -name '*.html' \) -exec brotli -q 11 -kf {} \;; fi

types:
	go run ./cmd/gentypes

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./...; else go vet ./...; fi
	@if [ -f ui/package.json ]; then cd ui && pnpm run lint; else echo "ui/package.json not present yet; skipping UI lint"; fi

test:
	go test -race -coverprofile=coverage.out ./...
	@if [ -f ui/package.json ]; then cd ui && pnpm run test; else echo "ui/package.json not present yet; skipping UI tests"; fi

release:
	bash scripts/release.sh

clean:
	rm -rf dist/ ui/dist/ ui/.lighthouseci/ ui/test-results/ .lighthouseci/ data/ backup/ audit/ examples/data/ examples/backup/ examples/audit/
	rm -f coverage.out sigilbridge sigilbridge.exe
