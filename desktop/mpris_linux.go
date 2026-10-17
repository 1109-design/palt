// mpris_linux.go pauses and resumes media players (browsers, VLC, Spotify,
// ...) through the MPRIS D-Bus interface every Linux player speaks.
package main

import (
	"strings"

	"github.com/godbus/dbus/v5"
)

const mprisPrefix = "org.mpris.MediaPlayer2."

// pausePlaying pauses every player that is playing and returns their bus
// names, so resumePlayers can start exactly those again.
func pausePlaying() []string {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil
	}
	var names []string
	if conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names) != nil {
		return nil
	}
	var paused []string
	for _, n := range names {
		if !strings.HasPrefix(n, mprisPrefix) {
			continue
		}
		obj := conn.Object(n, "/org/mpris/MediaPlayer2")
		v, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.PlaybackStatus")
		if err != nil || v.Value() != "Playing" {
			continue
		}
		if obj.Call("org.mpris.MediaPlayer2.Player.Pause", 0).Err == nil {
			paused = append(paused, n)
		}
	}
	return paused
}

func resumePlayers(names []string) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return
	}
	for _, n := range names {
		conn.Object(n, "/org/mpris/MediaPlayer2").Call("org.mpris.MediaPlayer2.Player.Play", 0)
	}
}
