package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Secure pipe between two paired devices, over the LAN or a relayed stream.
//
// Handshake (64 bytes each way):
//
//	initiator → epkI | HMAC(authKey, "palt-hs-i|" session epkI)
//	responder → epkR | HMAC(authKey, "palt-hs-r|" session epkI epkR)
//
// The MACs prove both ends hold the pairing key and bind the exchange to
// this session. Traffic keys come from an ephemeral X25519 exchange, so a
// leaked pairing key does not expose past transfers:
//
//	okm = HKDF-SHA256(X25519(...), salt=authKey, info="palt-relay-v1" epkI epkR, 64)
//	initiator→responder key = okm[:32], responder→initiator key = okm[32:]
//
// Frames: u32 big-endian ciphertext length, then AES-256-GCM ciphertext.
// Nonce is 4 zero bytes + u64 big-endian frame counter per direction.

const (
	maxFramePlain = 64 * 1024
	gcmTagSize    = 16
	hkdfInfo      = "palt-relay-v1"
)

var ErrHandshake = errors.New("secure handshake failed — are both devices paired with the same code?")

// Handshake runs the key exchange over raw and returns the encrypted conn.
func Handshake(raw net.Conn, keys Keys, session string, initiator bool) (*SecureConn, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	epk := priv.PublicKey().Bytes()
	sess := []byte(session)

	raw.SetDeadline(time.Now().Add(15 * time.Second))
	defer raw.SetDeadline(time.Time{})

	var epkI, epkR []byte
	peerMsg := make([]byte, 64)
	if initiator {
		epkI = epk
		out := append(append([]byte{}, epkI...), mac(keys.AuthKey, []byte("palt-hs-i|"), sess, epkI)...)
		if _, err := raw.Write(out); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(raw, peerMsg); err != nil {
			return nil, ErrHandshake
		}
		epkR = peerMsg[:32]
		if !hmac.Equal(peerMsg[32:], mac(keys.AuthKey, []byte("palt-hs-r|"), sess, epkI, epkR)) {
			return nil, ErrHandshake
		}
	} else {
		if _, err := io.ReadFull(raw, peerMsg); err != nil {
			return nil, ErrHandshake
		}
		epkI = peerMsg[:32]
		if !hmac.Equal(peerMsg[32:], mac(keys.AuthKey, []byte("palt-hs-i|"), sess, epkI)) {
			return nil, ErrHandshake
		}
		epkR = epk
		out := append(append([]byte{}, epkR...), mac(keys.AuthKey, []byte("palt-hs-r|"), sess, epkI, epkR)...)
		if _, err := raw.Write(out); err != nil {
			return nil, err
		}
	}

	peerPub := epkR
	if !initiator {
		peerPub = epkI
	}
	pub, err := ecdh.X25519().NewPublicKey(peerPub)
	if err != nil {
		return nil, ErrHandshake
	}
	shared, err := priv.ECDH(pub)
	if err != nil {
		return nil, ErrHandshake
	}
	info := append(append([]byte(hkdfInfo), epkI...), epkR...)
	okm, err := hkdf.Key(sha256.New, shared, keys.AuthKey, string(info), 64)
	if err != nil {
		return nil, err
	}

	i2r, err := newGCM(okm[:32])
	if err != nil {
		return nil, err
	}
	r2i, err := newGCM(okm[32:])
	if err != nil {
		return nil, err
	}
	c := &SecureConn{Conn: raw}
	if initiator {
		c.w, c.r = i2r, r2i
	} else {
		c.w, c.r = r2i, i2r
	}
	return c, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

// SecureConn is a net.Conn whose payload is framed and AES-GCM encrypted.
type SecureConn struct {
	net.Conn

	wmu    sync.Mutex
	w      cipher.AEAD
	wCount uint64
	wbuf   []byte

	rmu     sync.Mutex
	r       cipher.AEAD
	rCount  uint64
	pending []byte // decrypted bytes not yet returned by Read
	rbuf    []byte
}

func nonce(n uint64) []byte {
	b := make([]byte, 12)
	binary.BigEndian.PutUint64(b[4:], n)
	return b
}

func (c *SecureConn) Write(p []byte) (int, error) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	written := 0
	for len(p) > 0 {
		n := len(p)
		if n > maxFramePlain {
			n = maxFramePlain
		}
		need := 4 + n + gcmTagSize
		if cap(c.wbuf) < need {
			c.wbuf = make([]byte, 4+maxFramePlain+gcmTagSize)
		}
		frame := c.wbuf[:4]
		binary.BigEndian.PutUint32(frame, uint32(n+gcmTagSize))
		frame = c.w.Seal(frame, nonce(c.wCount), p[:n], nil)
		c.wCount++
		if _, err := c.Conn.Write(frame); err != nil {
			return written, err
		}
		written += n
		p = p[n:]
	}
	return written, nil
}

func (c *SecureConn) Read(p []byte) (int, error) {
	c.rmu.Lock()
	defer c.rmu.Unlock()
	if len(c.pending) == 0 {
		var hdr [4]byte
		if _, err := io.ReadFull(c.Conn, hdr[:]); err != nil {
			if err == io.ErrUnexpectedEOF {
				return 0, io.ErrUnexpectedEOF
			}
			return 0, err
		}
		size := binary.BigEndian.Uint32(hdr[:])
		if size < gcmTagSize || size > maxFramePlain+gcmTagSize {
			return 0, fmt.Errorf("secure: bad frame size %d", size)
		}
		if cap(c.rbuf) < int(size) {
			c.rbuf = make([]byte, maxFramePlain+gcmTagSize)
		}
		ct := c.rbuf[:size]
		if _, err := io.ReadFull(c.Conn, ct); err != nil {
			if err == io.EOF {
				return 0, io.ErrUnexpectedEOF
			}
			return 0, err
		}
		plain, err := c.r.Open(ct[:0], nonce(c.rCount), ct, nil)
		if err != nil {
			return 0, errors.New("secure: frame failed authentication")
		}
		c.rCount++
		c.pending = plain
	}
	n := copy(p, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}

// CloseWrite half-closes the underlying socket when it supports it.
func (c *SecureConn) CloseWrite() error {
	if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return c.Conn.Close()
}
