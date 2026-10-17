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
	"sort"
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
	// probeLinkTimeout is how long a link has to answer after a network change.
	probeLinkTimeout = 3 * time.Second
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
	// LanIP is the last address reached other than over a USB cable, tried
	// when the device says it is back on Wi-Fi but discovery hasn't seen it.
	LanIP    string    `json:"lanIp,omitempty"`
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
		if pathKind(ip) != "usb" {
			d.LanIP = ip
		}
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
	lan   []lanAddr // fastest first
	seen  bool      // a LAN address is from a live sighting, not memory
	relay bool
}

// lanAddr is one LAN address of a device and how fast this computer
// reaches it (pathSpeed, also the link's priority when dialed).
type lanAddr struct {
	ip    string
	port  int
	speed int
}

func (c *linkCandidate) addLAN(ip string, port int) {
	for _, a := range c.lan {
		if a.ip == ip {
			return
		}
	}
	c.lan = append(c.lan, lanAddr{ip, port, pathSpeed(ip)})
	sort.SliceStable(c.lan, func(i, j int) bool { return c.lan[i].speed > c.lan[j].speed })
}

// switchTo returns the LAN addresses worth moving a direct link l to: those
// clearly faster than its current path, which the peer will also prefer.
func (c *linkCandidate) switchTo(l *link.Link) []lanAddr {
	cur := pathSpeed(l.RemoteIP())
	var out []lanAddr
	for _, a := range c.lan {
		if a.ip != l.RemoteIP() && a.speed > l.Prio() && a.speed*2 > cur*3 {
			out = append(out, a)
		}
	}
	return out
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
	probed := map[string]time.Time{} // when a link was last checked for a stale path

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
					c.addLAN(p.IPAddress, p.Port)
					c.seen = true
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
				if c := get(id); len(c.lan) == 0 {
					c.addLAN(d.LastIP, paltPort)
				}
			}
			// Back on Wi-Fi but not seen there yet: try where it was before.
			if d.LanIP != "" && id != self && a.statusOf(d.Name).Network == "wifi" {
				get(id).addLAN(d.LanIP, paltPort)
			}
		}

		for id, c := range cands {
			l := a.links.Get(id)
			switch {
			case l == nil:
			case l.Route == link.RouteDirect:
				// The device showing up at another address can mean its old
				// path died (its Wi-Fi went off) without this side noticing.
				if c.seen && len(c.lan) > 1 && time.Since(probed[id]) > 10*time.Second {
					probed[id] = time.Now()
					go func(l *link.Link) {
						if !l.Probe(probeLinkTimeout) {
							log.Printf("[link] %s stopped answering; reconnecting", l.Peer.Name)
							l.Close()
							a.kickConnector()
						}
					}(l)
				}
				// Move to a clearly faster path (a USB cable or Wi-Fi).
				if c.lan = c.switchTo(l); len(c.lan) == 0 {
					continue
				}
			case len(c.lan) == 0:
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

// connect opens a link to a device, fastest LAN address first. haveLink
// means a link is already up and only the LAN addresses are worth trying.
func (a *App) connect(id string, c linkCandidate, haveLink bool) error {
	p := a.pairing()
	var lanErr error
	for _, addr := range c.lan {
		dial := transfer.SecureDialer(transfer.DirectDialer(addr.ip, addr.port, directDialTimeout), p.DeviceID, p.Keys())
		if lanErr = a.openLink(dial, transfer.RouteDirect, addr.speed); lanErr == nil {
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
	return a.openLink(func() (net.Conn, error) { return rc.Dial(id) }, transfer.RouteRelay, 0)
}

// openLink dials, asks for a link and hands it to the link manager.
// prio is the link's priority over others to the same device.
func (a *App) openLink(dial transfer.Dialer, route string, prio int) error {
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
	_, err = a.links.RunPrio(conn, route, true, prio)
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
				Path:       l.Route,
			}
			if ip := l.RemoteIP(); ip != "" {
				p.IPAddress, p.Port = ip, a.lanPort(ip)
				p.Path = pathKind(ip)
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
			p.Path = pathKind(p.IPAddress)
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
				Path:       transfer.RouteRelay,
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
	if err := a.openLink(dial, transfer.RouteDirect, pathSpeed(ip)); err != nil {
		if errors.Is(err, secure.ErrHandshake) {
			return errors.New("that device isn't paired with this one — scan the code on it first")
		}
		return fmt.Errorf("could not connect: %v", err)
	}
	return nil
}

// pathLabel names a peer's Path for people: how PALT reaches it.
func pathLabel(path string) string {
	switch path {
	case "usb":
		return "USB cable"
	case "wifi":
		return "same Wi-Fi"
	case "lan":
		return "same network"
	case transfer.RouteRelay:
		return "via relay"
	}
	return ""
}

// onNetworkChanged drops links that no longer answer, then looks for
// devices again, so PALT moves to the new path (Wi-Fi back, a USB cable)
// within seconds instead of waiting for the dead link to time out.
func (a *App) onNetworkChanged() {
	if a.discovery != nil {
		a.discovery.Rescan()
	}
	go func() {
		a.probeLinks()
		a.kickConnector()
	}()
}

// probeLinks closes direct links that don't answer a ping in a few seconds.
func (a *App) probeLinks() {
	var wg sync.WaitGroup
	for _, l := range a.links.Links() {
		if l.Route != link.RouteDirect {
			continue
		}
		wg.Add(1)
		go func(l *link.Link) {
			defer wg.Done()
			if !l.Probe(probeLinkTimeout) {
				log.Printf("[link] %s stopped answering; reconnecting", l.Peer.Name)
				l.Close()
			}
		}(l)
	}
	wg.Wait()
}
