// Package link keeps one long-lived, encrypted channel open to each paired
// device. Live features (battery, notifications, clipboard, media, ...) talk
// over it as small JSON packets; bulk data still goes over separate
// transfer connections.
//
// A link is opened like any v2 transfer with action "link". After the accept
// byte both sides exchange packets:
//
//	u32 big-endian length | JSON {"t": type, "id": optional, "body": {...}}
//
// The first packet each way is "identity". Either side may dial; when two
// links to the same device end up open, both sides keep the same one: a
// direct link beats a relay link, otherwise the one dialed by the lower
// device ID wins.
//
// Must stay identical to mobile/lib/services/link/.
package link

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// conflictProbe is how long an old link has to answer when a new one to the
// same device would lose to it.
const conflictProbe = 2 * time.Second

const (
	maxPacket    = 1 << 20
	pingInterval = 20 * time.Second
	deadAfter    = 60 * time.Second
	identityWait = 15 * time.Second
)

// Packet types handled by the link itself.
const (
	TypeIdentity = "identity"
	TypePing     = "ping"
	TypePong     = "pong"
)

// Routes, matching transfer.RouteDirect / RouteRelay.
const (
	RouteDirect = "direct"
	RouteRelay  = "relay"
)

// Packet is one message on a link.
type Packet struct {
	T    string          `json:"t"`
	ID   string          `json:"id,omitempty"`
	Body json.RawMessage `json:"body,omitempty"`
}

// Decode unmarshals the packet body into v.
func (p Packet) Decode(v any) error {
	if len(p.Body) == 0 {
		return errors.New("empty body")
	}
	return json.Unmarshal(p.Body, v)
}

// Identity describes a device and what it can do.
type Identity struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OS      string `json:"os"`
	Version string `json:"version"`
	// Handles lists the packet types this device understands.
	Handles []string `json:"handles"`
	// Prio is the dialer's preference for this link over others to the
	// same device (0 = none); a faster path is dialed with a higher one.
	Prio int `json:"prio,omitempty"`
}

// Link is an open channel to one paired device.
type Link struct {
	Peer  Identity
	Route string

	conn      net.Conn
	initiator string // device ID of the side that dialed
	prio      int    // the dialer's Identity.Prio
	since     time.Time
	lastRecv  atomic.Int64 // unix nanos of the last packet read

	wmu    sync.Mutex
	closed chan struct{}
	once   sync.Once
}

// Prio is the dialer's preference for this link (see Identity.Prio).
func (l *Link) Prio() int { return l.prio }

// Probe pings the peer and reports whether anything came back within
// timeout. Used after a network change to drop a dead link in seconds
// rather than waiting for the read deadline.
func (l *Link) Probe(timeout time.Duration) bool {
	start := time.Now().UnixNano()
	if l.Send(TypePing, nil) != nil {
		return false
	}
	deadline := time.After(timeout)
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-l.closed:
			return false
		case <-deadline:
			return false
		case <-t.C:
			if l.lastRecv.Load() >= start {
				return true
			}
		}
	}
}

// Since is when the link came up.
func (l *Link) Since() time.Time { return l.since }

// RemoteIP is the peer's address for direct links, "" for relay links.
func (l *Link) RemoteIP() string {
	if l.Route != RouteDirect {
		return ""
	}
	if a, ok := l.conn.RemoteAddr().(*net.TCPAddr); ok {
		return a.IP.String()
	}
	host, _, _ := net.SplitHostPort(l.conn.RemoteAddr().String())
	return host
}

// Supports reports whether the peer handles packet type t.
func (l *Link) Supports(t string) bool {
	return slices.Contains(l.Peer.Handles, t)
}

// Send writes one packet. Safe for concurrent use.
func (l *Link) Send(t string, body any) error {
	return l.SendID(t, "", body)
}

