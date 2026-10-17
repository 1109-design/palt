package discovery

import (
	"encoding/json"
	"log"
	"net"
	"sync"
	"time"

	"palt/internal/models"
)

// The beacon is a UDP broadcast fallback for networks where mDNS doesn't get
// through (common with Android and with routers that filter multicast).
// Every PALT 2.0 device listens on UDP at the PALT port. Queries are
// broadcast; whoever hears one answers the sender directly, so one broadcast
// finds every device on the subnet.

const (
	beaconInterval = 15 * time.Second
	beaconMax      = 1024
)

type beaconMsg struct {
	Palt int    `json:"palt"`
	ID   string `json:"id"`
	Name string `json:"name"`
	OS   string `json:"os"`
	Port int    `json:"port"`
	// Q asks everyone who hears it to answer.
	Q bool `json:"q,omitempty"`
}

type beacon struct {
	svc   *Service
	conn  *net.UDPConn
	kickC chan struct{}

	mu      sync.Mutex
	replied map[string]time.Time
}

func newBeacon(svc *Service) (*beacon, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: svc.port})
	if err != nil {
		return nil, err
	}
	return &beacon{svc: svc, conn: conn, kickC: make(chan struct{}, 1), replied: map[string]time.Time{}}, nil
}

func (b *beacon) close() { b.conn.Close() }

func (b *beacon) kick() {
	select {
	case b.kickC <- struct{}{}:
	default:
	}
}

func (b *beacon) msg(q bool) []byte {
	m, _ := json.Marshal(beaconMsg{Palt: 2, ID: b.svc.deviceID, Name: b.svc.deviceName, OS: b.svc.os, Port: b.svc.port, Q: q})
	return m
}

func (b *beacon) run() {
	defer b.svc.wg.Done()
	go b.listen()
	t := time.NewTicker(beaconInterval)
	defer t.Stop()
	for {
		b.query()
		select {
		case <-b.svc.ctx.Done():
			return
		case <-t.C:
		case <-b.kickC:
		}
	}
}

// query broadcasts to the global broadcast address and to each interface's
// subnet broadcast (some stacks only route the latter).
func (b *beacon) query() {
	m := b.msg(true)
	targets := map[string]bool{"255.255.255.255": true}
	if ifaces, err := net.Interfaces(); err == nil {
		for _, ifc := range ifaces {
			if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagBroadcast == 0 || ifc.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := ifc.Addrs()
			for _, a := range addrs {
				ipn, ok := a.(*net.IPNet)
				if !ok || ipn.IP.To4() == nil {
					continue
				}
				ip, mask := ipn.IP.To4(), ipn.Mask
				if len(mask) == 16 {
					mask = mask[12:]
				}
				bc := make(net.IP, 4)
				for i := range bc {
					bc[i] = ip[i] | ^mask[i]
				}
				targets[bc.String()] = true
			}
		}
	}
	for t := range targets {
		b.conn.WriteToUDP(m, &net.UDPAddr{IP: net.ParseIP(t), Port: b.svc.port})
	}
}

func (b *beacon) listen() {
	buf := make([]byte, beaconMax)
	for {
		n, from, err := b.conn.ReadFromUDP(buf)
		if err != nil {
			if b.svc.ctx.Err() != nil {
				return
			}
			log.Printf("[discovery] Beacon read: %v", err)
			time.Sleep(time.Second)
			continue
		}
		var m beaconMsg
		if json.Unmarshal(buf[:n], &m) != nil || m.Palt < 2 || m.ID == "" || m.ID == b.svc.deviceID {
			continue
		}
		if m.Port <= 0 || m.Port > 65535 {
			m.Port = b.svc.port
		}
		osName := m.OS
		if osName == "" {
			osName = "unknown"
		}
		ip := from.IP.String()
		b.svc.upsert(models.Peer{
			ID:         ip,
			DeviceName: m.Name,
			IPAddress:  ip,
			Port:       m.Port,
			OS:         osName,
			DeviceID:   m.ID,
		})
		if m.Q && b.shouldReply(ip) {
			b.conn.WriteToUDP(b.msg(false), &net.UDPAddr{IP: from.IP, Port: from.Port})
		}
	}
}

// shouldReply rate-limits answers to one per second per address.
func (b *beacon) shouldReply(ip string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if time.Since(b.replied[ip]) < time.Second {
		return false
	}
	b.replied[ip] = time.Now()
	if len(b.replied) > 256 {
		b.replied = map[string]time.Time{ip: time.Now()}
	}
	return true
}
