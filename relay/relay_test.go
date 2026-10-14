package main

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"
)

const testGroup = "0123456789abcdef0123456789abcdef"

func startRelay(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go NewRelay().Serve(l)
	return l.Addr().String()
}

func send(t *testing.T, c net.Conn, m Msg) {
	t.Helper()
	m.V = protocolVersion
	b, _ := json.Marshal(m)
	if _, err := c.Write(append(b, '\n')); err != nil {
		t.Fatal(err)
	}
}

func recv(t *testing.T, c net.Conn) Msg {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	m, err := readLine(c)
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	return m
}

// recvType skips messages until one of type typ arrives.
func recvType(t *testing.T, c net.Conn, typ string) Msg {
	t.Helper()
	for {
		m := recv(t, c)
		if m.T == typ {
			return m
		}
	}
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func register(t *testing.T, addr, id, name string) net.Conn {
	c := dial(t, addr)
	send(t, c, Msg{T: "register", Group: testGroup, ID: id, Name: name, OS: "linux", Sig: "s"})
	if m := recv(t, c); m.T != "ok" {
		t.Fatalf("register %s: %+v", name, m)
	}
	return c
}

func TestPresenceAndPipe(t *testing.T) {
	addr := startRelay(t)
	const idA, idB = "aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb"

	ctlA := register(t, addr, idA, "laptop")
	ctlB := register(t, addr, idB, "phone")

	// A eventually sees both devices.
	for {
		m := recvType(t, ctlA, "peers")
		if len(m.Peers) == 2 {
			break
		}
	}

	const session = "1111111111111111"
	conA := dial(t, addr)
	send(t, conA, Msg{T: "connect", Group: testGroup, ID: idA, To: idB, Session: session})

	in := recvType(t, ctlB, "incoming")
	if in.Session != session || in.From != idA {
		t.Fatalf("incoming: %+v", in)
	}
	conB := dial(t, addr)
	send(t, conB, Msg{T: "accept", Group: testGroup, ID: idB, Session: session})

	if m := recv(t, conA); m.T != "ok" {
		t.Fatalf("connect reply: %+v", m)
	}
	if m := recv(t, conB); m.T != "ok" {
		t.Fatalf("accept reply: %+v", m)
	}

	// A → B, then half-close; B answers and closes.
	conA.Write([]byte("hello"))
	conA.(*net.TCPConn).CloseWrite()
	got, _ := io.ReadAll(conB)
	if string(got) != "hello" {
		t.Fatalf("B got %q", got)
	}
	conB.Write([]byte{0x01})
	conB.Close()
	conA.SetReadDeadline(time.Now().Add(3 * time.Second))
	back, _ := io.ReadAll(conA)
	if string(back) != "\x01" {
		t.Fatalf("A got %q", back)
	}
}

func TestConnectToOfflineDevice(t *testing.T) {
	addr := startRelay(t)
	register(t, addr, "aaaaaaaaaaaaaaaa", "laptop")

	c := dial(t, addr)
	send(t, c, Msg{T: "connect", Group: testGroup, ID: "aaaaaaaaaaaaaaaa", To: "cccccccccccccccc", Session: "2222222222222222"})
	if m := recv(t, c); m.T != "err" {
		t.Fatalf("want err, got %+v", m)
	}
}

func TestOtherGroupIsInvisible(t *testing.T) {
	addr := startRelay(t)
	ctl := register(t, addr, "aaaaaaaaaaaaaaaa", "laptop")

	other := dial(t, addr)
	send(t, other, Msg{T: "register", Group: "ffffffffffffffffffffffffffffffff", ID: "dddddddddddddddd", Name: "stranger", OS: "linux"})
	recv(t, other)

	// A connect from the other group must not reach our device.
	c := dial(t, addr)
	send(t, c, Msg{T: "connect", Group: "ffffffffffffffffffffffffffffffff", ID: "dddddddddddddddd", To: "aaaaaaaaaaaaaaaa", Session: "3333333333333333"})
	if m := recv(t, c); m.T != "err" {
		t.Fatalf("cross-group connect allowed: %+v", m)
	}

	// Our device only ever sees itself.
	ctl.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	br := bufio.NewReader(ctl)
	for {
		line, err := br.ReadBytes('\n')
		if err != nil {
			break
		}
		var m Msg
		json.Unmarshal(line, &m)
		if m.T == "peers" && len(m.Peers) != 1 {
			t.Fatalf("leaked peers: %+v", m.Peers)
		}
	}
}

func TestAcceptForUnknownSession(t *testing.T) {
	addr := startRelay(t)
	c := dial(t, addr)
	send(t, c, Msg{T: "accept", Group: testGroup, ID: "aaaaaaaaaaaaaaaa", Session: "4444444444444444"})
	if m := recv(t, c); m.T != "err" {
		t.Fatalf("want err, got %+v", m)
	}
}
