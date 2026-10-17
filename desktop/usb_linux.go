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
	"os"
	"os/exec"
	"path/filepath"
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
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []net.Interface
	for _, ifc := range ifaces {
		target, err := os.Readlink(filepath.Join("/sys/class/net", ifc.Name, "device/driver"))
		if err == nil && usbTetherDrivers[filepath.Base(target)] {
			out = append(out, ifc)
		}
	}
	return out
}

// isUSBAddr reports whether ip is reached over a USB tether.
func isUSBAddr(ip string) bool {
	addr := net.ParseIP(ip)
	if addr == nil {
		return false
	}
	for _, ifc := range usbTetherIfaces() {
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.Contains(addr) {
				return true
			}
		}
	}
	return false
}

// watchUSBTether keeps tether connections from taking over the laptop's
// internet route, and wakes the connector when a cable is plugged in.
func (a *App) watchUSBTether() {
	tuned := map[string]bool{}
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
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
