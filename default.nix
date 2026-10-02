{
  lib,
  buildGo127Module,
}:
buildGo127Module {
  pname = "discord-voice-rpc";
  version = "0.0.1-unstable-2026-03-20";

  src = lib.cleanSource (
    lib.fileset.toSource {
      root = ./.;
      fileset = lib.fileset.unions [
        ./main.go
        ./types.go
        ./client.go
        ./go.mod
        ./go.sum
      ];
    }
  );

  vendorHash = "sha256-2ijBXEYSKKvo1XB9NSYbD47xJagKxaO+Ug81jCAsnis=";

  ldflags = [
    "-s"
    "-w"
    "-X main.buildType=release"
  ];

  meta = {
    description = "A CLI tool for getting discord voice rpc data used for game overlay.";
    homepage = "https://github.com/Nadim147c/discord-voice-rpc";
    license = lib.licenses.gpl3Only;
    mainProgram = "discord-voice-rpc";
  };
}
