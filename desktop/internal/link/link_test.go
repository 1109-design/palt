package link

import (
	"net"
	"testing"
	"time"
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

func manager(id string) *Manager {
	return NewManager(func() Identity { return Identity{ID: id, Name: "dev-" + id, OS: "linux"} })
}

// connect runs both ends of a link between a (dialing) and b.
func connect(t *testing.T, a, b *Manager, route string) (*Link, *Link, error, error) {
	t.Helper()
	ca, cb := tcpPair(t)
	type res struct {
		l   *Link
		err error
	}
	ch := make(chan res, 1)
	go func() {
		l, err := b.Run(cb, route, false)
		ch <- res{l, err}
	}()
	la, errA := a.Run(ca, route, true)
	r := <-ch
	return la, r.l, errA, r.err
}

func TestLinkPackets(t *testing.T) {
	a, b := manager("aaaa"), manager("bbbb")
	got := make(chan Packet, 1)
	b.Handle("battery", func(l *Link, p Packet) { got <- p })

	la, lb, errA, errB := connect(t, a, b, RouteDirect)
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}
	if la.Peer.ID != "bbbb" || lb.Peer.ID != "aaaa" || lb.Peer.Name != "dev-aaaa" {
		t.Fatalf("identities: %+v %+v", la.Peer, lb.Peer)
	}
	if !la.Supports("battery") || lb.Supports("battery") {
		t.Fatal("capabilities not exchanged")
	}
	if err := la.Send("battery", map[string]int{"level": 42}); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-got:
		var body struct{ Level int }
		if p.Decode(&body) != nil || body.Level != 42 {
			t.Fatalf("body: %s", p.Body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("packet never arrived")
	}
	if a.Get("bbbb") != la || b.GetByName("dev-aaaa") != lb {
		t.Fatal("links not registered")
	}

	la.Close()
	deadline := time.Now().Add(3 * time.Second)
	for b.Get("aaaa") != nil {
		if time.Now().After(deadline) {
			t.Fatal("peer never noticed the link closing")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDuplicateLinksAgree(t *testing.T) {
	a, b := manager("aaaa"), manager("bbbb")
	// b dials first, then a: a's link (lower ID dialed) must win on both sides.
	_, _, err1, err2 := connect(t, b, a, RouteDirect)
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	first := a.Get("bbbb")
	la, lb, errA, errB := connect(t, a, b, RouteDirect)
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}
	if a.Get("bbbb") != la || b.Get("aaaa") != lb {
		t.Fatal("sides kept different links")
	}
	select {
	case <-first.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("losing link was not closed")
	}

	// A relay link never replaces a direct one.
	_, _, errA, errB = connect(t, a, b, RouteRelay)
	if errA == nil || errB == nil {
		t.Fatal("relay link should lose to the direct one")
	}
	if a.Get("bbbb") != la {
		t.Fatal("direct link was replaced")
	}
}

func TestDirectReplacesRelay(t *testing.T) {
	a, b := manager("aaaa"), manager("bbbb")
	lr, _, _, _ := connect(t, b, a, RouteRelay)
	if lr == nil {
		t.Fatal("relay link failed")
	}
	// b dialed directly; even though b has the higher ID, direct wins.
	ld, _, errA, errB := connect(t, b, a, RouteDirect)
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}
	if b.Get("aaaa") != ld || a.Get("bbbb").Route != RouteDirect {
		t.Fatal("direct link did not replace relay link")
	}
}
