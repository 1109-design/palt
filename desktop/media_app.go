// media_app.go is the media remote between paired devices: each side tells
// the other what is playing, and either can control the other's players and
// volume.
//
// Packet types (must match mobile/lib/features/media.dart):
//
//	media.players  either way  MediaState: players, output volume, sample time
//	media.action   either way  {player, action, value}
//	                           player actions: playpause, play, pause, next,
//	                           previous, seek (value ms), volume (value 0-1);
//	                           player "": volume (value 0-100), mute (toggle)
package main

import (
	"errors"
	"log"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/link"
)

var (
	errUnknownPlayer = errors.New("that player isn't running")
	errUnknownAction = errors.New("unknown media action")
)

// MediaPlayer is one app that plays media.
type MediaPlayer struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Title      string  `json:"title"`
	Artist     string  `json:"artist"`
	Album      string  `json:"album"`
	Art        string  `json:"art"` // web URL or data: URL, "" none
	TrackID    string  `json:"-"`
	Playing    bool    `json:"playing"`
	CanNext    bool    `json:"canNext"`
	CanPrev    bool    `json:"canPrev"`
	CanSeek    bool    `json:"canSeek"`
	CanControl bool    `json:"canControl"`
	Position   int64   `json:"position"` // ms, at MediaState.At
	Length     int64   `json:"length"`   // ms, 0 unknown
	Volume     float64 `json:"volume"`   // 0-1, -1 not settable
}

// SystemVolume is a device's media output volume.
type SystemVolume struct {
	Level int  `json:"level"` // 0-100, -1 unknown
	Muted bool `json:"muted"`
}

// MediaState is everything a device reports about its media.
type MediaState struct {
	Players []MediaPlayer `json:"players"`
	Volume  SystemVolume  `json:"volume"`
	At      int64         `json:"at"` // unix ms the positions were read
}

type mediaState struct {
	mu    sync.Mutex
	peers map[string]MediaState // by device name
	kick  chan struct{}
}

func (a *App) setupMedia() {
	a.media.peers = map[string]MediaState{}
	a.media.kick = make(chan struct{}, 1)

	a.links.Handle("media.players", func(l *link.Link, p link.Packet) {
		var st MediaState
		if p.Decode(&st) != nil {
			return
		}
		a.media.mu.Lock()
		a.media.peers[l.Peer.Name] = st
		a.media.mu.Unlock()
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "peer_media", a.GetPeerMedia())
		}
	})
	a.links.Handle("media.action", func(l *link.Link, p link.Packet) {
		var b struct {
			Player string  `json:"player"`
			Action string  `json:"action"`
			Value  float64 `json:"value"`
		}
		if p.Decode(&b) != nil {
			return
		}
		go func() {
			var err error
			if b.Player == "" {
				err = setSystemVolume(b.Action, int(b.Value))
			} else {
				err = playerAction(b.Player, b.Action, b.Value)
			}
			if err != nil {
				log.Printf("[media] %s from %s: %v", b.Action, l.Peer.Name, err)
			}
			a.mediaChanged()
		}()
	})

	watchPlayers(a.mediaChanged)
	watchVolume(a.mediaChanged)
	go a.sendMediaLoop()
}

// mediaChanged asks for the media state to be sent again.
func (a *App) mediaChanged() {
	select {
	case a.media.kick <- struct{}{}:
	default:
	}
}

// sendMediaLoop sends this computer's media state to linked devices that
// show it: when each link comes up and after changes (batched).
func (a *App) sendMediaLoop() {
	told := map[*link.Link]bool{}
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		changed := false
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
		case <-a.media.kick:
			changed = true
			// Players fire several signals per change; let them settle.
			time.Sleep(300 * time.Millisecond)
			select {
			case <-a.media.kick:
			default:
			}
		}
		var targets []*link.Link
		alive := map[*link.Link]bool{}
		for _, l := range a.links.Links() {
			alive[l] = true
			if l.Supports("media.players") && (changed || !told[l]) {
				targets = append(targets, l)
			}
		}
		for l := range told {
			if !alive[l] {
				delete(told, l)
			}
		}
		if len(targets) == 0 {
			continue
		}
		st := MediaState{Players: listPlayers(), Volume: systemVolume(), At: time.Now().UnixMilli()}
		for _, l := range targets {
			l.Send("media.players", st)
			told[l] = true
		}
	}
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

// GetPeerMedia returns what each linked device reported playing.
func (a *App) GetPeerMedia() map[string]MediaState {
	out := map[string]MediaState{}
	a.media.mu.Lock()
	defer a.media.mu.Unlock()
	for name, st := range a.media.peers {
		if a.links != nil && a.links.GetByName(name) != nil {
			out[name] = st
		}
	}
	return out
}

// PeerMediaAction controls a player (or, with player "", the volume) on a
// paired device.
func (a *App) PeerMediaAction(name, player, action string, value float64) error {
	l, err := a.linkByName(name)
	if err != nil {
		return err
	}
	return l.Send("media.action", map[string]any{"player": player, "action": action, "value": value})
}
