{
  inputs = {
    devenv.url = "github:cachix/devenv";
    devlib.url = "github:shikanime-studio/devlib";
    git-hooks.url = "github:cachix/git-hooks.nix";
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    treefmt-nix.url = "github:numtide/treefmt-nix";
  };

  nixConfig = {
    extra-substituters = [
      "https://cachix.cachix.org"
      "https://devenv.cachix.org"
      "https://shikanime.cachix.org"
    ];
    extra-trusted-public-keys = [
      "cachix.cachix.org-1:eWNHQldwUO7G2VkjpnjDbWwy4KQ/HNxht7H4SSoMckM="
      "devenv.cachix.org-1:w1cLUi8dv3hnoSPGAuibQv+f9TZLr6cv/Hm9XgU50cw="
      "shikanime.cachix.org-1:OrpjVTH6RzYf2R97IqcTWdLRejF6+XbpFNNZJxKG8Ts="
    ];
  };

  outputs =
    inputs@{
      devenv,
      devlib,
      git-hooks,
      flake-parts,
      treefmt-nix,
      ...
    }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        devenv.flakeModule
        devlib.flakeModule
        git-hooks.flakeModule
        treefmt-nix.flakeModule
      ];
      perSystem =
        { pkgs, ... }:
        {
          devenv.shells.default = {
            buf = {
              enable = true;
              generate = {
                clean = true;
                inputs = [
                  { directory = "proto"; }
                ];
                plugins = [
                  {
                    include_imports = true;
                    opt = "target=ts";
                    out = "www/app/proto";
                    package = pkgs.protoc-gen-es;
                  }
                  {
                    opt = "paths=source_relative";
                    out = "pkgs/proto";
                    package = pkgs.protoc-gen-go;
                  }
                  {
                    opt = "paths=source_relative";
                    out = "pkgs/proto";
                    package = pkgs.protoc-gen-connect-go;
                  }
                ];
                version = "v2";
              };
            };
            cachix = {
              enable = true;
              push = "shikanime";
            };
            containers = pkgs.lib.mkForce { };
            docker.enable = true;
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
              pkgs.ko
              pkgs.nushell
              pkgs.scaleway-cli
              pkgs.skaffold
            ];
            services.postgres.enable = true;
            treefmt = {
              enable = true;
              config = {
                enableDefaultExcludes = true;
                programs.prettier.enable = true;
                settings.global.excludes = [
                  "public/**"
                ];
              };
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
