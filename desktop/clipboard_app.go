// clipboard_app.go syncs the clipboard with paired devices: copying on one
// device makes the text available to paste on the others. Per-device
// switches decide which way it flows. A short history is kept.
//
// Packet types (must match mobile/lib/features/clipboard.dart):
//
//	clipboard  either way  {text}
package main

import (
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/link"
)

const (
	maxClipboardText = 256 * 1024
	keepClipboard    = 50
)

// ClipboardEntry is one item of clipboard history.
type ClipboardEntry struct {
	Text string `json:"text"`
	From string `json:"from"` // device name, "" for this computer
	Time int64  `json:"time"` // ms since epoch
}

type clipboardState struct {
	mu      sync.Mutex
	last    string // last text seen or set, so syncing doesn't echo
	history []ClipboardEntry
}

func (a *App) setupClipboard() {
	a.links.Handle("clipboard", a.onRemoteClipboard)
	watchClipboard(a.onLocalClipboard)
}

func (a *App) remember(text, from string) bool {
	a.clip.mu.Lock()
	defer a.clip.mu.Unlock()
	if text == a.clip.last {
		return false
	}
	a.clip.last = text
	a.clip.history = append([]ClipboardEntry{{Text: text, From: from, Time: time.Now().UnixMilli()}}, a.clip.history...)
	if len(a.clip.history) > keepClipboard {
		a.clip.history = a.clip.history[:keepClipboard]
	}
	return true
}

func (a *App) emitClipboard() {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "clipboard_history", a.GetClipboardHistory())
	}
}

// onLocalClipboard sends a new copy on this computer to devices that want it.
func (a *App) onLocalClipboard(text string) {
	if text == "" || len(text) > maxClipboardText || !a.remember(text, "") {
		return
	}
	a.emitClipboard()
	for _, l := range a.links.Links() {
		if l.Supports("clipboard") && a.setting(l.Peer.Name, "clipboardSend") {
			l.Send("clipboard", map[string]string{"text": text})
		}
	}
}

func (a *App) onRemoteClipboard(l *link.Link, p link.Packet) {
	var b struct{ Text string }
	if p.Decode(&b) != nil || b.Text == "" || len(b.Text) > maxClipboardText {
		return
	}
	if !a.setting(l.Peer.Name, "clipboardReceive") || !a.remember(b.Text, l.Peer.Name) {
		return
	}
	a.setClipboard(b.Text)
	a.emitClipboard()
}

// setClipboard puts text on this computer's clipboard.
func (a *App) setClipboard(text string) error {
	a.clip.mu.Lock()
	a.clip.last = text
	a.clip.mu.Unlock()
	if setClipboardViaHelper(text) {
		return nil
	}
	return wailsruntime.ClipboardSetText(a.ctx, text)
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

// ClipboardHelper describes the GNOME clipboard helper for the UI.
type ClipboardHelper struct {
	Running   bool `json:"running"`
	Installed bool `json:"installed"`
}

func (a *App) GetClipboardHelper() ClipboardHelper {
	return ClipboardHelper{Running: clipboardHelperRunning(), Installed: clipboardHelperInstalled()}
}

// InstallClipboardHelper installs and enables the helper (active after the
// next login on Wayland).
func (a *App) InstallClipboardHelper() error { return installClipboardHelper() }

// GetClipboardHistory returns recent clipboard texts, newest first.
func (a *App) GetClipboardHistory() []ClipboardEntry {
	a.clip.mu.Lock()
	defer a.clip.mu.Unlock()
	return append([]ClipboardEntry{}, a.clip.history...)
}

// CopyFromHistory puts an earlier clipboard text back on the clipboard.
func (a *App) CopyFromHistory(text string) error { return a.setClipboard(text) }
