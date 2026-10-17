// links_app.go keeps a link open to every paired device that can be reached,
// builds the device list the UI shows, and picks the route for each send.
//
// The connector tries, for each paired device it knows about: the LAN
// address mDNS or the beacon reported, the last LAN address that worked, and
// the relay. A relay link is upgraded as soon as the LAN works again.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"palt/internal/link"
	"palt/internal/models"
	"palt/internal/secure"
	"palt/internal/transfer"
)

const (
	connectInterval = 5 * time.Second
	// Backoff caps for a device that doesn't answer: short while it is being
	// seen on the LAN right now, long for a remembered address.
	backoffSeen  = 30 * time.Second
	backoffKnown = 5 * time.Minute
	// A device answering with another pairing key is not in our group.
	backoffOtherGroup = 10 * time.Minute
)

// knownDevice is a paired device seen before, kept so it can be found again
// even when discovery fails.
type knownDevice struct {
	Name     string    `json:"name"`
	OS       string    `json:"os"`
	LastIP   string    `json:"lastIp,omitempty"`
	LastSeen time.Time `json:"lastSeen"`
}

func knownDevicesFile() string {
	dir, err := configDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "devices.json")
}

func (a *App) loadKnownDevices() map[string]knownDevice {
	out := map[string]knownDevice{}
	if f := knownDevicesFile(); f != "" {
		if data, err := os.ReadFile(f); err == nil {
			_ = json.Unmarshal(data, &out)
		}
	}
	return out
}

func (a *App) rememberDevice(l *link.Link) {
	a.knownMu.Lock()
	defer a.knownMu.Unlock()
	devices := a.loadKnownDevices()
	d := devices[l.Peer.ID]
	d.Name, d.OS, d.LastSeen = l.Peer.Name, l.Peer.OS, time.Now()
	if ip := l.RemoteIP(); ip != "" {
		d.LastIP = ip
	}
	devices[l.Peer.ID] = d
	if f := knownDevicesFile(); f != "" {
		if data, err := json.MarshalIndent(devices, "", "  "); err == nil {
			_ = os.WriteFile(f, data, 0o644)
		}
	}
}

// setupLinks creates the link manager and hooks it into the transfer server.
// Call before the transfer server starts.
func (a *App) setupLinks() {
	a.links = link.NewManager(func() link.Identity {
		host, _ := os.Hostname()
		return link.Identity{ID: a.pairing().DeviceID, Name: host, OS: runtime.GOOS, Version: AppVersion}
	})
	a.links.OnChange = a.emitPeers
	a.links.OnUp = a.rememberDevice

	a.transferServer.Keys = func() (secure.Keys, bool) { return a.pairing().Keys(), true }
	a.transferServer.OnLink = func(conn net.Conn, meta transfer.Metadata) {
		if _, err := a.links.Run(conn, meta.Route, false); err != nil {
			log.Printf("[link] Incoming link from %s refused: %v", meta.SenderName, err)
		}
	}
}

func (a *App) kickConnector() {
	select {
	case a.connectKick <- struct{}{}:
	default:
	}
}

type linkCandidate struct {
	lanIP   string
	lanPort int
	seen    bool // the LAN address is from a live sighting, not memory
	relay   bool
}

