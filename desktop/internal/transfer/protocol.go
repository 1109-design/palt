package transfer

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"palt/internal/secure"
)

// Port defines the default TCP port for the PALT file transfer protocol.
// This perfectly aligns with the advertised mDNS port.
const Port = 9876

// MaxOfferSize sets an upper limit on the JSON handshake size (64KB) to prevent overflow attacks.
// It has to fit a full text message plus JSON escaping overhead.
const MaxOfferSize = 64 * 1024

// MaxTextBytes caps the UTF-8 size of a single text message.
const MaxTextBytes = 16 * 1024

// Action values for the handshake
const (
	ActionOffer = "offer" // file batch; payload bytes follow an accept
	ActionText  = "text"  // text message carried inside the handshake itself
	ActionLink  = "link"  // long-lived packet channel; paired devices only
)

// Response Bytes
const (
	ResponseReject byte = 0x00
	ResponseAccept byte = 0x01
)

type FileMeta struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Metadata models the JSON handshake sent right after connection.
type Metadata struct {
	Action     string     `json:"action"` // "offer" implies batch
	TransferID string     `json:"transferId"`
	Files      []FileMeta `json:"files"`
	TotalSize  int64      `json:"totalSize"`
	SenderName string     `json:"senderName"`
	Text       string     `json:"text,omitempty"` // only set for ActionText

	// Route is how this connection reached us ("direct" or "relay"). It is
	// set locally by the receiver, never trusted from the wire.
	Route string `json:"route,omitempty"`

	// Paired is true when the sender proved it holds our pairing key, and
	// SenderID is its device ID. Both are set locally by the receiver.
	Paired   bool   `json:"paired,omitempty"`
	SenderID string `json:"senderId,omitempty"`
}

// ─── v2: paired connections ──────────────────────────────────────────────────
//
// A v1 connection starts with the offer's u32 length, whose first byte is
// always 0 (offers are ≤ 64 KB). A v2 connection starts with the magic
// "PALT" instead, then:
//
//	u8 version (2) | u16 big-endian length | hello JSON {"from","nonce"}
//
// followed by the secure handshake (session = "palt-lan|" + hello JSON), and
// then the v1 exchange (offer/text/link) inside the encrypted pipe.

const (
	helloMagic   = "PALT"
	helloVersion = 2
	maxHello     = 4096
)

// Hello opens a v2 connection.
type Hello struct {
	From  string `json:"from"`  // sender's device ID
	Nonce string `json:"nonce"` // fresh per connection
}

// ErrNotPaired means a v2 connection arrived but this device has no pairing key.
var ErrNotPaired = errors.New("not paired")

func helloSession(raw []byte) string { return "palt-lan|" + string(raw) }

// SecureDialer wraps a raw dialer: it sends the v2 hello and runs the secure
// handshake, so what it returns is an encrypted pipe to a paired device.
func SecureDialer(raw Dialer, selfID string, keys secure.Keys) Dialer {
	return func() (net.Conn, error) {
		conn, err := raw()
		if err != nil {
			return nil, err
		}
		sc, err := secureClient(conn, selfID, keys)
		if err != nil {
			conn.Close()
			return nil, err
		}
		return sc, nil
	}
}

func secureClient(conn net.Conn, selfID string, keys secure.Keys) (net.Conn, error) {
	hello, _ := json.Marshal(Hello{From: selfID, Nonce: secure.RandomHex(16)})
	frame := make([]byte, 0, 7+len(hello))
	frame = append(frame, helloMagic...)
	frame = append(frame, helloVersion)
	frame = binary.BigEndian.AppendUint16(frame, uint16(len(hello)))
	frame = append(frame, hello...)
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(frame); err != nil {
		return nil, err
	}
	conn.SetWriteDeadline(time.Time{})
	return secure.Handshake(conn, keys, helloSession(hello), true)
}

