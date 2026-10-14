# PALT Relay — plan

Goal: transfer between my devices when they are NOT on the same network, using a
DigitalOcean droplet as a blind byte pipe. LAN (direct) stays the default and is
always tried first. Nothing is ever written to disk on the droplet.

## Pieces

1. **`relay/`** — standalone Go server (stdlib only), runs on the droplet.
   - One TCP port (default `9877`).
   - Each connection opens with one JSON line; three kinds:
     - `register` — long-lived control connection. Device joins its *group*,
       gets live `peers` lists of the group, and `incoming` session notices.
       Client pings every 20 s; relay drops it after 60 s of silence.
     - `connect` — sender asks for a pipe to device `to`, with a random session id.
       Relay pushes `incoming` to the target's control conn, waits ≤ 15 s.
     - `accept` — target opens a fresh conn for that session. Relay replies `ok`
       to both, then `io.Copy`s bytes both ways in memory (half-close aware).
   - Limits: line ≤ 4 KB, sessions per group, idle timeout. No disk, no logs of payload.

2. **Pairing code** — one secret shared by all my devices:
   `palt1:<relay-host>:<port>:<base64url 32-byte key>`
   - Generated on desktop ("Remote" dialog), pasted on every other device
     (can be sent over LAN with a PALT text message).
   - `groupId  = HMAC(key, "palt-group-id")` → what the relay sees.
   - `authKey  = HMAC(key, "palt-auth")` → never leaves devices.
   - Presence entries carry `sig = HMAC(authKey, id|name|os)`; clients drop peers
     with bad sigs, so the relay (or anyone who sniffs groupId) can't inject devices.

3. **End-to-end encryption** over every relayed pipe (relay is blind):
   - Handshake: X25519 ephemeral keys, each side MACs its key with `authKey`
     (proves group membership). Keys = HKDF-SHA256(shared, salt=authKey,
     info="palt-relay-v1"|epkI|epkR) → one AES-256-GCM key per direction.
   - Frames: `u32 len | ciphertext`, nonce = per-direction counter, ≤ 64 KB plaintext.
   - Inside the encrypted pipe the **existing PALT protocol runs unchanged**
     (offer JSON → verdict byte → raw file bytes / text).

4. **Desktop (Go/Wails)**
   - `internal/relay` client: control loop with reconnect/backoff, presence list,
     `Dial(peerId)` and incoming-session handling → `transfer.Server.HandleConn`.
   - `internal/securepipe`: handshake + framed AES-GCM conn implementing `net.Conn`.
   - Transfer client takes a dialer instead of ip:port.
   - Route choice: peer visible on LAN → direct (default). Else relay. If a direct
     dial fails and the peer is on the relay → fall back to relay.
   - Bindings: `GetRelayConfig`, `SetRelayCode`, `GenerateRelayCode`, `GetRelayStatus`.

5. **Mobile (Flutter)**
   - Same relay client + secure pipe in Dart (`cryptography` + `cryptography_flutter`
     for native AES-GCM).
   - Transfer service refactored onto a small `PaltConn` interface so the same
     send/receive code runs over a `Socket` or an encrypted relay pipe.
   - Relay connection lives while the app is running (no background wake-up yet).

6. **Route indication (both apps)**
   - Device cards/pages: `DIRECT` (LAN) or `RELAY` badge.
   - Progress UI shows "Direct" / "via Relay".
   - History entries record `route` (`direct` | `relay`) and the thread shows it.

## Deploy (droplet)
- `cd relay && go build -o palt-relay .` → `/usr/local/bin/palt-relay`
- systemd unit `relay/palt-relay.service`, open TCP 9877 in ufw / DO firewall.

## Test plan
- Go unit tests: securepipe round-trip + tamper, relay pairing/pipe end-to-end on localhost.
- Manual: phone on mobile data ↔ laptop on Wi-Fi: files both ways, text, reject,
  trusted auto-accept; same LAN still shows DIRECT.

## Out of scope (later)
- Background wake on Android (FCM), NAT hole-punching, resumable transfers.
