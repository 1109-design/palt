package relay

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"palt/internal/secure"
)

const (
	protocolVersion = 1
	pingInterval    = 20 * time.Second
	readTimeout     = 60 * time.Second
	dialTimeout     = 10 * time.Second
	maxLine         = 64 * 1024
)

// Peer is another of my devices currently connected to the relay.
type Peer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	OS   string `json:"os"`
}

// Status values reported through OnStatus.
const (
	StatusOff        = "off"
	StatusConnecting = "connecting"
	StatusOnline     = "online"
	StatusError      = "error"
)

type msg struct {
	T       string     `json:"t"`
	V       int        `json:"v,omitempty"`
	Group   string     `json:"group,omitempty"`
	ID      string     `json:"id,omitempty"`
	Name    string     `json:"name,omitempty"`
	OS      string     `json:"os,omitempty"`
	Sig     string     `json:"sig,omitempty"`
	To      string     `json:"to,omitempty"`
	From    string     `json:"from,omitempty"`
	Session string     `json:"session,omitempty"`
	Msg     string     `json:"msg,omitempty"`
	Peers   []wirePeer `json:"peers,omitempty"`
}

type wirePeer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	OS   string `json:"os"`
	Sig  string `json:"sig"`
}

// Client keeps this device registered with the relay and opens/accepts
// encrypted pipes to other devices of the same pairing group.
type Client struct {
	code     Code
	keys     secure.Keys
	deviceID string
	name     string
	os       string

	// OnPeers is called with the verified peer list whenever it changes.
	OnPeers func([]Peer)
	// OnStatus is called when the connection state changes.
	OnStatus func(status, detail string)
	// OnIncoming receives each accepted, already-encrypted pipe and the
	// device it comes from. The handler owns the conn.
	OnIncoming func(conn net.Conn, from Peer)

	mu     sync.Mutex
	peers  map[string]Peer
	stop   chan struct{}
	ctl    net.Conn
	status string
	detail string
}

func NewClient(code Code, deviceID, name, os string) *Client {
	return &Client{
		code:     code,
		keys:     code.Keys(),
		deviceID: deviceID,
		name:     name,
		os:       os,
		peers:    map[string]Peer{},
		stop:     make(chan struct{}),
		status:   StatusConnecting,
	}
}

// Start runs the control loop in the background, reconnecting with backoff.
func (c *Client) Start() { go c.loop() }

// Stop disconnects and ends the control loop.
func (c *Client) Stop() {
	c.mu.Lock()
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
	ctl := c.ctl
	c.mu.Unlock()
	if ctl != nil {
		ctl.Close()
	}
	c.setPeers(nil)
	c.setStatus(StatusOff, "")
}

func (c *Client) Status() (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status, c.detail
}

func (c *Client) Peers() []Peer {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Peer, 0, len(c.peers))
	for _, p := range c.peers {
		out = append(out, p)
	}
	return out
}

// PeerByName finds a relay peer by its broadcast name.
func (c *Client) PeerByName(name string) (Peer, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.peers {
		if p.Name == name {
			return p, true
		}
	}
	return Peer{}, false
}

func (c *Client) stopped() bool {
	select {
	case <-c.stop:
		return true
	default:
		return false
	}
}

func (c *Client) setStatus(s, detail string) {
	c.mu.Lock()
	changed := c.status != s || c.detail != detail
	c.status, c.detail = s, detail
	c.mu.Unlock()
	if changed && c.OnStatus != nil {
		c.OnStatus(s, detail)
	}
}

func (c *Client) setPeers(list []wirePeer) {
	next := map[string]Peer{}
	for _, p := range list {
		if p.ID == c.deviceID {
			continue
		}
		if !c.keys.VerifyPresence(p.ID, p.Name, p.OS, p.Sig) {
			log.Printf("[relay] Ignoring peer with bad signature: %q", p.Name)
			continue
		}
		next[p.ID] = Peer{ID: p.ID, Name: p.Name, OS: p.OS}
	}
	c.mu.Lock()
	c.peers = next
	c.mu.Unlock()
	if c.OnPeers != nil {
		c.OnPeers(c.Peers())
	}
}

