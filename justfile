version := `git describe --tags --always --dirty 2>/dev/null || echo dev`

build:
    go build -ldflags "-X main.version={{version}}" -o bin/synd ./cmd/synd/

install: build
    cp bin/synd ~/.local/bin/synd
    @echo "Installed synd {{version}}"

test:
    go vet ./...
    go test ./...
