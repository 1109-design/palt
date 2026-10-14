package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"regexp"
	"sync"
	"time"
)

const (
	protocolVersion = 1

	maxLineBytes      = 4096
	maxDevicesInGroup = 32
	maxPipesPerGroup  = 16

	helloTimeout   = 10 * time.Second // first line must arrive within this
	controlTimeout = 60 * time.Second // control conn dropped after this much silence
	acceptTimeout  = 15 * time.Second // target must answer an incoming notice within this
	pipeIdle       = 10 * time.Minute // a pipe with no traffic either way is closed
)

var (
	groupRe   = regexp.MustCompile(`^[0-9a-f]{32}$`)
	idRe      = regexp.MustCompile(`^[0-9a-f]{16,64}$`)
	sessionRe = regexp.MustCompile(`^[0-9a-f]{16,64}$`)
)

// Msg is every JSON line exchanged with clients.
type Msg struct {
	T       string `json:"t"`
	V       int    `json:"v,omitempty"`
	Group   string `json:"group,omitempty"`
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	OS      string `json:"os,omitempty"`
	Sig     string `json:"sig,omitempty"`
	To      string `json:"to,omitempty"`
	From    string `json:"from,omitempty"`
	Session string `json:"session,omitempty"`
	Msg     string `json:"msg,omitempty"`
	Peers   []Peer `json:"peers,omitempty"`
}

// Peer is a device as other members of its group see it. Sig is an HMAC the
// apps compute and verify with their shared key; the relay just carries it.
type Peer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	OS   string `json:"os"`
	Sig  string `json:"sig"`
}

type device struct {
	peer Peer
	conn net.Conn
	wmu  sync.Mutex
}

func (d *device) send(m Msg) error {
	b, _ := json.Marshal(m)
	d.wmu.Lock()
	defer d.wmu.Unlock()
	d.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := d.conn.Write(append(b, '\n'))
	return err
}

type pending struct {
	group string
	to    string
	ch    chan net.Conn
}

// Relay holds the in-memory registry. There is no persistent state.
type Relay struct {
	mu       sync.Mutex
	groups   map[string]map[string]*device // group -> device id -> device
	sessions map[string]*pending
	pipes    map[string]int // group -> active pipe count
}

func NewRelay() *Relay {
	return &Relay{
		groups:   make(map[string]map[string]*device),
		sessions: make(map[string]*pending),
		pipes:    make(map[string]int),
	}
}

func (r *Relay) Serve(l net.Listener) error {
	for {
		c, err := l.Accept()
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return err
		}
		go r.handle(c)
	}
}

func (r *Relay) handle(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(30 * time.Second)
	}
	c.SetReadDeadline(time.Now().Add(helloTimeout))
	m, err := readLine(c)
	if err != nil {
		c.Close()
		return
	}
	if m.V != protocolVersion {
		reply(c, Msg{T: "err", Msg: "unsupported protocol version"})
		c.Close()
		return
	}
	if !groupRe.MatchString(m.Group) {
		reply(c, Msg{T: "err", Msg: "bad group"})
		c.Close()
		return
	}
	switch m.T {
	case "register":
		r.handleRegister(c, m)
	case "connect":
		r.handleConnect(c, m)
	case "accept":
		r.handleAccept(c, m) // takes ownership of c
	default:
		reply(c, Msg{T: "err", Msg: "unknown request"})
		c.Close()
	}
}

func (r *Relay) handleRegister(c net.Conn, m Msg) {
	defer c.Close()
	if !idRe.MatchString(m.ID) || len(m.Name) == 0 || len(m.Name) > 128 || len(m.OS) > 32 || len(m.Sig) > 128 {
		reply(c, Msg{T: "err", Msg: "bad register"})
		return
	}
	d := &device{peer: Peer{ID: m.ID, Name: m.Name, OS: m.OS, Sig: m.Sig}, conn: c}

	r.mu.Lock()
	g := r.groups[m.Group]
	if g == nil {
		g = make(map[string]*device)
		r.groups[m.Group] = g
	}
	old := g[m.ID]
	if old == nil && len(g) >= maxDevicesInGroup {
		r.mu.Unlock()
		reply(c, Msg{T: "err", Msg: "group is full"})
		return
	}
	g[m.ID] = d
	r.mu.Unlock()
	if old != nil {
		old.conn.Close() // same device reconnected; drop the stale socket
	}

	log.Printf("[%s] + %s (%s)", short(m.Group), m.Name, short(m.ID))
	d.send(Msg{T: "ok"})
	r.broadcastPeers(m.Group)

	br := bufio.NewReaderSize(c, maxLineBytes)
	for {
		c.SetReadDeadline(time.Now().Add(controlTimeout))
		line, err := readBufferedLine(br)
		if err != nil {
			break
		}
		var in Msg
		if json.Unmarshal(line, &in) != nil {
			break
		}
		if in.T == "ping" {
			if d.send(Msg{T: "pong"}) != nil {
				break
			}
		}
	}

	r.mu.Lock()
	removed := false
	if g := r.groups[m.Group]; g != nil && g[m.ID] == d {
		delete(g, m.ID)
		removed = true
		if len(g) == 0 {
			delete(r.groups, m.Group)
		}
	}
	r.mu.Unlock()
	if removed {
		log.Printf("[%s] - %s (%s)", short(m.Group), m.Name, short(m.ID))
		r.broadcastPeers(m.Group)
	}
}

