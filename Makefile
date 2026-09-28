.PHONY: test lint build site compose-check security-check

test:
	go test ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...

build:
	CGO_ENABLED=0 go build -trimpath -o dist/yamsplus ./cmd/yamsplus

site:
	cd site && npm ci && npm run build

compose-check:
	go run ./cmd/yamsplus --root ./.yamsplus-test install --config examples/usenet.yaml --skip-start
	docker compose --project-directory ./.yamsplus-test/opt/yamsplus --file ./.yamsplus-test/opt/yamsplus/compose.yaml config --quiet

security-check:
	go vet ./...
	GOTOOLCHAIN=go1.26.5 go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
	GOTOOLCHAIN=go1.26.5 go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
	gitleaks dir . --no-banner --redact --config .gitleaks.toml
