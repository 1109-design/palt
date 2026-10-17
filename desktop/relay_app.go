// relay_app.go wires the palt-relay client into the app: pairing-code
// config, merging relay peers with LAN peers, and picking a route per send.
//
// LAN (direct) is always preferred. A peer is only shown as a relay peer when
// it is not visible on the LAN, and a failed direct dial falls back to the
// relay when the peer is connected there.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/models"
	"palt/internal/relay"
	"palt/internal/transfer"
)

// relayPeerIP stands in for an address on peers that are only reachable
// through the relay. The frontend passes it back to SendFile/SendText.
const relayPeerIP = "relay"

const directDialTimeout = 4 * time.Second

// LAN liveness probing for peers seen both on the LAN and on the relay.
// A phone that drops off Wi-Fi sends no mDNS goodbye, so without probing the
// stale LAN entry would hide its relay entry until the mDNS TTL runs out.
const (
	probeInterval = 4 * time.Second
	probeTimeout  = 1500 * time.Millisecond
	probeFailures = 2 // consecutive failed dials before the LAN entry is dropped
)

type relayConfig struct {
	Code     string `json:"code"`
	DeviceID string `json:"deviceId"`
}

// RelayStatus is shown in the Remote dialog.
type RelayStatus struct {
	Configured bool   `json:"configured"`
	Status     string `json:"status"` // off | connecting | online | error
	Detail     string `json:"detail"`
	Server     string `json:"server"`
	Code       string `json:"code"`
	Peers      int    `json:"peers"`
}

func relayConfigFile() string {
	dir, err := configDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "relay.json")
}

func loadRelayConfig() relayConfig {
	var cfg relayConfig
	if f := relayConfigFile(); f != "" {
		if data, err := os.ReadFile(f); err == nil {
			_ = json.Unmarshal(data, &cfg)
		}
	}
	return cfg
}

func saveRelayConfig(cfg relayConfig) error {
	f := relayConfigFile()
	if f == "" {
		return errors.New("could not locate config directory")
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// The pairing code is a secret: keep it private to this user.
	return os.WriteFile(f, data, 0o600)
}

// deviceID returns this install's stable relay id, creating it on first use.
func (a *App) deviceID(cfg *relayConfig) string {
	if cfg.DeviceID == "" {
		b := make([]byte, 8)
		rand.Read(b)
		cfg.DeviceID = hex.EncodeToString(b)
	}
	return cfg.DeviceID
}

// startRelay (re)connects to the relay with the saved pairing code.
func (a *App) startRelay() {
	// Stop the old client outside the lock: Stop fires callbacks that read a.relay.
	a.stopRelay()

	a.relayMu.Lock()
	cfg := loadRelayConfig()
	if cfg.Code == "" {
		a.relayMu.Unlock()
		a.emitRelayStatus()
		a.emitPeers()
		return
	}
	code, err := relay.ParseCode(cfg.Code)
	if err != nil {
		a.relayMu.Unlock()
		log.Printf("[relay] Saved pairing code is invalid: %v", err)
		a.emitRelayStatus()
		return
	}
	hadID := cfg.DeviceID != ""
	id := a.deviceID(&cfg)
	if !hadID {
		_ = saveRelayConfig(cfg)
	}

	hostname, _ := os.Hostname()
	c := relay.NewClient(code, id, hostname, runtime.GOOS)
	c.OnPeers = func([]relay.Peer) {
		a.emitPeers()
		// A relay (re)registration usually means a device changed networks:
		// look for it on the LAN now and re-check the LAN entries we have.
		if a.discovery != nil {
			a.discovery.Rescan()
		}
		a.kickProbe()
	}
	c.OnStatus = func(string, string) { a.emitRelayStatus() }
	c.OnIncoming = func(conn net.Conn, from relay.Peer) {
		log.Printf("[relay] Incoming transfer from %s", from.Name)
		a.transferServer.HandleConn(conn, transfer.RouteRelay)
	}
	a.relay = c
	a.relayMu.Unlock()
	c.Start()
	a.emitRelayStatus()
}

func (a *App) stopRelay() {
	a.relayMu.Lock()
	old := a.relay
	a.relay = nil
	a.relayMu.Unlock()
	if old != nil {
		old.Stop()
	}
}

func (a *App) relayClient() *relay.Client {
	a.relayMu.Lock()
	defer a.relayMu.Unlock()
	return a.relay
}

func (a *App) emitRelayStatus() {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "relay_status", a.GetRelayStatus())
	}
}

func (a *App) emitPeers() {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "peers_changed", a.GetPeers())
		a.refreshTray()
	}
}

