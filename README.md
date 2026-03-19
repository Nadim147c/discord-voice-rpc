# Discord Voice RPC

A cli tool for getting discord voice rpc data used for game overlay. This is made for
using with quickshell to show overlay over entire screen. It uses IPC to get data
from discord.

> [!WARNING]
> This is completely derived from
> [thaYt/qs-discord](https://github.com/thaYt/qs-discord) licensed under
> [GPL-3.0](https://github.com/thaYt/qs-discord/blob/main/LICENSE) License.

## Installation

- Manual Install

```bash
git clone https://github.com/Nadim147c/discord-voice-rpc.git
cd discord-voice-rpc
just build-install # installs to ~/.local/bin
```

- Go Install

```
go install github.com/Nadim147c/discord-voice-rpc@latest
```

## Usage

```bash
discord-voice-rpc
```

There is Quickshell (QML) [exmaple](./example/quickshell) as a reference. You use
that QML binding to show any PanelWindow Or any other widgets.

It is also possible to use Quickshell hyprland binding to only show overlay over a
fullscreen window.

```qml
import Quickshell.Hyprland
import qs.services.discordVoiceRPC // example path

Loader {
    active: Hyprland.focusedWorkspace.hasFullscreen
    sourceComponent: MyCustomDiscordOverlay {}
}
```

## License

This project is licensed under the [GPL-3.0](./LICENSE) License and derived from
[thaYt/qs-discord](https://github.com/thaYt/qs-discord) licensed under
[GPL-3.0](https://github.com/thaYt/qs-discord/blob/main/LICENSE) License.

The [example](./example/quickshell) is licensed under [MIT](LICENSE-MIT) License to
make it easier to copy and use.
