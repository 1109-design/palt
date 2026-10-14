# palt-relay

Lets PALT devices reach each other when they are **not** on the same network.
It's a blind pipe: devices that share a pairing code are matched by a group id
derived from that code, and bytes are copied between their sockets in memory.
Every relayed stream is end-to-end encrypted by the apps (X25519 + AES-256-GCM),
so the server can't read files or messages, and nothing is written to disk.

Same-network transfers never touch the relay — PALT always tries direct first.

Stdlib-only Go, one TCP port (default `9877`).

## Deploy on a droplet (Ubuntu/Debian)

```bash
# 1. Go ≥ 1.24 (apt's golang-go is often too old) and git
sudo snap install go --classic
sudo apt-get update && sudo apt-get install -y git

# 2. Build
git clone https://github.com/h200137j/palt.git
cd palt/relay
go build -o palt-relay .
go test ./...            # optional sanity check

# 3. Install as a service
sudo install -m 755 palt-relay /usr/local/bin/palt-relay
sudo install -m 644 palt-relay.service /etc/systemd/system/palt-relay.service
sudo systemctl daemon-reload
sudo systemctl enable --now palt-relay
systemctl status palt-relay --no-pager

# 4. Open the port
sudo ufw allow 9877/tcp   # if ufw is active
# …and in the DigitalOcean Cloud Firewall, if one is attached: inbound TCP 9877
```

Logs (device names joining/leaving and pipe open/close only — never payload):

```bash
journalctl -u palt-relay -f
```

## Pair your devices

1. Desktop → globe icon in the top bar → enter the droplet IP → **Generate**.
2. Copy the `palt1:…` pairing code. On the same Wi-Fi you can send it to your
   phone as a PALT message.
3. Phone → globe icon → paste the code → **Connect**.

Devices that are only reachable through the relay show a **RELAY** tag; devices
on your network show **DIRECT**. Transfers, history and progress show the
route they took.

## Update

```bash
cd palt && git pull && cd relay && go build -o palt-relay . \
  && sudo install -m 755 palt-relay /usr/local/bin/palt-relay \
  && sudo systemctl restart palt-relay
```

## Protocol

Every connection starts with one JSON line (`\n`-terminated, ≤ 4 KB):

| `t`        | purpose |
|------------|---------|
| `register` | long-lived control connection; gets `peers` lists and `incoming` notices; pings every 20 s |
| `connect`  | ask for a pipe to device `to` with a fresh `session` id |
| `accept`   | target's answer to `incoming`; relay replies `ok` to both, then pipes raw bytes |

Inside a pipe the apps run their own handshake and framed AES-GCM, then the
normal PALT protocol. See `desktop/internal/relay/securepipe.go`.

Limits: 32 devices and 16 concurrent pipes per group, 15 s to answer an
incoming pipe, pipes closed after 10 min of no traffic.