// runConnector opens links to paired devices until ctx is done.
func (a *App) runConnector() {
	type state struct {
		next     time.Time
		failures int
		busy     bool
	}
	var mu sync.Mutex
	states := map[string]*state{}

	t := time.NewTicker(connectInterval)
	defer t.Stop()
	for {
		self := a.pairing().DeviceID
		cands := map[string]*linkCandidate{}
		get := func(id string) *linkCandidate {
			c := cands[id]
			if c == nil {
				c = &linkCandidate{}
				cands[id] = c
			}
			return c
		}
		if a.discovery != nil {
			for _, p := range a.discovery.GetPeers() {
				if p.DeviceID != "" && p.DeviceID != self {
					c := get(p.DeviceID)
					c.lanIP, c.lanPort, c.seen = p.IPAddress, p.Port, true
				}
			}
		}
		if rc := a.relayClient(); rc != nil {
			for _, p := range rc.Peers() {
				get(p.ID).relay = true
			}
		}
		for id, d := range a.loadKnownDevices() {
			if d.LastIP != "" && id != self {
				if c := get(id); c.lanIP == "" {
					c.lanIP, c.lanPort = d.LastIP, paltPort
				}
			}
		}

		for id, c := range cands {
			l := a.links.Get(id)
			if l != nil && (l.Route == link.RouteDirect || c.lanIP == "") {
				continue
			}
			mu.Lock()
			st := states[id]
			if st == nil {
				st = &state{}
				states[id] = st
			}
			if st.busy || time.Now().Before(st.next) {
				mu.Unlock()
				continue
			}
			st.busy = true
			mu.Unlock()

			go func(id string, c linkCandidate, haveLink bool) {
				err := a.connect(id, c, haveLink)
				mu.Lock()
				defer mu.Unlock()
				st.busy = false
				if err == nil {
					st.failures, st.next = 0, time.Time{}
					return
				}
				st.failures++
				limit := backoffKnown
				if c.seen {
					limit = backoffSeen
				}
				wait := min(connectInterval<<min(st.failures, 8), limit)
				if errors.Is(err, secure.ErrHandshake) {
					wait = backoffOtherGroup
				}
				st.next = time.Now().Add(wait)
			}(id, *c, l != nil)
		}

		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
		case <-a.connectKick:
			// A kick means something changed; retry right away.
			mu.Lock()
			for _, st := range states {
				if st.failures < 3 {
					st.next = time.Time{}
				}
			}
			mu.Unlock()
		}
	}
}

// connect opens a link to a device, LAN first. haveLink means a relay link
// is already up and only the LAN is worth trying.
func (a *App) connect(id string, c linkCandidate, haveLink bool) error {
	p := a.pairing()
	var lanErr error
	if c.lanIP != "" {
		dial := transfer.SecureDialer(transfer.DirectDialer(c.lanIP, c.lanPort, directDialTimeout), p.DeviceID, p.Keys())
		if lanErr = a.openLink(dial, transfer.RouteDirect); lanErr == nil {
			return nil
		}
		if errors.Is(lanErr, secure.ErrHandshake) {
			return lanErr
		}
	}
	if haveLink || !c.relay {
		if lanErr == nil {
			lanErr = errors.New("no route")
		}
		return lanErr
	}
	rc := a.relayClient()
	if rc == nil {
		return errors.New("relay is off")
	}
	return a.openLink(func() (net.Conn, error) { return rc.Dial(id) }, transfer.RouteRelay)
}

// openLink dials, asks for a link and hands it to the link manager.
func (a *App) openLink(dial transfer.Dialer, route string) error {
	conn, err := dial()
	if err != nil {
		return err
	}
	host, _ := os.Hostname()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	err = transfer.WriteOffer(conn, &transfer.Metadata{Action: transfer.ActionLink, TransferID: generateTransferID(), SenderName: host})
	if err == nil {
		verdict := make([]byte, 1)
		if _, err = io.ReadFull(conn, verdict); err == nil && verdict[0] != transfer.ResponseAccept {
			err = errors.New("link refused")
		}
	}
	conn.SetDeadline(time.Time{})
	if err != nil {
		conn.Close()
		return err
	}
	_, err = a.links.Run(conn, route, true)
	return err
}

// ─── Device list and routes ──────────────────────────────────────────────────

// lanPort is the port a LAN address advertised, or the default.
func (a *App) lanPort(ip string) int {
	if a.discovery != nil {
		for _, p := range a.discovery.GetPeers() {
			if p.IPAddress == ip {
				return p.Port
			}
		}
	}
	return paltPort
}

