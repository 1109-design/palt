// features_app.go holds the everyday live features that run over links:
// phone battery and signal, ringing a device to find it, pings, and
// desktop notifications for incoming messages.
//
// Packet types (must match mobile/lib/features/):
//
//	battery      phone → desktop  {level 0-100, charging}
//	connectivity phone → desktop  {network "5G"/"LTE"/"Wi-Fi"..., signal 0-4, -1 unknown}
//	ring         either way       {} start ringing to be found
//	ring.stop    either way       {} stop ringing
//	user_ping    either way       {message?}
package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/link"
)

const (
	lowBatteryLevel = 15
	ringSound       = "/usr/share/sounds/freedesktop/stereo/phone-incoming-call.oga"
	ringFor         = 2 * time.Minute
)

// DeviceStatus is what a paired device reports about itself.
type DeviceStatus struct {
	Battery  int    `json:"battery"` // -1 unknown
	Charging bool   `json:"charging"`
	Network  string `json:"network"`
	Signal   int    `json:"signal"` // 0-4, -1 unknown
}

type featureState struct {
	mu          sync.Mutex
	status      map[string]DeviceStatus // by device name
	lowNotified map[string]bool

	ringMu    sync.Mutex
	ringStop  chan struct{}
	ringNotif uint32
}

func (a *App) setupFeatures() {
	a.features.status = map[string]DeviceStatus{}
	a.features.lowNotified = map[string]bool{}

	a.links.Handle("battery", a.onBattery)
	a.links.Handle("connectivity", a.onConnectivity)
	a.links.Handle("ring", func(l *link.Link, p link.Packet) { a.startRinging(l.Peer.Name) })
	a.links.Handle("ring.stop", func(l *link.Link, p link.Packet) { a.StopRinging() })
	a.links.Handle("user_ping", func(l *link.Link, p link.Packet) {
		var b struct{ Message string }
		p.Decode(&b)
		msg := b.Message
		if msg == "" {
			msg = "Ping!"
		}
		notify(Notification{Title: l.Peer.Name, Body: msg, Icon: "phone"})
	})
}

func (a *App) statusOf(name string) DeviceStatus {
	a.features.mu.Lock()
	defer a.features.mu.Unlock()
	if st, ok := a.features.status[name]; ok {
		return st
	}
	return DeviceStatus{Battery: -1, Signal: -1}
}

func (a *App) updateStatus(name string, change func(st *DeviceStatus)) {
	a.features.mu.Lock()
	st, ok := a.features.status[name]
	if !ok {
		st = DeviceStatus{Battery: -1, Signal: -1}
	}
	change(&st)
	a.features.status[name] = st
	a.features.mu.Unlock()
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "device_status", a.GetDeviceStatus())
	}
	a.refreshTray()
}

func (a *App) onBattery(l *link.Link, p link.Packet) {
	var b struct {
		Level    int  `json:"level"`
		Charging bool `json:"charging"`
	}
	if p.Decode(&b) != nil || b.Level < 0 || b.Level > 100 {
		return
	}
	name := l.Peer.Name
	a.updateStatus(name, func(st *DeviceStatus) { st.Battery, st.Charging = b.Level, b.Charging })

	a.features.mu.Lock()
	warn := b.Level <= lowBatteryLevel && !b.Charging && !a.features.lowNotified[name]
	if warn {
		a.features.lowNotified[name] = true
	} else if b.Charging || b.Level > lowBatteryLevel+5 {
		a.features.lowNotified[name] = false
	}
	a.features.mu.Unlock()
	if warn {
		notify(Notification{
			Title: fmt.Sprintf("%s battery low", name),
			Body:  fmt.Sprintf("%d%% left", b.Level),
			Icon:  "battery-caution",
		})
	}
}

func (a *App) onConnectivity(l *link.Link, p link.Packet) {
	var b struct {
		Network string `json:"network"`
		Signal  int    `json:"signal"`
	}
	if p.Decode(&b) != nil {
		return
	}
	if b.Signal < -1 || b.Signal > 4 {
		b.Signal = -1
	}
	a.updateStatus(l.Peer.Name, func(st *DeviceStatus) { st.Network, st.Signal = b.Network, b.Signal })
}

// ─── Find my desktop ─────────────────────────────────────────────────────────

