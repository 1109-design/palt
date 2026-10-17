// path_linux.go estimates how fast this computer can reach a LAN address,
// so links (and the transfers that follow them) take the faster of Wi-Fi
// and a USB cable.
package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// pathKind names how an address is reached: "usb", "wifi" or "lan".
func pathKind(ip string) string {
	ifc := ifaceFor(ip)
	switch {
	case ifc == "":
		return "lan"
	case isUSBIface(ifc):
		return "usb"
	case isWirelessIface(ifc):
		return "wifi"
	}
	return "lan"
}

// pathSpeed is a rough usable speed in Mbit/s to ip, at least 1.
func pathSpeed(ip string) int {
	ifc := ifaceFor(ip)
	var mbps float64
	switch {
	case ifc == "":
		mbps = 50
	case isWirelessIface(ifc):
		// Wi-Fi delivers about half its link rate, and device to device
		// every byte crosses the air twice (laptop → router → phone).
		mbps = float64(wifiRate()) * 0.3
	case isUSBIface(ifc):
		// USB tethering reports its bus speed; RNDIS/NCM overhead and the
		// phone's CPU cap it well below that.
		mbps = min(float64(linkSpeed(ifc)), 480) * 0.6
		if mbps <= 0 {
			mbps = 200
		}
	default:
		mbps = float64(linkSpeed(ifc)) * 0.9
	}
	return max(int(mbps), 1)
}

// ifaceFor returns the local interface on the same subnet as ip.
func ifaceFor(ip string) string {
	addr := net.ParseIP(ip)
	if addr == nil {
		return ""
	}
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.Contains(addr) {
				return ifc.Name
			}
		}
	}
	return ""
}

func isUSBIface(name string) bool {
	target, err := os.Readlink(filepath.Join("/sys/class/net", name, "device/driver"))
	return err == nil && usbTetherDrivers[filepath.Base(target)]
}

func isWirelessIface(name string) bool {
	_, err := os.Stat(filepath.Join("/sys/class/net", name, "wireless"))
	return err == nil
}

// linkSpeed is the interface's reported speed in Mbit/s, 0 if unknown.
func linkSpeed(name string) int {
	b, err := os.ReadFile(filepath.Join("/sys/class/net", name, "speed"))
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return max(n, 0)
}

var wifiRateCache struct {
	sync.Mutex
	at   time.Time
	mbps int
}

// wifiRate is the current Wi-Fi link rate in Mbit/s from NetworkManager.
func wifiRate() int {
	c := &wifiRateCache
	c.Lock()
	defer c.Unlock()
	if time.Since(c.at) < 15*time.Second {
		return c.mbps
	}
	c.at, c.mbps = time.Now(), 100
	out, err := exec.Command("nmcli", "-t", "-f", "IN-USE,RATE", "device", "wifi", "list", "--rescan", "no").Output()
	if err != nil {
		return c.mbps
	}
	for _, line := range strings.Split(string(out), "\n") {
		if rate, ok := strings.CutPrefix(line, "*:"); ok {
			if n, err := strconv.Atoi(strings.Fields(rate + " 0")[0]); err == nil && n > 0 {
				c.mbps = n
			}
			break
		}
	}
	return c.mbps
}