// SendID writes one packet carrying a request/response id.
func (l *Link) SendID(t, id string, body any) error {
	p := Packet{T: t, ID: id}
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		p.Body = b
	}
	return l.write(p)
}

func (l *Link) write(p Packet) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if len(b) > maxPacket {
		return fmt.Errorf("packet %s too large (%d bytes)", p.T, len(b))
	}
	frame := binary.BigEndian.AppendUint32(make([]byte, 0, 4+len(b)), uint32(len(b)))
	frame = append(frame, b...)
	l.wmu.Lock()
	defer l.wmu.Unlock()
	select {
	case <-l.closed:
		return net.ErrClosed
	default:
	}
	l.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	_, err = l.conn.Write(frame)
	if err != nil {
		l.Close()
	}
	return err
}

// Close ends the link.
func (l *Link) Close() {
	l.once.Do(func() {
		close(l.closed)
		l.conn.Close()
	})
}

// Done is closed when the link ends.
func (l *Link) Done() <-chan struct{} { return l.closed }

func readPacket(r io.Reader) (Packet, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Packet{}, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 || n > maxPacket {
		return Packet{}, fmt.Errorf("bad packet size %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return Packet{}, err
	}
	var p Packet
	if err := json.Unmarshal(buf, &p); err != nil {
		return Packet{}, err
	}
	return p, nil
}

// ─── Manager ─────────────────────────────────────────────────────────────────

// Handler receives packets of one type.
type Handler func(l *Link, p Packet)

// Manager owns the links of this device.
type Manager struct {
	self func() Identity

	// OnChange is called when a link comes up or goes down.
	OnChange func()
	// OnUp is called once a link is established (after OnChange).
	OnUp func(l *Link)

	mu       sync.Mutex
	links    map[string]*Link
	handlers map[string]Handler
}

// NewManager makes a manager; self is called for each identity packet so
// name changes are picked up.
func NewManager(self func() Identity) *Manager {
	return &Manager{
		self:     self,
		links:    map[string]*Link{},
		handlers: map[string]Handler{},
	}
}

// Handle registers the handler for packet type t. Register handlers before
// links come up.
func (m *Manager) Handle(t string, h Handler) {
	m.mu.Lock()
	m.handlers[t] = h
	m.mu.Unlock()
}

// HandledTypes lists the packet types this device understands.
func (m *Manager) HandledTypes() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{TypePing, TypePong}
	for t := range m.handlers {
		out = append(out, t)
	}
	return out
}

// Get returns the link to a device, or nil.
func (m *Manager) Get(id string) *Link {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.links[id]
}

// GetByName returns the link to a device by its name, or nil.
func (m *Manager) GetByName(name string) *Link {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, l := range m.links {
		if l.Peer.Name == name {
			return l
		}
	}
	return nil
}

// Links returns the open links.
func (m *Manager) Links() []*Link {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Link, 0, len(m.links))
	for _, l := range m.links {
		out = append(out, l)
	}
	return out
}

// CloseAll ends every link.
func (m *Manager) CloseAll() {
	for _, l := range m.Links() {
		l.Close()
	}
}

// Run takes over conn (an accepted link connection), exchanges identities
// and serves packets until the link ends. initiator is true on the side
// that dialed. It returns once the link is registered or has failed; the
// link then runs in the background.
func (m *Manager) Run(conn net.Conn, route string, initiator bool) (*Link, error) {
	return m.RunPrio(conn, route, initiator, 0)
}

