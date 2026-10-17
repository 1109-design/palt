package secure

import (
	"bytes"
	"crypto/rand"
	"io"
	"net"
	"testing"
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

func testKeys() Keys {
	k := make([]byte, 32)
	rand.Read(k)
	return DeriveKeys(k)
}

// Fixed vector shared with the Dart implementation's test.
func TestKeyVector(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	k := DeriveKeys(key)
	if k.GroupID != "523da52e979666af9cf4525abf8fd282" {
		t.Errorf("group id = %s", k.GroupID)
	}
	if sig := k.PresenceSig("aaaaaaaaaaaaaaaa", "laptop", "linux"); sig != "87f067807a42146f8032c9714ef4eb84afaad7996f208cdd4d8ed793554bdd53" {
		t.Errorf("presence sig = %s", sig)
	}
}

func TestSecurePipe(t *testing.T) {
	keys := testKeys()
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
		if _, err := Handshake(b, testKeys(), "sess", false); err != nil {
			b.Close()
		}
	}()
	if _, err := Handshake(a, testKeys(), "sess", true); err == nil {
		t.Fatal("handshake with different keys must fail")
	}
}

func TestSecurePipeTamper(t *testing.T) {
	keys := testKeys()
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
