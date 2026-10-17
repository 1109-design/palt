// calls_app.go shows a paired phone's calls on the desktop: who's calling
// (with a button to silence the phone), media paused for the call and
// resumed after, and missed calls.
//
// It also reads the call log and the SIMs, and places, answers and ends
// calls on the phone (the call's audio stays on the phone).
//
// Packet types (must match mobile/lib/features/calls.dart). Requests carry
// an id and are answered by "<type>.reply" with the same id.
//
//	call         phone → desktop  {event: ringing|talking|missed|ended, number,
//	                               name, photo?, direction, since}
//	call.mute    desktop → phone  {}
//	call.place   desktop → phone  {number, sim} → {error?}
//	call.answer  desktop → phone  {} → {error?}
//	call.end     desktop → phone  {} → {error?}
//	calls.log    desktop → phone  {before, limit} → {calls}
//	phone.sims   desktop → phone  {} → {sims}
package main

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/link"
)

// ActiveCall is a call in progress on a paired phone.
type ActiveCall struct {
	Device    string `json:"device"`
	State     string `json:"state"`     // ringing | talking
	Direction string `json:"direction"` // incoming | outgoing
	Number    string `json:"number"`
	Name      string `json:"name"`
	Photo     string `json:"photo"` // data: URL or ""
	Since     int64  `json:"since"` // unix ms picked up / dialled, 0 ringing
}

// CallLogEntry is one call in a phone's call log.
type CallLogEntry struct {
	Number   string `json:"number"`
	Name     string `json:"name"`
	Type     string `json:"type"` // incoming outgoing missed rejected blocked voicemail
	Date     int64  `json:"date"`
	Duration int64  `json:"duration"` // seconds
	SIM      int    `json:"sim"`      // slot, 0 unknown
}

// SIM is one of a phone's active SIMs.
type SIM struct {
	ID      int    `json:"id"`   // subscription id, used to pick it
	Slot    int    `json:"slot"` // 1-based
	Name    string `json:"name"`
	Carrier string `json:"carrier"`
}

type callState struct {
	mu     sync.Mutex
	notif  map[string]uint32      // device → ringing notification
	paused map[string][]string    // device → players we paused
	active map[string]*ActiveCall // device → call in progress
}

func (a *App) setupCalls() {
	a.calls.notif = map[string]uint32{}
	a.calls.paused = map[string][]string{}
	a.calls.active = map[string]*ActiveCall{}
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
		Event     string `json:"event"`
		Number    string `json:"number"`
		Name      string `json:"name"`
		Photo     string `json:"photo"`
		Direction string `json:"direction"`
		Since     int64  `json:"since"`
	}
	if p.Decode(&b) != nil {
		return
	}
	device := l.Peer.Name
	a.trackCall(device, b.Event, ActiveCall{
		Device: device, Direction: b.Direction, Number: b.Number, Name: b.Name, Since: b.Since,
		Photo: dataURL(b.Photo),
	})
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
			Actions: []NotifyAction{
				{Key: "answer", Label: "Answer"}, {Key: "decline", Label: "Decline"}, {Key: "mute", Label: "Silence"},
			},
			OnAction: func(k string) {
				switch k {
				case "mute":
					l.Send("call.mute", map[string]any{})
				case "answer":
					a.AnswerCall(device)
				case "decline":
					a.EndCall(device)
				default:
					a.showWindow()
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

func dataURL(b64 string) string {
	if b64 == "" {
		return ""
	}
	return "data:image/jpeg;base64," + b64
}

// trackCall keeps the call in progress per device and tells the window.
func (a *App) trackCall(device, event string, c ActiveCall) {
	a.calls.mu.Lock()
	switch event {
	case "ringing", "talking":
		c.State = event
		if prev := a.calls.active[device]; prev != nil && c.Number == "" {
			c.Number, c.Name, c.Photo = prev.Number, prev.Name, prev.Photo
		}
		if c.State == "talking" && c.Since == 0 {
			c.Since = time.Now().UnixMilli()
		}
		a.calls.active[device] = &c
	default:
		delete(a.calls.active, device)
	}
	a.calls.mu.Unlock()
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "active_calls", a.GetActiveCalls())
	}
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

// GetActiveCalls returns the calls in progress on linked phones.
func (a *App) GetActiveCalls() []ActiveCall {
	a.calls.mu.Lock()
	defer a.calls.mu.Unlock()
	out := []ActiveCall{}
	for _, c := range a.calls.active {
		if a.links != nil && a.links.GetByName(c.Device) != nil {
			out = append(out, *c)
		}
	}
	return out
}

func (a *App) callsLink(device string) (*link.Link, error) {
	l, err := a.linkByName(device)
	if err != nil {
		return nil, err
	}
	if !l.Supports("call.place") {
		return nil, errors.New("update PALT on " + device + " for calls")
	}
	return l, nil
}

// callRequest sends a call command and returns the phone's error, if any.
func (a *App) callRequest(device, t string, body any) error {
	l, err := a.callsLink(device)
	if err != nil {
		return err
	}
	p, err := l.Request(t, body, smsRequestTimeout)
	if err != nil {
		return err
	}
	var r struct{}
	return smsReply(p, &r)
}

// PlaceCall dials a number on the phone, on SIM simID (0 = phone's default).
func (a *App) PlaceCall(device, number string, simID int) error {
	if number == "" {
		return errors.New("no number to call")
	}
	return a.callRequest(device, "call.place", map[string]any{"number": number, "sim": simID})
}

// AnswerCall picks up the phone's ringing call.
func (a *App) AnswerCall(device string) error {
	return a.callRequest(device, "call.answer", map[string]any{})
}

// EndCall hangs up (or declines) the phone's call.
func (a *App) EndCall(device string) error {
	return a.callRequest(device, "call.end", map[string]any{})
}

// GetCallLog returns up to 80 calls older than before (unix ms, 0 = newest).
func (a *App) GetCallLog(device string, before int64) ([]CallLogEntry, error) {
	l, err := a.callsLink(device)
	if err != nil {
		return nil, err
	}
	p, err := l.Request("calls.log", map[string]int64{"before": before, "limit": 80}, smsRequestTimeout)
	if err != nil {
		return nil, err
	}
	var r struct {
		Calls []CallLogEntry `json:"calls"`
	}
	if err := smsReply(p, &r); err != nil {
		return nil, err
	}
	if r.Calls == nil {
		r.Calls = []CallLogEntry{}
	}
	return r.Calls, nil
}

// GetSIMs returns the phone's active SIMs (one or two).
func (a *App) GetSIMs(device string) ([]SIM, error) {
	l, err := a.callsLink(device)
	if err != nil {
		return nil, err
	}
	p, err := l.Request("phone.sims", map[string]any{}, smsRequestTimeout)
	if err != nil {
		return nil, err
	}
	var r struct {
		Sims []SIM `json:"sims"`
	}
	if err := smsReply(p, &r); err != nil {
		return nil, err
	}
	if r.Sims == nil {
		r.Sims = []SIM{}
	}
	return r.Sims, nil
}

// MuteRinger silences the phone's ringing (the call keeps ringing quietly).
func (a *App) MuteRinger(device string) error {
	l, err := a.linkByName(device)
	if err != nil {
		return err
	}
	return l.Send("call.mute", map[string]any{})
}
