<div align="center">
  <img src="assets/icon.svg" width="96"/>
  <br/><br/>
  <h1>PALT</h1>
  <p><b>P</b>eer <b>A</b>nd <b>L</b>ocal <b>T</b>ransfer</p>
  <p>Frictionless, high-speed local network file transfer — Linux and macOS Desktop, and Android.</p>

  [![CI/CD](https://github.com/h200137j/palt/actions/workflows/release.yml/badge.svg)](https://github.com/h200137j/palt/actions/workflows/release.yml)
</div>

---

PALT bridges Desktop and Mobile over raw TCP. No USB cables, no cloud uploads — it detects sibling devices over the local network via `mDNS` and streams files point-to-point at full network speed.

Powered by **Go** and **Wails** on Desktop, **Flutter** on Mobile.

---

## Design

**Beacon** — dark instrument panel, signal-amber accent, mono data readouts.

| Token | Value |
|---|---|
| Background | `#0C1220` |
| Surface | `#131B2C` / `#1A2438` |
| Hairline | `#263450` |
| Accent | `#FFB454` |
| Text | `#EAF0FB` / `#8FA0BC` |
| Fonts | Bricolage Grotesque / Inter / JetBrains Mono |

Depth from surface steps and hairlines — no shadows. One amber action per
screen. Every wire-derived value (IPs, speeds, sizes) is set in mono. The
local device is drawn as a live beacon: rings pulse while mDNS broadcasts.
Full spec in [`design.md`](design.md).

---

## Features

- **Blazing Fast TCP:** Core transfer runs strictly over local interfaces — unthrottled, no cloud hops.
- **Zero-Config Discovery:** ZeroConf / mDNS (`_palt._tcp.local.`) — devices see each other the moment the app opens.
- **Cross-Platform:** Linux and macOS Desktop (Wails + React + MUI) and Android (Flutter + Material 3 + Riverpod).
- **Drag & Drop:** Drop files straight onto a device card on desktop to send them.
- **Inbound Safety:** Manual accept/reject handshake on every incoming transfer.
- **Native Downloads:** Received files land directly in the Android public `Downloads` directory.
- **Lives in the Tray (Linux):** Starts on login straight into the system tray. The tray lists connected devices — pick one to open its chat. Closing the window keeps PALT running; quit from the tray. Toggle **Start on login** there too.
- **Remote via Relay (optional):** Not on the same network? Point PALT at your own [`palt-relay`](relay/) server. Transfers are end-to-end encrypted and never stored; same-network transfers still go direct. Each device and transfer is tagged **DIRECT** or **RELAY**.

---

## Architecture

```
palt/
├── .github/workflows/     # CI/CD — .apk, .deb and macOS .zip release automation
├── assets/                # Shared assets (icon.svg source)
├── relay/                 # palt-relay server (optional, for remote transfers)
├── desktop/               # Wails desktop app (Linux, macOS)
│   ├── main.go
│   ├── app.go             # Wails bindings
│   └── internal/
│       ├── discovery/     # mDNS advertiser + browser
│       └── transfer/      # Raw TCP chunked streamer
│   └── frontend/          # Vite + React + MUI
│
└── mobile/                # Flutter Android app
    ├── assets/icon.png
    └── lib/
        ├── theme/         # Concrete Lemon design tokens
        ├── ui/            # Screens & widgets
        ├── services/      # Android NSD + socket streaming
        └── providers/     # Riverpod state
```

---

## Building from Source

### Prerequisites

| Platform | Required |
|---|---|
| Desktop | Go ≥ 1.21, Node.js ≥ 18, [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation/) |
| Linux deps | `sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev build-essential` |
| Mobile | Flutter SDK ≥ 3.19, Android Studio or CLI tools |

### Desktop (Linux)

```bash
cd desktop
go mod tidy
wails dev -tags webkit2_41          # dev mode (live reload)
wails build -platform linux/amd64 -tags webkit2_41  # release binary
```

### Desktop (macOS)

Needs the Xcode Command Line Tools (`xcode-select --install`).

```bash
cd desktop
wails dev                                # dev mode (live reload)
wails build -platform darwin/universal   # build/bin/palt.app (Intel + Apple silicon)
```

Release builds are ad-hoc signed, not notarized: the first time, right-click
`PALT.app` → **Open**, or run `xattr -dr com.apple.quarantine PALT.app`.
macOS asks for **Local Network** access on first launch — allow it, or PALT
can't see your devices.

On macOS, sending/receiving files, chat, clipboard sync, media volume,
battery/network status, keep-awake and joining shared Wi-Fi work.
Notifications have no action buttons (answer in the window), and the tray,
start on login, Bluetooth, USB, hotspot, media players and fingerprint
unlock are still Linux-only.

### Mobile (Android)

```bash
cd mobile
flutter pub get
flutter run
```

---

## Technical Notes

**Android hostname:** Modern Android returns `localhost` for `Platform.localHostname`. PALT queries the HAL via `device_info_plus` for real model designations (e.g. *SM-S938B*) and uses IP-based deduplication instead of hostname strings.

**Discovery lifecycle:** All peers broadcast on port `9876`. TXT records carry OS signatures (`linux`, `android`) for platform icon injection. Dead peers are reaped via TTL (Go) or `removeWhere` on multicast drop (Dart).

---

<div align="center">
  <sub>Built by <a href="https://github.com/h200137j">uriel</a> · v1.1.0</sub>
</div>
