// hotspot_linux.go turns this computer into a Wi-Fi hotspot through
// NetworkManager, to give a paired phone this computer's internet.
//
// Only while the Wi-Fi card is free (internet over a cable or USB): most
// cards can't be a hotspot and a Wi-Fi client at once except on the same
// channel, and regulations usually forbid hotspots on 5 GHz. On Wi-Fi the
// phone simply joins the same network instead (Share Wi-Fi).
package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"palt/internal/wifiqr"
)

const hotspotConn = "PALT Hotspot"

// errOnWifi means this computer is on Wi-Fi itself; share that instead.
var errOnWifi = errors.New("this computer is on Wi-Fi")

// wifiDevice returns the first Wi-Fi interface and its NetworkManager state.
func wifiDevice() (name, state string) {
	out, _ := exec.Command("nmcli", "-t", "-f", "DEVICE,TYPE,STATE", "device").Output()
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Split(line, ":")
		if len(f) == 3 && f[1] == "wifi" {
			return f[0], f[2]
		}
	}
	return "", ""
}

// hotspotNetwork is the hotspot's name and password, made once and kept so
// the phone recognises it next time.
func hotspotNetwork() (wifiqr.Network, error) {
	dir, err := configDir()
	if err != nil {
		return wifiqr.Network{}, err
	}
	file := filepath.Join(dir, "hotspot.json")
	var n wifiqr.Network
	if data, err := os.ReadFile(file); err == nil && json.Unmarshal(data, &n) == nil && n.SSID != "" {
		return n, nil
	}
	host, _ := os.Hostname()
	if len(host) > 20 {
		host = host[:20]
	}
	const letters = "abcdefghijkmnpqrstuvwxyz23456789"
	b := make([]byte, 12)
	rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	n = wifiqr.Network{SSID: "PALT " + host, Password: string(b), Security: "WPA"}
	data, _ := json.Marshal(n)
	return n, os.WriteFile(file, data, 0o600)
}

// startHotspot starts the hotspot and returns its network.
func startHotspot() (wifiqr.Network, error) {
	dev, state := wifiDevice()
	if dev == "" {
		return wifiqr.Network{}, errors.New("this computer has no Wi-Fi")
	}
	if strings.HasPrefix(state, "connected") && !hotspotActive() {
		return wifiqr.Network{}, errOnWifi
	}
	n, err := hotspotNetwork()
	if err != nil {
		return n, err
	}
	// 2.4 GHz: allowed everywhere and reached by every phone.
	out, err := exec.Command("nmcli", "device", "wifi", "hotspot", "ifname", dev,
		"con-name", hotspotConn, "ssid", n.SSID, "band", "bg", "password", n.Password).CombinedOutput()
	if err != nil {
		return n, fmt.Errorf("couldn't start the hotspot: %s", strings.TrimSpace(strings.TrimPrefix(string(out), "Error: ")))
	}
	return n, nil
}

// stopHotspot turns the hotspot off.
func stopHotspot() error {
	if !hotspotActive() {
		return nil
	}
	return exec.Command("nmcli", "connection", "down", "id", hotspotConn).Run()
}

func hotspotActive() bool {
	out, _ := exec.Command("nmcli", "-t", "-f", "NAME", "connection", "show", "--active").Output()
	for _, line := range strings.Split(string(out), "\n") {
		if line == hotspotConn {
			return true
		}
	}
	return false
}
