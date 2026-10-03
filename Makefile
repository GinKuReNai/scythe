.PHONY: build analyzer test vet format check
analyzer:
	cd analyzer/typescript && npm ci && npm run build
build: analyzer
	mkdir -p bin/analyzer
	cp -R analyzer/typescript/dist analyzer/typescript/package.json bin/analyzer/
	mkdir -p bin/analyzer/node_modules
	cp -R analyzer/typescript/node_modules/typescript bin/analyzer/node_modules/
	CGO_ENABLED=0 go build -o bin/deadcode ./cmd/deadcode
test: analyzer
	go test ./...
	cd analyzer/typescript && npm test
vet:
	go vet ./...
format:
	gofmt -w cmd internal
	cd analyzer/typescript && npm run format
check: build test vet
	test -z "$$(gofmt -l cmd internal)"
	cd analyzer/typescript && npm run format:check