// mergedPeers returns LAN peers plus relay peers that are not on the LAN.
func (a *App) mergedPeers() []models.Peer {
	out := []models.Peer{}
	onLAN := map[string]bool{}
	if a.discovery != nil {
		for _, p := range a.discovery.GetPeers() {
			p.Via = transfer.RouteDirect
			onLAN[p.DeviceName] = true
			out = append(out, p)
		}
	}
	if rc := a.relayClient(); rc != nil {
		for _, p := range rc.Peers() {
			if onLAN[p.Name] {
				continue
			}
			out = append(out, models.Peer{
				ID:         "relay-" + p.ID,
				DeviceName: p.Name,
				IPAddress:  relayPeerIP,
				Port:       0,
				OS:         p.OS,
				Via:        transfer.RouteRelay,
			})
		}
	}
	return out
}

// dialPeer connects to a peer, preferring the LAN. It returns the route used.
func (a *App) dialPeer(peerIP string, peerPort int, peerName string) (net.Conn, string, error) {
	var directErr error
	if peerIP != relayPeerIP {
		conn, err := transfer.DirectDialer(peerIP, peerPort, directDialTimeout)()
		if err == nil {
			return conn, transfer.RouteDirect, nil
		}
		directErr = err
	} else if a.discovery != nil {
		// The peer may have shown up on the LAN since the UI last refreshed.
		for _, p := range a.discovery.GetPeers() {
			if p.DeviceName == peerName {
				if conn, err := transfer.DirectDialer(p.IPAddress, p.Port, directDialTimeout)(); err == nil {
					return conn, transfer.RouteDirect, nil
				}
			}
		}
	}

	rc := a.relayClient()
	if rc != nil {
		if p, ok := rc.PeerByName(peerName); ok {
			log.Printf("[app] Sending to %s via relay", peerName)
			conn, err := rc.Dial(p.ID)
			if err != nil {
				return nil, "", err
			}
			return conn, transfer.RouteRelay, nil
		}
	}
	if directErr != nil {
		return nil, "", directErr
	}
	return nil, "", fmt.Errorf("%s is not connected to the relay", peerName)
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

// GetRelayStatus reports the relay connection for the Remote dialog.
func (a *App) GetRelayStatus() RelayStatus {
	cfg := loadRelayConfig()
	st := RelayStatus{Status: relay.StatusOff, Code: cfg.Code}
	if code, err := relay.ParseCode(cfg.Code); err == nil {
		st.Configured = true
		st.Server = code.Addr()
	} else if cfg.Code != "" {
		st.Status, st.Detail = relay.StatusError, err.Error()
	}
	if rc := a.relayClient(); rc != nil {
		st.Status, st.Detail = rc.Status()
		st.Peers = len(rc.Peers())
	}
	return st
}

// SetRelayCode saves a pairing code (from another device) and connects.
// An empty code turns the relay off.
func (a *App) SetRelayCode(code string) error {
	code = strings.TrimSpace(code)
	cfg := loadRelayConfig()
	if code != "" {
		parsed, err := relay.ParseCode(code)
		if err != nil {
			return err
		}
		code = parsed.String()
	}
	cfg.Code = code
	if err := saveRelayConfig(cfg); err != nil {
		return err
	}
	a.startRelay()
	return nil
}

// GenerateRelayCode creates a new pairing code for the relay at addr
// ("host" or "host:port"), saves it, connects, and returns it so it can be
// entered on the other devices.
func (a *App) GenerateRelayCode(addr string) (string, error) {
	code, err := relay.NewCode(addr)
	if err != nil {
		return "", err
	}
	if err := a.SetRelayCode(code.String()); err != nil {
		return "", err
	}
	return code.String(), nil
}

// ─── LAN liveness probe ──────────────────────────────────────────────────────

// runLANProbe periodically dials LAN peers that are also on the relay and
// drops the LAN entry once it stops answering, so the UI switches to RELAY
// within seconds. Runs until ctx is done.
func (a *App) runLANProbe() {
	failures := map[string]int{}
	t := time.NewTicker(probeInterval)
	defer t.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
		case <-a.probeKick:
		}
		rc := a.relayClient()
		if rc == nil || a.discovery == nil {
			continue
		}
		onRelay := map[string]bool{}
		for _, p := range rc.Peers() {
			onRelay[p.Name] = true
		}
		seen := map[string]bool{}
		for _, p := range a.discovery.GetPeers() {
			if !onRelay[p.DeviceName] {
				continue
			}
			seen[p.IPAddress] = true
			conn, err := transfer.DirectDialer(p.IPAddress, p.Port, probeTimeout)()
			if err == nil {
				conn.Close()
				delete(failures, p.IPAddress)
				continue
			}
			failures[p.IPAddress]++
			if failures[p.IPAddress] >= probeFailures {
				delete(failures, p.IPAddress)
				a.discovery.Evict(p.IPAddress)
			}
		}
		for ip := range failures {
			if !seen[ip] {
				delete(failures, ip)
			}
		}
	}
}

func (a *App) kickProbe() {
	select {
	case a.probeKick <- struct{}{}:
	default:
	}
}

// RescanPeers re-queries the LAN and re-checks LAN entries (refresh button).
func (a *App) RescanPeers() {
	if a.discovery != nil {
		a.discovery.Rescan()
	}
	a.kickProbe()
}