// readHello reads the rest of a v2 hello after the magic.
func readHello(r io.Reader) (Hello, []byte, error) {
	var hdr [3]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Hello{}, nil, err
	}
	if hdr[0] != helloVersion {
		return Hello{}, nil, fmt.Errorf("unsupported protocol version %d", hdr[0])
	}
	n := binary.BigEndian.Uint16(hdr[1:])
	if n == 0 || n > maxHello {
		return Hello{}, nil, errors.New("bad hello size")
	}
	raw := make([]byte, n)
	if _, err := io.ReadFull(r, raw); err != nil {
		return Hello{}, nil, err
	}
	var h Hello
	if err := json.Unmarshal(raw, &h); err != nil || h.From == "" {
		return Hello{}, nil, errors.New("bad hello")
	}
	return h, raw, nil
}

// Routes a transfer can take.
const (
	RouteDirect = "direct"
	RouteRelay  = "relay"
)

// Dialer opens a connection to the peer a transfer is going to.
type Dialer func() (net.Conn, error)

// DirectDialer dials a peer on the local network.
func DirectDialer(ip string, port int, timeout time.Duration) Dialer {
	return func() (net.Conn, error) {
		return net.DialTimeout("tcp", net.JoinHostPort(ip, strconv.Itoa(port)), timeout)
	}
}

// ReadOffer parses the exact 4-byte length header and subsequent JSON payload.
func ReadOffer(r io.Reader) (*Metadata, error) {
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lengthBuf); err != nil {
		return nil, err
	}
	return readOfferBody(r, lengthBuf)
}

// readOfferBody reads an offer whose 4-byte length prefix was already read.
func readOfferBody(r io.Reader, lengthBuf []byte) (*Metadata, error) {
	length := uint32(lengthBuf[3]) | uint32(lengthBuf[2])<<8 | uint32(lengthBuf[1])<<16 | uint32(lengthBuf[0])<<24

	if length > MaxOfferSize {
		return nil, io.ErrShortBuffer // Protective limit
	}

	// 2. Read exact JSON payload string
	jsonBuf := make([]byte, length)
	if _, err := io.ReadFull(r, jsonBuf); err != nil {
		return nil, err
	}

	// 3. Unmarshal
	var meta Metadata
	if err := json.Unmarshal(jsonBuf, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// WriteOffer serialized the Metadata with the 4-byte prefix frame to the network output stream.
func WriteOffer(w io.Writer, meta *Metadata) error {
	jsonBytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	length := uint32(len(jsonBytes))

	// Write 4-byte length prefix (big-endian)
	if err := writeLengthPrefix(w, length); err != nil {
		return err
	}

	// Write JSON payload
	if _, err := w.Write(jsonBytes); err != nil {
		return err
	}

	return nil
}

func writeLengthPrefix(w io.Writer, length uint32) error {
	prefix := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}
	_, err := w.Write(prefix)
	return err
}

// TrackingWriter is a custom io.Writer decorator that triggers callbacks on byte progression.
type TrackingWriter struct {
	Out           io.Writer
	Total         int64
	Written       int64
	OnProgress    func(written int64, total int64)
	BroadcastStep int64 // Throttle emit frequency
	lastBroadcast int64
}

func (tw *TrackingWriter) Write(p []byte) (n int, err error) {
	n, err = tw.Out.Write(p)
	tw.Written += int64(n)

	if tw.OnProgress != nil && tw.Written-tw.lastBroadcast >= tw.BroadcastStep || tw.Written == tw.Total {
		tw.OnProgress(tw.Written, tw.Total)
		tw.lastBroadcast = tw.Written
	}

	return n, err
}

// TrackingReader is the read-side decorator equivalent of TrackingWriter
type TrackingReader struct {
	In            io.Reader
	Total         int64
	ReadAmt       int64
	OnProgress    func(read int64, total int64)
	BroadcastStep int64
	lastBroadcast int64
}

func (tr *TrackingReader) Read(p []byte) (n int, err error) {
	n, err = tr.In.Read(p)
	tr.ReadAmt += int64(n)

	if tr.OnProgress != nil && tr.ReadAmt-tr.lastBroadcast >= tr.BroadcastStep || tr.ReadAmt == tr.Total {
		tr.OnProgress(tr.ReadAmt, tr.Total)
		tr.lastBroadcast = tr.ReadAmt
	}

	return n, err
}
