# sing-box-go

A lightweight tray controller for Windows that runs the official `sing-box` core in the background and exposes a
small, native control surface: system proxy, TUN mode, Clash mode switching, selector switching and a link to the
Clash dashboard.

The goal is to keep the full power of a normal sing-box config while adding only the Windows-specific glue, without
becoming another all-in-one GUI client.

- Uses the official `sing-box.exe` — nothing is embedded, nothing is bundled
- One JSON or `.bpf` config, used as-is
- Native Win32 controls (no GUI toolkit) with explicit accessible names, so screen readers such as NVDA and
  争渡读屏 can navigate the tray menu and settings dialog
- Small idle footprint: no periodic HTTP polling after startup; the Clash API is contacted at startup, when the
  menu is opened and when you switch something

## Features

- Tray icon reflects the current state: TUN active, system proxy enabled, or idle
- Left-click the tray icon to open the settings dialog
- Right-click for the context menu:
    - **更多** (More) — restart, autostart, project page
    - **模式** (Mode) — Clash mode switcher, populated from the Clash API
    - **节点** (Nodes) — selector outbounds; the outbound currently used by the core is marked, even when it was
      changed from outside this program (for example from the dashboard)
    - **TUN 模式** (TUN mode) — available when the config contains a `tun` inbound; asks for elevation when needed
    - **系统代理** (System proxy) — toggles the Windows proxy settings
    - **面板** (Dashboard) — opens the Clash dashboard if one is configured
    - **退出** (Quit) — gracefully stops the core and exits
- The system proxy is restored on the next launch when it was left enabled, and is disabled on normal exit
- TUN state is remembered across restarts; enabling TUN relaunches the program elevated (UAC)
- Selector choices can be remembered across restarts
- Remote `.bpf` profiles can be updated automatically
- Single instance per user session; starting a second copy is refused
- Optional autostart through a scheduled task

## Requirements

- Windows 10 or later (64-bit)
- The official `sing-box` for Windows (`sing-box.exe`), downloaded from the upstream project:
  <https://github.com/SagerNet/sing-box/releases>
- For TUN mode, the `wintun.dll` that ships with the official Windows archive must be next to `sing-box.exe`
- A sing-box config that satisfies the requirements below

## Getting started

1. Build the program (there are no prebuilt binaries):

   ```sh
   go build -ldflags "-H=windowsgui" -trimpath -o sing-box-go.exe .
   ```

2. Put `sing-box-go.exe` in any folder. The settings dialog creates `sing-box-go.ini` when you save for the first
   time.
3. Put `sing-box.exe` into the same folder, or point `sb-dir` at its folder.
4. Run `sing-box-go.exe`. On the first launch the settings dialog opens; choose the core directory and the config
   file. After saving, a tray icon appears and the core starts.

The tray menu and the settings dialog are in Chinese.

## sing-box configuration requirements

### Required: mixed inbound

The program always needs a `mixed` inbound: it reads `listen` and `listen_port` to configure the Windows system proxy
and refuses to start without one.

```jsonc
{
  "inbounds": [
    {
      "type": "mixed",
      "tag": "mixed-in", // any tag, not used by the program
      "listen": "127.0.0.1",
      "listen_port": 1080
    }
  ]
}
```

### Optional: tun inbound

If the config contains an inbound of type `tun`, the TUN menu item appears. Without it, TUN mode is unavailable.

```jsonc
{
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "address": ["172.19.0.1/30"],
      "auto_route": true
    }
  ]
}
```

When TUN is disabled, the program starts the core with the `tun` inbounds removed from the in-memory config; the file
on disk is never modified.

### Required for node/mode switching: Clash API

Node switching, mode switching and the dashboard link use the Clash API:

```jsonc
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "secret": "change-me",
      "external_ui": "ui" // optional; an http(s) URL is used as-is
    }
  }
}
```

- `external_controller` — API address. `0.0.0.0` is rewritten to `127.0.0.1` for the dashboard link.
- `secret` — API token. It is appended to the dashboard URL automatically.
- `external_ui` — optional. An `http(s)://` URL is opened as-is; any other value is ignored and
  `http://<controller>/ui/` is used.

If the config contains selectors but no `clash_api` section, the program adds one in memory with
`external_controller = "127.0.0.1:9090"` and a generated secret. This works for the current run only; the config file is
not rewritten.

### Optional: selectors for node switching

Selectors are shown in the **节点** menu. The selector `tag` is the submenu title, its `outbounds` are the items, and
`default` is the initial choice.

```jsonc
{
  "outbounds": [
    {
      "type": "selector",
      "tag": "proxy-select",
      "outbounds": ["Proxy1", "Proxy2"],
      "default": "Proxy1",
      "interrupt_exist_connections": true
    }
  ]
}
```

Switching an item sends `PUT /proxies/<tag>` to the Clash API and then closes existing connections, so the change
applies immediately.

### Optional: Clash modes

If the core reports a `mode-list`, the **模式** menu is shown. Selecting a mode sends `PATCH /configs` to the Clash API.

## Program configuration

Settings are stored in `sing-box-go.ini` next to the executable:

```ini
[sing-box-go]
sb-dir =
sb-config-file = config.json
tun-start-mode = off
system-proxy-auto = 0
; Selector menu layout: "auto", "flat" or "nested"
selector-menu-layout = auto
selector-persist = 0
; Optional: write a log file (relative paths are resolved against the program folder)
; log-file = sing-box-go.log
```

