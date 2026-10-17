// usb_linux.go handles a phone connected by USB cable with USB tethering
// on: a private network between the two that works without Wi-Fi or data.
//
// The tether shows up as a wired connection, which NetworkManager prefers
// over Wi-Fi for internet. A phone without data would then cut the laptop
// off, so PALT gives tether connections a route metric above Wi-Fi's: the
// laptop keeps using Wi-Fi and only falls back to the phone's data.
package main

import (
	"log"
	"net"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// usbTetherMetric is above NetworkManager's Wi-Fi default (600).
const usbTetherMetric = "900"

// Kernel drivers for phone USB tethering (Android RNDIS/NCM, iPhone).
var usbTetherDrivers = map[string]bool{
	"rndis_host": true,
	"cdc_ncm":    true,
	"cdc_ether":  true,
	"ipheth":     true,
}

// usbTetherIfaces returns the network interfaces backed by a phone's USB
// tethering.
func usbTetherIfaces() []net.Interface {
	ifaces, _ := net.Interfaces()
	var out []net.Interface
	for _, ifc := range ifaces {
		if isUSBIface(ifc.Name) {
			out = append(out, ifc)
		}
	}
	return out
}

// watchNetwork follows this computer's network interfaces: tether
// connections are kept from taking over the internet route, and any change
// (cable in or out, Wi-Fi on or off) re-checks links at once.
func (a *App) watchNetwork() {
	tuned := map[string]bool{}
	last := networkFingerprint()
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		if fp := networkFingerprint(); fp != last {
			last = fp
			log.Printf("[net] Network changed: %s", fp)
			a.onNetworkChanged()
		}
		seen := map[string]bool{}
		for _, ifc := range usbTetherIfaces() {
			seen[ifc.Name] = true
			if tuned[ifc.Name] {
				continue
			}
			if tuneUSBTether(ifc.Name) {
				tuned[ifc.Name] = true
				log.Printf("[usb] Phone connected by USB on %s", ifc.Name)
				a.kickConnector()
			}
		}
		for name := range tuned {
			if !seen[name] {
				delete(tuned, name)
			}
		}
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
		}
	}
}

// networkFingerprint lists the IPv4 addresses of the interfaces that are up.
func networkFingerprint() string {
	var out []string
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
				out = append(out, ifc.Name+"="+n.IP.String())
			}
		}
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

// tuneUSBTether sets the route metric of the connection on iface; false
// while NetworkManager hasn't set the interface up yet.
func tuneUSBTether(iface string) bool {
	out, err := exec.Command("nmcli", "-g", "GENERAL.CONNECTION", "device", "show", iface).Output()
	conn := strings.TrimSpace(string(out))
	if err != nil || conn == "" {
		return false
	}
	out, _ = exec.Command("nmcli", "-g", "ipv4.route-metric", "connection", "show", "id", conn).Output()
	if strings.TrimSpace(string(out)) == usbTetherMetric {
		return true
	}
	if err := exec.Command("nmcli", "connection", "modify", "id", conn,
		"ipv4.route-metric", usbTetherMetric, "ipv6.route-metric", usbTetherMetric).Run(); err != nil {
		log.Printf("[usb] Couldn't lower the priority of %s: %v", conn, err)
		return true
	}
	exec.Command("nmcli", "device", "reapply", iface).Run()
	return true
}
