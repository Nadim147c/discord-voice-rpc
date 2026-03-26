<p align="center">
<img src="https://cdn.prod.website-files.com/6257adef93867e50d84d30e2/66e3d74e9607e61eeec9c91b_Logo.svg">
</p>
<h1 align="center">Discord Voice RPC</h1>
<h3 align="center"> A lightweight tool for fetching Discord voice RPC data as JSON stream</h3>

<h1 align="center">
<a href="https://pkg.go.dev/github.com/Nadim147c/discord-voice-rpc">
<img src="https://img.shields.io/github/go-mod/go-version/Nadim147c/discord-voice-rpc?style=for-the-badge&logo=go&labelColor=11140F&color=BBE9AA">
</a>
<a href="https://github.com/Nadim147c/discord-voice-rpc">
<img src="https://img.shields.io/github/stars/Nadim147c/discord-voice-rpc?style=for-the-badge&logo=github&labelColor=11140F&color=BBE9AA">
</a>
<a href="https://github.com/Nadim147c/discord-voice-rpc/blob/main/LICENSE">
<img src="https://img.shields.io/github/license/Nadim147c/discord-voice-rpc?style=for-the-badge&logo=gplv3&labelColor=11140F&color=BBE9AA">
</a>
<a href="https://github.com/Nadim147c/discord-voice-rpc/commits">
<img src="https://img.shields.io/github/last-commit/Nadim147c/discord-voice-rpc?style=for-the-badge&logo=git&labelColor=11140F&color=BBE9AA">
</a>
</h1>

> [!NOTE]
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
