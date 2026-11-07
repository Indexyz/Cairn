{
  description = "Cairn - fact-graph based collaborative exploration protocol";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs =
    inputs:
    inputs.flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];

      perSystem =
        { pkgs, ... }:
        let
          cairn = pkgs.callPackage ./nix/package.nix { };
        in
        {
          packages = {
            inherit cairn;
            web = cairn.web;
            default = cairn;
          };

          devShells.default = pkgs.mkShell {
            packages = [
              pkgs.python3
              pkgs.uv
              pkgs.nodejs_22
            ];
          };
        };

      flake = {
        nixosModules = {
          cairn =
            { pkgs, ... }:
            {
              imports = [ (import ./nix/module.nix) ];
              services.cairn.package = inputs.self.packages.${pkgs.stdenv.hostPlatform.system}.cairn;
            };
          default =
            { pkgs, ... }:
            {
              imports = [ (import ./nix/module.nix) ];
              services.cairn.package = inputs.self.packages.${pkgs.stdenv.hostPlatform.system}.cairn;
            };
        };

        overlays.default = final: prev: {
          cairn = final.callPackage ./nix/package.nix { };
        };

        nixosConfigurations.example = inputs.nixpkgs.lib.nixosSystem {
          system = "x86_64-linux";
          modules = [
            inputs.self.nixosModules.default
            (import ./nix/example.nix)
          ];
        };
      };
    };
}
