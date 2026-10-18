// bluetooth_app.go shows each device's Bluetooth on the other and turns it
// on and off: whether it's on, and what's connected (with battery).
//
// Phones can't switch Bluetooth by themselves on Android 13+: turning one
// on shows Android's "turn on Bluetooth?" prompt there, and turning it off
// opens its Bluetooth settings (result "asked" / "settings").
//
// Packet types (must match mobile/lib/features/bluetooth.dart). Requests
// carry an id and are answered by "<type>.reply" with the same id.
//
//	bluetooth      either way  BluetoothState, on link up and on change
//	bluetooth.set  either way  {on} → {result: done|asked|settings, error?}
package main

import (
	"errors"
	"log"
	"reflect"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/link"
)

// BluetoothDevice is a connected Bluetooth device.
type BluetoothDevice struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`    // headset, phone, mouse, … ("" unknown)
	Battery int    `json:"battery"` // 0-100, -1 unknown
}

// BluetoothState is a device's Bluetooth.
type BluetoothState struct {
	Available bool              `json:"available"` // has a Bluetooth adapter
	On        bool              `json:"on"`
	Devices   []BluetoothDevice `json:"devices"` // connected ones
}

var peerBluetooth = struct {
	sync.Mutex
	m map[string]BluetoothState // by device name
}{m: map[string]BluetoothState{}}

func (a *App) setupBluetooth() {
	a.links.Handle("bluetooth", func(l *link.Link, p link.Packet) {
		var st BluetoothState
		if p.Decode(&st) != nil {
			return
		}
		peerBluetooth.Lock()
		peerBluetooth.m[l.Peer.Name] = st
		peerBluetooth.Unlock()
		wailsruntime.EventsEmit(a.ctx, "bluetooth_status", l.Peer.Name)
	})
	a.links.Handle("bluetooth.set", func(l *link.Link, p link.Packet) {
		var b struct {
			On bool `json:"on"`
		}
		p.Decode(&b)
		reply := map[string]string{"result": "done"}
		if err := setLocalBluetooth(b.On); err != nil {
			reply["error"] = err.Error()
		}
		log.Printf("[bluetooth] %s turned Bluetooth %v: %s", l.Peer.Name, b.On, reply["error"])
		l.SendID("bluetooth.set.reply", p.ID, reply)
		go a.sendBluetooth(nil)
	})

	// Tell new links, and everyone on a change.
	go func() {
		var last BluetoothState
		told := map[*link.Link]bool{}
		for range time.Tick(4 * time.Second) {
			st := localBluetooth()
			changed := !reflect.DeepEqual(st, last)
			last = st
			alive := map[*link.Link]bool{}
			for _, l := range a.links.Links() {
				alive[l] = true
				if (changed || !told[l]) && l.Supports("bluetooth") {
					l.Send("bluetooth", st)
				}
				told[l] = true
			}
			for l := range told {
				if !alive[l] {
					delete(told, l)
				}
			}
		}
	}()
}

// sendBluetooth sends this computer's Bluetooth now (to l, or everyone).
func (a *App) sendBluetooth(only *link.Link) {
	time.Sleep(1500 * time.Millisecond) // let the adapter settle
	st := localBluetooth()
	for _, l := range a.links.Links() {
		if (only == nil || l == only) && l.Supports("bluetooth") {
			l.Send("bluetooth", st)
		}
	}
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

// GetPeerBluetooth returns a device's Bluetooth as last reported.
func (a *App) GetPeerBluetooth(device string) BluetoothState {
	peerBluetooth.Lock()
	defer peerBluetooth.Unlock()
	st := peerBluetooth.m[device]
	if st.Devices == nil {
		st.Devices = []BluetoothDevice{}
	}
	return st
}

// SetPeerBluetooth asks a device to turn Bluetooth on or off. A phone may
// answer "asked" (its prompt is showing) or "settings" (opened there).
func (a *App) SetPeerBluetooth(device string, on bool) (string, error) {
	l, err := a.linkByName(device)
	if err != nil {
		return "", err
	}
	if !l.Supports("bluetooth.set") {
		return "", errors.New("update PALT on " + device + " for this")
	}
	p, err := l.Request("bluetooth.set", map[string]bool{"on": on}, 20*time.Second)
	if err != nil {
		return "", err
	}
	var r struct {
		Result string `json:"result"`
	}
	err = smsReply(p, &r)
	return r.Result, err
}
