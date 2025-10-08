{
  inputs = {
    devenv.url = "github:cachix/devenv";
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    templ.url = "github:a-h/templ";
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
      flake-parts,
      templ,
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
          self',
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
              nixfmt.enable = true;
              prettier.enable = true;
              shfmt.enable = true;
              statix.enable = true;
              terraform.enable = true;
            };
            settings.global.excludes = [
              "*.terraform.lock.hcl"
              ".gitattributes"
              "LICENSE"
            ];
          };
          devenv.shells.default = {
            cachix = {
              enable = true;
              push = "shikanime";
            };
            containers = pkgs.lib.mkForce { };
            git-hooks.hooks = {
              actionlint.enable = true;
              deadnix.enable = true;
              flake-checker.enable = true;
              shellcheck.enable = true;
              tflint.enable = true;
            };
            languages = {
              go.enable = true;
              opentofu.enable = true;
              nix.enable = true;
            };
            packages = [
              pkgs.gitnr
              pkgs.ko
              pkgs.nodejs
              pkgs.nushell
              pkgs.scaleway-cli
              pkgs.skaffold
              templ.packages.${system}.templ
            ];
            processes = {
              devenv.exec = ''
                ${templ.packages.${system}.templ}/bin/templ generate \
                  --watch \
                  --proxy http://localhost:8080 \
                  --open-browser false
              '';
              tailwindcss.exec = ''
                ${pkgs.tailwindcss}/bin/tailwindcss \
                  -i ./cmd/myawesomelist/app/assets/app.css \
                  -o ./cmd/myawesomelist/app/public/styles.css \
                  --minify \
                  --watch
              '';
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
