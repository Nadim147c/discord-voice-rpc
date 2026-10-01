{
  inputs.nixpkgs.url = "https://channels.nixos.org/nixos-unstable/nixexprs.tar.xz";

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
        default = pkgs.callPackage ./default.nix { };
        discord-voice-rpc = default;
      });

      devShells = perSystem (pkgs: {
        default = pkgs.mkShell {
          name = "discord-voice-rpc-dev";
          buildInputs = with pkgs; [
            go_1_27
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
