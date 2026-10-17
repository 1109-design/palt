// notifications_app.go mirrors a paired phone's notifications on the
// desktop: shown as desktop notifications with the phone app's icon and
// action buttons, dismissed both ways, and answerable through "Reply".
//
// Packet types (must match mobile/lib/features/notification_sync.dart):
//
//	notification          phone → desktop  {id, app, appId, title, text, time,
//	                                        actions: [label], replyable, silent, icon?}
//	notification.removed  phone → desktop  {id}
//	notification.dismiss  desktop → phone  {id}
//	notification.action   desktop → phone  {id, index}
//	notification.reply    desktop → phone  {id, text}
package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/link"
)

const keepPhoneNotifications = 100

// PhoneNotification is one notification from a paired device.
type PhoneNotification struct {
	ID        string   `json:"id"`
	Device    string   `json:"device"`
	App       string   `json:"app"`
	AppID     string   `json:"appId"`
	Title     string   `json:"title"`
	Text      string   `json:"text"`
	Time      int64    `json:"time"` // ms since epoch
	Actions   []string `json:"actions"`
	Replyable bool     `json:"replyable"`
	Icon      string   `json:"icon,omitempty"` // local file path
}

type notificationState struct {
	mu     sync.Mutex
	shown  map[string]uint32               // device|id → desktop notification ID
	recent map[string][]*PhoneNotification // device → newest first
}

var safeAppID = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func (a *App) setupNotificationSync() {
	a.notes.shown = map[string]uint32{}
	a.notes.recent = map[string][]*PhoneNotification{}
	a.links.Handle("notification", a.onPhoneNotification)
	a.links.Handle("notification.removed", func(l *link.Link, p link.Packet) {
		var b struct{ ID string }
		if p.Decode(&b) == nil {
			a.removePhoneNotification(l.Peer.Name, b.ID)
		}
	})
}

// iconFile stores an app icon (base64 PNG) in the cache and returns its path.
func iconFile(appID, b64 string) string {
	if b64 == "" || appID == "" {
		return ""
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	dir = filepath.Join(dir, "palt", "icons")
	path := filepath.Join(dir, safeAppID.ReplaceAllString(appID, "_")+".png")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return ""
	}
	os.MkdirAll(dir, 0o755)
	if os.WriteFile(path, data, 0o644) != nil {
		return ""
	}
	return path
}

func (a *App) onPhoneNotification(l *link.Link, p link.Packet) {
	var b struct {
		PhoneNotification
		Icon   string `json:"icon"`
		Silent bool   `json:"silent"`
	}
	if p.Decode(&b) != nil || b.ID == "" {
		return
	}
	device := l.Peer.Name
	n := b.PhoneNotification
	n.Device = device
	n.Icon = iconFile(n.AppID, b.Icon)
	if n.Time == 0 {
		n.Time = time.Now().UnixMilli()
	}

	a.notes.mu.Lock()
	list := a.notes.recent[device]
	for i, old := range list {
		if old.ID == n.ID {
			list = append(list[:i], list[i+1:]...)
			break
		}
	}
	a.notes.recent[device] = append([]*PhoneNotification{&n}, list...)
	if len(a.notes.recent[device]) > keepPhoneNotifications {
		a.notes.recent[device] = a.notes.recent[device][:keepPhoneNotifications]
	}
	key := device + "|" + n.ID
	replaces := a.notes.shown[key]
	a.notes.mu.Unlock()
	a.emitPhoneNotifications(device)

	if !a.setting(device, "notifications") {
		return
	}
	actions := []NotifyAction{{Key: "default", Label: "Open"}}
	if n.Replyable {
		actions = append(actions, NotifyAction{Key: "reply", Label: "Reply"})
	}
	for i, label := range n.Actions {
		if label != "" && i < 3 {
			actions = append(actions, NotifyAction{Key: fmt.Sprintf("a%d", i), Label: label})
		}
	}
	title := n.Title
	if title == "" {
		title = n.App
	}
	body := n.Text
	if n.App != "" && n.Title != "" {
		body = n.Text + "\n— " + n.App + " on " + device
	}
	id := notify(Notification{
		Title:    title,
		Body:     body,
		Icon:     n.Icon,
		Actions:  actions,
		Replaces: replaces,
		Silent:   b.Silent,
		Category: "im.received",
		OnAction: func(k string) { a.phoneNotificationAction(l, device, n, k) },
		OnClosed: func(byUser bool) {
			a.notes.mu.Lock()
			if a.notes.shown[key] != 0 {
				delete(a.notes.shown, key)
			}
			a.notes.mu.Unlock()
			if byUser {
				l.Send("notification.dismiss", map[string]string{"id": n.ID})
			}
		},
	})
	if id != 0 {
		a.notes.mu.Lock()
		a.notes.shown[key] = id
		a.notes.mu.Unlock()
	}
}

func (a *App) phoneNotificationAction(l *link.Link, device string, n PhoneNotification, key string) {
	switch {
	case key == "reply":
		a.showWindow()
		wailsruntime.EventsEmit(a.ctx, "notification_reply", n)
	case len(key) > 1 && key[0] == 'a':
		var i int
		fmt.Sscanf(key[1:], "%d", &i)
		l.Send("notification.action", map[string]any{"id": n.ID, "index": i})
	default:
		a.showWindow()
		wailsruntime.EventsEmit(a.ctx, "tray_open_device", device)
	}
}

// removePhoneNotification drops a notification the phone no longer shows.
func (a *App) removePhoneNotification(device, id string) {
	a.notes.mu.Lock()
	key := device + "|" + id
	nid := a.notes.shown[key]
	delete(a.notes.shown, key)
	list := a.notes.recent[device]
	for i, n := range list {
		if n.ID == id {
			a.notes.recent[device] = append(list[:i], list[i+1:]...)
			break
		}
	}
	a.notes.mu.Unlock()
	closeNotification(nid)
	a.emitPhoneNotifications(device)
}

func (a *App) emitPhoneNotifications(device string) {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "phone_notifications", map[string]any{
			"device": device, "items": a.GetPhoneNotifications(device),
		})
	}
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

// GetPhoneNotifications returns a device's current notifications, newest first.
func (a *App) GetPhoneNotifications(device string) []PhoneNotification {
	a.notes.mu.Lock()
	defer a.notes.mu.Unlock()
	out := []PhoneNotification{}
	for _, n := range a.notes.recent[device] {
		out = append(out, *n)
	}
	return out
}

// ReplyToNotification answers a phone notification, as if typed in it.
func (a *App) ReplyToNotification(device, id, text string) error {
	l, err := a.linkByName(device)
	if err != nil {
		return err
	}
	return l.Send("notification.reply", map[string]string{"id": id, "text": text})
}

// DismissPhoneNotification dismisses a notification on the phone.
func (a *App) DismissPhoneNotification(device, id string) error {
	l, err := a.linkByName(device)
	if err != nil {
		return err
	}
	a.removePhoneNotification(device, id)
	return l.Send("notification.dismiss", map[string]string{"id": id})
}

// RunPhoneNotificationAction presses one of a notification's buttons on the phone.
func (a *App) RunPhoneNotificationAction(device, id string, index int) error {
	l, err := a.linkByName(device)
	if err != nil {
		return err
	}
	return l.Send("notification.action", map[string]any{"id": id, "index": index})
}
