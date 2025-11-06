{
  inputs = {
    devenv.url = "github:cachix/devenv";
    devlib.url = "github:shikanime-studio/devlib";
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    treefmt-nix.url = "github:numtide/treefmt-nix";
  };

  nixConfig = {
    extra-public-keys = [
      "shikanime.cachix.org-1:OrpjVTH6RzYf2R97IqcTWdLRejF6+XbpFNNZJxKG8Ts="
      "devenv.cachix.org-1:w1cLUi8dv3hnoSPGAuibQv+f9TZLr6cv/Hm9XgU50cw="
    ];
    extra-substituters = [
      "https://shikanime.cachix.org"
      "https://devenv.cachix.org"
    ];
  };

  outputs =
    inputs@{
      devenv,
      devlib,
      flake-parts,
      treefmt-nix,
      ...
    }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        devenv.flakeModule
        treefmt-nix.flakeModule
      ];
      perSystem =
        {
          pkgs,
          system,
          ...
        }:
        {
          treefmt = {
            projectRootFile = "flake.nix";
            enableDefaultExcludes = true;
            programs = {
              gofmt.enable = true;
              golines.enable = true;
              nixfmt.enable = true;
              prettier.enable = true;
              statix.enable = true;
              terraform.enable = true;
            };
            settings.global.excludes = [
              "public/**"
              "*.terraform.lock.hcl"
              ".gitattributes"
              "LICENSE"
            ];
          };
          devenv = {
            modules = [
              devlib.devenvModule
            ];
            shells.default = {
              cachix = {
                enable = true;
                push = "shikanime";
              };
              containers = pkgs.lib.mkForce { };
              github.enable = true;
              gitignore = {
                enable = true;
                enableDefaultTemplates = true;
              };
              languages = {
                go.enable = true;
                javascript.enable = true;
                nix.enable = true;
                opentofu.enable = true;
                shell.enable = true;
              };
              packages = [
                pkgs.buf
                pkgs.gitnr
                pkgs.go-migrate
                pkgs.ko
                pkgs.nodejs
                pkgs.nushell
                pkgs.protoc-gen-connect-go
                pkgs.protoc-gen-es
                pkgs.protoc-gen-go
                pkgs.scaleway-cli
                pkgs.skaffold
              ];
              services.postgres.enable = true;
            };
          };
        };
      systems = [
        "x86_64-linux"
        "x86_64-darwin"
        "aarch64-linux"
        "aarch64-darwin"
      ];
    };
}