// RunPrio is Run for a dialer that prefers this link over others to the
// same device by prio (see Identity.Prio).
func (m *Manager) RunPrio(conn net.Conn, route string, initiator bool, prio int) (*Link, error) {
	self := m.self()
	self.Handles = m.HandledTypes()
	if initiator {
		self.Prio = prio
	}
	l := &Link{conn: conn, Route: route, closed: make(chan struct{}), since: time.Now()}
	if err := l.Send(TypeIdentity, self); err != nil {
		conn.Close()
		return nil, err
	}
	conn.SetReadDeadline(time.Now().Add(identityWait))
	first, err := readPacket(conn)
	if err != nil || first.T != TypeIdentity {
		conn.Close()
		return nil, fmt.Errorf("no identity from peer: %v", err)
	}
	if err := first.Decode(&l.Peer); err != nil || l.Peer.ID == "" || l.Peer.ID == self.ID {
		conn.Close()
		return nil, errors.New("bad identity from peer")
	}
	if initiator {
		l.initiator, l.prio = self.ID, prio
	} else {
		l.initiator, l.prio = l.Peer.ID, l.Peer.Prio
	}

	if !m.register(l, self.ID) {
		l.Close()
		return nil, errors.New("already linked")
	}
	log.Printf("[link] Up: %s (%s, %s)", l.Peer.Name, l.Peer.ID, route)
	if m.OnChange != nil {
		m.OnChange()
	}
	if m.OnUp != nil {
		m.OnUp(l)
	}
	go m.serve(l)
	return l, nil
}

// register adds l, resolving a duplicate link to the same device. It
// returns false when l lost and must be closed.
func (m *Manager) register(l *Link, selfID string) bool {
	m.mu.Lock()
	old := m.links[l.Peer.ID]
	if old != nil {
		select {
		case <-old.closed:
			old = nil
		default:
		}
	}
	if old != nil && !better(l, old, selfID) {
		m.mu.Unlock()
		// The peer only dials again when it lost the old link (its network
		// changed), so check that the old one still works before refusing.
		if old.Probe(conflictProbe) {
			return false
		}
		log.Printf("[link] %s: old link stopped answering, taking the new one", l.Peer.Name)
		old.Close()
		m.mu.Lock()
		if cur := m.links[l.Peer.ID]; cur != nil && cur != old {
			m.mu.Unlock()
			return false
		}
	}
	m.links[l.Peer.ID] = l
	m.mu.Unlock()
	if old != nil {
		old.Close()
	}
	return true
}

// better reports whether link a should replace link b. Both sides of a pair
// reach the same answer.
func better(a, b *Link, selfID string) bool {
	if a.Route != b.Route {
		return a.Route == RouteDirect
	}
	if a.prio != b.prio {
		return a.prio > b.prio
	}
	low := min(selfID, a.Peer.ID)
	if (a.initiator == low) != (b.initiator == low) {
		return a.initiator == low
	}
	return false // same kind: keep the one we have
}

func (m *Manager) serve(l *Link) {
	defer func() {
		l.Close()
		m.mu.Lock()
		current := m.links[l.Peer.ID] == l
		if current {
			delete(m.links, l.Peer.ID)
		}
		m.mu.Unlock()
		if current {
			log.Printf("[link] Down: %s", l.Peer.Name)
			if m.OnChange != nil {
				m.OnChange()
			}
		}
	}()

	go func() {
		t := time.NewTicker(pingInterval)
		defer t.Stop()
		for {
			select {
			case <-l.closed:
				return
			case <-t.C:
				if l.Send(TypePing, nil) != nil {
					return
				}
			}
		}
	}()

	for {
		l.conn.SetReadDeadline(time.Now().Add(deadAfter))
		p, err := readPacket(l.conn)
		if err != nil {
			return
		}
		l.lastRecv.Store(time.Now().UnixNano())
		switch p.T {
		case TypePing:
			l.Send(TypePong, nil)
		case TypePong, TypeIdentity:
		default:
			m.mu.Lock()
			h := m.handlers[p.T]
			m.mu.Unlock()
			// Handlers run in order on the read loop; anything slow must
			// hand off to its own goroutine.
			if h != nil {
				h(l, p)
			}
		}
	}
}

// Broadcast sends a packet to every linked device that handles its type.
func (m *Manager) Broadcast(t string, body any) {
	for _, l := range m.Links() {
		if l.Supports(t) {
			l.Send(t, body)
		}
	}
}
