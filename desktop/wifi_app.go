// wifi_app.go shares Wi-Fi networks between paired devices, so joining a
// network on one lets the other join it too. The receiver always asks
// before joining.
//
// Packet type (must match mobile/lib/features/):
//
//	wifi.share        either way  wifiqr.Network {ssid, password, security, hidden}
//	internet.request  either way  {} "I have no internet, share yours"
//
// Wi-Fi QR codes ("WIFI:S:...;;") in received pictures are offered the same
// way, which is how a phone's Wi-Fi share screen reaches the laptop.
package main

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/link"
	"palt/internal/wifiqr"
)

func (a *App) setupWifiShare() {
	a.links.Handle("internet.request", func(l *link.Link, p link.Packet) {
		go func() {
			if err := a.ShareInternet(l.Peer.Name); err != nil {
				log.Printf("[wifi] Internet for %s: %v", l.Peer.Name, err)
				l.Send("user_ping", map[string]any{"message": "Couldn't share internet: " + err.Error()})
			}
		}()
	})
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

// ─── Share internet ──────────────────────────────────────────────────────────

// ShareInternet gives a paired device this computer's internet: the same
// Wi-Fi when this computer is on Wi-Fi, else a hotspot from its Wi-Fi card.
// The device asks before joining either.
func (a *App) ShareInternet(name string) error {
	if !localStatus().Internet {
		return fmt.Errorf("this computer has no internet to share")
	}
	l, err := a.linkByName(name)
	if err != nil {
		return err
	}
	if !l.Supports("wifi.share") {
		return fmt.Errorf("update PALT on %s to share internet", name)
	}
	n, err := startHotspot()
	if errors.Is(err, errOnWifi) {
		return a.ShareWifi(name)
	}
	if err != nil {
		return err
	}
	log.Printf("[wifi] Hotspot %q on; offering it to %s", n.SSID, name)
	a.emitHotspot()
	return l.Send("wifi.share", n)
}

// StopSharingInternet turns this computer's hotspot off.
func (a *App) StopSharingInternet() error {
	err := stopHotspot()
	a.emitHotspot()
	return err
}

// AskForInternet asks a paired phone to share its mobile data (it shows
// how: hotspot or USB tethering).
func (a *App) AskForInternet(name string) error {
	l, err := a.linkByName(name)
	if err != nil {
		return err
	}
	if !l.Supports("internet.request") {
		return fmt.Errorf("update PALT on %s for this", name)
	}
	return l.Send("internet.request", map[string]any{})
}

// GetHotspotActive reports whether this computer's hotspot is on.
func (a *App) GetHotspotActive() bool { return hotspotActive() }

func (a *App) emitHotspot() {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "hotspot", hotspotActive())
	}
	a.refreshTray()
}

// offerInternetHelp notices when one side of a link has internet and the
// other doesn't, and offers the fix once per outage.
func (a *App) offerInternetHelp(name string, cur DeviceStatus) {
	a.features.mu.Lock()
	offered := a.features.offlineOffered[name]
	if cur.Internet {
		delete(a.features.offlineOffered, name)
	}
	a.features.mu.Unlock()
	if cur.Internet || offered {
		return
	}
	// A device joining a network reports no internet for a moment before
	// the check completes; offer only if it is still offline after that.
	time.AfterFunc(10*time.Second, func() {
		if a.statusOf(name).Internet || !localStatus().Internet || a.links.GetByName(name) == nil {
			return
		}
		a.features.mu.Lock()
		if a.features.offlineOffered[name] {
			a.features.mu.Unlock()
			return
		}
		a.features.offlineOffered[name] = true
		a.features.mu.Unlock()
		a.notifyNoInternet(name)
	})
}

func (a *App) notifyNoInternet(name string) {
	notify(Notification{
		Title:   name + " has no internet",
		Body:    "Share this computer's internet with it?",
		Icon:    "network-wireless",
		Actions: []NotifyAction{{Key: "share", Label: "Share internet"}},
		OnAction: func(key string) {
			if key != "share" {
				return
			}
			if err := a.ShareInternet(name); err != nil {
				notify(Notification{Title: "Couldn't share internet", Body: err.Error(), Icon: "dialog-warning"})
			}
		},
	})
}
