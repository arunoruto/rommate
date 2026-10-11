{
  pkgs,
  lib,
  config,
  inputs,
  ...
}:

{
  languages.go = {
    enable = true;
    package = pkgs.go;
  };

  packages = with pkgs; [
    gopls # Go LSP
    gotools # goimports & friends
    golangci-lint
    delve # debugger
    nixd # Nix LSP
    taplo # TOML LSP/formatter (config.toml)
    sqlite # inspect the state database
  ];

  env.CGO_ENABLED = "0";

  scripts = {
    # Native build for the current machine
    build.exec = ''
      go build -trimpath -o dist/rommate ./cmd/rommate
    '';
    build-arm64.exec = ''
      GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o dist/rommate-linux-arm64 ./cmd/rommate
    '';
    build-amd64.exec = ''
      GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/rommate-linux-amd64 ./cmd/rommate
    '';
    # Usage: deploy [user@host]  (defaults to the Armada handheld)
    deploy.exec = ''
      host="''${1:-armada@armada.local}"
      build-arm64 && scp dist/rommate-linux-arm64 "$host:.local/bin/rommate"
    '';
  };

  git-hooks.hooks = {
    gofmt.enable = true;
    govet.enable = true;
    nixfmt.enable = true;
  };
}
