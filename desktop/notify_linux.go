// notify_linux.go shows desktop notifications through the freedesktop
// Notifications service (GNOME Shell, KDE, ...), with action buttons.
package main

import (
	"log"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	notifyDest  = "org.freedesktop.Notifications"
	notifyPath  = "/org/freedesktop/Notifications"
	notifyIface = "org.freedesktop.Notifications"
)

// Notification is one desktop notification.
type Notification struct {
	Title string
	Body  string
	Icon  string // icon name or file path; "" = PALT's icon
	// Actions are shown as buttons; "default" is a click on the
	// notification itself. OnAction gets the key of the one chosen.
	Actions  []NotifyAction
	OnAction func(key string)
	// Urgent notifications stay until dismissed.
	Urgent bool
	// Replaces is the ID of an earlier notification to update in place.
	Replaces uint32
	// OnClosed is called when the notification goes away; byUser is true
	// when the user dismissed it.
	OnClosed func(byUser bool)
	// Silent notifications make no sound (e.g. updates of earlier ones).
	Silent bool
	// Category is a freedesktop category such as "im.received".
	Category string
}

type NotifyAction struct{ Key, Label string }

type notifier struct {
	once    sync.Once
	conn    *dbus.Conn
	mu      sync.Mutex
	pending map[uint32]Notification
}

var desktopNotifier notifier

func (n *notifier) init() {
	n.once.Do(func() {
		conn, err := dbus.ConnectSessionBus()
		if err != nil {
			log.Printf("[notify] No session bus: %v", err)
			return
		}
		n.conn = conn
		n.pending = map[uint32]Notification{}
		conn.AddMatchSignal(dbus.WithMatchInterface(notifyIface), dbus.WithMatchObjectPath(notifyPath))
		signals := make(chan *dbus.Signal, 16)
		conn.Signal(signals)
		go func() {
			for s := range signals {
				if len(s.Body) < 2 {
					continue
				}
				id, _ := s.Body[0].(uint32)
				n.mu.Lock()
				nt, ok := n.pending[id]
				if ok && s.Name == notifyIface+".NotificationClosed" {
					delete(n.pending, id)
				}
				n.mu.Unlock()
				if !ok {
					continue
				}
				switch s.Name {
				case notifyIface + ".ActionInvoked":
					if key, _ := s.Body[1].(string); nt.OnAction != nil {
						go nt.OnAction(key)
					}
				case notifyIface + ".NotificationClosed":
					reason, _ := s.Body[1].(uint32)
					if nt.OnClosed != nil {
						go nt.OnClosed(reason == 2)
					}
				}
			}
		}()
	})
}

// notify shows a notification and returns its ID (0 on failure).
func notify(nt Notification) uint32 {
	n := &desktopNotifier
	n.init()
	if n.conn == nil {
		return 0
	}
	actions := []string{}
	for _, a := range nt.Actions {
		actions = append(actions, a.Key, a.Label)
	}
	icon := nt.Icon
	if icon == "" {
		icon = "palt"
	}
	urgency := byte(1)
	timeout := int32(-1)
	if nt.Urgent {
		urgency, timeout = 2, 0
	}
	hints := map[string]dbus.Variant{
		"urgency":       dbus.MakeVariant(urgency),
		"desktop-entry": dbus.MakeVariant("palt"),
	}
	if nt.Silent {
		hints["suppress-sound"] = dbus.MakeVariant(true)
	}
	if nt.Category != "" {
		hints["category"] = dbus.MakeVariant(nt.Category)
	}
	var id uint32
	err := n.conn.Object(notifyDest, notifyPath).Call(notifyIface+".Notify", 0,
		"PALT", nt.Replaces, icon, nt.Title, nt.Body, actions, hints, timeout).Store(&id)
	if err != nil {
		log.Printf("[notify] %v", err)
		return 0
	}
	n.mu.Lock()
	n.pending[id] = nt
	n.mu.Unlock()
	return id
}

// closeNotification removes a notification shown earlier.
func closeNotification(id uint32) {
	n := &desktopNotifier
	if id == 0 || n.conn == nil {
		return
	}
	n.conn.Object(notifyDest, notifyPath).Call(notifyIface+".CloseNotification", 0, id)
}
