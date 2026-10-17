// pairing_app.go keeps this device's identity and pairing key, and exposes
// pairing to the frontend.
//
// Every install belongs to exactly one group. A fresh install is a group of
// one; entering another device's code (or scanning its QR) joins that group.
// Devices in the same group are "paired": they authenticate and encrypt
// every connection, auto-accept each other's files and get the live
// features. Devices from other groups can still exchange files and text the
// old way, with a prompt every time.
package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"

	"palt/internal/pairing"
	"palt/internal/relay"
)

// PairingInfo is shown in the pairing dialog.
type PairingInfo struct {
	DeviceID string `json:"deviceId"`
	Code     string `json:"code"`  // give this (or its QR) to the other device
	Relay    string `json:"relay"` // "host:port", "" when LAN only
}

func pairingFile() string {
	dir, err := configDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "pairing.json")
}

// pairing returns this device's identity, loading or creating it on first use.
func (a *App) pairing() pairing.Pairing {
	a.pairMu.Lock()
	defer a.pairMu.Unlock()
	if a.pair.DeviceID != "" {
		return a.pair
	}
	f := pairingFile()
	if data, err := os.ReadFile(f); err == nil {
		var p pairing.Pairing
		if json.Unmarshal(data, &p) == nil && p.DeviceID != "" && len(p.Key) == 32 {
			a.pair = p
			return p
		}
	}
	a.pair = migratePairing()
	if err := savePairing(a.pair); err != nil {
		log.Printf("[pairing] Could not save identity: %v", err)
	}
	return a.pair
}

// migratePairing builds an identity from a 1.x relay config, so devices that
// already share a relay code stay paired, or starts a fresh group.
func migratePairing() pairing.Pairing {
	p := pairing.New()
	dir, err := configDir()
	if err != nil {
		return p
	}
	var old struct {
		Code     string `json:"code"`
		DeviceID string `json:"deviceId"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "relay.json"))
	if err != nil || json.Unmarshal(data, &old) != nil {
		return p
	}
	if old.DeviceID != "" {
		p.DeviceID = old.DeviceID
	}
	if c, err := relay.ParseCode(old.Code); err == nil {
		p.Key, p.Relay = c.Key, c.Addr()
		log.Printf("[pairing] Kept the group from the relay pairing code")
	}
	return p
}

func savePairing(p pairing.Pairing) error {
	f := pairingFile()
	if f == "" {
		return errors.New("could not locate config directory")
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	// The pairing key is a secret: keep it private to this user.
	return os.WriteFile(f, data, 0o600)
}

// updatePairing changes and saves the identity. When the group changed,
// links to the old group are dropped; the relay always restarts.
func (a *App) updatePairing(change func(p *pairing.Pairing)) error {
	before := a.pairing()
	a.pairMu.Lock()
	p := a.pair
	change(&p)
	if err := savePairing(p); err != nil {
		a.pairMu.Unlock()
		return err
	}
	a.pair = p
	a.pairMu.Unlock()

	if string(before.Key) != string(p.Key) && a.links != nil {
		a.links.CloseAll()
	}
	a.startRelay()
	a.kickConnector()
	return nil
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

// GetPairing returns this device's pairing code for the pairing dialog.
func (a *App) GetPairing() PairingInfo {
	p := a.pairing()
	return PairingInfo{DeviceID: p.DeviceID, Code: p.Code(), Relay: p.Relay}
}

// JoinPairing joins the group of the device that showed code. A code that
// names a relay also sets the relay.
func (a *App) JoinPairing(code string) error {
	key, relayAddr, err := pairing.ParseCode(code)
	if err != nil {
		return err
	}
	return a.updatePairing(func(p *pairing.Pairing) {
		p.Key = key
		if relayAddr != "" {
			p.Relay = relayAddr
		}
	})
}

// ResetPairing starts a new group: every other device is unpaired until it
// enters the new code.
func (a *App) ResetPairing() error {
	return a.updatePairing(func(p *pairing.Pairing) { p.Key = pairing.NewKey() })
}

// SetRelayServer sets the relay for remote use ("host" or "host:port");
// empty turns it off. Every device of the group needs the same relay.
func (a *App) SetRelayServer(addr string) error {
	norm, err := pairing.NormalizeRelay(addr)
	if err != nil {
		return err
	}
	return a.updatePairing(func(p *pairing.Pairing) { p.Relay = norm })
}
