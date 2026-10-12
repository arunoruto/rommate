---
title: Development
weight: 7
---

rommate is written in Go and has no CGO, so it cross-compiles to every platform from any machine.

## Development shell

With [Nix](https://nixos.org), everything you need comes from one shell: Go, gopls, golangci-lint, delve and just.

```sh
nix develop github:arunoruto/flake#go-lsp
```

With [nix-direnv](https://github.com/nix-community/nix-direnv), the repository's `.envrc` loads it automatically. To
run a single command inside it, use `nix develop github:arunoruto/flake#go-lsp -c just check`.

Without Nix, install Go (the version in `go.mod`), [just](https://just.systems) and
[golangci-lint](https://golangci-lint.run).

## Tasks

All common tasks are [just](https://just.systems) recipes. Run `just` to list them.

```sh
just build                      # → dist/rommate for this machine
just build-target linux arm64   # → dist/rommate-linux-arm64
just run platforms              # go run with arguments
just deploy                     # build for ARM64 and copy to armada@armada.local
just check                      # formatting, tidy modules, vet, lint and tests
just package                    # release binaries and checksums → dist/release
```

## Website

This site lives in `docs/` and is built with [Hugo](https://gohugo.io) and the
[Hextra](https://imfing.github.io/hextra/) theme. If Hugo is not installed, the recipes fetch it from nixpkgs.

```sh
just site          # preview at http://localhost:1313/rommate/
just site-build    # build into docs/public
just screenshots   # re-render the TUI screenshots from sample data
```

The folder mapping table is generated from the Go code by `just site-data`, so it always matches what rommate does.

## Releases

Releases follow [Semantic Versioning](https://semver.org) and are derived from
[Conventional Commits](https://www.conventionalcommits.org):

| Commit | Version bump |
|---|---|
| `fix: …` | patch |
| `feat: …` | minor |
| `feat!: …` or a `BREAKING CHANGE:` footer | major (minor while on 0.x) |
| `docs:`, `ci:`, `build:`, `test:`, … | none |

To publish, run the **Release** workflow from the
[Actions tab](https://github.com/arunoruto/rommate/actions/workflows/release.yml) or with
`gh workflow run release.yml`. It runs CI, works out the next version, builds the binaries with `just package` and
creates the GitHub release with a changelog and checksums.
