package relay

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"palt/internal/transfer"
)

func tcpPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	ch := make(chan net.Conn, 1)
	go func() {
		c, _ := l.Accept()
		ch <- c
	}()
	a, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	b := <-ch
	t.Cleanup(func() { a.Close(); b.Close() })
	return a, b
}

func testCode(t *testing.T) Code {
	c, err := NewCode("127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCodeRoundTrip(t *testing.T) {
	c := testCode(t)
	p, err := ParseCode(c.String())
	if err != nil {
		t.Fatal(err)
	}
	if p.Host != c.Host || p.Port != c.Port || !bytes.Equal(p.Key, c.Key) {
		t.Fatalf("round trip mismatch: %+v vs %+v", p, c)
	}
	for _, bad := range []string{"", "palt1:", "palt1:host:1:short", "x:host:1:" + c.String()[len(c.String())-43:]} {
		if _, err := ParseCode(bad); err == nil {
			t.Errorf("ParseCode(%q) should fail", bad)
		}
	}
	if def, _ := NewCode("relay.example.com"); def.Port != DefaultPort {
		t.Errorf("default port = %d", def.Port)
	}
}

// Fixed vector shared with the Dart implementation's test.
func TestKeyVector(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	k := Code{Host: "h", Port: 1, Key: key}.Keys()
	if k.GroupID != "523da52e979666af9cf4525abf8fd282" {
		t.Errorf("group id = %s", k.GroupID)
	}
	if sig := k.PresenceSig("aaaaaaaaaaaaaaaa", "laptop", "linux"); sig != "87f067807a42146f8032c9714ef4eb84afaad7996f208cdd4d8ed793554bdd53" {
		t.Errorf("presence sig = %s", sig)
	}
}

func TestSecurePipe(t *testing.T) {
	keys := testCode(t).Keys()
	a, b := tcpPair(t)

	type res struct {
		c   *SecureConn
		err error
	}
	ch := make(chan res, 1)
	go func() {
		c, err := Handshake(b, keys, "sess", false)
		ch <- res{c, err}
	}()
	ca, err := Handshake(a, keys, "sess", true)
	if err != nil {
		t.Fatal(err)
	}
	r := <-ch
	if r.err != nil {
		t.Fatal(r.err)
	}
	cb := r.c

	payload := make([]byte, 300*1024+7) // spans several frames
	rand.Read(payload)
	go func() {
		ca.Write(payload)
		ca.CloseWrite()
	}()
	got, err := io.ReadAll(cb)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("payload mismatch")
	}

	cb.Write([]byte{1})
	one := make([]byte, 1)
	if _, err := io.ReadFull(ca, one); err != nil || one[0] != 1 {
		t.Fatalf("reply: %v %v", one, err)
	}
}

func TestSecurePipeWrongKey(t *testing.T) {
	a, b := tcpPair(t)
	go func() {
		if _, err := Handshake(b, testCode(t).Keys(), "sess", false); err != nil {
			b.Close()
		}
	}()
	if _, err := Handshake(a, testCode(t).Keys(), "sess", true); err == nil {
		t.Fatal("handshake with different keys must fail")
	}
}

func TestSecurePipeTamper(t *testing.T) {
	keys := testCode(t).Keys()
	a, b := tcpPair(t)
	// Middle-man that flips one byte of the first data frame.
	ma, mb := tcpPair(t)
	go func() {
		buf := make([]byte, 64)
		io.ReadFull(a, buf) // initiator hello → responder
		mb.Write(buf)
		io.ReadFull(mb, buf) // responder hello → initiator
		a.Write(buf)
		frame := make([]byte, 4+5+gcmTagSize)
		io.ReadFull(a, frame)
		frame[6] ^= 0xff
		mb.Write(frame)
	}()
	ch := make(chan *SecureConn, 1)
	go func() {
		c, _ := Handshake(ma, keys, "s", false)
		ch <- c
	}()
	ca, err := Handshake(b, keys, "s", true)
	if err != nil {
		t.Fatal(err)
	}
	ca.Write([]byte("hello"))
	cb := <-ch
	if cb == nil {
		t.Fatal("responder handshake failed")
	}
	if _, err := cb.Read(make([]byte, 16)); err == nil {
		t.Fatal("tampered frame must be rejected")
	}
}

