// relay_app.go wires the palt-relay client into the app. The relay address
// and key come from the pairing (pairing_app.go).
//
// LAN (direct) is always preferred. A peer is only shown as a relay peer when
// it is not visible on the LAN, and a failed direct dial falls back to the
// relay when the peer is connected there.
package main

import (
	"log"
	"net"
	"os"
	"runtime"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

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

// RelayStatus is shown in the Remote dialog.
type RelayStatus struct {
	Configured bool   `json:"configured"`
	Status     string `json:"status"` // off | connecting | online | error
	Detail     string `json:"detail"`
	Server     string `json:"server"`
	Code       string `json:"code"`
	Peers      int    `json:"peers"`
}

// startRelay (re)connects to the relay with the saved pairing code.
func (a *App) startRelay() {
	// Stop the old client outside the lock: Stop fires callbacks that read a.relay.
	a.stopRelay()

	p := a.pairing()
	a.relayMu.Lock()
	code, ok := p.RelayCode()
	if !ok {
		a.relayMu.Unlock()
		a.emitRelayStatus()
		a.emitPeers()
		return
	}

	hostname, _ := os.Hostname()
	c := relay.NewClient(code, p.DeviceID, hostname, runtime.GOOS)
	c.OnPeers = func([]relay.Peer) {
		a.emitPeers()
		// A relay (re)registration usually means a device changed networks:
		// look for it on the LAN now and re-check the LAN entries we have.
		if a.discovery != nil {
			a.discovery.Rescan()
		}
		a.kickProbe()
		a.kickConnector()
	}
	c.OnStatus = func(string, string) { a.emitRelayStatus() }
	c.OnIncoming = func(conn net.Conn, from relay.Peer) {
		log.Printf("[relay] Incoming connection from %s", from.Name)
		a.transferServer.HandlePaired(conn, transfer.RouteRelay, from.ID)
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

// ─── Wails bindings ──────────────────────────────────────────────────────────

// GetRelayStatus reports the relay connection for the pairing dialog.
func (a *App) GetRelayStatus() RelayStatus {
	p := a.pairing()
	st := RelayStatus{Status: relay.StatusOff, Code: p.Code()}
	if code, ok := p.RelayCode(); ok {
		st.Configured = true
		st.Server = code.Addr()
	}
	if rc := a.relayClient(); rc != nil {
		st.Status, st.Detail = rc.Status()
		st.Peers = len(rc.Peers())
	}
	return st
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
