# rommate task runner. Run `just` to list recipes.
#
# This file is the single place where build, test and release commands live:
# the devenv shell, local development and CI all call these recipes.

set shell := ["bash", "-euo", "pipefail", "-c"]

# Build metadata embedded via -ldflags (see cmd/rommate/main.go).
version := env("VERSION", `git describe --tags --always --dirty 2>/dev/null || echo dev`)
commit := `git rev-parse --short HEAD 2>/dev/null || echo unknown`
date := `date -u +%Y-%m-%dT%H:%M:%SZ`
ldflags := "-s -w -X main.version=" + version + " -X main.commit=" + commit + " -X main.date=" + date

pkg := "./cmd/rommate"
dist := "dist"

# Release targets as os/arch pairs.
targets := "linux/amd64 linux/arm64 darwin/amd64 darwin/arm64"

export CGO_ENABLED := "0"

# List recipes
default:
    @just --list --unsorted

# ── development ──────────────────────────────────────────────────────────────

# Build for the current machine → dist/rommate
build:
    go build -trimpath -ldflags '{{ ldflags }}' -o {{ dist }}/rommate {{ pkg }}

# Build for one platform, e.g. `just build-target linux arm64`
build-target os arch:
    GOOS={{ os }} GOARCH={{ arch }} go build -trimpath -ldflags '{{ ldflags }}' \
        -o {{ dist }}/rommate-{{ os }}-{{ arch }} {{ pkg }}

# Build every release target
build-all:
    for t in {{ targets }}; do just build-target "${t%/*}" "${t#*/}"; done

# Run rommate with arguments, e.g. `just run platforms`
run *args:
    go run -ldflags '{{ ldflags }}' {{ pkg }} {{ args }}

# Build for the Armada handheld and copy it over
deploy host="armada@armada.local": (build-target "linux" "arm64")
    scp {{ dist }}/rommate-linux-arm64 {{ host }}:.local/bin/rommate
    ssh {{ host }} '.local/bin/rommate --version'

# ── quality ──────────────────────────────────────────────────────────────────

# Run the tests
test *args:
    go test {{ args }} ./...

# Run the tests with coverage → dist/coverage.out
cover:
    mkdir -p {{ dist }}
    go test -coverprofile={{ dist }}/coverage.out ./...
    go tool cover -func={{ dist }}/coverage.out | tail -1

# Format all Go code
fmt:
    gofmt -w cmd internal

# Fail if any Go file is not formatted
fmt-check:
    @files="$(gofmt -l cmd internal)"; if [ -n "$files" ]; then echo "not gofmt'ed:"; echo "$files"; exit 1; fi

# Run go vet
vet:
    go vet ./...

# Run golangci-lint
lint:
    golangci-lint run ./...

# Fail if go.mod/go.sum are not tidy
tidy-check:
    go mod tidy -diff

# Everything CI checks: formatting, tidy modules, vet, lint and tests
check: fmt-check tidy-check vet lint test

# ── release ──────────────────────────────────────────────────────────────────

# Print the version that builds would embed
version:
    @echo {{ version }}

# Build all targets and package them with checksums → dist/release
package: clean build-all
    mkdir -p {{ dist }}/release
    for t in {{ targets }}; do \
        os="${t%/*}"; arch="${t#*/}"; \
        name="rommate_{{ version }}_${os}_${arch}"; \
        mkdir -p "{{ dist }}/$name"; \
        cp "{{ dist }}/rommate-${os}-${arch}" "{{ dist }}/$name/rommate"; \
        cp README.md "{{ dist }}/$name/"; \
        if [ -f LICENSE ]; then cp LICENSE "{{ dist }}/$name/"; fi; \
        tar -C {{ dist }} -czf "{{ dist }}/release/$name.tar.gz" "$name"; \
        rm -rf "{{ dist }}/$name"; \
    done
    cd {{ dist }}/release && (command -v sha256sum >/dev/null && sha256sum *.tar.gz || shasum -a 256 *.tar.gz) > checksums.txt
    @echo "packages in {{ dist }}/release:" && ls -1 {{ dist }}/release

# Remove build output
clean:
    rm -rf {{ dist }}