// startTestRelay builds and runs a real palt-relay from ../../../relay and
// returns its port.
func startTestRelay(t *testing.T) int {
	t.Helper()
	relayDir, _ := filepath.Abs("../../../relay")
	if _, err := os.Stat(filepath.Join(relayDir, "relay.go")); err != nil {
		t.Skip("relay sources not found")
	}
	bin := filepath.Join(t.TempDir(), "palt-relay")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = relayDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build relay: %v\n%s", err, out)
	}
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	srv := exec.Command(bin, "-addr", "127.0.0.1:"+strconv.Itoa(port))
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Process.Kill(); srv.Wait() })
	time.Sleep(300 * time.Millisecond)
	return port
}

// TestThroughRelay runs two clients against a real palt-relay.
func TestThroughRelay(t *testing.T) {
	port := startTestRelay(t)

	code := testCode(t)
	code.Port = port

	got := make(chan []byte, 1)
	laptop := NewClient(code, "aaaaaaaaaaaaaaaa", "laptop", "linux")
	phone := NewClient(code, "bbbbbbbbbbbbbbbb", "phone", "android")
	phone.OnIncoming = func(c net.Conn, from Peer) {
		defer c.Close()
		if from.Name != "laptop" {
			t.Errorf("incoming from %q", from.Name)
		}
		b, _ := io.ReadAll(c)
		c.Write([]byte{1})
		got <- b
	}
	laptop.Start()
	phone.Start()
	t.Cleanup(func() { laptop.Stop(); phone.Stop() })

	deadline := time.Now().Add(5 * time.Second)
	var peer Peer
	for {
		if p, ok := laptop.PeerByName("phone"); ok {
			peer = p
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("phone never appeared on laptop")
		}
		time.Sleep(50 * time.Millisecond)
	}

	conn, err := laptop.Dial(peer.ID)
	if err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 1<<20)
	rand.Read(payload)
	conn.Write(payload)
	conn.(*SecureConn).CloseWrite()
	verdict := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, verdict); err != nil || verdict[0] != 1 {
		t.Fatalf("verdict %v %v", verdict, err)
	}
	if b := <-got; !bytes.Equal(b, payload) {
		t.Fatal("payload mismatch through relay")
	}

	// A device with another pairing code is invisible.
	stranger := NewClient(testCode(t), "cccccccccccccccc", "stranger", "linux")
	stranger.code.Port = port
	stranger.Start()
	defer stranger.Stop()
	time.Sleep(500 * time.Millisecond)
	if _, ok := laptop.PeerByName("stranger"); ok {
		t.Fatal("device from another group is visible")
	}
}

