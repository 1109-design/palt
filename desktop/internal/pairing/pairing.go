// Package pairing is the one secret all my devices share, and the code used
// to hand it to a new device.
//
// Code format (also shown as a QR code):
//
//	palt2:<base64url 32-byte key>               LAN only
//	palt2:<base64url 32-byte key>@<host:port>   plus a relay for remote use
//
// palt1 relay codes ("palt1:<host>:<port>:<key>") are still accepted: their
// key becomes the pairing key, so devices paired through the relay stay paired.
//
// Must stay identical to mobile/lib/services/secure/pairing.dart.
package pairing

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"strconv"
	"strings"

	"palt/internal/relay"
	"palt/internal/secure"
)

const codePrefix = "palt2:"

// Pairing is this device's identity and the group it belongs to.
type Pairing struct {
	DeviceID string `json:"deviceId"`
	Key      []byte `json:"key"`             // 32 bytes shared by all paired devices
	Relay    string `json:"relay,omitempty"` // "host:port", empty = LAN only
}

// New returns a fresh identity in a group of its own.
func New() Pairing {
	return Pairing{DeviceID: secure.RandomHex(8), Key: NewKey()}
}

func NewKey() []byte {
	k := make([]byte, 32)
	rand.Read(k)
	return k
}

func (p Pairing) Keys() secure.Keys { return secure.DeriveKeys(p.Key) }

// Code is what another device needs to join this group.
func (p Pairing) Code() string {
	s := codePrefix + base64.RawURLEncoding.EncodeToString(p.Key)
	if p.Relay != "" {
		s += "@" + p.Relay
	}
	return s
}

// RelayCode is the relay client's view of the group, if a relay is set.
func (p Pairing) RelayCode() (relay.Code, bool) {
	if p.Relay == "" {
		return relay.Code{}, false
	}
	host, portStr, err := net.SplitHostPort(p.Relay)
	if err != nil {
		return relay.Code{}, false
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return relay.Code{}, false
	}
	return relay.Code{Host: host, Port: port, Key: p.Key}, true
}

// ParseCode reads a palt2 or palt1 code into a key and relay address.
func ParseCode(s string) (key []byte, relayAddr string, err error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "palt1:") {
		c, err := relay.ParseCode(s)
		if err != nil {
			return nil, "", err
		}
		return c.Key, c.Addr(), nil
	}
	if !strings.HasPrefix(s, codePrefix) {
		return nil, "", errors.New("pairing code must start with palt2:")
	}
	rest := s[len(codePrefix):]
	keyStr := rest
	if i := strings.Index(rest, "@"); i >= 0 {
		keyStr = rest[:i]
		relayAddr, err = NormalizeRelay(rest[i+1:])
		if err != nil {
			return nil, "", err
		}
	}
	key, err = base64.RawURLEncoding.DecodeString(keyStr)
	if err != nil || len(key) != 32 {
		return nil, "", errors.New("pairing code has a bad key")
	}
	return key, relayAddr, nil
}

// NormalizeRelay turns "host" or "host:port" into "host:port"; empty stays empty.
func NormalizeRelay(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", nil
	}
	c, err := relay.NewCode(addr)
	if err != nil {
		return "", err
	}
	return c.Addr(), nil
}
