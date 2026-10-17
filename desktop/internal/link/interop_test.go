package link

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"palt/internal/secure"
	"palt/internal/transfer"
)

// TestDartLANInterop checks the Go and Dart implementations of paired LAN
// connections and links against each other. It needs Flutter:
// PALT_DART_INTEROP=1 go test -run DartLAN ./internal/link
func TestDartLANInterop(t *testing.T) {
	if os.Getenv("PALT_DART_INTEROP") == "" {
		t.Skip("set PALT_DART_INTEROP=1 to run (needs flutter)")
	}
	key := make([]byte, 32)
	rand.Read(key)
	keys := secure.DeriveKeys(key)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(func() Identity { return Identity{ID: "aaaaaaaaaaaaaaaa", Name: "laptop", OS: "linux"} })
	texts := make(chan transfer.Metadata, 1)
	phonePort := make(chan int, 1)
	m.Handle("echo", func(l *Link, p Packet) { l.SendID("echo", p.ID, p.Body) })
	m.Handle("port", func(l *Link, p Packet) {
		var b struct{ Port int }
		p.Decode(&b)
		phonePort <- b.Port
	})
	srv := &transfer.Server{
		Keys:   func() (secure.Keys, bool) { return keys, true },
		OnText: func(meta transfer.Metadata) { texts <- meta },
		OnLink: func(c net.Conn, meta transfer.Metadata) { m.Run(c, meta.Route, false) },
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go srv.HandleConn(c, transfer.RouteDirect)
		}
	}()
	defer l.Close()

	flutter := exec.Command("flutter", "test", "test/lan_test.dart", "--plain-name", "interop")
	flutter.Dir, _ = filepath.Abs("../../../mobile")
	flutter.Env = append(os.Environ(),
		"PALT_LAN_KEY="+base64.StdEncoding.EncodeToString(key),
		fmt.Sprintf("PALT_LAN_PORT=%d", l.Addr().(*net.TCPAddr).Port))
	var out bytes.Buffer
	flutter.Stdout, flutter.Stderr = &out, &out
	if err := flutter.Start(); err != nil {
		t.Skipf("flutter not available: %v", err)
	}
	fail := func(f string, a ...any) {
		flutter.Process.Kill()
		t.Fatalf(f+"\n%s", append(a, out.String())...)
	}

	// 1. Dart sends a paired text message.
	select {
	case meta := <-texts:
		if !meta.Paired || meta.SenderID != "bbbbbbbbbbbbbbbb" || meta.Text != "hello from dart" {
			fail("text: %+v", meta)
		}
	case <-time.After(90 * time.Second):
		fail("no text from Dart")
	}

	// 2. Dart opens a link and tells us its own server port; then Go
	// dials Dart and opens a second link, which replaces the first on
	// both sides (lower ID "aaaa…" dialed it).
	var port int
	select {
	case port = <-phonePort:
	case <-time.After(30 * time.Second):
		fail("no link from Dart")
	}
	first := m.Get("bbbbbbbbbbbbbbbb")
	if first == nil || first.Peer.Name != "phone" || !first.Supports("ping") {
		fail("first link: %+v", first)
	}
	dial := transfer.SecureDialer(transfer.DirectDialer("127.0.0.1", port, 5*time.Second), "aaaaaaaaaaaaaaaa", keys)
	conn, err := dial()
	if err != nil {
		fail("dial Dart: %v", err)
	}
	transfer.WriteOffer(conn, &transfer.Metadata{Action: transfer.ActionLink, TransferID: "x", SenderName: "laptop"})
	v := make([]byte, 1)
	if _, err := io.ReadFull(conn, v); err != nil || v[0] != transfer.ResponseAccept {
		fail("Dart refused link: %v %v", v, err)
	}
	second, err := m.Run(conn, RouteDirect, true)
	if err != nil {
		fail("second link: %v", err)
	}
	select {
	case <-first.Done():
	case <-time.After(5 * time.Second):
		fail("first link not replaced")
	}
	// Dart checks it kept the same link by answering on it.
	if err := second.Send("done", map[string]string{"ok": "yes"}); err != nil {
		fail("send done: %v", err)
	}
	if err := flutter.Wait(); err != nil {
		t.Fatalf("flutter test failed: %v\n%s", err, out.String())
	}
}
