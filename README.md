# gonnect-vpn-example

This repository demonstrates a simple peer-to-peer VPN built on top of the
[`gonnect`](https://github.com/asciimoth/gonnect) ecosystem.

VPN nodes talk over WebSocket transport. A node can run as a server or a
client. Traffic is forwarded between the transport connection and a selected
TUN backend.

The project is intentionally an example app rather than a production VPN. It
exists to show how the same forwarding core can be surfaced through several
different clients and runtime environments.

## Implemented TUN Backends

- `native`: native OS TUN device configured with an address and route for the
  selected subnet.
- `vtun+http`: userspace virtual TUN backed by
  [gonnect-netstack](https://github.com/asciimoth/gonnect-netstack), with an HTTP server listening on the
  VPN address.
- `vtun+socks`: userspace virtual TUN backed by
  [gonnect-netstack](https://github.com/asciimoth/gonnect-netstack), with a local SOCKS proxy for
  reaching the VPN network.

When a new client connects to a server, the latest client session becomes the
active one.

## Implemented Clients

### CLI

The CLI in `cli/` is the most direct way to run the example VPN.

It supports:

- server mode over WebSocket
- client mode over WebSocket
- all current TUN backends: `native`, `vtun+http`, `vtun+socks`

Examples:

```sh
go build -o vpn ./cli

# Start a demo server that serves the web UI and accepts VPN clients.
./vpn --serve 127.0.0.1:9090 --tun vtun+http

# Start a native client.
sudo ./vpn --conn ws://127.0.0.1:9090/ws-vpn --tun native --name tun0 --addr 10.200.1.2/24 --subnet 10.200.1.0/24
```

Convenience command:

```sh
just serve
```

That starts a local demo server on `http://127.0.0.1:9090`.

### Web Client

The web client lives in `web/` and runs as a [WebAssembly](https://webassembly.org/) app in the browser.

It is an app-local userspace client:

- connects to the VPN server over WebSocket
- creates an in-browser `vtun`
- runs HTTP requests through the VPN

Build it with:

```sh
just build-web
```

Then open the server page from `just serve`, or publish the static assets from
`web/` / `pages/`.  
There is also a [web client instance hosted on GitHub pages](https://asciimoth.github.io/gonnect-vpn-example/).

### Desktop GUI

The desktop GUI lives in `gui/` and is built with [Gio](https://gioui.org/).

It supports:

- server mode and client mode
- `native`, `vtun+http`, and `vtun+socks` device types
- an additional plain `vtun` client mode for in-app HTTP and ICMP tools
- live logs and session controls

Build and run:

```sh
just build-gui
just run-gui
```

### Android

The Android client lives in `android/` with a native Kotlin UI and gomobile
bindings into the Go client code.

It currently provides two Android client paths:

- an app-local userspace `vtun` demo for HTTP requests and ICMP ping
- an Android `VpnService` path that establishes a real Android TUN and forwards
  the configured VPN subnet through Go

The Android path currently demonstrates the core plumbing:

- VPN permission flow
- service-owned TUN lifecycle
- TUN file descriptor handoff into Go
- outbound socket protection via `VpnService.protect`

Build and install:

```sh
just build-android-apk
just install-android-apk
```

More Android-specific notes are in
[docs/android-work-plan.md](docs/android-work-plan.md) and
[docs/android-kotlin-handoff.md](docs/android-kotlin-handoff.md).

## Project Layout

- `cli/`: CLI entrypoint
- `gui/`: desktop GUI client
- `web/`: browser client and embedded demo assets
- `android/`: native Android app
- `mobilelib/`: gomobile bridge for Android
- `clientcore/`: shared client-side vtun and Android-TUN logic
- `device/`: TUN backend construction
- `transport/`: WebSocket transport wrapper
- `runner/`: shared CLI server/client session wiring
- `cfg/`: config parsing and validation

## Notes

- Some backends require elevated privileges, especially `native`.
- The Android `VpnService` path is implemented for the example app, but it is
  still intentionally simpler than a production mobile VPN.

