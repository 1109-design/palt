// mpris_linux.go controls media players (browsers, VLC, Spotify, ...)
// through the MPRIS D-Bus interface every Linux player speaks: pausing
// them for calls, and the media remote on paired phones.
package main

import (
	"encoding/base64"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"

	"palt/internal/thumbs"
)

const (
	mprisPrefix = "org.mpris.MediaPlayer2."
	mprisPath   = "/org/mpris/MediaPlayer2"
	mprisPlayer = "org.mpris.MediaPlayer2.Player"
)

// pausePlaying pauses every player that is playing and returns their bus
// names, so resumePlayers can start exactly those again.
func pausePlaying() []string {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil
	}
	var paused []string
	for _, n := range mprisNames(conn) {
		obj := conn.Object(n, mprisPath)
		v, err := obj.GetProperty(mprisPlayer + ".PlaybackStatus")
		if err != nil || v.Value() != "Playing" {
			continue
		}
		if obj.Call(mprisPlayer+".Pause", 0).Err == nil {
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
		conn.Object(n, mprisPath).Call(mprisPlayer+".Play", 0)
	}
}

func mprisNames(conn *dbus.Conn) []string {
	var names, out []string
	if conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names) != nil {
		return nil
	}
	for _, n := range names {
		if strings.HasPrefix(n, mprisPrefix) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// listPlayers describes every running player.
func listPlayers() []MediaPlayer {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil
	}
	out := []MediaPlayer{}
	for _, n := range mprisNames(conn) {
		obj := conn.Object(n, mprisPath)
		var props map[string]dbus.Variant
		if obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, mprisPlayer).Store(&props) != nil {
			continue
		}
		p := MediaPlayer{ID: n, Name: strings.TrimPrefix(n, mprisPrefix), Volume: -1}
		if v, err := obj.GetProperty("org.mpris.MediaPlayer2.Identity"); err == nil {
			if s, ok := v.Value().(string); ok && s != "" {
				p.Name = s
			}
		}
		str := func(k string) string { s, _ := props[k].Value().(string); return s }
		flag := func(k string) bool { b, _ := props[k].Value().(bool); return b }
		p.Playing = str("PlaybackStatus") == "Playing"
		p.CanNext, p.CanPrev = flag("CanGoNext"), flag("CanGoPrevious")
		p.CanSeek, p.CanControl = flag("CanSeek"), flag("CanControl")
		if v, ok := props["Position"].Value().(int64); ok {
			p.Position = v / 1000
		}
		if v, ok := props["Volume"].Value().(float64); ok {
			p.Volume = v
		}
		meta, _ := props["Metadata"].Value().(map[string]dbus.Variant)
		p.Title, _ = meta["xesam:title"].Value().(string)
		p.Album, _ = meta["xesam:album"].Value().(string)
		if artists, ok := meta["xesam:artist"].Value().([]string); ok {
			p.Artist = strings.Join(artists, ", ")
		}
		switch l := meta["mpris:length"].Value().(type) {
		case int64:
			p.Length = l / 1000
		case uint64:
			p.Length = int64(l / 1000)
		}
		p.TrackID = trackID(meta)
		if art, _ := meta["mpris:artUrl"].Value().(string); art != "" {
			p.Art = artData(art)
		}
		// Full-size art from the PALT browser extension (browsers pass on
		// only a ~150px thumbnail): album "Netflix ⟨art:URL⟩".
		if m := albumArtRe.FindStringSubmatch(p.Album); m != nil {
			p.Album = strings.TrimSpace(p.Album[:len(p.Album)-len(m[0])])
			p.Art = m[1]
		}
		out = append(out, p)
	}
	return out
}

func trackID(meta map[string]dbus.Variant) string {
	switch v := meta["mpris:trackid"].Value().(type) {
	case dbus.ObjectPath:
		return string(v)
	case string:
		return v
	}
	return ""
}

var albumArtRe = regexp.MustCompile(`\s*⟨art:(https?://[^⟩\s]+)⟩$`)

var artCache struct {
	sync.Mutex
	url, data string
}

// artData turns a cover art URL into something a phone can show: web URLs
// as they are, local files as a small JPEG data URL.
func artData(art string) string {
	if strings.HasPrefix(art, "http://") || strings.HasPrefix(art, "https://") {
		return art
	}
	c := &artCache
	c.Lock()
	defer c.Unlock()
	if c.url == art {
		return c.data
	}
	c.url, c.data = art, ""
	u, err := url.Parse(art)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	src, err := os.ReadFile(u.Path)
	if err != nil {
		return ""
	}
	jpg, err := thumbs.Make(src)
	if err != nil {
		return ""
	}
	c.data = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpg)
	return c.data
}

// playerAction runs one remote control action on a player.
func playerAction(id, action string, value float64) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(id, mprisPrefix) {
		return errUnknownPlayer
	}
	obj := conn.Object(id, mprisPath)
	switch action {
	case "playpause":
		return obj.Call(mprisPlayer+".PlayPause", 0).Err
	case "play":
		return obj.Call(mprisPlayer+".Play", 0).Err
	case "pause":
		return obj.Call(mprisPlayer+".Pause", 0).Err
	case "next":
		return obj.Call(mprisPlayer+".Next", 0).Err
	case "previous":
		return obj.Call(mprisPlayer+".Previous", 0).Err
	case "seek": // value: position in ms
		var props map[string]dbus.Variant
		if obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, mprisPlayer).Store(&props) != nil {
			return errUnknownPlayer
		}
		meta, _ := props["Metadata"].Value().(map[string]dbus.Variant)
		track := dbus.ObjectPath(trackID(meta))
		if !track.IsValid() {
			// No track id: move relative to where it is now.
			pos, _ := props["Position"].Value().(int64)
			return obj.Call(mprisPlayer+".Seek", 0, int64(value*1000)-pos).Err
		}
		return obj.Call(mprisPlayer+".SetPosition", 0, track, int64(value*1000)).Err
	case "volume": // value: 0-1
		return obj.SetProperty(mprisPlayer+".Volume", dbus.MakeVariant(max(0, min(value, 1))))
	}
	return errUnknownAction
}

// watchPlayers calls onChange whenever a player starts, stops or changes.
func watchPlayers(onChange func()) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return
	}
	conn.AddMatchSignal(dbus.WithMatchObjectPath(mprisPath), dbus.WithMatchInterface("org.freedesktop.DBus.Properties"))
	conn.AddMatchSignal(dbus.WithMatchObjectPath(mprisPath), dbus.WithMatchInterface(mprisPlayer), dbus.WithMatchMember("Seeked"))
	conn.AddMatchSignal(dbus.WithMatchSender("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"))
	ch := make(chan *dbus.Signal, 32)
	conn.Signal(ch)
	go func() {
		for s := range ch {
			if s.Name == "org.freedesktop.DBus.NameOwnerChanged" {
				if len(s.Body) == 0 {
					continue
				}
				if name, _ := s.Body[0].(string); !strings.HasPrefix(name, mprisPrefix) {
					continue
				}
			}
			onChange()
		}
	}()
}