func (r *Relay) broadcastPeers(group string) {
	r.mu.Lock()
	g := r.groups[group]
	devs := make([]*device, 0, len(g))
	peers := make([]Peer, 0, len(g))
	for _, d := range g {
		devs = append(devs, d)
		peers = append(peers, d.peer)
	}
	r.mu.Unlock()
	for _, d := range devs {
		go d.send(Msg{T: "peers", Peers: peers})
	}
}

func (r *Relay) handleConnect(c net.Conn, m Msg) {
	if !idRe.MatchString(m.To) || !sessionRe.MatchString(m.Session) {
		reply(c, Msg{T: "err", Msg: "bad connect"})
		c.Close()
		return
	}

	r.mu.Lock()
	target := r.groups[m.Group][m.To]
	_, dup := r.sessions[m.Session]
	busy := r.pipes[m.Group] >= maxPipesPerGroup
	var p *pending
	if target != nil && !dup && !busy {
		p = &pending{group: m.Group, to: m.To, ch: make(chan net.Conn, 1)}
		r.sessions[m.Session] = p
		r.pipes[m.Group]++
	}
	r.mu.Unlock()

	switch {
	case target == nil:
		reply(c, Msg{T: "err", Msg: "device is not connected to the relay"})
		c.Close()
		return
	case dup:
		reply(c, Msg{T: "err", Msg: "session already exists"})
		c.Close()
		return
	case busy:
		reply(c, Msg{T: "err", Msg: "too many transfers in progress"})
		c.Close()
		return
	}
	defer func() {
		r.mu.Lock()
		delete(r.sessions, m.Session)
		r.pipes[m.Group]--
		if r.pipes[m.Group] <= 0 {
			delete(r.pipes, m.Group)
		}
		r.mu.Unlock()
		// An accept that raced the timeout is left in the channel; close it.
		select {
		case late := <-p.ch:
			late.Close()
		default:
		}
	}()

	if err := target.send(Msg{T: "incoming", Session: m.Session, From: m.ID}); err != nil {
		reply(c, Msg{T: "err", Msg: "device is not reachable"})
		c.Close()
		return
	}

	var other net.Conn
	select {
	case other = <-p.ch:
	case <-time.After(acceptTimeout):
		reply(c, Msg{T: "err", Msg: "device did not answer"})
		c.Close()
		return
	}

	// Remove the session before piping so a late duplicate accept is refused.
	r.mu.Lock()
	delete(r.sessions, m.Session)
	r.mu.Unlock()

	c.SetDeadline(time.Time{})
	other.SetDeadline(time.Time{})
	if reply(c, Msg{T: "ok"}) != nil || reply(other, Msg{T: "ok"}) != nil {
		c.Close()
		other.Close()
		return
	}
	log.Printf("[%s] pipe %s open", short(m.Group), short(m.Session))
	pipe(c, other)
	log.Printf("[%s] pipe %s closed", short(m.Group), short(m.Session))
}

func (r *Relay) handleAccept(c net.Conn, m Msg) {
	r.mu.Lock()
	p := r.sessions[m.Session]
	if p != nil && p.group == m.Group && p.to == m.ID {
		delete(r.sessions, m.Session)
	} else {
		p = nil
	}
	r.mu.Unlock()

	if p == nil {
		reply(c, Msg{T: "err", Msg: "unknown session"})
		c.Close()
		return
	}
	p.ch <- c // the connect side now owns c
}

// pipe copies bytes both ways until both directions finish, passing
// half-closes through so request/response exchanges complete cleanly.
func pipe(a, b net.Conn) {
	defer a.Close()
	defer b.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	cp := func(dst, src net.Conn) {
		defer wg.Done()
		_, err := io.Copy(idleWriter{dst}, idleReader{src})
		if err != nil {
			// A broken side ends the session for both.
			a.Close()
			b.Close()
			return
		}
		if tc, ok := dst.(*net.TCPConn); ok {
			tc.CloseWrite()
		} else {
			dst.Close()
		}
	}
	go cp(a, b)
	go cp(b, a)
	wg.Wait()
}

type idleReader struct{ net.Conn }

func (r idleReader) Read(p []byte) (int, error) {
	r.Conn.SetReadDeadline(time.Now().Add(pipeIdle))
	return r.Conn.Read(p)
}

type idleWriter struct{ net.Conn }

func (w idleWriter) Write(p []byte) (int, error) {
	w.Conn.SetWriteDeadline(time.Now().Add(pipeIdle))
	return w.Conn.Write(p)
}

func reply(c net.Conn, m Msg) error {
	b, _ := json.Marshal(m)
	c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := c.Write(append(b, '\n'))
	c.SetWriteDeadline(time.Time{})
	return err
}

// readLine reads one JSON line byte by byte, so no bytes that follow it
// (raw pipe data) are swallowed by a buffer.
func readLine(c net.Conn) (Msg, error) {
	var m Msg
	buf := make([]byte, 0, 256)
	one := make([]byte, 1)
	for {
		if _, err := io.ReadFull(c, one); err != nil {
			return m, err
		}
		if one[0] == '\n' {
			break
		}
		buf = append(buf, one[0])
		if len(buf) > maxLineBytes {
			return m, errors.New("line too long")
		}
	}
	err := json.Unmarshal(buf, &m)
	return m, err
}

func readBufferedLine(br *bufio.Reader) ([]byte, error) {
	line, err := br.ReadSlice('\n')
	if err == bufio.ErrBufferFull {
		return nil, errors.New("line too long")
	}
	return line, err
}

func short(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
