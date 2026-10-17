// Package secure holds what makes two of my devices trust each other: the
// shared pairing key, the keys derived from it, and the encrypted pipe every
// connection between paired devices runs through (LAN and relay alike).
//
// The key derivation and handshake must stay byte-for-byte identical to
// mobile/lib/services/secure/.
package secure

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// Keys are what a pairing key expands into.
type Keys struct {
	GroupID string // sent to the relay; lets it match devices, reveals nothing else
	AuthKey []byte // never leaves the device
}

// DeriveKeys expands the 32-byte pairing key shared by all my devices.
func DeriveKeys(pairingKey []byte) Keys {
	gid := mac(pairingKey, []byte("palt-group-id"))
	return Keys{
		GroupID: hex.EncodeToString(gid)[:32],
		AuthKey: mac(pairingKey, []byte("palt-auth")),
	}
}

// PresenceSig authenticates a device's presence entry to the other devices,
// so the relay can't inject or rename devices.
func (k Keys) PresenceSig(id, name, os string) string {
	return hex.EncodeToString(mac(k.AuthKey, []byte("palt-presence|"+id+"|"+name+"|"+os)))
}

func (k Keys) VerifyPresence(id, name, os, sig string) bool {
	want := k.PresenceSig(id, name, os)
	return hmac.Equal([]byte(want), []byte(sig))
}

func mac(key []byte, parts ...[]byte) []byte {
	h := hmac.New(sha256.New, key)
	for _, p := range parts {
		h.Write(p)
	}
	return h.Sum(nil)
}

// RandomHex returns n random bytes as hex.
func RandomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