// startRinging plays a loud sound until stopped or ringFor passes, and shows
// the window with a "Found it" button.
func (a *App) startRinging(from string) {
	a.features.ringMu.Lock()
	if a.features.ringStop != nil {
		a.features.ringMu.Unlock()
		return
	}
	stop := make(chan struct{})
	a.features.ringStop = stop
	a.features.ringMu.Unlock()

	log.Printf("[ring] Ringing, asked by %s", from)
	a.showWindow()
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "ringing", map[string]any{"from": from, "on": true})
	}
	id := notify(Notification{
		Title:    "PALT is ringing",
		Body:     from + " is looking for this computer",
		Icon:     "audio-volume-high",
		Urgent:   true,
		Actions:  []NotifyAction{{Key: "stop", Label: "Found it"}},
		OnAction: func(string) { a.StopRinging() },
	})
	a.features.ringMu.Lock()
	a.features.ringNotif = id
	a.features.ringMu.Unlock()

	go func() {
		deadline := time.After(ringFor)
		for {
			player := exec.Command("paplay", ringSound)
			if _, err := os.Stat(ringSound); err != nil {
				player = exec.Command("canberra-gtk-play", "-i", "phone-incoming-call")
			}
			done := make(chan struct{})
			if err := player.Start(); err != nil {
				log.Printf("[ring] No sound player: %v", err)
				close(done)
			} else {
				go func() { player.Wait(); close(done) }()
			}
			select {
			case <-stop:
				if player.Process != nil {
					player.Process.Kill()
				}
				return
			case <-deadline:
				if player.Process != nil {
					player.Process.Kill()
				}
				a.StopRinging()
				return
			case <-done:
			}
		}
	}()
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

// GetDeviceStatus returns battery and signal per linked device name.
func (a *App) GetDeviceStatus() map[string]DeviceStatus {
	out := map[string]DeviceStatus{}
	if a.links == nil {
		return out
	}
	for _, l := range a.links.Links() {
		out[l.Peer.Name] = a.statusOf(l.Peer.Name)
	}
	return out
}

// StopRinging silences this computer.
func (a *App) StopRinging() {
	a.features.ringMu.Lock()
	stop, id := a.features.ringStop, a.features.ringNotif
	a.features.ringStop, a.features.ringNotif = nil, 0
	a.features.ringMu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	closeNotification(id)
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "ringing", map[string]any{"on": false})
	}
}

// linkByName returns the link to a paired device or an error for the UI.
func (a *App) linkByName(name string) (*link.Link, error) {
	if a.links != nil {
		if l := a.links.GetByName(name); l != nil {
			return l, nil
		}
	}
	return nil, fmt.Errorf("%s isn't connected as a paired device", name)
}

// RingDevice makes a paired device ring loudly, even on silent.
func (a *App) RingDevice(name string) error {
	l, err := a.linkByName(name)
	if err != nil {
		return err
	}
	if !l.Supports("ring") {
		return fmt.Errorf("update PALT on %s to ring it", name)
	}
	return l.Send("ring", map[string]any{})
}

// PingDevice shows a "Ping!" notification on a paired device.
func (a *App) PingDevice(name string) error {
	l, err := a.linkByName(name)
	if err != nil {
		return err
	}
	return l.Send("user_ping", map[string]any{})
}

// SendClipboard sends this computer's clipboard text to a device.
func (a *App) SendClipboard(name string) error {
	text, err := wailsruntime.ClipboardGetText(a.ctx)
	if err != nil || strings.TrimSpace(text) == "" {
		return fmt.Errorf("the clipboard has no text")
	}
	for _, p := range a.mergedPeers() {
		if p.DeviceName == name {
			return a.SendText(p.IPAddress, p.Port, name, text)
		}
	}
	return fmt.Errorf("%s is not connected", name)
}

// ─── Incoming messages ───────────────────────────────────────────────────────

// singleURL returns text as a URL when it is nothing but one web link.
func singleURL(text string) (string, bool) {
	t := strings.TrimSpace(text)
	if strings.ContainsAny(t, " \n\t") {
		return "", false
	}
	u, err := url.Parse(t)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	return t, true
}

// notifyText shows an incoming message as a desktop notification while the
// window is in the tray. Links get an "Open" button.
func (a *App) notifyText(from, text string, copied bool) {
	if !a.windowHidden.Load() {
		return
	}
	body := text
	if len(body) > 300 {
		body = body[:300] + "…"
	}
	if copied {
		body += "\n(copied to clipboard)"
	}
	actions := []NotifyAction{{Key: "default", Label: "Open chat"}}
	link, isURL := singleURL(text)
	if isURL {
		actions = append(actions, NotifyAction{Key: "open", Label: "Open link"})
	}
	notify(Notification{
		Title:   from,
		Body:    body,
		Icon:    "mail-message-new",
		Actions: actions,
		OnAction: func(key string) {
			if key == "open" && isURL {
				exec.Command("xdg-open", link).Start()
				return
			}
			a.showWindow()
			wailsruntime.EventsEmit(a.ctx, "tray_open_device", from)
		},
	})
}
