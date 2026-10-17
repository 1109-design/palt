// calls_app.go shows a paired phone's calls on the desktop: who's calling
// (with a button to silence the phone), media paused for the call and
// resumed after, and missed calls.
//
// Packet types (must match mobile/lib/features/calls.dart):
//
//	call       phone → desktop  {event: ringing|talking|missed|ended, number, name, photo?}
//	call.mute  desktop → phone  {}
package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/link"
)

type callState struct {
	mu     sync.Mutex
	notif  map[string]uint32   // device → ringing notification
	paused map[string][]string // device → players we paused
}

func (a *App) setupCalls() {
	a.calls.notif = map[string]uint32{}
	a.calls.paused = map[string][]string{}
	a.links.Handle("call", a.onCall)
}

// callerPhoto stores a contact photo for the notification icon.
func callerPhoto(b64 string) string {
	if b64 == "" {
		return ""
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return ""
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(dir, "palt", "caller.jpg")
	os.MkdirAll(filepath.Dir(path), 0o755)
	if os.WriteFile(path, data, 0o644) != nil {
		return ""
	}
	return path
}

func (a *App) onCall(l *link.Link, p link.Packet) {
	var b struct {
		Event  string `json:"event"`
		Number string `json:"number"`
		Name   string `json:"name"`
		Photo  string `json:"photo"`
	}
	if p.Decode(&b) != nil {
		return
	}
	device := l.Peer.Name
	who := b.Name
	if who == "" {
		who = b.Number
	}
	if who == "" {
		who = "Unknown number"
	}
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "call", map[string]string{"device": device, "event": b.Event, "who": who, "number": b.Number})
	}

	a.calls.mu.Lock()
	ringing := a.calls.notif[device]
	a.calls.mu.Unlock()

	// Any call, incoming or one you make, pauses what's playing here.
	pause := func() {
		if !a.setting(device, "pauseOnCall") {
			return
		}
		if paused := pausePlaying(); len(paused) > 0 {
			a.calls.mu.Lock()
			a.calls.paused[device] = append(a.calls.paused[device], paused...)
			a.calls.mu.Unlock()
		}
	}

	switch b.Event {
	case "ringing":
		pause()
		body := device + " is ringing"
		if b.Name != "" && b.Number != "" {
			body = b.Number + " · " + body
		}
		icon := callerPhoto(b.Photo)
		if icon == "" {
			icon = "call-start"
		}
		id := notify(Notification{
			Title:    "Call from " + who,
			Body:     body,
			Icon:     icon,
			Urgent:   true,
			Category: "call.incoming",
			Replaces: ringing,
			Actions:  []NotifyAction{{Key: "mute", Label: "Mute ringer"}},
			OnAction: func(k string) {
				if k == "mute" {
					l.Send("call.mute", map[string]any{})
				}
			},
		})
		a.calls.mu.Lock()
		a.calls.notif[device] = id
		a.calls.mu.Unlock()

	case "talking":
		closeNotification(ringing)
		pause() // outgoing calls start here; a no-op if already paused

	case "missed", "ended":
		closeNotification(ringing)
		a.calls.mu.Lock()
		delete(a.calls.notif, device)
		paused := a.calls.paused[device]
		delete(a.calls.paused, device)
		a.calls.mu.Unlock()
		resumePlayers(paused)
		if b.Event == "missed" {
			notify(Notification{
				Title:    "Missed call from " + who,
				Body:     "On " + device,
				Icon:     "call-missed",
				Category: "call.unanswered",
			})
		}
	}
}
