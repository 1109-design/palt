// palt-relay is a blind byte pipe that lets PALT devices reach each other
// when they are not on the same network.
//
// Devices of one owner share a pairing key. The relay only ever sees a group
// id derived from it, so it can match devices of the same group but can't read
// what they send: every relayed stream is end-to-end encrypted by the apps.
// Nothing is written to disk; bytes are copied between sockets in memory.
//
// Wire protocol: every connection starts with one JSON line from the client.
//
//	register  long-lived control connection; receives "peers" and "incoming"
//	connect   ask for a pipe to device "to" under a fresh "session" id
//	accept    the target's answer to an "incoming" notice for "session"
//
// After the relay answers {"t":"ok"} on a connect/accept connection, the
// socket carries raw bytes to and from the other device.
package main

import (
	"flag"
	"log"
	"net"
)

func main() {
	addr := flag.String("addr", ":9877", "TCP address to listen on")
	flag.Parse()

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %s: %v", *addr, err)
	}
	log.Printf("palt-relay listening on %s", *addr)
	log.Fatal(NewRelay().Serve(l))
}
