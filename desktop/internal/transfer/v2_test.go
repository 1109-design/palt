package transfer

import (
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"palt/internal/secure"
)

func testKeys() secure.Keys {
	k := make([]byte, 32)
	rand.Read(k)
	return secure.DeriveKeys(k)
}

// startServer runs a Server on a free port.
func startServer(t *testing.T, keys secure.Keys) (*Server, Dialer) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{listener: l, Keys: func() (secure.Keys, bool) { return keys, true }}
	s.running.Store(true)
	go s.acceptLoop()
	t.Cleanup(s.Stop)
	return s, func() (net.Conn, error) { return net.Dial("tcp", l.Addr().String()) }
}

func TestPairedText(t *testing.T) {
	keys := testKeys()
	s, raw := startServer(t, keys)
	got := make(chan Metadata, 1)
	s.OnText = func(m Metadata) { got <- m }

	if err := SendText(SecureDialer(raw, "phone1", keys), "t1", "phone", "hi"); err != nil {
		t.Fatal(err)
	}
	m := <-got
	if !m.Paired || m.SenderID != "phone1" || m.Text != "hi" {
		t.Fatalf("paired text: %+v", m)
	}

	// Plain v1 still works and is never marked paired, whatever it claims.
	if err := SendText(raw, "t2", "phone", "plain"); err != nil {
		t.Fatal(err)
	}
	if m := <-got; m.Paired || m.SenderID != "" {
		t.Fatalf("v1 text marked paired: %+v", m)
	}

	// A device from another group can't get in.
	if err := SendText(SecureDialer(raw, "evil", testKeys()), "t3", "phone", "x"); err == nil {
		t.Fatal("wrong key must be refused")
	}
}

func TestPairedFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	keys := testKeys()
	s, raw := startServer(t, keys)
	done := make(chan Metadata, 1)
	s.OnOffer = func(m Metadata) bool { return m.Paired }
	s.OnComplete = func(m Metadata, paths []string) { done <- m }

	src := filepath.Join(t.TempDir(), "a.bin")
	data := make([]byte, 200*1024+3)
	rand.Read(data)
	os.WriteFile(src, data, 0o644)
	if err := SendFiles(SecureDialer(raw, "phone1", keys), []string{src}, "f1", "phone", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-done:
		if !m.Paired {
			t.Fatal("not paired")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("transfer never completed")
	}
	out, _ := os.ReadFile(filepath.Join(home, "Downloads", "PALT", "a.bin"))
	if string(out) != string(data) {
		t.Fatal("file corrupted")
	}
}

func TestLinkHandoff(t *testing.T) {
	keys := testKeys()
	s, raw := startServer(t, keys)
	linked := make(chan net.Conn, 1)
	s.OnLink = func(c net.Conn, m Metadata) {
		if !m.Paired || m.SenderID != "phone1" {
			t.Errorf("link meta: %+v", m)
		}
		linked <- c
	}
	conn, err := SecureDialer(raw, "phone1", keys)()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	WriteOffer(conn, &Metadata{Action: ActionLink, TransferID: "l", SenderName: "phone"})
	v := make([]byte, 1)
	if _, err := conn.Read(v); err != nil || v[0] != ResponseAccept {
		t.Fatalf("verdict %v %v", v, err)
	}
	sc := <-linked
	conn.Write([]byte("ping"))
	buf := make([]byte, 4)
	if _, err := sc.Read(buf); err != nil || string(buf) != "ping" {
		t.Fatalf("link data: %q %v", buf, err)
	}

	// Unpaired devices can't open links.
	c2, _ := raw()
	defer c2.Close()
	WriteOffer(c2, &Metadata{Action: ActionLink, TransferID: "l2", SenderName: "x"})
	c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	if n, _ := c2.Read(v); n == 1 && v[0] == ResponseAccept {
		t.Fatal("plain link accepted")
	}
}