func (c *Client) loop() {
	backoff := 2 * time.Second
	for !c.stopped() {
		c.setStatus(StatusConnecting, c.code.Addr())
		start := time.Now()
		err := c.session()
		c.setPeers(nil)
		if c.stopped() {
			return
		}
		if err != nil {
			log.Printf("[relay] Disconnected: %v", err)
			c.setStatus(StatusError, err.Error())
		}
		if time.Since(start) > time.Minute {
			backoff = 2 * time.Second
		}
		select {
		case <-c.stop:
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// session runs one control connection until it fails.
func (c *Client) session() error {
	conn, err := net.DialTimeout("tcp", c.code.Addr(), dialTimeout)
	if err != nil {
		return fmt.Errorf("cannot reach relay %s", c.code.Addr())
	}
	defer conn.Close()
	c.mu.Lock()
	c.ctl = conn
	c.mu.Unlock()
	if c.stopped() {
		return nil
	}

	var wmu sync.Mutex
	write := func(m msg) error {
		b, _ := json.Marshal(m)
		wmu.Lock()
		defer wmu.Unlock()
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err := conn.Write(append(b, '\n'))
		return err
	}

	err = write(msg{
		T: "register", V: protocolVersion, Group: c.keys.GroupID,
		ID: c.deviceID, Name: c.name, OS: c.os,
		Sig: c.keys.PresenceSig(c.deviceID, c.name, c.os),
	})
	if err != nil {
		return err
	}

	br := bufio.NewReaderSize(conn, maxLine)
	conn.SetReadDeadline(time.Now().Add(readTimeout))
	first, err := readMsg(br)
	if err != nil {
		return fmt.Errorf("relay did not answer: %w", err)
	}
	if first.T != "ok" {
		return fmt.Errorf("relay refused: %s", first.Msg)
	}
	c.setStatus(StatusOnline, c.code.Addr())
	log.Printf("[relay] Online at %s", c.code.Addr())

	done := make(chan struct{})
	defer close(done)
	go func() {
		t := time.NewTicker(pingInterval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if write(msg{T: "ping"}) != nil {
					conn.Close()
					return
				}
			}
		}
	}()

	for {
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		m, err := readMsg(br)
		if err != nil {
			if c.stopped() {
				return nil
			}
			return errors.New("lost connection to relay")
		}
		switch m.T {
		case "peers":
			c.setPeers(m.Peers)
		case "incoming":
			go c.acceptSession(m.Session, m.From)
		}
	}
}

func (c *Client) acceptSession(session, from string) {
	c.mu.Lock()
	peer, known := c.peers[from]
	c.mu.Unlock()
	if !known {
		log.Printf("[relay] Incoming session from unknown device %s; ignoring", from)
		return
	}
	raw, err := c.openPipe(msg{T: "accept", V: protocolVersion, Group: c.keys.GroupID, ID: c.deviceID, Session: session})
	if err != nil {
		log.Printf("[relay] Accept failed: %v", err)
		return
	}
	sc, err := secure.Handshake(raw, c.keys, session, false)
	if err != nil {
		log.Printf("[relay] Incoming handshake from %s failed: %v", peer.Name, err)
		raw.Close()
		return
	}
	if c.OnIncoming != nil {
		c.OnIncoming(sc, peer)
	} else {
		sc.Close()
	}
}

// Dial opens an encrypted pipe to a peer through the relay.
func (c *Client) Dial(peerID string) (net.Conn, error) {
	session := randomHex(16)
	raw, err := c.openPipe(msg{T: "connect", V: protocolVersion, Group: c.keys.GroupID, ID: c.deviceID, To: peerID, Session: session})
	if err != nil {
		return nil, err
	}
	sc, err := secure.Handshake(raw, c.keys, session, true)
	if err != nil {
		raw.Close()
		return nil, err
	}
	return sc, nil
}

// openPipe sends a connect/accept request and waits for the relay's "ok".
func (c *Client) openPipe(req msg) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", c.code.Addr(), dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("cannot reach relay %s", c.code.Addr())
	}
	b, _ := json.Marshal(req)
	conn.SetDeadline(time.Now().Add(25 * time.Second))
	if _, err := conn.Write(append(b, '\n')); err != nil {
		conn.Close()
		return nil, err
	}
	// Read the reply byte by byte: pipe data may follow it immediately.
	reply, err := readMsgUnbuffered(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("relay did not answer: %w", err)
	}
	if reply.T != "ok" {
		conn.Close()
		return nil, fmt.Errorf("relay: %s", reply.Msg)
	}
	conn.SetDeadline(time.Time{})
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(30 * time.Second)
	}
	return conn, nil
}

func readMsg(br *bufio.Reader) (msg, error) {
	var m msg
	line, err := br.ReadSlice('\n')
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(line, &m)
	return m, err
}

func readMsgUnbuffered(r io.Reader) (msg, error) {
	var m msg
	buf := make([]byte, 0, 128)
	one := make([]byte, 1)
	for {
		if _, err := io.ReadFull(r, one); err != nil {
			return m, err
		}
		if one[0] == '\n' {
			break
		}
		buf = append(buf, one[0])
		if len(buf) > 4096 {
			return m, errors.New("reply too long")
		}
	}
	err := json.Unmarshal(buf, &m)
	return m, err
}