// mergedPeers lists linked (paired) devices first, then other LAN devices,
// then relay devices without a link yet.
func (a *App) mergedPeers() []models.Peer {
	out := []models.Peer{}
	seenID := map[string]bool{}
	seenName := map[string]bool{}
	if a.links != nil {
		for _, l := range a.links.Links() {
			p := models.Peer{
				ID:         "dev-" + l.Peer.ID,
				DeviceName: l.Peer.Name,
				OS:         l.Peer.OS,
				Via:        l.Route,
				DeviceID:   l.Peer.ID,
				Paired:     true,
				IPAddress:  relayPeerIP,
			}
			if ip := l.RemoteIP(); ip != "" {
				p.IPAddress, p.Port = ip, a.lanPort(ip)
			}
			seenID[l.Peer.ID], seenName[l.Peer.Name] = true, true
			out = append(out, p)
		}
	}
	if a.discovery != nil {
		for _, p := range a.discovery.GetPeers() {
			if seenID[p.DeviceID] || (p.DeviceID == "" && seenName[p.DeviceName]) {
				continue
			}
			p.Via = transfer.RouteDirect
			seenName[p.DeviceName] = true
			if p.DeviceID != "" {
				seenID[p.DeviceID] = true
			}
			out = append(out, p)
		}
	}
	if rc := a.relayClient(); rc != nil {
		for _, p := range rc.Peers() {
			if seenID[p.ID] || seenName[p.Name] {
				continue
			}
			out = append(out, models.Peer{
				ID:         "relay-" + p.ID,
				DeviceName: p.Name,
				IPAddress:  relayPeerIP,
				OS:         p.OS,
				Via:        transfer.RouteRelay,
				DeviceID:   p.ID,
			})
		}
	}
	return out
}

// dialPeer connects to a peer for a transfer and returns the route used.
// Paired devices get an encrypted connection, over the LAN when possible.
// Other devices get the plain v1 protocol on the LAN.
func (a *App) dialPeer(peerIP string, peerPort int, peerName string) (net.Conn, string, error) {
	p := a.pairing()
	rc := a.relayClient()

	if l := a.links.GetByName(peerName); l != nil {
		var lanErr error
		if ip := l.RemoteIP(); ip != "" {
			raw := transfer.DirectDialer(ip, a.lanPort(ip), directDialTimeout)
			conn, err := transfer.SecureDialer(raw, p.DeviceID, p.Keys())()
			if err == nil {
				return conn, transfer.RouteDirect, nil
			}
			lanErr = err
		}
		if rc != nil {
			if conn, err := rc.Dial(l.Peer.ID); err == nil {
				return conn, transfer.RouteRelay, nil
			} else if lanErr == nil {
				lanErr = err
			}
		}
		return nil, "", fmt.Errorf("could not reach %s: %v", peerName, lanErr)
	}

	var directErr error
	if peerIP != relayPeerIP {
		conn, err := transfer.DirectDialer(peerIP, peerPort, directDialTimeout)()
		if err == nil {
			return conn, transfer.RouteDirect, nil
		}
		directErr = err
	}
	if rc != nil {
		if rp, ok := rc.PeerByName(peerName); ok {
			log.Printf("[app] Sending to %s via relay", peerName)
			conn, err := rc.Dial(rp.ID)
			if err != nil {
				return nil, "", err
			}
			return conn, transfer.RouteRelay, nil
		}
	}
	if directErr != nil {
		return nil, "", directErr
	}
	return nil, "", fmt.Errorf("%s is not reachable", peerName)
}

// ConnectByIP links to a paired device at a LAN address, for networks where
// discovery doesn't get through. The address is remembered once it works.
func (a *App) ConnectByIP(addr string) error {
	ip, port := strings.TrimSpace(addr), paltPort
	if h, p, err := net.SplitHostPort(ip); err == nil {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("bad port %q", p)
		}
		ip, port = h, n
	}
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("%q is not an IP address", addr)
	}
	pr := a.pairing()
	dial := transfer.SecureDialer(transfer.DirectDialer(ip, port, directDialTimeout), pr.DeviceID, pr.Keys())
	if err := a.openLink(dial, transfer.RouteDirect); err != nil {
		if errors.Is(err, secure.ErrHandshake) {
			return errors.New("that device isn't paired with this one — scan the code on it first")
		}
		return fmt.Errorf("could not connect: %v", err)
	}
	return nil
}
