// wifi_app.go shares Wi-Fi networks between paired devices, so joining a
// network on one lets the other join it too. The receiver always asks
// before joining.
//
// Packet type (must match mobile/lib/features/):
//
//	wifi.share   either way       wifiqr.Network {ssid, password, security, hidden}
//
// Wi-Fi QR codes ("WIFI:S:...;;") in received pictures are offered the same
// way, which is how a phone's Wi-Fi share screen reaches the laptop.
package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/link"
	"palt/internal/wifiqr"
)

func (a *App) setupWifiShare() {
	a.links.Handle("wifi.share", func(l *link.Link, p link.Packet) {
		var n wifiqr.Network
		if p.Decode(&n) != nil || n.SSID == "" {
			return
		}
		a.offerWifi(n, l.Peer.Name)
	})
}

// offerWifi asks whether to join a network another device shared.
func (a *App) offerWifi(n wifiqr.Network, from string) {
	if cur, err := currentWifi(); err == nil && cur.SSID == n.SSID {
		notify(Notification{Title: "Already on " + n.SSID, Body: from + " shared this Wi-Fi network", Icon: "network-wireless"})
		return
	}
	log.Printf("[wifi] %s shared network %q", from, n.SSID)
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "wifi_offer", map[string]any{"from": from, "ssid": n.SSID})
	}
	notify(Notification{
		Title:   "Join " + n.SSID + "?",
		Body:    from + " shared this Wi-Fi network",
		Icon:    "network-wireless",
		Urgent:  true,
		Actions: []NotifyAction{{Key: "join", Label: "Join"}, {Key: "no", Label: "Not now"}},
		OnAction: func(key string) {
			if key != "join" {
				return
			}
			if err := joinWifi(n); err != nil {
				log.Printf("[wifi] %v", err)
				notify(Notification{Title: "Wi-Fi not joined", Body: err.Error(), Icon: "network-wireless-offline"})
				return
			}
			notify(Notification{Title: "Joined " + n.SSID, Icon: "network-wireless"})
		},
	})
}

// scanForWifiQR offers any Wi-Fi QR code found in received pictures.
func (a *App) scanForWifiQR(paths []string, from string) {
	for _, p := range paths {
		switch strings.ToLower(filepath.Ext(p)) {
		case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp":
		default:
			continue
		}
		if n, err := wifiqr.FromImage(p); err == nil {
			a.offerWifi(n, from)
		}
	}
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

// GetCurrentWifi returns the name of the Wi-Fi network this computer is on,
// "" when it isn't on one it can share.
func (a *App) GetCurrentWifi() string {
	n, err := currentWifi()
	if err != nil {
		return ""
	}
	return n.SSID
}

// ShareWifi sends this computer's Wi-Fi network, with its password, to a
// paired device, which asks before saving it.
func (a *App) ShareWifi(name string) error {
	l, err := a.linkByName(name)
	if err != nil {
		return err
	}
	if !l.Supports("wifi.share") {
		return fmt.Errorf("update PALT on %s to share Wi-Fi", name)
	}
	n, err := currentWifi()
	if err != nil {
		return err
	}
	log.Printf("[wifi] Sharing %q with %s", n.SSID, name)
	return l.Send("wifi.share", n)
}
