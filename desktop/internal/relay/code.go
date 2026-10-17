// Package relay connects PALT to a palt-relay server so devices can reach
// each other when they are not on the same network.
//
// All devices of one owner share a pairing code. The relay only learns a
// group id derived from it; every relayed stream is end-to-end encrypted with
// keys only the devices can derive (see securepipe.go).
//
// The wire format here must stay byte-for-byte identical to
// mobile/lib/services/relay/.
package relay

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"palt/internal/secure"
)

const (
	codePrefix  = "palt1:"
	DefaultPort = 9877
)

// Code is a parsed pairing code: "palt1:<host>:<port>:<base64url key>".
type Code struct {
	Host string
	Port int
	Key  []byte // 32 random bytes shared by all paired devices
}

func ParseCode(s string) (Code, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, codePrefix) {
		return Code{}, errors.New("pairing code must start with palt1:")
	}
	rest := s[len(codePrefix):]
	i := strings.LastIndex(rest, ":")
	if i < 0 {
		return Code{}, errors.New("pairing code is incomplete")
	}
	hostPort, keyStr := rest[:i], rest[i+1:]
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil || host == "" {
		return Code{}, errors.New("pairing code has a bad relay address")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return Code{}, errors.New("pairing code has a bad relay port")
	}
	key, err := base64.RawURLEncoding.DecodeString(keyStr)
	if err != nil || len(key) != 32 {
		return Code{}, errors.New("pairing code has a bad key")
	}
	return Code{Host: host, Port: port, Key: key}, nil
}

func (c Code) String() string {
	return codePrefix + net.JoinHostPort(c.Host, strconv.Itoa(c.Port)) + ":" + base64.RawURLEncoding.EncodeToString(c.Key)
}

func (c Code) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// NewCode makes a fresh pairing code for a relay at addr ("host" or "host:port").
func NewCode(addr string) (Code, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return Code{}, errors.New("enter the relay server address")
	}
	host, port := addr, DefaultPort
	if h, p, err := net.SplitHostPort(addr); err == nil {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return Code{}, fmt.Errorf("bad port %q", p)
		}
		host, port = h, n
	}
	if host == "" || strings.ContainsAny(host, " /") {
		return Code{}, fmt.Errorf("bad relay address %q", addr)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return Code{}, err
	}
	return Code{Host: host, Port: port, Key: key}, nil
}

func (c Code) Keys() secure.Keys { return secure.DeriveKeys(c.Key) }

func randomHex(n int) string { return secure.RandomHex(n) }
