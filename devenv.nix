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
    just # task runner; see the justfile for build/test/release recipes
  ];

  env.CGO_ENABLED = "0";

  git-hooks.hooks = {
    gofmt.enable = true;
    govet.enable = true;
    nixfmt.enable = true;
  };
}
