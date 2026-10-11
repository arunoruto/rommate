# rommate task runner. Run `just` to list recipes.
#
# This file is the single place where build, test and release commands live:
# local development (nix develop github:arunoruto/flake#go-lsp) and CI all
# call these recipes.

set shell := ["bash", "-euo", "pipefail", "-c"]

# Build metadata embedded via -ldflags (see cmd/rommate/main.go).
version := env("VERSION", `git describe --tags --always --dirty 2>/dev/null || echo dev`)
commit := `git rev-parse --short HEAD 2>/dev/null || echo unknown`
date := env("BUILD_DATE", `date -u +%Y-%m-%dT%H:%M:%SZ`)
ldflags := "-s -w -X main.version=" + version + " -X main.commit=" + commit + " -X main.date=" + date

pkg := "./cmd/rommate"
dist := "dist"

# Release targets as os/arch pairs. Windows builds are published but untested.
targets := "linux/amd64 linux/arm64 darwin/arm64 windows/amd64"

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
        -o {{ dist }}/rommate-{{ os }}-{{ arch }}{{ if os == "windows" { ".exe" } else { "" } }} {{ pkg }}

# Build every release target
build-all:
    for t in {{ targets }}; do \
        VERSION={{ version }} BUILD_DATE={{ date }} just dist={{ dist }} build-target "${t%/*}" "${t#*/}"; \
    done

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

# Build all release binaries with checksums → dist/release
#
# Assets keep the same names in every release (rommate-linux-arm64, …) so
# https://github.com/arunoruto/rommate/releases/latest/download/<asset> always
# points to the newest build.
package: clean
    mkdir -p {{ dist }}/release
    just dist={{ dist }}/release build-all
    cd {{ dist }}/release && (command -v sha256sum >/dev/null && sha256sum rommate-* || shasum -a 256 rommate-*) > checksums.txt
    @echo "release assets in {{ dist }}/release:" && ls -1 {{ dist }}/release

# ── website (docs/) ──────────────────────────────────────────────────────────

# Hugo from PATH, or from nixpkgs when it isn't installed.
hugo := if `command -v hugo || true` != "" { "hugo" } else { "nix run nixpkgs#hugo --" }
freeze := if `command -v freeze || true` != "" { "freeze" } else { "nix run nixpkgs#charm-freeze --" }

# Serve the website with live reload at http://localhost:1313/rommate/
site: site-data
    cd docs && {{ hugo }} server --buildDrafts --disableFastRender

# Build the website → docs/public (CI passes the Pages URL as base_url)
site-build base_url="": site-data
    cd docs && {{ hugo }} --gc --minify {{ if base_url != "" { "--baseURL " + base_url } else { "" } }}

# Regenerate files the website takes from the repo (folder mappings, install.sh)
site-data:
    go run {{ pkg }} mappings --json > docs/data/mappings.json
    cp install.sh docs/static/install.sh

# Render TUI screenshots for the website → docs/static/images/screenshots
screenshots:
    #!/usr/bin/env bash
    set -euo pipefail
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT
    ROMMATE_SCREENSHOT_DIR="$tmp" go test -count=1 -run TestScreenshots ./internal/tui/
    out=docs/static/images/screenshots
    mkdir -p "$out"
    for f in "$tmp"/*.ansi; do
        name="$(basename "$f" .ansi)"
        {{ freeze }} --execute "cat $f" --window --padding 24 --margin 0 --border.radius 10 \
            --background "#16161E" --font.family "JetBrains Mono" --font.size 14 \
            --output "$out/$name.png" < /dev/null > /dev/null
        echo "wrote $out/$name.png"
    done

# Remove build output
clean:
    rm -rf {{ dist }}