// TestDartInterop checks the Go and Dart implementations against each other
// through a real relay. It needs Flutter: PALT_DART_INTEROP=1 go test -run Dart
func TestDartInterop(t *testing.T) {
	if os.Getenv("PALT_DART_INTEROP") == "" {
		t.Skip("set PALT_DART_INTEROP=1 to run (needs flutter)")
	}
	port := startTestRelay(t)
	code := testCode(t)
	code.Port = port

	const fromPhone = 3*1024*1024 + 11
	laptop := NewClient(code, "aaaaaaaaaaaaaaaa", "laptop", "linux")
	incoming := make(chan error, 1)
	laptop.OnIncoming = func(c net.Conn, from Peer) {
		defer c.Close()
		buf := make([]byte, fromPhone)
		if _, err := io.ReadFull(c, buf); err != nil {
			incoming <- err
			return
		}
		for i := range buf {
			if buf[i] != byte((i*7)&0xff) {
				incoming <- fmt.Errorf("byte %d corrupted", i)
				return
			}
		}
		sum := sha256.Sum256(buf)
		c.Write(sum[:])
		incoming <- nil
	}
	laptop.Start()
	defer laptop.Stop()

	flutter := exec.Command("flutter", "test", "test/relay_test.dart", "--plain-name", "interop")
	flutter.Dir, _ = filepath.Abs("../../../mobile")
	flutter.Env = append(os.Environ(), "PALT_INTEROP_CODE="+code.String())
	var out bytes.Buffer
	flutter.Stdout, flutter.Stderr = &out, &out
	if err := flutter.Start(); err != nil {
		t.Skipf("flutter not available: %v", err)
	}

	// Go → Dart once the phone shows up.
	deadline := time.Now().Add(90 * time.Second)
	var phone Peer
	for {
		if p, ok := laptop.PeerByName("phone"); ok {
			phone = p
			break
		}
		if time.Now().After(deadline) {
			flutter.Process.Kill()
			t.Fatalf("phone never connected\n%s", out.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
	conn, err := laptop.Dial(phone.ID)
	if err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 1<<20+3)
	rand.Read(payload)
	conn.Write(payload)
	conn.(*SecureConn).CloseWrite()
	sum := make([]byte, 32)
	conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	if _, err := io.ReadFull(conn, sum); err != nil {
		t.Fatalf("reading digest from Dart: %v\n%s", err, out.String())
	}
	want := sha256.Sum256(payload)
	if !bytes.Equal(sum, want[:]) {
		t.Fatal("Dart received corrupted data")
	}
	conn.Close()

	// Dart → Go
	select {
	case err := <-incoming:
		if err != nil {
			t.Fatalf("Dart → Go: %v", err)
		}
	case <-time.After(60 * time.Second):
		t.Fatalf("Dart never dialed\n%s", out.String())
	}
	if err := flutter.Wait(); err != nil {
		t.Fatalf("flutter test failed: %v\n%s", err, out.String())
	}
}

// TestTransferOverRelay runs the real PALT file and text protocol through an
// encrypted relay pipe, as the app does.
func TestTransferOverRelay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // the transfer server saves to ~/Downloads/PALT
	port := startTestRelay(t)
	code := testCode(t)
	code.Port = port

	srv := transfer.NewServer(0)
	srv.OnOffer = func(meta transfer.Metadata) bool { return true }
	done := make(chan transfer.Metadata, 2)
	srv.OnComplete = func(meta transfer.Metadata, saved []string) { done <- meta }
	srv.OnText = func(meta transfer.Metadata) { done <- meta }

	laptop := NewClient(code, "aaaaaaaaaaaaaaaa", "laptop", "linux")
	phone := NewClient(code, "bbbbbbbbbbbbbbbb", "phone", "android")
	phone.OnIncoming = func(c net.Conn, from Peer) { srv.HandleConn(c, transfer.RouteRelay) }
	laptop.Start()
	phone.Start()
	t.Cleanup(func() { laptop.Stop(); phone.Stop() })

	var peer Peer
	for deadline := time.Now().Add(5 * time.Second); ; {
		if p, ok := laptop.PeerByName("phone"); ok {
			peer = p
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("phone never appeared")
		}
		time.Sleep(50 * time.Millisecond)
	}
	dial := func() (net.Conn, error) { return laptop.Dial(peer.ID) }

	src := filepath.Join(t.TempDir(), "photo.bin")
	content := make([]byte, 5<<20+123)
	rand.Read(content)
	os.WriteFile(src, content, 0o644)

	if err := transfer.SendFiles(dial, []string{src}, "t1", "laptop", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case meta := <-done:
		if meta.Route != transfer.RouteRelay {
			t.Errorf("route = %q", meta.Route)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("file never completed")
	}
	got, err := os.ReadFile(filepath.Join(home, "Downloads", "PALT", "photo.bin"))
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("received file mismatch: %v", err)
	}

	if err := transfer.SendText(dial, "t2", "laptop", "hello over relay"); err != nil {
		t.Fatal(err)
	}
	select {
	case meta := <-done:
		if meta.Text != "hello over relay" {
			t.Errorf("text = %q", meta.Text)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("text never arrived")
	}
}
