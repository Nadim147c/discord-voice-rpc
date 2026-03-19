{
  lib,
  buildGoModule,
}:
buildGoModule {
  pname = "discord-voice-rpc";
  version = "0.0.1-unstable-2025-03-20";

  src = ../.;

  vendorHash = "";

  ldflags = [
    "-s"
    "-w"
  ];

  meta = {
    description = "A CLI tool for getting discord voice rpc data used for game overlay.";
    homepage = "https://github.com/Nadim147c/discord-voice-rpc";
    license = lib.licenses.gpl3Only;
    mainProgram = "discord-voice-rpc";
  };
}
