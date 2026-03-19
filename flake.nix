{
  inputs.nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";

  outputs =
    { nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      perSystem = f: nixpkgs.lib.genAttrs systems (system: f (import nixpkgs { inherit system; }));
    in
    {
      packages = perSystem (pkgs: rec {
        default = pkgs.callPackage ./default.nix {};
        discord-voice-rpc = default;
      });

      devShells = perSystem (pkgs: {
        default = pkgs.mkShell {
          name = "discord-voice-rpc-dev";
          buildInputs = with pkgs; [
            go
            gofumpt
            golangci-lint
            golangci-lint-langserver
            gotestsum
            gopls
            just
          ];
        };
      });
    };
}