| Key | Description |
| --- | --- |
| `sb-dir` | Folder containing `sing-box.exe`. Empty means the folder with `sing-box-go.exe`. |
| `sb-config-file` | sing-box config file (JSON or `.bpf`). A relative name is resolved against the program folder first, then `sb-dir`. |
| `tun-start-mode` | TUN state used only on the very first launch: `off` (default) or `on`. Later launches use the remembered state. |
| `system-proxy-auto` | System proxy state used only on the very first launch: `1` enabled, `0` disabled (default). |
| `selector-menu-layout` | Node menu layout: `auto` decides by item count, `flat` shows each selector as a disabled header with checkable items below, `nested` uses submenus. |
| `selector-persist` | `1` restores the last selected outbound on the next launch and also remembers choices made from outside the app (for example from the dashboard); `0` (default) starts from the config `default` and does not remember choices between launches (a program self-restart still keeps the current selection). |
| `log-file` | Optional log file path. Relative paths are resolved against the program folder. Empty disables logging. |

The settings dialog edits `sb-dir`, `sb-config-file` and `selector-menu-layout`. Saving a different core directory or
config file restarts the program automatically.

Note: sing-box itself also remembers selector choices when `experimental.cache_file.enabled` is set in the config. In
that case `selector-persist = 0` cannot force the config `default`; the core restores its own cached choice. The option
controls only the program's own state and whether it re-applies that state at startup.

### State file

Runtime state is kept in `sing-box-go.state.json` next to the executable:

- `tun` — whether TUN was active on the last run
- `system-proxy` — whether the system proxy was enabled on the last run
- `selectors` — the last selected outbound for each selector (restored on the next launch when
  `selector-persist = 1`)

Delete this file to fall back to `tun-start-mode` and `system-proxy-auto`.

### Autostart

Autostart is implemented as a scheduled task named `sing-box-go` that runs the current executable at logon with the
highest available privileges. Enabling or disabling it may prompt for UAC. If the executable is moved, re-enable
autostart so the task points at the new path.

## Command-line switches

These are mainly used internally when the program relaunches itself, but they can also be passed manually:

| Switch | Effect |
| --- | --- |
| `-tun` | Start with TUN enabled (requests elevation if needed). |
| `-restart` | Wait for an already running instance to exit before starting. |
| `-autostart-enable` | Create the autostart scheduled task. |
| `-autostart-disable` | Remove the autostart scheduled task. |

## BPF profiles

Besides plain JSON, `sb-config-file` can point to a `.bpf` profile — the binary profile format used by the official
sing-box clients (SFA for Android, SFI/SFM for Apple platforms).

`.bpf` is not a documented public format; it is defined by the `libbox` sources in the sing-box repository. A file
consists of a 1-byte message type, a 1-byte version and a gzip-compressed payload containing the profile name, type,
JSON config and, for remote profiles, the URL, auto-update flag, interval and last-updated timestamp.

For remote profiles with auto-update enabled, the program checks for a new config about one minute after launch and
then at the profile's interval (at least every 15 minutes). The downloaded config is written back to the `.bpf` file.
The running core is **not** restarted automatically; the new config takes effect on the next launch.

To create or edit a `.bpf` profile in the browser, use
[sing-box-bpf-editor](https://hdrover.github.io/sing-box-bpf-editor/) — it runs fully client-side.

## Building and testing

Requirements:

- Go 1.26 or newer
- `golang.org/x/sys` (resolved by `go mod download`)

```sh
go build -ldflags "-H=windowsgui" -trimpath -o sing-box-go.exe .
go test .
```

`-H=windowsgui` suppresses the console window. The UI is written directly against Win32 (GDI/user32/shell32), so the
executable has no runtime dependencies beyond the standard Windows system libraries.

### Source layout

| File | Purpose |
| --- | --- |
| `main.go` | Startup, single instance, DPI/COM initialization, first-run settings, elevation |
| `ui.go` | Tray icon, context menu, system proxy and TUN actions, autostart, shutdown |
| `settings.go` | Win32 settings dialog and file/folder pickers |
| `win32.go` | Win32 bindings, structs, accessibility names, small helpers |
| `drover.go` | Application state, selector/mode state, runtime persistence, shutdown |
| `supervisor.go` | Core process supervision (job object, stdin config, Clash API checks) |
| `apiclient.go` | Minimal Clash API client |
| `config.go` | Config parsing, requirement checks, in-memory TUN stripping, Clash API creation |
| `bpf.go` | BPF profile encoding and decoding |
| `configupdater.go` | Remote BPF profile updates |
| `options.go`, `appstate.go` | INI options and JSON state file |
| `autostart.go`, `elevation.go`, `singleinstance.go` | Windows integration helpers |
| `systemproxy.go`, `tuncleanup.go`, `singboxcli.go`, `logger.go` | System proxy, Wintun cleanup, core CLI helpers, logging |

## Limitations

- Only one config file at a time; there is no profile switching in the UI.
- TUN mode requires an elevated process and a `tun` inbound.
- A remote BPF config update does not restart the running core.
- The program does not modify the sing-box config file; in-memory changes (TUN removal, generated Clash API) are
  per-run only.
