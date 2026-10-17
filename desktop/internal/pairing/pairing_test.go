package pairing

import (
	"bytes"
	"testing"

	"palt/internal/relay"
)

func TestCodeRoundTrip(t *testing.T) {
	p := New()
	key, addr, err := ParseCode(p.Code())
	if err != nil || !bytes.Equal(key, p.Key) || addr != "" {
		t.Fatalf("LAN code: %v %q", err, addr)
	}
	p.Relay = "relay.example.com:9877"
	key, addr, err = ParseCode(p.Code())
	if err != nil || !bytes.Equal(key, p.Key) || addr != p.Relay {
		t.Fatalf("relay code: %v %q", err, addr)
	}
	if rc, ok := p.RelayCode(); !ok || rc.Host != "relay.example.com" || rc.Port != 9877 {
		t.Fatalf("relay code view: %+v", rc)
	}
	for _, bad := range []string{"", "palt2:", "palt2:abc", "palt3:x", "palt2:" + p.Code()[6:] + "@bad host"} {
		if _, _, err := ParseCode(bad); err == nil {
			t.Errorf("ParseCode(%q) should fail", bad)
		}
	}
}

// Fixed vector shared with mobile/test/relay_test.dart.
func TestCodeVector(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	if c := (Pairing{Key: key}).Code(); c != "palt2:AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8" {
		t.Errorf("code = %s", c)
	}
}

func TestPalt1CodeKeepsGroup(t *testing.T) {
	c, _ := relay.NewCode("10.0.0.1:9000")
	key, addr, err := ParseCode(c.String())
	if err != nil || !bytes.Equal(key, c.Key) || addr != "10.0.0.1:9000" {
		t.Fatalf("palt1: %v %q", err, addr)
	}
	p := Pairing{Key: key, Relay: addr}
	if p.Keys().GroupID != c.Keys().GroupID {
		t.Fatal("palt1 group changed")
	}
}

func TestNormalizeRelay(t *testing.T) {
	if a, _ := NormalizeRelay("relay.example.com"); a != "relay.example.com:9877" {
		t.Errorf("default port: %q", a)
	}
	if a, _ := NormalizeRelay(""); a != "" {
		t.Errorf("empty: %q", a)
	}
}
